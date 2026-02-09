# 20. Cost & Latency Engineering

## Why This Chapter?

An agent works, but the LLM API bill grows uncontrollably. One request costs $5, another — $0.10. Why? Without cost control and latency optimization, you can't:
- Predict monthly budget
- Optimize expensive requests
- Guarantee response time to user

Cost & latency engineering gives you budget and performance control. Without it, you can spend thousands on simple requests—or ship an agent so slow nobody uses it.

### Real-World Case Study

**Situation:** A DevOps agent works in production for a week. LLM API bill — $5000 per month instead of expected $500.

**Problem:** Agent uses GPT-4 for all requests, even for simple status checks. No token limits, no caching, no fallback to cheaper models. One request "check server status" uses 50,000 tokens due to large context.

**Solution:** Token budgets, iteration limits, caching, model routing by task complexity. Now simple requests use a cheap model (GPT-4o-mini), complex ones use a powerful model (GPT-4o). Bill reduced to $600 per month.

> **Note:** LLM API prices change frequently. Check provider websites for current pricing. Prices in this chapter are approximate and shown to illustrate relative cost differences.

## Theory in Simple Terms

### What Is Cost Engineering?

Cost Engineering is the control and optimization of LLM API usage costs. Main levers:
1. **Model selection** — powerful models (GPT-4o) cost several times more than lightweight ones (GPT-4o-mini)
2. **Token count** — larger context = more expensive
3. **Request count** — each LLM call costs money
4. **Caching** — identical requests can be skipped

### What Is Latency Engineering?

Latency Engineering is response time control. Main factors:
1. **Model** — powerful models are slower than lightweight ones
2. **Context size** — more tokens = more processing time
3. **Iteration count** — more ReAct cycles = more time
4. **Timeouts** — protection against hangs

## How It Works (Step by Step)

### Step 1: Token Budgets

Set token limit per request:

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

### Step 2: Iteration Limits

Limit the number of ReAct loop iterations (see [`labs/lab04-autonomy/main.go`](https://github.com/kshvakov/ai-agent-course/blob/main/labs/lab04-autonomy/main.go)):

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
        
        // Check budget
        if err := checkTokenBudget(tokenBudget); err != nil {
            return "", fmt.Errorf("stopping: %v", err)
        }
        
        // ... rest of code ...
    }
    
    return "", fmt.Errorf("max iterations (%d) exceeded", maxIterations)
}
```

### Step 2.5: Artifacts for large tool results

One of the fastest ways to reduce cost and latency is to stop putting large tool outputs directly into `messages[]`.

Pattern:

- store the raw tool output as an artifact in external storage,
- add only a short excerpt (for example, top-20 lines) plus `artifact_id` to the conversation,
- fetch the artifact later in slices if needed (`range`/`offset`/`limit`).

This often works better than summarization. You keep the full data, but you keep it out of the context window.

### Step 3: LLM Result Caching

Cache results for identical requests:

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
    // Create key from message content
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

### Step 4: Model Routing by Complexity

Use cheaper models for simple tasks (see [`labs/lab09-context-optimization/main.go`](https://github.com/kshvakov/ai-agent-course/blob/main/labs/lab09-context-optimization/main.go)):

```go
func selectModel(taskComplexity string) string {
    switch taskComplexity {
    case "simple":
        return "gpt-4o-mini" // Cheaper and faster
    case "complex":
        return "gpt-4o" // Better quality, but more expensive
    default:
        return "gpt-4o-mini"
    }
}

func assessTaskComplexity(userInput string) string {
    // Simple tasks: status check, read logs
    simpleKeywords := []string{"check", "status", "read", "get", "list"}
    for _, keyword := range simpleKeywords {
        if strings.Contains(strings.ToLower(userInput), keyword) {
            return "simple"
        }
    }
    // Complex tasks: analysis, planning, problem solving
    return "complex"
}
```

### Step 5: Fallback Models

Implement fallback chain on errors or budget exceeded:

```go
func createChatCompletionWithFallback(ctx context.Context, client *openai.Client, req openai.ChatCompletionRequest) (*openai.ChatCompletionResponse, error) {
    models := []string{"gpt-4o", "gpt-4o-mini"}
    
    for _, model := range models {
        req.Model = model
        resp, err := client.CreateChatCompletion(ctx, req)
        if err == nil {
            return resp, nil
        }
        // If error, try next model
    }
    
    return nil, fmt.Errorf("all models failed")
}
```

### Step 6: Timeouts

Set timeout for entire agent run and for each tool call:

```go
import "context"
import "time"

func runAgentWithTimeout(ctx context.Context, client *openai.Client, userInput string) (string, error) {
    // Timeout for entire agent run (5 minutes)
    ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
    defer cancel()
    
    // ... agent loop ...
    
    for i := 0; i < maxIterations; i++ {
        // Timeout for each LLM call (30 seconds)
        callCtx, callCancel := context.WithTimeout(ctx, 30*time.Second)
        resp, err := client.CreateChatCompletion(callCtx, req)
        callCancel()
        
        if err != nil {
            if err == context.DeadlineExceeded {
                return "", fmt.Errorf("LLM call timeout")
            }
            return "", err
        }
        
        // ... rest of code ...
    }
}
```

## Where to Integrate This in Our Code

### Integration Point 1: Agent Loop

In [`labs/lab04-autonomy/main.go`](https://github.com/kshvakov/ai-agent-course/blob/main/labs/lab04-autonomy/main.go) add budget check and iteration limit:

```go
const MaxIterations = 10
const MaxTokensPerRequest = 10000

// In loop:
for i := 0; i < MaxIterations; i++ {
    resp, err := client.CreateChatCompletion(ctx, req)
    // Check tokens
    if resp.Usage.TotalTokens > MaxTokensPerRequest {
        return "", fmt.Errorf("token limit exceeded")
    }
    // ... rest of code ...
}
```

### Integration Point 2: Context Optimization

In [`labs/lab09-context-optimization/main.go`](https://github.com/kshvakov/ai-agent-course/blob/main/labs/lab09-context-optimization/main.go) token counting already exists. Add budget check:

```go
func adaptiveContextManagement(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage, maxTokens int) []openai.ChatCompletionMessage {
    usedTokens := countTokensInMessages(messages)
    
    // If budget exceeded, apply aggressive optimization
    if usedTokens > MaxTokensPerRequest {
        return compressOldMessages(ctx, client, messages, maxTokens)
    }
    
    // ... rest of logic ...
}
```

## Mini Code Example

Complete example with cost control based on [`labs/lab04-autonomy/main.go`](https://github.com/kshvakov/ai-agent-course/blob/main/labs/lab04-autonomy/main.go):

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

    userInput := "I'm out of disk space. Fix it."
    
    // Determine task complexity
    model := selectModel("simple") // For simple tasks use GPT-3.5
    
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
        // Check budget before request
        if err := checkTokenBudget(tokenBudget); err != nil {
            fmt.Printf("Budget exceeded: %v\n", err)
            break
        }

        req := openai.ChatCompletionRequest{
            Model:    model,
            Messages: messages,
            Tools:    tools,
        }

        // Timeout for each LLM call
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

        // Update budget
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

## Multi-Model in a Single Agent Run

In [Step 4](#step-4-model-routing-by-complexity) we picked one model for the entire request: simple task — cheap model, complex — expensive. But within a single agent run, different **stages** need different levels of intelligence.

Think about it: to pick a tool from a list, the model doesn't need deep understanding of the task. It's like grabbing a screwdriver from a toolbox — a mechanical action. But to analyze the tool's result and formulate an answer — that takes thinking.

### Three Iteration Stages

Each iteration of the agent loop (see [Chapter 04: Autonomy and Loops](../04-autonomy-and-loops/README.md)) consists of three stages:

| Stage | What Happens | Needs a Powerful Model? | Example Model |
|-------|-------------|------------------------|---------------|
| 1. Tool selection | LLM decides which tool to call | No — tool list is fixed | GPT-4o-mini (~$0.15/1M input) |
| 2. Result analysis | LLM interprets tool output | Yes — needs context understanding | GPT-4o (~$2.50/1M input) |
| 3. Final answer | LLM formulates user response | Yes — answer quality is critical | GPT-4o (~$2.50/1M input) |

### Economics: Concrete Numbers

Say the agent completes a task in 8 iterations. Each LLM call uses ~2000 tokens.

**Single model for all stages (GPT-4o):**
- 8 iterations × 2 calls (tool selection + analysis) = 16 calls
- 16 × ~$0.01 = **~$0.16 per task**

**Multi-model approach:**
- 8 tool selection calls × ~$0.0002 (GPT-4o-mini) = $0.0016
- 7 result analysis calls × ~$0.01 (GPT-4o) = $0.07
- 1 final answer × ~$0.01 (GPT-4o) = $0.01
- Total: **~$0.08 per task**

~50% savings with no quality loss. At 10,000 requests per month, that's the difference between $1,600 and $800.

### Implementation: ModelRouter

```go
// Stage defines the agent iteration stage
type Stage int

const (
    StageToolSelection Stage = iota // Tool selection
    StageResultAnalysis             // Tool result analysis
    StageFinalAnswer                // Final answer generation
)

// ModelRouter selects a model based on the iteration stage.
// A cheap model handles tool selection from a fixed list,
// a powerful model is needed for analysis and answer generation.
type ModelRouter struct {
    CheapModel    string // For mechanical decisions (tool selection)
    PowerfulModel string // For analysis and answer generation
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

Integration into the agent loop:

```go
func runAgentMultiModel(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage, tools []openai.Tool) (string, error) {
    router := NewModelRouter()

    for i := 0; i < MaxIterations; i++ {
        // Stage 1: tool selection — cheap model
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

        // If no tool calls — this is the final answer,
        // but it was generated by the cheap model. Regenerate with the powerful one.
        if len(msg.ToolCalls) == 0 {
            return regenerateWithPowerfulModel(ctx, client, router, messages)
        }

        // Execute tools
        toolResults := executeTools(msg.ToolCalls)
        messages = append(messages, toolResults...)

        // Stage 2: result analysis — powerful model
        // Next loop iteration decides: another tool call or final answer.
        // If another tool call is needed, we switch back to the cheap model.
    }

    return "", fmt.Errorf("max iterations exceeded")
}

// regenerateWithPowerfulModel regenerates the final answer with the powerful model.
// The cheap model is good at picking tools, but the final answer
// must be high quality.
func regenerateWithPowerfulModel(ctx context.Context, client *openai.Client, router *ModelRouter, messages []openai.ChatCompletionMessage) (string, error) {
    // Remove the last response from the cheap model
    messages = messages[:len(messages)-1]

    req := openai.ChatCompletionRequest{
        Model:    router.SelectModel(StageFinalAnswer),
        Messages: messages,
        // No tools — request text-only answer
    }
    resp, err := client.CreateChatCompletion(ctx, req)
    if err != nil {
        return "", err
    }
    return resp.Choices[0].Message.Content, nil
}
```

> **Connection with other chapters:** The Function Calling mechanism used at the tool selection stage is covered in [Chapter 03: Tools and Function Calling](../03-tools-and-function-calling/README.md). The agent loop itself — in [Chapter 04: Autonomy and Loops](../04-autonomy-and-loops/README.md).

## Streaming to Reduce Perceived Latency

There are two kinds of latency:
- **Actual** — time from request to the last token of the response
- **Perceived** — time the user *feels* as waiting

Streaming doesn't speed up generation. The model spends the same total time on the response. But the user sees the first words in 200-500 ms instead of waiting 5-10 seconds for the full response. This fundamentally changes UX.

### How Streaming Works

Without streaming: request → wait 5 seconds → entire response at once.

With streaming: request → 200 ms → first token → tokens appear one by one → response complete.

Under the hood it's Server-Sent Events (SSE) — the HTTP connection stays open, and the server sends data as it becomes available.

### Streaming in the Agent Loop

```go
func streamAgentResponse(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage) error {
    req := openai.ChatCompletionRequest{
        Model:    "gpt-4o",
        Messages: messages,
        Stream:   true, // Enable streaming
    }

    stream, err := client.CreateChatCompletionStream(ctx, req)
    if err != nil {
        return fmt.Errorf("stream error: %w", err)
    }
    defer stream.Close()

    for {
        chunk, err := stream.Recv()
        if errors.Is(err, io.EOF) {
            fmt.Println() // Newline after response
            return nil
        }
        if err != nil {
            return fmt.Errorf("stream recv error: %w", err)
        }

        // Each chunk contains one or more tokens
        if len(chunk.Choices) > 0 {
            token := chunk.Choices[0].Delta.Content
            fmt.Print(token) // Print token immediately, no buffering
        }
    }
}
```

### SSE Streaming for Web Clients

If the agent runs as an HTTP server, streaming is delivered to the client via SSE:

```go
func handleStream(w http.ResponseWriter, r *http.Request) {
    // Check streaming support
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
            // SSE format: "data: ...\n\n"
            fmt.Fprintf(w, "data: %s\n\n", token)
            flusher.Flush() // Send immediately, no buffering
        }
    }
}
```

### When Streaming Doesn't Help

Streaming reduces perceived latency only for the final user-facing response. Intermediate LLM calls (tool selection, result analysis) don't need streaming — the user doesn't see them. For intermediate calls, actual latency matters. Reduce it by picking a faster model and optimizing context.

## Parallel Tool Call Execution

The LLM can return multiple tool calls in a single iteration (see [Chapter 03: Tools and Function Calling](../03-tools-and-function-calling/README.md)). If the tools are independent, you can execute them in parallel.

Parallel execution **doesn't save tokens** — the LLM generated all tool calls in one request anyway. But it **cuts wall-clock time** — the time from the start to the end of an iteration.

### When It's Safe to Parallelize

| Safe | Unsafe |
|------|--------|
| `check_disk` + `check_memory` — read different resources | `create_file` → `write_to_file` — second depends on first |
| `get_logs("app-1")` + `get_logs("app-2")` — different servers | `stop_service` → `deploy` → `start_service` — strict order |
| `dns_lookup` + `ping` — independent checks | `read_config` → `apply_config` — read first, then apply |

Rule: if one tool's result is needed as input for another — execute sequentially. Otherwise — in parallel.

### Implementation: executeToolsParallel

```go
// ToolResult stores the result of a single tool execution
type ToolResult struct {
    ToolCallID string
    Content    string
    Err        error
}

// executeToolsParallel runs all tool calls concurrently.
// Each tool runs in its own goroutine.
func executeToolsParallel(ctx context.Context, toolCalls []openai.ToolCall) []openai.ChatCompletionMessage {
    results := make([]ToolResult, len(toolCalls))

    var wg sync.WaitGroup
    for i, tc := range toolCalls {
        wg.Add(1)
        go func(idx int, call openai.ToolCall) {
            defer wg.Done()

            // Per-tool timeout
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

    // Collect results as messages
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

Integration into the agent loop — replace sequential execution with parallel:

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

    // Before: sequential execution
    // for _, tc := range msg.ToolCalls { ... }

    // After: parallel execution
    toolMessages := executeToolsParallel(ctx, msg.ToolCalls)
    messages = append(messages, toolMessages...)
}
```

### How Much Time You Save

Say the agent called 3 tools in one iteration. Each tool takes ~2 seconds.

- **Sequential:** 2 + 2 + 2 = 6 seconds
- **Parallel:** max(2, 2, 2) = 2 seconds

Over 8 iterations with 2-3 tool calls each — that's the difference between 48 seconds and 16 seconds. The user waits 3x less.

> **Connection with Chapter 04:** In [Chapter 04: Autonomy and Loops](../04-autonomy-and-loops/README.md), tool calls run sequentially. Parallel execution is a natural optimization once the agent works reliably and you start optimizing latency.

## Common Errors

### Error 1: No Token Limits

**Symptom:** LLM API bill grows uncontrollably. One request can use 100,000 tokens.

**Cause:** Token usage is not checked before sending request.

**Solution:**
```go
// BAD
resp, _ := client.CreateChatCompletion(ctx, req)
// No token check

// GOOD
tokenBudget.Used += resp.Usage.TotalTokens
if err := checkTokenBudget(tokenBudget); err != nil {
    return "", err
}
```

### Error 2: No Iteration Limit

**Symptom:** Agent loops and makes 50+ iterations, spending thousands of tokens.

**Cause:** No limit on ReAct loop iterations.

**Solution:**
```go
// BAD
for {
    // Infinite loop
}

// GOOD
for i := 0; i < MaxIterations; i++ {
    // Limited iterations
}
```

### Error 3: Using GPT-4 for All Tasks

**Symptom:** Simple requests cost 15x more than needed.

**Cause:** Always using the most expensive model.

**Solution:**
```go
// BAD
req.Model = "gpt-4o" // Always GPT-4

// GOOD
model := selectModel(assessTaskComplexity(userInput))
req.Model = model
```

### Error 4: No Caching

**Symptom:** Identical requests execute repeatedly, wasting tokens.

**Cause:** No cache for LLM results.

**Solution:**
```go
// BAD
resp, _ := client.CreateChatCompletion(ctx, req)
// New request every time

// GOOD
key := getCacheKey(messages)
if result, ok := getCachedResult(key); ok {
    return result, nil
}
resp, _ := client.CreateChatCompletion(ctx, req)
setCachedResult(key, resp.Choices[0].Message.Content, 1*time.Hour)
```

### Error 5: No Timeouts

**Symptom:** Agent hangs for 10+ minutes, user waits.

**Cause:** No timeout for LLM calls or tools.

**Solution:**
```go
// BAD
resp, _ := client.CreateChatCompletion(ctx, req)
// May hang forever

// GOOD
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()
resp, err := client.CreateChatCompletion(ctx, req)
if err == context.DeadlineExceeded {
    return "", fmt.Errorf("timeout")
}
```

## Mini-Exercises

### Exercise 1: Implement Token Budget Check

Add budget check to [`labs/lab04-autonomy/main.go`](https://github.com/kshvakov/ai-agent-course/blob/main/labs/lab04-autonomy/main.go):

```go
func checkTokenBudget(used int, limit int) error {
    // Your code here
    // Return error if limit exceeded
}
```

**Expected result:**
- Function returns error if tokens used exceed limit
- Function returns nil if limit not exceeded

### Exercise 2: Implement Model Routing

Create a function to select model by task complexity:

```go
func selectModelByComplexity(userInput string) string {
    // Your code here
    // Return GPT-3.5 for simple tasks, GPT-4 for complex
}
```

**Expected result:**
- Simple tasks (check, status, read) → GPT-3.5
- Complex tasks (analyze, fix, plan) → GPT-4

### Exercise 3: Implement Caching

Add simple in-memory caching for LLM results:

```go
var cache = make(map[string]CacheEntry)

func getCachedResult(key string) (string, bool) {
    // Your code here
}

func setCachedResult(key string, result string, ttl time.Duration) {
    // Your code here
}
```

**Expected result:**
- Identical requests returned from cache
- Cache has TTL (time to live)

## Completion Criteria / Checklist

**Completed (production ready):**
- [x] Token budgets implemented with check before each request
- [x] ReAct loop iteration limit set
- [x] Model routing by task complexity implemented
- [x] LLM result caching implemented
- [x] Timeouts set for LLM calls and agent run
- [x] Token usage tracked and warned when exceeded

**Not completed:**
- [ ] No token limits
- [ ] No iteration limit
- [ ] Always using most expensive model
- [ ] No caching
- [ ] No timeouts

## Connection with Other Chapters

- **Observability:** Logging token usage — [Chapter 19: Observability and Tracing](../19-observability-and-tracing/README.md)
- **Context Optimization:** Managing context size — [Chapter 13: Context Engineering](../13-context-engineering/README.md)
- **Agent Loop:** Basic agent loop — [Chapter 04: Autonomy and Loops](../04-autonomy-and-loops/README.md)

