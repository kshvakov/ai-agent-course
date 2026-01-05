# Методическое пособие: Lab 09 — Context Optimization

## Зачем это нужно?

В этой лабораторной работе вы научитесь управлять контекстным окном LLM — критически важный навык для создания долгоживущих агентов.

### Реальный кейс

**Ситуация:** Вы создали автономного DevOps агента, который решает инциденты. Агент работает в цикле:
1. Проверяет метрики
2. Анализирует логи
3. Проверяет конфигурацию
4. Исправляет проблему
5. Проверяет снова

После 15 шагов контекст переполняется (4000 токенов), и агент:
- Забывает начальную задачу
- Повторяет уже выполненные шаги
- Теряет важную информацию

**Проблема:** История слишком длинная, не влезает в контекстное окно.

**Решение:** Оптимизация контекста — сжатие старых сообщений через саммаризацию, сохраняя важную информацию.

## Теория простыми словами

### Контекстное окно — это ограничение

**Контекстное окно** — это максимальное количество токенов, которое модель может "увидеть" за раз.

**Примеры лимитов:**
- GPT-3.5-turbo: 4,096 токенов
- GPT-4: 8,192 токенов
- GPT-4-turbo: 128,000 токенов
- Llama 2: 4,096 токенов

**Что входит в контекст:**
- System Prompt (200-500 токенов)
- История диалога (растет с каждым сообщением)
- Tool calls и их результаты (50-200 токенов каждый)
- Новый запрос пользователя

### Почему контекст переполняется?

**Пример:**
```
Сообщение 1: "Привет! Меня зовут Иван" (10 токенов)
Сообщение 2: "Я работаю DevOps инженером" (8 токенов)
... (еще 18 сообщений по 20 токенов) ...
Сообщение 21: "Как меня зовут?" (5 токенов)

ИТОГО: 10 + 8 + (18 × 20) + 5 = 383 токена
```

Но если каждое сообщение содержит результаты инструментов (100+ токенов), контекст быстро переполняется.

### Техники оптимизации

#### 1. Подсчет токенов

**Зачем:** Всегда знать, сколько токенов используется.

**Как:**
- Приблизительно: 1 токен ≈ 4 символа (английский), ≈ 3 символа (русский)
- Точно: Использовать библиотеку `tiktoken` или `tiktoken-go`

**Пример:**
```go
func estimateTokens(text string) int {
    return len(text) / 4  // Приблизительно
}
```

#### 2. Обрезка истории (Truncation)

**Зачем:** Быстрое решение, когда история слишком длинная.

**Как:** Оставляем только последние N сообщений.

**Проблема:** Теряем важную информацию из начала.

**Пример:**
```go
// Оставляем только последние 10 сообщений
if len(messages) > 10 {
    messages = append(
        []openai.ChatCompletionMessage{messages[0]},  // System
        messages[len(messages)-9:]...,  // Последние 9
    )
}
```

#### 3. Саммаризация (Summarization)

**Зачем:** Сохранить важную информацию, сжав старые сообщения.

**Как:** Используем LLM для создания краткого резюме.

**Что сохраняем:**
- Важные факты (имя пользователя, роль)
- Решения (что было сделано)
- Текущее состояние задачи

**Пример:**
```
Исходная история (2000 токенов):
- User: "Меня зовут Иван"
- Assistant: "Привет, Иван!"
- User: "Я DevOps инженер"
- Assistant: "Отлично!"
... (еще 50 сообщений)

Сжатая версия (200 токенов):
Summary: "Пользователь Иван, DevOps инженер. Обсуждали настройку сервера.
Текущая задача: проверка мониторинга."
```

#### 4. Адаптивное управление

**Зачем:** Выбирать технику в зависимости от ситуации.

**Логика:**
- < 80% заполненности → ничего не делаем
- 80-90% → приоритизация (сохраняем важные сообщения)
- > 90% → саммаризация (сжимаем старые сообщения)

## Алгоритм выполнения

### Шаг 1: Подсчет токенов

```go
func estimateTokens(text string) int {
    // Приблизительная оценка
    // Для русского: 1 токен ≈ 3 символа
    // Для английского: 1 токен ≈ 4 символа
    return len(text) / 4
}

func countTokensInMessages(messages []openai.ChatCompletionMessage) int {
    total := 0
    for _, msg := range messages {
        total += estimateTokens(msg.Content)
        // Tool calls тоже занимают токены
        if len(msg.ToolCalls) > 0 {
            total += len(msg.ToolCalls) * 80  // Примерно 80 токенов на вызов
        }
    }
    return total
}
```

### Шаг 2: Обрезка истории

```go
func truncateHistory(messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
    if len(messages) == 0 {
        return messages
    }
    
    systemMsg := messages[0]
    result := []openai.ChatCompletionMessage{systemMsg}
    currentTokens := estimateTokens(systemMsg.Content)
    
    // Идем с конца и добавляем сообщения, пока не достигнем лимита
    for i := len(messages) - 1; i > 0; i-- {
        msgTokens := estimateTokens(messages[i].Content)
        if len(messages[i].ToolCalls) > 0 {
            msgTokens += len(messages[i].ToolCalls) * 80
        }
        
        if currentTokens + msgTokens > maxTokens {
            break
        }
        
        // Добавляем в начало результата
        result = append([]openai.ChatCompletionMessage{messages[i]}, result...)
        currentTokens += msgTokens
    }
    
    return result
}
```

### Шаг 3: Саммаризация

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
1. Important facts about the user (name, role, preferences)
2. Key decisions made
3. Current state of the task

Conversation:
%s`, conversation)
    
    // Вызываем LLM
    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: openai.GPT3Dot5Turbo,
        Messages: []openai.ChatCompletionMessage{
            {
                Role:    openai.ChatMessageRoleSystem,
                Content: "You are a conversation summarizer. Create concise summaries that preserve important facts.",
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

### Шаг 4: Сжатие контекста

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

### Шаг 5: Приоритизация

```go
func prioritizeMessages(messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
    if len(messages) == 0 {
        return messages
    }
    
    important := []openai.ChatCompletionMessage{messages[0]}  // System
    
    // Всегда сохраняем последние 5 сообщений
    startIdx := len(messages) - 5
    if startIdx < 1 {
        startIdx = 1
    }
    
    for i := startIdx; i < len(messages); i++ {
        msg := messages[i]
        important = append(important, msg)
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

### Шаг 6: Адаптивное управление

```go
func adaptiveContextManagement(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
    usedTokens := countTokensInMessages(messages)
    
    if usedTokens < threshold80 {
        // Все хорошо, ничего не делаем
        return messages
    } else if usedTokens < threshold90 {
        // Применяем легкую оптимизацию: приоритизация
        return prioritizeMessages(messages, maxTokens)
    } else {
        // Критично! Применяем саммаризацию
        return compressOldMessages(ctx, client, messages, maxTokens)
    }
}
```

## Типовые ошибки

### Ошибка 1: Неправильный подсчет токенов

**Симптом:** Контекст переполняется раньше, чем ожидалось.

**Причина:** Не учтены Tool calls, которые занимают много токенов.

**Решение:**
```go
// Учитывайте Tool calls
if len(msg.ToolCalls) > 0 {
    total += len(msg.ToolCalls) * 80
}
```

### Ошибка 2: Саммаризация теряет важную информацию

**Симптом:** Агент забывает имя пользователя после саммаризации.

**Причина:** Промпт для саммаризации не указывает, что сохранять.

**Решение:** Явно укажите в промпте, что сохранять:
```
"Keep important facts about the user (name, role, preferences)"
```

### Ошибка 3: Саммаризация вызывается слишком часто

**Симптом:** Медленная работа агента из-за частых вызовов LLM для саммаризации.

**Причина:** Саммаризация применяется при каждом запросе.

**Решение:** Используйте адаптивное управление — саммаризация только при > 90% заполненности.

### Ошибка 4: System Prompt теряется при обрезке

**Симптом:** Агент забывает свою роль.

**Причина:** System Prompt не сохранен при обрезке.

**Решение:** Всегда сохраняйте `messages[0]` (System Prompt).

## Мини-упражнения

### Упражнение 1: Точный подсчет токенов

Используйте библиотеку `github.com/pkoukk/tiktoken-go` для точного подсчета:

```go
import "github.com/pkoukk/tiktoken-go"

func countTokensAccurate(text string, model string) int {
    enc, _ := tiktoken.EncodingForModel(model)
    tokens := enc.Encode(text, nil, nil)
    return len(tokens)
}
```

### Упражнение 2: Тестирование на длинном диалоге

Создайте тест с 30+ сообщениями и убедитесь, что:
- Контекст не переполняется
- Агент помнит начало разговора
- Саммаризация работает корректно

## Критерии сдачи

✅ **Сдано:**
- Реализован подсчет токенов (приблизительный или точный)
- Реализована обрезка истории
- Реализована саммаризация через LLM
- Реализовано адаптивное управление
- Агент помнит начало разговора после оптимизации
- Контекст не переполняется в длинном диалоге (20+ сообщений)

❌ **Не сдано:**
- Контекст переполняется
- Агент забывает важную информацию после оптимизации
- Саммаризация не работает или работает некорректно
- System Prompt теряется при обрезке
- Код не компилируется

---

**Следующий шаг:** После успешного прохождения Lab 09 вы освоили все ключевые техники работы с агентами! Можете перейти к изучению [Multi-Agent Systems](../lab08-multi-agent/README.md) или [RAG](../lab07-rag/README.md).

