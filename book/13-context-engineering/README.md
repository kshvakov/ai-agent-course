# 13. Context Engineering

## Why does this matter?

The context window is finite. In a long run, the agent will sooner or later hit the limit. This chapter is about how to live with that.

The main idea in one line: **when simple memory runs out, condense it**. Everything else — a stable system prompt, careful tool prompts, and genuine token counts.

If you came here from textbooks where "context engineering" means layers with fixed proportions, message importance scoring, a BudgetTracker with warnings injected into the system prompt, and three compression strategies to choose from — exhale. A real agent needs none of that. Below I'll explain why.

### A real-world case

**Situation:** The agent is already 25 iterations in. On the last response the provider reported: `usage.prompt_tokens = 102_400` (the model is 128K). The next tool result will add a couple thousand more tokens. Next comes overflow.

**The problem:**
- Do nothing — the next request fails.
- Drop the "less important" messages — you break the `tool_call ↔ tool_result` pair, and the provider returns a validation error.
- Rebuild the context "in layers with proportions" — you reset the prompt cache, and every subsequent request becomes 5-10x more expensive.

**The solution:** once per run, condense the old part of the history via an LLM, leave the last N messages untouched, and keep going. That is **condense**. One threshold to trigger it, one limit, no ostrich tactics.

## The Main Idea (With Experience)

With experience you realize: a real production agent consists of four simple things.

1. **A simple loop.** `while True: response = llm.Chat(messages); if there are tool_calls — execute them and put the results into messages; otherwise — return the response`. Chapter [04. Autonomy and Loops](../04-autonomy-and-loops/README.md).
2. **Simple memory.** One linear `[]Message` array. It grows strictly from old to new. Nothing is reordered, nothing is deleted in the middle. Chapter [12. Agent Memory](../12-agent-memory/README.md).
3. **Condense on overflow.** One trigger condition, one action (see below).
4. **Prompt engineering for tools.** Good `description`s, explicit parameter requirements, clear error messages. This buys you far more than any "clever context assembly."

Anything more complex usually treats the wrong disease. If you feel the urge to add "prioritization," "layers with proportions," or a "dynamic system prompt" — almost always that's a signal the task has a bad architecture (it needs splitting into sub-runs, or moving knowledge into external retrieval), not that the formula needs more weights.

## The theory in plain words

### What the context consists of

The context for a single request is the `messages[]` you send to the provider. Four layers live in it — but **as a mental model**, not as four separate `system` messages in code.

| Layer | What goes there | Where physically |
|------|------------------|---------------|
| **System** | Role, style, skills, rules (stable within a run) | `messages[0]`, type `system` |
| **History** | All user/assistant/tool messages | `messages[1..N-1]`, in order of appearance |
| **Facts from long-term memory** | Relevant facts, selected at run start | In the **first user message** of the run, not in system |
| **Live state** | Task progress, files read, plan | In tool results, in Notes, in the frame's last user message |

The key rule: **the prefix is stable, changes go to the tail**. That way the provider's prompt cache works, and most requests cost pennies. If the prefix mutates, the cache breaks and every request is paid in full.

### Anchoring bias: be careful with "facts"

If **user preferences** ("the user thinks the problem is in the DB") or **hypotheses** end up in the "facts," the model takes them as truth and bends its answer around them — even when the data says otherwise.

The cure is simple: call things by their proper names in the text itself. Not "fact," but "user hypothesis" or "assumption, needs verification." No clever `Type == "hypothesis"` filter in code — the model understands a Russian/English prefix just fine.

```text
[fact] Server web-01 has not responded to ping since 14:32 UTC.
[user hypothesis] The user believes the problem is in the DB (unconfirmed).
[constraint] Do not touch the prod cluster until 18:00 UTC.
```

### Context operations

In real life there are only two:

1. **Append** — add a new message to the end of `messages[]`.
2. **Condense** — once per run, replace the old part of the history with a summary.

That's it. No Select / Extract / Layer / Reorder.

## How it works (step by step)

### Step 1: Linear history

```go
type History struct {
    mu       sync.Mutex
    messages []llm.Message
}

func (h *History) Append(msg llm.Message) {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.messages = append(h.messages, msg)
}

// Messages returns a copy of the history — this is what goes into the LLM request.
func (h *History) Messages() []llm.Message {
    h.mu.Lock()
    defer h.mu.Unlock()
    out := make([]llm.Message, len(h.messages))
    copy(out, h.messages)
    return out
}

// Replace is the only way to modify what has already been written.
// It is needed only for condense — nowhere else do we rewrite the history.
func (h *History) Replace(msgs []llm.Message) {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.messages = append(h.messages[:0], msgs...)
}

// Snapshot/Restore — checkpointing, audit, rolling back a failed condense.
func (h *History) Snapshot() []llm.Message  { return h.Messages() }
func (h *History) Restore(snap []llm.Message) { h.Replace(snap) }
```

That's all. `Replace` is needed only for condense — nowhere else do we rewrite the history. Sanitization (empty assistant messages, orphan tool results, the unbreakability of `tool_call ↔ tool_result` pairs) is built into the history itself — the same `History` as in [Chapter 12](../12-agent-memory/README.md).

### Step 2: One threshold, one action

```go
type Run struct {
    mem           *History
    contextWindow int     // the model's limit, e.g. 128_000
    condenseAt    float64 // one configurable Threshold ∈ (0,1); typical value ~0.85
    condenseDone  bool    // limit: condense at most once per run
    lastTokens    int     // usage.PromptTokens from the provider's last response
}

// Called before each subsequent Chat request.
func (r *Run) BeforeNextRequest(ctx context.Context) error {
    used := float64(r.lastTokens) / float64(r.contextWindow)
    if used >= r.condenseAt && !r.condenseDone {
        if err := r.condense(ctx); err != nil {
            // CondenseErrorEvent: a loop event, not a crash — the run continues
            // with the uncondensed history. Step 3 handles it reactively.
            r.condenseDone = true
            return nil
        }
        r.condenseDone = true
    }
    return nil
}
```

### Step 3: Reacting to overflow from the provider

Sometimes the estimate based on `lastTokens` isn't fast enough: the next tool result turns out fatter than expected, and the provider returns a `ContextOverflowError`. We do the same thing, but reactively.

```go
resp, err := client.Complete(ctx, Request{Messages: r.mem.Messages()})
if isContextOverflow(err) {
    if r.condenseDone {
        return r.wrapUp(ctx) // condense already happened — graceful save
    }
    if err := r.condense(ctx); err != nil {
        // Condense failure — a loop event (CondenseErrorEvent), not a run crash:
        // the run continues with the uncondensed history. The attempt is spent —
        // there will be no further condenses.
        r.condenseDone = true
        return nil // the next iteration proceeds with the uncondensed history
    }
    r.condenseDone = true
    resp, err = client.Complete(ctx, Request{Messages: r.mem.Messages()}) // retry
}
```

There are just two triggers: **threshold** (proactive) and **overflow** (reactive). One action: `condense`. One limit: `once per run`. On a second overflow after a successful condense — `wrapUp` (see below), not another condense. If condense fails, the run does not crash: it continues with the uncondensed history; when the next iteration hits overflow again, condense is already spent — the only path left is `wrapUp`. No bisection ("maybe just compress a quarter?").

### Step 4: Condense — what's inside

```go
func (r *Run) condense(ctx context.Context) error {
    snap := r.mem.Snapshot() // the original — for audit and rollback
    if len(snap) < 6 {
        return nil // nothing to condense
    }

    head := snap[1 : len(snap)-4] // everything except system and the last 4
    tail := snap[len(snap)-4:]
    system := snap[0]

    // The summarization request carries THE SAME cacheable prefix (system + tools),
    // so the head is read from the warm cache rather than paid in full.
    summary, err := r.summarizeWithLLM(ctx, head)
    if err != nil {
        return err // the original is untouched — see the handling in Step 3
    }

    next := make([]llm.Message, 0, 2+len(tail))
    next = append(next, system)
    next = append(next, llm.Message{
        Role:    "user",
        Content: "Context of previous work:\n\n" + summary,
    })
    next = append(next, tail...)
    r.mem.Replace(next)
    return nil
}
```

Four key points:

1. **Full replacement of the history**, not "cutting out the middle." Cleaner and more predictable.
2. **The summary goes in as a `user` message**, not an `assistant` one. The model then treats it as "here's the context, continue" rather than as its own answer that it may contradict.
3. **The tail (the last 3-5 messages) is untouched.** This is insurance against something important and immediately needed by the model right now getting lost in the summary.
4. **Summarization reads the history from the warm cache.** The LLM call for the summary carries the same system + tools prefix as the regular iterations — the head is not paid at full price, so condense costs noticeably less than it seems.

> **How the real SDK does it.** The loop's memory is a linear `History` (`New`/`Append`/`Replace`/`Snapshot`/`Restore`). The config has a single `Threshold ∈ (0,1)`, typical value ~0.85; condense runs at most once per run. A condense failure is emitted as an `EvCondenseErr` event: the run continues with the uncondensed history, and the "condense ≤ 1/run" invariant is not violated. Cache metrics (CacheRead / CacheCreation) are parsed from the response's `Usage`.

### Step 5: Wrap-up on a second overflow

If condense has already happened, we don't condense again (repeated condensing loses details catastrophically). Instead: save the progress, give the user a partial result, and exit the run. The exception is a condense that failed with an error (`CondenseErrorEvent`): the attempt was wasted, the run continues with the uncondensed history; on the next overflow — `wrapUp`, same as after a successful condense.

```go
func (r *Run) wrapUp(ctx context.Context) error {
    snapshot := r.mem.Snapshot()
    if err := r.checkpoint.Save(snapshot); err != nil {
        return err
    }
    return ErrRunWrappedUp // the top level catches this and shows it to the user
}
```

The checkpoint exists so the user can hit "Continue" and pick up with a different model / different context. See [Chapter 11: State Management](../11-state-management/README.md).

## Counting Tokens Correctly

The hierarchy of sources, from best to worst:

1. **`usage.PromptTokens` from the provider's response.** The exact number you were billed for. Take it from the model's last response and use it to decide "is it time to condense." This is the **primary source** — don't invent your own counter when this one exists. The PromptTokens rule was introduced back in [Chapter 01: LLM Fundamentals](../01-llm-fundamentals/README.md) — here it is simply applied to the condense threshold.
2. **The model's tokenizer** (e.g., `tiktoken` for OpenAI, `anthropic.count_tokens` for Anthropic). Needed in one case: you want to estimate the weight of a not-yet-sent message to decide "send it, or condense first."
3. **Word/character approximation** — the last resort. Fine for a rough estimate when (1) and (2) are unavailable.

For more on the danger of char-based estimates, see [Chapter 12, Mistake 7](../12-agent-memory/README.md#mistake-7-estimating-tokens-as-char3).

```go
// The most common pattern: after each Chat response, store the token count.
resp, err := client.Complete(ctx, Request{Messages: msgs})
if err == nil {
    r.lastTokens = resp.Usage.PromptTokens
}
```

No `WordBasedCounter` with a `TokensPerWord = 2.0` rate for Russian. No `len(content)/3` counting. The provider has already counted — take the ready number.

## Model Limits

**Don't hardcode a model dictionary in code.** The model zoo changes faster than code gets updated, and a stale dictionary will give wrong answers for fresh models.

Minimal structure and where to keep it:

```go
type Model struct {
    ID            string
    ContextWindow int // max input + output
    MaxOutput     int // usually smaller than ContextWindow
}

func SafeBudget(m Model, reserveOutput int) int {
    if reserveOutput == 0 {
        reserveOutput = m.MaxOutput
    }
    return m.ContextWindow - reserveOutput
}
```

Where to get `Model` from:
- your LLM SDK's model catalog (preferred);
- the service config (if the SDK doesn't provide a catalog);
- environment variables for private deployments.

The main thing — keep it **in one place**; don't smear `_ = 128000` across 15 files.

## When Truncate Applies

`truncate` (dropping messages from the middle while keeping head and tail) is **not needed** in an agent loop with prompt cache: the middle changes → the cache resets on every iteration, and a broken `tool_call ↔ tool_result` pair means a `400 Bad Request`. It remains a tool for single-shot requests and providers without cache. If condense has already fired (1/run) and the context is still overflowing — not truncate, but `wrapUp`.

## The Weight of Tools and Tool Results

It's not just the system prompt and the history you pay for in context. Two constant sources of growth that are often forgotten:

**1. The `tools` block is paid in every request.** Each tool's schema (name, description, parameters) is sent to the provider on every iteration — 4 typical tools already mean ~742 tokens of overhead per request of the run. So descriptions must be short and precise — this is not the place for essays. How to write good schemas — [Chapter 03: Tools & Function Calling](../03-tools-and-function-calling/README.md).

**2. Tool results — the main lever for saving context: FilteredContent.** A tool result exists in two versions: **full** — for the UI and audit (what the human sees), and **filtered** — for the model (what the LLM sees). The classic case: the full output of `go test` (tens of kilobytes) goes to the log/UI for the human; the model gets only the list of failing tests with the last lines of the stack trace. The model almost never needs raw tens-of-KB output, but almost always needs "what broke and where."

Plus two hard budgets worth baking into the loop:

- **Budget per tool result: ~128 KB.** Whatever doesn't fit is truncated with a marker `... truncated, full output in artifact`; the full output is written to an artifact **outside the context** ([Chapter 11: State Management](../11-state-management/README.md)).
- **A limit on images per session.** Images are expensive in tokens and are not compressed by condense (they live in messages); keep a counter and cap the number of images in a single session.

## The Condensation Prompt

Condense quality is 80% determined by the prompt. A bare "summarize this" produces a useless retelling. A good prompt works on the principle of **"handing the task over to a colleague who is about to pick it up."**

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
- **Clear sections.** Otherwise the model writes an essay, and it's impossible to extract "what's left to do" from the summary.
- **A specificity requirement with examples.** Without it you get fluff like "authentication was implemented."
- **A hard word limit.** If you don't set one, the summary balloons to nearly the size of the original.

## System Prompt: Stability Beats Frugality

The system prompt consumes tokens on **every iteration** of the loop. The temptation arises to "optimize": show the long instructions on the first iteration and remove part of them on subsequent ones. On paper — savings. In practice — almost always more expensive.

### Why the "adaptive system prompt" loses

Modern providers cache the **prefix** of the request. A cached token is much cheaper than a regular one:

| Provider | Cache hit discount |
|-----------|---------------------|
| OpenAI | ~50% of input price |
| Anthropic | ~90% of input price |
| Z.AI / GLM | up to ~80% (depends on the model) |

If the system prompt changes between iterations, the cache resets **for everything after the point of change**, and you pay full price not only for the changed sections but for the entire message history as well.

Simple arithmetic on a typical run (system 25K, history ~30K by iteration 5, 8 iterations total):

| Strategy | Input cost per run | Cache hit ratio |
|-----------|-------------------|-----------------|
| Stable system | ~$0.04 (Anthropic) | ~85% |
| "Adaptive": dropped 1500 tokens on iterations 2-8 | ~$0.18 (Anthropic) | ~5% |

Saving 1500 input tokens costs you cache hits on 25-30K prefix tokens on each of the 7 subsequent iterations. **4-5x more expensive.**

### What to do instead

**1. The system prompt is stable within a run.** All dynamic data goes into tool results, into Notes, or into the frame's first user message. Details — [Chapter 12: Live state without mutating the system prompt](../12-agent-memory/README.md#live-state-without-mutating-the-system-prompt).

**2. Fix stable inclusions once.** Date, working directory, mode (`debug` / `prod`) — write them into the system prompt at run start and don't touch them again.

**3. Want a "warning" on overflow — put it in user, not in system.** If you really want to hint to the model "start wrapping up," add a short sentence to the **last user message** of the current frame; don't mutate the system. The cache stays intact.

**4. Conditional "knowledge" is better via a tool than a conditional prompt.** If you want to include an SOP during the analysis phase but not during the action phase, package the SOP as a `tool` (`get_sop("incident_diagnosis")`) and let the model call it itself. The tool result is appended to the tail — the cache is unaffected.

### The 20-25% guideline

The system prompt shouldn't take up a noticeable share of the window; a rough guideline is ~20-25% (for 128K that's ~25-32K tokens). This is a **heuristic, not a rule**: the exact share depends on the task, and nothing "breaks" at 18% or 30%. If it grows — cut the **base version** at the start, not "between iterations." Never put live state in there at all — that's [Mistake 6 from Chapter 12](../12-agent-memory/README.md#mistake-6-live-state-in-the-system-prompt).

## Common Mistakes

### Mistake 1: Dynamic system prompt

**Symptom:** Run cost is 4-5x higher than expected; cache hit ratio in the provider's logs is around 5%.

**Cause:** The system prompt is rebuilt on every iteration (including `current_time`, the current plan, files read, per-iteration dynamic sections).

**Fix:**

```go
// BAD: cache miss on every iteration
sys := fmt.Sprintf(`You are an agent. Current time: %s. Files read: %v. Iteration: %d`,
    time.Now(), filesRead, iteration)

// GOOD: stable prefix
sys := `You are an agent. Use tools to read files when needed.`
// The next user message or tool result will show the agent current_time / filesRead / iteration.
```

### Mistake 2: Live state in the system prompt

**Symptom:** The same 4-5x overspend + on every new tool call the cache hit drops to zero.

**Cause:** Task progress (`Read files: a.go, b.go, c.go`) is updated in the system prompt after every tool call.

**Fix:** Live state lives in tool results / in the last user message / in Notes — that is, **in the tail of the history**. The prefix is untouched. Detailed breakdown — [Chapter 12: Live state without mutating the system prompt](../12-agent-memory/README.md#live-state-without-mutating-the-system-prompt).

### Mistake 3: Importance scoring and reordering

**Symptom:** The provider returns `400 Bad Request: tool_use without matching tool_result` (or the mirror error).

**Cause:** A "smart" algorithm rated some `tool` messages as "unimportant" and dropped them, leaving an `assistant` message with dangling `tool_calls`.

**Fix:** Don't do message scoring or reordering. Use **condense** — it replaces the entire old portion with a text summary, and the problem of broken `tool_call ↔ tool_result` pairs disappears by construction.

### Mistake 4: Re-condensing an already condensed context

**Symptom:** After 2-3 condenses the agent forgets the goal of the task, mixes up files, repeats actions.

**Cause:** Condense is called on every overflow, with no limit.

**Fix:**
```go
// BAD: infinite condensing
for {
    resp, err := llm.Complete(ctx, Request{Messages: messages})
    if isContextOverflow(err) {
        messages = condense(ctx, messages)
        continue
    }
}

// GOOD: at most once per Run, then wrapUp
condenseDone := false
for {
    resp, err := llm.Complete(ctx, Request{Messages: messages})
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

### Mistake 5: Counting tokens via `len(content)/3`

**Symptom:** Condense doesn't kick in until the actual overflow — or the opposite: it fires when the context is only 30% full.

**Cause:** You are using a char-based estimate ("3 characters = 1 token"). For Russian, the real factor is 1.5-2x that estimate; for code, it's 0.5x. That's a ±50% error.

**Solution:** Take `usage.PromptTokens` from the provider's response to the previous request. It is a **fact** counted by the provider itself, and it comes for free. See [Chapter 12, Mistake 7](../12-agent-memory/README.md#mistake-7-estimating-tokens-as-char3) for details.

### Mistake 6: Hardcoding a model dictionary in code

**Symptom:** After switching models, the agent behaves "oddly" — it hits the overflow sooner than it should, or starts condensing late.

**Cause:** A dictionary like `var ModelLimits = map[string]int{"glm-5.3": 128000, ...}` wasn't updated for the new model, so it got the default value of `4096`.

**Solution:** Take the limit from your SDK's model catalog (see the [Model limits](#model-limits) section above). One place, don't scatter it across the code.

## Exercises

### Exercise 1: BeforeNextRequest

Implement a function that decides whether condense should run **before** the next request.

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
- Returns `true` if `lastTokens / contextWindow >= 0.85` (the typical threshold) and condense hasn't happened yet.
- Returns `false` in every other case.
- No message counters, no layers.

### Exercise 2: Reactive condense on overflow

Extend the loop so that on `ContextOverflowError` it condenses once and retries the request, and on the second occurrence — exits with `wrapUp`.
```go
for {
    resp, err := client.Complete(ctx, Request{Messages: mem.Messages()})
    // your code
}
```

**Expected result:**
- Condense is called at most once per Run.
- On a second overflow — `wrapUp`, not another condense.
- Non-overflow errors are propagated up, not "treated" with a condense.

### Exercise 3: Moving facts from system to user

Given: code where long-term-memory facts were added as a separate `system` message and rewritten on every iteration. Task: restructure it so the facts land in the **first user message of the Run** and never change again (if the set of facts changes within a Run, that's a signal you need an external search tool over long-term memory, not a context mutation).

**Expected result:**
- One `system` message per Run in the code, immutable.
- Facts are added exactly to the first `user` message of the Run.
- Cache hit ratio in the provider's logs grows.

## Acceptance Criteria / Checklist

✅ **Pass:**
- [x] You understand the 4 context layers as a **mental model**, not as 4 separate `system` messages
- [x] You take `usage.PromptTokens` from the provider as the primary source of token counts
- [x] Model limits live in one place (catalog/config), not scattered across the code
- [x] Condensing is one configurable threshold (typical value ~0.85, threshold or overflow) and one limit (once per Run)
- [x] Condense is a full replacement of history, the summary as `user`, the tail of the last 3-5 messages untouched
- [x] You know that the `tools` block is paid for in every request, and that a tool result has a full version (UI/audit) and a filtered one (for the model)
- [x] A condense failure is a `CondenseErrorEvent`, and the Run continues with the uncondensed history
- [x] The system prompt is stable within a Run (live state lives outside the system prompt, see [Chapter 12](../12-agent-memory/README.md#live-state-without-mutating-the-system-prompt))
- [x] You understand the trade-off: an "adaptive prompt" almost always loses to the prompt cache on cost
- [x] You know that on a repeated overflow the move is `wrapUp`, not another condense

❌ **Fail:**
- [ ] Context grows without bound — no threshold-based condense
- [ ] Condense is called again on an already condensed context (details are lost exponentially)
- [ ] A dynamic system prompt changes on every iteration (cache miss)
- [ ] Live state (progress, files read, current date on every iteration) sits in `system`
- [ ] Importance scoring and message reordering — this breaks `tool_call ↔ tool_result` chains
- [ ] Counting tokens via `len(content)/3` instead of `usage.PromptTokens`
- [ ] A hardcoded model dictionary in code — it goes stale faster than the code is updated
- [ ] "Layers with shares" (`SystemRatio: 0.10, FactsRatio: 0.10 ...`) — an illusion of control, not control

## For the curious

> This section is for those who still want to go deeper. Feel free to skip it.

### Why one threshold instead of two

The literature often features a cascade: "warning at 75% → condense at 85% → wrap-up at 95%". In practice, with the condition `usage.PromptTokens >= contextWindow * 0.85`, a single threshold covers all cases, because:

- By the time you reach 85%, you already have **exact** information about consumption (from the last `usage`). There is no uncertainty that would require a "warning" as a separate level.
- A warning via mutating the system prompt is an anti-pattern (cache miss). Via mutating an assistant message — also a bad idea (the model sees someone else's "voice"). That leaves inserting it into a user message — and then it's no longer a separate level, just "added a line to the next user message".
- Wrap-up is not a condensing level but an **outcome**. It doesn't shrink the context; it saves state and ends the Run. Logically it comes **after** condense, not alongside it.

### What about block memory and recall

For REPL-like interaction (one complex request → one big result → next request), the literature describes **block memory** with a `recall(block_id)` tool: history is cataloged into blocks, summaries go into the active context, and the full contents of a block are loaded on demand by the model. This is **not the main path**: two sources of truth (catalog and originals), double cost (summary calls + cache misses), and an extra layer on top of a linear history. A breakdown of why this is almost always unnecessary is in [Chapter 12, "For the curious"](../12-agent-memory/README.md#for-the-curious).

### When the context genuinely doesn't fit

If a task is so big that even after condense 128K isn't enough, that's a signal not to "optimize compression" but to **split the task into sub-Runs** (see [Chapter 09: Architecture](../09-agent-architecture/README.md)). Each sub-Run works in its own context, the result is saved to long-term memory or a file, and the top level assembles the results afterwards. This is how every production agent capable of working with large codebases does it.

## Relation to Other Chapters

- **[Chapter 04: Autonomy and Loops](../04-autonomy-and-loops/README.md)** — the basic loop into which `BeforeNextRequest` and the overflow response are built
- **[Chapter 11: State Management](../11-state-management/README.md)** — `wrapUp` saves state via a checkpoint
- **[Chapter 12: Agent Memory Systems](../12-agent-memory/README.md)** — linear History, facts in user, live state without mutating the system prompt
- **[Chapter 20: Cost & Latency Engineering](../20-cost-latency-engineering/README.md)** — prefix stability, prompt cache, the cost of condense

**Important:** Context Engineering is about **assembling the context for a single request**. Storing knowledge between sessions is covered in Memory; persistent data (schemas, policies) in State Management; external search in RAG.

## What's Next?

After mastering context engineering, move on to:
- **[14. Ecosystem and Frameworks](../14-ecosystem-and-frameworks/README.md)** — an overview of popular agent frameworks: where they help and where they get in the way.

---

**Navigation:** [← Chapter 12: Memory](../12-agent-memory/README.md) | [Table of Contents](../README.md) | [Chapter 14: Ecosystem →](../14-ecosystem-and-frameworks/README.md)
