# Cost & Latency Engineering

## Зачем это нужно?

Агент работает, но счёт за LLM API растёт неконтролируемо. Один запрос стоит $5, а другой — $0.10. Почему? Без контроля стоимости и оптимизации latency вы не можете:
- Предсказать бюджет на месяц
- Оптимизировать дорогие запросы
- Гарантировать время ответа пользователю

Cost & Latency Engineering — это контроль бюджета и производительности. Без него вы рискуете потратить тысячи долларов на простые запросы или получить медленный агент, который пользователи не будут использовать.

### Реальный кейс

**Ситуация:** Агент для DevOps работает в проде неделю. Счёт за LLM API — $5000 за месяц вместо ожидаемых $500.

**Проблема:** Агент использует GPT-4 для всех запросов, даже для простых проверок статуса. Нет лимитов на токены, нет кэширования, нет fallback на более дешёвые модели. Один запрос "проверь статус сервера" использует 50,000 токенов из-за большого контекста.

**Решение:** Бюджеты токенов, лимиты итераций, кэширование, маршрутизация моделей по сложности задачи. Теперь простые запросы используют GPT-3.5 ($0.002 за 1K токенов), сложные — GPT-4 ($0.03 за 1K токенов). Счёт снизился до $600 в месяц.

## Теория простыми словами

### Что такое Cost Engineering?

Cost Engineering — это контроль и оптимизация стоимости использования LLM API. Основные рычаги:
1. **Выбор модели** — GPT-4 дороже GPT-3.5 в 15 раз
2. **Количество токенов** — чем больше контекст, тем дороже
3. **Количество запросов** — каждый вызов LLM стоит денег
4. **Кэширование** — одинаковые запросы можно не повторять

### Что такое Latency Engineering?

Latency Engineering — это контроль времени ответа. Основные факторы:
1. **Модель** — GPT-4 медленнее GPT-3.5
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

Ограничьте количество итераций ReAct loop (см. `labs/lab04-autonomy/main.go`):

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

Используйте более дешёвые модели для простых задач (см. `labs/lab09-context-optimization/main.go`):

```go
func selectModel(taskComplexity string) string {
    switch taskComplexity {
    case "simple":
        return openai.GPT3Dot5Turbo // Дешевле и быстрее
    case "complex":
        return openai.GPT4 // Лучше качество, но дороже
    default:
        return openai.GPT3Dot5Turbo
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
    models := []string{openai.GPT4, openai.GPT3Dot5Turbo}
    
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

В `labs/lab04-autonomy/main.go` добавьте проверку бюджета и лимит итераций:

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

В `labs/lab09-context-optimization/main.go` уже есть подсчёт токенов. Добавьте проверку бюджета:

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

Полный пример с контролем стоимости на базе `labs/lab04-autonomy/main.go`:

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
        return openai.GPT3Dot5Turbo
    }
    return openai.GPT4
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
req.Model = openai.GPT4 // Всегда GPT-4

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

Добавьте проверку бюджета в `labs/lab04-autonomy/main.go`:

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

✅ **Сдано (готовность к прод):**
- Реализованы бюджеты токенов с проверкой перед каждым запросом
- Установлен лимит итераций ReAct loop
- Реализована маршрутизация моделей по сложности задачи
- Реализовано кэширование результатов LLM
- Установлены таймауты для вызовов LLM и agent run
- Отслеживается использование токенов и предупреждается при превышении

❌ **Не сдано:**
- Нет лимитов на токены
- Нет лимита итераций
- Всегда используется самая дорогая модель
- Нет кэширования
- Нет таймаутов

## Связь с другими главами

- **Observability:** Логирование использования токенов — [Observability и Tracing](observability.md)
- **Context Optimization:** Управление размером контекста — [Lab 09: Context Optimization](../../../labs/lab09-context-optimization/METHOD.md)
- **Agent Loop:** Базовый цикл агента — [Глава 05: Автономность и Циклы](../05-autonomy-and-loops/README.md)

---

**Навигация:** [← Observability](observability.md) | [Оглавление главы 12](README.md) | [Workflow и State Management →](workflow_state.md)

