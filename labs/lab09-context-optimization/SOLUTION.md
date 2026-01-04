# Lab 09 Solution: Context Optimization

## 🎯 Goal
In this lab, we learned to manage LLM context window: count tokens, apply optimization techniques (truncation, summarization) and implement adaptive context management.

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
    
    // Always keep System Prompt
    systemMsg := messages[0]
    result := []openai.ChatCompletionMessage{systemMsg}
    currentTokens := estimateTokens(systemMsg.Content)
    
    // Go from end and add messages until we reach limit
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
        Model: openai.GPT3Dot5Turbo,
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
        return messages  // Nothing to compress
    }
    
    systemMsg := messages[0]
    oldMessages := messages[1 : len(messages)-10]  // All except last 10
    recentMessages := messages[len(messages)-10:]   // Last 10
    
    // Compress old messages
    summary := summarizeMessages(ctx, client, oldMessages)
    
    // Assemble new context
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
    
    // Always keep last 5 messages (current context)
    startIdx := len(messages) - 5
    if startIdx < 1 {
        startIdx = 1
    }
    
    // Add recent messages
    for i := startIdx; i < len(messages); i++ {
        important = append(important, messages[i])
    }
    
    // Keep tool results and errors from old messages
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
        // Everything is fine, do nothing
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

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sashabaranov/go-openai"
)

const (
	maxContextTokens = 4000
	threshold80      = int(float64(maxContextTokens) * 0.8)
	threshold90      = int(float64(maxContextTokens) * 0.9)
)

func main() {
	token := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if token == "" {
		token = "dummy"
	}

	config := openai.DefaultConfig(token)
	if baseURL != "" {
		config.BaseURL = baseURL
	}
	client := openai.NewClientWithConfig(config)

	ctx := context.Background()

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: "You are a polite assistant. Remember important details about the user.",
		},
	}

	fmt.Println("=== Lab 09: Context Optimization ===")
	fmt.Println("Enter messages. After 10+ messages, context will start optimizing.")
	fmt.Println("Try asking about early messages after optimization.\n")

	testMessages := []string{
		"Hello! My name is Ivan, I work as a DevOps engineer at TechCorp.",
		"We have a server on Ubuntu 22.04.",
		"We use Docker for application containerization.",
		"Our main stack: PostgreSQL, Redis, Nginx.",
		"We deployed monitoring via Prometheus and Grafana.",
		"We have CI/CD on GitLab CI.",
		"We use Terraform for infrastructure management.",
		"Our applications run in Kubernetes cluster.",
		"We use Ansible for server configuration.",
		"We have backup via Bacula.",
		"We monitor logs via ELK Stack.",
		"We have alerting system via PagerDuty.",
		"We use Vault for secret management.",
		"Our code is stored in GitLab.",
		"We use Jira for task management.",
		"We have documentation in Confluence.",
		"We conduct code reviews for all changes.",
		"We have automated testing.",
		"We use SonarQube for code analysis.",
		"We have staging environment for testing.",
		"What's my name?",
		"Where do I work?",
		"What's our stack?",
	}

	for i, userMsg := range testMessages {
		fmt.Printf("\n[Message %d] User: %s\n", i+1, userMsg)

		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: userMsg,
		})

		messages = adaptiveContextManagement(ctx, client, messages, maxContextTokens)

		usedTokens := countTokensInMessages(messages)
		fmt.Printf("📊 Tokens used: %d / %d (%.1f%%)\n", usedTokens, maxContextTokens, float64(usedTokens)/float64(maxContextTokens)*100)

		resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model:     openai.GPT3Dot5Turbo,
			Messages:  messages,
			Temperature: 0.7,
		})
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			continue
		}

		assistantMsg := resp.Choices[0].Message
		fmt.Printf("Assistant: %s\n", assistantMsg.Content)

		messages = append(messages, assistantMsg)
	}

	fmt.Println("\n=== Test completed ===")
}

func estimateTokens(text string) int {
	return len(text) / 4
}

func countTokensInMessages(messages []openai.ChatCompletionMessage) int {
	total := 0
	for _, msg := range messages {
		total += estimateTokens(msg.Content)
		if len(msg.ToolCalls) > 0 {
			total += len(msg.ToolCalls) * 80
		}
	}
	return total
}

func truncateHistory(messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
	if len(messages) == 0 {
		return messages
	}
	
	systemMsg := messages[0]
	result := []openai.ChatCompletionMessage{systemMsg}
	currentTokens := estimateTokens(systemMsg.Content)
	
	for i := len(messages) - 1; i > 0; i-- {
		msg := messages[i]
		msgTokens := estimateTokens(msg.Content)
		if len(msg.ToolCalls) > 0 {
			msgTokens += len(msg.ToolCalls) * 80
		}
		
		if currentTokens + msgTokens > maxTokens {
			break
		}
		
		result = append([]openai.ChatCompletionMessage{msg}, result...)
		currentTokens += msgTokens
	}
	
	return result
}

func summarizeMessages(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage) string {
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
	
	summaryPrompt := fmt.Sprintf(`Summarize this conversation, keeping only:
1. Important facts about the user (name, role, preferences, context)
2. Key decisions made
3. Current state of the task or conversation

Conversation:
%s`, conversation)
	
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo,
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
		Temperature: 0,
	})
	
	if err != nil {
		return fmt.Sprintf("Error summarizing: %v", err)
	}
	
	return resp.Choices[0].Message.Content
}

func compressOldMessages(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
	if len(messages) <= 10 {
		return messages
	}
	
	systemMsg := messages[0]
	oldMessages := messages[1 : len(messages)-10]
	recentMessages := messages[len(messages)-10:]
	
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

func prioritizeMessages(messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
	if len(messages) == 0 {
		return messages
	}
	
	important := []openai.ChatCompletionMessage{messages[0]}
	
	startIdx := len(messages) - 5
	if startIdx < 1 {
		startIdx = 1
	}
	
	for i := startIdx; i < len(messages); i++ {
		important = append(important, messages[i])
	}
	
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

func adaptiveContextManagement(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
	usedTokens := countTokensInMessages(messages)
	
	if usedTokens < threshold80 {
		return messages
	} else if usedTokens < threshold90 {
		optimized := prioritizeMessages(messages, maxTokens)
		fmt.Printf("  ⚡ Prioritization applied (was %d tokens)\n", usedTokens)
		return optimized
	} else {
		fmt.Printf("  🔥 Summarization applied (was %d tokens)\n", usedTokens)
		compressed := compressOldMessages(ctx, client, messages, maxTokens)
		newTokens := countTokensInMessages(compressed)
		fmt.Printf("  ✅ After compression: %d tokens (saved %d)\n", newTokens, usedTokens-newTokens)
		return compressed
	}
}
```

## 🎓 Key Points

1. **Token counting** — always know how many tokens are used
2. **Adaptive management** — choose technique based on context fullness
3. **Summarization** — preserves important information when compressing context
4. **Prioritization** — fast optimization without LLM call

## 🧪 Testing

Run the code and verify:
- After 10+ messages, prioritization is applied
- After 20+ messages, summarization is applied
- Agent remembers user's name and other important details
- Context doesn't overflow

---

**Next step:** After successfully completing Lab 09, you've mastered all key agent techniques! You can proceed to study [Multi-Agent Systems](../lab08-multi-agent/README.md) or [RAG](../lab07-rag/README.md).
