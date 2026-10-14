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
    RunID            string       `json:"run_id"`
    UserInput        string       `json:"user_input"`
    ToolCalls        []ToolCall   `json:"tool_calls"`
    ToolResults      []ToolResult `json:"tool_results"`
    FinalAnswer      string       `json:"final_answer"`
    PromptTokens     int          `json:"prompt_tokens"`
    CompletionTokens int          `json:"completion_tokens"`
    CacheReadTokens  int          `json:"cache_read_tokens"`
    Latency          time.Duration `json:"latency"`
    Timestamp        time.Time    `json:"timestamp"`
}
```

> **Почему не `TotalTokens`.** Истина о расходе токенов — `Usage.PromptTokens` / `Usage.CompletionTokens` из ответа провайдера (канон [главы 13](../13-context-engineering/README.md)). Агрегат `Usage.TotalTokens` как метрика бюджета — ошибка: он скрывает кэш-префиксы и смешивает платные токены с кэш-дешёвыми. Попадание в prompt cache видно по `CacheReadTokens > 0` (названия полей у провайдеров отличаются — `cache_read` / `cached_tokens` / `CacheRead`, — суть одна). Поэтому в `AgentRun` три отдельных поля, а не одна сумма.

```go
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

Вставьте логирование в цикл агента (канон цикла — [глава 04](../04-autonomy-and-loops/README.md); лаба [`lab04-autonomy`](https://github.com/kshvakov/ai-agent-course/tree/main/translations/ru/labs/lab04-autonomy) переведена на тот же нейтральный клиент). Примеры используют нейтральный клиент из главы 04: `client.Complete(ctx, Request{...})` возвращает ответ с `resp.Choices[0].Message` и `resp.Usage` (`PromptTokens` / `CompletionTokens` / `CacheReadTokens` от провайдера):

```go
func runAgent(ctx context.Context, client Client, userInput string) (string, error) {
    runID := generateRunID()
    startTime := time.Now()
    
    // Создаём структуру для логирования
    run := AgentRun{
        RunID:     runID,
        UserInput: userInput,
        ToolCalls: []ToolCall{},
        Timestamp: time.Now(),
    }
    
    messages := []Message{
        {Role: "system", Content: "You are an autonomous DevOps agent."},
        {Role: "user", Content: userInput},
    }
    toolDefs := []ToolDef{/* инструменты хода (гл. 03) */}

    maxIterations := 5 // терминальная граница хода (гл. 04)

    // THE LOOP с логированием
    for i := 0; i < maxIterations; i++ {
        resp, err := client.Complete(ctx, Request{Messages: messages, Tools: toolDefs})
        if err != nil {
            logAgentRunError(runID, err)
            return "", err
        }
        
        msg := resp.Choices[0].Message
        messages = append(messages, msg)
        
        // Логируем использование токенов — по полям провайдера, не по TotalTokens:
        // сумма скрывает кэш-префиксы и смешивает платное с кэш-дешёвым
        run.PromptTokens += resp.Usage.PromptTokens
        run.CompletionTokens += resp.Usage.CompletionTokens
        run.CacheReadTokens += resp.Usage.CacheReadTokens
        
        if len(msg.ToolCalls) == 0 {
            run.FinalAnswer = msg.Content
            run.Latency = time.Since(startTime)
            logAgentRun(run)
            return msg.Content, nil
        }
        
        // Логируем tool calls
        for _, toolCall := range msg.ToolCalls {
            run.ToolCalls = append(run.ToolCalls, ToolCall{
                ID:        toolCall.ID,
                Name:      toolCall.Name,
                Arguments: toolCall.Arguments,
                Timestamp: time.Now(),
            })
            
            // Выполняем инструмент с логированием
            result, err := executeToolWithLogging(runID, toolCall)
            if err != nil {
                run.ToolResults = append(run.ToolResults, ToolResult{
                    ToolCallID: toolCall.ID,
                    Error:      err.Error(),
                    Timestamp:  time.Now(),
                })
                // Ошибка — тоже результат для модели (гл. 03), не пустой контент
                result = "error: " + err.Error()
            } else {
                run.ToolResults = append(run.ToolResults, ToolResult{
                    ToolCallID: toolCall.ID,
                    Result:     result,
                    Timestamp:  time.Now(),
                })
            }
            
            messages = append(messages, Message{
                Role:       "tool",
                Content:    result,
                ToolCallID: toolCall.ID,
            })
        }
    }
    
    run.Latency = time.Since(startTime)
    logAgentRun(run)
    // Исчерпание лимита — терминальная граница (STYLE, гл. 04)
    return run.FinalAnswer, fmt.Errorf("agent exceeded max iterations (%d)", maxIterations)
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

В функции выполнения инструментов добавьте логирование (канон tools — [глава 03](../03-tools-and-function-calling/README.md); ревизованная lab02-tools использует тот же клиент):

```go
func executeToolWithLogging(runID string, toolCall ToolCall) (string, error) {
    startTime := time.Now()
    
    log.Printf("TOOL_CALL_START: run_id=%s tool_id=%s tool_name=%s args=%s",
        runID, toolCall.ID, toolCall.Name, toolCall.Arguments)
    
    var result string
    var err error
    
    switch toolCall.Name {
    case "check_disk":
        result = checkDisk()
    case "clean_logs":
        result = cleanLogs()
    default:
        err = fmt.Errorf("unknown tool: %s", toolCall.Name)
    }
    
    latency := time.Since(startTime)
    
    if err != nil {
        log.Printf("TOOL_CALL_ERROR: run_id=%s tool_id=%s tool_name=%s error=%s latency=%v",
            runID, toolCall.ID, toolCall.Name, err.Error(), latency)
        return "", err
    }
    
    log.Printf("TOOL_CALL_SUCCESS: run_id=%s tool_id=%s tool_name=%s latency=%v",
        runID, toolCall.ID, toolCall.Name, latency)
    
    return result, nil
}
```

## Где это встраивать в нашем коде

### Точка интеграции 1: Agent Loop

В своём рантайме из главы 04 (ревизованная lab04-autonomy использует ту же форму цикла) — добавьте логирование в цикл агента:

```go
// В начале main():
runID := generateRunID()
startTime := time.Now()

// В цикле перед вызовом LLM:
log.Printf("AGENT_ITERATION: run_id=%s iteration=%d", runID, i)

// После получения ответа — токены по полям провайдера:
log.Printf("AGENT_RESPONSE: run_id=%s prompt_tokens=%d completion_tokens=%d cache_read=%d",
    runID, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.CacheReadTokens)

// После выполнения инструментов:
log.Printf("TOOL_EXECUTED: run_id=%s tool=%s result=%s", runID, toolCall.Name, result)
```

### Точка интеграции 2: Tool Execution

В своём рантайме добавьте логирование при выполнении инструментов (ревизованная lab02-tools использует тот же клиент):

```go
// В функции выполнения инструмента:
func executeTool(runID string, toolCall ToolCall) (string, error) {
    log.Printf("TOOL_START: run_id=%s tool=%s", runID, toolCall.Name)
    // ... выполнение ...
    log.Printf("TOOL_END: run_id=%s tool=%s result=%s", runID, toolCall.Name, result)
    return result, nil
}
```

## Мини-пример кода

Компактный пример с observability на базе цикла из главы 04 — те же пять шагов выше, собранные в один листинг (клиент — нейтральный, из [главы 04](../04-autonomy-and-loops/README.md)):

```go
package main

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "log"
    "time"
)

// AgentRun и ToolCall/ToolResult — как в Шаге 1.

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

func checkDisk() string  { return "Disk usage 95%. Largest folder: /var/log" }
func cleanLogs() string  { return "Logs cleaned. Freed 20GB." }

// runAgentWithObservability — цикл из главы 04 с трассировкой по шагам 2–5.
func runAgentWithObservability(ctx context.Context, client Client, userInput string) (string, error) {
    runID := generateRunID()
    startTime := time.Now()

    run := AgentRun{
        RunID:     runID,
        UserInput: userInput,
        ToolCalls: []ToolCall{},
        Timestamp: time.Now(),
    }

    messages := []Message{
        {Role: "system", Content: "You are an autonomous DevOps agent."},
        {Role: "user", Content: userInput},
    }
    toolDefs := []ToolDef{/* инструменты хода (гл. 03) */}

    maxIterations := 5 // терминальная граница хода (гл. 04)

    for i := 0; i < maxIterations; i++ {
        log.Printf("AGENT_ITERATION: run_id=%s iteration=%d", runID, i)

        resp, err := client.Complete(ctx, Request{Messages: messages, Tools: toolDefs})
        if err != nil {
            log.Printf("AGENT_ERROR: run_id=%s error=%v", runID, err)
            return "", err
        }

        msg := resp.Choices[0].Message
        messages = append(messages, msg)

        // Токены — по полям провайдера (см. Шаг 3)
        run.PromptTokens += resp.Usage.PromptTokens
        run.CompletionTokens += resp.Usage.CompletionTokens
        run.CacheReadTokens += resp.Usage.CacheReadTokens

        if len(msg.ToolCalls) == 0 {
            run.FinalAnswer = msg.Content
            run.Latency = time.Since(startTime)
            logAgentRun(run)
            return msg.Content, nil
        }

        for _, toolCall := range msg.ToolCalls {
            log.Printf("TOOL_START: run_id=%s tool_id=%s tool_name=%s", runID, toolCall.ID, toolCall.Name)
            run.ToolCalls = append(run.ToolCalls, ToolCall{
                ID: toolCall.ID, Name: toolCall.Name, Arguments: toolCall.Arguments,
                Timestamp: time.Now(),
            })

            var result string
            switch toolCall.Name {
            case "check_disk":
                result = checkDisk()
            case "clean_logs":
                result = cleanLogs()
            default:
                // Allowlist: неизвестный инструмент — ошибка для модели, не паника
                result = "error: unknown tool " + toolCall.Name
            }
            log.Printf("TOOL_END: run_id=%s tool_id=%s tool_name=%s", runID, toolCall.ID, toolCall.Name)

            run.ToolResults = append(run.ToolResults, ToolResult{
                ToolCallID: toolCall.ID, Result: result, Timestamp: time.Now(),
            })
            messages = append(messages, Message{Role: "tool", Content: result, ToolCallID: toolCall.ID})
        }
    }

    run.Latency = time.Since(startTime)
    logAgentRun(run)
    // Исчерпание лимита — терминальная граница (STYLE, гл. 04)
    return run.FinalAnswer, fmt.Errorf("agent exceeded max iterations (%d)", maxIterations)
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
│   └── llm_api_request
├── tool_call: check_disk
│   └── ssh_connection
├── llm_call (итерация 2)
│   └── llm_api_request
├── tool_call: clean_logs
│   └── file_system_operation
└── llm_call (итерация 3 — финальный ответ)
```

Каждый span содержит: имя операции, время начала и конца, атрибуты (run_id, tool_name, tokens) и ссылку на родительский span.

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

> **Правило: PII не попадает в спан в открытом виде.** Спаны уходят во внешнюю систему трейсинга (Jaeger / Tempo / DataDog), где их видят SRE, аналитики, иногда вендор. `userInput`, `tool.args`, `tool.result`, `final_answer` могут содержать персональные данные, токены, секреты, медицинские/финансовые сведения. Поэтому в `attribute.String(...)` мы кладём **только** одно из:
>
> - **метаданные**: `len(input)`, `tool.name`, кол-во токенов, latency, status;
> - **redacted-форму**: префикс/суффикс + хэш (`"client@a***...x9f3"`);
> - **корреляционный ID**: `prompt_id`, `tool_call_id`, `message_hash`, по которым можно найти полный контент в защищённом хранилище с RBAC и retention-политикой.
>
> Никогда — сырой текст. Если очень нужен дамп для отладки — отдельным sampler-ом и в защищённый storage, а не в общий трейс.

Оборачиваем каждый шаг агента в span:

```go
import (
    "crypto/sha256"
    "fmt"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("devops-agent")

// redact превращает любое потенциально чувствительное значение в безопасную метку:
// длина исходного текста + короткий sha256-префикс. По этой метке можно сопоставить
// span с записью в защищённом хранилище логов, но восстановить исходный текст нельзя.
//
// Тот же принцип работает на уровень ниже: redaction по умолчанию включают и в вывод
// инструментов — модель и логи видят отфильтрованную версию (секреты, ключи, PII),
// полный результат остаётся в защищённом хранилище. Подробности — в
// [Главе 17: Security and Governance](../17-security-and-governance/README.md)
// и [Главе 24: Data and Privacy](../24-data-and-privacy/README.md).
func redact(s string) string {
    if s == "" {
        return ""
    }
    sum := sha256.Sum256([]byte(s))
    return fmt.Sprintf("len=%d sha256=%x", len(s), sum[:6])
}

func runAgentWithTracing(ctx context.Context, client Client, userInput string) (string, error) {
    runID := generateRunID()
    model := "glm-5.3" // имя модели — безопасный атрибут, в отличие от args/result

    // Корневой span — весь запуск агента. Полный текст userInput НЕ кладём в атрибут;
    // оставляем только run_id (по нему ищем входящий запрос в защищённом хранилище)
    // и redacted-метку для дебага.
    ctx, rootSpan := tracer.Start(ctx, "agent_run",
        trace.WithAttributes(
            attribute.String("run_id", runID),
            attribute.String("user_input.redacted", redact(userInput)),
        ),
    )
    defer rootSpan.End()

    messages := []Message{
        {Role: "system", Content: systemPrompt},
        {Role: "user", Content: userInput},
    }
    toolDefs := []ToolDef{/* инструменты хода (гл. 03) */}

    maxIterations := 5 // терминальная граница хода (гл. 04)

    for i := 0; i < maxIterations; i++ {
        // Span для каждой итерации цикла
        ctx, iterSpan := tracer.Start(ctx, "agent_iteration",
            trace.WithAttributes(attribute.Int("iteration", i)),
        )

        // Span для вызова LLM
        llmCtx, llmSpan := tracer.Start(ctx, "llm_call",
            trace.WithAttributes(attribute.String("model", model)),
        )

        resp, err := client.Complete(llmCtx, Request{Messages: messages, Tools: toolDefs})
        if err != nil {
            llmSpan.RecordError(err)
            llmSpan.End()
            iterSpan.End()
            return "", err
        }

        // Токены — prompt/completion отдельно, плюс cache-read:
        // попадание в prompt cache провайдера видно по cache_read > 0
        llmSpan.SetAttributes(
            attribute.Int("tokens.prompt", resp.Usage.PromptTokens),
            attribute.Int("tokens.completion", resp.Usage.CompletionTokens),
            attribute.Int("tokens.cache_read", resp.Usage.CacheReadTokens),
        )
        llmSpan.End()

        msg := resp.Choices[0].Message
        // Assistant-реплика — в историю до tool-вызовов: tool-результаты
        // обязаны идти сразу за своим assistant-сообщением (протокол, гл. 03)
        messages = append(messages, msg)
        if len(msg.ToolCalls) == 0 {
            // Финальный ответ модели тоже может содержать PII — пишем только redacted.
            // Полный ответ — в защищённое хранилище по run_id.
            rootSpan.SetAttributes(
                attribute.String("final_answer.redacted", redact(msg.Content)),
                attribute.Int("final_answer.length", len(msg.Content)),
            )
            iterSpan.End()
            return msg.Content, nil
        }

        // Span для каждого tool call: имя инструмента и tool_call_id безопасны,
        // arguments и result — нет.
        for _, toolCall := range msg.ToolCalls {
            _, toolSpan := tracer.Start(ctx, "tool_call",
                trace.WithAttributes(
                    attribute.String("tool.name", toolCall.Name),
                    attribute.String("tool.call_id", toolCall.ID),
                    attribute.String("tool.args.redacted", redact(toolCall.Arguments)),
                ),
            )

            result, err := executeTool(toolCall)
            if err != nil {
                toolSpan.RecordError(err)
                toolSpan.SetAttributes(attribute.Bool("tool.error", true))
                // Ошибка — тоже результат для модели: цикл продолжается
                result = "error: " + err.Error()
            } else {
                toolSpan.SetAttributes(
                    attribute.String("tool.result.redacted", redact(result)),
                    attribute.Int("tool.result.length", len(result)),
                )
            }
            toolSpan.End()

            messages = append(messages, Message{
                Role: "tool", Content: result, ToolCallID: toolCall.ID,
            })
        }

        iterSpan.End()
    }

    // Исчерпание лимита — терминальная граница (STYLE, гл. 04)
    return "", fmt.Errorf("agent exceeded max iterations (%d)", maxIterations)
}
```

> **Что писать в защищённое хранилище.** Связку «`run_id` → исходный `userInput` / `final_answer`» и «`tool.call_id` → исходные `arguments` / `result`» — в отдельный логгер с шифрованием на стороне сервиса, RBAC по ролям SRE/Privacy/Compliance и retention 7–30 дней. Так у вас остаётся возможность дебажить инцидент, но в общем трейсе нет ни одного байта PII.

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
func runWorker(ctx context.Context, client Client, model string, workerName string, task string, toolDefs []ToolDef) (string, error) {
    // Создаём дочерний span — он автоматически привяжется к родителю из ctx
    ctx, span := tracer.Start(ctx, "worker_call",
        trace.WithAttributes(
            attribute.String("worker.name", workerName),
            attribute.String("worker.task.redacted", redact(task)),
        ),
    )
    defer span.End()

    messages := []Message{
        {Role: "system", Content: fmt.Sprintf("You are %s.", workerName)},
        {Role: "user", Content: task},
    }

    for i := 0; i < 5; i++ {
        _, llmSpan := tracer.Start(ctx, "worker_llm_call",
            trace.WithAttributes(attribute.Int("iteration", i)),
        )

        resp, err := client.Complete(ctx, Request{Messages: messages, Tools: toolDefs})

        if err != nil {
            // Ошибка провайдера до чтения Usage: resp == nil — паниковать нечему
            llmSpan.RecordError(err)
            llmSpan.End()
            return "", err
        }

        llmSpan.SetAttributes(
            attribute.Int("tokens.prompt", resp.Usage.PromptTokens),
            attribute.Int("tokens.completion", resp.Usage.CompletionTokens),
        )
        llmSpan.End()

        msg := resp.Choices[0].Message
        // Assistant-реплика с tool_calls — в историю до их результатов (гл. 03)
        messages = append(messages, msg)
        if len(msg.ToolCalls) == 0 {
            // Результат Worker-а может содержать PII — только redacted-метка
            span.SetAttributes(attribute.String("worker.result.redacted", redact(msg.Content)))
            return msg.Content, nil
        }

        for _, tc := range msg.ToolCalls {
            _, toolSpan := tracer.Start(ctx, "worker_tool_call",
                trace.WithAttributes(
                    attribute.String("tool.name", tc.Name),
                ),
            )
            result, err := executeTool(tc)
            if err != nil {
                // Ошибка логируется, а не глотается: модели уходит error-результат
                toolSpan.RecordError(err)
                result = "error: " + err.Error()
            }
            toolSpan.End()

            messages = append(messages, Message{
                Role: "tool", Content: result, ToolCallID: tc.ID,
            })
        }
    }

    return "", fmt.Errorf("worker %s exceeded max iterations", workerName)
}
```

Supervisor вызывает Worker через обычную передачу `ctx`:

```go
func runSupervisor(ctx context.Context, client Client, userInput string) (string, error) {
    ctx, span := tracer.Start(ctx, "supervisor_run",
        trace.WithAttributes(
            attribute.String("user_input.redacted", redact(userInput)),
        ),
    )
    defer span.End()

    // Supervisor решает, кому делегировать
    // ... (LLM call для маршрутизации) ...

    // ctx уже содержит trace context Supervisor-а.
    // Worker создаст дочерний span автоматически.
    networkResult, err := runWorker(ctx, client, model, "network_expert", "Check connectivity to db-host:5432", networkToolDefs)
    if err != nil {
        span.RecordError(err)
        return "", err
    }

    dbResult, err := runWorker(ctx, client, model, "db_expert", "Get PostgreSQL version", dbToolDefs)
    if err != nil {
        span.RecordError(err)
        return "", err
    }

    span.SetAttributes(
        attribute.String("network_result.redacted", redact(networkResult)),
        attribute.String("db_result.redacted", redact(dbResult)),
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
    TotalCalls             int64
    TotalPromptTokens      int64
    TotalCompletionTokens  int64
    TotalCacheReadTokens   int64
    TotalLatency           time.Duration
    TotalCost              float64
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

func (cm *ComponentMetrics) RecordLLMCall(model string, u Usage, latency time.Duration) {
    cm.mu.Lock()
    defer cm.mu.Unlock()

    cm.llm.TotalCalls++
    cm.llm.TotalPromptTokens += int64(u.PromptTokens)
    cm.llm.TotalCompletionTokens += int64(u.CompletionTokens)
    cm.llm.TotalCacheReadTokens += int64(u.CacheReadTokens)
    cm.llm.TotalLatency += latency
    cm.llm.TotalCost += callCost(model, u) // цены — из конфига, см. ниже
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

messages := []Message{/* история хода (гл. 04) */}
toolDefs := []ToolDef{/* инструменты хода (гл. 03) */}

// При вызове LLM
llmStart := time.Now()
resp, err := client.Complete(ctx, Request{Messages: messages, Tools: toolDefs})
if err == nil {
    metrics.RecordLLMCall(model, resp.Usage, time.Since(llmStart))
}

// При вызове инструмента
toolStart := time.Now()
result, err := executeTool(toolCall)
metrics.RecordToolCall(toolCall.Name, time.Since(toolStart), err)
```

### Стоимость: таблица цен per-модель в конфиге

Цены меняются — держите их в конфиге, не в коде. Одна цена «за токен» не работает: prompt, completion и кэш-чтение тарифицируются по-разному, и у каждой модели свои тарифы.

```go
// ModelPricing — тарифы одной модели, $ за 1M токенов.
type ModelPricing struct {
    PromptPer1M     float64 // обычные prompt-токены
    CompletionPer1M float64 // сгенерированные токены
    CacheReadPer1M  float64 // чтение кэш-префикса — обычно в разы дешевле prompt
}

// Загружается из конфига (YAML/env) при старте — не хардкодится в исходнике.
var pricing = map[string]ModelPricing{ /* из loadPricingFromConfig() */ }

// callCost считает стоимость одного вызова по Usage от провайдера.
// Кэш-чтение тарифицируем отдельной ставкой, а не как полный prompt.
func callCost(model string, u Usage) float64 {
    p := pricing[model]
    cached := float64(u.CacheReadTokens)
    fresh := float64(u.PromptTokens - u.CacheReadTokens)
    return fresh/1e6*p.PromptPer1M +
        cached/1e6*p.CacheReadPer1M +
        float64(u.CompletionTokens)/1e6*p.CompletionPer1M
}
```

### Связь трейсов с evals

Трейсы показывают **как** агент выполнил задачу. Evals показывают **насколько хорошо** он это сделал. Связка trace → eval даёт полную картину:

```go
type TracedEvalResult struct {
    TraceID          string   `json:"trace_id"`          // ID трейса из OpenTelemetry
    EvalLevel        string   `json:"eval_level"`        // task, tool, trajectory, topic
    Score            float64  `json:"score"`
    ToolsUsed        []string `json:"tools_used"`
    PromptTokens     int      `json:"prompt_tokens"`
    CompletionTokens int      `json:"completion_tokens"`
    TotalLatency     string   `json:"total_latency"`
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

## Метрики и дашборды

Логи и трейсы отвечают на «что случилось в этом run». Метрики отвечают на «как система ведёт себя во времени». Стандартный стек: агент отдаёт Prometheus-метрики на `/metrics`, Grafana-дашборды рисуют их в динамике.

### Базовые метрики агента

| Метрика | Что показывает |
|---------|----------------|
| `llm_tokens_total{kind=prompt\|completion\|cache_read}` | расход токенов по видам; cache_read > 0 — попадание в prompt cache |
| `agent_iterations_total` | сколько ходов делает агент на задачу (растёт — агент зацикливается) |
| `tool_calls_total{tool}` / `tool_errors_total{tool}` | нагрузка и надёжность инструментов |
| `policy_denied_total{policy=fs\|authority}` | отказы политик безопасности (гл. 17) |
| `failovers_total` / `retries_total` | переключения на fallback-модель и ретраи клиента |
| `agent_run_duration_seconds` | latency ходов |

### Гард-тест «дашборд ↔ реестр метрик»

Дашборд тихо протухает: метрику переименовали — график стал пустым, и никто не заметил. Лечится тестом: держите единый реестр метрик (структура или список имён) и тест, который парсит JSON-модель дашборда и проверяет, что **каждая метрика реестра присутствует хотя бы на одной панели**. Переименовали метрику и забыли дашборд — падает CI, а не прод-мониторинг.

### Живые evals как часть наблюдаемости

Метрики показывают «сколько», но не «насколько хорошо». Поэтому рядом с Grafana-панелями живут живые evals: периодические прогоны агента на фиксированном наборе задач в прод-окружении, чьи скоры тоже становятся метриками (`eval_score{suite}`). Деградация качества видна на дашборде рядом с ростом latency — иногда это один и тот же инцидент. Как строить evals — [Глава 08: Evals and Reliability](../08-evals-and-reliability/README.md) и [Глава 23: Evals в CI/CD](../23-evals-in-cicd/README.md).

> **Как это устроено в реальном SDK.** `RunResult` несёт всё, что нужно для метрик одного хода: usage (prompt/completion/cache), итерации, список вызванных tools, failover-ы и итоговую модель. События цикла (дельта стрима, финальный ответ, старт/прогресс/конец инструмента) — естественные точки трассировки: на них подписываются и логгер, и сборщик метрик. Транзиентные ошибки ретраит сам клиент — retry-политика настраивается на модели, а не в теле цикла.

## Типовые ошибки

Короткая сводка типовых провалов наблюдаемости:

| # | Ошибка | Симптом | Решение |
|---|--------|---------|---------|
| 1 | Нет структурированного логирования | Логи — простой текст, не парсится автоматически | JSON-логи с полями (разбор ниже) |
| 2 | Нет `run_id` для корреляции | Логи компонентов не связываются в одну картину | Уникальный `run_id` на каждый run, пробрасывать всюду |
| 3 | Токены логируются одной суммой | Бюджет «плывёт», кэш не видно | `PromptTokens`/`CompletionTokens` + cache-read (разбор ниже) |
| 4 | Логируются только успехи | Ошибки теряются, причины падений неизвестны | Логировать и error-ветки, с контекстом |
| 5 | Только логи, нет метрик | Нет графиков latency/error rate/tokens во времени | Prometheus `/metrics` + Grafana (раздел выше) |

### Разбор 1: нет структурированного логирования

**Симптом:** логи в виде простого текста: "Agent executed tool check_disk". Невозможно автоматически парсить или анализировать.

**Причина:** `fmt.Printf` вместо структурированного JSON-логирования.

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

### Разбор 2: бюджет по `TotalTokens`

**Симптом:** счёт за API расходится с вашей метрикой токенов на десятки процентов; эффект prompt cache не виден вообще.

**Причина:** в качестве метрики бюджета используется `Usage.TotalTokens` — агрегат, который скрывает кэш-префиксы и смешивает платные токены с кэш-дешёвыми (канон [главы 13](../13-context-engineering/README.md)).

**Решение:** логируйте и суммируйте компоненты отдельно:

```go
// ПЛОХО
log.Printf("TOKENS_USED: run_id=%s tokens=%d", runID, resp.Usage.TotalTokens)

// ХОРОШО
log.Printf("TOKENS_USED: run_id=%s prompt=%d completion=%d cache_read=%d",
    runID, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.CacheReadTokens)
```

`cache_read > 0` — попадание в prompt cache провайдера; доля `cache_read / prompt` — прямая метрика того, насколько работает ваш неизменяемый system-префикс (гл. 20).

## Мини-упражнения

### Упражнение 1: Реализуйте структурированное логирование

Добавьте структурированное логирование в свой рантайм из главы 04 (ревизованная lab04-autonomy использует ту же форму цикла; канонический материал — в этой главе):

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
    TotalRuns          int64
    TotalPromptTokens  int64
    TotalCompletion    int64
    TotalErrors        int64
    AvgLatency         time.Duration
}

func (m *Metrics) RecordRun(usage Usage, latency time.Duration, err error) {
    // Ваш код здесь
    // Обновляйте метрики: usage.PromptTokens, usage.CompletionTokens — от провайдера
}
```

**Ожидаемый результат:**
- Метрики обновляются при каждом agent run
- Токены считаются по полям провайдера (prompt/completion), а не по TotalTokens
- Можно получить средние значения через методы типа `GetAvgLatency()`

## Критерии сдачи / Чек-лист

**Сдано (готовность к прод):**
- [x] Реализовано структурированное логирование (JSON формат)
- [x] Каждый agent run имеет уникальный `run_id`
- [x] Логируются все этапы: входной запрос → выбор инструментов → выполнение → результат
- [x] Токены логируются по полям провайдера: prompt / completion / cache_read
- [x] Tool calls логируются с `run_id` для кореляции
- [x] Ошибки логируются с контекстом, без секретов (redaction по умолчанию)
- [x] Метрики отслеживаются (latency, токены по видам, итерации, tool calls, error rate)

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


