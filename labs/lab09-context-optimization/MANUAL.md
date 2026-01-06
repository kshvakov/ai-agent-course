# Method Guide: Lab 09 — Context Optimization

## Why Is This Needed?

In this lab you'll learn to manage the LLM context window — a critical skill for creating long-lived agents.

### Real-World Case Study

**Situation:** You created an autonomous DevOps agent that resolves incidents. Agent works in a loop:
1. Checks metrics
2. Analyzes logs
3. Checks configuration
4. Fixes problem
5. Checks again

After 15 steps context overflows (4000 tokens), and agent:
- Forgets initial task
- Repeats already executed steps
- Loses important information

**Problem:** History is too long, doesn't fit in context window.

**Solution:** Context optimization — compress old messages via summarization, preserving important information.

## Theory in Simple Terms

### Context Window Is a Limitation

**Context window** is the maximum number of tokens the model can "see" at once.

**Example limits:**
- GPT-3.5-turbo: 4,096 tokens
- GPT-4: 8,192 tokens
- GPT-4-turbo: 128,000 tokens
- Llama 2: 4,096 tokens

**What's included in context:**
- System Prompt (200-500 tokens)
- Dialogue history (grows with each message)
- Tool calls and their results (50-200 tokens each)
- New user request

### Why Does Context Overflow?

**Example:**
```
Message 1: "Hi! My name is Ivan" (10 tokens)
Message 2: "I work as a DevOps engineer" (8 tokens)
... (18 more messages, 20 tokens each) ...
Message 21: "What's my name?" (5 tokens)

TOTAL: 10 + 8 + (18 × 20) + 5 = 383 tokens
```

But if each message contains tool results (100+ tokens), context quickly overflows.

### Optimization Techniques

#### 1. Token Counting

**Why:** Always know how many tokens are used.

**How:**
- Approximately: 1 token ≈ 4 characters (English), ≈ 3 characters (Russian)
- Precisely: Use `tiktoken` or `tiktoken-go` library

**Example:**
```go
func estimateTokens(text string) int {
    return len(text) / 4  // Approximately
}
```

#### 2. History Truncation

**Why:** Quick solution when history is too long.

**How:** Keep only last N messages.

**Problem:** Lose important information from beginning.

**Example:**
```go
// Keep only last 10 messages
if len(messages) > 10 {
    messages = append(
        []openai.ChatCompletionMessage{messages[0]},  // System
        messages[len(messages)-9:]...,  // Last 9
    )
}
```

#### 3. Summarization

**Why:** Preserve important information, compress old messages.

**How:** Use LLM to create brief summary.

**What to preserve:**
- Important facts (user name, role)
- Decisions (what was done)
- Current task state

**Example:**
```
Original history (2000 tokens):
- User: "My name is Ivan"
- Assistant: "Hi, Ivan!"
- User: "I'm a DevOps engineer"
- Assistant: "Great!"
... (50 more messages)

Compressed version (200 tokens):
Summary: "User Ivan, DevOps engineer. Discussed server setup.
Current task: checking monitoring."
```

#### 4. Adaptive Management

**Why:** Choose technique based on situation.

**Logic:**
- < 80% fill → do nothing
- 80-90% → prioritization (preserve important messages)
- > 90% → summarization (compress old messages)

## Execution Algorithm

### Step 1: Token Counting

```go
func estimateTokens(text string) int {
    // Approximate estimate
    // For Russian: 1 token ≈ 3 characters
    // For English: 1 token ≈ 4 characters
    return len(text) / 4
}

func countTokensInMessages(messages []openai.ChatCompletionMessage) int {
    total := 0
    for _, msg := range messages {
        total += estimateTokens(msg.Content)
        // Tool calls also take tokens
        if len(msg.ToolCalls) > 0 {
            total += len(msg.ToolCalls) * 80  // Approximately 80 tokens per call
        }
    }
    return total
}
```

### Step 2: History Truncation

```go
func truncateHistory(messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
    if len(messages) == 0 {
        return messages
    }
    
    systemMsg := messages[0]
    result := []openai.ChatCompletionMessage{systemMsg}
    currentTokens := estimateTokens(systemMsg.Content)
    
    // Go from end and add messages until limit reached
    for i := len(messages) - 1; i > 0; i-- {
        msgTokens := estimateTokens(messages[i].Content)
        if len(messages[i].ToolCalls) > 0 {
            msgTokens += len(messages[i].ToolCalls) * 80
        }
        
        if currentTokens + msgTokens > maxTokens {
            break
        }
        
        // Add to beginning of result
        result = append([]openai.ChatCompletionMessage{messages[i]}, result...)
        currentTokens += msgTokens
    }
    
    return result
}
```

### Step 3: Summarization

```go
func summarizeMessages(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage) string {
    // Collect text of all messages (except System)
    conversation := ""
    for i := 1; i < len(messages); i++ {
        msg := messages[i]
        role := "User"
        if msg.Role == openai.ChatMessageRoleAssistant {
            role = "Assistant"
        } else if msg.Role == openai.ChatMessageRoleTool {
            role = "Tool"
        }
        conversation += fmt.Sprintf("%s: %s\n", role, msg.Content)
    }
    
    // Create summarization prompt
    summaryPrompt := fmt.Sprintf(`Summarize this conversation, keeping only:
1. Important facts about the user (name, role, preferences)
2. Key decisions made
3. Current state of the task

Conversation:
%s`, conversation)
    
    // Call LLM
    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: openai.GPT3Dot5Turbo,
        Messages: []openai.ChatCompletionMessage{
            {
                Role:    openai.ChatMessageRoleSystem,
                Content: "You are a conversation summarizer. Create concise summaries that preserve important facts.",
            },
            {
                Role:    openai.ChatMessageRoleUser,
                Content: summaryPrompt,
            },
        },
        Temperature: 0,  // Deterministic summarization
    })
    
    if err != nil {
        return fmt.Sprintf("Error summarizing: %v", err)
    }
    
    return resp.Choices[0].Message.Content
}
```

### Step 4: Context Compression

```go
func compressOldMessages(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
    if len(messages) <= 10 {
        return messages  // Nothing to compress
    }
    
    systemMsg := messages[0]
    oldMessages := messages[1 : len(messages)-10]  // All except last 10
    recentMessages := messages[len(messages)-10:]   // Last 10
    
    // Compress old messages
    summary := summarizeMessages(ctx, client, oldMessages)
    
    // Build new context
    compressed := []openai.ChatCompletionMessage{
        systemMsg,
        {
            Role:    openai.ChatMessageRoleSystem,
            Content: fmt.Sprintf("Summary of previous conversation:\n%s", summary),
        },
    }
    compressed = append(compressed, recentMessages...)
    
    return compressed
}
```

### Step 5: Prioritization

```go
func prioritizeMessages(messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
    if len(messages) == 0 {
        return messages
    }
    
    important := []openai.ChatCompletionMessage{messages[0]}  // System
    
    // Always preserve last 5 messages
    startIdx := len(messages) - 5
    if startIdx < 1 {
        startIdx = 1
    }
    
    for i := startIdx; i < len(messages); i++ {
        msg := messages[i]
        important = append(important, msg)
    }
    
    // Preserve tool results and errors from old messages
    for i := 1; i < startIdx; i++ {
        msg := messages[i]
        if msg.Role == openai.ChatMessageRoleTool {
            important = append(important, msg)
        } else if strings.Contains(strings.ToLower(msg.Content), "error") {
            important = append(important, msg)
        }
    }
    
    return important
}
```

### Step 6: Adaptive Management

```go
func adaptiveContextManagement(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
    usedTokens := countTokensInMessages(messages)
    
    if usedTokens < threshold80 {
        // All good, do nothing
        return messages
    } else if usedTokens < threshold90 {
        // Apply light optimization: prioritization
        return prioritizeMessages(messages, maxTokens)
    } else {
        // Critical! Apply summarization
        return compressOldMessages(ctx, client, messages, maxTokens)
    }
}
```

## Common Errors

### Error 1: Incorrect Token Counting

**Symptom:** Context overflows earlier than expected.

**Cause:** Tool calls not accounted for, which take many tokens.

**Solution:**
```go
// Account for Tool calls
if len(msg.ToolCalls) > 0 {
    total += len(msg.ToolCalls) * 80
}
```

### Error 2: Summarization Loses Important Information

**Symptom:** Agent forgets user name after summarization.

**Cause:** Summarization prompt doesn't specify what to preserve.

**Solution:** Explicitly specify in prompt what to preserve:
```
"Keep important facts about the user (name, role, preferences)"
```

### Error 3: Summarization Called Too Often

**Symptom:** Slow agent work due to frequent LLM calls for summarization.

**Cause:** Summarization applied on every request.

**Solution:** Use adaptive management — summarization only at > 90% fill.

### Error 4: System Prompt Lost During Truncation

**Symptom:** Agent forgets its role.

**Cause:** System Prompt not preserved during truncation.

**Solution:** Always preserve `messages[0]` (System Prompt).

## Mini-Exercises

### Exercise 1: Accurate Token Counting

Use `github.com/pkoukk/tiktoken-go` library for accurate counting:

```go
import "github.com/pkoukk/tiktoken-go"

func countTokensAccurate(text string, model string) int {
    enc, _ := tiktoken.EncodingForModel(model)
    tokens := enc.Encode(text, nil, nil)
    return len(tokens)
}
```

### Exercise 2: Testing on Long Dialogue

Create a test with 30+ messages and ensure:
- Context doesn't overflow
- Agent remembers beginning of conversation
- Summarization works correctly

## Completion Criteria

✅ **Completed:**
- Token counting implemented (approximate or accurate)
- History truncation implemented
- Summarization via LLM implemented
- Adaptive management implemented
- Agent remembers beginning of conversation after optimization
- Context doesn't overflow in long dialogue (20+ messages)

❌ **Not completed:**
- Context overflows
- Agent forgets important information after optimization
- Summarization doesn't work or works incorrectly
- System Prompt lost during truncation
- Code doesn't compile

---

**Next step:** After successfully completing Lab 09 you've mastered all key agent techniques! You can proceed to study [Multi-Agent Systems](../lab08-multi-agent/README.md) or [RAG](../lab07-rag/README.md).
