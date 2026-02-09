package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// ToolDefinition описывает инструмент в каталоге
type ToolDefinition struct {
	Name        string
	Description string
	Tags        []string
	RiskLevel   string
}

// Каталог инструментов с примерами Linux-подобных команд
var toolCatalog = []ToolDefinition{
	{Name: "grep", Description: "Search for patterns in text. Use for filtering lines matching a pattern.", Tags: []string{"filter", "search", "text"}, RiskLevel: "safe"},
	{Name: "sort", Description: "Sort lines of text alphabetically or numerically.", Tags: []string{"sort", "order", "text"}, RiskLevel: "safe"},
	{Name: "head", Description: "Show first N lines. Use for limiting output.", Tags: []string{"limit", "filter", "text"}, RiskLevel: "safe"},
	{Name: "tail", Description: "Show last N lines. Use for limiting output.", Tags: []string{"limit", "filter", "text"}, RiskLevel: "safe"},
	{Name: "uniq", Description: "Remove duplicate lines. Use with -c flag to count occurrences.", Tags: []string{"deduplicate", "count", "text"}, RiskLevel: "safe"},
	{Name: "wc", Description: "Count lines, words, or characters.", Tags: []string{"count", "text"}, RiskLevel: "safe"},
	{Name: "cut", Description: "Extract columns from text. Use for parsing structured data.", Tags: []string{"extract", "parse", "text"}, RiskLevel: "safe"},
	{Name: "awk", Description: "Pattern scanning and processing. Use for complex text transformations.", Tags: []string{"transform", "parse", "text"}, RiskLevel: "safe"},
	{Name: "sed", Description: "Stream editor for filtering and transforming text.", Tags: []string{"transform", "filter", "text"}, RiskLevel: "safe"},
	{Name: "tr", Description: "Translate or delete characters. Use for character-level transformations.", Tags: []string{"transform", "text"}, RiskLevel: "safe"},
	// Добавлено больше инструментов для демонстрации большого каталога (50+ инструментов)
	{Name: "find", Description: "Search for files in directory tree.", Tags: []string{"file", "search"}, RiskLevel: "moderate"},
	{Name: "ls", Description: "List directory contents.", Tags: []string{"file", "list"}, RiskLevel: "safe"},
	{Name: "cat", Description: "Display file contents.", Tags: []string{"file", "read"}, RiskLevel: "safe"},
	{Name: "rm", Description: "Remove files or directories. DANGEROUS: Can delete data permanently.", Tags: []string{"file", "delete"}, RiskLevel: "dangerous"},
}

// Пример логов для тестирования
var sampleLogs = `2024-01-01 10:00:00 INFO Application started
2024-01-01 10:01:00 ERROR Database connection failed
2024-01-01 10:02:00 WARN High memory usage detected
2024-01-01 10:03:00 ERROR Database connection failed
2024-01-01 10:04:00 INFO User logged in
2024-01-01 10:05:00 ERROR File not found
2024-01-01 10:06:00 ERROR Database connection failed
2024-01-01 10:07:00 INFO Request processed
2024-01-01 10:08:00 ERROR Permission denied
2024-01-01 10:09:00 ERROR Database connection failed
2024-01-01 10:10:00 INFO Cache cleared
2024-01-01 10:11:00 ERROR Database connection failed
2024-01-01 10:12:00 WARN Slow query detected
2024-01-01 10:13:00 ERROR File not found
2024-01-01 10:14:00 ERROR Database connection failed`

// PipelineStep описывает один шаг в пайплайне
type PipelineStep struct {
	Tool string                 `json:"tool"`
	Args map[string]interface{} `json:"args"`
}

// Pipeline описывает полное определение пайплайна
type Pipeline struct {
	Steps          []PipelineStep `json:"steps"`
	RiskLevel      string         `json:"risk_level"`
	ExpectedOutput string         `json:"expected_output,omitempty"`
}

// TODO: Реализуйте searchToolCatalog
// Ищет релевантные инструменты в каталоге по запросу
// Возвращает top-k наиболее релевантных (по совпадению в описании и тегах)
func searchToolCatalog(query string, topK int) []ToolDefinition {
	// TODO: Реализуйте совпадение по ключевым словам
	// 1. Приведите запрос к нижнему регистру
	// 2. Для каждого инструмента проверьте совпадение в описании или тегах
	// 3. Соберите совпадающие инструменты
	// 4. Верните top-k (ограничьте результаты)
	return []ToolDefinition{}
}

// TODO: Реализуйте функции выполнения инструментов
// Это моки, симулирующие Linux-команды на строковых данных

func executeGrep(input string, pattern string) string {
	// TODO: Отфильтруйте строки, содержащие паттерн
	// Разбейте вход по переносам строк, отфильтруйте строки с паттерном, соедините обратно
	return ""
}

func executeSort(input string) string {
	// TODO: Отсортируйте строки по алфавиту
	// Разбейте по переносам строк, отсортируйте, соедините обратно
	return ""
}

func executeHead(input string, lines int) string {
	// TODO: Верните первые N строк
	// Разбейте по переносам строк, возьмите первые N, соедините обратно
	return ""
}

func executeUniq(input string, count bool) string {
	// TODO: Удалите дубликаты строк
	// Если count=true, также подсчитайте вхождения (как uniq -c)
	// Разбейте по переносам строк, дедуплицируйте, соедините обратно
	return ""
}

// executeToolStep выполняет один шаг инструмента
func executeToolStep(toolName string, args map[string]interface{}, input string) (string, error) {
	switch toolName {
	case "grep":
		pattern, ok := args["pattern"].(string)
		if !ok {
			return "", fmt.Errorf("grep requires 'pattern' argument")
		}
		return executeGrep(input, pattern), nil
	case "sort":
		return executeSort(input), nil
	case "head":
		lines, ok := args["lines"].(float64)
		if !ok {
			return "", fmt.Errorf("head requires 'lines' argument")
		}
		return executeHead(input, int(lines)), nil
	case "uniq":
		count := false
		if c, ok := args["count"].(bool); ok {
			count = c
		}
		return executeUniq(input, count), nil
	default:
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}
}

// TODO: Реализуйте executePipeline
// Парсит pipeline JSON, валидирует уровень риска, выполняет шаги последовательно
func executePipeline(pipelineJSON string, inputData string) (string, error) {
	// TODO: Распарсите JSON
	// TODO: Валидируйте уровень риска (отклоните "dangerous")
	// TODO: Выполните шаги последовательно (вывод шага N становится входом шага N+1)
	// TODO: Верните финальный результат
	return "", fmt.Errorf("not implemented")
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

	// 2. Определение инструментов
	tools := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "search_tool_catalog",
				Description: "Search tool catalog for relevant tools. Use this BEFORE building pipelines to find which tools are available.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"query": {"type": "string", "description": "Search query (e.g., 'error filter sort')"},
						"top_k": {"type": "number", "description": "Number of tools to return (default: 5)"}
					},
					"required": ["query"]
				}`),
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "execute_pipeline",
				Description: "Execute a pipeline of tools. Provide pipeline JSON with 'steps' (array of {tool, args}), 'risk_level' (safe/moderate/dangerous), and optional 'expected_output'.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"pipeline": {"type": "string", "description": "JSON pipeline definition"},
						"input_data": {"type": "string", "description": "Input data (e.g., log file content)"}
					},
					"required": ["pipeline", "input_data"]
				}`),
			},
		},
	}

	systemPrompt := `You are a DevOps troubleshooting agent.
CRITICAL RULES:
1. BEFORE building a pipeline, you MUST search the tool catalog using search_tool_catalog
2. Use only tools returned by search_tool_catalog
3. Build pipeline JSON with steps, risk_level, and expected_output
4. Always set risk_level to "safe" unless the pipeline involves dangerous operations
5. Pipeline steps execute sequentially (each step's output becomes next step's input)

Example pipeline JSON:
{
    "steps": [
        {"tool": "grep", "args": {"pattern": "ERROR"}},
        {"tool": "sort", "args": {}},
        {"tool": "head", "args": {"lines": 10}}
    ],
    "risk_level": "safe",
    "expected_output": "Top 10 error lines, sorted"
}`

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
		{Role: openai.ChatMessageRoleUser, Content: "Find top 5 most frequent error lines from the logs, sorted by frequency"},
	}

	fmt.Println("Starting Agent with Tool Retrieval...")
	fmt.Printf("Tool catalog size: %d tools\n", len(toolCatalog))
	fmt.Printf("Sample logs: %d lines\n", len(strings.Split(sampleLogs, "\n")))

	// 3. ЦИКЛ АГЕНТА
	for i := 0; i < 10; i++ {
		req := openai.ChatCompletionRequest{
			Model:    "gpt-4o-mini",
			Messages: messages,
			Tools:    tools,
		}

		resp, err := client.CreateChatCompletion(ctx, req)
		if err != nil {
			panic(fmt.Sprintf("API Error: %v", err))
		}

		msg := resp.Choices[0].Message
		messages = append(messages, msg)

		// 4. Анализ ответа
		if len(msg.ToolCalls) == 0 {
			fmt.Println("\nAI:", msg.Content)
			break
		}

		// 5. Выполнение инструментов
		for _, toolCall := range msg.ToolCalls {
			fmt.Printf("\nExecuting tool: %s\n", toolCall.Function.Name)

			var result string

			if toolCall.Function.Name == "search_tool_catalog" {
				var args struct {
					Query string  `json:"query"`
					TopK  float64 `json:"top_k,omitempty"`
				}
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
					result = fmt.Sprintf("Error: Invalid JSON: %v", err)
				} else {
					topK := 5
					if args.TopK > 0 {
						topK = int(args.TopK)
					}
					relevantTools := searchToolCatalog(args.Query, topK)
					result = fmt.Sprintf("Found %d relevant tools:\n", len(relevantTools))
					for _, tool := range relevantTools {
						result += fmt.Sprintf("- %s: %s (tags: %v)\n", tool.Name, tool.Description, tool.Tags)
					}
				}
			} else if toolCall.Function.Name == "execute_pipeline" {
				var args struct {
					Pipeline  string `json:"pipeline"`
					InputData string `json:"input_data"`
				}
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
					result = fmt.Sprintf("Error: Invalid JSON: %v", err)
				} else {
					result, err = executePipeline(args.Pipeline, args.InputData)
					if err != nil {
						result = fmt.Sprintf("Error: %v", err)
					}
				}
			} else {
				result = fmt.Sprintf("Error: Unknown tool %s", toolCall.Function.Name)
			}

			fmt.Println("Tool Output:", result)

			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    result,
				ToolCallID: toolCall.ID,
			})
		}
	}
}
