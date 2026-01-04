# Lab 02 Solution: Function Calling

## 📝 Разбор решения

### Инициализация для Локальной модели
Обратите внимание на использование `NewClientWithConfig`. Это стандартный паттерн для всех лабораторных.

### Проблемы с Локальными Моделями
Если ваша модель не вызывает функцию, а просто пишет текст (например: "I will check the server..."), значит модель **не обучена** работать с тулами. Попробуйте другую модель (например, `Hermes-2-Pro-Llama-3`).

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

func runGetServerStatus(ip string) string {
	if ip == "192.168.1.10" {
		return "ONLINE (Load: 0.5)"
	}
	return "OFFLINE"
}

func main() {
	// Config
	token := os.Getenv("OPENAI_API_KEY")
	if token == "" { token = "dummy" }
	baseURL := os.Getenv("OPENAI_BASE_URL")
	
	config := openai.DefaultConfig(token)
	if baseURL != "" {
		config.BaseURL = baseURL
	}
	client := openai.NewClientWithConfig(config)

	// Tools
	tools := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "get_server_status",
				Description: "Get the status of a server by IP",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"ip": { "type": "string", "description": "IP address of the server" }
					},
					"required": ["ip"]
				}`),
			},
		},
	}

	// Request
	req := openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: "Is server 192.168.1.10 online?"},
		},
		Tools: tools,
	}

	ctx := context.Background()
	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		fmt.Printf("Error: %v\n(Check if your local server is running!)\n", err)
		return
	}

	msg := resp.Choices[0].Message

	// Handling
	if len(msg.ToolCalls) > 0 {
		call := msg.ToolCalls[0]
		fmt.Printf("🤖 AI wants to call: %s\n", call.Function.Name)
		fmt.Printf("📦 Arguments JSON: %s\n", call.Function.Arguments)

		if call.Function.Name == "get_server_status" {
			var args struct {
				IP string `json:"ip"`
			}
			json.Unmarshal([]byte(call.Function.Arguments), &args)

			result := runGetServerStatus(args.IP)
			fmt.Printf("✅ Execution Result: %s\n", result)
		}
	} else {
		fmt.Println("AI answered with text (Tool call failed or not needed):", msg.Content)
	}
}
```
