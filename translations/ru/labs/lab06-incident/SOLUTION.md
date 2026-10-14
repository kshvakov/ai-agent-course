# Lab 06 Solution: The Incident (Advanced Planning)

## 📝 Глубокий анализ решения

### Chain-of-Thought в действии

Обратите внимание на System Prompt в решении:
`"Think step by step following this SOP: 1. Check HTTP... 2. Check Logs..."`

**Зачем это нужно?**

Без этого промпта модель видит: `User: Fix it`.  
Ее вероятностный механизм может выдать: `Call: restart_service`. Это самое "популярное" действие.

С этим промптом модель вынуждена сгенерировать текст:
- "Step 1: I need to check HTTP status." → Это повышает вероятность вызова `check_http`
- "HTTP is 502. Step 2: I need to check logs." → Это повышает вероятность вызова `read_logs`

Мы **направляем внимание** модели по нужному руслу.

### Таблица решений (Decision Table)

Для инцидента "Payment Service 502" агент должен следовать этой таблице:

| Симптом | Гипотеза | Проверка | Действие | Верификация |
|---------|----------|----------|----------|-------------|
| HTTP 502 | Сервис упал | `check_http()` → 502 | - | - |
| HTTP 502 | Ошибка в логах | `read_logs()` → "Syntax error" | `rollback_deploy()` | `check_http()` → 200 |
| HTTP 502 | Ошибка в логах | `read_logs()` → "Connection refused" | `restart_service()` | `check_http()` → 200 |
| HTTP 502 | Временный сбой | `read_logs()` → "Transient error" | `restart_service()` | `check_http()` → 200 |

**Важно:** Агент не должен действовать без проверки логов!

### Что делать, если модель "тупит" (локальная)?

1. **Force Thinking:** В промпте напишите: *"Before calling any tool, output a thought starting with 'THOUGHT:' describing what you want to do."*

2. **Reduce Scope:** Уберите лишние инструменты. Если у вас 10 инструментов, модель может запутаться.

3. **Few-Shot:** Добавьте в историю диалога пример идеального решения инцидента:
   ```json
   User: "Service down"
   Assistant: "THOUGHT: Checking status first."
   Tool: check_http...
   ```
   Это самый мощный способ заставить модель работать правильно (In-Context Learning).

### 🔍 Полный код решения

```go
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
	// Конфигурация клиента: LLM_API_KEY / LLM_BASE_URL / LLM_MODEL из окружения (или из .env)
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

	messages := []llm.Message{
		{Role: "system", Content: sopPrompt},
		{Role: "user", Content: "Payment Service is down (502). Fix it."},
	}

	// The Loop
	for i := 0; i < 15; i++ {
		req := llm.Request{
			Model:       cfg.Model, // Из LLM_MODEL (по умолчанию glm-5.3)
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
```

### Ожидаемый вывод

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

### Диагностика проблем

Если агент не следует SOP:

1. **Проверьте модель:** Запустите Lab 00. Если Function Calling провален, модель не подходит.
2. **Усильте промпт:** Добавьте "CRITICAL:" перед важными инструкциями.
3. **Добавьте Few-Shot:** Покажите модели пример правильного поведения.

---

**Следующий шаг:** Изучите [Lab 07: RAG](../lab07-rag/README.md) для работы с документацией.
