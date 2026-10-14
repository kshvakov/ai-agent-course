package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/kshvakov/agent/llm"
)

// maxIterations limits the agent loop to avoid infinite cycles.
const maxIterations = 10

// Mock Tools
func checkDisk() string { return "Disk Usage: 95% (CRITICAL). Large folder: /var/log" }
func cleanLogs() string { return "Logs cleaned. Freed 20GB." }

func main() {
	// 1. Client setup: LLM_API_KEY / LLM_BASE_URL / LLM_MODEL from the environment (or a sourced .env)
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

	// 2. Define tools
	tools := []llm.ToolDef{
		{
			Name:        "check_disk",
			Description: "Check current disk usage",
			Parameters:  json.RawMessage(`{"type": "object", "properties": {}}`),
		},
		{
			Name:        "clean_logs",
			Description: "Delete old logs to free space",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"older_than_days": {
						"type": "integer",
						"description": "Only delete logs older than this many days (optional)"
					}
				}
			}`),
		},
	}

	messages := []llm.Message{
		{Role: "system", Content: "You are an autonomous DevOps agent."},
		{Role: "user", Content: "I'm out of disk space. Fix it."},
	}

	fmt.Println("Starting Agent Loop...")

	// 3. THE LOOP
	for i := 0; i < maxIterations; i++ {
		req := llm.Request{
			Model:       cfg.Model,
			Messages:    messages,
			Tools:       tools,
			Temperature: 0.2, // low temperature for stable agent behavior
		}

		resp, err := client.Complete(ctx, req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "API Error: %v\n", err)
			os.Exit(1)
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
			fmt.Printf("Executing tool: %s\n", toolCall.Name)

			var result string
			switch toolCall.Name {
			case "check_disk":
				result = checkDisk()
			case "clean_logs":
				result = cleanLogs()
			default:
				// Allowlist semantics: an unknown tool is reported back to the model
				result = fmt.Sprintf("Error: unknown tool %s", toolCall.Name)
			}

			fmt.Println("Tool Output:", result)

			messages = append(messages, llm.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: toolCall.ID,
			})
		}
	}
}
