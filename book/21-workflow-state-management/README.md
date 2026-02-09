# 21. Workflow and State Management in Production

## Why This Chapter?

**IMPORTANT:** Basic state management concepts (idempotency, retries, deadlines, persistence) are covered in [Chapter 11: State Management](../11-state-management/README.md). This chapter is about **production realities**: queues, asynchrony, scaling, and distributed state.

In production, agents process thousands of tasks in parallel. Without production-ready workflow you cannot:
- Process tasks asynchronously through queues
- Scale task processing horizontally
- Guarantee reliability in distributed systems
- Manage task priorities

### Real-World Case Study

**Situation:** A team launched a DevOps agent. It runs synchronously — one HTTP request, one agent run. On Friday evening, 200 deploy tasks arrive. The agent processes them one by one. By Monday the queue is still backed up, and 30 tasks have timed out.

**Problem:** Synchronous processing does not scale. There is no prioritization (a hotfix waits alongside routine work). When a worker crashes, tasks are lost. Users see no progress and submit duplicates.

**Solution:** An async task queue with a worker pool, SSE for progress updates, and the Saga pattern for multi-step deploys with rollback. The agent scales horizontally, tasks are not lost, and users see progress in real time.

## Theory in Simple Terms

### What Is Workflow in Production?

Workflow is a sequence of steps to complete a task. In production, a workflow outlives a single HTTP request and survives worker restarts. Basic concepts (task state, idempotency, retry, deadlines, persistence) are covered in [Chapter 11](../11-state-management/README.md). Here we look at how those concepts work at scale.

### Production Pattern: AgentState + Artifacts Instead of a "Fat" Context

In production, workflows often outlive a single HTTP request and survive worker restarts. It helps to store **AgentState** as the canonical agent run state (goal, budgets, plan, facts, open questions, risk flags), and keep large tool outputs as **artifacts**:

- the runtime/worker stores raw outputs (logs, JSON, files) in external storage,
- the state stores `artifact_id + summary + bytes`,
- the model receives only a short excerpt (top-k lines) plus `artifact_id`.

This keeps costs predictable and prevents context blow-ups on long runs.

## How It Works (Step by Step)

Basic patterns (task structure, idempotency, retry with backoff, deadlines, state persistence) are covered in [Chapter 11: State Management](../11-state-management/README.md). Here we build on that foundation.

### Step 1: Task Queue

Instead of synchronous processing, use a queue. The client submits a task and immediately gets a `task_id`. A worker picks the task up, executes it, and stores the result.

```
Client → [Task Queue] → Worker → [Result Store] → Client
```

### Step 2: Worker Pool

Run multiple workers. Each pulls tasks from a shared queue. Scale horizontally: more tasks — more workers.

### Step 3: Real-Time Progress

Send updates via SSE or WebSocket. The user sees which step the agent is on and which tool it is calling.

### Step 4: Compensation on Failure

For multi-step operations, use the Saga pattern. If step N fails, roll back steps N-1, ..., 1 in reverse order.

## Async Communication

### Why Synchronous Calls Don't Scale

A synchronous agent call is an HTTP request that blocks until the response arrives. If a task takes 10 minutes, the caller is blocked for 10 minutes. With 100 concurrent tasks you need 100 threads that just sit and wait. This does not scale.

Problems with the synchronous approach:
- The caller is blocked for the entire execution time
- HTTP timeouts kill long-running tasks (typically 30–60 seconds)
- On server restart, all in-flight requests are lost
- No prioritization: urgent tasks wait alongside routine ones

### Pattern: Task Queue

The async approach decouples task submission from result retrieval. The caller puts a task on the queue and immediately gets a `task_id`. A worker picks the task, executes it, and puts the result back.

```
Client → [Task Queue] → Worker → [Result Queue] → Client
```

In Go you can implement a task queue with channels. In production you would use RabbitMQ, Kafka, or Redis Streams — but the principle is the same.

```go
// TaskQueue — a simple channel-based task queue
type TaskQueue struct {
    tasks   chan *Task
    results map[string]chan *Task
    mu      sync.RWMutex
}

func NewTaskQueue(bufferSize int) *TaskQueue {
    return &TaskQueue{
        tasks:   make(chan *Task, bufferSize),
        results: make(map[string]chan *Task),
    }
}

// Submit puts a task on the queue and returns task_id
func (q *TaskQueue) Submit(userInput string) string {
    task := &Task{
        ID:        generateTaskID(),
        UserInput: userInput,
        State:     TaskPending,
        CreatedAt: time.Now(),
    }

    // Result channel for this specific task
    q.mu.Lock()
    q.results[task.ID] = make(chan *Task, 1)
    q.mu.Unlock()

    q.tasks <- task
    return task.ID
}

// Wait blocks until the task result is ready or the timeout fires
func (q *TaskQueue) Wait(taskID string, timeout time.Duration) (*Task, error) {
    q.mu.RLock()
    ch, exists := q.results[taskID]
    q.mu.RUnlock()

    if !exists {
        return nil, fmt.Errorf("task not found: %s", taskID)
    }

    select {
    case result := <-ch:
        return result, nil
    case <-time.After(timeout):
        return nil, fmt.Errorf("timeout waiting for task: %s", taskID)
    }
}

// Worker pulls tasks from the queue and executes them
func (q *TaskQueue) Worker(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case task := <-q.tasks:
            task.State = TaskRunning
            task.UpdatedAt = time.Now()

            // Execute the task (agent loop goes here)
            result, err := doWork(task.UserInput)

            if err != nil {
                task.State = TaskFailed
                task.Error = err.Error()
            } else {
                task.State = TaskCompleted
                task.Result = result
            }
            task.UpdatedAt = time.Now()

            // Send the result back
            q.mu.RLock()
            if ch, ok := q.results[task.ID]; ok {
                ch <- task
            }
            q.mu.RUnlock()
        }
    }
}
```

Usage:

```go
func main() {
    queue := NewTaskQueue(100)

    // Start 3 workers
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    for i := 0; i < 3; i++ {
        go queue.Worker(ctx)
    }

    // Client submits a task and immediately gets the ID
    taskID := queue.Submit("Check disks and clean up logs")
    fmt.Printf("Task accepted: %s\n", taskID)

    // Client waits for the result (or polls later)
    result, err := queue.Wait(taskID, 5*time.Minute)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    fmt.Printf("Result: %s\n", result.Result)
}
```

### Pattern: Event-Driven

In an event-driven architecture the agent does not call services directly. It publishes an event ("disk cleaned"), and other services subscribe to that event and react on their own.

```go
// EventBus — a simple event bus
type Event struct {
    Type    string         // "task.completed", "tool.executed", "agent.error"
    TaskID  string
    Payload map[string]any
    Time    time.Time
}

type EventHandler func(Event)

type EventBus struct {
    handlers map[string][]EventHandler
    mu       sync.RWMutex
}

func NewEventBus() *EventBus {
    return &EventBus{handlers: make(map[string][]EventHandler)}
}

func (b *EventBus) Subscribe(eventType string, handler EventHandler) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.handlers[eventType] = append(b.handlers[eventType], handler)
}

func (b *EventBus) Publish(event Event) {
    b.mu.RLock()
    defer b.mu.RUnlock()

    for _, handler := range b.handlers[event.Type] {
        go handler(event) // Handlers run asynchronously
    }
}
```

Subscription examples:

```go
bus := NewEventBus()

// Monitoring service subscribes to errors
bus.Subscribe("agent.error", func(e Event) {
    log.Printf("[ALERT] Agent %s: error — %v", e.TaskID, e.Payload["error"])
})

// Notification service subscribes to completions
bus.Subscribe("task.completed", func(e Event) {
    notifyUser(e.TaskID, e.Payload["result"].(string))
})

// Agent publishes events as it works
bus.Publish(Event{
    Type:    "task.completed",
    TaskID:  "task-123",
    Payload: map[string]any{"result": "Disk cleaned, freed 20 GB"},
    Time:    time.Now(),
})
```

### Webhooks for Notifications

A webhook is an HTTP call to the client's URL when a task completes. The client registers the URL when submitting the task. The agent sends a POST request with the result.

```go
// WebhookNotifier sends the result to the client's URL
type WebhookNotifier struct {
    client *http.Client
}

func (n *WebhookNotifier) Notify(webhookURL string, task *Task) error {
    payload, err := json.Marshal(task)
    if err != nil {
        return err
    }

    resp, err := n.client.Post(webhookURL, "application/json", bytes.NewReader(payload))
    if err != nil {
        return fmt.Errorf("webhook failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        return fmt.Errorf("webhook returned %d", resp.StatusCode)
    }
    return nil
}
```

### What to Use in Production

| Tool           | When to use                                  |
|----------------|----------------------------------------------|
| Redis Streams  | Simple queues, Redis already in the stack    |
| RabbitMQ       | Classic queues with acknowledgments          |
| Kafka          | High throughput, event log                   |
| NATS           | Lightweight pub/sub, microservices           |

Go channels work well for prototypes and single-process setups. For a distributed system you need an external queue.

## Agent-to-UI (A2UI)

### The Problem: Users Left in the Dark

The user submits a task. The agent works for 30 seconds. The user stares at a spinner with no idea what is happening. Maybe the agent is stuck? Maybe it is almost done? The user hits "Cancel" at second 25 — five seconds before the result.

The fix is to push progress updates in real time.

### Update Structure

Before choosing a transport, define what you send:

```go
// ProgressUpdate — a single progress update for the UI
type ProgressUpdate struct {
    TaskID    string    `json:"task_id"`
    Step      int       `json:"step"`       // Current step number (agent loop iteration)
    TotalStep int       `json:"total_step"` // Expected number of steps (0 = unknown)
    Status    string    `json:"status"`     // "thinking", "tool_call", "tool_result", "completed", "error"
    Tool      string    `json:"tool,omitempty"`    // Which tool is being called
    Message   string    `json:"message"`           // Human-readable description
    Time      time.Time `json:"time"`
}
```

Example updates as the agent runs:

```go
// Iteration 1: agent is thinking
update := ProgressUpdate{
    TaskID: "task-123", Step: 1, Status: "thinking",
    Message: "Analyzing the task...",
}

// Iteration 2: tool call
update = ProgressUpdate{
    TaskID: "task-123", Step: 2, Status: "tool_call",
    Tool: "check_disk", Message: "Checking disks...",
}

// Iteration 2: tool result
update = ProgressUpdate{
    TaskID: "task-123", Step: 2, Status: "tool_result",
    Tool: "check_disk", Message: "Disk /dev/sda1: 95% used",
}

// Iteration 3: done
update = ProgressUpdate{
    TaskID: "task-123", Step: 3, Status: "completed",
    Message: "Cleaned 20 GB of logs, 45% free now",
}
```

### Server-Sent Events (SSE)

SSE is the simplest way to push updates from server to client. It is a regular HTTP request that stays open. The server sends `data: ...` lines as updates arrive.

Advantages of SSE:
- Works over plain HTTP (proxies, load balancers)
- The browser reconnects automatically
- Sufficient for one-way delivery (server → client)

```go
// SSEHandler streams progress updates to the browser
func SSEHandler(queue *TaskQueue) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        taskID := r.URL.Query().Get("task_id")
        if taskID == "" {
            http.Error(w, "task_id required", http.StatusBadRequest)
            return
        }

        // SSE headers
        w.Header().Set("Content-Type", "text/event-stream")
        w.Header().Set("Cache-Control", "no-cache")
        w.Header().Set("Connection", "keep-alive")

        flusher, ok := w.(http.Flusher)
        if !ok {
            http.Error(w, "streaming not supported", http.StatusInternalServerError)
            return
        }

        // Subscribe to updates for this task
        updates := queue.Subscribe(taskID)
        defer queue.Unsubscribe(taskID, updates)

        for {
            select {
            case <-r.Context().Done():
                return // Client disconnected

            case update, ok := <-updates:
                if !ok {
                    return // Channel closed, task finished
                }

                data, _ := json.Marshal(update)
                fmt.Fprintf(w, "data: %s\n\n", data)
                flusher.Flush()

                // Close the stream when the task is done
                if update.Status == "completed" || update.Status == "error" {
                    return
                }
            }
        }
    }
}
```

Client side (JavaScript in the browser):

```javascript
const source = new EventSource("/api/progress?task_id=task-123");

source.onmessage = (event) => {
    const update = JSON.parse(event.data);
    console.log(`[${update.status}] ${update.message}`);

    // Update the UI
    document.getElementById("status").textContent = update.message;
    if (update.total_step > 0) {
        const pct = Math.round((update.step / update.total_step) * 100);
        document.getElementById("progress").style.width = pct + "%";
    }

    if (update.status === "completed" || update.status === "error") {
        source.close();
    }
};
```

### WebSockets

You need a WebSocket when the client sends data during execution. For example, the user wants to clarify a task, cancel it, or change its priority. SSE is server → client only. WebSocket is bidirectional.

When to use WebSocket instead of SSE:
- The user can cancel a task mid-flight
- The user answers agent questions (human-in-the-loop)
- The agent needs confirmation before a dangerous operation

### Publishing Updates from the Agent Loop

For the SSE handler to receive updates, the agent loop must emit them. Add a callback to the agent loop:

```go
// ProgressCallback — a function that sends progress updates
type ProgressCallback func(update ProgressUpdate)

func runAgentLoop(
    ctx context.Context,
    client *openai.Client,
    task *Task,
    onProgress ProgressCallback,
) error {
    messages := []openai.ChatCompletionMessage{
        {Role: openai.ChatMessageRoleSystem, Content: "You are a DevOps agent."},
        {Role: openai.ChatMessageRoleUser, Content: task.UserInput},
    }

    for i := 0; i < 10; i++ {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        // Notify: agent is thinking
        onProgress(ProgressUpdate{
            TaskID: task.ID, Step: i + 1, Status: "thinking",
            Message: fmt.Sprintf("Iteration %d: analyzing...", i+1),
            Time: time.Now(),
        })

        resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
            Model:    "gpt-4o-mini",
            Messages: messages,
            Tools:    tools,
        })
        if err != nil {
            return err
        }

        msg := resp.Choices[0].Message
        messages = append(messages, msg)

        if len(msg.ToolCalls) == 0 {
            // Final answer
            onProgress(ProgressUpdate{
                TaskID: task.ID, Step: i + 1, Status: "completed",
                Message: msg.Content, Time: time.Now(),
            })
            return nil
        }

        // Notify about each tool call
        for _, tc := range msg.ToolCalls {
            onProgress(ProgressUpdate{
                TaskID: task.ID, Step: i + 1, Status: "tool_call",
                Tool: tc.Function.Name,
                Message: fmt.Sprintf("Calling %s...", tc.Function.Name),
                Time: time.Now(),
            })

            result := executeTool(tc)
            messages = append(messages, openai.ChatCompletionMessage{
                Role: openai.ChatMessageRoleTool, Content: result, ToolCallID: tc.ID,
            })

            onProgress(ProgressUpdate{
                TaskID: task.ID, Step: i + 1, Status: "tool_result",
                Tool: tc.Function.Name, Message: result, Time: time.Now(),
            })
        }
    }

    return fmt.Errorf("max iterations reached")
}
```

## Distributed Patterns

### Saga: Multi-Step Operations with Compensation

The agent executes a chain of steps. Each step changes system state. If step N fails, you need to roll back steps N-1, N-2, ..., 1. This is the Saga pattern.

Example: the agent deploys an application.

```
Step 1: Create DNS record       → Compensate: Delete DNS record
Step 2: Issue certificate        → Compensate: Revoke certificate
Step 3: Deploy container         → Compensate: Remove container
Step 4: Configure monitoring     → Compensate: Delete alerts
```

If step 3 fails, you must run compensations for steps 2 and 1 (in reverse order).

```go
// SagaStep — one step of a saga
type SagaStep struct {
    Name       string
    Execute    func(ctx context.Context) error
    Compensate func(ctx context.Context) error // Rollback
}

// Saga — a chain of steps with compensation
type Saga struct {
    Steps     []SagaStep
    completed []int // Indexes of completed steps
}

func NewSaga(steps ...SagaStep) *Saga {
    return &Saga{Steps: steps}
}

// Run executes all steps. On error, rolls back completed ones.
func (s *Saga) Run(ctx context.Context) error {
    for i, step := range s.Steps {
        log.Printf("[saga] Executing step %d: %s", i+1, step.Name)

        if err := step.Execute(ctx); err != nil {
            log.Printf("[saga] Step %d (%s) failed: %v", i+1, step.Name, err)
            log.Printf("[saga] Starting compensation...")

            // Roll back completed steps in reverse order
            s.compensate(ctx)

            return fmt.Errorf("saga failed at step %d (%s): %w", i+1, step.Name, err)
        }

        s.completed = append(s.completed, i)
    }

    log.Printf("[saga] All %d steps completed", len(s.Steps))
    return nil
}

// compensate rolls back completed steps in reverse order
func (s *Saga) compensate(ctx context.Context) {
    for i := len(s.completed) - 1; i >= 0; i-- {
        idx := s.completed[i]
        step := s.Steps[idx]

        log.Printf("[saga] Compensating step %d: %s", idx+1, step.Name)

        if err := step.Compensate(ctx); err != nil {
            // Compensation should not fail, but log it
            log.Printf("[saga] ERROR compensating step %d: %v", idx+1, err)
        }
    }
}
```

Usage:

```go
func deployApp(ctx context.Context, appName string) error {
    saga := NewSaga(
        SagaStep{
            Name: "create_dns",
            Execute: func(ctx context.Context) error {
                return createDNSRecord(appName + ".example.com")
            },
            Compensate: func(ctx context.Context) error {
                return deleteDNSRecord(appName + ".example.com")
            },
        },
        SagaStep{
            Name: "create_cert",
            Execute: func(ctx context.Context) error {
                return issueCertificate(appName + ".example.com")
            },
            Compensate: func(ctx context.Context) error {
                return revokeCertificate(appName + ".example.com")
            },
        },
        SagaStep{
            Name: "deploy_container",
            Execute: func(ctx context.Context) error {
                return deployContainer(appName, "v1.2.3")
            },
            Compensate: func(ctx context.Context) error {
                return removeContainer(appName)
            },
        },
    )

    return saga.Run(ctx)
}
```

### Distributed State

When agents run on different machines, in-memory storage does not work. You need a shared store:

| Store          | When to use                                       |
|----------------|---------------------------------------------------|
| PostgreSQL     | Durable storage, transactions, complex queries    |
| Redis          | Fast access, TTL, simple data structures          |
| etcd           | Coordination, distributed locks, leader election  |

A typical pattern is PostgreSQL for persistent state plus Redis for locks and caching.

```go
// DistributedTaskStore — task store backed by PostgreSQL
type DistributedTaskStore struct {
    db *sql.DB
}

// Claim atomically picks a task from the queue (only one worker gets it)
func (s *DistributedTaskStore) Claim(ctx context.Context, workerID string) (*Task, error) {
    var task Task

    // UPDATE ... RETURNING with locking: only one worker gets the task
    err := s.db.QueryRowContext(ctx, `
        UPDATE tasks
        SET state = 'running', worker_id = $1, updated_at = now()
        WHERE id = (
            SELECT id FROM tasks
            WHERE state = 'pending'
            ORDER BY created_at
            FOR UPDATE SKIP LOCKED
            LIMIT 1
        )
        RETURNING id, user_input, state, created_at, updated_at
    `, workerID).Scan(&task.ID, &task.UserInput, &task.State, &task.CreatedAt, &task.UpdatedAt)

    if err == sql.ErrNoRows {
        return nil, nil // No tasks in the queue
    }

    return &task, err
}
```

`FOR UPDATE SKIP LOCKED` is the key construct. It locks the row for the current worker while other workers skip locked rows. This gives concurrent access without mutual blocking.

### Workflow Engines

For complex production workflows, teams use specialized engines:

- **Temporal** — the most popular choice. You write workflows as regular code (Go, Java, Python). The engine handles retries, state, and timeouts.
- **Cadence** — Temporal's predecessor (from Uber).

Temporal is useful when you have dozens of steps, complex branching logic, and need guaranteed delivery. For 3–5 steps, the Saga from this chapter is enough.

## Common Errors

### Error 1: Running Long Tasks Synchronously

**Symptom:** The HTTP request times out after 30 seconds and the task is not finished. The user retries, creating a duplicate.

**Cause:** The agent loop runs inside the HTTP handler. A long task exceeds the reverse-proxy timeout.

**Solution:**
```go
// BAD: agent loop inside the HTTP handler
func handleTask(w http.ResponseWriter, r *http.Request) {
    result := runAgentLoop(r.Context(), task) // May run for 10 minutes
    json.NewEncoder(w).Encode(result)
}

// GOOD: accept the task, execute asynchronously
func handleTask(w http.ResponseWriter, r *http.Request) {
    taskID := queue.Submit(task)
    w.WriteHeader(http.StatusAccepted)
    json.NewEncoder(w).Encode(map[string]string{"task_id": taskID})
}
```

### Error 2: No Concurrency Protection When Claiming Tasks

**Symptom:** Two workers pick the same task. Result: duplicated work, conflicting writes, corrupted state.

**Cause:** `SELECT ... WHERE state = 'pending'` without locking. Both workers see the same row and both claim it.

**Solution:**
```sql
-- BAD: two workers can grab the same task
SELECT id FROM tasks WHERE state = 'pending' LIMIT 1;
UPDATE tasks SET state = 'running' WHERE id = $1;

-- GOOD: atomic claim with locking
UPDATE tasks
SET state = 'running', worker_id = $1
WHERE id = (
    SELECT id FROM tasks
    WHERE state = 'pending'
    ORDER BY created_at
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
RETURNING id;
```

### Error 3: Compensation in the Wrong Order

**Symptom:** The saga rolls back steps in forward order. The DNS record is deleted before the container — the container is left with a dangling certificate, and users get errors.

**Cause:** Compensation runs in order 1, 2, 3 instead of 3, 2, 1.

**Solution:**
```go
// BAD: rollback in forward order
for i := 0; i < len(completed); i++ {
    steps[completed[i]].Compensate(ctx)
}

// GOOD: rollback in reverse order
for i := len(completed) - 1; i >= 0; i-- {
    steps[completed[i]].Compensate(ctx)
}
```

### Error 4: No Progress Updates for the User

**Symptom:** The user submitted a task. Two minutes pass. The user has no idea whether the agent is working. They hit "Cancel" or submit a duplicate.

**Cause:** No progress updates are sent. The agent works silently.

**Solution:**
```go
// BAD: agent loop with no feedback
for i := 0; i < maxIter; i++ {
    resp, _ := client.CreateChatCompletion(ctx, req)
    // ... processing ...
}

// GOOD: send updates on every step
for i := 0; i < maxIter; i++ {
    onProgress(ProgressUpdate{
        TaskID: task.ID, Step: i + 1, Status: "thinking",
        Message: fmt.Sprintf("Step %d: analyzing...", i+1),
    })
    resp, _ := client.CreateChatCompletion(ctx, req)
    // ... processing with onProgress for tool_call/tool_result ...
}
```

### Error 5: Unbounded Queue

**Symptom:** Under load the service eats all memory and crashes with OOM. Or the queue grows indefinitely and tasks wait for hours.

**Cause:** The channel or queue is created without a buffer or with an excessively large buffer. There is no rejection on overflow.

**Solution:**
```go
// BAD: unbounded queue
tasks := make(chan *Task) // Blocking channel, but no backpressure

// GOOD: bounded buffer + rejection on overflow
tasks := make(chan *Task, 1000)

func (q *TaskQueue) Submit(task *Task) error {
    select {
    case q.tasks <- task:
        return nil
    default:
        return fmt.Errorf("queue full, try later") // HTTP 429
    }
}
```

## Mini-Exercises

### Exercise 1: Implement an SSE Endpoint

Create an HTTP handler that streams agent progress updates via Server-Sent Events:

```go
func SSEProgressHandler(w http.ResponseWriter, r *http.Request) {
    // 1. Get task_id from the query parameter
    // 2. Set Content-Type: text/event-stream headers
    // 3. Subscribe to task updates
    // 4. Stream updates until the task completes
}
```

**Expected result:**
- The client receives updates in real time
- The stream closes when the task finishes
- Proper handling of client disconnect (`r.Context().Done()`)

### Exercise 2: Implement a Deploy Saga

Create a Saga with 3 steps and compensation:

```go
func deploySaga(appName string) *Saga {
    return NewSaga(
        // Step 1: create namespace
        // Step 2: deploy container
        // Step 3: configure routing
        // Each step with compensation
    )
}
```

**Expected result:**
- On success, all 3 steps execute
- On failure at step 3, steps 2 and 1 are rolled back in reverse order
- Every step and every compensation is logged

### Exercise 3: Implement Concurrent Task Claiming

Write a `Claim` function that atomically picks a task from PostgreSQL:

```go
func (s *Store) Claim(ctx context.Context, workerID string) (*Task, error) {
    // Use FOR UPDATE SKIP LOCKED
    // Return nil, nil if there are no tasks
}
```

**Expected result:**
- Two workers never claim the same task
- Tasks are claimed in creation order
- Empty queue is handled correctly

## Completion Criteria / Checklist

**Completed (production ready):**
- [x] Tasks are processed asynchronously via a queue (not inside the HTTP handler)
- [x] The worker pool scales horizontally
- [x] Users see progress in real time (SSE or WebSocket)
- [x] Multi-step operations use the Saga pattern with compensation
- [x] Concurrent task claiming (`FOR UPDATE SKIP LOCKED`)
- [x] The queue has a size limit and backpressure

**Not completed:**
- [ ] Synchronous processing inside the HTTP handler
- [ ] No progress updates for the user
- [ ] Two workers can claim the same task
- [ ] No compensation on multi-step operation failure

## Connection with Other Chapters

- **[Chapter 11: State Management](../11-state-management/README.md)** — Basic concepts: idempotency, retries, deadlines, persistence. This chapter builds on them.
- **[Chapter 04: Autonomy and Loops](../04-autonomy-and-loops/README.md)** — The agent loop that runs inside the worker
- **[Chapter 07: Multi-Agent Systems](../07-multi-agent/README.md)** — Coordinating multiple agents through queues and events
- **[Chapter 19: Observability and Tracing](../19-observability-and-tracing/README.md)** — Logging and tracing distributed tasks
- **[Chapter 20: Cost & Latency Engineering](../20-cost-latency-engineering/README.md)** — Cost control when scaling

## What's Next?

Once you understand production workflow patterns, move on to:
- **[22. Prompt and Program Management](../22-prompt-program-management/README.md)** — Managing prompts and configuration in production