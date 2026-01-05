package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sashabaranov/go-openai"
)

const (
	maxContextTokens = 4000 // Context window limit (GPT-3.5-turbo)
	threshold80      = int(float64(maxContextTokens) * 0.8)
	threshold90      = int(float64(maxContextTokens) * 0.9)
)

func main() {
	// Client setup
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

	// Initialize history
	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: "You are a polite assistant. Remember important details about the user.",
		},
	}

	fmt.Println("=== Lab 09: Context Optimization ===")
	fmt.Println("Enter messages. After 10+ messages, context will start optimizing.")
	fmt.Println("Try asking about early messages after optimization.\n")

	// Simulate long conversation
	testMessages := []string{
		"Hello! My name is Ivan, I work as a DevOps engineer at TechCorp.",
		"We have a server on Ubuntu 22.04.",
		"We use Docker for application containerization.",
		"Our main stack: PostgreSQL, Redis, Nginx.",
		"We deployed monitoring via Prometheus and Grafana.",
		"We have CI/CD on GitLab CI.",
		"We use Terraform for infrastructure management.",
		"Our applications run in a Kubernetes cluster.",
		"We use Ansible for server configuration.",
		"We have backup via Bacula.",
		"We monitor logs via ELK Stack.",
		"We have an alerting system via PagerDuty.",
		"We use Vault for secret management.",
		"Our code is stored in GitLab.",
		"We use Jira for task management.",
		"We have documentation in Confluence.",
		"We conduct code reviews for all changes.",
		"We have automated testing.",
		"We use SonarQube for code analysis.",
		"We have a staging environment for testing.",
		"What's my name?",      // Memory check
		"Where do I work?",     // Memory check
		"What's our stack?",    // Memory check
	}

	for i, userMsg := range testMessages {
		fmt.Printf("\n[Message %d] User: %s\n", i+1, userMsg)

		// Add user message
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: userMsg,
		})

		// Optimize context before each request
		messages = adaptiveContextManagement(ctx, client, messages, maxContextTokens)

		// Show statistics
		usedTokens := countTokensInMessages(messages)
		fmt.Printf("📊 Tokens used: %d / %d (%.1f%%)\n", usedTokens, maxContextTokens, float64(usedTokens)/float64(maxContextTokens)*100)

		// Send request
		resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model:      openai.GPT3Dot5Turbo,
			Messages:   messages,
			Temperature: 0.7,
		})
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			continue
		}

		assistantMsg := resp.Choices[0].Message
		fmt.Printf("Assistant: %s\n", assistantMsg.Content)

		// Add response to history
		messages = append(messages, assistantMsg)
	}

	fmt.Println("\n=== Test completed ===")
}

// TODO 1: Implement approximate token counting
// Hint: 1 token ≈ 4 characters for English, ≈ 3 characters for Russian
func estimateTokens(text string) int {
	// TODO: Implement counting
	return 0
}

// TODO 2: Implement token counting in all messages
// Consider: System/User/Assistant messages, Tool calls also take tokens
func countTokensInMessages(messages []openai.ChatCompletionMessage) int {
	total := 0
	// TODO: Go through all messages and count tokens
	// Consider: ToolCalls also take tokens (approximately 80 tokens per call)
	return total
}

// TODO 3: Implement history truncation
// Always keep System Prompt (messages[0])
// Keep last messages until you reach maxTokens
func truncateHistory(messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
	// TODO: Implement truncation
	// Hint: Go from the end and add messages until you reach the limit
	return messages
}

// TODO 4: Implement summarization of old messages
// Use LLM to create a brief summary
// Preserve: important facts, decisions, current task state
func summarizeMessages(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage) string {
	// TODO: Collect text of all messages (except System)
	// TODO: Create summarization prompt
	// TODO: Call LLM to create summary
	// TODO: Return result
	
	// Hint: Use this prompt:
	// "Summarize this conversation, keeping only:
	//  1. Important decisions made
	//  2. Key facts discovered
	//  3. Current state of the task
	//  Conversation: [text]"
	
	return ""
}

// TODO 5: Implement context compression via summarization
// Split messages into "old" and "new" (last 10)
// Compress old ones via summarizeMessages
// Assemble new context: System + Summary + Recent
func compressOldMessages(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
	// TODO: Implement compression
	return messages
}

// TODO 6: Implement message prioritization
// Preserve:
// - System Prompt (always)
// - Last 5 messages (current context)
// - Messages with tool results (Role == "tool")
// - Messages with errors (contain "error")
func prioritizeMessages(messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
	// TODO: Implement prioritization
	return messages
}

// TODO 7: Implement adaptive context management
// If context < 80% — do nothing
// If 80-90% — apply prioritization
// If > 90% — apply summarization
func adaptiveContextManagement(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
	usedTokens := countTokensInMessages(messages)
	
	// TODO: Implement optimization technique selection logic
	// Hint: Use threshold80 and threshold90
	
	return messages
}
