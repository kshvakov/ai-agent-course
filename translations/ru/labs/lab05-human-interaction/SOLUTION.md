# Lab 05 Solution: Human-in-the-Loop

## 📝 Разбор решения

### Локальные модели и безопасность
Для этой лабы **критически** важно качество модели. 
Маленькие модели (7B) часто игнорируют инструкции безопасности ("Always ask confirmation").
Рекомендуется использовать:
*   `Llama 3 70B` (если влезает в память/квантованная)
*   `Mixtral 8x7B`
*   `Command R+`

Если модель удаляет базу без спроса — попробуйте усилить System Prompt, добавив примеры (Few-Shot Prompting).

### 🛡️ Дополнительная защита: Runtime Confirmation Gate

**Важно:** Нельзя полагаться только на промпт и качество модели для безопасности. Даже если модель вернула `tool_call` для опасного действия, **runtime должен проверить риск и заблокировать выполнение** до получения явного подтверждения от пользователя.

**Почему это критично:**
- Маленькие модели (7B) могут игнорировать инструкции безопасности
- Даже большие модели могут ошибаться или быть скомпрометированы через prompt injection
- Безопасность должна быть **встроена в слой исполнения**, а не зависеть от "дисциплины" модели

**Как это работает:**

```go
// Функция проверки риска на уровне runtime
func calculateRisk(toolName string, args json.RawMessage) float64 {
    risks := map[string]float64{
        "delete_db":     0.9,  // Критическое действие
        "restart_service": 0.3, // Средний риск
        "read_logs":     0.0,  // Безопасное действие
    }
    return risks[toolName]
}

// Проверка наличия подтверждения в истории
func hasConfirmationInHistory(messages []openai.ChatCompletionMessage) bool {
    for _, msg := range messages {
        if msg.Role == openai.ChatMessageRoleUser {
            content := strings.ToLower(strings.TrimSpace(msg.Content))
            if content == "yes" || content == "подтверждаю" || strings.Contains(content, "confirm") {
                return true
            }
        }
    }
    return false
}

// Модифицированная функция выполнения инструмента
func executeToolWithSafetyCheck(toolCall openai.ToolCall, messages []openai.ChatCompletionMessage) (string, error) {
    // Проверка риска на уровне Runtime
    riskScore := calculateRisk(toolCall.Function.Name, json.RawMessage(toolCall.Function.Arguments))
    
    if riskScore > 0.8 {
        // Проверяем, было ли подтверждение
        if !hasConfirmationInHistory(messages) {
            // НЕ выполняем инструмент! Возвращаем специальный код
            return "REQUIRES_CONFIRMATION: This action requires explicit user confirmation. Ask the user to confirm.", nil
        }
    }
    
    // Если подтверждение есть или риск низкий — выполняем
    return executeTool(toolCall)
}
```

**Интеграция в цикл агента:**

```go
// В цикле выполнения инструментов
for _, toolCall := range msg.ToolCalls {
    fmt.Printf("  [⚙️ System] Checking tool: %s\n", toolCall.Function.Name)
    
    result, err := executeToolWithSafetyCheck(toolCall, messages)
    if err != nil {
        // Обработка ошибки
        break
    }
    
    // Если требуется подтверждение — НЕ выполняем, а возвращаем в модель
    if strings.Contains(result, "REQUIRES_CONFIRMATION") {
        // Добавляем результат как tool message
        messages = append(messages, openai.ChatCompletionMessage{
            Role:       openai.ChatMessageRoleTool,
            Content:    result,  // Модель увидит "REQUIRES_CONFIRMATION"
            ToolCallID: toolCall.ID,
        })
        
        // Отправляем запрос снова — модель увидит требование подтверждения
        // и сгенерирует текстовый вопрос пользователю
        continue  // Продолжаем цикл агента
    }
    
    // Если подтверждение получено — выполняем инструмент
    fmt.Printf("  [✅ Result] %s\n", result)
    messages = append(messages, openai.ChatCompletionMessage{
        Role:       openai.ChatMessageRoleTool,
        Content:    result,
        ToolCallID: toolCall.ID,
    })
}
```

**UI Flow с кнопками подтверждения:**

В реальном приложении вместо текстового подтверждения можно использовать UI:

1. **Runtime обнаруживает опасное действие:**
   - Модель вернула `tool_call("delete_db", {"name": "prod"})`
   - Runtime проверяет риск → `riskScore = 0.9 > 0.8`
   - Подтверждения нет → блокируем выполнение

2. **Показываем пользователю превью:**
   ```
   ⚠️ Опасное действие требует подтверждения
   
   Действие: Удаление базы данных
   Параметры: name = "prod"
   Риск: Высокий (0.9)
   
   [Подтвердить] [Отменить]
   ```

3. **После подтверждения:**
   - Пользователь нажимает "Подтвердить"
   - Добавляем в историю: `{role: "user", content: "yes"}`
   - Повторяем цикл агента
   - Теперь `hasConfirmationInHistory()` вернёт `true`
   - Runtime разрешает выполнение

**Преимущества подхода:**
- ✅ Безопасность не зависит от размера модели
- ✅ Даже если модель "галлюцинирует" опасное действие, оно не выполнится
- ✅ Пользователь видит превью действия перед подтверждением
- ✅ Можно добавить дополнительные проверки (allowlist, валидация аргументов)

**Подробнее:** См. [Главу 05: Безопасность и Human-in-the-Loop](../../book/05-safety-and-hitl/README.md) для расширенного описания этого подхода.

### 🔍 Полный код решения

```go
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// --- Mock Tools ---

func deleteDB(name string) string {
	return fmt.Sprintf("✅ Database '%s' has been DELETED.", name)
}

func sendEmail(to, subject, body string) string {
	return fmt.Sprintf("📧 Email sent to %s. Subject: %s.", to, subject)
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

	tools := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "delete_db",
				Description: "Delete a database by name. DANGEROUS.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": { "name": { "type": "string" } },
					"required": ["name"]
				}`),
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "send_email",
				Description: "Send an email",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"to": { "type": "string" },
						"subject": { "type": "string" },
						"body": { "type": "string" }
					},
					"required": ["to", "subject", "body"]
				}`),
			},
		},
	}

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: "You are a helpful assistant. IMPORTANT: 1) Always ask for explicit confirmation before deleting anything. 2) If user parameters are missing, ask clarifying questions.",
		},
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("🛡️  Safe Agent is ready. (Try: 'Delete prod_db' or 'Send email to bob')")

	// Main Chat Loop
	for {
		fmt.Print("\nUser > ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "exit" {
			break
		}
		if input == "" {
			continue
		}

		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: input,
		})

		// Agent Execution Loop
		for {
			req := openai.ChatCompletionRequest{
				Model:    openai.GPT4,
				Messages: messages,
				Tools:    tools,
			}

			resp, err := client.CreateChatCompletion(ctx, req)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				break
			}

			msg := resp.Choices[0].Message
			messages = append(messages, msg)

			// Если это текст - выводим и отдаем управление пользователю
			if len(msg.ToolCalls) == 0 {
				fmt.Printf("Agent > %s\n", msg.Content)
				break
			}

			// Если это инструменты - выполняем их автономно
			for _, toolCall := range msg.ToolCalls {
				fmt.Printf("  [⚙️ System] Executing tool: %s\n", toolCall.Function.Name)

				var result string
				if toolCall.Function.Name == "delete_db" {
					var args struct { Name string `json:"name"` }
					json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
					result = deleteDB(args.Name)
				} else if toolCall.Function.Name == "send_email" {
					var args struct { To, Subject, Body string }
					json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
					result = sendEmail(args.To, args.Subject, args.Body)
				}

				fmt.Printf("  [✅ Result] %s\n", result)

				messages = append(messages, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    result,
					ToolCallID: toolCall.ID,
				})
			}
		}
	}
}
```
