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

## Completion Criteria / Checklist

**Completed:**
- [x] Understand different memory types
- [x] Can store and retrieve information
- [x] Implement TTL and cleanup
- [x] Integrate memory with agent

**Not completed:**
- [ ] No TTL, memory grows infinitely
- [ ] Storing everything without filtering
- [ ] Only simple keyword search

## Connection with Other Chapters

- **[Chapter 09: Agent Anatomy](../09-agent-architecture/README.md)** — Memory is a key agent component
- **[Chapter 13: Context Engineering](../13-context-engineering/README.md)** — Memory feeds context management (facts from memory are used when assembling context)
- **[Chapter 08: Evals and Reliability](../08-evals-and-reliability/README.md)** — Memory affects agent consistency

**IMPORTANT:** Memory (this chapter) is responsible for **storing and retrieving** information. Managing how this information is included in LLM context is described in [Context Engineering](../13-context-engineering/README.md).

## What's Next?

After understanding memory systems, proceed to:
- **[13. Context Engineering](../13-context-engineering/README.md)** — Learn how to efficiently manage context from memory, state, and retrieval

