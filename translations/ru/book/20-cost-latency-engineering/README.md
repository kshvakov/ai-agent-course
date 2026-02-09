# 20. Cost & Latency Engineering

## Зачем это нужно?

Агент работает, но счёт за LLM API растёт неконтролируемо. Один запрос стоит $5, а другой — $0.10. Почему? Без контроля стоимости и оптимизации latency вы не можете:
- Предсказать бюджет на месяц
- Оптимизировать дорогие запросы
- Гарантировать время ответа пользователю

Cost & Latency Engineering — это контроль бюджета и производительности. Без него легко потратить тысячи долларов на простые запросы или получить медленного агента, которым никто не будет пользоваться.

### Реальный кейс

**Ситуация:** Агент для DevOps работает в проде неделю. Счёт за LLM API — $5000 за месяц вместо ожидаемых $500.

**Проблема:** Агент использует GPT-4 для всех запросов, даже для простых проверок статуса. Нет лимитов на токены, нет кэширования, нет fallback на более дешёвые модели. Один запрос "проверь статус сервера" использует 50,000 токенов из-за большого контекста.

**Решение:** Бюджеты токенов, лимиты итераций, кэширование, маршрутизация моделей по сложности задачи. Теперь простые запросы используют дешёвую модель (GPT-4o-mini), сложные — мощную (GPT-4o). Счёт снизился до $600 в месяц.

> **Примечание:** Цены на LLM API меняются часто. Актуальные тарифы проверяйте на сайтах провайдеров. Цены в этой главе приведены для иллюстрации порядка величин.

## Теория простыми словами

### Что такое Cost Engineering?

Cost Engineering — это контроль и оптимизация стоимости использования LLM API. Основные рычаги:
1. **Выбор модели** — мощные модели (GPT-4o) дороже лёгких (GPT-4o-mini) в разы
2. **Количество токенов** — чем больше контекст, тем дороже
3. **Количество запросов** — каждый вызов LLM стоит денег
4. **Кэширование** — одинаковые запросы можно не повторять

### Что такое Latency Engineering?

Latency Engineering — это контроль времени ответа. Основные факторы:
1. **Модель** — мощные модели медленнее лёгких
2. **Размер контекста** — больше токенов = больше времени обработки
3. **Количество итераций** — больше циклов ReAct = больше времени
4. **Таймауты** — защита от зависаний

## Как это работает (пошагово)

### Шаг 1: Бюджеты токенов

Установите лимит токенов на запрос:

```go
const (
    MaxTokensPerRequest = 10000
    MaxIterations      = 10
)

type TokenBudget struct {
    Used  int
    Limit int
}

func checkTokenBudget(budget TokenBudget) error {
    if budget.Used > budget.Limit {
        return fmt.Errorf("token budget exceeded: %d > %d", budget.Used, budget.Limit)
    }
    return nil
}
```

### Шаг 2: Лимиты итераций

Ограничьте количество итераций ReAct loop (см. [`labs/lab04-autonomy/main.go`](https://github.com/kshvakov/ai-agent-course/blob/main/translations/ru/labs/lab04-autonomy/main.go)):

```go
func runAgent(ctx context.Context, client *openai.Client, userInput string) (string, error) {
    maxIterations := 10
    tokenBudget := TokenBudget{Limit: MaxTokensPerRequest}
    
    for i := 0; i < maxIterations; i++ {
        resp, err := client.CreateChatCompletion(ctx, req)
        if err != nil {
            return "", err
        }
        
        tokenBudget.Used += resp.Usage.TotalTokens
        
        // Проверяем бюджет
        if err := checkTokenBudget(tokenBudget); err != nil {
            return "", fmt.Errorf("stopping: %v", err)
        }
        
        // ... остальной код ...
    }
    
    return "", fmt.Errorf("max iterations (%d) exceeded", maxIterations)
}
```

### Шаг 2.5: Артефакты для больших результатов инструментов

Один из самых быстрых способов уменьшить cost/latency — перестать складывать "толстые" результаты инструментов прямо в `messages[]`.

Паттерн:

- сохраняем сырой результат инструмента во внешнем хранилище как **артефакт**,
- в диалог добавляем только короткий excerpt (например, top-20 строк) + `artifact_id`,
- если нужно — по запросу достаём артефакт кусками (`range`/`offset`/`limit`).

Это часто дешевле и надёжнее, чем саммаризация: вы не теряете данные, просто переносите их из контекста в хранилище.

### Шаг 3: Кэширование результатов LLM

Кэшируйте результаты для одинаковых запросов:

```go
import (
    "crypto/sha256"
    "encoding/hex"
    "sync"
    "time"
)

type CacheEntry struct {
    Result   string
    ExpiresAt time.Time
}

var cache = sync.Map{} // map[string]*CacheEntry

func getCacheKey(messages []openai.ChatCompletionMessage) string {
    // Создаём ключ из содержимого сообщений
    data := ""
    for _, msg := range messages {
        data += msg.Role + ":" + msg.Content
    }
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:])
}

func getCachedResult(key string) (string, bool) {
    entry, ok := cache.Load(key)
    if !ok {
        return "", false
    }
    e := entry.(*CacheEntry)
    if time.Now().After(e.ExpiresAt) {
        cache.Delete(key)
        return "", false
    }
    return e.Result, true
}

func setCachedResult(key string, result string, ttl time.Duration) {
    cache.Store(key, &CacheEntry{
        Result:    result,
        ExpiresAt: time.Now().Add(ttl),
    })
}
```

### Шаг 4: Маршрутизация моделей по сложности

Используйте более дешёвые модели для простых задач (см. [`labs/lab09-context-optimization/main.go`](https://github.com/kshvakov/ai-agent-course/blob/main/translations/ru/labs/lab09-context-optimization/main.go)):

```go
func selectModel(taskComplexity string) string {
    switch taskComplexity {
    case "simple":
        return "gpt-4o-mini" // Дешевле и быстрее
    case "complex":
        return "gpt-4o" // Лучше качество, но дороже
    default:
        return "gpt-4o-mini"
    }
}

func assessTaskComplexity(userInput string) string {
    // Простые задачи: проверка статуса, чтение логов
    simpleKeywords := []string{"check", "status", "read", "get", "list"}
    for _, keyword := range simpleKeywords {
        if strings.Contains(strings.ToLower(userInput), keyword) {
            return "simple"
        }
    }
    // Сложные задачи: анализ, планирование, решение проблем
    return "complex"
}
```

### Шаг 5: Fallback-модели

Реализуйте цепочку fallback при ошибках или превышении бюджета:

```go
func createChatCompletionWithFallback(ctx context.Context, client *openai.Client, req openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
    models := []string{"gpt-4o", "gpt-4o-mini"}
    
    for _, model := range models {
        req.Model = model
        resp, err := client.CreateChatCompletion(ctx, req)
        if err == nil {
            return resp, nil
        }
        // Если ошибка, пробуем следующую модель
    }
    
    return nil, fmt.Errorf("all models failed")
}
```

### Шаг 6: Таймауты

Установите timeout для всего agent run и для каждого вызова инструмента:

```go
import "context"
import "time"

func runAgentWithTimeout(ctx context.Context, client *openai.Client, userInput string) (string, error) {
    // Timeout для всего agent run (5 минут)
    ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
    defer cancel()
    
    // ... agent loop ...
    
    for i := 0; i < maxIterations; i++ {
        // Timeout для каждого вызова LLM (30 секунд)
        callCtx, callCancel := context.WithTimeout(ctx, 30*time.Second)
        resp, err := client.CreateChatCompletion(callCtx, req)
        callCancel()
        
        if err != nil {
            if err == context.DeadlineExceeded {
                return "", fmt.Errorf("LLM call timeout")
            }
            return "", err
        }
        
        // ... остальной код ...
    }
}
```

## Где это встраивать в нашем коде

### Точка интеграции 1: Agent Loop

В [`labs/lab04-autonomy/main.go`](https://github.com/kshvakov/ai-agent-course/blob/main/translations/ru/labs/lab04-autonomy/main.go) добавьте проверку бюджета и лимит итераций:

```go
const MaxIterations = 10
const MaxTokensPerRequest = 10000

// В цикле:
for i := 0; i < MaxIterations; i++ {
    resp, err := client.CreateChatCompletion(ctx, req)
    // Проверяем токены
    if resp.Usage.TotalTokens > MaxTokensPerRequest {
        return "", fmt.Errorf("token limit exceeded")
    }
    // ... остальной код ...
}
```

### Точка интеграции 2: Context Optimization

В [`labs/lab09-context-optimization/main.go`](https://github.com/kshvakov/ai-agent-course/blob/main/translations/ru/labs/lab09-context-optimization/main.go) уже есть подсчёт токенов. Добавьте проверку бюджета:

```go
func adaptiveContextManagement(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
    usedTokens := countTokensInMessages(messages)
    
    // Если превышен бюджет, применяем агрессивную оптимизацию
    if usedTokens > MaxTokensPerRequest {
        return compressOldMessages(ctx, client, messages, maxTokens)
    }
    
    // ... остальная логика ...
}
```

## Мини-пример кода

Полный пример с контролем стоимости на базе [`labs/lab04-autonomy/main.go`](https://github.com/kshvakov/ai-agent-course/blob/main/translations/ru/labs/lab04-autonomy/main.go):

```go
package main

import (
    "context"
    "fmt"
    "os"
    "time"

    "github.com/sashabaranov/go-openai"
)

const (
    MaxTokensPerRequest = 10000
    MaxIterations      = 10
    LLMTimeout         = 30 * time.Second
    AgentTimeout       = 5 * time.Minute
)

type TokenBudget struct {
    Used  int
    Limit int
}

func checkTokenBudget(budget TokenBudget) error {
    if budget.Used > budget.Limit {
        return fmt.Errorf("token budget exceeded: %d > %d", budget.Used, budget.Limit)
    }
    return nil
}

func selectModel(taskComplexity string) string {
    if taskComplexity == "simple" {
        return "gpt-4o-mini"
    }
    return "gpt-4o"
}

func checkDisk() string { return "Disk Usage: 95% (CRITICAL). Large folder: /var/log" }
func cleanLogs() string { return "Logs cleaned. Freed 20GB." }

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

    ctx, cancel := context.WithTimeout(context.Background(), AgentTimeout)
    defer cancel()

    userInput := "У меня кончилось место. Разберись."
    
    // Определяем сложность задачи
    model := selectModel("simple") // Для простых задач используем GPT-3.5
    
    tokenBudget := TokenBudget{Limit: MaxTokensPerRequest}

    tools := []openai.Tool{
        {
            Type: openai.ToolTypeFunction,
            Function: &openai.FunctionDefinition{
                Name:        "check_disk",
                Description: "Check current disk usage",
            },
        },
        {
            Type: openai.ToolTypeFunction,
            Function: &openai.FunctionDefinition{
                Name:        "clean_logs",
                Description: "Delete old logs to free space",
            },
        },
    }

    messages := []openai.ChatCompletionMessage{
        {Role: openai.ChatMessageRoleSystem, Content: "You are an autonomous DevOps agent."},
        {Role: openai.ChatMessageRoleUser, Content: userInput},
    }

    fmt.Println("Starting Agent Loop...")

    for i := 0; i < MaxIterations; i++ {
        // Проверяем бюджет перед запросом
        if err := checkTokenBudget(tokenBudget); err != nil {
            fmt.Printf("Budget exceeded: %v\n", err)
            break
        }

        req := openai.ChatCompletionRequest{
            Model:    model,
            Messages: messages,
            Tools:    tools,
        }

        // Timeout для каждого вызова LLM
        callCtx, callCancel := context.WithTimeout(ctx, LLMTimeout)
        resp, err := client.CreateChatCompletion(callCtx, req)
        callCancel()

        if err != nil {
            if err == context.DeadlineExceeded {
                fmt.Printf("LLM call timeout\n")
                break
            }
            panic(fmt.Sprintf("API Error: %v", err))
        }

        msg := resp.Choices[0].Message
        messages = append(messages, msg)

        // Обновляем бюджет
        tokenBudget.Used += resp.Usage.TotalTokens
        fmt.Printf("Tokens used: %d / %d\n", tokenBudget.Used, tokenBudget.Limit)

        if len(msg.ToolCalls) == 0 {
            fmt.Println("AI:", msg.Content)
            break
        }

        for _, toolCall := range msg.ToolCalls {
            fmt.Printf("Executing tool: %s\n", toolCall.Function.Name)

            var result string
            if toolCall.Function.Name == "check_disk" {
                result = checkDisk()
            } else if toolCall.Function.Name == "clean_logs" {
                result = cleanLogs()
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
```

## Multi-model в одном Agent Run

В [Шаге 4](#шаг-4-маршрутизация-моделей-по-сложности) мы выбирали модель один раз на весь запрос: простая задача — дешёвая модель, сложная — дорогая. Но внутри одного agent run разные **стадии** требуют разного уровня интеллекта.

Подумайте: чтобы выбрать инструмент из списка, модели не нужно глубокое понимание задачи. Это как выбор отвёртки из ящика — механическое действие. А вот чтобы проанализировать результат инструмента и сформулировать ответ — нужно думать.

### Три стадии итерации

Каждая итерация цикла агента (см. [Глава 04: Автономность и Циклы](../04-autonomy-and-loops/README.md)) состоит из трёх стадий:

| Стадия | Что происходит | Нужна мощная модель? | Пример модели |
|--------|---------------|---------------------|---------------|
| 1. Выбор инструмента | LLM решает, какой tool вызвать | Нет — список tools фиксирован | GPT-4o-mini (~$0.15/1M input) |
| 2. Анализ результата | LLM интерпретирует ответ инструмента | Да — нужно понимание контекста | GPT-4o (~$2.50/1M input) |
| 3. Финальный ответ | LLM формулирует ответ пользователю | Да — качество ответа критично | GPT-4o (~$2.50/1M input) |

### Экономика: считаем конкретно

Допустим, агент выполняет задачу за 8 итераций. Каждый вызов LLM использует ~2000 токенов.

**Одна модель на все стадии (GPT-4o):**
- 8 итераций × 2 вызова (tool selection + analysis) = 16 вызовов
- 16 × ~$0.01 = **~$0.16 за задачу**

**Multi-model подход:**
- 8 вызовов tool selection × ~$0.0002 (GPT-4o-mini) = $0.0016
- 7 вызовов анализа результатов × ~$0.01 (GPT-4o) = $0.07
- 1 финальный ответ × ~$0.01 (GPT-4o) = $0.01
- Итого: **~$0.08 за задачу**

Экономия ~50% без потери качества. На 10,000 запросов в месяц это разница между $1,600 и $800.

### Реализация: ModelRouter

```go
// Stage определяет стадию итерации агента
type Stage int

const (
    StageToolSelection Stage = iota // Выбор инструмента
    StageResultAnalysis             // Анализ результата инструмента
    StageFinalAnswer                // Генерация финального ответа
)

// ModelRouter выбирает модель в зависимости от стадии итерации.
// Дешёвая модель справляется с выбором tool из фиксированного списка,
// мощная модель нужна для анализа и генерации ответа.
type ModelRouter struct {
    CheapModel    string // Для механических решений (tool selection)
    PowerfulModel string // Для анализа и генерации ответа
}

func NewModelRouter() *ModelRouter {
    return &ModelRouter{
        CheapModel:    "gpt-4o-mini",
        PowerfulModel: "gpt-4o",
    }
}

func (r *ModelRouter) SelectModel(stage Stage) string {
    switch stage {
    case StageToolSelection:
        return r.CheapModel
    case StageResultAnalysis, StageFinalAnswer:
        return r.PowerfulModel
    default:
        return r.PowerfulModel
    }
}
```

Интеграция в agent loop:

```go
func runAgentMultiModel(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage, tools []openai.Tool) (string, error) {
    router := NewModelRouter()

    for i := 0; i < MaxIterations; i++ {
        // Стадия 1: выбор инструмента — дешёвая модель
        req := openai.ChatCompletionRequest{
            Model:    router.SelectModel(StageToolSelection),
            Messages: messages,
            Tools:    tools,
        }
        resp, err := client.CreateChatCompletion(ctx, req)
        if err != nil {
            return "", err
        }

        msg := resp.Choices[0].Message
        messages = append(messages, msg)

        // Если tool calls нет — это финальный ответ,
        // но он сгенерирован дешёвой моделью. Перегенерируем мощной.
        if len(msg.ToolCalls) == 0 {
            return regenerateWithPowerfulModel(ctx, client, router, messages)
        }

        // Выполняем инструменты
        toolResults := executeTools(msg.ToolCalls)
        messages = append(messages, toolResults...)

        // Стадия 2: анализ результата — мощная модель
        // Следующая итерация цикла решит: нужен ещё tool call или финальный ответ.
        // Если нужен ещё tool call, мы снова переключимся на дешёвую модель.
    }

    return "", fmt.Errorf("max iterations exceeded")
}

// regenerateWithPowerfulModel перегенерирует финальный ответ мощной моделью.
// Дешёвая модель хорошо выбирает tools, но финальный ответ
// должен быть качественным.
func regenerateWithPowerfulModel(ctx context.Context, client *openai.Client, router *ModelRouter, messages []openai.ChatCompletionMessage) (string, error) {
    // Убираем последний ответ дешёвой модели
    messages = messages[:len(messages)-1]

    req := openai.ChatCompletionRequest{
        Model:    router.SelectModel(StageFinalAnswer),
        Messages: messages,
        // Без tools — просим только текстовый ответ
    }
    resp, err := client.CreateChatCompletion(ctx, req)
    if err != nil {
        return "", err
    }
    return resp.Choices[0].Message.Content, nil
}
```

> **Связь с другими главами:** Механизм Function Calling, который мы используем на стадии tool selection, описан в [Главе 03: Инструменты и Function Calling](../03-tools-and-function-calling/README.md). Сам цикл агента — в [Главе 04: Автономность и Циклы](../04-autonomy-and-loops/README.md).

## Streaming для снижения Perceived Latency

Latency бывает двух видов:
- **Фактическая** — время от запроса до последнего токена ответа
- **Воспринимаемая (perceived)** — время, которое пользователь *ощущает* как ожидание

Streaming не ускоряет генерацию. Модель тратит столько же времени на весь ответ. Но пользователь видит первые слова через 200-500 мс вместо того, чтобы ждать 5-10 секунд до появления полного ответа. Это принципиально меняет UX.

### Как работает streaming

Без streaming: запрос → ожидание 5 секунд → весь ответ целиком.

Со streaming: запрос → 200 мс → первый токен → токены появляются по одному → ответ готов.

Технически это Server-Sent Events (SSE) — HTTP-соединение остаётся открытым, сервер отправляет данные по мере готовности.

### Streaming в agent loop

```go
func streamAgentResponse(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage) error {
    req := openai.ChatCompletionRequest{
        Model:    "gpt-4o",
        Messages: messages,
        Stream:   true, // Включаем streaming
    }

    stream, err := client.CreateChatCompletionStream(ctx, req)
    if err != nil {
        return fmt.Errorf("stream error: %w", err)
    }
    defer stream.Close()

    for {
        chunk, err := stream.Recv()
        if errors.Is(err, io.EOF) {
            fmt.Println() // Перевод строки после ответа
            return nil
        }
        if err != nil {
            return fmt.Errorf("stream recv error: %w", err)
        }

        // Каждый chunk содержит один или несколько токенов
        if len(chunk.Choices) > 0 {
            token := chunk.Choices[0].Delta.Content
            fmt.Print(token) // Выводим токен сразу, без буферизации
        }
    }
}
```

### Streaming через SSE для веб-клиентов

Если агент работает как HTTP-сервер, streaming передаётся клиенту через SSE:

```go
func handleStream(w http.ResponseWriter, r *http.Request) {
    // Проверяем поддержку streaming
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "streaming not supported", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")

    ctx := r.Context()
    stream, err := client.CreateChatCompletionStream(ctx, req)
    if err != nil {
        return
    }
    defer stream.Close()

    for {
        chunk, err := stream.Recv()
        if errors.Is(err, io.EOF) {
            fmt.Fprintf(w, "data: [DONE]\n\n")
            flusher.Flush()
            return
        }
        if err != nil {
            return
        }

        if len(chunk.Choices) > 0 {
            token := chunk.Choices[0].Delta.Content
            // SSE формат: "data: ...\n\n"
            fmt.Fprintf(w, "data: %s\n\n", token)
            flusher.Flush() // Отправляем сразу, без буферизации
        }
    }
}
```

### Когда streaming не помогает

Streaming снижает perceived latency только для финального ответа пользователю. Промежуточные вызовы LLM (выбор инструмента, анализ результатов) не нужно стримить — пользователь их не видит. Для промежуточных вызовов важна фактическая latency. Её снижают выбором быстрой модели и оптимизацией контекста.

## Параллельное выполнение Tool Calls

LLM может вернуть несколько tool calls за одну итерацию (см. [Глава 03: Инструменты и Function Calling](../03-tools-and-function-calling/README.md)). Если инструменты независимы друг от друга, их можно выполнить параллельно.

Параллельное выполнение **не экономит токены** — LLM всё равно сгенерировал все tool calls за один запрос. Но оно **сокращает wall-clock time** — время от начала до конца итерации.

### Когда безопасно параллелить

| Безопасно | Небезопасно |
|-----------|-------------|
| `check_disk` + `check_memory` — читают разные ресурсы | `create_file` → `write_to_file` — второй зависит от первого |
| `get_logs("app-1")` + `get_logs("app-2")` — разные серверы | `stop_service` → `deploy` → `start_service` — строгий порядок |
| `dns_lookup` + `ping` — независимые проверки | `read_config` → `apply_config` — сначала прочитать, потом применить |

Правило: если результат одного инструмента нужен как вход для другого — выполнять последовательно. Если нет — параллельно.

### Реализация: executeToolsParallel

```go
// ToolResult хранит результат выполнения одного инструмента
type ToolResult struct {
    ToolCallID string
    Content    string
    Err        error
}

// executeToolsParallel запускает все tool calls одновременно.
// Каждый инструмент выполняется в отдельной горутине.
func executeToolsParallel(ctx context.Context, toolCalls []openai.ToolCall) []openai.ChatCompletionMessage {
    results := make([]ToolResult, len(toolCalls))

    var wg sync.WaitGroup
    for i, tc := range toolCalls {
        wg.Add(1)
        go func(idx int, call openai.ToolCall) {
            defer wg.Done()

            // Таймаут на каждый инструмент отдельно
            toolCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
            defer cancel()

            content, err := executeTool(toolCtx, call.Function.Name, call.Function.Arguments)
            results[idx] = ToolResult{
                ToolCallID: call.ID,
                Content:    content,
                Err:        err,
            }
        }(i, tc)
    }
    wg.Wait()

    // Собираем результаты в формате messages
    var messages []openai.ChatCompletionMessage
    for _, r := range results {
        content := r.Content
        if r.Err != nil {
            content = fmt.Sprintf("error: %v", r.Err)
        }
        messages = append(messages, openai.ChatCompletionMessage{
            Role:       openai.ChatMessageRoleTool,
            Content:    content,
            ToolCallID: r.ToolCallID,
        })
    }
    return messages
}
```

Интеграция в agent loop — заменяем последовательное выполнение на параллельное:

```go
for i := 0; i < MaxIterations; i++ {
    resp, err := client.CreateChatCompletion(ctx, req)
    if err != nil {
        return "", err
    }

    msg := resp.Choices[0].Message
    messages = append(messages, msg)

    if len(msg.ToolCalls) == 0 {
        return msg.Content, nil
    }

    // Было: последовательное выполнение
    // for _, tc := range msg.ToolCalls { ... }

    // Стало: параллельное выполнение
    toolMessages := executeToolsParallel(ctx, msg.ToolCalls)
    messages = append(messages, toolMessages...)
}
```

### Сколько экономим по времени

Допустим, агент вызвал 3 инструмента за одну итерацию. Каждый инструмент выполняется ~2 секунды.

- **Последовательно:** 2 + 2 + 2 = 6 секунд
- **Параллельно:** max(2, 2, 2) = 2 секунды

На 8 итерациях с 2-3 tool calls каждая — это разница между 48 секундами и 16 секундами. Пользователь ждёт в 3 раза меньше.

> **Связь с Главой 04:** В [Главе 04: Автономность и Циклы](../04-autonomy-and-loops/README.md) tool calls выполняются последовательно. Параллельное выполнение — естественная оптимизация, когда агент стабильно работает и вы начинаете оптимизировать latency.

## Типовые ошибки

### Ошибка 1: Нет лимитов на токены

**Симптом:** Счёт за LLM API растёт неконтролируемо. Один запрос может использовать 100,000 токенов.

**Причина:** Не проверяется использование токенов перед отправкой запроса.

**Решение:**
```go
// ПЛОХО
resp, _ := client.CreateChatCompletion(ctx, req)
// Нет проверки токенов

// ХОРОШО
tokenBudget.Used += resp.Usage.TotalTokens
if err := checkTokenBudget(tokenBudget); err != nil {
    return "", err
}
```

### Ошибка 2: Нет лимита итераций

**Симптом:** Агент зацикливается и делает 50+ итераций, тратя тысячи токенов.

**Причина:** Нет ограничения на количество итераций ReAct loop.

**Решение:**
```go
// ПЛОХО
for {
    // Бесконечный цикл
}

// ХОРОШО
for i := 0; i < MaxIterations; i++ {
    // Ограниченное количество итераций
}
```

### Ошибка 3: Использование GPT-4 для всех задач

**Симптом:** Простые запросы стоят в 15 раз дороже, чем нужно.

**Причина:** Всегда используется самая дорогая модель.

**Решение:**
```go
// ПЛОХО
req.Model = "gpt-4o" // Всегда GPT-4

// ХОРОШО
model := selectModel(assessTaskComplexity(userInput))
req.Model = model
```

### Ошибка 4: Нет кэширования

**Симптом:** Одинаковые запросы выполняются повторно, тратя токены.

**Причина:** Нет кэша для результатов LLM.

**Решение:**
```go
// ПЛОХО
resp, _ := client.CreateChatCompletion(ctx, req)
// Каждый раз новый запрос

// ХОРОШО
key := getCacheKey(messages)
if result, ok := getCachedResult(key); ok {
    return result, nil
}
resp, _ := client.CreateChatCompletion(ctx, req)
setCachedResult(key, resp.Choices[0].Message.Content, 1*time.Hour)
```

### Ошибка 5: Нет таймаутов

**Симптом:** Агент зависает на 10+ минут, пользователь ждёт.

**Причина:** Нет timeout для вызовов LLM или инструментов.

**Решение:**
```go
// ПЛОХО
resp, _ := client.CreateChatCompletion(ctx, req)
// Может зависнуть навсегда

// ХОРОШО
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()
resp, err := client.CreateChatCompletion(ctx, req)
if err == context.DeadlineExceeded {
    return "", fmt.Errorf("timeout")
}
```

## Мини-упражнения

### Упражнение 1: Реализуйте проверку бюджета токенов

Добавьте проверку бюджета в [`labs/lab04-autonomy/main.go`](https://github.com/kshvakov/ai-agent-course/blob/main/translations/ru/labs/lab04-autonomy/main.go):

```go
func checkTokenBudget(used int, limit int) error {
    // Ваш код здесь
    // Верните ошибку, если превышен лимит
}
```

**Ожидаемый результат:**
- Функция возвращает ошибку, если использовано токенов больше лимита
- Функция возвращает nil, если лимит не превышен

### Упражнение 2: Реализуйте маршрутизацию моделей

Создайте функцию выбора модели по сложности задачи:

```go
func selectModelByComplexity(userInput string) string {
    // Ваш код здесь
    // Верните GPT-3.5 для простых задач, GPT-4 для сложных
}
```

**Ожидаемый результат:**
- Простые задачи (check, status, read) → GPT-3.5
- Сложные задачи (analyze, fix, plan) → GPT-4

### Упражнение 3: Реализуйте кэширование

Добавьте простое in-memory кэширование результатов LLM:

```go
var cache = make(map[string]CacheEntry)

func getCachedResult(key string) (string, bool) {
    // Ваш код здесь
}

func setCachedResult(key string, result string, ttl time.Duration) {
    // Ваш код здесь
}
```

**Ожидаемый результат:**
- Одинаковые запросы возвращаются из кэша
- Кэш имеет TTL (время жизни)

## Критерии сдачи / Чек-лист

**Сдано (готовность к прод):**
- [x] Реализованы бюджеты токенов с проверкой перед каждым запросом
- [x] Установлен лимит итераций ReAct loop
- [x] Реализована маршрутизация моделей по сложности задачи
- [x] Реализовано кэширование результатов LLM
- [x] Установлены таймауты для вызовов LLM и agent run
- [x] Отслеживается использование токенов и предупреждается при превышении

**Не сдано:**
- [ ] Нет лимитов на токены
- [ ] Нет лимита итераций
- [ ] Всегда используется самая дорогая модель
- [ ] Нет кэширования
- [ ] Нет таймаутов

## Связь с другими главами

- **Observability:** Логирование использования токенов — [Глава 19: Observability и Tracing](../19-observability-and-tracing/README.md)
- **Context Optimization:** Управление размером контекста — [Глава 13: Context Engineering](../13-context-engineering/README.md)
- **Agent Loop:** Базовый цикл агента — [Глава 04: Автономность и Циклы](../04-autonomy-and-loops/README.md)


