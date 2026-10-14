package main

import (
	"context"
	"fmt"
	"os"

	"github.com/kshvakov/agent/llm"
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

	fmt.Println("🚨 ALERT: Payment Service is DOWN (502).")
	fmt.Println("--- Agent Taking Over ---")

	tools := []llm.ToolDef{
		{Name: "check_http", Description: "Check service HTTP status"},
		{Name: "read_logs", Description: "Read service logs. Do this if HTTP is 500/502."},
		{Name: "restart_service", Description: "Restart the service. Use ONLY if logs show transient error."},
		{Name: "rollback_deploy", Description: "Rollback to previous version. Use if logs show Config/Syntax error."},
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

	messages := []llm.Message{
		{Role: "system", Content: sopPrompt},
		{Role: "user", Content: "Payment Service is down (502). Fix it."},
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
		req := llm.Request{
			Model:       cfg.Model,
			Messages:    messages,
			Tools:       tools,
			Temperature: 0, // Deterministic behavior
		}

		resp, err := client.Complete(ctx, req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "API Error: %v\n", err)
			os.Exit(1)
		}

		msg := resp.Choices[0].Message
		messages = append(messages, msg)

		if len(msg.ToolCalls) == 0 {
			fmt.Printf("\n🤖 Agent: %s\n", msg.Content)
			break
		}

		fmt.Printf("\n🧠 Thought: %s\n", msg.Content) // Print Chain of Thought

		for _, toolCall := range msg.ToolCalls {
			fmt.Printf("🔧 Call: %s\n", toolCall.Name)

			var result string
			switch toolCall.Name {
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

			messages = append(messages, llm.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: toolCall.ID,
			})
		}
	}
}
