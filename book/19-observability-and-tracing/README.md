# 19. Observability and Tracing

## Why does this matter?

Without observability you are working blind. The agent performed an action, but you can't tell:
- Why did it pick that particular tool?
- How long did the execution take?
- How many tokens were used?
- Where did the error occur?

Observability is the "eyes" of your agent. Without it, it's hard to debug problems, optimize performance, and understand the agent's behavior in production.

### A real-world case

**Situation:** An agent in production performed a data deletion operation without confirmation. A user complained.

**Problem:** You can't figure out why the agent didn't ask for confirmation. The logs contain only "Agent executed delete_database". There is no information about which tools were available, what prompt was used, or what arguments were passed.

**Solution:** Structured logging with tracing helps you see the full picture: input request → tool selection → execution → result. Now you can see whether the agent didn't see the confirmation tool, or the prompt was wrong.

## The theory in plain words

### What is observability?

Observability is the ability to understand what is happening inside a system by observing its outputs (logs, metrics, traces).

**The three pillars of observability:**
1. **Logs** — what happened (events, errors, actions)
2. **Metrics** — how much and how often (latency, token usage, error rate)
3. **Traces** — how it all fits together (the full path of a request through the system)

### How does tracing work in agents?

Every agent run is a chain of events:
1. The user sends a request → a `run_id` is created
2. The agent analyzes the request → we log the input
3. The agent selects tools → we log the `tool_calls`
4. The tools execute → we log the results
5. The agent produces an answer → we log the final answer

All these events are linked through `run_id`, so you can reconstruct the full picture of the run.

## How it works (step by step)

### Step 1: A structure for logging an agent run

Create a structure to store information about an agent run:

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

> **Why not `TotalTokens`.** The ground truth on token spend is `Usage.PromptTokens` / `Usage.CompletionTokens` from the provider's response (the canon is [chapter 13](../13-context-engineering/README.md)). The aggregate `Usage.TotalTokens` is the wrong budget metric: it hides cache prefixes and mixes paid tokens with cache-cheap ones. A prompt cache hit shows up as `CacheReadTokens > 0` (providers name the field differently — `cache_read` / `cached_tokens` / `CacheRead` — but the meaning is the same). That's why `AgentRun` has three separate fields instead of one total.

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

### Step 2: Generating a unique run_id

At the start of every agent run, generate a unique ID:

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

### Step 3: Logging in the agent loop

Insert logging into the agent loop (the canon for the loop is [chapter 04](../04-autonomy-and-loops/README.md); the [`lab04-autonomy`](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab04-autonomy) lab is not yet revised and will be updated in a separate pass). The examples use the neutral client from chapter 04: `client.Complete(ctx, Request{...})` returns a response with `resp.Choices[0].Message` and `resp.Usage` (`PromptTokens` / `CompletionTokens` / `CacheReadTokens` from the provider):

```go
func runAgent(ctx context.Context, client Client, userInput string) (string, error) {
    runID := generateRunID()
    startTime := time.Now()
    
    // Create the structure for logging
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
    toolDefs := []ToolDef{/* run tools (ch. 03) */}

    maxIterations := 5 // terminal run boundary (ch. 04)

    // THE LOOP with logging
    for i := 0; i < maxIterations; i++ {
        resp, err := client.Complete(ctx, Request{Messages: messages, Tools: toolDefs})
        if err != nil {
            logAgentRunError(runID, err)
            return "", err
        }
        
        msg := resp.Choices[0].Message
        messages = append(messages, msg)
        
        // Log token usage — by the provider's fields, not TotalTokens:
        // the total hides cache prefixes and mixes paid tokens with cache-cheap ones
        run.PromptTokens += resp.Usage.PromptTokens
        run.CompletionTokens += resp.Usage.CompletionTokens
        run.CacheReadTokens += resp.Usage.CacheReadTokens
        
        if len(msg.ToolCalls) == 0 {
            run.FinalAnswer = msg.Content
            run.Latency = time.Since(startTime)
            logAgentRun(run)
            return msg.Content, nil
        }
        
        // Log the tool calls
        for _, toolCall := range msg.ToolCalls {
            run.ToolCalls = append(run.ToolCalls, ToolCall{
                ID:        toolCall.ID,
                Name:      toolCall.Name,
                Arguments: toolCall.Arguments,
                Timestamp: time.Now(),
            })
            
            // Execute the tool with logging
            result, err := executeToolWithLogging(runID, toolCall)
            if err != nil {
                run.ToolResults = append(run.ToolResults, ToolResult{
                    ToolCallID: toolCall.ID,
                    Error:      err.Error(),
                    Timestamp:  time.Now(),
                })
                // An error is a result too for the model (ch. 03), not empty content
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
    // Exhausting the iteration limit is a terminal run boundary (STYLE, ch. 04)
    return run.FinalAnswer, fmt.Errorf("agent exceeded max iterations (%d)", maxIterations)
}
```

### Step 4: Structured logging

Implement the logging function in JSON format:

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

### Step 5: Logging tool execution

In the tool execution function, add logging (the canon for tools is [chapter 03](../03-tools-and-function-calling/README.md); the lab02-tools lab is not yet revised):

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

## Where to plug this into our code

### Integration point 1: Agent loop

In your runtime from chapter 04 (the lab04-autonomy lab is not yet revised) — add logging to the agent loop:

```go
// At the start of main():
runID := generateRunID()
startTime := time.Now()

// In the loop, before the LLM call:
log.Printf("AGENT_ITERATION: run_id=%s iteration=%d", runID, i)

// After receiving the response — tokens by the provider's fields:
log.Printf("AGENT_RESPONSE: run_id=%s prompt_tokens=%d completion_tokens=%d cache_read=%d",
    runID, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.CacheReadTokens)

// After the tools execute:
log.Printf("TOOL_EXECUTED: run_id=%s tool=%s result=%s", runID, toolCall.Name, result)
```

### Integration point 2: Tool execution

In your runtime, add logging when tools execute (the lab02-tools lab is not yet revised):

```go
// In the tool execution function:
func executeTool(runID string, toolCall ToolCall) (string, error) {
    log.Printf("TOOL_START: run_id=%s tool=%s", runID, toolCall.Name)
    // ... execution ...
    log.Printf("TOOL_END: run_id=%s tool=%s result=%s", runID, toolCall.Name, result)
    return result, nil
}
```

## Mini code example

A compact example with observability built on the chapter 04 loop — the same five steps above, assembled into one listing (the client is the neutral one from [chapter 04](../04-autonomy-and-loops/README.md)):
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

// AgentRun and ToolCall/ToolResult — same as in Step 1.

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

// runAgentWithObservability — the loop from chapter 04 with tracing per steps 2–5.
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
    toolDefs := []ToolDef{/* run tools (ch. 03) */}

    maxIterations := 5 // terminal run boundary (ch. 04)

    for i := 0; i < maxIterations; i++ {
        log.Printf("AGENT_ITERATION: run_id=%s iteration=%d", runID, i)

        resp, err := client.Complete(ctx, Request{Messages: messages, Tools: toolDefs})
        if err != nil {
            log.Printf("AGENT_ERROR: run_id=%s error=%v", runID, err)
            return "", err
        }

        msg := resp.Choices[0].Message
        messages = append(messages, msg)

        // Tokens — from the provider's fields (see Step 3)
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
                // Allowlist: an unknown tool is an error for the model, not a panic
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
    // Exhausting the limit — a terminal boundary (STYLE, ch. 04)
    return run.FinalAnswer, fmt.Errorf("agent exceeded max iterations (%d)", maxIterations)
}
```

## Distributed Tracing — OpenTelemetry for Agents

So far we have been logging events as a flat list: `TOOL_START`, `TOOL_END`, `AGENT_ITERATION`. This works for a single agent. But as soon as the agent calls external services (APIs, databases, other agents), flat logs stop helping: you won't see what exactly slowed the chain down.

Distributed tracing solves this problem. Every operation becomes a **span** — an interval of time with a start, an end, and a context. Spans nest inside one another, forming a tree.

### Span-based tracing for agents

For an agent, the span tree looks like this:

```
agent_run (root span)
├── llm_call (iteration 1)
│   └── llm_api_request
├── tool_call: check_disk
│   └── ssh_connection
├── llm_call (iteration 2)
│   └── llm_api_request
├── tool_call: clean_logs
│   └── file_system_operation
└── llm_call (iteration 3 — final answer)
```

Each span contains: the operation name, start and end times, attributes (run_id, tool_name, tokens), and a reference to the parent span.

### OpenTelemetry: initialization

OpenTelemetry (OTEL) is the standard for tracing. It supports exporting to Jaeger, Zipkin, Grafana Tempo, and other backends.

Initializing the tracer:

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

### Spans for agent iterations and tool calls

> **Rule: PII never goes into a span in plain form.** Spans leave for an external tracing system (Jaeger / Tempo / DataDog), where SREs, analysts, and sometimes vendors can see them. `userInput`, `tool.args`, `tool.result`, and `final_answer` may contain personal data, tokens, secrets, medical or financial information. That's why in `attribute.String(...)` we put **only** one of the following:
>
> - **metadata**: `len(input)`, `tool.name`, token counts, latency, status;
> - a **redacted form**: prefix/suffix + a hash (`"client@a***...x9f3"`);
> - a **correlation ID**: `prompt_id`, `tool_call_id`, `message_hash` — something you can use to find the full content in a protected storage with RBAC and a retention policy.
>
> Never raw text. If you really need a dump for debugging — use a separate sampler and a protected storage, not the shared trace.

We wrap every step of the agent in a span:

```go
import (
    "crypto/sha256"
    "fmt"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("devops-agent")

// redact turns any potentially sensitive value into a safe label:
// the length of the original text plus a short sha256 prefix. You can use
// this label to correlate a span with a record in a protected log storage,
// but the original text cannot be recovered.
//
// The same principle works one level down: redaction is enabled by default
// in tool output too — the model and the logs see a filtered version
// (secrets, keys, PII), while the full result stays in protected storage.
// Details in [Chapter 17: Security and Governance](../17-security-and-governance/README.md)
// and [Chapter 24: Data and Privacy](../24-data-and-privacy/README.md).
func redact(s string) string {
    if s == "" {
        return ""
    }
    sum := sha256.Sum256([]byte(s))
    return fmt.Sprintf("len=%d sha256=%x", len(s), sum[:6])
}

func runAgentWithTracing(ctx context.Context, client Client, userInput string) (string, error) {
    runID := generateRunID()
    model := "glm-5.3" // the model name is a safe attribute, unlike args/result

    // The root span covers the entire agent run. We do NOT put the full
    // userInput text into attributes; we keep only run_id (used to look up
    // the incoming request in protected storage) and a redacted label for debugging.
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
    toolDefs := []ToolDef{/* run tools (ch. 03) */}

    maxIterations := 5 // terminal run boundary (ch. 04)

    for i := 0; i < maxIterations; i++ {
        // A span for each iteration of the loop
        ctx, iterSpan := tracer.Start(ctx, "agent_iteration",
            trace.WithAttributes(attribute.Int("iteration", i)),
        )

        // A span for the LLM call
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

        // Tokens — prompt/completion separately, plus cache-read:
        // a hit on the provider's prompt cache shows up as cache_read > 0
        llmSpan.SetAttributes(
            attribute.Int("tokens.prompt", resp.Usage.PromptTokens),
            attribute.Int("tokens.completion", resp.Usage.CompletionTokens),
            attribute.Int("tokens.cache_read", resp.Usage.CacheReadTokens),
        )
        llmSpan.End()

        msg := resp.Choices[0].Message
        // Append the assistant message to the history before the tool calls:
        // tool results must immediately follow their assistant message (protocol, ch. 03)
        messages = append(messages, msg)
        if len(msg.ToolCalls) == 0 {
            // The model's final answer can contain PII too — store only the redacted form.
            // The full answer goes to protected storage keyed by run_id.
            rootSpan.SetAttributes(
                attribute.String("final_answer.redacted", redact(msg.Content)),
                attribute.Int("final_answer.length", len(msg.Content)),
            )
            iterSpan.End()
            return msg.Content, nil
        }

        // A span for each tool call: the tool name and tool_call_id are safe,
        // arguments and result are not.
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
                // An error is a result too for the model: the loop continues
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

    // Exhausting the limit — a terminal boundary (STYLE, ch. 04)
    return "", fmt.Errorf("agent exceeded max iterations (%d)", maxIterations)
}
```

> **What to write to protected storage.** Store the pairs "`run_id` → original `userInput` / `final_answer`" and "`tool.call_id` → original `arguments` / `result`" in a separate logger with service-side encryption, RBAC across SRE/Privacy/Compliance roles, and a 7–30 day retention. That way you can still debug an incident, but the shared trace contains not a single byte of PII.

### Propagation: passing context between services

If the agent calls an external HTTP service, the tracing context needs to be passed along. OTEL uses the `traceparent` and `tracestate` HTTP headers:

```go
import (
    "go.opentelemetry.io/otel/propagation"
    "net/http"
)

// Sender: inject the context into the HTTP headers
func callExternalService(ctx context.Context, url string) (*http.Response, error) {
    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)

    // OTEL injects traceparent into the request headers
    otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

    return http.DefaultClient.Do(req)
}

// Receiver: extract the context from the HTTP headers
func handleRequest(w http.ResponseWriter, r *http.Request) {
    // Extract the trace context from the incoming request
    ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

    // From now on, spans in this service will be part of the original trace
    ctx, span := tracer.Start(ctx, "external_service.handle")
    defer span.End()

    // ... handle the request ...
}
```

After that, in Jaeger or Grafana Tempo you will see a single trace passing through all the services.

> **OTEL exporters:** For local development, Jaeger is convenient (`docker run -p 16686:16686 jaegertracing/all-in-one`). For production — Grafana Tempo or Zipkin. OTEL supports them all through a single API.

## Multi-Agent Tracing — Tracing Multiple Agents

In [Chapter 07: Multi-Agent Systems](../07-multi-agent/README.md) we split agents into a Supervisor and Workers. Each Worker is a separate agent loop with its own context. Without tracing, it is impossible to tell which Worker slowed things down, which one made a mistake, or how a task traveled through the whole system.

### Parent-child: supervisor span → worker span

The key idea: the Supervisor's span becomes the **parent** of the Workers' spans. This way we see the full execution tree:

```
supervisor_run
├── llm_call (the Supervisor decides whom to delegate to)
├── worker_call: network_expert
│   ├── llm_call
│   └── tool_call: ping_host
├── worker_call: db_expert
│   ├── llm_call
│   └── tool_call: check_db_version
└── llm_call (the Supervisor assembles the final answer)
```

### The code: passing the trace context from supervisor to worker

```go
// Worker receives a context with trace information from the Supervisor
func runWorker(ctx context.Context, client Client, model string, workerName string, task string, toolDefs []ToolDef) (string, error) {
    // Create a child span — it automatically links to the parent from ctx
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
            // Provider error before reading Usage: resp == nil — nothing to panic over
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
        // The assistant message with tool_calls goes into the history before their results (ch. 03)
        messages = append(messages, msg)
        if len(msg.ToolCalls) == 0 {
            // The Worker's result may contain PII — a redacted label only
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
                // The error is logged, not swallowed: an error result goes to the model
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

The Supervisor calls the Worker by simply passing `ctx` along:

```go
func runSupervisor(ctx context.Context, client Client, userInput string) (string, error) {
    ctx, span := tracer.Start(ctx, "supervisor_run",
        trace.WithAttributes(
            attribute.String("user_input.redacted", redact(userInput)),
        ),
    )
    defer span.End()

    // Supervisor decides whom to delegate to
    // ... (LLM call for routing) ...

    // ctx already carries the Supervisor's trace context.
    // The Worker will create a child span automatically.
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

    // ... the Supervisor assembles the final answer ...
    return finalAnswer, nil
}
```

### Cross-agent correlation

The trace ID is automatically propagated through `ctx`. All spans within one trace share a common `trace_id`. In Jaeger you search by this ID and see the full path: from the user's request through the Supervisor to each Worker and back.

If a Worker runs as a separate HTTP service, use the propagation from the previous section — pass `traceparent` in headers.

## Component-Level Metrics

Overall metrics (`total_tokens`, `avg_latency`) show the big picture. But diagnosis requires granularity: which tool exactly is slow? Which LLM call burns the most tokens? Which search query returns irrelevant results?

### Per-component metrics

Three categories of component-level metrics:

| Component | Metrics | Why |
|-----------|---------|-----|
| **Tool** | Latency, error rate, calls/min | Find slow tools |
| **LLM** | Tokens, latency, cost per call | Optimize token spend |
| **Retrieval** | Relevance score, documents found, latency | Improve RAG quality |

### Code: collecting per-component metrics

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
    RelevanceScores  []float64 // for computing the average
}

func NewComponentMetrics() *ComponentMetrics {
    return &ComponentMetrics{
        tools:     make(map[string]*ToolMetrics),
        llm:       &LLMMetrics{},
        retrieval: &RetrievalMetrics{},
    }
}
```

Recording metrics on a tool call:

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
    cm.llm.TotalCost += callCost(model, u) // prices come from config, see below
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

Usage in the agent loop:

```go
metrics := NewComponentMetrics()

messages := []Message{/* run history (ch. 04) */}
toolDefs := []ToolDef{/* run tools (ch. 03) */}

// On an LLM call
llmStart := time.Now()
resp, err := client.Complete(ctx, Request{Messages: messages, Tools: toolDefs})
if err == nil {
    metrics.RecordLLMCall(model, resp.Usage, time.Since(llmStart))
}

// On a tool call
toolStart := time.Now()
result, err := executeTool(toolCall)
metrics.RecordToolCall(toolCall.Name, time.Since(toolStart), err)
```

### Cost: a per-model price table in config

Prices change — keep them in config, not in code. A single "per token" price does not work: prompt, completion and cache read are billed differently, and each model has its own rates.

```go
// ModelPricing — rates for one model, $ per 1M tokens.
type ModelPricing struct {
    PromptPer1M     float64 // regular prompt tokens
    CompletionPer1M float64 // generated tokens
    CacheReadPer1M  float64 // cache prefix reads — usually several times cheaper than prompt
}

// Loaded from config (YAML/env) at startup — not hardcoded in the source.
var pricing = map[string]ModelPricing{ /* from loadPricingFromConfig() */ }

// callCost computes the cost of one call from the provider's Usage.
// Cache reads are billed at a separate rate, not as full prompt.
func callCost(model string, u Usage) float64 {
    p := pricing[model]
    cached := float64(u.CacheReadTokens)
    fresh := float64(u.PromptTokens - u.CacheReadTokens)
    return fresh/1e6*p.PromptPer1M +
        cached/1e6*p.CacheReadPer1M +
        float64(u.CompletionTokens)/1e6*p.CompletionPer1M
}
```

### Linking traces to evals

Traces show **how** the agent completed the task. Evals show **how well** it did. A trace → eval link gives the full picture:

```go
type TracedEvalResult struct {
    TraceID          string   `json:"trace_id"`          // trace ID from OpenTelemetry
    EvalLevel        string   `json:"eval_level"`        // task, tool, trajectory, topic
    Score            float64  `json:"score"`
    ToolsUsed        []string `json:"tools_used"`
    PromptTokens     int      `json:"prompt_tokens"`
    CompletionTokens int      `json:"completion_tokens"`
    TotalLatency     string   `json:"total_latency"`
}

// After an agent run finishes: record the eval result linked to the trace
func recordTracedEval(ctx context.Context, result TracedEvalResult) {
    span := trace.SpanFromContext(ctx)
    result.TraceID = span.SpanContext().TraceID().String()

    // Now, given the TraceID, you can find the full trace in Jaeger
    // and understand WHY the eval failed
    log.Printf("EVAL_RESULT: %+v", result)
}
```

When an eval in CI/CD shows a regression (see [Chapter 23: Evals in CI/CD](../23-evals-in-cicd/README.md)), you take the `trace_id` from the result, find the trace in Jaeger and see the whole chain: which tool returned an unexpected result, which LLM call spent an anomalously large number of tokens, where the error occurred.

## Metrics and Dashboards

Logs and traces answer "what happened in this run". Metrics answer "how the system behaves over time". The standard stack: the agent exposes Prometheus metrics on `/metrics`, and Grafana dashboards chart them over time.

### Basic agent metrics

| Metric | What it shows |
|---------|----------------|
| `llm_tokens_total{kind=prompt\|completion\|cache_read}` | token spend by kind; cache_read > 0 — a prompt cache hit |
| `agent_iterations_total` | how many iterations the agent makes per task (growing — the agent is stuck in a loop) |
| `tool_calls_total{tool}` / `tool_errors_total{tool}` | tool load and reliability |
| `policy_denied_total{policy=fs\|authority}` | security policy denials (ch. 17) |
| `failovers_total` / `retries_total` | failovers to a fallback model and client retries |
| `agent_run_duration_seconds` | run latency |

### The "dashboard ↔ metric registry" guard test

A dashboard silently rots: a metric gets renamed, the chart goes empty, and nobody notices. The cure is a test: keep a single metric registry (a struct or a list of names) and a test that parses the dashboard's JSON model and verifies that **every metric in the registry appears on at least one panel**. Rename a metric and forget the dashboard — CI fails, not your production monitoring.

### Live evals as part of observability

Metrics show "how much", but not "how well". That is why live evals live alongside the Grafana panels: periodic eval runs of the agent on a fixed task set in the production environment, whose scores also become metrics (`eval_score{suite}`). A quality degradation is visible on the dashboard next to rising latency — sometimes it is the same incident. How to build evals — [Chapter 08: Evals and Reliability](../08-evals-and-reliability/README.md) and [Chapter 23: Evals in CI/CD](../23-evals-in-cicd/README.md).

> **How the real SDK does it.** `RunResult` carries everything needed for the metrics of a single run: usage (prompt/completion/cache), iterations, the list of tools called, failovers and the final model. Loop events (stream deltas, the final answer, tool start/progress/end) are natural tracing points: both the logger and the metrics collector subscribe to them. Transient errors are retried by the client itself — the retry policy is configured on the model, not in the body of the loop.

## Common Mistakes

A brief summary of typical observability failures:

| # | Mistake | Symptom | Fix |
|---|--------|---------|---------|
| 1 | No structured logging | Logs are plain text, cannot be parsed automatically | JSON logs with fields (breakdown below) |
| 2 | No `run_id` for correlation | Component logs cannot be tied into one picture | A unique `run_id` per run, propagated everywhere |
| 3 | Tokens logged as one total | The budget "drifts", cache is invisible | `PromptTokens`/`CompletionTokens` + cache read (breakdown below) |
| 4 | Only successes are logged | Errors are lost, causes of failures unknown | Log error branches too, with context |
| 5 | Logs only, no metrics | No latency/error rate/token charts over time | Prometheus `/metrics` + Grafana (section above) |

### Breakdown 1: no structured logging

**Symptom:** logs as plain text: "Agent executed tool check_disk". Impossible to parse or analyze automatically.

**Cause:** `fmt.Printf` instead of structured JSON logging.

**Fix:**
```go
// BAD:
fmt.Printf("Agent executed tool %s\n", toolName)

// GOOD:
logJSON, _ := json.Marshal(map[string]any{
    "run_id": runID,
    "tool": toolName,
    "timestamp": time.Now(),
})
log.Printf("TOOL_EXECUTED: %s", string(logJSON))
```

### Breakdown 2: budget based on `TotalTokens`

**Symptom:** the API bill diverges from your token metric by tens of percent; the effect of the prompt cache is not visible at all.

**Cause:** `Usage.TotalTokens` is used as the budget metric — an aggregate that hides cache prefixes and mixes paid tokens with cache-cheap ones (the canon of [chapter 13](../13-context-engineering/README.md)).

**Fix:** log and sum the components separately:

```go
// BAD:
log.Printf("TOKENS_USED: run_id=%s tokens=%d", runID, resp.Usage.TotalTokens)

// GOOD:
log.Printf("TOKENS_USED: run_id=%s prompt=%d completion=%d cache_read=%d",
    runID, resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.CacheReadTokens)
```

`cache_read > 0` — a hit on the provider's prompt cache; the ratio `cache_read / prompt` is a direct measure of how well your immutable system prefix works (ch. 20).

## Exercises

### Exercise 1: Implement structured logging

Add structured logging to your runtime from chapter 04 (the lab04-autonomy lab is not yet revised — use it as a pointer; the canonical material is in this chapter):

```go
func logAgentRun(runID string, userInput string, toolCalls []ToolCall, result string) {
    // Your code here
    // Log in JSON format
}
```

**Expected result:**
- Logs in JSON format
- Contain all the required fields: run_id, user_input, tool_calls, result, timestamp

### Exercise 2: Add tracing of tool calls

Implement a tool call logging function with latency measurement:

```go
func logToolCall(runID string, toolCallID string, toolName string, latency time.Duration) {
    // Your code here
    // Log in the format: TOOL_CALL: run_id=... tool_id=... tool_name=... latency=...
}
```

**Expected result:**
- Every tool call is logged with run_id
- Latency is measured and logged
- The format is easy to parse

### Exercise 3: Implement metrics

Add metric counting (a simple in-memory counter will do):

```go
type Metrics struct {
    TotalRuns          int64
    TotalPromptTokens  int64
    TotalCompletion    int64
    TotalErrors        int64
    AvgLatency         time.Duration
}

func (m *Metrics) RecordRun(usage Usage, latency time.Duration, err error) {
    // Your code here
    // Update the metrics: usage.PromptTokens, usage.CompletionTokens — from the provider
}
```

**Expected result:**
- Metrics are updated on every agent run
- Tokens are counted by the provider's fields (prompt/completion), not by TotalTokens
- Averages can be obtained via methods like `GetAvgLatency()`

## Acceptance Criteria / Checklist

**Passing (production readiness):**
- [x] Structured logging is implemented (JSON format)
- [x] Every agent run has a unique `run_id`
- [x] All stages are logged: input request → tool selection → execution → result
- [x] Tokens are logged by the provider's fields: prompt / completion / cache_read
- [x] Tool calls are logged with `run_id` for correlation
- [x] Errors are logged with context, without secrets (redaction by default)
- [x] Metrics are tracked (latency, tokens by kind, iterations, tool calls, error rate)

**Not passing:**
- [ ] Logs are plain text without structure
- [ ] No `run_id` for log correlation
- [ ] Tokens and latency are not logged
- [ ] Errors are not logged
- [ ] No metrics

## Relation to Other Chapters

- **Best Practices:** general logging practices are covered in [Chapter 16: Best Practices](../16-best-practices/README.md)
- **Agent Loop:** the basic agent loop is covered in [Chapter 04: Autonomy and Loops](../04-autonomy-and-loops/README.md)
- **Tools:** tool execution is covered in [Chapter 03: Tools and Function Calling](../03-tools-and-function-calling/README.md)
- **Cost Engineering:** using tokens for cost control — [Chapter 20: Cost & Latency Engineering](../20-cost-latency-engineering/README.md)

