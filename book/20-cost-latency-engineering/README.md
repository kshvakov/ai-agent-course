# 20. Cost & Latency Engineering

## Why This Chapter?

An agent works, but the LLM API bill grows uncontrollably. One request costs $5, another — $0.10. Why? Without cost control and latency optimization, you can't:
- Predict monthly budget
- Optimize expensive requests
- Guarantee response time to user

Cost and Latency Engineering provides budget and performance control. Without it, you risk spending thousands of dollars on simple requests or ending up with a slow agent that users won't use.

### Real-World Case Study

**Situation:** A DevOps agent works in production for a week. LLM API bill — $5000 per month instead of expected $500.

**Problem:** Agent uses GPT-4 for all requests, even for simple status checks. No token limits, no caching, no fallback to cheaper models. One request "check server status" uses 50,000 tokens due to large context.

**Solution:** Token budgets, iteration limits, caching, model routing by task complexity. Now simple requests use GPT-3.5 ($0.002 per 1K tokens), complex — GPT-4 ($0.03 per 1K tokens). Bill reduced to $600 per month.

## Theory in Simple Terms

### What Is Cost Engineering?

Cost Engineering is the control and optimization of LLM API usage costs. Main levers:
1. **Model selection** — GPT-4 is 15x more expensive than GPT-3.5
2. **Token count** — larger context = more expensive
3. **Request count** — each LLM call costs money
4. **Caching** — identical requests can be skipped

### What Is Latency Engineering?

Latency Engineering is response time control. Main factors:
1. **Model** — GPT-4 is slower than GPT-3.5
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

Limit the number of ReAct loop iterations (see `labs/lab04-autonomy/main.go`):

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

Use cheaper models for simple tasks (see `labs/lab09-context-optimization/main.go`):

```go
func selectModel(taskComplexity string) string {
    switch taskComplexity {
    case "simple":
        return openai.GPT3Dot5Turbo // Cheaper and faster
    case "complex":
        return openai.GPT4 // Better quality, but more expensive
    default:
        return openai.GPT3Dot5Turbo
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
    models := []string{openai.GPT4, openai.GPT3Dot5Turbo}
    
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

In `labs/lab04-autonomy/main.go` add budget check and iteration limit:

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

In `labs/lab09-context-optimization/main.go` token counting already exists. Add budget check:

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

Complete example with cost control based on `labs/lab04-autonomy/main.go`:

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
req.Model = openai.GPT4 // Always GPT-4

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

Add budget check to `labs/lab04-autonomy/main.go`:

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

✅ **Completed (production ready):**
- Token budgets implemented with check before each request
- ReAct loop iteration limit set
- Model routing by task complexity implemented
- LLM result caching implemented
- Timeouts set for LLM calls and agent run
- Token usage tracked and warned when exceeded

❌ **Not completed:**
- No token limits
- No iteration limit
- Always using most expensive model
- No caching
- No timeouts

## Connection with Other Chapters

- **Observability:** Logging token usage — [Chapter 19: Observability and Tracing](../19-observability-and-tracing/README.md)
- **Context Optimization:** Managing context size — [Chapter 13: Context Engineering](../13-context-engineering/README.md)
- **Agent Loop:** Basic agent loop — [Chapter 04: Autonomy and Loops](../04-autonomy-and-loops/README.md)

---

**Navigation:** [← Observability and Tracing](../19-observability-and-tracing/README.md) | [Table of Contents](../README.md) | [Workflow and State Management in Production →](../21-workflow-state-management/README.md)
