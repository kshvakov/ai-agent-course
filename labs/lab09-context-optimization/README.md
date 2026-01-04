# Lab 09: Context Optimization

## Goal
Learn to manage LLM context window: count tokens, apply optimization techniques (truncation, summarization), and implement adaptive context management.

## Theory

### Context Overflow Problem

When an agent works in a long dialogue or executes many steps in an autonomous loop, message history grows. Sooner or later, it doesn't fit in the model's context window (e.g., 4k tokens for GPT-3.5-turbo).

**What happens:**
- Model doesn't see the start of conversation
- Model "forgets" important details
- API returns error "context length exceeded"

### Optimization Techniques

1. **Token counting** — always know how many tokens are used
2. **Truncation** — keep only last N messages
3. **Summarization** — compress old messages via LLM
4. **Adaptive management** — choose technique based on fill level

See more: [Chapter 03: Context Optimization](../../docs/book/03-agent-architecture/README.md#оптимизация-контекста-context-optimization)

## Assignment

In the `main.go` file, implement a context optimization system for long dialogue.

### Part 1: Token Counting

Implement functions:
- `estimateTokens(text string) int` — approximate estimate (1 token ≈ 4 characters)
- `countTokensInMessages(messages []openai.ChatCompletionMessage) int` — count tokens in entire history

### Part 2: History Truncation

Implement function `truncateHistory(messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage`:
- Always preserve System Prompt
- Keep last messages until limit is reached

### Part 3: Summarization

Implement function `summarizeMessages(messages []openai.ChatCompletionMessage) string`:
- Use LLM to create brief summary of old messages
- Preserve important facts, decisions, current task state

### Part 4: Adaptive Management

Implement function `adaptiveContextManagement(messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage`:
- If context < 80% — do nothing
- If 80-90% — apply prioritization (preserve important messages)
- If > 90% — apply summarization

### Testing Scenario

Run a long dialogue (20+ messages) and ensure that:
1. Agent continues to remember the start of conversation (via summarization)
2. Context doesn't overflow
3. Agent correctly answers questions about early messages

## Example

```
User: Hello! My name is Ivan, I work as a DevOps engineer.
Assistant: Hello, Ivan! How can I help?

... (many messages) ...

User: Do you remember my name?
Assistant: Yes, of course! Your name is Ivan, you're a DevOps engineer.
```

**Without optimization:** After 20 messages, context will overflow, agent will forget the name.

**With optimization:** Old messages are compressed into summary, but important information (name, role) is preserved.

## Important

- Use `OPENAI_BASE_URL` for local models
- For summarization, you can use the same model or a cheaper/faster one
- Test on real long dialogues (20+ messages)

## Completion Criteria

✅ **Completed:**
- Token counting implemented
- History truncation implemented
- Summarization via LLM implemented
- Adaptive management implemented
- Agent remembers start of conversation after optimization
- Context doesn't overflow

❌ **Not completed:**
- Context overflows
- Agent forgets important information after optimization
- Summarization doesn't work
- Code doesn't compile

---

**Next step:** After successfully completing Lab 09, you've mastered all key agent techniques! You can proceed to study [Multi-Agent Systems](../lab08-multi-agent/README.md) or [RAG](../lab07-rag/README.md).
