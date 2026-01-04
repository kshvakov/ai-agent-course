# Lab 02 Solution: Function Calling

## 📝 Solution Breakdown

### Initialization for Local Model
Note the use of `NewClientWithConfig`. This is a standard pattern for all labs.

### How to Determine if Model Supports Function Calling?

**Before starting this lab, be sure to run Lab 00!** It will check if your model supports Function Calling.

**If Lab 00 failed:**
- Model is not trained on Function Calling
- Need a different model (e.g., `Hermes-2-Pro-Llama-3`, `Mistral-7B-Instruct-v0.2`)

**If Lab 00 passed, but model doesn't call functions in this lab:**

1. **Check tool description (`Description`):**
   ```go
   Description: "Get the status of a server by IP"  // ✅ Good: specific
   Description: "Server stuff"  // ❌ Bad: too general
   ```

2. **Check Temperature:**
   ```go
   Temperature: 0,  // ✅ For agents always 0
   Temperature: 0.7,  // ❌ May cause instability
   ```

3. **Add Few-Shot examples to prompt:**
   ```go
   systemPrompt := `You are a DevOps assistant.
   Example:
   User: "Check server"
   Assistant: {"tool": "get_server_status", "args": {"ip": "192.168.1.1"}}
   `
   ```
   > **Note:** This is an educational demonstration of format in prompt text. With real Function Calling, model returns call in `tool_calls` field (see [Chapter 04: Tools](../../docs/book/04-tools-and-function-calling/README.md)).

### Tool Call Validation

**Important:** Always validate arguments before execution!

```go
// 1. Function name check
allowedTools := map[string]bool{
    "get_server_status": true,
}
if !allowedTools[call.Function.Name] {
    return fmt.Errorf("unknown tool: %s", call.Function.Name)
}

// 2. JSON validation
if !json.Valid([]byte(call.Function.Arguments)) {
    return fmt.Errorf("invalid JSON in arguments")
}

// 3. Parse and check required fields
var args struct {
    IP string `json:"ip"`
}
if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
    return fmt.Errorf("failed to parse arguments: %v", err)
}
if args.IP == "" {
    return fmt.Errorf("ip is required")
}
```

### Common Problems and Solutions

#### Problem 1: Model Doesn't Call Function

**Symptom:** `len(msg.ToolCalls) == 0`, model responds with text.

**Diagnosis:**
1. Run Lab 00 — if failed, model is not suitable
2. Check `Description` — make it specific
3. Set `Temperature = 0`

**Solution:**
```go
// Improve description:
Description: "Get the status of a server by IP address. Use this when user asks about server status or connectivity."

// Add to System Prompt:
systemPrompt := `You are a DevOps assistant. When user asks about server status, you MUST call get_server_status tool.`
```

#### Problem 2: Broken JSON in Arguments

**Symptom:** `json.Unmarshal` returns error.

**Example:**
```json
{"ip": "192.168.1.10"  // Missing closing brace
```

**Solution:**
```go
// Validate before parsing
if !json.Valid([]byte(call.Function.Arguments)) {
    return fmt.Errorf("invalid JSON: %s", call.Function.Arguments)
}
```

#### Problem 3: Wrong Function Name

**Symptom:** Model calls function with different name.

**Example:**
```json
{"name": "check_server"}  // But function is called "get_server_status"
```

**Solution:**
```go
// Validate name
if call.Function.Name != "get_server_status" {
    return fmt.Errorf("unknown function: %s. Available: get_server_status", call.Function.Name)
}
```

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
