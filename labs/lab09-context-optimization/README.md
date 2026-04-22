# Lab 09: Context Optimization

## Goal

Learn to manage the LLM context window correctly:

1. **Count tokens the way the provider counts them** — `usage.PromptTokens` from the response, not your own estimates.
2. **Decide "time to compress" by a single threshold** (`0.80` of the model window), not a 3-4-step ladder of techniques.
3. **Compress history with a single `condense` pass**, preserving `tool_call ↔ tool_result` pairs.
4. **React to `ContextOverflowError`** with a reactive `condense` + exactly one retry.

This is the baseline. In [Lab 11](../lab11-memory-context/README.md), the second horizon is added on top — long-term cross-session memory (as tools).

What this lab does **not** teach (because it's harmful — see [Ch. 13: Common Errors](../../book/13-context-engineering/README.md#common-errors)):

- ❌ `len(text)/4` as the primary source of token counts.
- ❌ The adaptive ladder `prioritize → summarize → truncate`.
- ❌ Message scoring and history reordering.
- ❌ Truncate without checking `tool_call ↔ tool_result` pairs.
- ❌ Summary in the `system` role (kills prompt cache and mixes data with instructions).

## Theory

### One threshold, one reaction

```text
response N+1 ──┐
               ▼
       usage.PromptTokens  ← this is the source of truth
               │
               ▼
    > contextMax * 0.80?
               │
          ┌────┴────┐
          No        Yes
          │          │
       next       condense() ── once per Run
      request        │
                  next request
```

If after `condense` the provider still returns `ContextOverflowError`, do `condense + retry` exactly once. Another overflow → bubble the error up. Split the task.

### Where `estimateTokens(text)` is still useful

Your own estimate `≈ len(content)/3` (or something fancier via `tiktoken-go`) is useful **before sending a request** — so you don't fire off a request that's clearly over the window and waste a round-trip. But the primary source of token counts in production stays `resp.Usage.PromptTokens`.

Rule of thumb:

| Where | What we use |
|---|---|
| Decision "do we need to compress now" | `lastTokens = resp.Usage.PromptTokens` (from the previous response) |
| "Will this specific message even fit" (pre-send) | `estimateTokens(content)` |
| Metrics / billing | only `usage.*` from the provider |

### Why condense, not truncate

A naive `messages = messages[-N:]` or "let's keep only the last 5 + tool results" tears pairs apart:

```text
[..., assistant{tool_call:tc_42}, tool{tc_42, "ok"}, ...]
                                   ^
                          cut here — provider returns 400
```

`condense` preserves:

1. The **system** message in `messages[0]` — byte-for-byte, important for prompt cache.
2. The tail (last N messages), expanding the boundary to the left so tool pairs stay intact.
3. Replaces the middle of the history with **a single user message** "Context of previous work: …".

See more:

- [Chapter 13: Budget — one threshold, one reaction](../../book/13-context-engineering/README.md)
- [Chapter 12: Agent Memory Systems](../../book/12-agent-memory/README.md)

## Task

In `main.go`, implement a correct context-management loop for a long dialogue with a test tool call.

### Part 1: Pre-send token estimate

```go
func estimateTokens(text string) int { /* len(text)/3 as a baseline */ }
func estimateMessages(msgs []openai.ChatCompletionMessage) int
```

These functions are **only for a pre-send estimate**. Do not use them as the source of truth.

### Part 2: `Run` with `usage.PromptTokens` tracking

```go
type Run struct {
    messages     []openai.ChatCompletionMessage
    lastTokens   int  // resp.Usage.PromptTokens from the previous response
    contextMax   int  // model window, e.g. 128_000
    condenseDone bool
}
```

Requirements:

- `messages[0]` (system) is fixed at the start of the Run and **does not change**.
- After every provider response: `r.lastTokens = resp.Usage.PromptTokens`.
- Before every next request: if `lastTokens > contextMax * 0.80` → `condense` (once).

### Part 3: `condense` + `safeTail`

```go
func (r *Run) condense(ctx context.Context) error
func safeTail(msgs []openai.ChatCompletionMessage, n int) []openai.ChatCompletionMessage
```

`safeTail` returns **at least** N trailing messages, expanding the boundary to the left if the tail starts with a `tool` whose matching `assistant.tool_call` is not in the tail.

`condense`:

1. Fires at most once per Run.
2. `system := messages[0]`, `tail := safeTail(messages, 4)`, `head := messages[1 : len(messages)-len(tail)]`.
3. Gets a summary via a separate LLM request.
4. Assembles: `[system, user("Context of previous work:\n\n"+summary), tail...]`.

### Part 4: reactive `condense` on overflow

In the `Run.Step` loop:

```go
resp, err := r.callLLM(ctx)
if err != nil {
    if isContextOverflow(err) {
        if cerr := r.condense(ctx); cerr != nil { return "", cerr }
        resp, err = r.callLLM(ctx) // exactly one retry
        if err != nil { return "", fmt.Errorf("overflow even after condense: %w", err) }
    } else {
        return "", err
    }
}
```

### Part 5: "prediction vs reality" comparison

In the test scenario, after every response print:

```text
Step 12: estimated=1840, actual=1812 (Δ=+28, +1.5%), threshold@80%=2560
```

This exercise shows how accurate your `estimateMessages` is relative to real `usage.PromptTokens`. A good estimate stays within 10-20%; precision isn't the point — the point is to never send a clearly overflowing request.

### Test scenario

In `main.go`, run a long dialogue and intentionally lower `contextMax` mid-lab (e.g. to 4000) so you actually hit:

1. Crossing the `0.80` threshold → proactive condense.
2. (Optional) simulated `ContextOverflowError` → reactive condense.
3. `usage.PromptTokens` growing in the log and the compaction firing exactly once.

## What to verify by hand

1. After a proactive `condense`: `messages[0]` is unchanged (compare the string), and the history length has shrunk.
2. After `condense` there are no "orphan" tool messages: the first message of the tail is either `user` or an `assistant` with no pending `tool_call`.
3. `condense` fired exactly once — the `condenseDone = true` flag is set.
4. `estimateMessages` differs from `resp.Usage.PromptTokens` within reason (same order of magnitude).

## Completion criteria

**Done:**

- [x] The "time to compress" decision is made from `resp.Usage.PromptTokens`, not from your own estimates.
- [x] Single threshold `0.80` of `contextMax` + reactive `condense` on `ContextOverflowError`.
- [x] `condense` fires at most once per Run.
- [x] After `condense`: `messages[0]` is stable; `tool_call ↔ tool_result` pairs are intact; the summary lives as a `user` message, not `system`.
- [x] `estimateTokens` is used only for a pre-send estimate, not as the source of truth.
- [x] Logs show estimated/actual tokens and the moment condense fires.

**Not done:**

- [ ] The decision is made from `len(content)/3` without `usage.PromptTokens`.
- [ ] An adaptive `prioritize → summarize → truncate` ladder or message scoring.
- [ ] Truncate without `safeTail` (tool pairs are lost).
- [ ] Summary in the `system` role, or a `system + facts + summary + working` glue.
- [ ] Multiple condense calls in a row within one Run with no explicit limit.
- [ ] Hardcoded `maxContextTokens = 4000` (legacy from small-window models) — take the window from model metadata or configuration; modern models have 128k–1M.

---

**Next step:** [Lab 10: Planning and Workflows](../lab10-planning-workflows/README.md) — task decomposition and state preservation. You'll add long-term cross-session memory (as tools) in [Lab 11: Memory & Context](../lab11-memory-context/README.md).
