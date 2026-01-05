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
	return fmt.Sprintf("Database '%s' has been DELETED.", name)
}

func sendEmail(to, subject, body string) string {
	return fmt.Sprintf("Email sent to %s. Subject: %s. Body len: %d", to, subject, len(body))
}

func main() {
	// 1. Config for Local LLM
	token := os.Getenv("OPENAI_API_KEY")
	if token == "" { token = "dummy" }
	config := openai.DefaultConfig(token)
	if baseURL := os.Getenv("OPENAI_BASE_URL"); baseURL != "" {
		config.BaseURL = baseURL
	}
	client := openai.NewClientWithConfig(config)
	
	ctx := context.Background()

	// 2. Tools
	tools := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "delete_db",
				Description: "Delete a database by name. DANGEROUS ACTION.",
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
	fmt.Println("Agent is ready. (Try: 'Delete prod_db' or 'Send email to bob')")

	// 3. Interactive Chat Loop
	for {
		fmt.Print("\nUser > ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "exit" {
			break
		}

		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: input,
		})

		// 4. Agent Execution Loop
		for {
			req := openai.ChatCompletionRequest{
				Model:    openai.GPT4, // Local servers usually ignore this, but for OpenAI it's better to keep GPT-4 for safety
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

			if len(msg.ToolCalls) == 0 {
				fmt.Printf("Agent > %s\n", msg.Content)
				break
			}

			for _, toolCall := range msg.ToolCalls {
				fmt.Printf("  [System] Executing tool: %s\n", toolCall.Function.Name)
				var result string
				
				// TODO: Implement tool calls here
				result = "Executed" 

				messages = append(messages, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    result,
					ToolCallID: toolCall.ID,
				})
			}
		}
	}
}
