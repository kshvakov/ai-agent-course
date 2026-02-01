# 11. State Management

## Why This Chapter?

An agent executes a long task (e.g., application deployment), and then the server reboots. The task is lost, the user waits, but nothing happens. Without state management, you cannot:
- Resume execution after failure
- Guarantee idempotency (repeated call doesn't create duplicates)
- Handle errors with retry
- Set deadlines for long tasks

State Management provides reliability for long-lived agents. Without it, an agent cannot work with tasks that take minutes or hours.

### Real-World Case Study

**Situation:** Agent deploys an application. The process takes 10 minutes. On the 8th minute, the server reboots.

**Problem:** Task is lost. User doesn't know what happened. On restart, agent starts from the beginning, creating duplicates.

**Solution:** State persistence in DB, operation idempotency, retry with backoff, deadlines. Now the agent can resume execution from where it stopped, and repeated calls don't create duplicates.

## Theory in Simple Terms

### What Is State Management?

State Management is about saving agent state between restarts. This allows:
- Resume execution after failure
- Track task progress
- Guarantee idempotency

### What Is Idempotency?

Idempotency is a property of an operation: a repeated call gives the same result as the first. For example, "create file" isn't idempotent (creates duplicates), but "create file if it doesn't exist" is idempotent.

### Connection with Planning

State Management is closely related to [Planning](../10-planning-and-workflows/README.md), but focuses on execution reliability, not task decomposition. Planning creates a plan, State Management guarantees its reliable execution.

## How It Works (Step by Step)

### Step 0: Agent state as a contract (AgentState)

In the examples below we store a task state (`Task`). For production agents it's often helpful to also define a canonical **agent run** state. This makes long-running loops easier to operate.

What you get:

- resume after restarts,
- revise the plan when new facts arrive,
- enforce HITL based on policy,
- keep the context small by using artifacts.

A minimal shape (simplified):

```json
{
  "goal": "Deploy service X to staging",
  "constraints": {
    "human_in_the_loop": { "required_for_risk_levels": ["write_local", "external_action"] }
  },
  "budget": {
    "max_steps": 20,
    "max_wall_time_ms": 300000,
    "max_llm_tokens": 200000,
    "max_artifact_bytes_in_context": 8000
  },
  "plan": ["Check current status", "Collect config", "Apply changes", "Verify again"],
  "known_facts": [{ "key": "service", "value": "X", "source": "user" }],
  "open_questions": ["Which namespace should we deploy to?"],
  "artifacts": [{ "artifact_id": "log_123", "type": "tool_result.logs", "summary": "nginx error log", "bytes": 48231 }],
  "risk_flags": ["budget_pressure"]
}
```

To update this state between steps, use a **StatePatch**: "append facts", "replace plan", "add open questions". This also supports a clean split of responsibilities: one component normalizes observations, another selects the next action.

### Step 1: Task Structure with State

Create a structure to store task state:

```go
type TaskState string

const (
    TaskPending   TaskState = "pending"
    TaskRunning   TaskState = "running"
    TaskCompleted TaskState = "completed"
    TaskFailed    TaskState = "failed"
)

type Task struct {
    ID        string    `json:"id"`
    UserInput string    `json:"user_input"`
    State     TaskState `json:"state"`
    Result    string    `json:"result,omitempty"`
    Error     string    `json:"error,omitempty"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

### Step 2: Operation Idempotency

Check if the task was already executed:

```go
func executeTask(id string) error {
    // Load task from DB
    task, exists := getTask(id)
    if !exists {
        return fmt.Errorf("task not found: %s", id)
    }
    
    // Check idempotency
    if task.State == TaskCompleted {
        return nil // Already executed, do nothing
    }
    
    // Set state to "running"
    task.State = TaskRunning
    task.UpdatedAt = time.Now()
    saveTask(task)
    
    // Execute task...
    result, err := doWork(task.UserInput)
    
    if err != nil {
        task.State = TaskFailed
        task.Error = err.Error()
    } else {
        task.State = TaskCompleted
        task.Result = result
    }
    
    task.UpdatedAt = time.Now()
    saveTask(task)
    
    return err
}
```

### Step 3: Retry with Exponential Backoff

Retry call on error with increasing delay:

```go
func executeWithRetry(fn func() error, maxRetries int) error {
    var lastErr error
    
    for i := 0; i < maxRetries; i++ {
        err := fn()
        if err == nil {
            return nil
        }
        
        lastErr = err
        
        // Don't backoff after last attempt
        if i < maxRetries-1 {
            backoff := time.Duration(1<<i) * time.Second // 1s, 2s, 4s, 8s...
            time.Sleep(backoff)
        }
    }
    
    return fmt.Errorf("failed after %d retries: %v", maxRetries, lastErr)
}
```

### Step 4: Deadlines

Set timeout for entire agent run and for each step:

```go
func runAgentWithDeadline(ctx context.Context, client *openai.Client, userInput string) (string, error) {
    // Deadline for entire agent run (5 minutes)
    ctx, cancel := context.WithDeadline(ctx, time.Now().Add(5*time.Minute))
    defer cancel()
    
    // ... agent loop ...
    
    for i := 0; i < maxIterations; i++ {
        // Check deadline before each iteration
        select {
        case <-ctx.Done():
            return "", fmt.Errorf("deadline exceeded")
        default:
        }
        
        // ... execution ...
    }
}
```

### Step 5: State Persistence

Save task state to DB (or file for simplicity):

```go
// Simple file-based implementation
var tasks = make(map[string]*Task)
var tasksMutex sync.RWMutex

func saveTask(task *Task) {
    tasksMutex.Lock()
    defer tasksMutex.Unlock()
    
    task.UpdatedAt = time.Now()
    tasks[task.ID] = task
    
    // Save to file (for simplicity)
    data, _ := json.Marshal(tasks)
    os.WriteFile("tasks.json", data, 0644)
}

func getTask(id string) (*Task, bool) {
    tasksMutex.RLock()
    defer tasksMutex.RUnlock()
    
    task, exists := tasks[id]
    return task, exists
}
```

### Step 6: Resume Execution

Continue task execution after failure:

```go
func resumeTask(taskID string) error {
    task, exists := getTask(taskID)
    if !exists {
        return fmt.Errorf("task not found: %s", taskID)
    }
    
    // If task already completed, do nothing
    if task.State == TaskCompleted {
        return nil
    }
    
    // If task failed, can retry
    if task.State == TaskFailed {
        task.State = TaskPending
        saveTask(task)
    }
    
    // Continue execution
    return executeTask(taskID)
}
```

## Where to Integrate This in Our Code

### Integration Point 1: Agent Loop

In `labs/lab04-autonomy/main.go` add state persistence:

```go
// At start of agent run:
taskID := generateTaskID()
task := &Task{
    ID:        taskID,
    UserInput: userInput,
    State:     TaskRunning,
    CreatedAt: time.Now(),
}
saveTask(task)

// In loop save progress:
task.State = TaskRunning
saveTask(task)

// After completion:
task.State = TaskCompleted
task.Result = finalAnswer
saveTask(task)
```

### Integration Point 2: Tool Execution

In `labs/lab02-tools/main.go` add retry for tools:

```go
func executeToolWithRetry(toolCall openai.ToolCall) (string, error) {
    return executeWithRetry(func() error {
        result, err := executeTool(toolCall)
        if err != nil {
            return err
        }
        return nil
    }, 3)
}
```

## Mini Code Example

Complete example with workflow and state management based on `labs/lab04-autonomy/main.go`:

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "sync"
    "time"

    "github.com/sashabaranov/go-openai"
)

type TaskState string

const (
    TaskPending   TaskState = "pending"
    TaskRunning   TaskState = "running"
    TaskCompleted TaskState = "completed"
    TaskFailed    TaskState = "failed"
)

type Task struct {
    ID        string    `json:"id"`
    UserInput string    `json:"user_input"`
    State     TaskState `json:"state"`
    Result    string    `json:"result,omitempty"`
    Error     string    `json:"error,omitempty"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

var tasks = make(map[string]*Task)
var tasksMutex sync.RWMutex

func generateTaskID() string {
    return fmt.Sprintf("task-%d", time.Now().UnixNano())
}

func saveTask(task *Task) {
    tasksMutex.Lock()
    defer tasksMutex.Unlock()
    
    task.UpdatedAt = time.Now()
    tasks[task.ID] = task
    
    data, _ := json.Marshal(tasks)
    os.WriteFile("tasks.json", data, 0644)
}

func getTask(id string) (*Task, bool) {
    tasksMutex.RLock()
    defer tasksMutex.RUnlock()
    
    task, exists := tasks[id]
    return task, exists
}

func executeWithRetry(fn func() error, maxRetries int) error {
    var lastErr error
    
    for i := 0; i < maxRetries; i++ {
        err := fn()
        if err == nil {
            return nil
        }
        
        lastErr = err
        
        if i < maxRetries-1 {
            backoff := time.Duration(1<<i) * time.Second
            fmt.Printf("Retry %d/%d after %v...\n", i+1, maxRetries, backoff)
            time.Sleep(backoff)
        }
    }
    
    return fmt.Errorf("failed after %d retries: %v", maxRetries, lastErr)
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

    ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Minute))
    defer cancel()

    userInput := "I'm out of disk space. Fix it."

    // Create task
    taskID := generateTaskID()
    task := &Task{
        ID:        taskID,
        UserInput: userInput,
        State:     TaskRunning,
        CreatedAt: time.Now(),
    }
    saveTask(task)

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

    fmt.Printf("Starting Agent Loop (task_id: %s)...\n", taskID)

    for i := 0; i < 5; i++ {
        // Check deadline
        select {
        case <-ctx.Done():
            task.State = TaskFailed
            task.Error = "deadline exceeded"
            saveTask(task)
            fmt.Println("Deadline exceeded")
            return
        default:
        }

        req := openai.ChatCompletionRequest{
            Model:    openai.GPT3Dot5Turbo,
            Messages: messages,
            Tools:    tools,
        }

        resp, err := client.CreateChatCompletion(ctx, req)
        if err != nil {
            task.State = TaskFailed
            task.Error = err.Error()
            saveTask(task)
            panic(fmt.Sprintf("API Error: %v", err))
        }

        msg := resp.Choices[0].Message
        messages = append(messages, msg)

        if len(msg.ToolCalls) == 0 {
            task.State = TaskCompleted
            task.Result = msg.Content
            saveTask(task)
            fmt.Println("AI:", msg.Content)
            break
        }

        for _, toolCall := range msg.ToolCalls {
            fmt.Printf("Executing tool: %s\n", toolCall.Function.Name)

            var result string
            err := executeWithRetry(func() error {
                if toolCall.Function.Name == "check_disk" {
                    result = checkDisk()
                } else if toolCall.Function.Name == "clean_logs" {
                    result = cleanLogs()
                }
                return nil
            }, 3)

            if err != nil {
                task.State = TaskFailed
                task.Error = err.Error()
                saveTask(task)
                fmt.Printf("Tool execution failed: %v\n", err)
                continue
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

### Error 1: No Idempotency

**Symptom:** Repeated call creates duplicates (e.g., creates two files instead of one).

**Cause:** Operations don't check if they were already executed.

**Solution:**
```go
// BAD
func createFile(filename string) error {
    os.WriteFile(filename, []byte("data"), 0644)
    return nil
}

// GOOD
func createFileIfNotExists(filename string) error {
    if _, err := os.Stat(filename); err == nil {
        return nil // Already exists
    }
    return os.WriteFile(filename, []byte("data"), 0644)
}
```

### Error 2: No Retry on Errors

**Symptom:** Agent fails on first temporary error (network error, timeout).

**Cause:** No retries on errors.

**Solution:**
```go
// BAD
result, err := executeTool(toolCall)
if err != nil {
    return "", err // Immediately return error
}

// GOOD
err := executeWithRetry(func() error {
    result, err := executeTool(toolCall)
    return err
}, 3)
```

### Error 3: No Deadlines

**Symptom:** Agent hangs forever, user waits.

**Cause:** No timeout for operations.

**Solution:**
```go
// BAD
resp, _ := client.CreateChatCompletion(ctx, req)
// May hang forever

// GOOD
ctx, cancel := context.WithDeadline(ctx, time.Now().Add(5*time.Minute))
defer cancel()
resp, err := client.CreateChatCompletion(ctx, req)
```

### Error 4: State Not Persisted

**Symptom:** After restart, agent starts from beginning, losing progress.

**Cause:** State stored only in memory.

**Solution:**
```go
// BAD
var taskState = "running" // Only in memory

// GOOD
task.State = TaskRunning
saveTask(task) // Save to DB/file
```

## Mini-Exercises

### Exercise 1: Implement Retry with Backoff

Implement a retry execution function:

```go
func executeWithRetry(fn func() error, maxRetries int) error {
    // Your code here
    // Retry call with exponential backoff
}
```

**Expected result:**
- Function retries call on error
- Uses exponential backoff (1s, 2s, 4s...)
- Function returns error after retries exhausted

### Exercise 2: Implement Idempotency

Create a function that checks if task was already executed:

```go
func executeTaskIfNotDone(taskID string) error {
    // Your code here
    // Check task state before execution
}
```

**Expected result:**
- If task already completed, function returns nil without execution
- If task not completed, function executes it and saves state

## Completion Criteria / Checklist

✅ **Completed (production ready):**
- Operation idempotency implemented (repeated call doesn't create duplicates)
- Retries with exponential backoff implemented
- Deadlines set for agent run and individual operations
- Task state persisted between restarts
- Can resume task execution after failure

❌ **Not completed:**
- No idempotency
- No retry on errors
- No deadlines
- State not persisted

## Connection with Other Chapters

- **[Chapter 04: Autonomy and Loops](../04-autonomy-and-loops/README.md)** — Basic agent loop
- **[Chapter 10: Planning and Workflow Patterns](../10-planning-and-workflows/README.md)** — State Management guarantees reliable plan execution
- **[Chapter 19: Observability and Tracing](../19-observability-and-tracing/README.md)** — Logging task state
- **[Chapter 20: Cost & Latency Engineering](../20-cost-latency-engineering/README.md)** — Cost control for long tasks

## What's Next?

After understanding state management, proceed to:
- **[12. Agent Memory Systems](../12-agent-memory/README.md)** — Learn how agents remember and retrieve information

---

**Navigation:** [← Planning and Workflow Patterns](../10-planning-and-workflows/README.md) | [Table of Contents](../README.md) | [Agent Memory Systems →](../12-agent-memory/README.md)
