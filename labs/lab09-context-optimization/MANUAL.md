# Manual: Lab 09 — Context Optimization

## Why this matters

The lab teaches **context hygiene**: counting tokens correctly, deciding correctly when "it's time to compress", and compressing correctly. No multi-level strategies — that's been thrown out as over-engineering in [Ch. 13](../../book/13-context-engineering/README.md).

### Real-world case

A DevOps agent in production. After an hour of working with logs and many tools:

- **Without token counting**: sooner or later the provider returns `context_length_exceeded`, the agent crashes mid-incident.
- **With `len(text)/4` as the source of truth**: your counter drifts from the real `usage.PromptTokens` by 30-50% (Russian, special characters, JSON tool outputs). Decisions are made with that error baked in.
- **With `usage.PromptTokens`**: you know exactly what the billing system charged you. One threshold, one reaction.

## Theory in simple terms

### Where token counts come from

```text
provider response
  ├── choices[0].message
  └── usage
        ├── prompt_tokens     ← how much went into input (system + history + tools)
        ├── completion_tokens ← how much was generated
        └── total_tokens
```

`prompt_tokens` is the authoritative value. The provider counted exactly the amount it billed you for. Any counter you write yourself is good only before the first response (cold start) or for the "will this message fit before sending it" estimate.

### One threshold, one reaction

It's tempting to do "do nothing → prioritize → summarize → eviction". Don't. In practice this is enough:

```text
if lastTokens > contextMax * 0.80 && !condenseDone {
    condense()
}
```

And separately — the reactive branch:

```text
if isContextOverflow(err) {
    condense()
    retry once
}
```

Why the ladder is bad:

- Each "technique" breaks something (tool-call pairs, prompt cache).
- Complex selection logic → harder to debug.
- In reality, after the first `condense` you won't hit the window again in the same Run. If you do, the task isn't decomposable, and the right thing is to bubble the error up — not to pile on yet another technique.

### Tool-pair protection

The OpenAI-compatible protocol: an `assistant` message with `tool_calls` MUST be followed by `tool` messages with the matching `tool_call_id`. If compression/truncation breaks the pair, the provider returns 400.

```text
ok:    [system, user, assistant{tc:42}, tool{tc:42}, assistant, ...]
fail:  [system, ......................   tool{tc:42}, assistant, ...]
                                          ^^ no matching assistant tool_call in the tail
```

Solution: `safeTail`, which while building the tail moves the left boundary further left as long as the first element of the tail is a `tool`.

### Where to put the summary

Only as a regular `user` message — **not** as `system`:

```text
[system, user("Context of previous work:\n\n<summary>"), tail...]
```

Why not `system`:

- prompt cache is built around a stable prefix. Every change to system → cache miss → expensive.
- The model distinguishes "instructions" (system) from "data" (user). Mixing them creates confusion: the model can start "executing" the summary as an instruction.

## Step-by-step

### Step 1: rough token estimate (for pre-send)

```go
func estimateTokens(text string) int {
    if text == "" { return 0 }
    return len(text) / 3 + 1
}

func estimateMessages(msgs []openai.ChatCompletionMessage) int {
    total := 4 // envelope overhead
    for _, m := range msgs {
        total += estimateTokens(m.Content) + 4
        for _, tc := range m.ToolCalls {
            total += estimateTokens(tc.Function.Name) +
                     estimateTokens(tc.Function.Arguments) + 8
        }
    }
    return total
}
```

This is a poor estimate for billing, but good enough for "don't send a clearly overflowing batch". For production use `tiktoken-go` for your model — but still cross-check against `usage.PromptTokens`.

### Step 2: the `Run` loop

```go
type Run struct {
    messages     []openai.ChatCompletionMessage
    lastTokens   int
    contextMax   int
    condenseDone bool

    client *openai.Client
    model  string
    tools  []openai.Tool
}

func (r *Run) Step(ctx context.Context, userInput string) (string, error) {
    r.messages = append(r.messages, openai.ChatCompletionMessage{
        Role: openai.ChatMessageRoleUser, Content: userInput,
    })

    for {
        if r.lastTokens > 0 &&
            float64(r.lastTokens) > float64(r.contextMax)*0.80 {
            if err := r.condense(ctx); err != nil {
                return "", err
            }
        }

        resp, err := r.callLLM(ctx)
        if err != nil {
            if !isContextOverflow(err) { return "", err }
            if cerr := r.condense(ctx); cerr != nil { return "", cerr }
            resp, err = r.callLLM(ctx)
            if err != nil {
                return "", fmt.Errorf("overflow even after condense: %w", err)
            }
        }

        r.lastTokens = resp.Usage.PromptTokens
        msg := resp.Choices[0].Message
        r.messages = append(r.messages, msg)

        if len(msg.ToolCalls) == 0 {
            return msg.Content, nil
        }

        for _, tc := range msg.ToolCalls {
            result := r.dispatchTool(ctx, tc)
            r.messages = append(r.messages, openai.ChatCompletionMessage{
                Role:       openai.ChatMessageRoleTool,
                ToolCallID: tc.ID,
                Name:       tc.Function.Name,
                Content:    result,
            })
        }
    }
}
```

### Step 3: `safeTail`

```go
func safeTail(msgs []openai.ChatCompletionMessage, n int) []openai.ChatCompletionMessage {
    if n > len(msgs)-1 { n = len(msgs) - 1 }
    start := len(msgs) - n
    for start > 1 && msgs[start].Role == openai.ChatMessageRoleTool {
        start--
    }
    return msgs[start:]
}
```

This is enough for most cases. If you want it perfect (don't separate an assistant with pending `tool_calls` from its tools), do a reverse walk and verify that for every `tool` in the tail there's a matching `assistant` with that `tool_call_id`.

### Step 4: `condense`

```go
func (r *Run) condense(ctx context.Context) error {
    if r.condenseDone || len(r.messages) < 6 {
        return nil
    }

    system := r.messages[0]
    tail := safeTail(r.messages, 4)
    head := r.messages[1 : len(r.messages)-len(tail)]

    summary, err := r.summarize(ctx, head)
    if err != nil {
        return err
    }

    next := make([]openai.ChatCompletionMessage, 0, 2+len(tail))
    next = append(next, system)
    next = append(next, openai.ChatCompletionMessage{
        Role:    openai.ChatMessageRoleUser,
        Content: "Context of previous work:\n\n" + summary,
    })
    next = append(next, tail...)

    r.messages = next
    r.condenseDone = true
    return nil
}
```

Prompt for `summarize`:

```text
You are compressing the agent's working transcript into a brief handoff for the next step.
Preserve:
1. The user's original task.
2. Decisions already made and the reasoning behind them.
3. Which files / resources have been read and what's relevant in them.
4. What still needs to be done.
Drop pleasantries and chatter.
```

### Step 5: estimated/actual log

After every `r.callLLM` print:

```go
estimated := estimateMessages(r.messages)
fmt.Printf("Step %d: estimated=%d, actual=%d (Δ=%+d, %+.1f%%), threshold@80%%=%d\n",
    step, estimated, r.lastTokens, r.lastTokens-estimated,
    float64(r.lastTokens-estimated)*100/float64(estimated),
    int(float64(r.contextMax)*0.80))
```

## Common errors

### Error 1: `len(text)/4` as the source of truth

**Symptom:** `condense` fires either too early (LLM summary call wasted) or too late (you get a 400 from the provider).

**Cause:** your counter doesn't reflect the model's actual tokenization + envelope/tools overhead.

**Solution:** `r.lastTokens = resp.Usage.PromptTokens` after **every** response. Your own counter — only for the "will this fit" estimate before sending.

### Error 2: the `prioritize → summarize → truncate` ladder

**Symptom:** three optimization branches in the code; one of them eventually breaks tool-call pairs or reorders messages.

**Cause:** copied from old articles. In reality one operation is enough.

**Solution:** one `condense` by threshold + reactive condense on overflow. If you overflow again after the first condense — fail, decompose the task.

### Error 3: truncate without checking tool pairs

**Symptom:** the provider returns 400 "tool_calls without matching tool messages".

**Cause:** you cut exactly between `assistant.tool_call` and the `tool` result.

**Solution:** `safeTail` or an explicit reverse walk that checks pairs.

### Error 4: summary in `system`

**Symptom:** every `condense` makes a single request more expensive (cache miss); the model occasionally starts "executing" the summary as instructions.

**Cause:** the summary was placed in a `system` message or glued onto the original system.

**Solution:** the summary is a regular `user` message between `system[0]` and the tail.

### Error 5: condense fires every step

**Symptom:** the agent slows down, every step adds an extra LLM call, the history shrinks-grows-shrinks, context gets confused.

**Cause:** no limit.

**Solution:** a `condenseDone bool` flag. If you overflow again after the first condense — that's a signal the task isn't decomposable. Bubble the error up.

### Error 6: hardcoded `maxContextTokens = 4000`

**Symptom:** the code is tied to one model; switching models breaks limits.

**Cause:** a constant in the file.

**Solution:** the window comes from model configuration (provider metadata / model catalog), not hardcoded.

## Mini-exercises

### Exercise 1: precise estimation via `tiktoken-go`

Add `github.com/pkoukk/tiktoken-go`, implement `estimateTokensTiktoken(text, modelName)`, run both counters and compare with `usage.PromptTokens`. Tiktoken gives precision for OpenAI models, but still won't cover provider-specific tokenizers (Anthropic, GigaChat, etc.).

### Exercise 2: a "perfect" `safeTail`

Rewrite `safeTail` so that it:

1. Doesn't separate `assistant{tool_calls=[a,b,c]}` from `tool{a}, tool{b}, tool{c}`.
2. Returns exactly `>=N` messages, not `=N` (if a pair boundary blocks it — expand).

### Exercise 3: artifacts for large tool results

If a tool returns >2000 tokens (logs, JSON dumps), don't put the whole output in `messages`. Save it to a file/KV under an `artifact_id`, and in the `tool` message put a short excerpt + the ID. This reduces token spend and makes `condense` less frequent.

## Completion criteria

✅ **Done:**

- [x] The "time to compress" decision is made from `resp.Usage.PromptTokens`.
- [x] Single threshold `0.80` + reactive `condense` on `ContextOverflowError`.
- [x] `condense` runs at most once per Run; the retry after the reactive condense is exactly one.
- [x] After `condense`: `messages[0]` is stable; tool-call pairs are intact; the summary is in the `user` role.
- [x] Logs show estimated and actual tokens; the moment condense fires is visible.
- [x] The model window comes from configuration, not hardcoded.

❌ **Not done:**

- [ ] `len(text)/4` as the source of truth.
- [ ] The `prioritize → summarize → truncate` ladder.
- [ ] Truncate without `safeTail`.
- [ ] Summary in the `system` role (or `system+summary+facts+working` glue).
- [ ] Multiple condense calls in a row without a limit.
- [ ] Hardcoded `4000` as the "default window" — that's legacy from old models; take the limit from model metadata / configuration.

---

**Next step:** [Lab 10: Planning and Workflows](../lab10-planning-workflows/README.md). Long-term memory — in [Lab 11: Memory & Context](../lab11-memory-context/README.md).
