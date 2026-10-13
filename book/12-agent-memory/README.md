# 12. Agent Memory Systems

## Why does this matter?

An agent needs memory to hold context between iterations of the loop, to not forget decisions within a single task, and to avoid starting every conversation from scratch. Without memory, an agent spends tokens re-explaining things, loses the thread of the task by the 5th iteration, and doesn't remember anything from a week ago.

But memory isn't just "where to put the data." Any work with memory touches the LLM's context window, the token budget, and the provider's **prompt cache**. Careless memory architecture turns a cheap agent into an expensive and slow one.

In this chapter we'll look at how memory works in a production agent: what horizons exist, what to store immutably, why the only history-condensing operation is `condense`, and why a "dynamic system prompt" is usually a costly mistake.

### A real-world case

**Situation:** A user asks the agent: "What was the database problem we fixed last week?" The agent replies: "I have no information about that."

**Problem:** The agent has no memory of past conversations. Every interaction starts from zero.

**Solution:** Long-term memory — an external store (DB/files) that keeps key facts (decisions, artifacts, preferences). The agent reads from it on demand via tools; stale entries are cleaned up by TTL so the store doesn't turn into a garbage dump.

## The theory in plain words

### Two memory horizons

An agent has two distinct levels of memory, and they must not be confused:

**Within a run (between loop iterations):**
- The message history of the current task: `[]Message`.
- Grows from iteration to iteration.
- Bounded by the model's context window.
- When the run ends, it may be persisted — or forgotten.

**Between runs / sessions (long-term):**
- Decisions, facts, preferences, artifacts.
- Stored in DBs/files.
- Never goes into the LLM context in full — the agent selectively retrieves from it with tools, when it needs to.

Most memory design mistakes happen because these two levels get mixed up: trying to store the entire run in long-term storage, or conversely, turning long-term storage into part of the system prompt and paying tokens for it on every iteration.

### Conceptual classification (as used in the literature)

**Short-term (working) memory** — the state of the current task: what the agent has already done, which files it has read, what plan is in progress.

**Long-term memory** — survives the end of a run: facts, preferences, histories of decisions.

**Episodic** — concrete events: "The user asked about disk space on 2026-01-06."

**Semantic** — generalizations: "The user prefers JSON responses." Derived from episodes.

These terms are useful for communication, but in code you usually only need to distinguish "the history of the current run" and "the persistent store."

### Memory operations

Loop memory is a concrete structure (a linear history), and it has exactly five operations:

1. **Append** — add messages to the end.
2. **Messages** — get a copy of the history for the LLM request.
3. **Replace** — full replacement of the history (called only by condense, ch. 13).
4. **Snapshot** — save the state (checkpoint, audit, rollback of a failed condense).
5. **Restore** — restore what was saved.

Store / Retrieve / Forget are the vocabulary for the **external store** of long-term memory, not operations on the loop history. Don't mix them: within a run — history and only history; between runs — an external store the agent reaches with tools.

### Principle: memory is an immutable history

The main rule everything below rests on:

> **The history of already-sent messages is never rewritten. It is only appended to (append) or fully replaced (replace).**

Three practical consequences follow from this rule:

1. **Don't rewrite the past.** No "let's delete that extra tool result" or "let's tighten up that assistant reply." This breaks the prompt cache on the tail and sometimes makes the model imitate its own earlier style.
2. **The system prompt is stable.** Don't put dynamic data (what was read, what the plan is) into the system prompt — it's a computational tax on every iteration.
3. **Condensing is a full replacement.** If the history has grown too large, we deliberately, entirely, and rarely replace it with a summary + tail (see condense below). No partial rewrites.

These rules look strict, but they are exactly what makes the difference between "an agent that runs fast and predictably" and "an agent that's expensive, slow, and loses context."

## How it works (step by step)

### Step 1: History — the loop memory

Loop memory is concrete: it is a **linear message history**. No Store/Retrieve/Forget interfaces, no blocks or layers — a flat `[]Message` under a mutex and five operations:

```go
type History struct {
    mu   sync.Mutex
    msgs []llm.Message
}

func NewHistory() *History { return &History{} }

// Append adds messages to the end. The only normal growth operation.
func (h *History) Append(msgs ...llm.Message) {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.msgs = append(h.msgs, msgs...)
}

// Messages returns a COPY of the history — this is what goes into the LLM request.
func (h *History) Messages() []llm.Message {
    h.mu.Lock()
    defer h.mu.Unlock()
    out := make([]llm.Message, len(h.msgs))
    copy(out, h.msgs)
    return out
}

// Replace is the only way to change what has already been recorded.
// Called only by condense (ch. 13) and by tests.
func (h *History) Replace(msgs []llm.Message) {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.msgs = append(h.msgs[:0], msgs...)
}

// Snapshot/Restore — save and restore the state:
// a checkpoint, auditing, rollback of a failed condense.
func (h *History) Snapshot() []llm.Message { return h.Messages() }

func (h *History) Restore(snap []llm.Message) { h.Replace(snap) }
```

Sanitization is built into the history itself: an empty assistant message is dropped, an orphan tool-result (a `tool_result` without its `tool_call`) is dropped, and `tool_call ↔ tool_result` pairs are inseparable. The history returned by `Messages()` is always valid for the provider — no separate "cleaner" is needed before the request.

> **How the real SDK does it.** The loop memory is a linear `History` (`New`/`Append`/`Replace`/`Snapshot`/`Restore`), with no blocks or layers. Sanitization (empty assistant messages, orphan tool-results) is built in. Cache metrics (CacheRead / CacheCreation) are parsed from the provider response's `Usage`.

### Step 2: Long-term memory — an external store and tools

Long-term memory is **not a layer in the loop memory**. It's an external store (DB, files) that the agent reads on demand — with ordinary tools (`search_memory`, `read_note`), just as it reads files or browses the web. A linear history lives within a run; long-term memory lives in the store, and the loop memory knows nothing about it.

Two working patterns:

1. **Prefetch at the start of a run.** Relevant facts are fetched from the store before the first request and placed into the run's **first user message**.
2. **On-demand tool.** The agent itself calls `search_memory(query)` when it realizes something is missing. The tool result is appended to the tail of the history — the cache doesn't suffer.

```go
// Pattern 1: facts go into the first user message, NOT the system prompt.
messages := []llm.Message{
    {Role: "system", Content: sysPrompt}, // constant within the run
    {Role: "user", Content: userQuery +
        "\n\nRelevant facts from long-term memory:\n- ..."},
}
```

Note: facts from long-term memory go **in the first user message**, not in the system prompt. If you change the system prompt at the start of every run depending on what was found in memory, you'll get a cache miss on every request.

What exactly to put in the store and how to organize retrieval (embeddings, hybrid with BM25) — see the [Long-term memory](#long-term-memory-between-sessions) section below.

## Linear Memory Within a Run

The default memory model at the level of a single run is **linear**: a flat `[]Message` that each step of the loop appends new content to. This is exactly the `History` from Step 1 — here we'll look at why nothing fancier is needed.

### Why this design

- **The prompt cache works.** Modern providers (OpenAI, Anthropic, Z.AI and compatible ones) cache the request prefix. A stable prefix means nearly free input tokens on repeated iterations. Any mutation of the tail invalidates the cache beyond the point of change; a mutation of the prefix invalidates the entire cache.
- **Fewer moving parts.** A separate history = a single source of truth. There's no "original" and "compact copy" that can drift apart.
- **Provider compatibility.** All drivers (OpenAI, Anthropic, Z.AI, vLLM) accept the same message array. Any additions — blocks, recall, parallel summaries — are already an extra layer on top (see "For the curious").

### Live state without mutating the system prompt

The temptation to put dynamic state into the system prompt is very strong:

```go
// BAD: changes on every iteration → cache miss for the whole prefix
sysPrompt := basePrompt +
    "\n\nFiles read: " + filesRead.Join(", ") +
    "\nLast actions: " + lastActions.Join(", ") +
    "\nPlan: " + plan.Render()
```

What's wrong: every change to `filesRead` or `plan` invalidates the cache for all ~N thousand tokens of the system prompt. On a long task this becomes a constant tax: you pay full price for input tokens on every iteration, even though 95% of the prefix never changed.

Where to put live state:

1. **In the result of the last tool call.** If a tool returned `read_file: {content}`, it's natural to append the agent's notes to it.
2. **In separate `Notes` structures**, rendered into the last `user` message or into the first message of the next iteration.
3. **In a tool the agent calls itself**, to set a checkpoint for itself (`update_plan`, `set_goal`).

```go
// GOOD: live state in the result of the last tool call
result := tool.Result{
    Content: actualOutput,
    Notes: []string{
        "plan: 2/5 done",
        "files touched: a.go, b.go",
    },
}
```

A stable system prompt + an append-only history = maximum cache hits and minimum surface for bugs.

For more on context assembly, budgets, and dynamic sections (when they really are needed and how to pay the least for them) — see [Chapter 13: Context Engineering](../13-context-engineering/README.md).

## Condense: The Only Condensing Operation

If the history has grown to a threshold, it gets condensed, and there is only one way to do it:

- **One threshold: ~0.85 of the context window.** It is computed from the provider's `usage.PromptTokens` in the last response; the "~3 characters per token" estimate is only a fallback for when the provider hasn't responded yet.
- **At most 1 condense per run.**
- **The head is summarized by a separate LLM call with the same cacheable prefix** (system + tools): the summarization reads the history from a warm cache instead of paying full price for it.
- **The tail (the last messages) is kept intact** — insurance against losing what the model needs right now.
- **Originals are not destroyed:** a `Snapshot()` is taken before `Replace`. That is the whole point of **never destroy originals** — not in parallel "compact copies" that can drift from the truth, but in a snapshot for recovery and audit.

There is **no** separate "compact" operation (structural rewriting of the history, compact messages with tags like `<prior_tool_use>`), and none is needed: it resets the prompt cache on the tail and makes the model imitate a foreign format as "its own style." If the history is too large — condense, period.

The summarization prompt and the reaction to a provider overflow — see [Chapter 13: Context Engineering](../13-context-engineering/README.md#the-condensation-prompt).

## Long-Term Memory (Between Sessions)

Within a run, a linear history is at work. Between sessions it is lost — you need an explicit external store from which the agent reads on demand with tools (Step 2 above).

### What to store

- **Decisions and their rationale.** "We chose PostgreSQL because we need transactions" — a fact that is useful forever.
- **Stable identifiers.** User name, namespace, environment, working directory.
- **Established preferences** — with an explicit `preference` type (see anchoring bias in ch. 13).
- **Task artifacts.** Links to created resources, migrations, PR IDs.

### What NOT to store

- **Every turn of the conversation.** That's noise; retrieval starts returning garbage.
- **Hypotheses and temporary status.** "The service is down right now" — will be stale by the next session and will throw off diagnostics.
- **Full session transcripts.** If you really must — store them separately, not in the same table as the facts for retrieval.

### Link to the lab

The implementation — writing facts to a file/DB, embedding-based retrieval, filtering by type — is practiced in [Lab 11: Memory & Context Engineering](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab11-memory-context).

## Checkpoint and Resume

A long run can fail midway: the process was killed, the rate limit ran out, the network dropped. A checkpoint saves the state (including the history's `Snapshot()`) so a restart doesn't start from zero. The basic implementation (the structure, save/load, integration with the agent loop) is in [Chapter 09: Agent Architecture](../09-agent-architecture/README.md#checkpoint-and-resume); advanced strategies (granularity, validation, rotation) are in [Chapter 11: State Management](../11-state-management/README.md#advanced-checkpoint-strategies).

In the context of memory, the rule is one: **save the original history, not the condensed one**. If the checkpoint contains the result of a condense, after a resume you won't be able to recover the details of tool calls.

### Shared memory between agents

In multi-agent systems, agents exchange information through a shared external store — the same external store, just with multiple writers. Data is partitioned by namespace so agents don't overwrite each other's:

```go
func sharedKey(agentID, key string) string {
    return fmt.Sprintf("shared:%s:%s", agentID, key)
}
```

The rules are the same as for long-term memory: nothing "shared" in the system prompt; other agents' artifacts are read with tools, into the tail of the history.

> **Related:** agent state management — [Chapter 11: State Management](../11-state-management/README.md) (a checkpoint is a special case of state persistence); exchange and coordination between agents — [Chapter 07: Multi-Agent](../07-multi-agent/README.md).

## Common Mistakes

### Mistake 1: No TTL

**Symptom:** Memory grows without bound, consuming storage and context. Retrieval returns stale facts.

**Cause:** Stale information is never forgotten.

**Solution:** Implement TTL and periodic cleanup. Different types of facts get different TTLs: user preferences live for a long time, temporary status for hours.

### Mistake 2: Saving everything

**Symptom:** Memory fills up with irrelevant information, retrieval becomes noisy.

**Cause:** No filtering of what is worth saving at all.

**Solution:** Save only important facts, not every turn of the conversation. A good filter is a separate lightweight LLM call at the end of a run: "extract from this dialog the facts worth remembering long-term."

### Mistake 3: Keyword search only

**Symptom:** Retrieval returns irrelevant results or misses important information.

**Cause:** Simple substring matching doesn't understand synonyms and paraphrases.

**Solution:** Use embeddings for semantic search. Hybrid (BM25 + embeddings) is usually better than either one alone.

### Mistake 4: Condensing destroys originals

**Symptom:** After condensing, the details of tool calls can't be recovered; resume and audit work with an incomplete history.

**Cause:** The original messages were overwritten by the condensing result (or "optimized" in place).

**Solution:** There are no compact copies — the only operation on the history is condense. "Never destroy originals" is implemented with a snapshot: before `Replace`, save a `Snapshot()` of the original — you need it for recovery, audit, and rollback if the summarization LLM call fails.

```go
snap := h.Snapshot() // the original is saved
summary, err := summarize(ctx, head(snap))
if err != nil {
    return err // the original is untouched, the run continues with the uncondensed history
}
h.Replace(newHistory(summary, tail))
```

### Mistake 5: Condense mid-loop without a threshold

**Symptom:** The agent loses context in the middle of a task, starts over or repeats actions. On the next iteration the model can't see what was just being discussed.

**Cause:** Condensing is called inside the agent loop, while the task is still in progress.

**Solution:** Condense by threshold (~0.85 of the context window) or on `ContextOverflowError`, and at most once per run. There are no other ways to rewrite a live history — don't touch it mid-task.

### Mistake 6: Live state in the system prompt

**Symptom:** On every iteration, prompt-cache hits ≈ 0%, latency grows linearly with the history length, cost is ×3–5 of what was expected. The provider's `cached_tokens` metric hovers around zero.

**Cause:** Changing data (the current file, files read, plan progress) is baked into the system prompt. Any mutation invalidates the whole prefix.

**Solution:** Keep the system prompt stable. Stable inclusions (the date, the working directory) are constants within a run: fixed once at startup and never changed — that's fine. The anti-pattern is recreating the agent with `time.Now()` in the system prompt on every request: every request gets a new prefix → a cache miss across the entire length. Live state goes into tool results, into the Notes of the last message, or into special tool calls (`update_plan`, `set_goal`) that the model makes itself.

### Mistake 7: Estimating tokens as char/3

**Symptom:** Threshold-condense fires at the wrong time — sometimes too early (context is lost for nothing), sometimes too late (you get a `ContextOverflowError`). Behavior varies widely between models.

**Cause:** "Length in characters / 3" is an approximation that systematically misses by 30% or more. For Russian, for code, and for models with updated tokenizers (e.g., after a vocabulary change), the miss is even bigger.

**Solution:** Take `Usage.PromptTokens` from the provider's response to the previous iteration — it's free and accurate. A character-based estimate is only needed for a new user message that hasn't gone to the LLM yet.

```go
// BAD
estimate := totalChars / 3
if estimate > threshold { condense() }

// GOOD
budget := lastUsage.PromptTokens + roughEstimate(newUserMessage)
if budget > threshold { condense() }
```

## For the curious

> This section is for those who still want to go deeper. Feel free to skip it.

### Block memory: a block catalog and recall

The idea looks attractive: group the history into blocks of "one request → one agent loop → a result," keep a catalog of blocks with short summaries, and let the model pull in a block's full contents on demand via a `recall(block_id)` tool. In a REPL agent, where every command is a self-contained story, it's a tempting way to let the model address the past ("look at what happened in command #3").

Why it isn't the main path:

- **Two sources of truth.** The catalog/summaries and the original blocks inevitably drift apart; debugging "what exactly did the model see" gets harder.
- **Double the cost.** LLM calls to summarize each block + cache misses when a large block is inserted into the middle of the history.
- **An extra layer.** The same 90% of cases are covered by condense (see above) + an external store with a search tool.

If you really need an addressable past — try an external store + a search tool first: it's the same recall, but without a second representation of the history.

## Exercises

### Exercise 1: File-backed store

Implement an external long-term memory store that survives a process restart:

```go
type FileStore struct {
    filepath string
    // ...
}

func (s *FileStore) Save(ctx context.Context, key string, value any) error {
    // Save to a JSON file (atomic write via temp + rename)
}
```

**Expected outcome:**
- The store survives a restart.
- Writes are atomic (no partially written files after a crash).
- There's a separate `Cleanup()` command to sweep by TTL.

### Exercise 2: Semantic search over the store

Implement retrieval via embeddings on top of the external store:

```go
func (s *FileStore) SearchSemantic(ctx context.Context, query string, limit int) ([]Item, error) {
    // Encode the query, compute cosine similarity to items, return top-K
}
```

**Expected outcome:**
- Finds relevant items without exact word matches.
- Returns the most similar items first.
- An embeddings model outage must not crash the search (errors are logged, the method returns whatever it managed to get).

### Exercise 3: History + threshold-condense

Implement `History` (Append / Messages / Replace / Snapshot / Restore) and a guard function:
```go
func shouldCondense(usage llm.Usage, ctxWindow int, threshold float64) bool {
    // true if usage.PromptTokens / ctxWindow >= threshold (a typical value is ~0.85)
}

func condense(ctx context.Context, ep llm.Endpoint, msgs []llm.Message) ([]llm.Message, error) {
    // 1. Split into head (the old part) and tail (the last 2 user steps)
    // 2. Ask ep to produce a summary of the head using the prompt from ch. 13;
    //    the summarization request carries the same system + tools prefix
    // 3. Return [summary as a user message] + tail
}
```

**Expected outcome:**
- The history is append-only, no mutations except `Replace`.
- The threshold triggers on real `PromptTokens`, not char/3.
- An original snapshot is saved before a condense succeeds (if the LLM call fails, the history is not lost).
- Condense is not called more than once per run.

## Acceptance Criteria / Checklist

✅ **Pass:**
- You understand the split between "within a run" vs "between sessions": a linear history vs an external store with tools.
- The loop memory is a `History` (Append / Messages / Replace / Snapshot / Restore) with sanitization built in.
- You don't mutate the system prompt with live state; the date and working directory are constants within a run.
- Condense is the only condensing operation: one threshold of ~0.85 based on `Usage.PromptTokens`, at most 1 per run, the original preserved via a snapshot.
- You count tokens via `Usage.PromptTokens`, not char/3.
- You implement TTL and filtering for the long-term store.

❌ **Fail:**
- Condense is called more than once per run.
- Live state lives in the system prompt.
- Char/3 is the only token counter.
- Condensing runs without a snapshot of the original (originals destroyed).
- Long-term memory with no TTL and no filter for what goes in.
- Block memory (a block catalog + `recall(block_id)`) in the main loop path — that's the "for the curious" path, not the default.

## Relation to Other Chapters

- **[Chapter 09: Agent Architecture](../09-agent-architecture/README.md)** — memory as one of the agent's key components, its connection to the runtime.
- **[Chapter 11: State Management](../11-state-management/README.md)** — checkpoints, idempotency, persisting state.
- **[Chapter 13: Context Engineering](../13-context-engineering/README.md)** — assembling context from memory, budgets, condense, dynamic system-prompt sections.
- **[Chapter 20: Cost & Latency Engineering](../20-cost-latency-engineering/README.md)** — why the prompt cache matters so much and how not to break it.

> **Boundary:** this chapter is responsible for **storing and retrieving** information. Managing how that information is assembled into the LLM context is covered in [Context Engineering](../13-context-engineering/README.md).

## What's Next?

Once you understand memory systems, move on to:
- **[13. Context Engineering](../13-context-engineering/README.md)** — you'll learn to assemble context from memory, state, and retrieval without breaking the prompt cache.
