# Методическое пособие: Lab 11 — Memory и Context Management

## Зачем это нужно?

В этой лабе вы реализуете систему памяти для агента: долговременное хранилище, извлечение фактов и эффективное управление контекстом.

### Реальный кейс

**Ситуация:** Агент работает с пользователем в течение длительного времени.

**Без памяти:**
- Пользователь: "Меня зовут Иван"
- Агент: "Привет, Иван!"
- [Через 20 сообщений]
- Пользователь: "Как меня зовут?"
- Агент: "Я не знаю" (забыл)

**С памятью:**
- Пользователь: "Меня зовут Иван"
- Агент извлекает факт: "Имя пользователя: Иван" → сохраняет в память
- [Через 20 сообщений]
- Пользователь: "Как меня зовут?"
- Агент извлекает из памяти: "Имя пользователя: Иван"
- Агент: "Ваше имя — Иван"

**Разница:** Память позволяет агенту помнить важную информацию между сессиями.

## Теория простыми словами

### Типы памяти

**Рабочая память (Кратковременная):**
- Недавние ходы разговора (последние 5-10 сообщений)
- Контекст текущей задачи
- Ограничена контекстным окном модели

**Долговременная память:**
- Важные факты, извлеченные из разговоров
- Предпочтения пользователя
- Прошлые решения и результаты
- Хранится отдельно от контекста

### Извлечение фактов

Не вся информация одинаково важна:
- Имя пользователя, предпочтения → важно (хранить)
- Временный статус сервера → менее важно (можно забыть)
- Принятые решения → важно (хранить)

**Пример:**
```
Разговор: "Привет, меня зовут Иван. Я работаю в TechCorp. Сервер сейчас работает."
Извлеченные факты:
  - Имя пользователя: Иван (важность: 10)
  - Компания: TechCorp (важность: 8)
  - Статус сервера: работает (важность: 2) → не сохраняем, это временное
```

### Слои контекста

Эффективное управление контекстом использует слои:

```
Финальный контекст = 
  System Prompt (всегда первый)
  + Слой фактов (релевантные факты из памяти)
  + Слой саммари (сжатая история)
  + Рабочая память (последние 5-10 сообщений)
```

**Пример:**
```
System Prompt: "Ты DevOps помощник"
Facts Layer: "Пользователь: Иван, компания: TechCorp"
Summary Layer: "Обсуждали проблему с сервером. Решили перезапустить."
Working Memory: 
  - User: "Проверь статус"
  - Assistant: "Проверяю..."
```

### Саммаризация

Когда контекст переполняется, сжимайте старые сообщения:
- Сохраняйте важную информацию
- Уменьшайте количество токенов
- Поддерживайте непрерывность контекста

**Пример саммари:**
```
Исходная история (2000 токенов):
- User: "Меня зовут Иван"
- Assistant: "Привет, Иван!"
- User: "У нас проблема с сервером"
- Assistant: "Опишите проблему"
... (еще 50 сообщений)

Саммари (200 токенов):
"Пользователь Иван, DevOps инженер из TechCorp. Обсуждали проблему с сервером. 
Текущая задача: диагностика. Важные решения: решили перезапустить сервис."
```

## Алгоритм выполнения

### Шаг 1: Хранилище памяти

```go
type FileMemory struct {
    items []MemoryItem
    file  string
}

func (m *FileMemory) Store(key string, value any, importance int) error {
    item := MemoryItem{
        Key:        key,
        Value:      fmt.Sprintf("%v", value),
        Importance: importance,
        Timestamp:  time.Now().Unix(),
    }
    m.items = append(m.items, item)
    return m.save()
}

func (m *FileMemory) Retrieve(query string, limit int) ([]MemoryItem, error) {
    var results []MemoryItem
    queryLower := strings.ToLower(query)
    
    for _, item := range m.items {
        if strings.Contains(strings.ToLower(item.Value), queryLower) {
            results = append(results, item)
        }
    }
    
    // Сортируем по важности
    sort.Slice(results, func(i, j int) bool {
        return results[i].Importance > results[j].Importance
    })
    
    if len(results) > limit {
        results = results[:limit]
    }
    
    return results, nil
}
```

### Шаг 2: Извлечение фактов

```go
func extractFacts(ctx context.Context, client *openai.Client, conversation string) ([]Fact, error) {
    prompt := fmt.Sprintf(`Извлеки важные факты из этого разговора.
    
Разговор:
%s

Верни факты в формате JSON:
{
  "facts": [
    {"key": "user_name", "value": "Иван", "importance": 10},
    {"key": "company", "value": "TechCorp", "importance": 8}
  ]
}

Важность: 1-10, где 10 - очень важно (имя пользователя, предпочтения), 
1-3 - временная информация (статус сервера, временные события).`, conversation)

    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: openai.GPT3Dot5Turbo,
        Messages: []openai.ChatCompletionMessage{
            {Role: "user", Content: prompt},
        },
        Temperature: 0,
    })
    if err != nil {
        return nil, err
    }
    
    // Парсим JSON ответ
    var data struct {
        Facts []Fact `json:"facts"`
    }
    json.Unmarshal([]byte(resp.Choices[0].Message.Content), &data)
    
    return data.Facts, nil
}
```

### Шаг 3: Саммаризация

```go
func summarizeConversation(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage) (string, error) {
    // Собираем текст всех сообщений (кроме System)
    var textParts []string
    for _, msg := range messages {
        if msg.Role != openai.ChatMessageRoleSystem && msg.Content != "" {
            textParts = append(textParts, fmt.Sprintf("%s: %s", msg.Role, msg.Content))
        }
    }
    conversationText := strings.Join(textParts, "\n")
    
    prompt := fmt.Sprintf(`Создай краткое резюме этого разговора, сохранив только:
1. Важные принятые решения
2. Ключевые обнаруженные факты (имя пользователя, предпочтения)
3. Текущее состояние задачи

Разговор:
%s

Резюме (максимум 200 слов):`, conversationText)

    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: openai.GPT3Dot5Turbo,
        Messages: []openai.ChatCompletionMessage{
            {Role: "user", Content: prompt},
        },
        Temperature: 0,
    })
    if err != nil {
        return "", err
    }
    
    return resp.Choices[0].Message.Content, nil
}
```

### Шаг 4: Сборка слоистого контекста

```go
func buildLayeredContext(
    systemPrompt string,
    memory Memory,
    summary string,
    workingMemory []openai.ChatCompletionMessage,
    query string,
) []openai.ChatCompletionMessage {
    messages := []openai.ChatCompletionMessage{
        {Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
    }
    
    // Слой фактов
    facts, _ := memory.Retrieve(query, 5)
    if len(facts) > 0 {
        var factTexts []string
        for _, fact := range facts {
            factTexts = append(factTexts, fmt.Sprintf("- %s: %s", fact.Key, fact.Value))
        }
        messages = append(messages, openai.ChatCompletionMessage{
            Role: openai.ChatMessageRoleSystem,
            Content: "Important facts:\n" + strings.Join(factTexts, "\n"),
        })
    }
    
    // Слой саммари
    if summary != "" {
        messages = append(messages, openai.ChatCompletionMessage{
            Role: openai.ChatMessageRoleSystem,
            Content: "Summary of previous conversation:\n" + summary,
        })
    }
    
    // Рабочая память
    messages = append(messages, workingMemory...)
    
    return messages
}
```

## Типовые ошибки

### Ошибка 1: Извлекаются все факты подряд

**Симптом:** Память переполняется неважными фактами.

**Причина:** Не фильтруются факты по важности.

**Решение:** Сохраняйте только факты с важностью >= 5.

### Ошибка 2: Саммаризация теряет важную информацию

**Симптом:** После саммаризации агент забывает имя пользователя.

**Причина:** Саммари не включает важные факты.

**Решение:** Укажите в промпте саммаризации сохранять важные факты.

### Ошибка 3: Факты не извлекаются по релевантности

**Симптом:** Агент не находит релевантные факты для текущего запроса.

**Причина:** Поиск по памяти не учитывает релевантность.

**Решение:** Используйте семантический поиск или улучшите ключевые слова.

## Критерии сдачи

✅ **Сдано:**
- Память сохраняется в файл
- Факты извлекаются через LLM
- Саммаризация уменьшает токены
- Контекст собирается из слоев
- Агент помнит важные факты между сессиями

❌ **Не сдано:**
- Память не сохраняется
- Факты не извлекаются
- Саммаризация теряет важную информацию
- Контекст не слоистый

