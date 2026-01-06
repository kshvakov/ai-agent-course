# 19. Observability and Tracing

## Why This Chapter?

Without observability, you work blind. An agent performed an action, but you cannot understand:
- Why did it choose this particular tool?
- How long did execution take?
- How many tokens were used?
- Where did the error occur?

Observability is the "vision" for your agent. Without it, you cannot debug problems, optimize performance, or understand agent behavior in production.

### Real-World Case Study

**Situation:** Agent in production performed a data deletion operation without confirmation. User complained.

**Problem:** You cannot understand why the agent didn't request confirmation. Logs only show "Agent executed delete_database". No information about which tools were available, what prompt was used, what arguments were passed.

**Solution:** Structured logging with tracing shows the full picture: input request → tool selection → execution → result. Now you can understand that the agent didn't receive a tool with confirmation or the prompt was incorrect.

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

All these events are linked through `run_id`, allowing reconstruction of the full execution picture.

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

✅ **Completed (production ready):**
- Structured logging implemented (JSON format)
- Each agent run has unique `run_id`
- All stages logged: input request → tool selection → execution → result
- Tokens and latency logged for each request
- Tool calls logged with `run_id` for correlation
- Errors logged with context
- Metrics tracked (latency, token usage, error rate)

❌ **Not completed:**
- Logs in plain text without structure
- No `run_id` for log correlation
- Tokens and latency not logged
- Errors not logged
- No metrics

## Connection with Other Chapters

- **Best Practices:** General logging practices studied in [Chapter 16: Best Practices](../16-best-practices/README.md)
- **Agent Loop:** Basic agent loop studied in [Chapter 04: Autonomy and Loops](../04-autonomy-and-loops/README.md)
- **Tools:** Tool execution studied in [Chapter 03: Tools and Function Calling](../03-tools-and-function-calling/README.md)
- **Cost Engineering:** Token usage for cost control — [Chapter 20: Cost & Latency Engineering](../20-cost-latency-engineering/README.md)

---

**Navigation:** [← Tool Protocols and Tool Servers](../18-tool-protocols-and-servers/README.md) | [Table of Contents](../README.md) | [Cost & Latency Engineering →](../20-cost-latency-engineering/README.md)
