# 12. Системы Памяти Агента

## Зачем это нужно?

Агентам нужна память, чтобы удерживать контекст между разговорами, учиться на прошлом опыте и не повторять одни и те же ошибки. Без этого агент быстро забывает важное и тратит токены на повторные объяснения.

В этой главе разберём системы памяти, которые помогают агентам запоминать, извлекать и забывать информацию эффективно.

### Реальный кейс

**Ситуация:** Пользователь спрашивает агента: "Какая была проблема с базой данных, которую мы исправили на прошлой неделе?" Агент отвечает: "У меня нет информации об этом."

**Проблема:** У агента нет памяти о прошлых разговорах. Каждое взаимодействие начинается с нуля, тратя контекст и время пользователя.

**Решение:** Система памяти сохраняет важные факты, достаёт их при необходимости и забывает устаревшее, чтобы не вылезать за лимиты контекста.

## Теория простыми словами

### Типы памяти

**Кратковременная память:**
- История текущего разговора (хранится в runtime, не в долговременном хранилище)
- Ограничена контекстным окном LLM
- Теряется при завершении разговора
- **Примечание:** Управление кратковременной памятью (саммаризация, отбор) описано в [Context Engineering](../13-context-engineering/README.md). Термин "рабочая память" используется в Context Engineering для обозначения недавних поворотов разговора в контексте.

**Долговременная память (постоянное хранилище):**
- Факты, предпочтения, прошлые решения
- Хранится в базе данных/файлах
- Сохраняется между разговорами

**Episodic память:**
- Конкретные события: "Пользователь спрашивал о месте на диске 2026-01-06"
- Полезна для отладки и обучения

**Semantic память:**
- Общие знания: "Пользователь предпочитает JSON ответы"
- Извлекается из эпизодов

### Операции с памятью

1. **Store** — Сохранить информацию для будущего
2. **Retrieve** — Найти релевантную информацию
3. **Forget** — Удалить устаревшую информацию
4. **Update** — Изменить существующую информацию

## Как это работает (пошагово)

### Шаг 1: Интерфейс памяти

```go
type Memory interface {
    Store(key string, value any, metadata map[string]any) error
    Retrieve(query string, limit int) ([]MemoryItem, error)
    Forget(key string) error
    Update(key string, value any) error
}

type MemoryItem struct {
    Key      string
    Value    any
    Metadata map[string]any
    Created  time.Time
    Accessed time.Time
    TTL      time.Duration // Время жизни
}
```

### Шаг 2: Сохранение информации

```go
type SimpleMemory struct {
    store map[string]MemoryItem
    mu    sync.RWMutex
}

func (m *SimpleMemory) Store(key string, value any, metadata map[string]any) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    m.store[key] = MemoryItem{
        Key:      key,
        Value:    value,
        Metadata: metadata,
        Created:  time.Now(),
        Accessed: time.Now(),
        TTL:      24 * time.Hour, // Дефолтный TTL
    }
    return nil
}
```

### Шаг 3: Извлечение с поиском

```go
func (m *SimpleMemory) Retrieve(query string, limit int) ([]MemoryItem, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    // Простой поиск по ключевым словам (в проде используйте embeddings)
    results := make([]MemoryItem, 0, len(m.store))
    queryLower := strings.ToLower(query)
    
    for _, item := range m.store {
        // Проверяем, не истёк ли срок
        if item.TTL > 0 && time.Since(item.Created) > item.TTL {
            continue
        }
        
        // Простое сопоставление ключевых слов
        valueStr := fmt.Sprintf("%v", item.Value)
        if strings.Contains(strings.ToLower(valueStr), queryLower) {
            item.Accessed = time.Now() // Обновляем время доступа
            results = append(results, item)
        }
    }
    
    // Сортируем по времени доступа (самые свежие первыми)
    sort.Slice(results, func(i, j int) bool {
        return results[i].Accessed.After(results[j].Accessed)
    })
    
    if len(results) > limit {
        results = results[:limit]
    }
    
    return results, nil
}
```

### Шаг 4: Забывание истёкших элементов

```go
func (m *SimpleMemory) Cleanup() error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    now := time.Now()
    for key, item := range m.store {
        if item.TTL > 0 && now.Sub(item.Created) > item.TTL {
            delete(m.store, key)
        }
    }
    return nil
}
```

### Шаг 5: Интеграция с агентом

```go
func runAgentWithMemory(ctx context.Context, client *openai.Client, memory Memory, userInput string) (string, error) {
    // Извлекаем релевантные воспоминания
    memories, _ := memory.Retrieve(userInput, 5)
    
    // Строим контекст с воспоминаниями
    messages := []openai.ChatCompletionMessage{
        {Role: "system", Content: "Ты полезный ассистент с доступом к памяти."},
    }
    
    // Добавляем релевантные воспоминания как контекст
    if len(memories) > 0 {
        memoryContext := "Релевантная прошлая информация:\n"
        for _, mem := range memories {
            memoryContext += fmt.Sprintf("- %s: %v\n", mem.Key, mem.Value)
        }
        messages = append(messages, openai.ChatCompletionMessage{
            Role:    "system",
            Content: memoryContext,
        })
    }
    
    messages = append(messages, openai.ChatCompletionMessage{
        Role:    "user",
        Content: userInput,
    })
    
    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model:    openai.GPT3Dot5Turbo,
        Messages: messages,
    })
    if err != nil {
        return "", err
    }
    
    answer := resp.Choices[0].Message.Content
    
    // Сохраняем важную информацию из разговора
    if shouldStore(userInput, answer) {
        key := generateKey(userInput)
        memory.Store(key, answer, map[string]any{
            "user_input": userInput,
            "timestamp": time.Now(),
        })
    }
    
    return answer, nil
}
```

## Типовые ошибки

### Ошибка 1: Нет TTL (Time To Live)

**Симптом:** Память растёт бесконечно, потребляя хранилище и контекст.

**Причина:** Не забывается устаревшая информация.

**Решение:** Реализуйте TTL и периодическую очистку.

### Ошибка 2: Сохранение всего

**Симптом:** Память заполняется нерелевантной информацией, делая извлечение шумным.

**Причина:** Нет фильтрации того, что сохранять.

**Решение:** Сохраняйте только важные факты, не каждый поворот разговора.

### Ошибка 3: Нет оптимизации извлечения

**Симптом:** Извлечение возвращает нерелевантные результаты или пропускает важную информацию.

**Причина:** Простое сопоставление ключевых слов вместо семантического поиска.

**Решение:** Используйте embeddings для семантического поиска по сходству.

## Мини-упражнения

### Упражнение 1: Реализуйте Memory Store

Создайте хранилище памяти, которое сохраняется на диск:

```go
type FileMemory struct {
    filepath string
    // Ваша реализация
}

func (m *FileMemory) Store(key string, value any, metadata map[string]any) error {
    // Сохранить в файл
}
```

**Ожидаемый результат:**
- Память сохраняется между перезапусками
- Можно загрузить из файла при старте

### Упражнение 2: Семантический поиск

Реализуйте извлечение с использованием embeddings:

```go
func (m *Memory) RetrieveSemantic(query string, limit int) ([]MemoryItem, error) {
    // Используйте embeddings для поиска семантически похожих элементов
}
```

**Ожидаемый результат:**
- Находит релевантные элементы даже без точного совпадения ключевых слов
- Возвращает самые похожие элементы первыми

## Критерии сдачи / Чек-лист

✅ **Сдано:**
- Понимаете разные типы памяти
- Можете сохранять и извлекать информацию
- Реализуете TTL и очистку
- Интегрируете память с агентом

❌ **Не сдано:**
- Нет TTL, память растёт бесконечно
- Сохранение всего без фильтрации
- Только простой поиск по ключевым словам

## Связь с другими главами

- **[Глава 09: Анатомия Агента](../09-agent-architecture/README.md)** — Память — ключевой компонент агента
- **[Глава 13: Context Engineering](../13-context-engineering/README.md)** — Память питает управление контекстом (факты из памяти используются при сборке контекста)
- **[Глава 08: Evals и Надежность](../08-evals-and-reliability/README.md)** — Память влияет на консистентность агента

**ВАЖНО:** Память (эта глава) отвечает за **хранение и извлечение** информации. Управление тем, как эта информация включается в контекст LLM, описано в [Context Engineering](../13-context-engineering/README.md).

## Что дальше?

После понимания систем памяти переходите к:
- **[13. Context Engineering](../13-context-engineering/README.md)** — Узнайте, как эффективно управлять контекстом из памяти, состояния и retrieval

---

**Навигация:** [← State Management](../11-state-management/README.md) | [Оглавление](../README.md) | [Context Engineering →](../13-context-engineering/README.md)

