# Lab 08 Solution: Multi-Agent Systems

## 📝 Разбор решения

### Ключевые моменты

1. **Изоляция контекста:** Каждый Worker создает свой собственный контекст диалога
2. **Инструменты Supervisor-а = вызовы Workers:** Supervisor не имеет прямых инструментов для работы с инфраструктурой
3. **Возврат результатов:** Ответы Workers должны быть добавлены в историю Supervisor-а с role: "tool"

### 🔍 Полный код решения

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/kshvakov/agent/llm"
)

// Mock Tools для Network Specialist
func ping(host string) string {
	fmt.Printf("   [NETWORK] Pinging %s...\n", host)
	return fmt.Sprintf("Host %s is reachable. Latency: 5ms", host)
}

// Mock Tools для DB Specialist
func runSQL(query string) string {
	fmt.Printf("   [DATABASE] Executing: %s\n", query)
	if query == "SELECT version()" {
		return "PostgreSQL 15.2"
	}
	return "Query executed successfully."
}

// Функция запуска Worker-а
func runWorkerAgent(role, systemPrompt, question string, tools []llm.ToolDef, client llm.Client, model string) string {
	ctx := context.Background()
	
	// Создаем НОВЫЙ контекст для работника (изоляция!)
	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: question},
	}

	fmt.Printf("   [%s] Starting work on: %s\n", role, question)

	// Простой цикл для работника (1-2 шага обычно)
	for i := 0; i < 5; i++ {
		req := llm.Request{
			Model:    model, // Из LLM_MODEL (по умолчанию glm-5.3)
			Messages: messages,
			Tools:   tools,
			Temperature: 0.1,
		}

		resp, err := client.Complete(ctx, req)
		if err != nil {
			return fmt.Sprintf("Worker error: %v", err)
		}

		msg := resp.Choices[0].Message
		messages = append(messages, msg)

		if len(msg.ToolCalls) == 0 {
			fmt.Printf("   [%s] Completed: %s\n", role, msg.Content)
			return msg.Content // Возвращаем финальный ответ работника
		}

		// Выполняем инструменты работника
		for _, toolCall := range msg.ToolCalls {
			var result string
			if toolCall.Name == "ping" {
				var args struct {
					Host string `json:"host"`
				}
				json.Unmarshal([]byte(toolCall.Arguments), &args)
				result = ping(args.Host)
			} else if toolCall.Name == "run_sql" {
				var args struct {
					Query string `json:"query"`
				}
				json.Unmarshal([]byte(toolCall.Arguments), &args)
				result = runSQL(args.Query)
			}

			messages = append(messages, llm.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: toolCall.ID,
			})
		}
	}
	return "Worker failed to complete task."
}

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

	// Инструменты для Workers
	netTools := []llm.ToolDef{
		{
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
	}

	dbTools := []llm.ToolDef{
		{
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
	}

	// Инструменты для Supervisor (вызов специалистов)
	supervisorTools := []llm.ToolDef{
		{
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
		{
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
	}

	supervisorPrompt := `You are a Supervisor agent. You coordinate specialized workers.
When you receive a task, delegate it to the appropriate specialist:
- Network questions → ask_network_expert
- Database questions → ask_database_expert
Collect results and provide a final answer to the user.`

	messages := []llm.Message{
		{Role: "system", Content: supervisorPrompt},
		{Role: "user", Content: "Проверь, доступен ли сервер БД db-host.example.com, и если да — узнай версию PostgreSQL"},
	}

	fmt.Println("🏁 Starting Multi-Agent System...\n")

	// Цикл Supervisor-а
	for i := 0; i < 10; i++ {
		req := llm.Request{
			Model:    cfg.Model, // Из LLM_MODEL (по умолчанию glm-5.3)
			Messages: messages,
			Tools:   supervisorTools,
			Temperature: 0.1,
		}

		resp, err := client.Complete(ctx, req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "API Error: %v\n", err)
			os.Exit(1)
		}

		msg := resp.Choices[0].Message
		messages = append(messages, msg)

		if len(msg.ToolCalls) == 0 {
			fmt.Printf("\n🤖 Supervisor Final Answer: %s\n", msg.Content)
			break
		}

		for _, toolCall := range msg.ToolCalls {
			fmt.Printf("🤖 Supervisor delegating to: %s\n", toolCall.Name)

			var workerResponse string
			var args struct {
				Question string `json:"question"`
			}
			json.Unmarshal([]byte(toolCall.Arguments), &args)

			if toolCall.Name == "ask_network_expert" {
				workerResponse = runWorkerAgent(
					"NetworkAdmin",
					"You are a Network Specialist. You know about connectivity, pings, and ports.",
					args.Question,
					netTools,
					client,
					cfg.Model,
				)
			} else if toolCall.Name == "ask_database_expert" {
				workerResponse = runWorkerAgent(
					"DBAdmin",
					"You are a Database Specialist. You know about SQL, schemas, and database versions.",
					args.Question,
					dbTools,
					client,
					cfg.Model,
				)
			}

			// Возвращаем ответ Worker-а Supervisor-у
			messages = append(messages, llm.Message{
				Role:       "tool",
				Content:    workerResponse,
				ToolCallID: toolCall.ID,
			})
		}
	}
}
```

### Ожидаемое поведение

1. Supervisor получает задачу: "Проверь доступность БД и версию"
2. Supervisor делегирует Network Specialist → проверяет доступность
3. Supervisor делегирует DB Specialist → узнает версию
4. Supervisor собирает результаты и отвечает пользователю

---

**Подробнее:** См. [Главу 07: Multi-Agent Systems](../../book/07-multi-agent/README.md) для расширенного описания Multi-Agent систем.
