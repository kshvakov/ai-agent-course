# Study Guide: Lab 01 — Hello, LLM!

## Why This Lab?

In this laboratory assignment, you'll learn the basics of interacting with LLMs: sending requests, receiving responses, and most importantly, **context management**. Without saving context (message history), it's impossible to build a dialogue.

### Real-World Case Study

**Situation:** You've created a chatbot for customer support. User writes:
- "I have a login problem"
- Bot responds: "Describe the problem in detail"
- User: "I forgot my password"
- Bot: "Describe the problem in detail" (again!)

**Problem:** Bot doesn't remember previous messages.

**Solution:** Send the entire dialogue history in each request.

## Theory in Simple Terms

### LLM is a Stateless System

**Stateless** means "without state". Each request to the model is a new request. It doesn't remember what you wrote a second ago.

To create the illusion of dialogue, we send the **entire** list of previous messages (history) every time.

### Message Structure

A message consists of:
- **Role:** `system`, `user`, `assistant`
- **Content:** Message text

**Example:**

```go
messages := []ChatCompletionMessage{
    {Role: "system", Content: "You are an experienced Linux administrator"},
    {Role: "user", Content: "How to check service status?"},
    {Role: "assistant", Content: "Use command systemctl status nginx"},
    {Role: "user", Content: "How to restart it?"},
}
```

The model sees the full history and understands context ("it" = nginx).

## Execution Algorithm

### Step 1: Client Initialization

```go
config := openai.DefaultConfig(token)
if baseURL != "" {
    config.BaseURL = baseURL  // For local models
}
client := openai.NewClientWithConfig(config)
```

### Step 2: Creating History

```go
messages := []openai.ChatCompletionMessage{
    {
        Role:    openai.ChatMessageRoleSystem,
        Content: "You are an experienced Linux administrator. Answer briefly and to the point.",
    },
}
```

**Important:** System Prompt sets the agent's role. This affects response style.

### Step 3: Chat Loop

```go
for {
    // 1. Read user input
    input := readUserInput()
    
    // 2. Add to history
    messages = append(messages, openai.ChatCompletionMessage{
        Role:    openai.ChatMessageRoleUser,
        Content: input,
    })
    
    // 3. Send ENTIRE history to API
    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model:    openai.GPT3Dot5Turbo,
        Messages: messages,  // Full history!
    })
    
    // 4. Get response
    answer := resp.Choices[0].Message.Content
    
    // 5. Save response to history
    messages = append(messages, openai.ChatCompletionMessage{
        Role:    openai.ChatMessageRoleAssistant,
        Content: answer,
    })
}
```

## Common Mistakes

### Mistake 1: History Not Saved

**Symptom:** Agent doesn't remember previous messages.

**Cause:** You're not adding the assistant's response to history.

**Solution:**
```go
// BAD
messages = append(messages, userMessage)
resp := client.CreateChatCompletion(...)
answer := resp.Choices[0].Message.Content
// History not updated!

// GOOD
messages = append(messages, userMessage)
resp := client.CreateChatCompletion(...)
messages = append(messages, resp.Choices[0].Message)  // Save response!
```

### Mistake 2: System Prompt Doesn't Work

**Symptom:** Agent responds in wrong style.

**Cause:** System Prompt not added or not added at the start.

**Solution:**
```go
// System Prompt must be FIRST message
messages := []openai.ChatCompletionMessage{
    {Role: "system", Content: "You are a DevOps engineer"},  // First!
    {Role: "user", Content: "..."},
}
```

### Mistake 3: Context Overflow

**Symptom:** After N messages, agent "forgets" the start of conversation.

**Cause:** History too long, doesn't fit in context window.

**Solution:**
```go
// History truncation (keep only last N messages)
if len(messages) > maxHistoryLength {
    // Keep System Prompt + last N-1 messages
    messages = append(
        []openai.ChatCompletionMessage{messages[0]},  // System
        messages[len(messages)-maxHistoryLength+1:]...,  // Last ones
    )
}
```

## Mini-Exercises

### Exercise 1: Change Role

Try different System Prompts:
- "You are a polite assistant"
- "You are a strict teacher"
- "You are a friendly colleague"

Observe how response style changes.

### Exercise 2: Add Token Counter

Count how many tokens are used in history:

```go
import "github.com/sashabaranov/go-openai"

// Approximate estimate (1 token ≈ 4 characters)
tokenCount := 0
for _, msg := range messages {
    tokenCount += len(msg.Content) / 4
}
fmt.Printf("Tokens used: %d\n", tokenCount)
```

## Completion Criteria

✅ **Completed:**
- Agent remembers previous messages
- System Prompt affects response style
- Code compiles and works

❌ **Not completed:**
- Agent doesn't remember context
- System Prompt is ignored
- Code doesn't compile

---

**Next step:** After successfully completing Lab 01, proceed to [Lab 02: Tools](../lab02-tools/README.md)
