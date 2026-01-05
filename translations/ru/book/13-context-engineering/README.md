# 13. Context Engineering

## Зачем это нужно?

Контекстные окна ограничены. По мере роста разговоров нужно управлять тем, какая информация остаётся в контексте, а что саммаризируется или удаляется. Плохое управление контекстом приводит к трате токенов, потере информации или путанице агентов.

Эта глава покрывает техники эффективного управления контекстом: слои, саммаризация, отбор фактов и адаптивное управление контекстом.

### Реальный кейс

**Ситуация:** Долго идущий разговор с агентом. После 50 поворотов контекст — 50K токенов. Новый запрос нуждается в недавней информации, но она похоронена в истории.

**Проблема:**
- Включить всю историю: Превышает лимит контекста, дорого
- Включить только недавнее: Теряет важный контекст из раннего
- Нет стратегии: Агент путается или пропускает критическую информацию

**Решение:** Context engineering: Слои контекста (рабочая память, саммари, факты), селективное извлечение релевантной информации, саммаризация старых разговоров с сохранением ключевых фактов.

## Теория простыми словами

### Слои контекста

**Рабочая память (недавние повороты):**
- Последние N поворотов разговора
- Всегда включены
- Наиболее релевантны для текущей задачи

**Слой саммари:**
- Саммаризированные старые разговоры
- Сохраняет ключевые факты
- Уменьшает использование токенов

**Слой фактов:**
- Извлечённые важные факты из [долговременной памяти](../12-agent-memory/README.md)
- Предпочтения пользователя, решения, ограничения
- Постоянны между разговорами
- **Примечание:** Хранение и извлечение фактов описано в [Memory](../12-agent-memory/README.md), здесь описывается только их использование в контексте

**Состояние задачи:**
- Прогресс текущей задачи
- Что сделано, что ожидает
- Позволяет возобновление

### Операции с контекстом

1. **Select** — Выбрать, что включить
2. **Summarize** — Сжать старую информацию
3. **Extract** — Извлечь ключевые факты
4. **Layer** — Организовать по важности/свежести

## Как это работает (пошагово)

### Шаг 1: Интерфейс Context Manager

```go
type ContextManager interface {
    AddMessage(msg openai.ChatCompletionMessage) error
    GetContext(maxTokens int) ([]openai.ChatCompletionMessage, error)
    Summarize() error
    ExtractFacts() ([]Fact, error)
}

type Fact struct {
    Key       string
    Value     string
    Source    string // Какой разговор
    Importance int   // 1-10
}
```

### Шаг 2: Слоистый контекст

```go
type LayeredContext struct {
    workingMemory []openai.ChatCompletionMessage // Недавние повороты
    summary       string                          // Саммаризированная история
    facts         []Fact                          // Извлечённые факты
    maxWorking    int                             // Макс поворотов в рабочей памяти
}

func (c *LayeredContext) GetContext(maxTokens int) ([]openai.ChatCompletionMessage, error) {
    var messages []openai.ChatCompletionMessage
    
    // Добавляем системный промпт с фактами
    if len(c.facts) > 0 {
        factsContext := "Важные факты:\n"
        for _, fact := range c.facts {
            factsContext += fmt.Sprintf("- %s: %s\n", fact.Key, fact.Value)
        }
        messages = append(messages, openai.ChatCompletionMessage{
            Role:    "system",
            Content: factsContext,
        })
    }
    
    // Добавляем саммари, если есть
    if c.summary != "" {
        messages = append(messages, openai.ChatCompletionMessage{
            Role:    "system",
            Content: "Саммари предыдущего разговора: " + c.summary,
        })
    }
    
    // Добавляем рабочую память (недавние повороты)
    messages = append(messages, c.workingMemory...)
    
    // Обрезаем, если превышает maxTokens
    return truncateToTokenLimit(messages, maxTokens), nil
}
```

### Шаг 3: Саммаризация

```go
func (c *LayeredContext) Summarize(ctx context.Context, client *openai.Client) error {
    if len(c.workingMemory) <= c.maxWorking {
        return nil // Ещё не нужно саммаризировать
    }
    
    // Получаем старые сообщения для саммаризации
    oldMessages := c.workingMemory[:len(c.workingMemory)-c.maxWorking]
    
    // Создаём промпт для саммаризации
    prompt := "Саммаризируй этот разговор, сохраняя ключевые факты и решения:\n\n"
    for _, msg := range oldMessages {
        prompt += fmt.Sprintf("%s: %s\n", msg.Role, msg.Content)
    }
    
    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: openai.GPT3Dot5Turbo,
        Messages: []openai.ChatCompletionMessage{
            {Role: "system", Content: "Ты агент саммаризации. Извлекай ключевые факты и решения."},
            {Role: "user", Content: prompt},
        },
        Temperature: 0,
    })
    if err != nil {
        return err
    }
    
    c.summary = resp.Choices[0].Message.Content
    
    // Оставляем только недавние сообщения в рабочей памяти
    c.workingMemory = c.workingMemory[len(c.workingMemory)-c.maxWorking:]
    
    return nil
}
```

### Шаг 4: Использование фактов из памяти

**ВАЖНО:** Извлечение и хранение фактов происходит в [Memory](../12-agent-memory/README.md). Здесь мы только используем уже извлечённые факты при сборке контекста.

```go
func (c *LayeredContext) GetContext(maxTokens int, memory Memory) ([]openai.ChatCompletionMessage, error) {
    var messages []openai.ChatCompletionMessage
    
    // Получаем факты из памяти (не извлекаем здесь!)
    facts, _ := memory.Retrieve("user_preferences", 10)
    
    // Добавляем системный промпт с фактами
    if len(facts) > 0 {
        factsContext := "Важные факты:\n"
        for _, fact := range facts {
            factsContext += fmt.Sprintf("- %s: %v\n", fact.Key, fact.Value)
        }
        messages = append(messages, openai.ChatCompletionMessage{
            Role:    "system",
            Content: factsContext,
        })
    }
    
    // Добавляем саммари, если есть
    if c.summary != "" {
        messages = append(messages, openai.ChatCompletionMessage{
            Role:    "system",
            Content: "Саммари предыдущего разговора: " + c.summary,
        })
    }
    
    // Добавляем рабочую память (недавние повороты)
    messages = append(messages, c.workingMemory...)
    
    // Обрезаем, если превышает maxTokens
    return truncateToTokenLimit(messages, maxTokens), nil
}
```

## Типовые ошибки

### Ошибка 1: Нет саммаризации

**Симптом:** Контекст растёт бесконечно, достигая лимитов токенов.

**Причина:** Никогда не саммаризируются старые разговоры.

**Решение:** Реализуйте периодическую саммаризацию, когда рабочая память превышает порог.

### Ошибка 2: Слишком агрессивная саммаризация

**Симптом:** Важные детали потеряны в саммари, агент делает ошибки.

**Причина:** Саммари слишком сжата, факты не извлечены.

**Решение:** Извлекайте факты перед саммаризацией, сохраняйте их отдельно.

### Ошибка 3: Нет отбора фактов

**Симптом:** Включение нерелевантных фактов тратит токены.

**Причина:** Включение всех фактов независимо от релевантности.

**Решение:** Оценивайте факты по важности, включайте только высокооценённые факты.

## Мини-упражнения

### Упражнение 1: Реализуйте саммаризацию

Создайте функцию, которая саммаризирует историю разговора:

```go
func summarizeConversation(messages []openai.ChatCompletionMessage) (string, error) {
    // Используйте LLM для создания саммари
}
```

**Ожидаемый результат:**
- Саммари сохраняет ключевые факты
- Значительно уменьшает количество токенов
- Можно восстановить основные моменты

## Критерии сдачи / Чек-лист

✅ **Сдано:**
- Понимаете слои контекста
- Можете саммаризировать разговоры
- Извлекаете и сохраняете факты
- Управляете контекстом в пределах лимитов токенов

❌ **Не сдано:**
- Нет саммаризации, контекст растёт бесконечно
- Слишком агрессивная саммаризация, потеря фактов
- Нет отбора фактов, трата токенов

## Связь с другими главами

- **[Глава 11: State Management](../11-state-management/README.md)** — Состояние задачи используется при сборке контекста
- **[Глава 12: Системы Памяти Агента](../12-agent-memory/README.md)** — Факты из памяти используются в контексте (хранение/извлечение описано там)
- **[Глава 20: Cost & Latency Engineering](../20-cost-latency-engineering/README.md)** — Бюджеты токенов управляют политиками отбора контекста

**ВАЖНО:** Context Engineering фокусируется на **сборке контекста** из различных источников (память, состояние, retrieval). Хранение данных описано в соответствующих главах (Memory, State Management, RAG).

## Что дальше?

После освоения context engineering переходите к:
- **[14. Экосистема и Фреймворки](../14-ecosystem-and-frameworks/README.md)** — Узнайте о фреймворках для агентов

---

**Навигация:** [← Системы Памяти Агента](../12-agent-memory/README.md) | [Оглавление](../README.md) | [Экосистема и Фреймворки →](../14-ecosystem-and-frameworks/README.md)

