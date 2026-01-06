# Lab 05 Solution: Human-in-the-Loop

## 📝 Solution Breakdown

### Local Models and Safety
For this lab, model quality is **critical**. 
Small models (7B) often ignore safety instructions ("Always ask confirmation").
Recommended to use:
*   `Llama 3 70B` (if fits in memory/quantized)
*   `Mixtral 8x7B`
*   `Command R+`

If the model deletes database without asking — try strengthening System Prompt, adding examples (Few-Shot Prompting).

### 🛡️ Additional Protection: Runtime Confirmation Gate

**Important:** Can't rely solely on prompt and model quality for safety. Even if the model returned `tool_call` for a dangerous action, **runtime must check risk and block execution** until explicit user confirmation is received.

**Why this is critical:**
- Small models (7B) may ignore safety instructions
- Even large models can make mistakes or be compromised via prompt injection
- Safety must be **built into the execution layer**, not depend on model "discipline"

**How it works:**

```go
// Risk check function at runtime level
func calculateRisk(toolName string, args json.RawMessage) float64 {
    risks := map[string]float64{
        "delete_db":     0.9,  // Critical action
        "restart_service": 0.3, // Medium risk
        "read_logs":     0.0,  // Safe action
    }
    return risks[toolName]
}

// Check for confirmation in history
func hasConfirmationInHistory(messages []openai.ChatCompletionMessage) bool {
    for _, msg := range messages {
        if msg.Role == openai.ChatMessageRoleUser {
            content := strings.ToLower(strings.TrimSpace(msg.Content))
            if content == "yes" || content == "confirm" || strings.Contains(content, "confirm") {
                return true
            }
        }
    }
    return false
}

// Modified tool execution function
func executeToolWithSafetyCheck(toolCall openai.ToolCall, messages []openai.ChatCompletionMessage) (string, error) {
    // Risk check at Runtime level
    riskScore := calculateRisk(toolCall.Function.Name, json.RawMessage(toolCall.Function.Arguments))
    
    if riskScore > 0.8 {
        // Check if confirmation was given
        if !hasConfirmationInHistory(messages) {
            // DON'T execute tool! Return special code
            return "REQUIRES_CONFIRMATION: This action requires explicit user confirmation. Ask the user to confirm.", nil
        }
    }
    
    // If confirmation exists or risk is low — execute
    return executeTool(toolCall)
}
```

**Integration into agent loop:**

```go
// In tool execution loop
for _, toolCall := range msg.ToolCalls {
    fmt.Printf("  [⚙️ System] Checking tool: %s\n", toolCall.Function.Name)
    
    result, err := executeToolWithSafetyCheck(toolCall, messages)
    if err != nil {
        // Handle error
        break
    }
    
    // If confirmation required — DON'T execute, return to model
    if strings.Contains(result, "REQUIRES_CONFIRMATION") {
        // Add result as tool message
        messages = append(messages, openai.ChatCompletionMessage{
            Role:       openai.ChatMessageRoleTool,
            Content:    result,  // Model will see "REQUIRES_CONFIRMATION"
            ToolCallID: toolCall.ID,
        })
        
        // Send request again — model will see confirmation requirement
        // and generate text question to user
        continue  // Continue agent loop
    }
    
    // If confirmation received — execute tool
    fmt.Printf("  [✅ Result] %s\n", result)
    messages = append(messages, openai.ChatCompletionMessage{
        Role:       openai.ChatMessageRoleTool,
        Content:    result,
        ToolCallID: toolCall.ID,
    })
}
```

**UI Flow with confirmation buttons:**

In a real application, instead of text confirmation, you can use UI:

1. **Runtime detects dangerous action:**
   - Model returned `tool_call("delete_db", {"name": "prod"})`
   - Runtime checks risk → `riskScore = 0.9 > 0.8`
   - No confirmation → block execution

2. **Show preview to user:**
   ```
   ⚠️ Dangerous action requires confirmation
   
   Action: Delete database
   Parameters: name = "prod"
   Risk: High (0.9)
   
   [Confirm] [Cancel]
   ```

3. **After confirmation:**
   - User clicks "Confirm"
   - Add to history: `{role: "user", content: "yes"}`
   - Repeat agent loop
   - Now `hasConfirmationInHistory()` returns `true`
   - Runtime allows execution

**Advantages of approach:**
- ✅ Safety doesn't depend on model size
- ✅ Even if model "hallucinates" dangerous action, it won't execute
- ✅ User sees action preview before confirmation
- ✅ Can add additional checks (allowlist, argument validation)

**More details:** See [Chapter 05: Safety and Human-in-the-Loop](../../book/05-safety-and-hitl/README.md) for extended description of this approach.

### 🔍 Complete Solution Code

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

			// If it's text - output and give control to user
			if len(msg.ToolCalls) == 0 {
				fmt.Printf("Agent > %s\n", msg.Content)
				break
			}

			// If it's tools - execute them autonomously
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
