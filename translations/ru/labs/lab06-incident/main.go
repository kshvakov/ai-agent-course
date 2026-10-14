package main

import (
	"context"
	"fmt"
	"os"

	"github.com/kshvakov/agent/llm"
)

// --- Environment Mock (Состояние системы) ---
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

	fmt.Println("🚨 ALERT: Payment Service is DOWN (502).")
	fmt.Println("--- Agent Taking Over ---")

	tools := []llm.ToolDef{
		{Name: "check_http", Description: "Check service HTTP status"},
		{Name: "read_logs", Description: "Read service logs. Do this if HTTP is 500/502."},
		{Name: "restart_service", Description: "Restart the service. Use ONLY if logs show transient error."},
		{Name: "rollback_deploy", Description: "Rollback to previous version. Use if logs show Config/Syntax error."},
	}

	// TODO: Добавьте SOP (Standard Operating Procedure) в System Prompt
	// SOP должен включать:
	// 1. Check HTTP status first
	// 2. If status is not 200, READ LOGS immediately
	// 3. Analyze logs и выберите правильное действие
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

	// TODO: Реализуйте цикл агента, который следует SOP строго
	// Цикл должен:
	// 1. Отправлять запрос в LLM
	// 2. Проверять, есть ли ToolCalls
	// 3. Если есть ToolCalls - выполнять инструменты
	// 4. Добавлять результаты в историю
	// 5. Повторять до тех пор, пока агент не ответит текстом

	// The Loop
	for i := 0; i < 15; i++ {
		req := llm.Request{
			Model:       cfg.Model,
			Messages:    messages,
			Tools:       tools,
			Temperature: 0, // Детерминированное поведение
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

		fmt.Printf("\n🧠 Thought: %s\n", msg.Content) // Печатаем Chain of Thought

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
