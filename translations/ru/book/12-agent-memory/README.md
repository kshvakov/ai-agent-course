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

## Checkpoint и Resume

Агент может работать часами над сложной задачей. Если процесс упадёт посередине, потеряется весь прогресс. Checkpoint сохраняет состояние разговора периодически. При сбое агент возобновляет работу с последней сохранённой точки.

### Периодическое сохранение состояния

Checkpoint — это снимок состояния агента в конкретный момент: история сообщений, содержимое памяти, текущий шаг выполнения. Сохраняйте checkpoint после каждого значимого шага (вызов инструмента, ответ пользователю).

```go
type MemoryCheckpoint struct {
    RunID     string                `json:"run_id"`
    Step      int                   `json:"step"`
    Messages  []Message             `json:"messages"`
    Memory    map[string]MemoryItem `json:"memory"`
    ToolState map[string]any        `json:"tool_state"`
    CreatedAt time.Time             `json:"created_at"`
}

func saveCheckpoint(ctx context.Context, store CheckpointStore, cp MemoryCheckpoint) error {
    data, err := json.Marshal(cp)
    if err != nil {
        return fmt.Errorf("не удалось сериализовать checkpoint: %w", err)
    }

    key := fmt.Sprintf("checkpoint:%s:%d", cp.RunID, cp.Step)
    return store.Set(ctx, key, data, 24*time.Hour) // TTL 24 часа
}

func loadCheckpoint(ctx context.Context, store CheckpointStore, runID string) (*MemoryCheckpoint, error) {
    // Ищем последний checkpoint для данного run
    pattern := fmt.Sprintf("checkpoint:%s:*", runID)
    keys, err := store.Keys(ctx, pattern)
    if err != nil {
        return nil, err
    }

    if len(keys) == 0 {
        return nil, nil // Нет checkpoint — начинаем с нуля
    }

    // Берём последний по номеру шага
    sort.Strings(keys)
    lastKey := keys[len(keys)-1]

    data, err := store.Get(ctx, lastKey)
    if err != nil {
        return nil, err
    }

    var cp MemoryCheckpoint
    if err := json.Unmarshal(data, &cp); err != nil {
        return nil, fmt.Errorf("не удалось десериализовать checkpoint: %w", err)
    }

    return &cp, nil
}
```

### Resume после сбоя

При запуске агент проверяет наличие checkpoint. Если он есть, восстанавливает состояние и продолжает с прерванного шага:

```go
func runAgentWithCheckpoints(ctx context.Context, client *openai.Client, store CheckpointStore, runID, input string) (string, error) {
    // Пытаемся восстановиться из checkpoint
    cp, err := loadCheckpoint(ctx, store, runID)
    if err != nil {
        return "", fmt.Errorf("ошибка загрузки checkpoint: %w", err)
    }

    var messages []Message
    step := 0

    if cp != nil {
        // Восстанавливаем состояние из checkpoint
        messages = cp.Messages
        step = cp.Step
        log.Printf("Восстановлены из checkpoint: run=%s, step=%d", runID, step)
    } else {
        // Начинаем с нуля
        messages = []Message{
            {Role: "system", Content: "Ты полезный ассистент."},
            {Role: "user", Content: input},
        }
    }

    // Продолжаем agent loop
    for {
        step++
        resp, err := callLLM(ctx, client, messages)
        if err != nil {
            return "", err
        }

        messages = append(messages, resp)

        // Сохраняем checkpoint после каждого шага
        if err := saveCheckpoint(ctx, store, MemoryCheckpoint{
            RunID:     runID,
            Step:      step,
            Messages:  messages,
            CreatedAt: time.Now(),
        }); err != nil {
            log.Printf("Не удалось сохранить checkpoint: %v", err)
        }

        if resp.ToolCalls == nil {
            return resp.Content, nil
        }

        // Выполняем инструменты...
    }
}
```

### Shared Memory между агентами

В мульти-агентных системах агенты обмениваются информацией через общее хранилище памяти. Каждый агент читает и пишет в общий store, разграничивая данные по namespace:

```go
type SharedMemoryStore struct {
    store CheckpointStore
}

// Запись с namespace агента
func (s *SharedMemoryStore) Put(ctx context.Context, agentID, key string, value any) error {
    fullKey := fmt.Sprintf("shared:%s:%s", agentID, key)
    data, _ := json.Marshal(value)
    return s.store.Set(ctx, fullKey, data, 0)
}

// Чтение данных другого агента
func (s *SharedMemoryStore) Get(ctx context.Context, agentID, key string) (any, error) {
    fullKey := fmt.Sprintf("shared:%s:%s", agentID, key)
    data, err := s.store.Get(ctx, fullKey)
    if err != nil {
        return nil, err
    }
    var result any
    return result, json.Unmarshal(data, &result)
}

// Получить все записи всех агентов (для supervisor)
func (s *SharedMemoryStore) ListAll(ctx context.Context) (map[string]any, error) {
    keys, _ := s.store.Keys(ctx, "shared:*")
    result := make(map[string]any)
    for _, key := range keys {
        val, _ := s.store.Get(ctx, key)
        var parsed any
        json.Unmarshal(val, &parsed)
        result[key] = parsed
    }
    return result, nil
}
```

> **Связь:** Подробнее об управлении состоянием агента — в [Главе 11: State Management](../11-state-management/README.md). Checkpoint — это частный случай персистенции состояния.

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

**Сдано:**
- [x] Понимаете разные типы памяти
- [x] Можете сохранять и извлекать информацию
- [x] Реализуете TTL и очистку
- [x] Интегрируете память с агентом

**Не сдано:**
- [ ] Нет TTL, память растёт бесконечно
- [ ] Сохранение всего без фильтрации
- [ ] Только простой поиск по ключевым словам

## Связь с другими главами

- **[Глава 09: Анатомия Агента](../09-agent-architecture/README.md)** — Память — ключевой компонент агента
- **[Глава 13: Context Engineering](../13-context-engineering/README.md)** — Память питает управление контекстом (факты из памяти используются при сборке контекста)
- **[Глава 08: Evals и Надежность](../08-evals-and-reliability/README.md)** — Память влияет на консистентность агента

**ВАЖНО:** Память (эта глава) отвечает за **хранение и извлечение** информации. Управление тем, как эта информация включается в контекст LLM, описано в [Context Engineering](../13-context-engineering/README.md).

## Что дальше?

После понимания систем памяти переходите к:
- **[13. Context Engineering](../13-context-engineering/README.md)** — Узнайте, как эффективно управлять контекстом из памяти, состояния и retrieval


