# Lab 06 Solution: The Incident (Advanced Planning)

## 📝 Deep Solution Analysis

### Chain-of-Thought in Action

Note the System Prompt in the solution:
`"Think step by step following this SOP: 1. Check HTTP... 2. Check Logs..."`

**Why is this needed?**

Without this prompt, the model receives: `User: Fix it`.  
Its probabilistic mechanism may output: `Call: restart_service`. This is the most "popular" action.

With this prompt, the model is forced to generate text:
- "Step 1: I need to check HTTP status." → This increases probability of calling `check_http`
- "HTTP is 502. Step 2: I need to check logs." → This increases probability of calling `read_logs`

We **direct the model's attention** in the right direction.

### Decision Table

For incident "Payment Service 502", the agent should follow this table:

| Symptom | Hypothesis | Check | Action | Verification |
|---------|------------|-------|--------|--------------|
| HTTP 502 | Service down | `check_http()` → 502 | - | - |
| HTTP 502 | Error in logs | `read_logs()` → "Syntax error" | `rollback_deploy()` | `check_http()` → 200 |
| HTTP 502 | Error in logs | `read_logs()` → "Connection refused" | `restart_service()` | `check_http()` → 200 |
| HTTP 502 | Transient error | `read_logs()` → "Transient error" | `restart_service()` | `check_http()` → 200 |

**Important:** Agent must not act without checking logs!

### What to Do If Model "Stutters" (Local)?

1. **Force Thinking:** In prompt write: *"Before calling any tool, output a thought starting with 'THOUGHT:' describing what you want to do."*

2. **Reduce Scope:** Remove extra tools. If you have 10 tools, the model may get confused.

3. **Few-Shot:** Add to dialogue history an example of ideal incident resolution:
   ```json
   User: "Service down"
   Assistant: "THOUGHT: Checking status first."
   Tool: check_http...
   ```
   This is the most powerful way to make the model work correctly (In-Context Learning).

### 🔍 Complete Solution Code

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/sashabaranov/go-openai"
)

// --- Environment Mock (System State) ---
var serviceState = map[string]string{
	"status":  "failed", // failed -> running
	"config":  "bad",    // bad -> good
	"version": "v2.0",   // v2.0 -> v1.9
}

// --- Tools Implementation ---

func checkHttp() string {
	fmt.Println("   [TOOL] Checking HTTP status...")
	if serviceState["status"] == "running" {
		return "200 OK"
	}
	return "502 Bad Gateway"
}

func readLogs() string {
	fmt.Println("   [TOOL] Reading logs...")
	if serviceState["config"] == "bad" {
		return "ERROR: Config syntax error in line 42. Unexpected token."
	}
	return "INFO: Service started successfully."
}

func restartService() string {
	fmt.Println("   [TOOL] Restarting service...")
	if serviceState["config"] == "bad" {
		return "Failed to start service. Exit code 1 (Config Error)."
	}
	serviceState["status"] = "running"
	return "Service restarted. Status: Active."
}

func rollback() string {
	fmt.Println("   [TOOL] Rolling back to previous version...")
	serviceState["config"] = "good"
	serviceState["version"] = "v1.9"
	serviceState["status"] = "running"
	return "Rollback complete. Version is now v1.9. Service is Active."
}

// --- Main Agent ---

func main() {
	// Config
	token := os.Getenv("OPENAI_API_KEY")
	if token == "" {
		token = "dummy"
	}
	config := openai.DefaultConfig(token)
	if baseURL := os.Getenv("OPENAI_BASE_URL"); baseURL != "" {
		config.BaseURL = baseURL
	}
	client := openai.NewClientWithConfig(config)

	ctx := context.Background()

	fmt.Println("🚨 ALERT: Payment Service is DOWN (502).")
	fmt.Println("--- Agent Taking Over ---")

	tools := []openai.Tool{
		{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: "check_http", Description: "Check service HTTP status"}},
		{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: "read_logs", Description: "Read service logs. Do this if HTTP is 500/502."}},
		{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: "restart_service", Description: "Restart the service. Use ONLY if logs show transient error."}},
		{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: "rollback_deploy", Description: "Rollback to previous version. Use if logs show Config/Syntax error."}},
	}

	// PROMPT ENGINEERING: SOP (Standard Operating Procedure)
	sopPrompt := `You are a Site Reliability Engineer (SRE).
Your goal is to fix the Payment Service.
Follow this Standard Operating Procedure (SOP) strictly:
1. Check HTTP status first.
2. If status is not 200, READ LOGS immediately. Do not guess.
3. Analyze logs:
   - If "Syntax Error" or "Config Error" -> ROLLBACK.
   - If "Connection Error" -> RESTART.
4. Verify fix by checking HTTP status again.

ALWAYS Think step by step. Output your thought process before calling a tool.`

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: sopPrompt},
		{Role: openai.ChatMessageRoleUser, Content: "Payment Service is down (502). Fix it."},
	}

	// The Loop
	for i := 0; i < 15; i++ {
		req := openai.ChatCompletionRequest{
			Model:       openai.GPT4,
			Messages:    messages,
			Tools:       tools,
			Temperature: 0, // Deterministic behavior
		}

		resp, err := client.CreateChatCompletion(ctx, req)
		if err != nil {
			panic(err)
		}

		msg := resp.Choices[0].Message
		messages = append(messages, msg)

		if len(msg.ToolCalls) == 0 {
			fmt.Printf("\n🤖 Agent: %s\n", msg.Content)
			break
		}

		fmt.Printf("\n🧠 Thought: %s\n", msg.Content) // Print Chain of Thought

		for _, toolCall := range msg.ToolCalls {
			fmt.Printf("🔧 Call: %s\n", toolCall.Function.Name)

			var result string
			switch toolCall.Function.Name {
			case "check_http":
				result = checkHttp()
			case "read_logs":
				result = readLogs()
			case "restart_service":
				result = restartService()
			case "rollback_deploy":
				result = rollback()
			}

			fmt.Printf("📦 Result: %s\n", result)

			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    result,
				ToolCallID: toolCall.ID,
			})
		}
	}
}
```

### Expected Output

```
🚨 ALERT: Payment Service is DOWN (502).
--- Agent Taking Over ---

🧠 Thought: I need to check the HTTP status first to confirm the issue.
🔧 Call: check_http
📦 Result: 502 Bad Gateway

🧠 Thought: The service is returning 502. I need to read the logs to understand why.
🔧 Call: read_logs
📦 Result: ERROR: Config syntax error in line 42. Unexpected token.

🧠 Thought: The logs show a config syntax error. A restart won't help. I need to rollback to the previous version.
🔧 Call: rollback_deploy
📦 Result: Rollback complete. Version is now v1.9. Service is Active.

🧠 Thought: I should verify that the service is now working.
🔧 Call: check_http
📦 Result: 200 OK

🤖 Agent: The service has been fixed. I rolled back to version v1.9 due to a config syntax error. The service is now returning 200 OK.
```

### Troubleshooting

If the agent doesn't follow SOP:

1. **Check model:** Run Lab 00. If Function Calling failed, the model isn't suitable.
2. **Strengthen prompt:** Add "CRITICAL:" before important instructions.
3. **Add Few-Shot:** Show model an example of correct behavior.

---

**Next step:** Study [Lab 07: RAG](../lab07-rag/README.md) for working with documentation.
