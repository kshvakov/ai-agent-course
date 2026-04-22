# 12. Agent Memory Systems

## Why This Chapter?

An agent needs memory to hold context between loop iterations, to remember decisions inside a single task, and to avoid starting every conversation from scratch. Without memory, the agent spends tokens on repeated explanations, loses the meaning of the task by iteration 5, and doesn't remember what happened a week ago.

But memory isn't just "where to put the data". Any work with memory touches the LLM context window, the token budget, and the provider's **prompt cache**. A sloppy memory architecture turns a cheap agent into an expensive and slow one.

In this chapter we'll unpack how memory is built in a production agent: which horizons exist, what to store immutably, how `compact` differs from `condense`, and why a "dynamic system prompt" is usually an expensive mistake.

### Real-World Case Study

**Situation:** The user asks the agent: "What was the database problem we fixed last week?" The agent answers: "I have no information about that."

**Problem:** The agent has no memory of past conversations. Every interaction starts from zero.

**Solution:** Long-term memory stores key facts (decisions, artifacts, preferences), lets them be retrieved on demand, and forgets stale data to keep within context limits.

## Theory in Simple Terms

### Two memory horizons

An agent has two distinct levels of memory, and they must not be mixed up:

**Inside a Run (between loop iterations):**
- The message history of the current task: `[]Message`.
- Grows iteration by iteration.
- Bounded by the model's context window.
- On Run completion — may be saved, or may be forgotten.

**Across Runs / sessions (long-term):**
- Decisions, facts, preferences, artifacts.
- Stored in a DB/files.
- Doesn't go into the LLM context in full — retrieved selectively.

Most memory design mistakes happen because these two levels get mixed up: people try to store the whole Run in long-term storage, or conversely — turn long-term storage into a piece of the system prompt and pay tokens for it on every iteration.

### Conceptual classification (as in the literature)

**Short-term (working) memory** — the state of the current task: what the agent has already done, which files it has read, what plan is in flight.

**Long-term memory** — outlives the Run: facts, preferences, histories of decisions.

**Episodic** — specific events: "User asked about disk space on 2026-01-06".

**Semantic** — generalizations: "User prefers JSON responses". Extracted from episodes.

These terms are useful for communication, but in code it's usually enough to distinguish "history of the current Run" and "persistent storage".

### Memory operations

1. **Store** — save information.
2. **Retrieve** — find what's relevant.
3. **Forget** — delete what's stale.
4. **Update** — change what's there.

### Principle: memory is an immutable history

The main rule everything else rests on:

> **The history of already sent messages is not rewritten. It is only appended (append) or fully replaced (replace).**

Three practical consequences follow from this rule:

1. **Don't rewrite the past.** No "let's delete that extra tool result" or "let's compress the assistant response". That busts prompt cache on the tail and sometimes pushes the model to imitate its own earlier style.
2. **The system prompt is stable.** Dynamic data (what was read, what plan is active) doesn't go into the system prompt — that's a compute tax on every iteration.
3. **Compression is a full replacement.** If there's too much history, we deliberately, as a whole, and rarely replace it with summary + tail (see condense below). No partial rewrites.

The rules look strict, but they are exactly what makes the difference between "agent is fast and predictable" and "agent is expensive, slow, and loses context".

## How It Works (Step by Step)

### Step 1: Basic memory interface

```go
type Memory interface {
    Store(key string, value any, metadata map[string]any) error
    Retrieve(query string, limit int) ([]MemoryItem, error)
    Forget(key string) error
    Update(key string, value any) error
}

type MemoryItem struct {
    Key      string
    Value    any
    Metadata map[string]any
    Created  time.Time
    Accessed time.Time
    TTL      time.Duration
}
```

### Step 2: Simple store with TTL

```go
type SimpleMemory struct {
    store map[string]MemoryItem
    mu    sync.RWMutex
}

func (m *SimpleMemory) Store(key string, value any, metadata map[string]any) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    m.store[key] = MemoryItem{
        Key:      key,
        Value:    value,
        Metadata: metadata,
        Created:  time.Now(),
        Accessed: time.Now(),
        TTL:      24 * time.Hour,
    }
    return nil
}

func (m *SimpleMemory) Cleanup() error {
    m.mu.Lock()
    defer m.mu.Unlock()

    now := time.Now()
    for key, item := range m.store {
        if item.TTL > 0 && now.Sub(item.Created) > item.TTL {
            delete(m.store, key)
        }
    }
    return nil
}
```

### Step 3: Retrieval

```go
func (m *SimpleMemory) Retrieve(query string, limit int) ([]MemoryItem, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    results := make([]MemoryItem, 0, len(m.store))
    queryLower := strings.ToLower(query)

    for _, item := range m.store {
        if item.TTL > 0 && time.Since(item.Created) > item.TTL {
            continue
        }
        valueStr := fmt.Sprintf("%v", item.Value)
        if strings.Contains(strings.ToLower(valueStr), queryLower) {
            item.Accessed = time.Now()
            results = append(results, item)
        }
    }

    sort.Slice(results, func(i, j int) bool {
        return results[i].Accessed.After(results[j].Accessed)
    })

    if len(results) > limit {
        results = results[:limit]
    }
    return results, nil
}
```

In production, keyword search is replaced by embeddings: the model encodes the query and each item as a vector, and retrieve returns the top-K by cosine similarity. That's a separate topic and depends heavily on the chosen vector DB; for a basic understanding, what's above is enough.

### Step 4: Integration with the agent

```go
func runAgentWithMemory(ctx context.Context, ep llm.Endpoint, mem Memory, userInput string) (string, error) {
    memories, _ := mem.Retrieve(userInput, 5)

    messages := []llm.Message{
        {Role: "system", Content: "You are a helpful assistant."},
    }

    if len(memories) > 0 {
        var sb strings.Builder
        sb.WriteString("Relevant facts from long-term memory:\n")
        for _, m := range memories {
            fmt.Fprintf(&sb, "- %s: %v\n", m.Key, m.Value)
        }
        messages = append(messages, llm.Message{
            Role:    "user",
            Content: sb.String(),
        })
    }

    messages = append(messages, llm.Message{Role: "user", Content: userInput})

    resp, err := ep.Chat(ctx, llm.Request{Messages: messages})
    if err != nil {
        return "", err
    }
    answer := resp.Content

    if shouldStore(userInput, answer) {
        _ = mem.Store(generateKey(userInput), answer, map[string]any{
            "user_input": userInput,
            "timestamp":  time.Now(),
        })
    }
    return answer, nil
}
```

Note: facts from long-term memory arrive **in the first user message**, not in the system prompt. If you change the system prompt at the start of every Run based on what was found in memory, you'll get a cache miss on every request.

## Linear memory inside a Run

The default memory model at the level of a single Run is **linear**: a flat `[]Message` to which every loop step appends something new.

### Minimal code

```go
type LinearMemory struct {
    msgs []llm.Message
    mu   sync.Mutex
}

func (m *LinearMemory) Append(msgs ...llm.Message) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.msgs = append(m.msgs, msgs...)
}

func (m *LinearMemory) Snapshot() []llm.Message {
    m.mu.Lock()
    defer m.mu.Unlock()
    out := make([]llm.Message, len(m.msgs))
    copy(out, m.msgs)
    return out
}

// Reset — the only way to change what's already been added.
// Used only by condense (see below) and by tests.
func (m *LinearMemory) Reset(msgs []llm.Message) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.msgs = append(m.msgs[:0], msgs...)
}
```

### Why exactly this way

- **Prompt cache works.** Modern providers (OpenAI, Anthropic, Z.AI and compatibles) cache the request prefix. A stable prefix means almost-free input tokens on repeat iterations. Any mutation of the tail busts the cache from the change point on; a mutation of the prefix busts the entire cache.
- **Fewer moving parts.** A single history = a single source of truth. There is no "original" and "compact copy" that can drift apart.
- **Provider compatibility.** All drivers (OpenAI, Anthropic, Sber, vLLM) accept the same message array. Any extras — recall, blocks, summaries — live at a higher level.

### Live state without mutating the system prompt

The temptation to put dynamic state into the system prompt is huge:

```go
// BAD: changes on every iteration → cache miss on the entire prefix
sysPrompt := basePrompt +
    "\n\nFiles read: " + filesRead.Join(", ") +
    "\nLast actions: " + lastActions.Join(", ") +
    "\nPlan: " + plan.Render()
```

What's wrong: every change to `filesRead` or `plan` invalidates the cache on all ~N thousand tokens of the system prompt. On a long task this becomes a steady tax: you pay full price for input tokens every iteration even though 95% of the prefix hasn't changed.

Where to put live state:

1. **In the result of the last tool call.** If a tool returned `read_file: {content}`, it's natural to append the agent's notes to it.
2. **In separate `Notes` structures** that are rendered into the last `user` message or the first message of the next iteration.
3. **In a tool that the agent calls itself** to set its own checkpoint (`update_plan`, `set_goal`).

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

For more on assembling context, budgets, and dynamic sections (when they really are needed, and how to pay minimally for them) — see [Chapter 13: Context Engineering](../13-context-engineering/README.md).

## Block Memory

The linear model solves 80% of tasks. The remaining 20% are cases where tasks inside a single agent process naturally split into closed episodes, and it's useful for the user/agent to address them by ID.

Typical cases:
- REPL interface: every user command is a separate episode, and on command #10 you want to give the model the chance to say "take a look at what I did in command #3".
- A long-running coaching agent where sessions are explicitly separated and it makes sense to pull only summaries across them.

Here **block memory** is useful: we group history into blocks along "one user request → one agent cycle → completion".

### Block structure (without compact-via-tags)

```go
type Block struct {
    ID       int
    Query    string         // first 120 chars of the first user message
    Messages []llm.Message  // original, immutable
    Tokens   int            // Usage.PromptTokens from the last provider response
    Summary  string         // 1-2 lines for the catalog
}
```

The principle:
- `Messages` is the truth. No "compact copies with trimmed tool args".
- `Summary` is a short string for the block catalog, not a replacement for the content. Produced when a block closes (one LLM call to a cheap model, or manually from the task title).
- `Tokens` — we take from the provider response, we don't count "by characters". That matters — see Error 7 below.

### Catalog and recall

The model doesn't see the content of past blocks directly. The first user message of the next block carries a **catalog** — a short list of blocks with summaries:

```
[CONTEXT BLOCKS]
#0: check disk usage — "Disk /data at 92%, cleaned logs" (~5K tokens)
#1: deploy service   — "Deployed v2.3.1 to staging" (~8K tokens)
#2: fix nginx config — "Updated proxy_pass for /api" (~3K tokens)
```

If the current task needs details — the model calls the tool `recall(block_id)`:

```go
type RecallTool struct{ store *BlockStore }

func (t *RecallTool) Execute(ctx context.Context, args RecallArgs) (string, error) {
    msgs, ok := t.store.Block(args.BlockID)
    if !ok {
        return "Block not found", nil
    }
    return formatMessages(msgs), nil
}
```

Recall returns the block's **original** `Messages`. That's crucial: the whole point of the mechanism is to have full data at hand in case the summary isn't enough.

### What NOT to do

Historically, "compact messages" were often added to the block model — the chain `assistant(tool_calls) → tool_result` was replaced with a single string like:

```
<prior_tool_use>
read_file path=a.go -- ok, 450 lines
exec git diff -- 123 lines, 5 files
</prior_tool_use>
```

On paper this gives 80-90% compression. In practice — two persistent problems:

1. **Cache invalidation.** Compact changes the tail of history → the next iteration is a full prompt cache miss on everything after the compact point.
2. **Imitation.** If the compact message is put in with the `assistant` role, the model often starts treating its (or someone else's) earlier compacts as "my style" and keeps writing in the same format (tags included). If put as `user` — noise goes up and the model's trust in the context goes down.

So compact-via-tags tends to hurt more than help with modern models. If there's too much history — compress through condense (LLM summary, see below), not through structural rewriting.

## Compact, Condense, Recall: what's the difference

Terminology often gets confused. Let's fix it:

| Strategy | Cost | Cache impact | When to use |
|---|---|---|---|
| **Compact** (structural) | pennies of CPU | full miss + imitation risk | Almost never. Only when you have a closed block and are upfront certain the cache is being busted anyway. |
| **Condense** (LLM) | one LLM call | full miss (full history replacement) | By threshold (≈75-80% of the context window) or on `ContextOverflowError` from the provider. At most once per Run. |
| **Recall** (model-driven) | one tool call | append, cache is unaffected | Only in the block model. The model itself requests an old block when it needs details. |

The **never destroy originals** principle applies equally to all three: condense creates a new history, but the originals (either as blocks or as a snapshot taken before condense) are preserved for recovery and audit.

Details of the condense prompt, threshold logic, and incremental summarization — see [Chapter 13: Context Engineering](../13-context-engineering/README.md#condensation-prompt).

## Long-term memory (across sessions)

Inside a Run, linear (or block) history is in charge. Across sessions it's lost — you need explicit storage.

### What to put in

- **Decisions and their rationale.** "We chose PostgreSQL because we need transactional guarantees" — that's a forever-useful fact.
- **Stable identifiers.** User name, namespace, environment, working directory.
- **Established preferences** — with an explicit `preference` type (see anchoring bias in Ch. 13).
- **Task artifacts.** Links to created resources, migrations, PR IDs.

### What NOT to put in

- **Every turn of conversation.** That's noise, retrieval starts returning garbage.
- **Hypotheses and temporary status.** "The service is failing right now" — will be stale by the next session and will mislead diagnosis.
- **Full session transcripts.** If you really need them — store them separately, not in the same table as facts for retrieval.

### Connection to the lab

Implementation: writing facts to a file/DB, retrieval via embeddings, filtering by type — is practiced in [Lab 11: Memory & Context Engineering](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab11-memory-context).

## Checkpoint and Resume

A long Run can fail halfway through: the process was killed, rate limit ran out, the network dropped. A checkpoint saves state so that on restart you don't begin from scratch.

The basic implementation (structure, save/load, integration with the agent loop) is described in [Chapter 09: Agent Anatomy](../09-agent-architecture/README.md#checkpoint-and-resume). Advanced strategies (granularity, validation, rotation) — in [Chapter 11: State Management](../11-state-management/README.md#advanced-checkpoint-strategies).

In the memory context, three rules matter:

- **Save the original, not the compact.** If the snapshot contains a compact version of the history, after resume you won't be able to recover the details.
- **Save after every significant step** (tool call, user-facing response). For short tasks (2-3 iterations) a checkpoint is overkill. For long ones (10+ iterations) — it's mandatory.
- **Set a TTL** (e.g. 24 hours) so that old snapshots don't accumulate.

### Shared Memory between agents

In multi-agent systems, agents exchange information through a shared store, namespacing data to keep it segregated:

```go
type SharedMemoryStore struct {
    store CheckpointStore
}

func (s *SharedMemoryStore) Put(ctx context.Context, agentID, key string, value any) error {
    fullKey := fmt.Sprintf("shared:%s:%s", agentID, key)
    data, _ := json.Marshal(value)
    return s.store.Set(ctx, fullKey, data, 0)
}

func (s *SharedMemoryStore) Get(ctx context.Context, agentID, key string) (any, error) {
    fullKey := fmt.Sprintf("shared:%s:%s", agentID, key)
    data, err := s.store.Get(ctx, fullKey)
    if err != nil {
        return nil, err
    }
    var result any
    return result, json.Unmarshal(data, &result)
}

func (s *SharedMemoryStore) ListAll(ctx context.Context) (map[string]any, error) {
    keys, _ := s.store.Keys(ctx, "shared:*")
    result := make(map[string]any)
    for _, key := range keys {
        val, _ := s.store.Get(ctx, key)
        var parsed any
        _ = json.Unmarshal(val, &parsed)
        result[key] = parsed
    }
    return result, nil
}
```

> **Link:** For more on managing agent state — see [Chapter 11: State Management](../11-state-management/README.md). A checkpoint is a special case of state persistence.

## Common Errors

### Error 1: No TTL

**Symptom:** Memory grows unbounded, consuming storage and context. Retrieval returns stale facts.

**Cause:** Stale information is never forgotten.

**Solution:** Implement a TTL and periodic cleanup. For different fact types — different TTLs: user preferences live for a long time, temporary status — hours.

### Error 2: Saving everything

**Symptom:** Memory fills up with irrelevant information, retrieval becomes noisy.

**Cause:** No filter for what's actually worth saving.

**Solution:** Save only important facts, not every turn of conversation. A good filter is a separate lightweight LLM call at the end of the Run: "extract from this dialogue the facts that are worth remembering long-term".

### Error 3: Keyword-only search

**Symptom:** Retrieval returns irrelevant results or misses important information.

**Cause:** Simple substring matching doesn't understand synonyms and paraphrases.

**Solution:** Use embeddings for semantic search. Hybrid (BM25 + embeddings) usually beats either one on its own.

### Error 4: Compact destroys the originals

**Symptom:** After compaction it's impossible to restore the details of tool calls. Recall returns the compressed version.

**Cause:** Original messages were deleted when the compact version was created.

**Solution:**

```go
// BAD: overwrote the original
block.Messages = compactMessages(block.Messages)

// GOOD: original is alive, compact is a separate view
block.Compact = compactMessages(block.Messages)
// block.Messages is untouched
```

The **never destroy originals** principle: any form of compression is a view, not a mutation. Originals are needed for recall, for recovery, and for audit.

### Error 5: Compact or condense mid-loop

**Symptom:** The agent loses context mid-task, starts over or repeats actions. On the next iteration the model doesn't see what was just discussed.

**Cause:** Compression is called inside the agent loop while the task is still executing.

**Solution:** Compact — only when closing a block. Condense — by threshold or on `ContextOverflowError`, and not more than once per Run. Don't touch the history of a live task.

### Error 6: Live state in the system prompt

**Symptom:** On every iteration the prompt cache hit rate is ≈ 0%, latency grows linearly with history length, cost is 3-5× of what's expected. The `cached_tokens` metric from the provider hovers near zero.

**Cause:** Changing data (current file, files read, plan progress) is written into the system prompt. Any mutation invalidates the entire prefix.

**Solution:** Keep the system prompt stable. Fix stable inclusions (date, working directory) once at the start of the Run and never change them again. Put live state into tool results, into Notes on the last message, or into special tool calls (`update_plan`, `set_goal`) that the model makes itself.

### Error 7: Estimating tokens via char/3

**Symptom:** Threshold-condense fires at the wrong time — sometimes too early (we lose context for no reason), sometimes too late (we get `ContextOverflowError`). Behavior varies a lot between models.

**Cause:** "Length in characters / 3" is an approximation that systematically misses by 30%+. For Russian, for code, and for models with updated tokenizers (e.g. after a vocabulary swap), the miss is even larger.

**Solution:** Take `Usage.PromptTokens` from the provider's response to the previous iteration — it's free and accurate. A char-based estimate is only needed for a new user message that hasn't been sent to the LLM yet.

```go
// BAD
estimate := totalChars / 3
if estimate > threshold { condense() }

// GOOD
budget := lastUsage.PromptTokens + roughEstimate(newUserMessage)
if budget > threshold { condense() }
```

## Mini-Exercises

### Exercise 1: File-backed Memory

Implement a memory store that survives a process restart:

```go
type FileMemory struct {
    filepath string
    // ...
}

func (m *FileMemory) Store(key string, value any, metadata map[string]any) error {
    // Save to a JSON file (atomic write via temp + rename)
}
```

**Expected result:**
- Memory survives restart.
- Writes are atomic (no partially written files on crash).
- There's a separate `Cleanup()` command to walk TTLs.

### Exercise 2: Semantic search

Implement retrieval via embeddings:

```go
func (m *Memory) RetrieveSemantic(query string, limit int) ([]MemoryItem, error) {
    // Encode the query, compute cosine similarity to items, return top-K
}
```

**Expected result:**
- Finds relevant items without exact word matches.
- Returns the most similar ones first.
- Embedding-model failure must not crash Retrieve (errors are logged, the method returns what it managed to get).

### Exercise 3: Linear memory + threshold-condense

Implement `LinearMemory` (Append / Snapshot / Reset) and a guard function:

```go
func shouldCondense(usage llm.Usage, ctxWindow int, threshold float64) bool {
    // true if usage.PromptTokens / ctxWindow >= threshold
}

func condense(ctx context.Context, ep llm.Endpoint, msgs []llm.Message) ([]llm.Message, error) {
    // 1. Split into head (old) and tail (last 2 user-steps)
    // 2. Ask ep to build a summary of head using the prompt from Ch. 13
    // 3. Return [summary as user message] + tail
}
```

**Expected result:**
- Memory is append-only, no mutations other than `Reset`.
- Threshold fires on real `PromptTokens`, not on char/3.
- The original snapshot is preserved until a successful condense (if the LLM call fails, history isn't lost).
- Condense is not called more than once per Run.

## Completion Criteria / Checklist

**Completed:**
- You understand the split "inside a Run" vs. "between sessions".
- You use linear memory by default; you switch to block memory deliberately.
- You don't mutate the system prompt with live state.
- You distinguish compact, condense, and recall by applicability.
- You count tokens via `Usage.PromptTokens`, not via char/3.
- You implement a TTL and filtering for long-term memory.
- You follow "never destroy originals".

**Not completed:**
- Compact or condense are called mid-loop.
- Live state lives in the system prompt.
- char/3 is the only token counter.
- Compact destroys the originals.
- Long-term memory has no TTL and no filter on what to put in it.
- Block memory is used without an explicit reason (like REPL or model-driven recall).

## Connection with Other Chapters

- **[Chapter 09: Agent Anatomy](../09-agent-architecture/README.md)** — memory as one of the key agent components, connection to the runtime.
- **[Chapter 11: State Management](../11-state-management/README.md)** — checkpoints, idempotency, persistent state.
- **[Chapter 13: Context Engineering](../13-context-engineering/README.md)** — assembling context from memory, budgets, condense, dynamic sections of the system prompt.
- **[Chapter 20: Cost & Latency Engineering](../20-cost-latency-engineering/README.md)** — why prompt cache matters so much and how not to break it.

> **Boundary:** this chapter owns **storing and retrieving** information. Managing how that information is assembled into the LLM context — in [Context Engineering](../13-context-engineering/README.md).

## What's Next?

After understanding memory systems, move on to:
- **[13. Context Engineering](../13-context-engineering/README.md)** — learn to assemble context from memory, state, and retrieval without breaking prompt cache.
