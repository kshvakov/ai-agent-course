# Lab 09 Solution: Context Optimization

## 🎯 Цель
В этой лабораторной работе мы научились управлять контекстным окном LLM: подсчитывать токены, применять техники оптимизации (обрезка, саммаризация) и реализовать адаптивное управление контекстом.

## 📝 Разбор решения

### 1. Подсчет токенов

**Приблизительный подсчет:**
```go
func estimateTokens(text string) int {
    // Для русского: 1 токен ≈ 3 символа
    // Для английского: 1 токен ≈ 4 символа
    // Используем среднее значение
    return len(text) / 4
}
```

**Подсчет во всех сообщениях:**
```go
func countTokensInMessages(messages []openai.ChatCompletionMessage) int {
    total := 0
    for _, msg := range messages {
        total += estimateTokens(msg.Content)
        // Tool calls тоже занимают токены (примерно 80 токенов на вызов)
        if len(msg.ToolCalls) > 0 {
            total += len(msg.ToolCalls) * 80
        }
    }
    return total
}
```

### 2. Обрезка истории

```go
func truncateHistory(messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
    if len(messages) == 0 {
        return messages
    }
    
    // Всегда сохраняем System Prompt
    systemMsg := messages[0]
    result := []openai.ChatCompletionMessage{systemMsg}
    currentTokens := estimateTokens(systemMsg.Content)
    
    // Идем с конца и добавляем сообщения, пока не достигнем лимита
    for i := len(messages) - 1; i > 0; i-- {
        msg := messages[i]
        msgTokens := estimateTokens(msg.Content)
        
        // Учитываем Tool calls
        if len(msg.ToolCalls) > 0 {
            msgTokens += len(msg.ToolCalls) * 80
        }
        
        if currentTokens + msgTokens > maxTokens {
            break
        }
        
        // Добавляем в начало результата (чтобы сохранить порядок)
        result = append([]openai.ChatCompletionMessage{msg}, result...)
        currentTokens += msgTokens
    }
    
    return result
}
```

### 3. Саммаризация

```go
func summarizeMessages(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage) string {
    // Собираем текст всех сообщений (кроме System)
    conversation := ""
    for i := 1; i < len(messages); i++ {
        msg := messages[i]
        role := "User"
        if msg.Role == openai.ChatMessageRoleAssistant {
            role = "Assistant"
        } else if msg.Role == openai.ChatMessageRoleTool {
            role = "Tool"
        }
        conversation += fmt.Sprintf("%s: %s\n", role, msg.Content)
    }
    
    // Создаем промпт для саммаризации
    summaryPrompt := fmt.Sprintf(`Summarize this conversation, keeping only:
1. Important facts about the user (name, role, preferences, context)
2. Key decisions made
3. Current state of the task or conversation

Conversation:
%s`, conversation)
    
    // Вызываем LLM для саммаризации
    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: "gpt-4o-mini",
        Messages: []openai.ChatCompletionMessage{
            {
                Role:    openai.ChatMessageRoleSystem,
                Content: "You are a conversation summarizer. Create concise summaries that preserve important facts about the user and the current state of the conversation.",
            },
            {
                Role:    openai.ChatMessageRoleUser,
                Content: summaryPrompt,
            },
        },
        Temperature: 0,  // Детерминированная саммаризация
    })
    
    if err != nil {
        return fmt.Sprintf("Error summarizing: %v", err)
    }
    
    return resp.Choices[0].Message.Content
}
```

### 4. Сжатие контекста

```go
func compressOldMessages(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
    if len(messages) <= 10 {
        return messages  // Нечего сжимать
    }
    
    systemMsg := messages[0]
    oldMessages := messages[1 : len(messages)-10]  // Все кроме последних 10
    recentMessages := messages[len(messages)-10:]   // Последние 10
    
    // Сжимаем старые сообщения
    summary := summarizeMessages(ctx, client, oldMessages)
    
    // Собираем новый контекст
    compressed := []openai.ChatCompletionMessage{
        systemMsg,
        {
            Role:    openai.ChatMessageRoleSystem,
            Content: fmt.Sprintf("Summary of previous conversation:\n%s", summary),
        },
    }
    compressed = append(compressed, recentMessages...)
    
    return compressed
}
```

### 5. Приоритизация

```go
func prioritizeMessages(messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
    if len(messages) == 0 {
        return messages
    }
    
    important := []openai.ChatCompletionMessage{messages[0]}  // System
    
    // Всегда сохраняем последние 5 сообщений (текущий контекст)
    startIdx := len(messages) - 5
    if startIdx < 1 {
        startIdx = 1
    }
    
    // Добавляем последние сообщения
    for i := startIdx; i < len(messages); i++ {
        important = append(important, messages[i])
    }
    
    // Сохраняем результаты инструментов и ошибки из старых сообщений
    for i := 1; i < startIdx; i++ {
        msg := messages[i]
        if msg.Role == openai.ChatMessageRoleTool {
            important = append(important, msg)
        } else if strings.Contains(strings.ToLower(msg.Content), "error") {
            important = append(important, msg)
        }
    }
    
    return important
}
```

### 6. Адаптивное управление

```go
func adaptiveContextManagement(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
    usedTokens := countTokensInMessages(messages)
    
    if usedTokens < threshold80 {
        // Все хорошо, ничего не делаем
        return messages
    } else if usedTokens < threshold90 {
        // Применяем легкую оптимизацию: приоритизация
        optimized := prioritizeMessages(messages, maxTokens)
        fmt.Printf("  ⚡ Применена приоритизация (было %d токенов)\n", usedTokens)
        return optimized
    } else {
        // Критично! Применяем саммаризацию
        fmt.Printf("  🔥 Применена саммаризация (было %d токенов)\n", usedTokens)
        compressed := compressOldMessages(ctx, client, messages, maxTokens)
        newTokens := countTokensInMessages(compressed)
        fmt.Printf("  ✅ После сжатия: %d токенов (сэкономлено %d)\n", newTokens, usedTokens-newTokens)
        return compressed
    }
}
```

## 🔍 Полный код решения

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

const (
	maxContextTokens = 4000
	threshold80      = int(float64(maxContextTokens) * 0.8)
	threshold90      = int(float64(maxContextTokens) * 0.9)
)

func main() {
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

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: "Ты вежливый помощник. Помни важные детали о пользователе.",
		},
	}

	fmt.Println("=== Lab 09: Context Optimization ===")
	fmt.Println("Введите сообщения. После 10+ сообщений контекст начнет оптимизироваться.")
	fmt.Println("Попробуйте спросить о ранних сообщениях после оптимизации.\n")

	testMessages := []string{
		"Привет! Меня зовут Иван, я работаю DevOps инженером в компании TechCorp.",
		"У нас есть сервер на Ubuntu 22.04.",
		"Мы используем Docker для контейнеризации приложений.",
		"Наш основной стек: PostgreSQL, Redis, Nginx.",
		"Мы развернули мониторинг через Prometheus и Grafana.",
		"У нас есть CI/CD на GitLab CI.",
		"Мы используем Terraform для управления инфраструктурой.",
		"Наши приложения работают в Kubernetes кластере.",
		"Мы используем Ansible для конфигурации серверов.",
		"У нас есть резервное копирование через Bacula.",
		"Мы мониторим логи через ELK Stack.",
		"У нас есть система алертинга через PagerDuty.",
		"Мы используем Vault для управления секретами.",
		"Наш код хранится в GitLab.",
		"Мы используем Jira для управления задачами.",
		"У нас есть документация в Confluence.",
		"Мы проводим код-ревью для всех изменений.",
		"У нас есть автоматизированное тестирование.",
		"Мы используем SonarQube для анализа кода.",
		"У нас есть staging окружение для тестирования.",
		"Как меня зовут?",
		"Где я работаю?",
		"Какой у нас стек?",
	}

	for i, userMsg := range testMessages {
		fmt.Printf("\n[Сообщение %d] User: %s\n", i+1, userMsg)

		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: userMsg,
		})

		messages = adaptiveContextManagement(ctx, client, messages, maxContextTokens)

		usedTokens := countTokensInMessages(messages)
		fmt.Printf("📊 Токенов использовано: %d / %d (%.1f%%)\n", usedTokens, maxContextTokens, float64(usedTokens)/float64(maxContextTokens)*100)

		resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model:     "gpt-4o-mini",
			Messages:  messages,
			Temperature: 0.7,
		})
		if err != nil {
			fmt.Printf("❌ Ошибка: %v\n", err)
			continue
		}

		assistantMsg := resp.Choices[0].Message
		fmt.Printf("Assistant: %s\n", assistantMsg.Content)

		messages = append(messages, assistantMsg)
	}

	fmt.Println("\n=== Тест завершен ===")
}

func estimateTokens(text string) int {
	return len(text) / 4
}

func countTokensInMessages(messages []openai.ChatCompletionMessage) int {
	total := 0
	for _, msg := range messages {
		total += estimateTokens(msg.Content)
		if len(msg.ToolCalls) > 0 {
			total += len(msg.ToolCalls) * 80
		}
	}
	return total
}

func truncateHistory(messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
	if len(messages) == 0 {
		return messages
	}
	
	systemMsg := messages[0]
	result := []openai.ChatCompletionMessage{systemMsg}
	currentTokens := estimateTokens(systemMsg.Content)
	
	for i := len(messages) - 1; i > 0; i-- {
		msg := messages[i]
		msgTokens := estimateTokens(msg.Content)
		if len(msg.ToolCalls) > 0 {
			msgTokens += len(msg.ToolCalls) * 80
		}
		
		if currentTokens + msgTokens > maxTokens {
			break
		}
		
		result = append([]openai.ChatCompletionMessage{msg}, result...)
		currentTokens += msgTokens
	}
	
	return result
}

func summarizeMessages(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage) string {
	conversation := ""
	for i := 1; i < len(messages); i++ {
		msg := messages[i]
		role := "User"
		if msg.Role == openai.ChatMessageRoleAssistant {
			role = "Assistant"
		} else if msg.Role == openai.ChatMessageRoleTool {
			role = "Tool"
		}
		conversation += fmt.Sprintf("%s: %s\n", role, msg.Content)
	}
	
	summaryPrompt := fmt.Sprintf(`Summarize this conversation, keeping only:
1. Important facts about the user (name, role, preferences, context)
2. Key decisions made
3. Current state of the task or conversation

Conversation:
%s`, conversation)
	
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: "gpt-4o-mini",
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are a conversation summarizer. Create concise summaries that preserve important facts about the user and the current state of the conversation.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: summaryPrompt,
			},
		},
		Temperature: 0,
	})
	
	if err != nil {
		return fmt.Sprintf("Error summarizing: %v", err)
	}
	
	return resp.Choices[0].Message.Content
}

func compressOldMessages(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
	if len(messages) <= 10 {
		return messages
	}
	
	systemMsg := messages[0]
	oldMessages := messages[1 : len(messages)-10]
	recentMessages := messages[len(messages)-10:]
	
	summary := summarizeMessages(ctx, client, oldMessages)
	
	compressed := []openai.ChatCompletionMessage{
		systemMsg,
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: fmt.Sprintf("Summary of previous conversation:\n%s", summary),
		},
	}
	compressed = append(compressed, recentMessages...)
	
	return compressed
}

func prioritizeMessages(messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
	if len(messages) == 0 {
		return messages
	}
	
	important := []openai.ChatCompletionMessage{messages[0]}
	
	startIdx := len(messages) - 5
	if startIdx < 1 {
		startIdx = 1
	}
	
	for i := startIdx; i < len(messages); i++ {
		important = append(important, messages[i])
	}
	
	for i := 1; i < startIdx; i++ {
		msg := messages[i]
		if msg.Role == openai.ChatMessageRoleTool {
			important = append(important, msg)
		} else if strings.Contains(strings.ToLower(msg.Content), "error") {
			important = append(important, msg)
		}
	}
	
	return important
}

func adaptiveContextManagement(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
	usedTokens := countTokensInMessages(messages)
	
	if usedTokens < threshold80 {
		return messages
	} else if usedTokens < threshold90 {
		optimized := prioritizeMessages(messages, maxTokens)
		fmt.Printf("  ⚡ Применена приоритизация (было %d токенов)\n", usedTokens)
		return optimized
	} else {
		fmt.Printf("  🔥 Применена саммаризация (было %d токенов)\n", usedTokens)
		compressed := compressOldMessages(ctx, client, messages, maxTokens)
		newTokens := countTokensInMessages(compressed)
		fmt.Printf("  ✅ После сжатия: %d токенов (сэкономлено %d)\n", newTokens, usedTokens-newTokens)
		return compressed
	}
}
```

## 🎓 Ключевые моменты

1. **Подсчет токенов** — всегда знайте, сколько токенов используется
2. **Адаптивное управление** — выбирайте технику в зависимости от заполненности контекста
3. **Саммаризация** — сохраняет важную информацию при сжатии контекста
4. **Приоритизация** — быстрая оптимизация без вызова LLM

## 🧪 Тестирование

Запустите код и убедитесь, что:
- После 10+ сообщений применяется приоритизация
- После 20+ сообщений применяется саммаризация
- Агент помнит имя пользователя и другие важные детали
- Контекст не переполняется

---

**Следующий шаг:** После успешного прохождения Lab 09 вы освоили все ключевые техники работы с агентами! Можете перейти к изучению [Multi-Agent Systems](../lab08-multi-agent/README.md) или [RAG](../lab07-rag/README.md).

