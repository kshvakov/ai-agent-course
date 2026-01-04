# Lab 08 Solution: Multi-Agent Systems

## 📝 Solution Breakdown

### Key Points

1. **Context isolation:** Each Worker creates its own dialogue context
2. **Supervisor tools = Worker calls:** Supervisor doesn't have direct tools for infrastructure work
3. **Return results:** Worker answers must be added to Supervisor's history with role: "tool"

### 🔍 Complete Solution Code

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/sashabaranov/go-openai"
)

// Mock Tools for Network Specialist
func ping(host string) string {
	fmt.Printf("   [NETWORK] Pinging %s...\n", host)
	return fmt.Sprintf("Host %s is reachable. Latency: 5ms", host)
}

// Mock Tools for DB Specialist
func runSQL(query string) string {
	fmt.Printf("   [DATABASE] Executing: %s\n", query)
	if query == "SELECT version()" {
		return "PostgreSQL 15.2"
	}
	return "Query executed successfully."
}

// Function to run Worker agent
func runWorkerAgent(role, systemPrompt, question string, tools []openai.Tool, client *openai.Client) string {
	ctx := context.Background()
	
	// Create NEW context for worker (isolation!)
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
		{Role: openai.ChatMessageRoleUser, Content: question},
	}

	fmt.Printf("   [%s] Starting work on: %s\n", role, question)

	// Simple loop for worker (usually 1-2 steps)
	for i := 0; i < 5; i++ {
		req := openai.ChatCompletionRequest{
			Model:    openai.GPT3Dot5Turbo,
			Messages: messages,
			Tools:   tools,
			Temperature: 0.1,
		}

		resp, err := client.CreateChatCompletion(ctx, req)
		if err != nil {
			return fmt.Sprintf("Worker error: %v", err)
		}

		msg := resp.Choices[0].Message
		messages = append(messages, msg)

		if len(msg.ToolCalls) == 0 {
			fmt.Printf("   [%s] Completed: %s\n", role, msg.Content)
			return msg.Content // Return worker's final answer
		}

		// Execute worker tools
		for _, toolCall := range msg.ToolCalls {
			var result string
			if toolCall.Function.Name == "ping" {
				var args struct {
					Host string `json:"host"`
				}
				json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
				result = ping(args.Host)
			} else if toolCall.Function.Name == "run_sql" {
				var args struct {
					Query string `json:"query"`
				}
				json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
				result = runSQL(args.Query)
			}

			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    result,
				ToolCallID: toolCall.ID,
			})
		}
	}
	return "Worker failed to complete task."
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

	// Tools for Workers
	netTools := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "ping",
				Description: "Ping a host to check connectivity",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"host": {"type": "string"}
					},
					"required": ["host"]
				}`),
			},
		},
	}

	dbTools := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "run_sql",
				Description: "Run a SQL query on the database",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"query": {"type": "string"}
					},
					"required": ["query"]
				}`),
			},
		},
	}

	// Tools for Supervisor (calling specialists)
	supervisorTools := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "ask_network_expert",
				Description: "Ask the network specialist about connectivity, pings, ports. Use this when you need to check if a host is reachable.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"question": {"type": "string"}
					},
					"required": ["question"]
				}`),
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "ask_database_expert",
				Description: "Ask the DB specialist about SQL, schemas, data, versions. Use this when you need database information.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"question": {"type": "string"}
					},
					"required": ["question"]
				}`),
			},
		},
	}

	supervisorPrompt := `You are a Supervisor agent. You coordinate specialized workers.
When you receive a task, delegate it to the appropriate specialist:
- Network questions → ask_network_expert
- Database questions → ask_database_expert
Collect results and provide a final answer to the user.`

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: supervisorPrompt},
		{Role: openai.ChatMessageRoleUser, Content: "Check if DB server db-host.example.com is available, and if yes — find out PostgreSQL version"},
	}

	fmt.Println("🏁 Starting Multi-Agent System...\n")

	// Supervisor loop
	for i := 0; i < 10; i++ {
		req := openai.ChatCompletionRequest{
			Model:    openai.GPT3Dot5Turbo,
			Messages: messages,
			Tools:   supervisorTools,
			Temperature: 0.1,
		}

		resp, err := client.CreateChatCompletion(ctx, req)
		if err != nil {
			panic(err)
		}

		msg := resp.Choices[0].Message
		messages = append(messages, msg)

		if len(msg.ToolCalls) == 0 {
			fmt.Printf("\n🤖 Supervisor Final Answer: %s\n", msg.Content)
			break
		}

		for _, toolCall := range msg.ToolCalls {
			fmt.Printf("🤖 Supervisor delegating to: %s\n", toolCall.Function.Name)

			var workerResponse string
			var args struct {
				Question string `json:"question"`
			}
			json.Unmarshal([]byte(toolCall.Function.Arguments), &args)

			if toolCall.Function.Name == "ask_network_expert" {
				workerResponse = runWorkerAgent(
					"NetworkAdmin",
					"You are a Network Specialist. You know about connectivity, pings, and ports.",
					args.Question,
					netTools,
					client,
				)
			} else if toolCall.Function.Name == "ask_database_expert" {
				workerResponse = runWorkerAgent(
					"DBAdmin",
					"You are a Database Specialist. You know about SQL, schemas, and database versions.",
					args.Question,
					dbTools,
					client,
				)
			}

			// Return Worker's answer to Supervisor
			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    workerResponse,
				ToolCallID: toolCall.ID,
			})
		}
	}
}
```

### Expected Behavior

1. Supervisor receives task: "Check DB availability and version"
2. Supervisor delegates to Network Specialist → checks availability
3. Supervisor delegates to DB Specialist → finds out version
4. Supervisor collects results and answers user

---

**More details:** See [Chapter 08: Multi-Agent Systems](../../docs/book/08-multi-agent/README.md) for extended description of Multi-Agent systems.
