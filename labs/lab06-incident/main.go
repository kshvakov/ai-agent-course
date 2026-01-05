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

	// TODO: Add SOP (Standard Operating Procedure) to System Prompt
	// SOP should include:
	// 1. Check HTTP status first
	// 2. If status is not 200, READ LOGS immediately
	// 3. Analyze logs and choose the correct action
	// 4. Verify fix by checking HTTP status again
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

	// TODO: Implement agent loop that strictly follows SOP
	// Loop should:
	// 1. Send request to LLM
	// 2. Check if there are ToolCalls
	// 3. If there are ToolCalls - execute tools
	// 4. Add results to history
	// 5. Repeat until agent responds with text

	// The Loop
	for i := 0; i < 15; i++ {
		req := openai.ChatCompletionRequest{
			Model:       openai.GPT3Dot5Turbo,
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
