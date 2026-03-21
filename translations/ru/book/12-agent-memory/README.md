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
        Model:    "gpt-4o-mini",
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

## Блочная память (Block Memory)

Простой `SimpleMemory` на map подходит для учебных целей. В production-агенте память устроена сложнее. Ключевая идея: **блочная архитектура**.

Блок (Block) — это единица взаимодействия. Один запрос пользователя, цепочка tool calls, финальный ответ — всё это один блок. Блок хранит оригинальные сообщения и их компактную версию.

### Жизненный цикл блока

```mermaid
flowchart LR
    A[Prepare] --> B[Append]
    B --> C[BuildContext]
    C --> D[Close]
    D --> E["Compact + Summary"]
```

1. **Prepare** — создаём новый блок, привязываем к хранилищу
2. **Append** — добавляем сообщения (user, assistant, tool results)
3. **BuildContext** — собираем историю из текущего и прошлых блоков
4. **Close** — финализируем: создаём compact-версию, считаем токены, генерируем summary

### Compact: 91% сжатие tool chains

При закрытии блока цепочки tool calls схлопываются в компактную форму:

```go
type Block struct {
    ID       int
    Query    string
    Messages []Message        // оригинал
    Compact  []Message        // сжатая версия
    Summary  string
    Tokens   int
}

func (b *Block) Close() {
    b.Query = extractQuery(b.Messages)      // первые 120 символов user-сообщения
    b.Compact = compactMessages(b.Messages)
    b.Tokens = estimateTokens(b.Compact)    // chars / 3
    b.Summary = buildSummary(b.Query, b.Messages, b.Compact)
}
```

Функция `compactMessages` схлопывает цепочки `assistant(tool_calls) → tool_result` в одно сообщение с тегами `<prior_tool_use>`:

```go
func compactMessages(msgs []Message) []Message {
    var result []Message
    var toolBuf strings.Builder

    for _, msg := range msgs {
        if msg.HasToolCalls() {
            for _, tc := range msg.ToolCalls {
                args := truncate(tc.Arguments, 80)
                res := truncate(findToolResult(msgs, tc.ID), 200)
                line := fmt.Sprintf("%s %s -- %s", tc.Name, args, res)
                if !isDuplicate(toolBuf.String(), line) {
                    toolBuf.WriteString(line + "\n")
                }
            }
            continue
        }
        if msg.Role == "tool" {
            continue // уже обработано выше
        }
        if toolBuf.Len() > 0 {
            result = append(result, Message{
                Role:    "user", // не assistant — модель не имитирует свой контент
                Content: "<prior_tool_use>\n" + toolBuf.String() + "</prior_tool_use>",
            })
            toolBuf.Reset()
        }
        result = append(result, msg)
    }
    return result
}
```

Compact-сообщения получают роль `user`, а не `assistant`. Это предотвращает ситуацию, когда модель воспринимает сжатый контент как свои предыдущие слова и начинает их имитировать.

### BuildContext: бюджетирование блоков

При сборке контекста мы берём compact-версии прошлых блоков (от новых к старым), пока хватает бюджета:

```go
func (m *Memory) BuildContext(currentBlock *Block, contextSize int) []Message {
    budget := contextSize - currentBlock.Tokens - 4096 // резерв

    var history []Message
    // от новых блоков к старым
    for i := len(m.blocks) - 1; i >= 0 && budget > 0; i-- {
        block := m.blocks[i]
        if block.Tokens > budget {
            break
        }
        history = append(block.Compact, history...)
        budget -= block.Tokens
    }

    return append(history, currentBlock.Messages...)
}
```

## Каталог блоков и Recall

Модель не видит содержимое прошлых блоков напрямую. Вместо этого в system prompt рендерится **каталог** — список блоков с краткими описаниями:

```
[CONTEXT BLOCKS]
Blocks from prior interactions (use recall tool to get full details):
#0: check disk usage — exec x3, read x1 → "Disk /data at 92%, cleaned logs" (~5K tokens)
#1: deploy service — edit x2, exec x4 → "Deployed v2.3.1 to staging" (~8K tokens)
#2: fix nginx config — read x2, edit x1 → "Updated proxy_pass for /api" (~3K tokens)
```

Когда модели нужны подробности, она вызывает инструмент `recall`:

```go
type RecallTool struct {
    memory *Memory
}

func (t *RecallTool) Execute(ctx context.Context, args RecallArgs) (string, error) {
    msgs := t.memory.BlockMessages(args.BlockID)
    if msgs == nil {
        return "Block not found", nil
    }
    return formatMessages(msgs), nil // полные оригинальные сообщения
}
```

Recall возвращает **оригинальные** сообщения блока, а не compact-версию. Это критично: compact теряет детали, а для глубокого анализа нужна полная информация.

## Working Memory (Рабочая память)

Working Memory — это динамическая секция system prompt, которая содержит контекст текущей задачи. В отличие от block memory (прошлые взаимодействия), Working Memory — это "что агент знает прямо сейчас".

### Компоненты Working Memory

| Компонент | Назначение |
|-----------|-----------|
| **TaskContext** | Текущая задача (первые 50 слов), прочитанные/изменённые файлы, последние действия |
| **LivePlan** | Цель, шаги с статусами (pending/in_progress/completed/cancelled) |
| **Budget** | Предупреждения о заполнении контекстного окна |
| **ContextBlocks** | Каталог прошлых блоков для recall |

### Render в system prompt

```go
type WorkingMemory struct {
    Task         string
    FilesRead    *Ring[string]  // кольцевой буфер, MRU-семантика
    FilesModified *Ring[string]
    LastActions  *Ring[string]
    Plan         *LivePlan
    Budget       *BudgetTracker
    Blocks       []BlockSummary
    maxChars     int            // ~6000 символов
}

func (wm *WorkingMemory) Render() string {
    var sb strings.Builder

    sb.WriteString("[TASK CONTEXT]\n")
    sb.WriteString("Task: " + wm.Task + "\n")
    sb.WriteString("Files read: " + wm.FilesRead.Join(", ") + "\n")
    sb.WriteString("Files modified: " + wm.FilesModified.Join(", ") + "\n")
    sb.WriteString("Recent: " + wm.LastActions.Join(", ") + "\n")

    if wm.Plan != nil {
        sb.WriteString("\n" + wm.Plan.Render() + "\n")
    }
    if wm.Budget.ShouldWarn() {
        sb.WriteString("\n[BUDGET]\n" + wm.Budget.Warning() + "\n")
    }
    if len(wm.Blocks) > 0 {
        sb.WriteString("\n[CONTEXT BLOCKS]\n")
        for _, b := range wm.Blocks {
            sb.WriteString(fmt.Sprintf("#%d: %s (~%dK tokens)\n", b.ID, b.Summary, b.Tokens/1000))
        }
    }

    // Trimming при превышении бюджета
    if sb.Len() > wm.maxChars {
        wm.LastActions.Trim(3)
        wm.FilesRead.Trim(5)
    }

    return sb.String()
}
```

### Проблема: Working Memory не персистентна

Working Memory живёт в оперативной памяти. Между сессиями она теряется: агент забывает план, перечитывает файлы, начинает с нуля.

Решение: передавать Working Memory как параметр в `Run()`, чтобы она переживала между REPL-циклами. Для персистенции между сессиями — реализовать Export/Import:

```go
func (wm *WorkingMemory) Export() WorkingMemorySnapshot {
    return WorkingMemorySnapshot{
        Task:          wm.Task,
        FilesRead:     wm.FilesRead.Items(),
        FilesModified: wm.FilesModified.Items(),
        Plan:          wm.Plan.Export(),
    }
}

func (wm *WorkingMemory) Restore(snap WorkingMemorySnapshot) {
    wm.Task = snap.Task
    for _, f := range snap.FilesRead {
        wm.FilesRead.Push(f)
    }
    // ...
}
```

> **4 уровня памяти в production-агенте:**
>
> 1. **Working Memory** — задача, план, бюджет, файлы (в system prompt)
> 2. **Block Memory** — завершённые взаимодействия (original + compact)
> 3. **Recall** — model-driven retrieval полных данных блока
> 4. **Condense** — emergency LLM-сжатие при overflow (см. [Context Engineering](../13-context-engineering/README.md))

## Checkpoint и Resume

Агент может работать часами над сложной задачей. Если процесс упадёт посередине, потеряется весь прогресс. Checkpoint (чекпоинт) сохраняет состояние разговора периодически. При сбое агент возобновляет работу с последней сохранённой точки.

Базовая реализация Checkpoint (структура, сохранение/загрузка, интеграция с agent loop) описана в [Главе 09: Анатомия Агента](../09-agent-architecture/README.md#checkpoint-и-resume-сохранение-и-восстановление). Продвинутые стратегии (гранулярность, валидация, ротация) — в [Главе 11: State Management](../11-state-management/README.md#продвинутые-стратегии-checkpoint).

Здесь мы рассматриваем, как Checkpoint связан с памятью агента:

- **Что сохранять:** историю сообщений (`messages[]`), содержимое памяти, состояние инструментов, текущий шаг выполнения.
- **Когда сохранять:** после каждого значимого шага (вызов инструмента, ответ пользователю). Для коротких задач (2-3 итерации) Checkpoint избыточен. Для длинных задач (10+ итераций) — обязателен.
- **TTL:** устанавливайте TTL на Checkpoint (например, 24 часа), чтобы устаревшие снимки не накапливались.

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

### Ошибка 4: Compact уничтожает оригиналы

**Симптом:** После компакции невозможно восстановить детали tool calls. Recall возвращает сжатую версию.

**Причина:** Оригинальные сообщения удалены при создании compact-версии.

**Решение:**

```go
// ПЛОХО: перезаписываем оригинал
block.Messages = compactMessages(block.Messages)

// ХОРОШО: храним оба варианта
block.Compact = compactMessages(block.Messages)
// block.Messages остаётся без изменений
```

Принцип **Never destroy originals**: condensation — это view, не мутация. Оригиналы нужны для recall по полной истории.

### Ошибка 5: Программная компакция mid-loop

**Симптом:** Агент теряет контекст посередине задачи, начинает заново или повторяет действия.

**Причина:** Компакция вызывается внутри цикла агента, пока задача ещё выполняется.

**Решение:** Compact — только при `Block.Close()`, когда взаимодействие завершено. Внутри цикла используйте LLM-конденсацию (condense), которая создаёт осмысленное резюме вместо механического сжатия.

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

### Упражнение 3: Реализуйте Block Memory с Compact

Реализуйте блочную память с компакцией tool chains:

```go
type BlockMemory struct {
    blocks []*Block
    current *Block
}

func (m *BlockMemory) NewBlock() *Block {
    // Создать новый блок
}

func (m *BlockMemory) CloseBlock() {
    // Закрыть текущий блок: compact + summary
}

func (m *BlockMemory) BuildContext(contextSize int) []Message {
    // Собрать историю из compact-версий прошлых блоков + текущий блок
}
```

**Ожидаемый результат:**
- Compact сжимает цепочку из 10 tool calls в 1-2 сообщения
- BuildContext укладывается в бюджет токенов
- Оригинальные сообщения сохраняются для recall

## Критерии сдачи / Чек-лист

**Сдано:**
- [x] Понимаете разные типы памяти (включая блочную и рабочую)
- [x] Можете сохранять и извлекать информацию
- [x] Реализуете TTL и очистку
- [x] Интегрируете память с агентом
- [x] Понимаете разницу между compact и оригиналом
- [x] Знаете, зачем нужна Working Memory

**Не сдано:**
- [ ] Нет TTL, память растёт бесконечно
- [ ] Сохранение всего без фильтрации
- [ ] Только простой поиск по ключевым словам
- [ ] Compact уничтожает оригиналы
- [ ] Нет блочной структуры — вся история в одном массиве

## Связь с другими главами

- **[Глава 09: Анатомия Агента](../09-agent-architecture/README.md)** — Память — ключевой компонент агента
- **[Глава 13: Context Engineering](../13-context-engineering/README.md)** — Память питает управление контекстом (факты из памяти используются при сборке контекста)
- **[Глава 08: Evals и Надежность](../08-evals-and-reliability/README.md)** — Память влияет на консистентность агента

**ВАЖНО:** Память (эта глава) отвечает за **хранение и извлечение** информации. Управление тем, как эта информация включается в контекст LLM, описано в [Context Engineering](../13-context-engineering/README.md).

## Что дальше?

После понимания систем памяти переходите к:
- **[13. Context Engineering](../13-context-engineering/README.md)** — Узнайте, как эффективно управлять контекстом из памяти, состояния и retrieval


