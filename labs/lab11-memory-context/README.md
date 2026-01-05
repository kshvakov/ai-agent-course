# Lab 11: Memory and Context Management

## Goal
Learn to implement memory systems: long-term storage, retrieval, fact extraction, and context layers.

## Theory

### Memory Systems

Agents need memory for:
- Remembering past conversations
- Storing important facts
- Retrieving relevant information
- Efficient context management

See more:
- [Chapter 12: Agent Memory Systems](../../book/12-agent-memory/README.md)
- [Chapter 13: Context Engineering](../../book/13-context-engineering/README.md)

## Task

In `main.go` implement a memory system with context management.

### Part 1: Memory Storage

Implement `Memory` interface:
- `Store(key string, value any) error`
- `Retrieve(query string, limit int) ([]MemoryItem, error)`
- `Forget(key string) error`

### Part 2: Fact Extraction

Implement function `extractFacts(conversation string) ([]Fact, error)`:
- Use LLM to extract important facts
- Rate facts by importance
- Store facts separately

### Part 3: Context Layers

Implement `LayeredContext`:
- Working memory (recent turns)
- Summary layer (summarized history)
- Facts layer (extracted facts)

### Part 4: Summarization

Implement function `summarizeConversation(messages []openai.ChatCompletionMessage) (string, error)`:
- Use LLM to create summary
- Preserve key facts
- Reduce token count

## Completion Criteria

✅ **Completed:**
- Memory storage implemented
- Fact extraction works
- Context layers implemented
- Summarization reduces tokens
- Agent remembers important facts

❌ **Not completed:**
- No memory persistence
- Facts not extracted
- Context not layered
- Summarization loses important information

---

**Next step:** After completing Lab 11, proceed to [Lab 12: Tool Server Protocol](../lab12-tool-server/README.md).
