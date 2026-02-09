# 19. Observability and Tracing

## Why This Chapter?

Without observability, you work blind. An agent performed an action, but you can't understand:
- Why did it choose this particular tool?
- How long did execution take?
- How many tokens were used?
- Where did the error occur?

Observability gives your agent "vision". Without it, you can't debug problems, optimize performance, or understand how the agent behaves in production.

### Real-World Case Study

**Situation:** Agent in production performed a data deletion operation without confirmation. User complained.

**Problem:** You cannot understand why the agent didn't request confirmation. Logs only show "Agent executed delete_database". No information about which tools were available, what prompt was used, what arguments were passed.

**Solution:** Structured logging with tracing shows the full picture: input request → tool selection → execution → result. Then you can see whether the agent missed a confirmation tool, or whether the prompt was wrong.

## Theory in Simple Terms

### What Is Observability?

Observability is the ability to understand what's happening inside a system by observing its outputs (logs, metrics, traces).

**Three pillars of observability:**
1. **Logs** — what happened (events, errors, actions)
2. **Metrics** — how much and how often (latency, token usage, error rate)
3. **Traces** — how it's connected (full request path through the system)

### How Does Tracing Work in Agents?

Each agent run is a chain of events:
1. User sends request → `run_id` created
2. Agent analyzes request → log input data
3. Agent selects tools → log `tool_calls`
4. Tools execute → log results
5. Agent formulates response → log final response

All these events are linked through `run_id`, so you can reconstruct the full execution flow.

## How It Works (Step by Step)

### Step 1: Agent Run Logging Structure

Create a structure to store agent run information:

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

### Step 2: Generate Unique run_id

At the start of each agent run, generate a unique ID:

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

### Step 3: Logging in Agent Loop

Insert logging into the agent loop (see `labs/lab04-autonomy/main.go`):

```go
func runAgent(ctx context.Context, client *openai.Client, userInput string) (string, error) {
    runID := generateRunID()
    startTime := time.Now()
    
    // Create structure for logging
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
    
    // THE LOOP with logging
    for i := 0; i < 5; i++ {
        req := openai.ChatCompletionRequest{
            Model:    "gpt-4o-mini",
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
        
        // Log token usage
        run.TokensUsed += resp.Usage.TotalTokens
        
        if len(msg.ToolCalls) == 0 {
            run.FinalAnswer = msg.Content
            run.Latency = time.Since(startTime)
            logAgentRun(run)
            return msg.Content, nil
        }
        
        // Log tool calls
        for _, toolCall := range msg.ToolCalls {
            toolCallLog := ToolCall{
                ID:        toolCall.ID,
                Name:      toolCall.Function.Name,
                Arguments: toolCall.Function.Arguments,
                Timestamp: time.Now(),
            }
            run.ToolCalls = append(run.ToolCalls, toolCallLog)
            
            // Execute tool with logging
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

### Step 4: Structured Logging

Implement logging function in JSON format:

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

### Step 5: Tool Execution Logging

In tool execution functions (see `labs/lab02-tools/main.go`) add logging:

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

## Where to Integrate This in Our Code

### Integration Point 1: Agent Loop

In `labs/lab04-autonomy/main.go` add logging to the agent loop:

```go
// At start of main():
runID := generateRunID()
startTime := time.Now()

// In loop before CreateChatCompletion:
log.Printf("AGENT_ITERATION: run_id=%s iteration=%d", runID, i)

// After getting response:
log.Printf("AGENT_RESPONSE: run_id=%s tokens_used=%d", runID, resp.Usage.TotalTokens)

// After executing tools:
log.Printf("TOOL_EXECUTED: run_id=%s tool=%s result=%s", runID, toolCall.Function.Name, result)
```

### Integration Point 2: Tool Execution

In `labs/lab02-tools/main.go` add logging when executing tools:

```go
// In tool execution function:
func executeTool(runID string, toolCall openai.ToolCall) (string, error) {
    log.Printf("TOOL_START: run_id=%s tool=%s", runID, toolCall.Function.Name)
    // ... execution ...
    log.Printf("TOOL_END: run_id=%s tool=%s result=%s", runID, toolCall.Function.Name, result)
    return result, nil
}
```

## Mini Code Example

Complete example with observability based on `labs/lab04-autonomy/main.go`:

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
    userInput := "I'm out of disk space. Fix it."

    // Generate run_id
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
            Model:    "gpt-4o-mini",
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

## Distributed Tracing — OpenTelemetry for Agents

So far we've been logging events as a flat list: `TOOL_START`, `TOOL_END`, `AGENT_ITERATION`. This works for a single agent. But once the agent calls external services (APIs, databases, other agents), flat logs won't help. You can't see what exactly slowed down the chain.

Distributed Tracing solves this problem. Each operation becomes a **span** — a time segment with a start, end, and context. Spans nest inside each other, forming a tree.

### Span-Based Tracing for Agents

For an agent, the span tree looks like this:

```
agent_run (root span)
├── llm_call (iteration 1)
│   └── openai_api_request
├── tool_call: check_disk
│   └── ssh_connection
├── llm_call (iteration 2)
│   └── openai_api_request
├── tool_call: clean_logs
│   └── file_system_operation
└── llm_call (iteration 3 — final answer)
```

Each span contains: operation name, start and end time, attributes (run_id, tool_name, tokens_used), and a reference to its parent span.

### OpenTelemetry: Initialization

OpenTelemetry (OTEL) is the standard for tracing. It supports export to Jaeger, Zipkin, Grafana Tempo, and other backends.

Tracer initialization:

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

### Spans for Agent Iterations and Tool Calls

Wrap each agent step in a span:

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("devops-agent")

func runAgentWithTracing(ctx context.Context, client *openai.Client, userInput string) (string, error) {
    // Root span — the entire agent run
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
        // Span for each loop iteration
        ctx, iterSpan := tracer.Start(ctx, "agent_iteration",
            trace.WithAttributes(attribute.Int("iteration", i)),
        )

        // Span for LLM call
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

        // Span for each tool call
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

### Propagation: Passing Context Between Services

When your agent calls an external HTTP service, the tracing context must travel with it. OTEL uses `traceparent` and `tracestate` HTTP headers:

```go
import (
    "go.opentelemetry.io/otel/propagation"
    "net/http"
)

// Sender: inject context into HTTP headers
func callExternalService(ctx context.Context, url string) (*http.Response, error) {
    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)

    // OTEL injects traceparent into request headers
    otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

    return http.DefaultClient.Do(req)
}

// Receiver: extract context from HTTP headers
func handleRequest(w http.ResponseWriter, r *http.Request) {
    // Extract trace context from incoming request
    ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

    // Spans in this service now belong to the original trace
    ctx, span := tracer.Start(ctx, "external_service.handle")
    defer span.End()

    // ... handle request ...
}
```

After this, you see a single trace spanning all services in Jaeger or Grafana Tempo.

> **OTEL exporters:** For local development, Jaeger is convenient (`docker run -p 16686:16686 jaegertracing/all-in-one`). For production — Grafana Tempo or Zipkin. OTEL supports all of them through a single API.

## Multi-Agent Tracing

In [Chapter 07: Multi-Agent Systems](../07-multi-agent/README.md) we split agents into a Supervisor and Workers. Each Worker is a separate agent loop with its own context. Without tracing, you can't tell which Worker stalled, which one failed, or how the task flowed through the entire system.

### Parent-Child: Supervisor Span → Worker Span

The key idea: the Supervisor's span becomes the **parent** of Worker spans. This gives you the full execution tree:

```
supervisor_run
├── llm_call (Supervisor decides who to delegate to)
├── worker_call: network_expert
│   ├── llm_call
│   └── tool_call: ping_host
├── worker_call: db_expert
│   ├── llm_call
│   └── tool_call: check_db_version
└── llm_call (Supervisor assembles final answer)
```

### Code: Passing Trace Context from Supervisor to Worker

```go
// Worker receives context with trace information from Supervisor
func runWorker(ctx context.Context, client *openai.Client, workerName string, task string, tools []openai.Tool) (string, error) {
    // Create child span — it automatically attaches to the parent from ctx
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
            Model:    "gpt-4o",
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

The Supervisor calls Workers by passing `ctx`:

```go
func runSupervisor(ctx context.Context, client *openai.Client, userInput string) (string, error) {
    ctx, span := tracer.Start(ctx, "supervisor_run",
        trace.WithAttributes(attribute.String("user_input", userInput)),
    )
    defer span.End()

    // Supervisor decides who to delegate to
    // ... (LLM call for routing) ...

    // ctx already contains the Supervisor's trace context.
    // Worker creates a child span automatically.
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

    // ... Supervisor assembles final answer ...
    return finalAnswer, nil
}
```

### Cross-Agent Correlation

Trace ID propagates automatically through `ctx`. All spans within a single trace share the same `trace_id`. In Jaeger you search by this ID and see the full path: from user request through Supervisor to each Worker and back.

If a Worker runs as a separate HTTP service, use propagation from the previous section — pass `traceparent` through headers.

## Component-Level Metrics

Aggregate metrics (`total_tokens`, `avg_latency`) show the overall picture. But for diagnostics you need granularity: which tool is slow? Which LLM call burns the most tokens? Which search query returns irrelevant results?

### Per-Component Metrics

Three categories of component-level metrics:

| Component | Metrics | Purpose |
|-----------|---------|---------|
| **Tool** | Latency, error rate, calls/min | Find slow tools |
| **LLM** | Tokens, latency, cost per call | Optimize token spend |
| **Retrieval** | Relevance score, docs found, latency | Improve RAG quality |

### Code: Collecting Component Metrics

```go
// ComponentMetrics collects metrics for each agent component
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
    RelevanceScores  []float64 // for computing average
}

func NewComponentMetrics() *ComponentMetrics {
    return &ComponentMetrics{
        tools:     make(map[string]*ToolMetrics),
        llm:       &LLMMetrics{},
        retrieval: &RetrievalMetrics{},
    }
}
```

Recording metrics on tool call:

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

Usage in agent loop:

```go
metrics := NewComponentMetrics()

// On LLM call
llmStart := time.Now()
resp, err := client.CreateChatCompletion(ctx, req)
metrics.RecordLLMCall(
    resp.Usage.PromptTokens,
    resp.Usage.CompletionTokens,
    time.Since(llmStart),
    0.000003, // $3 per 1M tokens
)

// On tool call
toolStart := time.Now()
result, err := executeTool(toolCall)
metrics.RecordToolCall(toolCall.Function.Name, time.Since(toolStart), err)
```

### Linking Traces to Evals

Traces show **how** the agent performed the task. Evals show **how well** it did. The trace → eval link gives you the full picture:

```go
type TracedEvalResult struct {
    TraceID       string  `json:"trace_id"`        // OpenTelemetry trace ID
    EvalLevel     string  `json:"eval_level"`       // task, tool, trajectory, topic
    Score         float64 `json:"score"`
    ToolsUsed     []string `json:"tools_used"`
    TotalTokens   int     `json:"total_tokens"`
    TotalLatency  string  `json:"total_latency"`
}

// After agent run completes: record eval result linked to trace
func recordTracedEval(ctx context.Context, result TracedEvalResult) {
    span := trace.SpanFromContext(ctx)
    result.TraceID = span.SpanContext().TraceID().String()

    // Now you can find the full trace in Jaeger by TraceID
    // and understand WHY the eval failed
    log.Printf("EVAL_RESULT: %+v", result)
}
```

When an eval in CI/CD shows a regression (see [Chapter 23: Evals in CI/CD](../23-evals-in-cicd/README.md)), you take the `trace_id` from the result, find the trace in Jaeger, and see the full chain: which tool returned an unexpected result, which LLM call burned abnormally many tokens, where the error occurred.

## Common Errors

### Error 1: No Structured Logging

**Symptom:** Logs in plain text: "Agent executed tool check_disk". Impossible to automatically parse or analyze logs.

**Cause:** Using `fmt.Printf` instead of structured JSON logging.

**Solution:**
```go
// BAD
fmt.Printf("Agent executed tool %s\n", toolName)

// GOOD
logJSON, _ := json.Marshal(map[string]any{
    "run_id": runID,
    "tool": toolName,
    "timestamp": time.Now(),
})
log.Printf("TOOL_EXECUTED: %s", string(logJSON))
```

### Error 2: No run_id for Correlation

**Symptom:** Cannot link logs from different components (agent, tools, external systems) for one request.

**Cause:** Each component logs independently, without a common identifier.

**Solution:**
```go
// BAD
log.Printf("Tool executed: %s", toolName)

// GOOD
runID := generateRunID() // At start of agent run
log.Printf("TOOL_EXECUTED: run_id=%s tool=%s", runID, toolName)
```

### Error 3: Tokens and Latency Not Logged

**Symptom:** Cannot understand execution cost or performance bottlenecks.

**Cause:** Token usage and execution time are not tracked.

**Solution:**
```go
// BAD
resp, _ := client.CreateChatCompletion(ctx, req)
// Tokens not logged

// GOOD
resp, _ := client.CreateChatCompletion(ctx, req)
log.Printf("TOKENS_USED: run_id=%s tokens=%d", runID, resp.Usage.TotalTokens)

startTime := time.Now()
// ... execution ...
log.Printf("LATENCY: run_id=%s latency=%v", runID, time.Since(startTime))
```

### Error 4: Only Successful Cases Logged

**Symptom:** Errors are not logged, cannot understand why agent failed.

**Cause:** Logging only in successful code branches.

**Solution:**
```go
// BAD
result, err := executeTool(toolCall)
if err != nil {
    return "", err // Error not logged
}

// GOOD
result, err := executeTool(toolCall)
if err != nil {
    log.Printf("TOOL_ERROR: run_id=%s tool=%s error=%v", runID, toolCall.Function.Name, err)
    return "", err
}
```

### Error 5: No Metrics (Only Logs)

**Symptom:** Cannot build graphs of latency, error rate, token usage over time.

**Cause:** Only events are logged, but metrics are not aggregated.

**Solution:** Use metrics (Prometheus, StatsD) in addition to logs:
```go
// Log event
log.Printf("TOOL_EXECUTED: run_id=%s tool=%s", runID, toolName)

// Send metric
metrics.IncrementCounter("tool_executions_total", map[string]string{"tool": toolName})
metrics.RecordLatency("tool_execution_duration", latency, map[string]string{"tool": toolName})
```

## Mini-Exercises

### Exercise 1: Implement Structured Logging

Add structured logging to `labs/lab04-autonomy/main.go`:

```go
func logAgentRun(runID string, userInput string, toolCalls []ToolCall, result string) {
    // Your code here
    // Log in JSON format
}
```

**Expected result:**
- Logs in JSON format
- Contains all necessary fields: run_id, user_input, tool_calls, result, timestamp

### Exercise 2: Add Tool Call Tracing

Implement a function to log tool call with latency measurement:

```go
func logToolCall(runID string, toolCallID string, toolName string, latency time.Duration) {
    // Your code here
    // Log in format: TOOL_CALL: run_id=... tool_id=... tool_name=... latency=...
}
```

**Expected result:**
- Each tool call logged with run_id
- Latency measured and logged
- Format allows easy log parsing

### Exercise 3: Implement Metrics

Add metric counting (can use simple in-memory counter):

```go
type Metrics struct {
    TotalRuns      int64
    TotalTokens    int64
    TotalErrors    int64
    AvgLatency     time.Duration
}

func (m *Metrics) RecordRun(tokens int, latency time.Duration, err error) {
    // Your code here
    // Update metrics
}
```

**Expected result:**
- Metrics updated on each agent run
- Can get average values through methods like `GetAvgLatency()`

## Completion Criteria / Checklist

**Completed (production ready):**
- [x] Structured logging implemented (JSON format)
- [x] Each agent run has unique `run_id`
- [x] All stages logged: input request → tool selection → execution → result
- [x] Tokens and latency logged for each request
- [x] Tool calls logged with `run_id` for correlation
- [x] Errors logged with context
- [x] Metrics tracked (latency, token usage, error rate)

**Not completed:**
- [ ] Logs in plain text without structure
- [ ] No `run_id` for log correlation
- [ ] Tokens and latency not logged
- [ ] Errors not logged
- [ ] No metrics

## Connection with Other Chapters

- **Best Practices:** General logging practices studied in [Chapter 16: Best Practices](../16-best-practices/README.md)
- **Agent Loop:** Basic agent loop studied in [Chapter 04: Autonomy and Loops](../04-autonomy-and-loops/README.md)
- **Tools:** Tool execution studied in [Chapter 03: Tools and Function Calling](../03-tools-and-function-calling/README.md)
- **Cost Engineering:** Token usage for cost control — [Chapter 20: Cost & Latency Engineering](../20-cost-latency-engineering/README.md)

