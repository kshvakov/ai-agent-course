# Lab 11: Memory and Context Management

## Goal
Learn how to implement memory systems: long-term storage, retrieval, fact extraction, and context layers.

## Theory

### Memory Systems

Agents need memory for:
- Remembering past conversations
- Storing important facts
- Retrieving relevant information
- Efficient context management

### Types of Memory

**Working Memory (Short-term):**
- Recent conversation turns
- Current task context
- Limited by context window

**Long-term Memory:**
- Important facts extracted from conversations
- User preferences
- Past decisions and outcomes

### Context Layers

Efficient context management uses layers:
- **Working memory layer** — recent turns (last 5-10 messages)
- **Summary layer** — summarized history (compressed old messages)
- **Facts layer** — extracted important facts

### Fact Extraction

Not all information is equally important:
- User name, preferences → important (store)
- Temporary status → less important (can forget)
- Decisions made → important (store)

LLM can extract facts from conversation:
- "User's name is Ivan" → Fact
- "Server is running" → Temporary status (not a fact)

### Summarization

When context overflows, summarize old messages:
- Preserve important information
- Reduce token count
- Maintain context continuity

See more:
- [Chapter 12: Agent Memory Systems](../../book/12-agent-memory/README.md)
- [Chapter 13: Context Engineering](../../book/13-context-engineering/README.md)

## Task

In `main.go` implement a memory system with context management.

### Part 1: Memory Storage

Implement `Memory` interface:
- `Store(key string, value any, importance int) error` — store fact
- `Retrieve(query string, limit int) ([]MemoryItem, error)` — search facts
- `Forget(key string) error` — delete fact

**Storage format:**
- Use file-based storage (JSON)
- Or in-memory map (for testing)

### Part 2: Fact Extraction

Implement function `extractFacts(conversation string) ([]Fact, error)`:
- Use LLM to extract important facts from conversation
- Rate facts by importance
- Store facts separately from conversation history

**Example:**
```
Conversation: "Hi, I'm Ivan. I work at TechCorp. We use Docker."
Extracted facts:
  - User name: Ivan
  - User company: TechCorp
  - Tech stack: Docker
```

### Part 3: Context Layers

Implement `LayeredContext`:
- **Working memory** — recent turns (last 5-10 messages)
- **Summary layer** — summarized history (compressed old messages)
- **Facts layer** — extracted facts (retrieved by relevance)

**Context assembly:**
```
Final context = System Prompt + Facts Layer + Summary Layer + Working Memory
```

### Part 4: Summarization

Implement function `summarizeConversation(messages []openai.ChatCompletionMessage) (string, error)`:
- Use LLM to create summary
- Preserve key facts (user name, decisions, important events)
- Reduce token count (2000 tokens → 200 tokens)

**Summary prompt example:**
```
Summarize this conversation, keeping only:
1. Important decisions made
2. Key facts discovered (user name, preferences)
3. Current state of the task
Conversation: [messages]
```

## Important

- Extract facts only when important (not every message)
- Summarize when context exceeds 80% of limit
- Preserve System Prompt always
- Retrieve relevant facts based on current query

## Completion Criteria

✅ **Completed:**
- Memory storage implemented
- Fact extraction works
- Context layers implemented
- Summarization reduces tokens
- Agent remembers important facts
- Facts retrieved by relevance

❌ **Not completed:**
- No memory persistence
- Facts not extracted
- Context not layered
- Summarization loses important information
- Facts not retrieved by relevance

---

**Next step:** After completing Lab 11, proceed to [Lab 12: Tool Server Protocol](../lab12-tool-server/README.md).
