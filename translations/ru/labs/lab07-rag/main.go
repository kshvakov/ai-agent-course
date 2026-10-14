package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/kshvakov/agent/llm"
)

// База знаний
var knowledgeBase = map[string]string{
	"restart_policy.txt":  "POLICY #12: Before restarting any server, you MUST run 'backup_db'. Failure to do so is a violation.",
	"backup_guide.txt":    "To run backup, use tool 'run_backup'. It takes no arguments.",
	"phoenix_restart.txt": "Phoenix server restart protocol: 1) Stop load balancer 2) Run backup_db 3) Restart Phoenix 4) Start load balancer",
}

// Mock Tools
func runBackup() string {
	return "Backup completed successfully."
}

func restartServer(name string) string {
	return fmt.Sprintf("Server '%s' restarted successfully.", name)
}

func searchKnowledgeBase(query string) string {
	var results []string
	queryLower := strings.ToLower(query)

	for filename, content := range knowledgeBase {
		if strings.Contains(strings.ToLower(content), queryLower) {
			results = append(results, fmt.Sprintf("File: %s\nContent: %s", filename, content))
		}
	}

	if len(results) == 0 {
		return "No documents found matching your query."
	}

	return strings.Join(results, "\n---\n")
}

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

	// 2. Определяем инструменты
	tools := []llm.ToolDef{
		{
			Name:        "search_knowledge_base",
			Description: "Search the knowledge base for policies, guides, and procedures. ALWAYS use this before any action that might have a policy or procedure.",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {"type": "string", "description": "Search query (e.g., 'restart', 'backup', 'phoenix')"}
				},
				"required": ["query"]
			}`),
		},
		{
			Name:        "run_backup",
			Description: "Run database backup. Required before server restarts.",
		},
		{
			Name:        "restart_server",
			Description: "Restart a server by name",
			Parameters: json.RawMessage(`{
				"type": "object",
				"properties": {
					"name": {"type": "string"}
				},
				"required": ["name"]
			}`),
		},
	}

	systemPrompt := `You are a DevOps Agent.
CRITICAL RULE: Before ANY restart action, you MUST search the knowledge base for policies and procedures.
If you don't know the procedure, search first. Always follow the policies you find.`

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: "Перезагрузи сервер Phoenix согласно регламенту"},
	}

	fmt.Println("Starting Agent with RAG...")

	// 3. THE LOOP
	for i := 0; i < 10; i++ {
		req := llm.Request{
			Model:    cfg.Model,
			Messages: messages,
			Tools:    tools,
		}

		resp, err := client.Complete(ctx, req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "API Error: %v\n", err)
			os.Exit(1)
		}

		msg := resp.Choices[0].Message
		messages = append(messages, msg)

		// 4. Анализируем ответ
		if len(msg.ToolCalls) == 0 {
			fmt.Println("AI:", msg.Content)
			break
		}

		// 5. Выполняем инструменты
		for _, toolCall := range msg.ToolCalls {
			fmt.Printf("Executing tool: %s\n", toolCall.Name)

			var result string
			if toolCall.Name == "search_knowledge_base" {
				var args struct {
					Query string `json:"query"`
				}
				json.Unmarshal([]byte(toolCall.Arguments), &args)
				result = searchKnowledgeBase(args.Query)
			} else if toolCall.Name == "run_backup" {
				result = runBackup()
			} else if toolCall.Name == "restart_server" {
				var args struct {
					Name string `json:"name"`
				}
				json.Unmarshal([]byte(toolCall.Arguments), &args)
				result = restartServer(args.Name)
			}

			fmt.Println("Tool Output:", result)

			messages = append(messages, llm.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: toolCall.ID,
			})
		}
	}
}
