package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/sashabaranov/go-openai"
)

// Mock Tools для Network Specialist
func ping(host string) string {
	return fmt.Sprintf("Host %s is reachable. Latency: 5ms", host)
}

// Mock Tools для DB Specialist
func runSQL(query string) string {
	if query == "SELECT version()" {
		return "PostgreSQL 15.2"
	}
	return "Query executed successfully."
}

// Функция запуска Worker-а
func runWorkerAgent(role, systemPrompt, question string, tools []openai.Tool, client *openai.Client) string {
	ctx := context.Background()
	
	// Создаем НОВЫЙ контекст для работника (изоляция!)
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
		{Role: openai.ChatMessageRoleUser, Content: question},
	}

	// Простой цикл для работника (1-2 шага обычно)
	for i := 0; i < 5; i++ {
		req := openai.ChatCompletionRequest{
			Model:    openai.GPT3Dot5Turbo,
			Messages: messages,
			Tools:   tools,
		}

		resp, err := client.CreateChatCompletion(ctx, req)
		if err != nil {
			return fmt.Sprintf("Worker error: %v", err)
		}

		msg := resp.Choices[0].Message
		messages = append(messages, msg)

		if len(msg.ToolCalls) == 0 {
			return msg.Content // Возвращаем финальный ответ работника
		}

		// Выполняем инструменты работника
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
	// 1. Настройка клиента (Local-First)
	token := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if token == "" {
		token = "dummy"
	}

	config := openai.DefaultConfig(token)
	if baseURL != "" {
		config.BaseURL = baseURL
	}
	client := openai.NewClientWithConfig(config)

	ctx := context.Background()

	// 2. Инструменты для Workers
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

	// 3. Инструменты для Supervisor (вызов специалистов)
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
		{Role: openai.ChatMessageRoleUser, Content: "Проверь, доступен ли сервер БД db-host.example.com, и если да — узнай версию PostgreSQL"},
	}

	fmt.Println("Starting Multi-Agent System...")

	// 4. Цикл Supervisor-а
	for i := 0; i < 10; i++ {
		req := openai.ChatCompletionRequest{
			Model:    openai.GPT3Dot5Turbo,
			Messages: messages,
			Tools:   supervisorTools,
		}

		resp, err := client.CreateChatCompletion(ctx, req)
		if err != nil {
			panic(fmt.Sprintf("API Error: %v", err))
		}

		msg := resp.Choices[0].Message
		messages = append(messages, msg)

		// 5. Анализируем ответ
		if len(msg.ToolCalls) == 0 {
			fmt.Println("Supervisor:", msg.Content)
			break
		}

		// 6. Выполняем инструменты Supervisor-а (делегируем Workers)
		for _, toolCall := range msg.ToolCalls {
			fmt.Printf("Supervisor delegating to: %s\n", toolCall.Function.Name)

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

			fmt.Printf("Worker response: %s\n", workerResponse)

			// Возвращаем ответ Worker-а Supervisor-у
			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    workerResponse,
				ToolCallID: toolCall.ID,
			})
		}
	}
}
