# Lab 05 Solution: Human-in-the-Loop

## 📝 Разбор решения

### Локальные модели и безопасность
Для этой лабы **критически** важно качество модели. 
Маленькие модели (7B) часто игнорируют инструкции безопасности ("Always ask confirmation").
Рекомендуется использовать:
*   `Llama 3 70B` (если влезает в память/квантованная)
*   `Mixtral 8x7B`
*   `Command R+`

Если модель удаляет базу без спроса — попробуйте усилить System Prompt, добавив примеры (Few-Shot Prompting).

### 🔍 Полный код решения

```go
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// --- Mock Tools ---

func deleteDB(name string) string {
	return fmt.Sprintf("✅ Database '%s' has been DELETED.", name)
}

func sendEmail(to, subject, body string) string {
	return fmt.Sprintf("📧 Email sent to %s. Subject: %s.", to, subject)
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

	tools := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "delete_db",
				Description: "Delete a database by name. DANGEROUS.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": { "name": { "type": "string" } },
					"required": ["name"]
				}`),
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "send_email",
				Description: "Send an email",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"to": { "type": "string" },
						"subject": { "type": "string" },
						"body": { "type": "string" }
					},
					"required": ["to", "subject", "body"]
				}`),
			},
		},
	}

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: "You are a helpful assistant. IMPORTANT: 1) Always ask for explicit confirmation before deleting anything. 2) If user parameters are missing, ask clarifying questions.",
		},
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("🛡️  Safe Agent is ready. (Try: 'Delete prod_db' or 'Send email to bob')")

	// Main Chat Loop
	for {
		fmt.Print("\nUser > ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "exit" {
			break
		}
		if input == "" {
			continue
		}

		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: input,
		})

		// Agent Execution Loop
		for {
			req := openai.ChatCompletionRequest{
				Model:    openai.GPT4,
				Messages: messages,
				Tools:    tools,
			}

			resp, err := client.CreateChatCompletion(ctx, req)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				break
			}

			msg := resp.Choices[0].Message
			messages = append(messages, msg)

			// Если это текст - выводим и отдаем управление пользователю
			if len(msg.ToolCalls) == 0 {
				fmt.Printf("Agent > %s\n", msg.Content)
				break
			}

			// Если это инструменты - выполняем их автономно
			for _, toolCall := range msg.ToolCalls {
				fmt.Printf("  [⚙️ System] Executing tool: %s\n", toolCall.Function.Name)

				var result string
				if toolCall.Function.Name == "delete_db" {
					var args struct { Name string `json:"name"` }
					json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
					result = deleteDB(args.Name)
				} else if toolCall.Function.Name == "send_email" {
					var args struct { To, Subject, Body string }
					json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
					result = sendEmail(args.To, args.Subject, args.Body)
				}

				fmt.Printf("  [✅ Result] %s\n", result)

				messages = append(messages, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    result,
					ToolCallID: toolCall.ID,
				})
			}
		}
	}
}
```
