# Lab 02 Solution: Function Calling

## 📝 Разбор решения

### Инициализация для Локальной модели
Обратите внимание на использование `NewClientWithConfig`. Это стандартный паттерн для всех лабораторных.

### Как определить, умеет ли модель Function Calling?

**Перед началом этой лабы обязательно запустите Lab 00!** Он проверит, поддерживает ли ваша модель Function Calling.

**Если Lab 00 не прошел:**
- Модель не обучена на Function Calling
- Нужна другая модель (например, `Hermes-2-Pro-Llama-3`, `Mistral-7B-Instruct-v0.2`)

**Если Lab 00 прошел, но в этой лабе модель не вызывает функции:**

1. **Проверьте описание инструмента (`Description`):**
   ```go
   Description: "Get the status of a server by IP"  // ✅ Хорошо: конкретно
   Description: "Server stuff"  // ❌ Плохо: слишком общее
   ```

2. **Проверьте Temperature:**
   ```go
   Temperature: 0,  // ✅ Для агентов всегда 0
   Temperature: 0.7,  // ❌ Может вызвать нестабильность
   ```

3. **Добавьте Few-Shot примеры в промпт:**
   ```go
   systemPrompt := `You are a DevOps assistant.
   Example:
   User: "Check server"
   Assistant: {"tool": "get_server_status", "args": {"ip": "192.168.1.1"}}
   `
   ```
   > **Примечание:** Это учебная демонстрация формата в тексте промпта. При реальном Function Calling модель возвращает вызов в поле `tool_calls` (см. [Главу 03: Инструменты](../../book/03-tools-and-function-calling/README.md)).

### Валидация вызова инструментов

**Важно:** Всегда валидируйте аргументы перед выполнением!

```go
// 1. Проверка имени функции
allowedTools := map[string]bool{
    "get_server_status": true,
}
if !allowedTools[call.Function.Name] {
    return fmt.Errorf("unknown tool: %s", call.Function.Name)
}

// 2. Валидация JSON
if !json.Valid([]byte(call.Function.Arguments)) {
    return fmt.Errorf("invalid JSON in arguments")
}

// 3. Парсинг и проверка обязательных полей
var args struct {
    IP string `json:"ip"`
}
if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
    return fmt.Errorf("failed to parse arguments: %v", err)
}
if args.IP == "" {
    return fmt.Errorf("ip is required")
}
```

### Типовые проблемы и их решение

#### Проблема 1: Модель не вызывает функцию

**Симптом:** `len(msg.ToolCalls) == 0`, модель отвечает текстом.

**Диагностика:**
1. Запустите Lab 00 — если провален, модель не подходит
2. Проверьте `Description` — сделайте его конкретным
3. Установите `Temperature = 0`

**Решение:**
```go
// Улучшите описание:
Description: "Get the status of a server by IP address. Use this when user asks about server status or connectivity."

// Добавьте в System Prompt:
systemPrompt := `You are a DevOps assistant. When user asks about server status, you MUST call get_server_status tool.`
```

#### Проблема 2: Сломанный JSON в аргументах

**Симптом:** `json.Unmarshal` возвращает ошибку.

**Пример:**
```json
{"ip": "192.168.1.10"  // Пропущена закрывающая скобка
```

**Решение:**
```go
// Валидация перед парсингом
if !json.Valid([]byte(call.Function.Arguments)) {
    return fmt.Errorf("invalid JSON: %s", call.Function.Arguments)
}
```

#### Проблема 3: Неправильное имя функции

**Симптом:** Модель вызывает функцию с другим именем.

**Пример:**
```json
{"name": "check_server"}  // Но функция называется "get_server_status"
```

**Решение:**
```go
// Валидация имени
if call.Function.Name != "get_server_status" {
    return fmt.Errorf("unknown function: %s. Available: get_server_status", call.Function.Name)
}
```

### 🔍 Полный код решения

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/sashabaranov/go-openai"
)

func runGetServerStatus(ip string) string {
	if ip == "192.168.1.10" {
		return "ONLINE (Load: 0.5)"
	}
	return "OFFLINE"
}

func main() {
	// Config
	token := os.Getenv("OPENAI_API_KEY")
	if token == "" { token = "dummy" }
	baseURL := os.Getenv("OPENAI_BASE_URL")
	
	config := openai.DefaultConfig(token)
	if baseURL != "" {
		config.BaseURL = baseURL
	}
	client := openai.NewClientWithConfig(config)

	// Tools
	tools := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "get_server_status",
				Description: "Get the status of a server by IP",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"ip": { "type": "string", "description": "IP address of the server" }
					},
					"required": ["ip"]
				}`),
			},
		},
	}

	// Request
	req := openai.ChatCompletionRequest{
		Model: "gpt-4o-mini",
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: "Is server 192.168.1.10 online?"},
		},
		Tools: tools,
	}

	ctx := context.Background()
	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		fmt.Printf("Error: %v\n(Check if your local server is running!)\n", err)
		return
	}

	msg := resp.Choices[0].Message

	// Handling
	if len(msg.ToolCalls) > 0 {
		call := msg.ToolCalls[0]
		fmt.Printf("🤖 AI wants to call: %s\n", call.Function.Name)
		fmt.Printf("📦 Arguments JSON: %s\n", call.Function.Arguments)

		if call.Function.Name == "get_server_status" {
			var args struct {
				IP string `json:"ip"`
			}
			json.Unmarshal([]byte(call.Function.Arguments), &args)

			result := runGetServerStatus(args.IP)
			fmt.Printf("✅ Execution Result: %s\n", result)
		}
	} else {
		fmt.Println("AI answered with text (Tool call failed or not needed):", msg.Content)
	}
}
```
