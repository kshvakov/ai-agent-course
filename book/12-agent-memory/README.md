# 12. Agent Memory Systems

## Why This Chapter?

Agents need memory to keep context across conversations, learn from past interactions, and avoid repeating mistakes. Without it, they forget important details and waste tokens re-explaining the same things.

This chapter covers memory systems that help agents remember, retrieve, and forget information efficiently.

### Real-World Case Study

**Situation:** User asks agent: "What was the database problem we fixed last week?" Agent responds: "I don't have information about that."

**Problem:** Agent has no memory of past conversations. Each interaction starts from scratch, wasting context and user time.

**Solution:** A memory system stores important facts, retrieves them when needed, and forgets outdated information so the agent stays within context limits.

## Theory in Simple Terms

### Memory Types

**Short-term memory:**
- Current conversation history (stored in runtime, not in long-term storage)
- Limited by the LLM context window
- Lost when the conversation ends
- **Note:** Short-term memory management (summarization, selection) is described in [Context Engineering](../13-context-engineering/README.md). The term "working memory" is used in Context Engineering to denote recent conversation turns in context.

**Long-term memory (persistent storage):**
- Facts, preferences, past decisions
- Stored in database/files
- Persists between conversations

**Episodic memory:**
- Specific events: "User asked about disk space on 2026-01-06"
- Useful for debugging and learning

**Semantic memory:**
- General knowledge: "User prefers JSON responses"
- Extracted from episodes

### Memory Operations

1. **Store** — Save information for future
2. **Retrieve** — Find relevant information
3. **Forget** — Delete outdated information
4. **Update** — Change existing information

## How It Works (Step by Step)

### Step 1: Memory Interface

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
    TTL      time.Duration // Time to live
}
```

### Step 2: Storing Information

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
        TTL:      24 * time.Hour, // Default TTL
    }
    return nil
}
```

### Step 3: Retrieval with Search

```go
func (m *SimpleMemory) Retrieve(query string, limit int) ([]MemoryItem, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    // Simple keyword search (in production use embeddings)
    results := make([]MemoryItem, 0, len(m.store))
    queryLower := strings.ToLower(query)
    
    for _, item := range m.store {
        // Check if expired
        if item.TTL > 0 && time.Since(item.Created) > item.TTL {
            continue
        }
        
        // Simple keyword matching
        valueStr := fmt.Sprintf("%v", item.Value)
        if strings.Contains(strings.ToLower(valueStr), queryLower) {
            item.Accessed = time.Now() // Update access time
            results = append(results, item)
        }
    }
    
    // Sort by access time (most recent first)
    sort.Slice(results, func(i, j int) bool {
        return results[i].Accessed.After(results[j].Accessed)
    })
    
    if len(results) > limit {
        results = results[:limit]
    }
    
    return results, nil
}
```

### Step 4: Forgetting Expired Items

```go
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

### Step 5: Integration with Agent

```go
func runAgentWithMemory(ctx context.Context, client *openai.Client, memory Memory, userInput string) (string, error) {
    // Retrieve relevant memories
    memories, _ := memory.Retrieve(userInput, 5)
    
    // Build context with memories
    messages := []openai.ChatCompletionMessage{
        {Role: "system", Content: "You are a helpful assistant with access to memory."},
    }
    
    // Add relevant memories as context
    if len(memories) > 0 {
        memoryContext := "Relevant past information:\n"
        for _, mem := range memories {
            memoryContext += fmt.Sprintf("- %s: %v\n", mem.Key, mem.Value)
        }
        messages = append(messages, openai.ChatCompletionMessage{
            Role:    "system",
            Content: memoryContext,
        })
    }
    
    messages = append(messages, openai.ChatCompletionMessage{
        Role:    "user",
        Content: userInput,
    })
    
    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model:    "gpt-4o-mini",
        Messages: messages,
    })
    if err != nil {
        return "", err
    }
    
    answer := resp.Choices[0].Message.Content
    
    // Store important information from conversation
    if shouldStore(userInput, answer) {
        key := generateKey(userInput)
        memory.Store(key, answer, map[string]any{
            "user_input": userInput,
            "timestamp": time.Now(),
        })
    }
    
    return answer, nil
}
```

## Checkpoint and Resume

An agent can work for hours on a complex task. If the process crashes mid-way, all progress is lost. Checkpoint saves the conversation state periodically. On failure, the agent resumes from the last saved point.

The basic Checkpoint implementation (structure, save/load, integration with agent loop) is described in [Chapter 09: Agent Anatomy](../09-agent-architecture/README.md#checkpoint-and-resume). Advanced strategies (granularity, validation, rotation) are covered in [Chapter 11: State Management](../11-state-management/README.md#advanced-checkpoint-strategies).

Here we look at how Checkpoint relates to agent memory:

- **What to save:** message history (`messages[]`), memory contents, tool state, current execution step.
- **When to save:** after each significant step (tool call, user response). For short tasks (2-3 iterations) Checkpoint is overkill. For long tasks (10+ iterations) it's mandatory.
- **TTL:** set a TTL on checkpoints (e.g. 24 hours) so stale snapshots don't accumulate.

### Shared Memory Between Agents

In multi-agent systems, agents exchange information through a shared memory store. Each agent reads and writes to a common store, separating data by namespace:

```go
type SharedMemoryStore struct {
    store CheckpointStore
}

// Write with agent namespace
func (s *SharedMemoryStore) Put(ctx context.Context, agentID, key string, value any) error {
    fullKey := fmt.Sprintf("shared:%s:%s", agentID, key)
    data, _ := json.Marshal(value)
    return s.store.Set(ctx, fullKey, data, 0)
}

// Read another agent's data
func (s *SharedMemoryStore) Get(ctx context.Context, agentID, key string) (any, error) {
    fullKey := fmt.Sprintf("shared:%s:%s", agentID, key)
    data, err := s.store.Get(ctx, fullKey)
    if err != nil {
        return nil, err
    }
    var result any
    return result, json.Unmarshal(data, &result)
}

// List all entries from all agents (for supervisor)
func (s *SharedMemoryStore) ListAll(ctx context.Context) (map[string]any, error) {
    keys, _ := s.store.Keys(ctx, "shared:*")
    result := make(map[string]any)
    for _, key := range keys {
        val, _ := s.store.Get(ctx, key)
        var parsed any
        json.Unmarshal(val, &parsed)
        result[key] = parsed
    }
    return result, nil
}
```

> **Connection:** For more on agent state management, see [Chapter 11: State Management](../11-state-management/README.md). Checkpoint is a special case of state persistence.

## Block Memory

A simple map-based `SimpleMemory` works for learning purposes. In a production agent, memory is more complex. The key idea: **block architecture**.

A Block is a unit of interaction. A user request, a chain of tool calls, the final response — all of that is one block. A block stores both original messages and their compact version.

### Block Lifecycle

```mermaid
flowchart LR
    A[Prepare] --> B[Append]
    B --> C[BuildContext]
    C --> D[Close]
    D --> E["Compact + Summary"]
```

1. **Prepare** — create a new block, bind it to the store
2. **Append** — add messages (user, assistant, tool results)
3. **BuildContext** — assemble history from current and past blocks
4. **Close** — finalize: create compact version, count tokens, generate summary

### Compact: 91% Tool Chain Compression

When a block is closed, tool call chains are collapsed into a compact form:

```go
type Block struct {
    ID       int
    Query    string
    Messages []Message // original
    Compact  []Message // compressed version
    Summary  string
    Tokens   int
}

func (b *Block) Close() {
    b.Query = extractQuery(b.Messages)      // first 120 chars of user message
    b.Compact = compactMessages(b.Messages)
    b.Tokens = estimateTokens(b.Compact)    // chars / 3
    b.Summary = buildSummary(b.Query, b.Messages, b.Compact)
}
```

The `compactMessages` function collapses `assistant(tool_calls) → tool_result` chains into a single message with `<prior_tool_use>` tags:

```go
func compactMessages(msgs []Message) []Message {
    var result []Message
    var toolBuf strings.Builder

    for _, msg := range msgs {
        if msg.HasToolCalls() {
            for _, tc := range msg.ToolCalls {
                args := truncate(tc.Arguments, 80)
                res := truncate(findToolResult(msgs, tc.ID), 200)
                line := fmt.Sprintf("%s %s -- %s", tc.Name, args, res)
                if !isDuplicate(toolBuf.String(), line) {
                    toolBuf.WriteString(line + "\n")
                }
            }
            continue
        }
        if msg.Role == "tool" {
            continue // already processed above
        }
        if toolBuf.Len() > 0 {
            result = append(result, Message{
                Role:    "user", // not assistant — the model must not imitate its own content
                Content: "<prior_tool_use>\n" + toolBuf.String() + "</prior_tool_use>",
            })
            toolBuf.Reset()
        }
        result = append(result, msg)
    }
    return result
}
```

Compact messages are assigned the `user` role, not `assistant`. This prevents the model from perceiving compressed content as its own previous words and starting to imitate them.

### BuildContext: Block Budgeting

When assembling context, we take compact versions of past blocks (from newest to oldest) until the budget runs out:

```go
func (m *Memory) BuildContext(currentBlock *Block, contextSize int) []Message {
    budget := contextSize - currentBlock.Tokens - 4096 // reserve

    var history []Message
    // from newest blocks to oldest
    for i := len(m.blocks) - 1; i >= 0 && budget > 0; i-- {
        block := m.blocks[i]
        if block.Tokens > budget {
            break
        }
        history = append(block.Compact, history...)
        budget -= block.Tokens
    }

    return append(history, currentBlock.Messages...)
}
```

## Block Catalog and Recall

The model does not see past block contents directly. Instead, a **catalog** — a list of blocks with brief descriptions — is rendered in the system prompt:

```
[CONTEXT BLOCKS]
Blocks from prior interactions (use recall tool to get full details):
#0: check disk usage — exec x3, read x1 → "Disk /data at 92%, cleaned logs" (~5K tokens)
#1: deploy service — edit x2, exec x4 → "Deployed v2.3.1 to staging" (~8K tokens)
#2: fix nginx config — read x2, edit x1 → "Updated proxy_pass for /api" (~3K tokens)
```

When the model needs details, it calls the `recall` tool:

```go
type RecallTool struct {
    memory *Memory
}

func (t *RecallTool) Execute(ctx context.Context, args RecallArgs) (string, error) {
    msgs := t.memory.BlockMessages(args.BlockID)
    if msgs == nil {
        return "Block not found", nil
    }
    return formatMessages(msgs), nil // full original messages
}
```

Recall returns **original** block messages, not the compact version. This is critical: compact loses details, and deep analysis requires full information.

## Working Memory

Working Memory is a dynamic section of the system prompt that holds the context of the current task. Unlike block memory (past interactions), Working Memory represents "what the agent knows right now."

### Working Memory Components

| Component | Purpose |
|-----------|---------|
| **TaskContext** | Current task (first 50 words), read/modified files, recent actions |
| **LivePlan** | Goal, steps with statuses (pending/in_progress/completed/cancelled) |
| **Budget** | Warnings about context window filling up |
| **ContextBlocks** | Catalog of past blocks for recall |

### Rendering in System Prompt

```go
type WorkingMemory struct {
    Task          string
    FilesRead     *Ring[string]  // ring buffer, MRU semantics
    FilesModified *Ring[string]
    LastActions   *Ring[string]
    Plan          *LivePlan
    Budget        *BudgetTracker
    Blocks        []BlockSummary
    maxChars      int            // ~6000 chars
}

func (wm *WorkingMemory) Render() string {
    var sb strings.Builder

    sb.WriteString("[TASK CONTEXT]\n")
    sb.WriteString("Task: " + wm.Task + "\n")
    sb.WriteString("Files read: " + wm.FilesRead.Join(", ") + "\n")
    sb.WriteString("Files modified: " + wm.FilesModified.Join(", ") + "\n")
    sb.WriteString("Recent: " + wm.LastActions.Join(", ") + "\n")

    if wm.Plan != nil {
        sb.WriteString("\n" + wm.Plan.Render() + "\n")
    }
    if wm.Budget.ShouldWarn() {
        sb.WriteString("\n[BUDGET]\n" + wm.Budget.Warning() + "\n")
    }
    if len(wm.Blocks) > 0 {
        sb.WriteString("\n[CONTEXT BLOCKS]\n")
        for _, b := range wm.Blocks {
            sb.WriteString(fmt.Sprintf("#%d: %s (~%dK tokens)\n", b.ID, b.Summary, b.Tokens/1000))
        }
    }

    // Trim when over budget
    if sb.Len() > wm.maxChars {
        wm.LastActions.Trim(3)
        wm.FilesRead.Trim(5)
    }

    return sb.String()
}
```

### The Problem: Working Memory Is Not Persistent

Working Memory lives in RAM. Between sessions it is lost: the agent forgets the plan, re-reads files, and starts from scratch.

Solution: pass Working Memory as a parameter to `Run()` so it survives across REPL cycles. For persistence between sessions, implement Export/Import:

```go
func (wm *WorkingMemory) Export() WorkingMemorySnapshot {
    return WorkingMemorySnapshot{
        Task:          wm.Task,
        FilesRead:     wm.FilesRead.Items(),
        FilesModified: wm.FilesModified.Items(),
        Plan:          wm.Plan.Export(),
    }
}

func (wm *WorkingMemory) Restore(snap WorkingMemorySnapshot) {
    wm.Task = snap.Task
    for _, f := range snap.FilesRead {
        wm.FilesRead.Push(f)
    }
    // ...
}
```

> **4 levels of memory in a production agent:**
>
> 1. **Working Memory** — task, plan, budget, files (in system prompt)
> 2. **Block Memory** — completed interactions (original + compact)
> 3. **Recall** — model-driven retrieval of full block data
> 4. **Condense** — emergency LLM compression on overflow (see [Context Engineering](../13-context-engineering/README.md))

## Common Errors

### Error 1: No TTL (Time To Live)

**Symptom:** Memory grows infinitely, consuming storage and context.

**Cause:** Outdated information is not forgotten.

**Solution:** Implement TTL and periodic cleanup.

### Error 2: Storing Everything

**Symptom:** Memory fills with irrelevant information, making retrieval noisy.

**Cause:** No filtering of what to store.

**Solution:** Store only important facts, not every conversation turn.

### Error 3: No Retrieval Optimization

**Symptom:** Retrieval returns irrelevant results or misses important information.

**Cause:** Simple keyword matching instead of semantic search.

**Solution:** Use embeddings for semantic similarity search.

### Error 4: Compact Destroys Originals

**Symptom:** After compaction, it is impossible to recover tool call details. Recall returns the compressed version.

**Cause:** Original messages were deleted when creating the compact version.

**Solution:**

```go
// BAD: overwriting the original
block.Messages = compactMessages(block.Messages)

// GOOD: store both versions
block.Compact = compactMessages(block.Messages)
// block.Messages remains unchanged
```

The **Never destroy originals** principle: condensation is a view, not a mutation. Originals are needed for recall over the full history.

### Error 5: Programmatic Compaction Mid-Loop

**Symptom:** Agent loses context in the middle of a task, restarts or repeats actions.

**Cause:** Compaction is triggered inside the agent loop while the task is still running.

**Solution:** Compact only on `Block.Close()`, when the interaction is complete. Inside the loop, use LLM condensation (condense), which creates a meaningful summary instead of mechanical compression.

## Mini-Exercises

### Exercise 1: Implement Memory Store

Create a memory store that persists to disk:

```go
type FileMemory struct {
    filepath string
    // Your implementation
}

func (m *FileMemory) Store(key string, value any, metadata map[string]any) error {
    // Save to file
}
```

**Expected result:**
- Memory persists between restarts
- Can load from file on startup

### Exercise 2: Semantic Search

Implement retrieval using embeddings:

```go
func (m *Memory) RetrieveSemantic(query string, limit int) ([]MemoryItem, error) {
    // Use embeddings to find semantically similar items
}
```

**Expected result:**
- Finds relevant items even without exact keyword match
- Returns most similar items first

### Exercise 3: Implement Block Memory with Compact

Implement block memory with tool chain compaction:

```go
type BlockMemory struct {
    blocks  []*Block
    current *Block
}

func (m *BlockMemory) NewBlock() *Block {
    // Create a new block
}

func (m *BlockMemory) CloseBlock() {
    // Close current block: compact + summary
}

func (m *BlockMemory) BuildContext(contextSize int) []Message {
    // Assemble history from compact versions of past blocks + current block
}
```

**Expected result:**
- Compact compresses a chain of 10 tool calls into 1-2 messages
- BuildContext fits within the token budget
- Original messages are preserved for recall

## Completion Criteria / Checklist

**Completed:**
- [x] Understand different memory types
- [x] Can store and retrieve information
- [x] Implement TTL and cleanup
- [x] Integrate memory with agent
- [x] Understand block memory architecture and compaction
- [x] Can implement block catalog and recall
- [x] Understand Working Memory and its components

**Not completed:**
- [ ] No TTL, memory grows infinitely
- [ ] Storing everything without filtering
- [ ] Only simple keyword search
- [ ] Compact destroys original messages
- [ ] Compaction triggered mid-loop instead of on block close

## Connection with Other Chapters

- **[Chapter 09: Agent Anatomy](../09-agent-architecture/README.md)** — Memory is a key agent component
- **[Chapter 13: Context Engineering](../13-context-engineering/README.md)** — Memory feeds context management (facts from memory are used when assembling context)
- **[Chapter 08: Evals and Reliability](../08-evals-and-reliability/README.md)** — Memory affects agent consistency

**IMPORTANT:** Memory (this chapter) is responsible for **storing and retrieving** information. Managing how this information is included in LLM context is described in [Context Engineering](../13-context-engineering/README.md).

## What's Next?

After understanding memory systems, proceed to:
- **[13. Context Engineering](../13-context-engineering/README.md)** — Learn how to efficiently manage context from memory, state, and retrieval

