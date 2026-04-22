# 13. Context Engineering

## Why This Chapter?

The context window is finite. On a long Run, the agent will eventually hit the limit. This chapter is about how to live with that.

The main idea in one line: **simple memory — ran out, compress**. Everything else is a stable system prompt, careful prompts for tools, and honest token numbers.

If you're coming from textbooks where "context engineering" means layers with fixed ratios, message-importance scoring, a `BudgetTracker` with warnings injected into the system prompt and three compression strategies to choose from — exhale. None of that is needed in a real agent. Below I explain why.

### Real-World Case Study

**Situation:** The agent is already on iteration 25. In the last response the provider reported `usage.prompt_tokens = 102_400` (model is 128K). The next tool result will add another couple of thousand tokens. After that — overflow.

**Problem:**
- Do nothing — the next request will fail.
- Drop "less important" messages — you'll tear apart `tool_call ↔ tool_result` pairs, and the provider will return a validation error.
- Rebuild the context "by layered ratios" — you'll bust the prompt cache, and every following request will become 5-10x more expensive.

**Solution:** once per Run, compress the old part of history through an LLM, leave the last N messages untouched, keep going. That's **condense**. One trigger threshold, one limit, no head-in-the-sand moves.

## The Main Idea (from experience)

With experience you realize: a real production agent consists of four simple things.

1. **A simple loop.** `while True: resp = llm.Chat(messages); if there are tool_calls — execute and put the results into messages; otherwise — return the response`. See [Chapter 04: Autonomy and Loops](../04-autonomy-and-loops/README.md).
2. **Simple memory.** One linear `[]Message` array. Grows strictly from old to new. Nothing is reordered, nothing is deleted from the middle. See [Chapter 12: Memory](../12-agent-memory/README.md).
3. **Compression on overflow.** One trigger condition, one action (see below).
4. **Prompt engineering for tools.** A good `description`, explicit parameter requirements, clear error messages. This pays off far more than any "clever context assembly".

Anything more complex usually cures the wrong disease. If you feel the urge to add "prioritization", "layers with ratios" or a "dynamic system prompt" — that's almost always a signal that the task has a bad architecture (it should be split into sub-Runs or knowledge should be moved into external retrieval), not that the formula is missing weights.

## Theory in Simple Terms

### What context is made of

The context for a single request is the `messages[]` you send to the provider. It holds four layers — but **as a mental model**, not as four separate `system` messages in the code.

| Layer | What goes in | Where it lives physically |
|-------|--------------|----------------------------|
| **System** | Role, style, skills, rules (stable within a Run) | `messages[0]`, type `system` |
| **History** | All user/assistant/tool messages | `messages[1..N-1]`, in order of appearance |
| **Facts from long-term memory** | Relevant facts selected at the start of the Run | In the **first user message** of the Run, not in system |
| **Live state** | Task progress, files read, plan | In tool results, in Notes, in the last user message of the frame |

The main rule: **the prefix is stable, changes go to the tail**. Then the provider's prompt cache works and most requests cost pennies. If the prefix mutates — the cache breaks, and every request is billed at the full rate.

### Anchoring bias: careful with facts

If **user preferences** ("the user thinks the problem is in the DB") or **hypotheses** sneak into "facts", the model treats them as the truth and bends its answer toward them — even when the data says otherwise.

The fix is simple: call things by their names in the text itself. Not "fact", but "user hypothesis" or "assumption, needs verification". No clever filter on `Type == "hypothesis"` in the code — the model understands English prefixes perfectly well.

```text
[fact] Server web-01 is not responding to ping since 14:32 UTC.
[user hypothesis] The user suspects the problem is in the DB (not confirmed).
[constraint] Do not touch the prod cluster until 18:00 UTC.
```

### Context operations

In real life there are only two:

1. **Append** — added a new message to the end of `messages[]`.
2. **Condense** — once per Run, replaced the older part of history with a summary.

That's it. No Select / Extract / Layer / Reorder.

## How It Works (Step by Step)

### Step 1: Linear memory

```go
type Memory struct {
    messages []llm.Message
}

func (m *Memory) Append(msg llm.Message) {
    m.messages = append(m.messages, msg)
}

func (m *Memory) Snapshot() []llm.Message {
    out := make([]llm.Message, len(m.messages))
    copy(out, m.messages)
    return out
}

func (m *Memory) Reset(msgs []llm.Message) {
    m.messages = msgs
}
```

That's it. `Reset` is only needed for `condense` — history is never rewritten anywhere else.

### Step 2: One threshold, one action

```go
type Run struct {
    mem           *Memory
    contextWindow int     // model limit, e.g. 128_000
    condenseAt    float64 // 0.80 — threshold for triggering condense
    condenseDone  bool    // limit: condense at most once per Run
    lastTokens    int     // usage.PromptTokens from the last provider response
}

// Called before each next Chat request.
func (r *Run) BeforeNextRequest(ctx context.Context) error {
    used := float64(r.lastTokens) / float64(r.contextWindow)
    if used >= r.condenseAt && !r.condenseDone {
        if err := r.condense(ctx); err != nil {
            return err
        }
        r.condenseDone = true
    }
    return nil
}
```

### Step 3: Reacting to overflow from the provider

Sometimes the estimate based on `lastTokens` is too late: the next tool result turns out heavier than expected and the provider returns a `ContextOverflowError`. We do the same thing, just reactively.

```go
resp, err := client.Chat(ctx, r.mem.Snapshot())
if isContextOverflow(err) {
    if r.condenseDone {
        return r.wrapUp(ctx) // condense already happened — graceful save
    }
    if err := r.condense(ctx); err != nil {
        return err
    }
    r.condenseDone = true
    resp, err = client.Chat(ctx, r.mem.Snapshot()) // retry
}
```

There are only two triggers: **threshold** (proactive) and **overflow** (reactive). One action: `condense`. One limit: `once per Run`. On a repeated overflow — `wrapUp` (see below), not another condense.

### Step 4: Condense — what's inside

```go
func (r *Run) condense(ctx context.Context) error {
    msgs := r.mem.Snapshot()
    if len(msgs) < 6 {
        return nil // nothing to compress
    }

    head := msgs[1 : len(msgs)-4] // everything except system and the last 4
    tail := msgs[len(msgs)-4:]
    system := msgs[0]

    summary, err := r.summarizeWithLLM(ctx, head)
    if err != nil {
        return err
    }

    next := make([]llm.Message, 0, 2+len(tail))
    next = append(next, system)
    next = append(next, llm.Message{
        Role:    "user",
        Content: "Context of previous work:\n\n" + summary,
    })
    next = append(next, tail...)
    r.mem.Reset(next)
    return nil
}
```

Three key points:

1. **Full replacement of history**, not "trimming the middle". Cleaner and more predictable.
2. **The summary goes in as `user`**, not as `assistant`. Then the model treats it as "here's the context, keep going", not as its own reply that can be contradicted.
3. **The tail (the last 3-5 messages) is untouched.** This is insurance against something important being lost in the summary when the model needs it right now.

### Step 5: Wrap-up on repeated overflow

If condense has already happened — don't compress again (re-compressing catastrophically loses details). Instead: save progress, hand the user a partial result, exit the Run.

```go
func (r *Run) wrapUp(ctx context.Context) error {
    snapshot := r.mem.Snapshot()
    if err := r.checkpoint.Save(snapshot); err != nil {
        return err
    }
    return ErrRunWrappedUp // the top level catches this and shows it to the user
}
```

A checkpoint is needed so the user can press "Continue" and resume with a different model / different context. See [Chapter 11: State Management](../11-state-management/README.md).

## Counting Tokens Correctly

A hierarchy of sources — from best to worst:

1. **`usage.PromptTokens` from the provider response.** The exact number you were billed on. Take it from the last model response and use it to decide "is it time to condense". This is the **primary source** — don't invent your own counter if you have this one.
2. **The model's tokenizer** (e.g. `tiktoken` for OpenAI, `anthropic.count_tokens` for Anthropic). Needed in one case: you want to estimate the weight of a not-yet-sent message to decide "send it or compress first".
3. **Word/character approximation** — last resort. Fine for a rough estimate when (1) and (2) aren't available.

For more on why char-based estimation is dangerous, see [Chapter 12, Error 7](../12-agent-memory/README.md#error-7-estimating-tokens-via-char3).

```go
// The most common pattern: save the token count after every Chat response.
resp, err := client.Chat(ctx, msgs)
if err == nil {
    r.lastTokens = resp.Usage.PromptTokens
}
```

No `WordBasedCounter` with `TokensPerWord = 2.0` for Russian. No `len(content)/3`. The provider already counted — use what's there.

## Model Limits

**Don't hardcode a model dictionary in code.** The model zoo changes faster than code is updated, and a stale dictionary will give the wrong answer for a fresh model.

Minimal structure and place to store it:

```go
type Model struct {
    ID            string
    ContextWindow int // maximum input + output
    MaxOutput     int // usually less than ContextWindow
}

func SafeBudget(m Model, reserveOutput int) int {
    if reserveOutput == 0 {
        reserveOutput = m.MaxOutput
    }
    return m.ContextWindow - reserveOutput
}
```

Where to get `Model` from:
- from your LLM SDK's model catalog (preferred);
- from the service config (if the SDK doesn't provide a catalog);
- from environment variables for private deployments.

The main thing — **in one place**, don't smear `_ = 128000` across 15 files.

## When to Use truncate

`truncate` (drop extra messages from the middle, keep head+tail) is a separate tool from `condense`. It's appropriate in three cases:

- **Single-shot requests** without a multi-step loop (the agent loop has nothing to do with it).
- **Providers without prompt cache** — nothing to save on, just have to fit under the limit.
- **Emergency fallback**, when `condense` has already spent its 1/Run quota and the context is still overflowing — easier to trim than to fail (but easier still — do `wrapUp` and save progress).

In an agent loop with prompt cache, `truncate` is **not needed** — it busts the cache on every iteration because the middle changes. If you really want it — write it carefully, preserving `tool_call ↔ tool_result` pairs intact (a torn pair is a `400 Bad Request` from the provider).

## Condensation Prompt

The quality of `condense` is 80% determined by the prompt. A bare "summarize" gives a useless retelling. A good prompt works on the principle of **"handing off a task to a teammate who is about to pick it up"**.

```text
You are summarizing a multi-turn agent run for a teammate
who will continue the work. Be specific and operational.

Required sections:
1. Goal — what the user wants
2. Key Findings — important discoveries (with file:line if relevant)
3. Resources Examined — files read, commands run
4. Decisions Made — choices and their rationale
5. Work Completed — what's done
6. Pending Items — what's left
7. Current State — where we stopped
8. Next Steps — what to do next

Be SPECIFIC:
- "Add JWT middleware to internal/auth/middleware.go:45" — GOOD
- "implement authentication" — BAD
- "Found memory leak in worker pool at pkg/pool/pool.go:120" — GOOD
- "found a bug" — BAD

Hard limit: {maxWords} words.
```

What matters:
- **Clear sections.** Otherwise the model writes an essay, and it's impossible to pull "what's left to do" out of the summary.
- **Demand specifics with examples.** Without them you get water like "authentication has been implemented".
- **A hard word limit.** If you don't specify one, the summary will bloat to nearly the size of the original.

## System Prompt: stability beats savings

The system prompt consumes tokens on **every iteration** of the loop. It's tempting to "optimize": show the long instructions on the first iteration and drop some on the rest. On paper — savings. In practice — almost always more expensive.

### Why an "adaptive system prompt" loses

Modern providers cache the request **prefix**. A cached token is much cheaper than a regular one:

| Provider | Cache hit discount |
|----------|--------------------|
| OpenAI | ~50% off input price |
| Anthropic | ~90% off input price |
| Z.AI / GLM | up to ~80% (model-dependent) |

If the system prompt changes between iterations — the cache is busted **on everything after the change point**, and you pay full price not only for the changed sections but for the entire message history too.

Simple arithmetic for a typical Run (system 25K, history by iteration 5 ~30K, 8 iterations total):

| Strategy | Input cost per Run | Cache hit ratio |
|----------|--------------------|-----------------|
| Stable system | ~$0.04 (Anthropic) | ~85% |
| "Adaptive": removed 1500 tokens on iterations 2-8 | ~$0.18 (Anthropic) | ~5% |

Saving 1500 input tokens turns into losing cache hits on 25-30K of prefix tokens on each of the next 7 iterations. **4-5x more expensive.**

### What to do instead

**1. The system prompt is stable within a Run.** All dynamic data — in tool results, in Notes, or in the first user message of the frame. Details — [Chapter 12: Live state without mutating the system prompt](../12-agent-memory/README.md#live-state-without-mutating-the-system-prompt).

**2. Fix stable inclusions once.** Date, working directory, mode (`debug` / `prod`) — write them into the system prompt at Run start and never touch them again.

**3. Want a "warning" on overflow — put it in user, not system.** If you really want to hint to the model "start wrapping up", add a short sentence to the **last user message** of the next frame, don't mutate the system. The cache is unharmed.

**4. Conditional "knowledge" — better via a tool than via a conditional prompt.** If you want to add an SOP at the analysis stage and hide it at the action stage, wrap the SOP as a `tool` (`get_sop("incident_diagnosis")`) and let the model call it itself. The tool result is added to the tail — cache is not affected.

### The 20-25% rule

The system prompt shouldn't exceed ~20-25% of the context window. For 128K — ~25-32K tokens. If it's growing — trim the **base version** at Run start, not "between iterations". Never put live state in it — that's [Error 6 from Chapter 12](../12-agent-memory/README.md#error-6-live-state-in-the-system-prompt).

## Common Errors

### Error 1: Dynamic system prompt

**Symptom:** Run cost is 4-5x higher than expected; cache hit ratio in provider logs is around 5%.

**Cause:** The system prompt is rebuilt on every iteration (inserting `current_time`, the current plan, files read, per-iteration dynamic sections).

**Solution:**

```go
// BAD: cache miss on every iteration
sys := fmt.Sprintf(`You are an agent. Current time: %s. Files read: %v. Iteration: %d`,
    time.Now(), filesRead, iteration)

// GOOD: stable prefix
sys := `You are an agent. Use tools to read files when needed.`
// And current_time / filesRead / iteration will be shown to the agent by the next user message or tool result.
```

### Error 2: Live state in the system prompt

**Symptom:** Same 4-5x cost overrun + on every new tool call the cache hit drops to zero.

**Cause:** Task progress (`Read files: a.go, b.go, c.go`) is updated in the system prompt after each tool call.

**Solution:** Live state lives in tool results / in the last user message / in Notes — that is, **in the tail of history**. Don't touch the prefix. Detailed breakdown — [Chapter 12: Live state without mutating the system prompt](../12-agent-memory/README.md#live-state-without-mutating-the-system-prompt).

### Error 3: Importance scoring and reordering

**Symptom:** The provider returns `400 Bad Request: tool_use without matching tool_result` (or the mirror error).

**Cause:** A "smart" algorithm scored some `tool` messages as "unimportant" and dropped them, leaving `assistant` with dangling `tool_calls`.

**Solution:** Don't do message scoring or reordering. Use **condense** — it replaces the entire old part with a text summary, and the problem of tearing `tool_call ↔ tool_result` pairs disappears by construction.

### Error 4: Re-condensing an already condensed context

**Symptom:** After 2-3 condenses the agent forgets the task goal, confuses files, repeats actions.

**Cause:** Condense is called every time on overflow, without a limit.

**Solution:**

```go
// BAD: infinite condensation
for {
    resp, err := llm.Chat(ctx, messages)
    if isContextOverflow(err) {
        messages = condense(ctx, messages)
        continue
    }
}

// GOOD: at most once per Run, then wrapUp
condenseDone := false
for {
    resp, err := llm.Chat(ctx, messages)
    if isContextOverflow(err) {
        if condenseDone {
            return wrapUpAndSaveProgress()
        }
        messages = condense(ctx, messages)
        condenseDone = true
        continue
    }
}
```

### Error 5: Counting tokens via `len(content)/3`

**Symptom:** Condense doesn't fire until the actual overflow, or vice versa — it fires when the context is only 30% full.

**Cause:** You're using a char-based estimate ("3 characters = 1 token"). For Russian the real ratio is 1.5-2x that estimate; for code — 0.5x. Error ±50%.

**Solution:** Take `usage.PromptTokens` from the provider response to the previous request. It's a **fact**, computed by the provider itself, and it comes for free. More — [Chapter 12, Error 7](../12-agent-memory/README.md#error-7-estimating-tokens-via-char3).

### Error 6: Hardcoded model dictionary in code

**Symptom:** When the model is swapped, the agent behaves "somehow off" — hits overflow earlier than it should, or starts `condense` too late.

**Cause:** A dictionary like `var ModelLimits = map[string]int{"gpt-4o": 128000, ...}` wasn't updated for the new model, and the new one got the default `4096`.

**Solution:** Take the limit from your SDK's model catalog (see the [Model Limits](#model-limits) section above). In one place, don't spread it across the code.

## Mini-Exercises

### Exercise 1: BeforeNextRequest

Implement a function that decides whether to run `condense` **before** the next request.

```go
type Run struct {
    contextWindow int
    lastTokens    int
    condenseDone  bool
}

func (r *Run) ShouldCondense() bool {
    // your code
}
```

**Expected result:**
- Returns `true` if `lastTokens / contextWindow >= 0.80` and condense hasn't happened yet.
- Returns `false` in all other cases.
- No message counters, no layers.

### Exercise 2: Reactive condense on overflow

Finish the loop so that on `ContextOverflowError` it runs condense once and retries the request, and on a second overflow it exits with `wrapUp`.

```go
for {
    resp, err := client.Chat(ctx, mem.Snapshot())
    // your code
}
```

**Expected result:**
- Condense is called at most once per Run.
- On a repeated overflow — `wrapUp`, not another condense.
- Non-overflow errors — propagated up, not "healed" with a condense.

### Exercise 3: Moving facts from system to user

Given: code where facts from long-term memory were added as a separate `system` message and rewritten on every iteration. Task: move them so the facts go into the **first user message of the Run** and are not changed after that (if the set of facts changes within a Run — that's a signal you need a `recall` tool, not context mutation).

**Expected result:**
- One `system` message per Run in the code, immutable.
- Facts are added exactly to the first `user` message of the Run.
- Cache hit ratio in provider logs grows.

## Completion Criteria / Checklist

**Completed:**
- [x] You understand the 4 context layers as a **mental model**, not as 4 separate `system` messages
- [x] You take `usage.PromptTokens` from the provider as the primary source for the token count
- [x] Model limits live in one place (catalog/config), not scattered across the code
- [x] Compression — one trigger (threshold or overflow), one limit (once per Run)
- [x] Condense — full history replacement, summary as `user`, the tail of the last 3-5 messages untouched
- [x] The system prompt is stable within a Run (live state — outside `system`, see [Chapter 12](../12-agent-memory/README.md#live-state-without-mutating-the-system-prompt))
- [x] You understand the trade-off: an "adaptive prompt" almost always loses to prompt cache on cost
- [x] You know that on a repeated overflow you do `wrapUp`, not another condense

**Not completed:**
- [ ] Context grows unbounded — no condense by threshold
- [ ] Condense is called again on an already condensed context (details are lost exponentially)
- [ ] A dynamic system prompt changes on every iteration (cache miss)
- [ ] Live state (progress, files read, current date on every iteration) sits in `system`
- [ ] Importance scoring and message reordering — tears `tool_call ↔ tool_result` chains
- [ ] Counting tokens via `len(content)/3` instead of `usage.PromptTokens`
- [ ] A hardcoded model dictionary in code — goes stale faster than the code is updated
- [ ] "Layers with ratios" (`SystemRatio: 0.10, FactsRatio: 0.10 ...`) — it's the illusion of control, not control

## For the Curious

> This section is for those who still want to go deeper. You can skip it.

### Why one threshold, not two

In the literature you often see a cascade "warning at 75% → condense at 80% → wrap-up at 90%". In practice, with the condition `usage.PromptTokens >= contextWindow * 0.80`, one threshold covers all cases, because:

- By the time you reach 80%, you already have **exact** information about consumption (from the last `usage`). No uncertainty that would require a separate "warning" level.
- A warning via mutation of the system prompt is an anti-pattern (cache miss). Via mutation of `assistant` — also a bad idea (the model sees someone else's "voice"). That leaves inserting into `user` — and then it's no longer a separate level, just "added a line to the next user message".
- Wrap-up isn't a level of compression, it's an **outcome**. It doesn't shrink context, it saves state and finishes the Run. Logically it stands **after** condense, not in parallel with it.

### What about block memory and recall

If you have an agent with REPL-like interaction (one complex request → one big result → next request), **block memory** with a `recall` tool can be useful: history is cataloged into "blocks", summaries go into the active context, and the full content of a block is loaded on demand by the model. That's no longer the base case; the breakdown is in [Chapter 12: Block Memory](../12-agent-memory/README.md#block-memory).

### When the context really doesn't fit

If the task is so big that 128K isn't enough even after condense — that's a signal not to "optimize compression" but to **split the task into sub-Runs** (see [Chapter 09: Architecture](../09-agent-architecture/README.md)). Each sub-Run runs in its own context, the result is saved to long-term memory or a file, and the top level then collects the results. That's how every production agent capable of working with large codebases does it.

## Connection with Other Chapters

- **[Chapter 04: Autonomy and Loops](../04-autonomy-and-loops/README.md)** — the simple loop with `BeforeNextRequest` and overflow reaction built in
- **[Chapter 11: State Management](../11-state-management/README.md)** — `wrapUp` saves state through a checkpoint
- **[Chapter 12: Agent Memory Systems](../12-agent-memory/README.md)** — linear memory, facts in user, live state without mutating system, recall for block memory
- **[Chapter 20: Cost & Latency Engineering](../20-cost-latency-engineering/README.md)** — prefix stability, prompt cache, cost of condense

**Important:** Context Engineering is about **assembling the context for a single request**. Storing knowledge between sessions is described in Memory; persistent data (schemas, policies) — in State Management; external search — in RAG.

## What's Next?

After mastering context engineering, move on to:
- **[14. Ecosystem and Frameworks](../14-ecosystem-and-frameworks/README.md)** — an overview of popular agent frameworks and where they help vs. get in the way.

---

**Navigation:** [← Chapter 12: Memory](../12-agent-memory/README.md) | [Table of Contents](../README.md) | [Chapter 14: Ecosystem →](../14-ecosystem-and-frameworks/README.md)
