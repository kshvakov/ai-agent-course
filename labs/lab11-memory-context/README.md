# Lab 11: Memory and Context Management

## Goal

Implement two things without over-engineering:

1. **Condense** — compressing the history within a single agent run when the model's window limit approaches (see [Ch. 13](../../book/13-context-engineering/README.md)).
2. **Long-term memory** — agent notes that survive process restart. Available as regular tools (`memory.save` / `memory.recall`), not as a "context layer" (see [Ch. 12](../../book/12-agent-memory/README.md)).

We **do not do**:

- ❌ `LayeredContext` (Working / Summary / Facts layers stitched into one prompt).
- ❌ Automatic LLM-driven fact extraction from every message.
- ❌ A dynamic system prompt with "current state".
- ❌ Message-importance scoring and history reordering.

Why — in detail in [Ch. 13: Common Errors](../../book/13-context-engineering/README.md#common-errors). Short version: everything listed above kills prompt cache, breaks `tool_call ↔ tool_result` pairs, and is almost always an attempt to solve an imaginary problem.

## Theory

### Two memory horizons

| Horizon | Lifetime | Who writes | Where it lives |
|---|---|---|---|
| **In-Run (short-term)** | one agent run | Runtime, automatically | `[]Message` in memory |
| **Long-term** | survives restart | the agent calls a tool explicitly | file / DB / KV |

In-Run memory is **a linear immutable array of messages** (system → user → assistant → tool → assistant → …). When `usage.PromptTokens` approaches the window edge (e.g. 80%) or the provider returns `ContextOverflowError`, do **one `condense`**: take the middle of the history, ask the LLM to summarize it, place the result as a `user` message "context of previous work", keep the last N messages intact (importantly — without breaking `tool_call ↔ tool_result` pairs).

Long-term memory is **not auto-magic**. It's two tools the agent calls deliberately: "remember X", "recall about Y". Same as a new employee taking notes in their personal Notion. If the agent didn't call the tool, nothing was saved. And that's right: we don't want to accumulate junk like "user said hi at 14:32".

### Why condense, not truncate

A naive `messages[-N:]` cut breaks tool-call pairs: you can cut exactly between an assistant's `tool_call` and the runtime's `tool_result` — and the provider returns 400. On top of that, you throw away context with no trace.

Condense:

1. Preserves the system prompt in full.
2. Replaces the middle with a single `user` message: "Context of previous work: …".
3. Keeps the last N messages intact, **verifying** that no tool pairs are broken.

See more:

- [Chapter 12: Agent Memory Systems](../../book/12-agent-memory/README.md)
- [Chapter 13: Context Engineering](../../book/13-context-engineering/README.md)

## Task

In `main.go`, implement a simple agent with the capabilities below.

### Part 1: Linear memory + token tracking

The `Run` struct holds only the linear history of one run:

```go
type Run struct {
    messages    []openai.ChatCompletionMessage // linear log; system at index 0
    lastTokens  int                            // usage.PromptTokens of the last response
    contextMax  int                            // model window, e.g. 128_000
    condenseDone bool                          // limit: one condense per Run
}
```

Requirements:

- The `system` message is fixed in `messages[0]` and **does not change** during the Run.
- After every provider response, update `lastTokens = resp.Usage.PromptTokens` — this is the source of truth, not your own `len(content)/3` estimates.
- Before every next request, check: `lastTokens > contextMax * 0.80` → run `condense` (once per Run).

### Part 2: Implementing condense

```go
func (r *Run) condense(ctx context.Context, llm LLM) error {
    if r.condenseDone || len(r.messages) < 6 {
        return nil
    }
    system := r.messages[0]
    tail := r.messages[len(r.messages)-4:]    // last 4 in full
    head := r.messages[1 : len(r.messages)-4] // everything between

    // Make sure tail starts with an intact pair:
    // if the first message in tail is a tool result without its tool_call in tail,
    // shift the boundary to the left so the pair stays intact.
    tail = ensureToolPairs(r.messages, tail)
    head = r.messages[1 : len(r.messages)-len(tail)]

    summary, err := summarizeWithLLM(ctx, llm, head)
    if err != nil {
        return err
    }

    next := make([]openai.ChatCompletionMessage, 0, 2+len(tail))
    next = append(next, system)
    next = append(next, openai.ChatCompletionMessage{
        Role:    "user",
        Content: "Context of previous work:\n\n" + summary,
    })
    next = append(next, tail...)

    r.messages = next
    r.condenseDone = true
    return nil
}
```

Prompt for `summarizeWithLLM`:

```text
Compress the transcript into a brief handoff for the agent's next step.
Preserve:
1. What was assigned (the original task).
2. Decisions already made, and why.
3. Which files / resources have been read and what's relevant in them.
4. What still needs to be done.

Don't paraphrase pleasantries; drop chatter.
```

Reactive branch: if the provider returns `ContextOverflowError`, run `condense` and **retry** the request once. If it overflows again — fail with a clear error (the agent was given too much in one step; decompose the task).

### Part 3: Long-term memory as a tool

Set up storage — a JSON file or SQLite. Minimal API:

```go
type Store interface {
    Save(ctx context.Context, key, value string) error
    Recall(ctx context.Context, query string) ([]Entry, error)
    Delete(ctx context.Context, key string) error
}

type Entry struct {
    Key       string
    Value     string
    CreatedAt time.Time
}
```

Tool registration with the agent:

- **`memory.save`** — parameters `key` (string), `value` (string). Returns "ok" or an error.
- **`memory.recall`** — parameter `query` (string). Returns up to 5 records. Substring search over `key+value` is enough for the lab; embeddings are overkill.
- **`memory.delete`** — parameter `key`. Useful for self-correction.

No automatic fact-saving by the LLM. The agent decides what to put in memory, through `memory.save`. This is both simpler and more honest — you can see in the logs what the agent decided to remember and why.

In the system prompt, give the agent a **short** instruction (1-2 sentences):

```text
You have memory.save / memory.recall / memory.delete tools for notes
that survive restart. Use them for stable facts about the user
and project. Don't store transient statuses.
```

Don't stitch the current memory contents into the system prompt — otherwise every save invalidates prompt cache. Memory is loaded via `memory.recall` when the agent asks for it itself.

### Part 4: Minimal CLI

`go run ./main.go --memory ./memory.json`

REPL: the user types messages, the agent replies, can call `memory.*` and other tools. After exit and re-launch, `memory.recall` should find old records.

## What to verify by hand

1. Run a long dialogue so that `usage.PromptTokens` crosses 80% — confirm that `condense` fired exactly once and `system[0]` stayed byte-for-byte the same.
2. Simulate `ContextOverflowError` (e.g. artificially lower `contextMax`) — confirm that the reactive condense fires and the request is retried.
3. Restart the process. Ask the agent about a fact you saved in the previous session — it should call `memory.recall` itself and answer.
4. Verify that the `system` prompt contains no current time, no "current state", and no memory contents. Only role, rules, and the tool descriptions.

## Common errors in this lab

- **Counting tokens via `len(content)/3`.** Use `resp.Usage.PromptTokens` — that's what the provider actually charged. Your own counter is good only for a pre-send estimate.
- **Doing `messages = messages[len(messages)-N:]`.** You'll break tool-call pairs and get a 400. Use `condense` or a careful window with pair checks.
- **Putting memory contents into the system prompt.** Every save invalidates prompt cache — cost and latency go up.
- **Doing a second, third, fourth condense in a row.** If you hit the window again after the first condense, the problem isn't memory — the task isn't decomposable. Refuse explicitly.
- **Auto-extracting "facts" from every message via the LLM.** You'll end up with noise like "user said thanks". Saving is a deliberate action by the agent through a tool.

## Completion criteria

**Done:**

- [x] The `system` message in `messages[0]` is stable for the whole Run (byte-for-byte).
- [x] The "time to compress" decision is made from `usage.PromptTokens`, not from your own estimates.
- [x] Trigger condition: `lastTokens > contextMax * 0.80` **or** reactively on `ContextOverflowError`.
- [x] `condense` runs at most once per Run.
- [x] After `condense`, the following are preserved: system, the summary as a user message, the last N messages; `tool_call ↔ tool_result` pairs are not broken.
- [x] Long-term memory is available to the agent as 3 tools (`save` / `recall` / `delete`), not as part of the system prompt.
- [x] Memory survives process restart.

**Not done:**

- [ ] `LayeredContext` or any dynamic stitching of the system prompt.
- [ ] Auto-extraction of facts from every message.
- [ ] Truncate without checking tool pairs.
- [ ] Memory contents glued into the system prompt on every request.
- [ ] Multiple condense calls in a row in one Run with no explicit limit.
- [ ] Window accounting via `len(content)/3` instead of `usage.PromptTokens`.

---

**Next step:** after completing Lab 11, move on to [Lab 12: Tool Server Protocol](../lab12-tool-server/README.md).
