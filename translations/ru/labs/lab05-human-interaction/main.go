package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/kshvakov/agent/llm"
)

// --- Mock Tools ---

func deleteDB(name string) string {
	return fmt.Sprintf("Database '%s' has been DELETED.", name)
}

func sendEmail(to, subject, body string) string {
	return fmt.Sprintf("Email sent to %s. Subject: %s. Body len: %d", to, subject, len(body))
}

func main() {
	// 1. Конфигурация клиента: LLM_API_KEY / LLM_BASE_URL / LLM_MODEL из окружения (или из .env)
	cfg, err := llm.LoadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	client := llm.NewOpenAI(
		llm.WithAPIKey(cfg.APIKey),
		llm.WithBaseURL(cfg.BaseURL),
		llm.WithModel(cfg.Model),
	)

	ctx := context.Background()

	// 2. Инструменты
	tools := []llm.ToolDef{
		{
			Name:        "delete_db",
			Description: "Delete a database by name. DANGEROUS ACTION.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": { "name": { "type": "string" } },
				"required": ["name"]
			}`),
		},
		{
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
	}

	messages := []llm.Message{
		{
			Role:    "system",
			Content: "You are a helpful assistant. IMPORTANT: 1) Always ask for explicit confirmation before deleting anything. 2) If user parameters are missing, ask clarifying questions.",
		},
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Agent is ready. (Try: 'Delete prod_db' or 'Send email to bob')")

	// 3. Внешний цикл (Chat)
	for {
		fmt.Print("\nUser > ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "exit" {
			break
		}

		messages = append(messages, llm.Message{
			Role:    "user",
			Content: input,
		})

		// 4. Внутренний цикл (Agent)
		for {
			req := llm.Request{
				Model:    cfg.Model,
				Messages: messages,
				Tools:    tools,
			}

			resp, err := client.Complete(ctx, req)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				break
			}

			msg := resp.Choices[0].Message
			messages = append(messages, msg)

			if len(msg.ToolCalls) == 0 {
				fmt.Printf("Agent > %s\n", msg.Content)
				break
			}

			for _, toolCall := range msg.ToolCalls {
				fmt.Printf("  [System] Executing tool: %s\n", toolCall.Name)
				var result string

				// TODO: Реализуйте вызовы инструментов здесь
				result = "Executed"

				messages = append(messages, llm.Message{
					Role:       "tool",
					Content:    result,
					ToolCallID: toolCall.ID,
				})
			}
		}
	}
}
