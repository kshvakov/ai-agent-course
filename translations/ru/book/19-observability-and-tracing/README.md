# 19. Observability и Tracing

## Зачем это нужно?

Без observability вы работаете вслепую. Агент выполнил действие, но вы не можете понять:
- Почему он выбрал именно этот инструмент?
- Сколько времени заняло выполнение?
- Сколько токенов было использовано?
- Где произошла ошибка?

Observability — это "зрение" для вашего агента. Без него сложно отлаживать проблемы, оптимизировать производительность и понимать поведение агента в проде.

### Реальный кейс

**Ситуация:** Агент в проде выполнил операцию удаления данных без подтверждения. Пользователь пожаловался.

**Проблема:** Вы не можете понять, почему агент не запросил подтверждение. В логах только "Agent executed delete_database". Нет информации о том, какие инструменты были доступны, какой промпт использовался, какие аргументы были переданы.

**Решение:** Структурированное логирование с трейсингом помогает увидеть полную картину: входной запрос → выбор инструментов → выполнение → результат. Теперь вы можете понять, что агент не увидел инструмент с подтверждением или промпт был неправильным.

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

Все эти события связаны через `run_id`, так что вы можете восстановить полную картину выполнения.

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

## Distributed Tracing — OpenTelemetry для агентов

До сих пор мы логировали события плоским списком: `TOOL_START`, `TOOL_END`, `AGENT_ITERATION`. Это работает для одного агента. Но как только агент вызывает внешние сервисы (API, БД, другие агенты), плоские логи не помогут. Вы не увидите, что именно затормозило цепочку.

Distributed Tracing (распределённый трейсинг) решает эту проблему. Каждая операция становится **span** — отрезком времени с началом, концом и контекстом. Span-ы вкладываются друг в друга, образуя дерево.

### Span-based трейсинг для агентов

Для агента дерево span-ов выглядит так:

```
agent_run (корневой span)
├── llm_call (итерация 1)
│   └── openai_api_request
├── tool_call: check_disk
│   └── ssh_connection
├── llm_call (итерация 2)
│   └── openai_api_request
├── tool_call: clean_logs
│   └── file_system_operation
└── llm_call (итерация 3 — финальный ответ)
```

Каждый span содержит: имя операции, время начала и конца, атрибуты (run_id, tool_name, tokens_used) и ссылку на родительский span.

### OpenTelemetry: инициализация

OpenTelemetry (OTEL) — стандарт для трейсинга. Поддерживает экспорт в Jaeger, Zipkin, Grafana Tempo и другие бэкенды.

Инициализация трейсера:

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func initTracer(ctx context.Context) (*sdktrace.TracerProvider, error) {
    exporter, err := otlptracehttp.New(ctx,
        otlptracehttp.WithEndpoint("localhost:4318"),
        otlptracehttp.WithInsecure(),
    )
    if err != nil {
        return nil, fmt.Errorf("create exporter: %w", err)
    }

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceNameKey.String("devops-agent"),
        )),
    )

    otel.SetTracerProvider(tp)
    return tp, nil
}
```

### Span-ы для итераций агента и tool calls

Оборачиваем каждый шаг агента в span:

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("devops-agent")

func runAgentWithTracing(ctx context.Context, client *openai.Client, userInput string) (string, error) {
    // Корневой span — весь запуск агента
    ctx, rootSpan := tracer.Start(ctx, "agent_run",
        trace.WithAttributes(
            attribute.String("user_input", userInput),
            attribute.String("run_id", generateRunID()),
        ),
    )
    defer rootSpan.End()

    messages := []openai.ChatCompletionMessage{
        {Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
        {Role: openai.ChatMessageRoleUser, Content: userInput},
    }

    for i := 0; i < maxIterations; i++ {
        // Span для каждой итерации цикла
        ctx, iterSpan := tracer.Start(ctx, "agent_iteration",
            trace.WithAttributes(attribute.Int("iteration", i)),
        )

        // Span для вызова LLM
        llmCtx, llmSpan := tracer.Start(ctx, "llm_call",
            trace.WithAttributes(attribute.String("model", model)),
        )

        resp, err := client.CreateChatCompletion(llmCtx, req)
        if err != nil {
            llmSpan.RecordError(err)
            llmSpan.End()
            iterSpan.End()
            return "", err
        }

        llmSpan.SetAttributes(
            attribute.Int("tokens.prompt", resp.Usage.PromptTokens),
            attribute.Int("tokens.completion", resp.Usage.CompletionTokens),
            attribute.Int("tokens.total", resp.Usage.TotalTokens),
        )
        llmSpan.End()

        msg := resp.Choices[0].Message
        if len(msg.ToolCalls) == 0 {
            rootSpan.SetAttributes(attribute.String("final_answer", msg.Content))
            iterSpan.End()
            return msg.Content, nil
        }

        // Span для каждого tool call
        for _, toolCall := range msg.ToolCalls {
            _, toolSpan := tracer.Start(ctx, "tool_call",
                trace.WithAttributes(
                    attribute.String("tool.name", toolCall.Function.Name),
                    attribute.String("tool.args", toolCall.Function.Arguments),
                ),
            )

            result, err := executeTool(toolCall)
            if err != nil {
                toolSpan.RecordError(err)
                toolSpan.SetAttributes(attribute.Bool("tool.error", true))
            } else {
                toolSpan.SetAttributes(attribute.String("tool.result", result))
            }
            toolSpan.End()
        }

        iterSpan.End()
    }

    return "", nil
}
```

### Propagation: передача контекста между сервисами

Если агент вызывает внешний HTTP-сервис, контекст трейсинга нужно передать дальше. OTEL использует HTTP-заголовки `traceparent` и `tracestate`:

```go
import (
    "go.opentelemetry.io/otel/propagation"
    "net/http"
)

// Отправитель: внедряем контекст в HTTP-заголовки
func callExternalService(ctx context.Context, url string) (*http.Response, error) {
    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)

    // OTEL внедряет traceparent в заголовки запроса
    otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

    return http.DefaultClient.Do(req)
}

// Получатель: извлекаем контекст из HTTP-заголовков
func handleRequest(w http.ResponseWriter, r *http.Request) {
    // Извлекаем trace context из входящего запроса
    ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

    // Теперь span-ы в этом сервисе будут частью исходного трейса
    ctx, span := tracer.Start(ctx, "external_service.handle")
    defer span.End()

    // ... обработка запроса ...
}
```

После этого в Jaeger или Grafana Tempo вы увидите единый трейс, проходящий через все сервисы.

> **Экспортеры OTEL:** Для локальной разработки удобен Jaeger (`docker run -p 16686:16686 jaegertracing/all-in-one`). Для прода — Grafana Tempo или Zipkin. OTEL поддерживает их все через единый API.

## Multi-Agent Tracing — трейсинг нескольких агентов

В [Главе 07: Multi-Agent Systems](../07-multi-agent/README.md) мы разделили агентов на Supervisor и Worker-ов. Каждый Worker — отдельный agent loop со своим контекстом. Без трейсинга невозможно понять, какой Worker затормозил, какой ошибся, и как задача прошла через всю систему.

### Parent-child: Supervisor span → Worker span

Ключевая идея: span Supervisor-а становится **родителем** для span-ов Worker-ов. Так мы видим полное дерево выполнения:

```
supervisor_run
├── llm_call (Supervisor решает, кому делегировать)
├── worker_call: network_expert
│   ├── llm_call
│   └── tool_call: ping_host
├── worker_call: db_expert
│   ├── llm_call
│   └── tool_call: check_db_version
└── llm_call (Supervisor собирает финальный ответ)
```

### Код: передача trace context от Supervisor к Worker

```go
// Worker принимает context с trace-информацией от Supervisor
func runWorker(ctx context.Context, client *openai.Client, workerName string, task string, tools []openai.Tool) (string, error) {
    // Создаём дочерний span — он автоматически привяжется к родителю из ctx
    ctx, span := tracer.Start(ctx, "worker_call",
        trace.WithAttributes(
            attribute.String("worker.name", workerName),
            attribute.String("worker.task", task),
        ),
    )
    defer span.End()

    messages := []openai.ChatCompletionMessage{
        {Role: openai.ChatMessageRoleSystem, Content: fmt.Sprintf("You are %s.", workerName)},
        {Role: openai.ChatMessageRoleUser, Content: task},
    }

    for i := 0; i < 5; i++ {
        _, llmSpan := tracer.Start(ctx, "worker_llm_call",
            trace.WithAttributes(attribute.Int("iteration", i)),
        )

        resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
            Model:    openai.GPT4,
            Messages: messages,
            Tools:    tools,
        })

        llmSpan.SetAttributes(
            attribute.Int("tokens.total", resp.Usage.TotalTokens),
        )
        llmSpan.End()

        if err != nil {
            span.RecordError(err)
            return "", err
        }

        msg := resp.Choices[0].Message
        if len(msg.ToolCalls) == 0 {
            span.SetAttributes(attribute.String("worker.result", msg.Content))
            return msg.Content, nil
        }

        for _, tc := range msg.ToolCalls {
            _, toolSpan := tracer.Start(ctx, "worker_tool_call",
                trace.WithAttributes(
                    attribute.String("tool.name", tc.Function.Name),
                ),
            )
            result, _ := executeTool(tc)
            toolSpan.End()

            messages = append(messages, openai.ChatCompletionMessage{
                Role: openai.ChatMessageRoleTool, Content: result, ToolCallID: tc.ID,
            })
        }

        messages = append(messages, msg)
    }

    return "", fmt.Errorf("worker %s exceeded max iterations", workerName)
}
```

Supervisor вызывает Worker через обычную передачу `ctx`:

```go
func runSupervisor(ctx context.Context, client *openai.Client, userInput string) (string, error) {
    ctx, span := tracer.Start(ctx, "supervisor_run",
        trace.WithAttributes(attribute.String("user_input", userInput)),
    )
    defer span.End()

    // Supervisor решает, кому делегировать
    // ... (LLM call для маршрутизации) ...

    // ctx уже содержит trace context Supervisor-а.
    // Worker создаст дочерний span автоматически.
    networkResult, err := runWorker(ctx, client, "network_expert", "Check connectivity to db-host:5432", networkTools)
    if err != nil {
        span.RecordError(err)
        return "", err
    }

    dbResult, err := runWorker(ctx, client, "db_expert", "Get PostgreSQL version", dbTools)
    if err != nil {
        span.RecordError(err)
        return "", err
    }

    span.SetAttributes(
        attribute.String("network_result", networkResult),
        attribute.String("db_result", dbResult),
    )

    // ... Supervisor собирает финальный ответ ...
    return finalAnswer, nil
}
```

### Cross-agent correlation

Trace ID автоматически прокидывается через `ctx`. Все span-ы внутри одного трейса имеют общий `trace_id`. В Jaeger вы ищете по этому ID и видите полный путь: от запроса пользователя через Supervisor к каждому Worker и обратно.

Если Worker работает как отдельный HTTP-сервис, используйте propagation из предыдущего раздела — передавайте `traceparent` через заголовки.

## Component-level Metrics — метрики по компонентам

Общие метрики (`total_tokens`, `avg_latency`) показывают картину в целом. Но для диагностики нужна гранулярность: какой именно инструмент тормозит? Какой LLM-вызов тратит больше всего токенов? Какой поисковый запрос возвращает нерелевантные результаты?

### Метрики по каждому компоненту

Три категории покомпонентных метрик:

| Компонент | Метрики | Зачем |
|-----------|---------|-------|
| **Tool** | Latency, error rate, вызовов/мин | Найти медленные инструменты |
| **LLM** | Токены, latency, стоимость за вызов | Оптимизировать расход токенов |
| **Retrieval** | Relevance score, документов найдено, latency | Улучшить качество RAG |

### Код: сбор метрик по компонентам

```go
// ComponentMetrics собирает метрики для каждого компонента агента
type ComponentMetrics struct {
    mu      sync.Mutex
    tools   map[string]*ToolMetrics
    llm     *LLMMetrics
    retrieval *RetrievalMetrics
}

type ToolMetrics struct {
    Name         string
    TotalCalls   int64
    TotalErrors  int64
    TotalLatency time.Duration
}

type LLMMetrics struct {
    TotalCalls       int64
    TotalTokens      int64
    TotalPromptTokens int64
    TotalCompletionTokens int64
    TotalLatency     time.Duration
    TotalCost        float64
}

type RetrievalMetrics struct {
    TotalQueries     int64
    TotalDocuments   int64
    TotalLatency     time.Duration
    RelevanceScores  []float64 // для расчёта среднего
}

func NewComponentMetrics() *ComponentMetrics {
    return &ComponentMetrics{
        tools:     make(map[string]*ToolMetrics),
        llm:       &LLMMetrics{},
        retrieval: &RetrievalMetrics{},
    }
}
```

Запись метрик при вызове инструмента:

```go
func (cm *ComponentMetrics) RecordToolCall(name string, latency time.Duration, err error) {
    cm.mu.Lock()
    defer cm.mu.Unlock()

    m, ok := cm.tools[name]
    if !ok {
        m = &ToolMetrics{Name: name}
        cm.tools[name] = m
    }

    m.TotalCalls++
    m.TotalLatency += latency
    if err != nil {
        m.TotalErrors++
    }
}

func (cm *ComponentMetrics) RecordLLMCall(promptTokens, completionTokens int, latency time.Duration, costPerToken float64) {
    cm.mu.Lock()
    defer cm.mu.Unlock()

    cm.llm.TotalCalls++
    cm.llm.TotalPromptTokens += int64(promptTokens)
    cm.llm.TotalCompletionTokens += int64(completionTokens)
    cm.llm.TotalTokens += int64(promptTokens + completionTokens)
    cm.llm.TotalLatency += latency
    cm.llm.TotalCost += float64(promptTokens+completionTokens) * costPerToken
}

func (cm *ComponentMetrics) RecordRetrieval(docsFound int, relevanceScore float64, latency time.Duration) {
    cm.mu.Lock()
    defer cm.mu.Unlock()

    cm.retrieval.TotalQueries++
    cm.retrieval.TotalDocuments += int64(docsFound)
    cm.retrieval.TotalLatency += latency
    cm.retrieval.RelevanceScores = append(cm.retrieval.RelevanceScores, relevanceScore)
}
```

Использование в agent loop:

```go
metrics := NewComponentMetrics()

// При вызове LLM
llmStart := time.Now()
resp, err := client.CreateChatCompletion(ctx, req)
metrics.RecordLLMCall(
    resp.Usage.PromptTokens,
    resp.Usage.CompletionTokens,
    time.Since(llmStart),
    0.000003, // $3 per 1M tokens
)

// При вызове инструмента
toolStart := time.Now()
result, err := executeTool(toolCall)
metrics.RecordToolCall(toolCall.Function.Name, time.Since(toolStart), err)
```

### Связь трейсов с evals

Трейсы показывают **как** агент выполнил задачу. Evals показывают **насколько хорошо** он это сделал. Связка trace → eval даёт полную картину:

```go
type TracedEvalResult struct {
    TraceID       string  `json:"trace_id"`        // ID трейса из OpenTelemetry
    EvalLevel     string  `json:"eval_level"`       // task, tool, trajectory, topic
    Score         float64 `json:"score"`
    ToolsUsed     []string `json:"tools_used"`
    TotalTokens   int     `json:"total_tokens"`
    TotalLatency  string  `json:"total_latency"`
}

// После завершения agent run: записываем результат eval с привязкой к трейсу
func recordTracedEval(ctx context.Context, result TracedEvalResult) {
    span := trace.SpanFromContext(ctx)
    result.TraceID = span.SpanContext().TraceID().String()

    // Теперь по TraceID можно найти полный трейс в Jaeger
    // и понять, ПОЧЕМУ eval провалился
    log.Printf("EVAL_RESULT: %+v", result)
}
```

Когда eval в CI/CD показывает регрессию (см. [Главу 23: Evals в CI/CD](../23-evals-in-cicd/README.md)), вы берёте `trace_id` из результата, находите трейс в Jaeger и видите всю цепочку: какой инструмент вернул неожиданный результат, какой LLM-вызов потратил аномально много токенов, где произошла ошибка.

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
- Формат легко парсить

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

**Сдано (готовность к прод):**
- [x] Реализовано структурированное логирование (JSON формат)
- [x] Каждый agent run имеет уникальный `run_id`
- [x] Логируются все этапы: входной запрос → выбор инструментов → выполнение → результат
- [x] Логируются токены и latency для каждого запроса
- [x] Tool calls логируются с `run_id` для кореляции
- [x] Ошибки логируются с контекстом
- [x] Метрики отслеживаются (latency, token usage, error rate)

**Не сдано:**
- [ ] Логи в виде простого текста без структуры
- [ ] Нет `run_id` для кореляции логов
- [ ] Не логируются токены и latency
- [ ] Ошибки не логируются
- [ ] Нет метрик

## Связь с другими главами

- **Best Practices:** Общие практики логирования изучены в [Главе 16: Best Practices](../16-best-practices/README.md)
- **Agent Loop:** Базовый цикл агента изучен в [Главе 04: Автономность и Циклы](../04-autonomy-and-loops/README.md)
- **Tools:** Выполнение инструментов изучено в [Главе 03: Инструменты и Function Calling](../03-tools-and-function-calling/README.md)
- **Cost Engineering:** Использование токенов для контроля стоимости — [Глава 20: Cost & Latency Engineering](../20-cost-latency-engineering/README.md)


