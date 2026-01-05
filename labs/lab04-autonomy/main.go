package main

import (
	"context"
	"fmt"
	"os"

	"github.com/sashabaranov/go-openai"
)

// Mock Tools
func checkDisk() string { return "Disk Usage: 95% (CRITICAL). Large folder: /var/log" }
func cleanLogs() string { return "Logs cleaned. Freed 20GB." }

func main() {
	// 1. Client setup (Local-First)
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

	// 2. Define tools
	tools := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "check_disk",
				Description: "Check current disk usage",
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "clean_logs",
				Description: "Delete old logs to free space",
			},
		},
	}

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: "You are an autonomous DevOps agent."},
		{Role: openai.ChatMessageRoleUser, Content: "I'm out of disk space. Fix it."},
	}

	fmt.Println("Starting Agent Loop...")

	// 3. THE LOOP
	for i := 0; i < 5; i++ {
		req := openai.ChatCompletionRequest{
			Model:    openai.GPT3Dot5Turbo,
			Messages: messages,
			Tools:    tools,
		}

		resp, err := client.CreateChatCompletion(ctx, req)
		if err != nil {
			panic(fmt.Sprintf("API Error: %v", err))
		}

		msg := resp.Choices[0].Message
		messages = append(messages, msg)

		// 4. Analyze response
		if len(msg.ToolCalls) == 0 {
			fmt.Println("AI:", msg.Content)
			break
		}

		// 5. Execute tools
		for _, toolCall := range msg.ToolCalls {
			fmt.Printf("Executing tool: %s\n", toolCall.Function.Name)

			var result string
			if toolCall.Function.Name == "check_disk" {
				result = checkDisk()
			} else if toolCall.Function.Name == "clean_logs" {
				result = cleanLogs()
			}

			fmt.Println("Tool Output:", result)

			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    result,
				ToolCallID: toolCall.ID,
			})
		}
	}
}
