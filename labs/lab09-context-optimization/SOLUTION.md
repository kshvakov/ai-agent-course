# Lab 09 Solution: Context Optimization

## 🎯 Goal
In this lab we learned to manage the LLM context window: count tokens, apply optimization techniques (truncation, summarization) and implement adaptive context management.

## 📝 Solution Breakdown

### 1. Token Counting

**Approximate counting:**
```go
func estimateTokens(text string) int {
    // For Russian: 1 token ≈ 3 characters
    // For English: 1 token ≈ 4 characters
    // Use average value
    return len(text) / 4
}
```

**Counting in all messages:**
```go
func countTokensInMessages(messages []openai.ChatCompletionMessage) int {
    total := 0
    for _, msg := range messages {
        total += estimateTokens(msg.Content)
        // Tool calls also take tokens (approximately 80 tokens per call)
        if len(msg.ToolCalls) > 0 {
            total += len(msg.ToolCalls) * 80
        }
    }
    return total
}
```

### 2. History Truncation

```go
func truncateHistory(messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
    if len(messages) == 0 {
        return messages
    }
    
    // Always preserve System Prompt
    systemMsg := messages[0]
    result := []openai.ChatCompletionMessage{systemMsg}
    currentTokens := estimateTokens(systemMsg.Content)
    
    // Go from end and add messages until limit reached
    for i := len(messages) - 1; i > 0; i-- {
        msg := messages[i]
        msgTokens := estimateTokens(msg.Content)
        
        // Account for Tool calls
        if len(msg.ToolCalls) > 0 {
            msgTokens += len(msg.ToolCalls) * 80
        }
        
        if currentTokens + msgTokens > maxTokens {
            break
        }
        
        // Add to beginning of result (to preserve order)
        result = append([]openai.ChatCompletionMessage{msg}, result...)
        currentTokens += msgTokens
    }
    
    return result
}
```

### 3. Summarization

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
1. Important facts about the user (name, role, preferences, context)
2. Key decisions made
3. Current state of the task or conversation

Conversation:
%s`, conversation)
    
    // Call LLM for summarization
    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: "gpt-4o-mini",
        Messages: []openai.ChatCompletionMessage{
            {
                Role:    openai.ChatMessageRoleSystem,
                Content: "You are a conversation summarizer. Create concise summaries that preserve important facts about the user and the current state of the conversation.",
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

### 4. Context Compression

```go
func compressOldMessages(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
    if len(messages) <= 10 {
        return messages
    }
    
    systemMsg := messages[0]
    oldMessages := messages[1 : len(messages)-10]  // All except last 10
    recentMessages := messages[len(messages)-10:]   // Last 10
    
    summary := summarizeMessages(ctx, client, oldMessages)
    
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

### 5. Prioritization

```go
func prioritizeMessages(messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
    if len(messages) == 0 {
        return messages
    }
    
    important := []openai.ChatCompletionMessage{messages[0]}  // System
    
    // Always preserve last 5 messages (current context)
    startIdx := len(messages) - 5
    if startIdx < 1 {
        startIdx = 1
    }
    
    // Add last messages
    for i := startIdx; i < len(messages); i++ {
        important = append(important, messages[i])
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

### 6. Adaptive Management

```go
func adaptiveContextManagement(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
    usedTokens := countTokensInMessages(messages)
    
    if usedTokens < threshold80 {
        // All good, do nothing
        return messages
    } else if usedTokens < threshold90 {
        // Apply light optimization: prioritization
        optimized := prioritizeMessages(messages, maxTokens)
        fmt.Printf("  ⚡ Prioritization applied (was %d tokens)\n", usedTokens)
        return optimized
    } else {
        // Critical! Apply summarization
        fmt.Printf("  🔥 Summarization applied (was %d tokens)\n", usedTokens)
        compressed := compressOldMessages(ctx, client, messages, maxTokens)
        newTokens := countTokensInMessages(compressed)
        fmt.Printf("  ✅ After compression: %d tokens (saved %d)\n", newTokens, usedTokens-newTokens)
        return compressed
    }
}
```

## 🔍 Complete Solution Code

See the full code in the MANUAL.md file above. The solution includes all functions for token counting, truncation, summarization, prioritization, and adaptive management.

## 🎓 Key Points

1. **Token counting** — always know how many tokens are used
2. **Adaptive management** — choose technique based on context fill level
3. **Summarization** — preserves important information when compressing context
4. **Prioritization** — fast optimization without LLM call

## 🧪 Testing

Run the code and ensure:
- After 10+ messages prioritization is applied
- After 20+ messages summarization is applied
- Agent remembers user name and other important details
- Context doesn't overflow

---

**Next step:** After successfully completing Lab 09 you've mastered all key agent techniques! You can proceed to study [Multi-Agent Systems](../lab08-multi-agent/README.md) or [RAG](../lab07-rag/README.md).
