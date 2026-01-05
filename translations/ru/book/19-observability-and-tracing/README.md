# 19. Observability и Tracing

## Зачем это нужно?

Без observability вы работаете вслепую. Агент выполнил действие, но вы не можете понять:
- Почему он выбрал именно этот инструмент?
- Сколько времени заняло выполнение?
- Сколько токенов было использовано?
- Где произошла ошибка?

Observability — это "зрение" для вашего агента. Без него вы не можете отлаживать проблемы, оптимизировать производительность или понимать поведение агента в проде.

### Реальный кейс

**Ситуация:** Агент в проде выполнил операцию удаления данных без подтверждения. Пользователь пожаловался.

**Проблема:** Вы не можете понять, почему агент не запросил подтверждение. В логах только "Agent executed delete_database". Нет информации о том, какие инструменты были доступны, какой промпт использовался, какие аргументы были переданы.

**Решение:** Структурированное логирование с трейсингом позволяет увидеть полную картину: входной запрос → выбор инструментов → выполнение → результат. Теперь вы можете понять, что агент не увидел инструмент с подтверждением или промпт был неправильным.

## Теория простыми словами

### Что такое Observability?

Observability — это способность понять, что происходит внутри системы, наблюдая за её выходными данными (логами, метриками, трейсами).

**Три столпа observability:**
1. **Логи** — что произошло (события, ошибки, действия)
2. **Метрики** — сколько и как часто (latency, token usage, error rate)
3. **Трейсы** — как это связано (полный путь запроса через систему)

### Как работает трейсинг в агентах?

Каждый agent run — это цепочка событий:
1. Пользователь отправляет запрос → создаётся `run_id`
2. Агент анализирует запрос → логируем входные данные
3. Агент выбирает инструменты → логируем `tool_calls`
4. Инструменты выполняются → логируем результаты
5. Агент формирует ответ → логируем финальный ответ

Все эти события связаны через `run_id`, что позволяет восстановить полную картину выполнения.

## Как это работает (пошагово)

### Шаг 1: Структура для логирования agent run

Создайте структуру для хранения информации о запуске агента:

```go
type AgentRun struct {
    RunID       string    `json:"run_id"`
    UserInput   string    `json:"user_input"`
    ToolCalls   []ToolCall `json:"tool_calls"`
    ToolResults []ToolResult `json:"tool_results"`
    FinalAnswer string    `json:"final_answer"`
    TokensUsed  int       `json:"tokens_used"`
    Latency     time.Duration `json:"latency"`
    Timestamp   time.Time `json:"timestamp"`
}

type ToolCall struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    Arguments string    `json:"arguments"`
    Timestamp time.Time `json:"timestamp"`
}

type ToolResult struct {
    ToolCallID string    `json:"tool_call_id"`
    Result     string    `json:"result"`
    Error      string    `json:"error,omitempty"`
    Latency    time.Duration `json:"latency"`
    Timestamp  time.Time `json:"timestamp"`
}
```

### Шаг 2: Генерация уникального run_id

В начале каждого agent run генерируйте уникальный ID:

```go
import (
    "crypto/rand"
    "encoding/hex"
    "time"
)

func generateRunID() string {
    bytes := make([]byte, 8)
    rand.Read(bytes)
    return hex.EncodeToString(bytes) + "-" + time.Now().Format("20060102-150405")
}
```

### Шаг 3: Логирование в agent loop

Вставьте логирование в цикл агента (см. `labs/lab04-autonomy/main.go`):

```go
func runAgent(ctx context.Context, client *openai.Client, userInput string) (string, error) {
    runID := generateRunID()
    startTime := time.Now()
    
    // Создаём структуру для логирования
    run := AgentRun{
        RunID:     runID,
        UserInput: userInput,
        ToolCalls: []ToolCall{},
        Timestamp: time.Now(),
    }
    
    messages := []openai.ChatCompletionMessage{
        {Role: openai.ChatMessageRoleSystem, Content: "You are an autonomous DevOps agent."},
        {Role: openai.ChatMessageRoleUser, Content: userInput},
    }
    
    // THE LOOP с логированием
    for i := 0; i < 5; i++ {
        req := openai.ChatCompletionRequest{
            Model:    openai.GPT3Dot5Turbo,
            Messages: messages,
            Tools:    tools,
        }
        
        resp, err := client.CreateChatCompletion(ctx, req)
        if err != nil {
            logAgentRunError(runID, err)
            return "", err
        }
        
        msg := resp.Choices[0].Message
        messages = append(messages, msg)
        
        // Логируем использование токенов
        run.TokensUsed += resp.Usage.TotalTokens
        
        if len(msg.ToolCalls) == 0 {
            run.FinalAnswer = msg.Content
            run.Latency = time.Since(startTime)
            logAgentRun(run)
            return msg.Content, nil
        }
        
        // Логируем tool calls
        for _, toolCall := range msg.ToolCalls {
            toolCallLog := ToolCall{
                ID:        toolCall.ID,
                Name:      toolCall.Function.Name,
                Arguments: toolCall.Function.Arguments,
                Timestamp: time.Now(),
            }
            run.ToolCalls = append(run.ToolCalls, toolCallLog)
            
            // Выполняем инструмент с логированием
            result, err := executeToolWithLogging(runID, toolCall)
            if err != nil {
                run.ToolResults = append(run.ToolResults, ToolResult{
                    ToolCallID: toolCall.ID,
                    Error:      err.Error(),
                    Timestamp:  time.Now(),
                })
            } else {
                run.ToolResults = append(run.ToolResults, ToolResult{
                    ToolCallID: toolCall.ID,
                    Result:     result,
                    Timestamp:  time.Now(),
                })
            }
            
            messages = append(messages, openai.ChatCompletionMessage{
                Role:       openai.ChatMessageRoleTool,
                Content:    result,
                ToolCallID: toolCall.ID,
            })
        }
    }
    
    run.Latency = time.Since(startTime)
    logAgentRun(run)
    return run.FinalAnswer, nil
}
```

### Шаг 4: Структурированное логирование

Реализуйте функцию логирования в JSON формате:

```go
import (
    "encoding/json"
    "log"
)

func logAgentRun(run AgentRun) {
    logJSON, err := json.Marshal(run)
    if err != nil {
        log.Printf("Failed to marshal agent run: %v", err)
        return
    }
    log.Printf("AGENT_RUN: %s", string(logJSON))
}

func logAgentRunError(runID string, err error) {
    logJSON, _ := json.Marshal(map[string]any{
        "run_id": runID,
        "error":  err.Error(),
        "timestamp": time.Now(),
    })
    log.Printf("AGENT_RUN_ERROR: %s", string(logJSON))
}
```

### Шаг 5: Логирование tool execution

В функции выполнения инструментов (см. `labs/lab02-tools/main.go`) добавьте логирование:

```go
func executeToolWithLogging(runID string, toolCall openai.ToolCall) (string, error) {
    startTime := time.Now()
    
    log.Printf("TOOL_CALL_START: run_id=%s tool_id=%s tool_name=%s args=%s",
        runID, toolCall.ID, toolCall.Function.Name, toolCall.Function.Arguments)
    
    var result string
    var err error
    
    switch toolCall.Function.Name {
    case "check_disk":
        result = checkDisk()
    case "clean_logs":
        result = cleanLogs()
    default:
        err = fmt.Errorf("unknown tool: %s", toolCall.Function.Name)
    }
    
    latency := time.Since(startTime)
    
    if err != nil {
        log.Printf("TOOL_CALL_ERROR: run_id=%s tool_id=%s tool_name=%s error=%s latency=%v",
            runID, toolCall.ID, toolCall.Function.Name, err.Error(), latency)
        return "", err
    }
    
    log.Printf("TOOL_CALL_SUCCESS: run_id=%s tool_id=%s tool_name=%s latency=%v",
        runID, toolCall.ID, toolCall.Function.Name, latency)
    
    return result, nil
}
```

## Где это встраивать в нашем коде

### Точка интеграции 1: Agent Loop

В `labs/lab04-autonomy/main.go` добавьте логирование в цикл агента:

```go
// В начале main():
runID := generateRunID()
startTime := time.Now()

// В цикле перед CreateChatCompletion:
log.Printf("AGENT_ITERATION: run_id=%s iteration=%d", runID, i)

// После получения ответа:
log.Printf("AGENT_RESPONSE: run_id=%s tokens_used=%d", runID, resp.Usage.TotalTokens)

// После выполнения инструментов:
log.Printf("TOOL_EXECUTED: run_id=%s tool=%s result=%s", runID, toolCall.Function.Name, result)
```

### Точка интеграции 2: Tool Execution

В `labs/lab02-tools/main.go` добавьте логирование при выполнении инструментов:

```go
// В функции выполнения инструмента:
func executeTool(runID string, toolCall openai.ToolCall) (string, error) {
    log.Printf("TOOL_START: run_id=%s tool=%s", runID, toolCall.Function.Name)
    // ... выполнение ...
    log.Printf("TOOL_END: run_id=%s tool=%s result=%s", runID, toolCall.Function.Name, result)
    return result, nil
}
```

## Мини-пример кода

Полный пример с observability на базе `labs/lab04-autonomy/main.go`:

```go
package main

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/sashabaranov/go-openai"
)

type AgentRun struct {
    RunID       string      `json:"run_id"`
    UserInput   string      `json:"user_input"`
    ToolCalls   []ToolCall  `json:"tool_calls"`
    ToolResults []ToolResult `json:"tool_results"`
    FinalAnswer string      `json:"final_answer"`
    TokensUsed  int         `json:"tokens_used"`
    Latency     string      `json:"latency"`
    Timestamp   time.Time   `json:"timestamp"`
}

type ToolCall struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    Arguments string    `json:"arguments"`
    Timestamp time.Time `json:"timestamp"`
}

type ToolResult struct {
    ToolCallID string    `json:"tool_call_id"`
    Result     string    `json:"result"`
    Error      string    `json:"error,omitempty"`
    Latency    string    `json:"latency"`
    Timestamp  time.Time `json:"timestamp"`
}

func generateRunID() string {
    bytes := make([]byte, 8)
    rand.Read(bytes)
    return hex.EncodeToString(bytes) + "-" + time.Now().Format("20060102-150405")
}

func logAgentRun(run AgentRun) {
    logJSON, err := json.Marshal(run)
    if err != nil {
        log.Printf("Failed to marshal: %v", err)
        return
    }
    log.Printf("AGENT_RUN: %s", string(logJSON))
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

    ctx := context.Background()
    userInput := "У меня кончилось место. Разберись."

    // Генерируем run_id
    runID := generateRunID()
    startTime := time.Now()

    run := AgentRun{
        RunID:     runID,
        UserInput: userInput,
        Timestamp: time.Now(),
    }

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

    fmt.Printf("Starting Agent Loop (run_id: %s)...\n", runID)

    for i := 0; i < 5; i++ {
        log.Printf("AGENT_ITERATION: run_id=%s iteration=%d", runID, i)

        req := openai.ChatCompletionRequest{
            Model:    openai.GPT3Dot5Turbo,
            Messages: messages,
            Tools:    tools,
        }

        resp, err := client.CreateChatCompletion(ctx, req)
        if err != nil {
            log.Printf("AGENT_ERROR: run_id=%s error=%v", runID, err)
            panic(fmt.Sprintf("API Error: %v", err))
        }

        msg := resp.Choices[0].Message
        messages = append(messages, msg)

        run.TokensUsed += resp.Usage.TotalTokens
        log.Printf("AGENT_RESPONSE: run_id=%s tokens_used=%d", runID, resp.Usage.TotalTokens)

        if len(msg.ToolCalls) == 0 {
            run.FinalAnswer = msg.Content
            run.Latency = time.Since(startTime).String()
            logAgentRun(run)
            fmt.Println("AI:", msg.Content)
            break
        }

        for _, toolCall := range msg.ToolCalls {
            toolCallStart := time.Now()
            
            toolCallLog := ToolCall{
                ID:        toolCall.ID,
                Name:      toolCall.Function.Name,
                Arguments: toolCall.Function.Arguments,
                Timestamp: time.Now(),
            }
            run.ToolCalls = append(run.ToolCalls, toolCallLog)

            fmt.Printf("Executing tool: %s\n", toolCall.Function.Name)
            log.Printf("TOOL_START: run_id=%s tool_id=%s tool_name=%s", runID, toolCall.ID, toolCall.Function.Name)

            var result string
            if toolCall.Function.Name == "check_disk" {
                result = checkDisk()
            } else if toolCall.Function.Name == "clean_logs" {
                result = cleanLogs()
            }

            toolCallLatency := time.Since(toolCallStart)
            log.Printf("TOOL_END: run_id=%s tool_id=%s tool_name=%s latency=%v", runID, toolCall.ID, toolCall.Function.Name, toolCallLatency)

            run.ToolResults = append(run.ToolResults, ToolResult{
                ToolCallID: toolCall.ID,
                Result:     result,
                Latency:    toolCallLatency.String(),
                Timestamp:  time.Now(),
            })

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

### Ошибка 1: Нет структурированного логирования

**Симптом:** Логи в виде простого текста: "Agent executed tool check_disk". Невозможно автоматически парсить или анализировать логи.

**Причина:** Использование `fmt.Printf` вместо структурированного JSON логирования.

**Решение:**
```go
// ПЛОХО
fmt.Printf("Agent executed tool %s\n", toolName)

// ХОРОШО
logJSON, _ := json.Marshal(map[string]any{
    "run_id": runID,
    "tool": toolName,
    "timestamp": time.Now(),
})
log.Printf("TOOL_EXECUTED: %s", string(logJSON))
```

### Ошибка 2: Нет run_id для кореляции

**Симптом:** Невозможно связать логи разных компонентов (агент, инструменты, внешние системы) для одного запроса.

**Причина:** Каждый компонент логирует независимо, без общего идентификатора.

**Решение:**
```go
// ПЛОХО
log.Printf("Tool executed: %s", toolName)

// ХОРОШО
runID := generateRunID() // В начале agent run
log.Printf("TOOL_EXECUTED: run_id=%s tool=%s", runID, toolName)
```

### Ошибка 3: Не логируются токены и latency

**Симптом:** Невозможно понять, сколько стоит выполнение или где узкие места по производительности.

**Причина:** Не отслеживается использование токенов и время выполнения.

**Решение:**
```go
// ПЛОХО
resp, _ := client.CreateChatCompletion(ctx, req)
// Токены не логируются

// ХОРОШО
resp, _ := client.CreateChatCompletion(ctx, req)
log.Printf("TOKENS_USED: run_id=%s tokens=%d", runID, resp.Usage.TotalTokens)

startTime := time.Now()
// ... выполнение ...
log.Printf("LATENCY: run_id=%s latency=%v", runID, time.Since(startTime))
```

### Ошибка 4: Логирование только успешных случаев

**Симптом:** Ошибки не логируются, невозможно понять, почему агент упал.

**Причина:** Логирование только в успешных ветках кода.

**Решение:**
```go
// ПЛОХО
result, err := executeTool(toolCall)
if err != nil {
    return "", err // Ошибка не логируется
}

// ХОРОШО
result, err := executeTool(toolCall)
if err != nil {
    log.Printf("TOOL_ERROR: run_id=%s tool=%s error=%v", runID, toolCall.Function.Name, err)
    return "", err
}
```

### Ошибка 5: Нет метрик (только логи)

**Симптом:** Невозможно построить графики latency, error rate, token usage во времени.

**Причина:** Логируются только события, но не агрегируются метрики.

**Решение:** Используйте метрики (Prometheus, StatsD) в дополнение к логам:
```go
// Логируем событие
log.Printf("TOOL_EXECUTED: run_id=%s tool=%s", runID, toolName)

// Отправляем метрику
metrics.IncrementCounter("tool_executions_total", map[string]string{"tool": toolName})
metrics.RecordLatency("tool_execution_duration", latency, map[string]string{"tool": toolName})
```

## Мини-упражнения

### Упражнение 1: Реализуйте структурированное логирование

Добавьте структурированное логирование в `labs/lab04-autonomy/main.go`:

```go
func logAgentRun(runID string, userInput string, toolCalls []ToolCall, result string) {
    // Ваш код здесь
    // Логируйте в формате JSON
}
```

**Ожидаемый результат:**
- Логи в формате JSON
- Содержат все необходимые поля: run_id, user_input, tool_calls, result, timestamp

### Упражнение 2: Добавьте трейсинг tool calls

Реализуйте функцию логирования tool call с измерением latency:

```go
func logToolCall(runID string, toolCallID string, toolName string, latency time.Duration) {
    // Ваш код здесь
    // Логируйте в формате: TOOL_CALL: run_id=... tool_id=... tool_name=... latency=...
}
```

**Ожидаемый результат:**
- Каждый tool call логируется с run_id
- Измеряется и логируется latency
- Формат позволяет легко парсить логи

### Упражнение 3: Реализуйте метрики

Добавьте подсчёт метрик (можно использовать простой in-memory счётчик):

```go
type Metrics struct {
    TotalRuns      int64
    TotalTokens    int64
    TotalErrors    int64
    AvgLatency     time.Duration
}

func (m *Metrics) RecordRun(tokens int, latency time.Duration, err error) {
    // Ваш код здесь
    // Обновляйте метрики
}
```

**Ожидаемый результат:**
- Метрики обновляются при каждом agent run
- Можно получить средние значения через методы типа `GetAvgLatency()`

## Критерии сдачи / Чек-лист

✅ **Сдано (готовность к прод):**
- Реализовано структурированное логирование (JSON формат)
- Каждый agent run имеет уникальный `run_id`
- Логируются все этапы: входной запрос → выбор инструментов → выполнение → результат
- Логируются токены и latency для каждого запроса
- Tool calls логируются с `run_id` для кореляции
- Ошибки логируются с контекстом
- Метрики отслеживаются (latency, token usage, error rate)

❌ **Не сдано:**
- Логи в виде простого текста без структуры
- Нет `run_id` для кореляции логов
- Не логируются токены и latency
- Ошибки не логируются
- Нет метрик

## Связь с другими главами

- **Best Practices:** Общие практики логирования изучены в [Главе 16: Best Practices](../16-best-practices/README.md)
- **Agent Loop:** Базовый цикл агента изучен в [Главе 04: Автономность и Циклы](../04-autonomy-and-loops/README.md)
- **Tools:** Выполнение инструментов изучено в [Главе 03: Инструменты и Function Calling](../03-tools-and-function-calling/README.md)
- **Cost Engineering:** Использование токенов для контроля стоимости — [Глава 20: Cost & Latency Engineering](../20-cost-latency-engineering/README.md)

---

**Навигация:** [← Протоколы Инструментов и Tool Servers](../18-tool-protocols-and-servers/README.md) | [Оглавление](../README.md) | [Cost & Latency Engineering →](../20-cost-latency-engineering/README.md)

