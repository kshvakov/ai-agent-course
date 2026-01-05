# Lab 07 Solution: RAG & Knowledge Base

## 📝 Разбор решения

### Ключевые моменты

1. **System Prompt должен быть строгим:** Агент должен понимать, что поиск в базе знаний обязателен перед действиями
2. **Результат поиска должен быть в контексте:** Добавляйте результат поиска в историю с role: "tool"
3. **Агент должен следовать найденным инструкциям:** После поиска агент должен использовать найденную информацию

### 🔍 Полный код решения

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// База знаний
var knowledgeBase = map[string]string{
	"restart_policy.txt": "POLICY #12: Before restarting any server, you MUST run 'backup_db'. Failure to do so is a violation.",
	"backup_guide.txt":   "To run backup, use tool 'run_backup'. It takes no arguments.",
	"phoenix_restart.txt": "Phoenix server restart protocol: 1) Stop load balancer 2) Run backup_db 3) Restart Phoenix 4) Start load balancer",
}

// Mock Tools
func runBackup() string {
	fmt.Println("   [SYSTEM] Running backup...")
	return "Backup completed successfully."
}

func restartServer(name string) string {
	fmt.Println("   [SYSTEM] Restarting server:", name)
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
	// Config
	token := os.Getenv("OPENAI_API_KEY")
	if token == "" { token = "dummy" }
	config := openai.DefaultConfig(token)
	if baseURL := os.Getenv("OPENAI_BASE_URL"); baseURL != "" {
		config.BaseURL = baseURL
	}
	client := openai.NewClientWithConfig(config)
	
	ctx := context.Background()

	// Tools
	tools := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
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
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "run_backup",
				Description: "Run database backup. Required before server restarts.",
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
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
		},
	}

	systemPrompt := `You are a DevOps Agent.
CRITICAL RULE: Before ANY restart action, you MUST search the knowledge base for policies and procedures.
If you don't know the procedure, search first. Always follow the policies you find.`

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
		{Role: openai.ChatMessageRoleUser, Content: "Перезагрузи сервер Phoenix согласно регламенту"},
	}

	fmt.Println("🏁 Starting Agent with RAG...\n")

	// THE AGENT LOOP
	for i := 0; i < 10; i++ {
		req := openai.ChatCompletionRequest{
			Model:    openai.GPT3Dot5Turbo,
			Messages: messages,
			Tools:    tools,
			Temperature: 0.1,
		}

		resp, err := client.CreateChatCompletion(ctx, req)
		if err != nil {
			panic(err)
		}

		msg := resp.Choices[0].Message
		messages = append(messages, msg)

		if len(msg.ToolCalls) == 0 {
			fmt.Printf("\n🤖 Final Answer: %s\n", msg.Content)
			break
		}

		for _, toolCall := range msg.ToolCalls {
			fmt.Printf("🤖 Agent decided to call: %s\n", toolCall.Function.Name)
			
			var result string
			switch toolCall.Function.Name {
			case "search_knowledge_base":
				var args struct {
					Query string `json:"query"`
				}
				json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
				result = searchKnowledgeBase(args.Query)
			case "run_backup":
				result = runBackup()
			case "restart_server":
				var args struct {
					Name string `json:"name"`
				}
				json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
				result = restartServer(args.Name)
			}

			fmt.Printf("   Result: %s\n", result)

			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    result,
				ToolCallID: toolCall.ID,
			})
		}
	}
}
```

### Ожидаемое поведение

1. Агент получает задачу: "Перезагрузи сервер Phoenix согласно регламенту"
2. Агент вызывает `search_knowledge_base("phoenix restart")` или `search_knowledge_base("phoenix")`
3. Находит документ с протоколом перезагрузки
4. Следует инструкциям: делает backup, затем перезагружает сервер

---

**Подробнее:** См. [Главу 06: RAG и База Знаний](../../book/06-rag/README.md) для расширенного описания RAG.
