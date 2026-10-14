# Lab 04 Solution: The Agent Loop (Autonomy)

## 📝 Solution Breakdown

### Using Local Models
For the ReAct loop it's very important that the model can **stably** call tools.
If a local model "glitches" (calls non-existent functions or forgets arguments), try lowering `temperature` to `0` or `0.1`.

### 🔍 Complete Solution Code

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/kshvakov/agent/llm"
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
	// Config: LLM_API_KEY / LLM_BASE_URL / LLM_MODEL from the environment (or a sourced .env)
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

	// Tools
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
		{Role: "system", Content: "You are an autonomous DevOps agent. Solve problems efficiently."},
		{Role: "user", Content: "I'm out of space on the server. Fix it."},
	}

	fmt.Println("🏁 Starting Agent Loop...\n")

	// THE AGENT LOOP
	for i := 0; i < 10; i++ {
		req := llm.Request{
			Model:       cfg.Model,
			Messages:    messages,
			Tools:       tools,
			Temperature: 0.2, // Low is better for agents
		}

		resp, err := client.Complete(ctx, req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "API Error: %v\n", err)
			os.Exit(1)
		}

		msg := resp.Choices[0].Message
		messages = append(messages, msg)

		if len(msg.ToolCalls) == 0 {
			fmt.Printf("\n🤖 Final Answer: %s\n", msg.Content)
			break
		}

		for _, toolCall := range msg.ToolCalls {
			fmt.Printf("🤖 Agent decided to call: %s\n", toolCall.Name)
			
			var result string
			switch toolCall.Name {
			case "check_disk":
				result = checkDisk()
			case "clean_logs":
				result = cleanLogs()
			default:
				result = fmt.Sprintf("Error: unknown tool %s", toolCall.Name)
			}

			fmt.Printf("📦 Tool Output: %s\n", result)

			messages = append(messages, llm.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: toolCall.ID,
			})
		}
		fmt.Println("--- Next Step ---")
	}
}
```
