# Lab 04 Solution: The Agent Loop (Autonomy)

## 📝 Разбор решения

### Использование Локальных Моделей
Для цикла ReAct очень важно, чтобы модель умела **стабильно** вызывать инструменты.
Если локальная модель "глючит" (вызывает несуществующие функции или забывает аргументы), попробуйте снизить `temperature` до `0` или `0.1`.

### 🔍 Полный код решения

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/sashabaranov/go-openai"
)

// --- Mock Tools ---
func checkDisk() string { 
	fmt.Println("   [SYSTEM] Checking disk usage...")
	return "Disk Usage: 95% (CRITICAL). Large folder: /var/log" 
}

func cleanLogs() string { 
	fmt.Println("   [SYSTEM] Cleaning logs...")
	return "Logs cleaned. Freed 20GB. Disk Usage is now 40%." 
}

func main() {
	// Config
	token := os.Getenv("OPENAI_API_KEY")
	if token == "" { token = "dummy" }
	config := openai.DefaultConfig(token)
	if baseURL := os.Getenv("OPENAI_BASE_URL"); baseURL != "" {
		config.BaseURL = baseURL
	}
	client := openai.NewClientWithConfig(config)
	
	ctx := context.Background()

	// Tools
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
		{Role: openai.ChatMessageRoleSystem, Content: "You are an autonomous DevOps agent. Solve problems efficiently."},
		{Role: openai.ChatMessageRoleUser, Content: "У меня кончилось место на сервере. Разберись."},
	}

	fmt.Println("🏁 Starting Agent Loop...\n")

	// THE AGENT LOOP
	for i := 0; i < 10; i++ {
		req := openai.ChatCompletionRequest{
			Model:    "gpt-4o-mini",
			Messages: messages,
			Tools:    tools,
			Temperature: 0.1, // Для агентов лучше пониже
		}

		resp, err := client.CreateChatCompletion(ctx, req)
		if err != nil {
			panic(err)
		}

		msg := resp.Choices[0].Message
		messages = append(messages, msg)

		if len(msg.ToolCalls) == 0 {
			fmt.Printf("\n🤖 Final Answer: %s\n", msg.Content)
			break
		}

		for _, toolCall := range msg.ToolCalls {
			fmt.Printf("🤖 Agent decided to call: %s\n", toolCall.Function.Name)
			
			var result string
			switch toolCall.Function.Name {
			case "check_disk":
				result = checkDisk()
			case "clean_logs":
				result = cleanLogs()
			default:
				result = "Error: Tool not found"
			}

			fmt.Printf("📦 Tool Output: %s\n", result)

			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    result,
				ToolCallID: toolCall.ID,
			})
		}
		fmt.Println("--- Next Step ---")
	}
}
```
