# 11. State Management

## Why This Chapter?

An agent runs a long task (say, a deployment), and then the server reboots. The task is gone. The user waits, but nothing happens. Without state management, you can't:
- Resume execution after failure
- Guarantee idempotency (repeated call doesn't create duplicates)
- Handle errors with retry
- Set deadlines for long tasks

State management is what makes long-lived agents reliable. Without it, tasks that take minutes or hours fall apart.

### Real-World Case Study

**Situation:** Agent deploys an application. The process takes 10 minutes. On the 8th minute, the server reboots.

**Problem:** Task is lost. User doesn't know what happened. On restart, agent starts from the beginning, creating duplicates.

**Solution:** Persist state in a DB, make operations idempotent, add retry with backoff, and enforce deadlines. Now the agent can resume from where it stopped, and repeated calls won't create duplicates.

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

In [`labs/lab04-autonomy/main.go`](https://github.com/kshvakov/ai-agent-course/blob/main/labs/lab04-autonomy/main.go) add state persistence:

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

In [`labs/lab02-tools/main.go`](https://github.com/kshvakov/ai-agent-course/blob/main/labs/lab02-tools/main.go) add retry for tools:

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

Complete example with workflow and state management based on [`labs/lab04-autonomy/main.go`](https://github.com/kshvakov/ai-agent-course/blob/main/labs/lab04-autonomy/main.go):

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
            Model:    "gpt-4o-mini",
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

## Store: Database-Backed Storage

The examples above use a `tasks.json` file to store state. This works for learning, but file storage is unreliable in production.

### Why a File Isn't Production-Ready

File storage has three problems:

1. **No atomicity.** If the process crashes during a write, the file gets corrupted.
2. **No concurrent access.** Two agents can't safely write to the same file.
3. **No queries.** To find all incomplete tasks, you have to read the entire file.

Databases solve all three problems. PostgreSQL is a solid choice for production. SQLite works well for local development.

### StateStore: Storage Interface

Separate the interface from the implementation. This lets you swap the store in tests and change it without rewriting agent logic.

```go
// StateStore defines the contract for agent state storage.
// Implementations can use PostgreSQL, SQLite, or in-memory storage.
type StateStore interface {
    Save(ctx context.Context, task *Task) error
    Get(ctx context.Context, id string) (*Task, error)
    ListByState(ctx context.Context, state TaskState) ([]*Task, error)
}
```

### PostgreSQL Implementation

```go
type PgStateStore struct {
    db *sql.DB
}

func NewPgStateStore(dsn string) (*PgStateStore, error) {
    db, err := sql.Open("pgx", dsn)
    if err != nil {
        return nil, fmt.Errorf("connect to postgres: %w", err)
    }
    return &PgStateStore{db: db}, nil
}

func (s *PgStateStore) Save(ctx context.Context, task *Task) error {
    // UPSERT: insert a new task or update an existing one
    query := `
        INSERT INTO agent_tasks (id, user_input, state, result, error, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, now())
        ON CONFLICT (id) DO UPDATE SET
            state      = EXCLUDED.state,
            result     = EXCLUDED.result,
            error      = EXCLUDED.error,
            updated_at = now()`

    _, err := s.db.ExecContext(ctx, query,
        task.ID, task.UserInput, task.State,
        task.Result, task.Error, task.CreatedAt,
    )
    return err
}

func (s *PgStateStore) Get(ctx context.Context, id string) (*Task, error) {
    task := &Task{}
    err := s.db.QueryRowContext(ctx,
        `SELECT id, user_input, state, result, error, created_at, updated_at
         FROM agent_tasks WHERE id = $1`, id,
    ).Scan(
        &task.ID, &task.UserInput, &task.State,
        &task.Result, &task.Error,
        &task.CreatedAt, &task.UpdatedAt,
    )
    if errors.Is(err, sql.ErrNoRows) {
        return nil, nil
    }
    return task, err
}

func (s *PgStateStore) ListByState(ctx context.Context, state TaskState) ([]*Task, error) {
    rows, err := s.db.QueryContext(ctx,
        `SELECT id, user_input, state, result, error, created_at, updated_at
         FROM agent_tasks WHERE state = $1 ORDER BY created_at`, state,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var tasks []*Task
    for rows.Next() {
        t := &Task{}
        if err := rows.Scan(
            &t.ID, &t.UserInput, &t.State,
            &t.Result, &t.Error,
            &t.CreatedAt, &t.UpdatedAt,
        ); err != nil {
            return nil, err
        }
        tasks = append(tasks, t)
    }
    return tasks, rows.Err()
}
```

### Transactions for Atomic Updates

When the agent executes a step, state must update atomically. If the step fails, the state must remain unchanged.

```go
func (s *PgStateStore) ExecuteStep(ctx context.Context, taskID string, stepFn func(*Task) error) error {
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("begin tx: %w", err)
    }
    defer tx.Rollback()

    // SELECT ... FOR UPDATE locks the row for the duration of the step.
    // Another agent won't be able to modify this task concurrently.
    task := &Task{}
    err = tx.QueryRowContext(ctx,
        `SELECT id, user_input, state, result, error, created_at, updated_at
         FROM agent_tasks WHERE id = $1 FOR UPDATE`, taskID,
    ).Scan(
        &task.ID, &task.UserInput, &task.State,
        &task.Result, &task.Error,
        &task.CreatedAt, &task.UpdatedAt,
    )
    if err != nil {
        return fmt.Errorf("lock task: %w", err)
    }

    // Execute step business logic
    if err := stepFn(task); err != nil {
        return fmt.Errorf("step failed: %w", err)
    }

    // Save updated state inside the transaction
    _, err = tx.ExecContext(ctx,
        `UPDATE agent_tasks SET state=$1, result=$2, error=$3, updated_at=now() WHERE id=$4`,
        task.State, task.Result, task.Error, task.ID,
    )
    if err != nil {
        return fmt.Errorf("save state: %w", err)
    }

    return tx.Commit()
}
```

The step either completes fully or rolls back. There is no "half-done" state.

## MCP for State

Model Context Protocol (MCP) lets you store and share agent state through standardized resources. For more on MCP, see [Chapter 18: Tool Protocols and Tool Servers](../18-tool-protocols-and-servers/README.md).

### Why MCP for State?

An MCP server acts as a single source of truth for state. Any agent or tool accesses it by URI. This solves two problems:

1. **Shared access.** Multiple agents read and update the same state.
2. **Standard protocol.** No need to write a custom API for each store.

### State Resource

Agent state is represented as an MCP resource with a URI:

```
state://agents/{agent_id}/tasks/{task_id}
```

One agent writes progress, another reads it and continues the work.

### Example: Reading Shared State

```go
// MCPStateResource represents agent state as an MCP resource.
type MCPStateResource struct {
    URI       string    `json:"uri"`
    AgentID   string    `json:"agent_id"`
    TaskID    string    `json:"task_id"`
    State     TaskState `json:"state"`
    Plan      []string  `json:"plan,omitempty"`
    Artifacts []string  `json:"artifacts,omitempty"`
}

// readSharedState reads another agent's state via MCP.
// Agent A wrote progress, agent B reads it and continues.
func readSharedState(
    ctx context.Context,
    mcpClient *mcp.Client,
    agentID, taskID string,
) (*MCPStateResource, error) {
    uri := fmt.Sprintf("state://agents/%s/tasks/%s", agentID, taskID)

    resource, err := mcpClient.ReadResource(ctx, uri)
    if err != nil {
        return nil, fmt.Errorf("read MCP resource %s: %w", uri, err)
    }

    var state MCPStateResource
    if err := json.Unmarshal(resource.Content, &state); err != nil {
        return nil, fmt.Errorf("decode state: %w", err)
    }
    return &state, nil
}
```

This approach is useful in [multi-agent systems](../07-multi-agent/README.md), where multiple agents work on the same task.

## Dynamic Context: Selecting Relevant State

### Problem: Not Everything Fits in the Context

The agent accumulates artifacts: logs, command outputs, intermediate data. Over time their volume exceeds the LLM context window. Send everything and the model loses focus. Send nothing and the model can't make decisions.

The solution is to select only the relevant state for the current step.

### Filtering by Relevance

The strategy is simple: first take data from the current step, then fill the remainder with the most recent facts.

```go
// ContextSlice is a slice of state that fits inside the context window.
type ContextSlice struct {
    Goal          string     `json:"goal"`
    CurrentStep   string     `json:"current_step"`
    Facts         []Fact     `json:"facts"`
    Artifacts     []Artifact `json:"artifacts"`
    OpenQuestions []string   `json:"open_questions"`
}

// filterRelevantState selects only what the current step needs
// from the full state.
// maxBytes caps the size to avoid overflowing the context window.
func filterRelevantState(state *AgentState, currentStep string, maxBytes int) *ContextSlice {
    slice := &ContextSlice{
        Goal:          state.Goal,
        CurrentStep:   currentStep,
        OpenQuestions: state.OpenQuestions,
    }

    usedBytes := 0

    // Priority 1: artifacts for the current step
    for _, a := range state.Artifacts {
        if a.Step == currentStep && usedBytes+a.Bytes <= maxBytes {
            slice.Artifacts = append(slice.Artifacts, a)
            usedBytes += a.Bytes
        }
    }

    // Priority 2: most recent facts (fresh data is more likely relevant)
    for i := len(state.KnownFacts) - 1; i >= 0; i-- {
        factSize := len(state.KnownFacts[i].Value)
        if usedBytes+factSize > maxBytes {
            break
        }
        slice.Facts = append(slice.Facts, state.KnownFacts[i])
        usedBytes += factSize
    }

    return slice
}
```

### When You Need This

Filtering becomes critical when the agent runs for more than 5-10 steps. For short tasks you can skip it. For more on context management, see [Chapter 20: Cost & Latency Engineering](../20-cost-latency-engineering/README.md).

## Advanced Checkpoint Strategies

The basic Checkpoint implementation (structure, save/load, agent loop integration) is covered in [Chapter 09: Agent Architecture](../09-agent-architecture/README.md). Here we look at advanced strategies for production.

### When to Save: Checkpoint Granularity

A checkpoint is a snapshot of state you can return to. Save frequency is a trade-off between reliability and performance:

| Strategy | When to save | Pros | Cons |
|----------|-------------|------|------|
| `every_step` | After every tool call | Minimal progress loss | Many DB writes |
| `every_iteration` | After every loop iteration | Balance of reliability and I/O | Lose intermediate steps |
| `on_state_change` | Only on state transition | Minimal I/O | Lose progress within a state |

### CheckpointManager

```go
type CheckpointStrategy string

const (
    CheckpointEveryStep      CheckpointStrategy = "every_step"
    CheckpointEveryIteration CheckpointStrategy = "every_iteration"
    CheckpointOnStateChange  CheckpointStrategy = "on_state_change"
)

type CheckpointManager struct {
    store    StateStore
    strategy CheckpointStrategy
    maxAge   time.Duration // Maximum checkpoint age
    maxCount int           // How many checkpoints to keep per task
}

// MaybeSave saves a checkpoint if the current trigger matches the strategy.
func (cm *CheckpointManager) MaybeSave(
    ctx context.Context,
    task *Task,
    trigger CheckpointStrategy,
) error {
    if trigger != cm.strategy {
        return nil // Not our trigger — skip
    }
    return cm.store.Save(ctx, task)
}
```

### Validation Before Resume

You can't blindly resume a task from a checkpoint. The checkpoint may be stale, and the state may be invalid.

```go
// ValidateAndResume loads a checkpoint and verifies it's usable.
func (cm *CheckpointManager) ValidateAndResume(ctx context.Context, taskID string) (*Task, error) {
    task, err := cm.store.Get(ctx, taskID)
    if err != nil {
        return nil, fmt.Errorf("load checkpoint: %w", err)
    }
    if task == nil {
        return nil, fmt.Errorf("checkpoint not found: %s", taskID)
    }

    // Check 1: checkpoint is not expired
    age := time.Since(task.UpdatedAt)
    if age > cm.maxAge {
        return nil, fmt.Errorf("checkpoint expired: age %v exceeds max %v", age, cm.maxAge)
    }

    // Check 2: state allows resumption
    switch task.State {
    case TaskCompleted:
        return task, nil // Already done, no re-execution needed
    case TaskRunning, TaskFailed:
        return task, nil // Can resume
    default:
        return nil, fmt.Errorf("cannot resume from state: %s", task.State)
    }
}
```

### Checkpoint Rotation

Checkpoints accumulate. Without cleanup they waste storage and complicate recovery. Rotation keeps only the last N checkpoints and deletes expired ones.

```go
// Cleanup removes expired checkpoints, keeping the last maxCount.
func (cm *CheckpointManager) Cleanup(ctx context.Context, taskID string) (int64, error) {
    result, err := cm.store.(*PgStateStore).db.ExecContext(ctx,
        `DELETE FROM agent_checkpoints
         WHERE task_id = $1
           AND created_at < $2
           AND id NOT IN (
               SELECT id FROM agent_checkpoints
               WHERE task_id = $1
               ORDER BY created_at DESC
               LIMIT $3
           )`,
        taskID, time.Now().Add(-cm.maxAge), cm.maxCount,
    )
    if err != nil {
        return 0, fmt.Errorf("cleanup checkpoints: %w", err)
    }
    return result.RowsAffected()
}
```

Good practice: run rotation after every successful save or on a schedule.

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

**Completed (production ready):**
- [x] Operation idempotency implemented (repeated call doesn't create duplicates)
- [x] Retries with exponential backoff implemented
- [x] Deadlines set for agent run and individual operations
- [x] Task state persisted between restarts
- [x] Can resume task execution after failure

**Not completed:**
- [ ] No idempotency
- [ ] No retry on errors
- [ ] No deadlines
- [ ] State not persisted

## Connection with Other Chapters

- **[Chapter 04: Autonomy and Loops](../04-autonomy-and-loops/README.md)** — Basic agent loop
- **[Chapter 10: Planning and Workflow Patterns](../10-planning-and-workflows/README.md)** — State Management guarantees reliable plan execution
- **[Chapter 19: Observability and Tracing](../19-observability-and-tracing/README.md)** — Logging task state
- **[Chapter 20: Cost & Latency Engineering](../20-cost-latency-engineering/README.md)** — Cost control for long tasks

## What's Next?

After understanding state management, proceed to:
- **[12. Agent Memory Systems](../12-agent-memory/README.md)** — Learn how agents remember and retrieve information

