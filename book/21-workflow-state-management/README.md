# 21. Workflow and State Management in Production

## Why does this matter?

The basic concepts of state management (idempotency, retries, deadlines, persistence) are covered in [Chapter 11: State Management](../11-state-management/README.md). This chapter is about **production reality**: queues, asynchrony, scaling, distributed state.

In production, agents process thousands of tasks in parallel. Without a production-ready workflow, you cannot:
- Process tasks asynchronously through queues
- Scale task processing horizontally
- Guarantee reliability in a distributed system
- Manage task priorities

### A real-world case

**Situation:** A team launched a DevOps agent. It works synchronously — one HTTP request, one agent run. On Friday evening, 200 deploy tasks come in. The agent processes them one by one. By Monday, the queue is still clogged, and 30 tasks have timed out.

**Problem:** Synchronous processing does not scale. There is no prioritization (a hotfix waits alongside routine work). When a worker crashes, tasks are lost. Users see no progress and duplicate their requests.

**Solution:** An asynchronous task queue with a pool of workers, SSE for showing progress, and Saga for multi-step deploys with rollback. The agent scales horizontally, tasks are not lost, and users see progress in real time.

## The theory in plain words

### What is a workflow in production?

A workflow is a sequence of steps for completing a task. In production, a workflow "lives" longer than a single HTTP request and survives worker restarts. The basic concepts (task state, idempotency, retries, deadlines, persistence) are covered in [Chapter 11](../11-state-management/README.md). Here we look at how these concepts work at scale.

### Production pattern: AgentState + artifacts instead of a "fat" context

In production, a workflow usually "lives" longer than a single HTTP request and survives worker restarts. So it pays to keep **AgentState** as the canonical state of an agent run (goal, budgets, plan, facts, questions, risk flags), and to store large tool results as **artifacts**:

- the runtime/worker saves the raw result (logs, JSON, files) to external storage,
- it adds a record of `artifact_id + summary + bytes` to the state,
- only a short excerpt (top-k lines) plus `artifact_id` is returned to the LLM context.

This keeps costs down and avoids "killing" the context window, even for long tasks with many steps.

### The FSM of an autonomous run: the mature form

[Chapter 11](../11-state-management/README.md) introduces the finite state machine of an autonomous run. In production it takes its full form:

```
queued → working → (waiting | paused); outcomes of working: asleep (run finished, a new run starts on mention) | done | failed | cancelled
```

The full transition diagram is in [chapter 11](../11-state-management/README.md#the-fsm-of-an-autonomous-run).

- **queued** — the run has been created and is waiting for a worker (a row in the DB queue);
- **working** — the worker is running the agent loop;
- **waiting** — the run needs an external event (human confirmation, result of a long-running operation);
- **paused** — a human has paused the run;
- **asleep** — the run has finished (report sent or skipped); the task "listens" to the stream: only a mention of the task wakes it;
- **done / failed / cancelled** — terminal states; there are no transitions out of them.

Three rules of a mature FSM:

1. **Every transition survives a worker restart.** The state is persisted to the database first, and only then does the worker act. If a worker dies, another picks up the run (or a sweep process restarts a stuck `working` run).
2. **The run deadline (~1 hour) and the iteration limit are terminal boundaries, not "regular errors".** The run stops with status `failed`, and the human gets a message saying the run was cut off by the deadline. Retrying is pointless: the same context will produce the same loop.
3. **Every "sleeping" state has a waker, and it is not a timer:** `waiting` is woken by an incoming event, `paused` by a human, `asleep` by a mention of the task in the stream. A sweep process running on a timer does not wake sleeping states — it restarts a stuck `working` run and delivers undelivered acks. Nothing "waits in memory" — only rows in the database.

For brevity, the rest of this chapter uses a subset of the FSM (`queued/working/done/failed`) — the remaining states plug in following the same "event → transition → persist" scheme.

## How it works (step by step)

The basic patterns (task structure, idempotency, retry with backoff, deadlines, state persistence) are covered in [Chapter 11: State Management](../11-state-management/README.md). Here we build on that foundation.

First, a map of the path. Tasks leave the HTTP handler and go into a **queue** (the client immediately gets a `task_id`), are picked up by a **pool of workers**, progress is delivered to the user via **SSE/WebSocket**, and multi-step operations with side effects are wrapped in a **Saga** with compensations. Each pattern below is covered in detail — let's start with asynchrony.

## Asynchronous Communication

### Why synchronous calls do not scale

A synchronous agent call is an HTTP request that waits for a response. If a task takes 10 minutes, the caller is blocked for 10 minutes. With 100 concurrent tasks you need 100 threads that are simply waiting. This does not scale.

Problems with the synchronous approach:
- The caller is blocked for the entire duration of the task
- HTTP timeouts cut off long tasks (usually 30–60 seconds)
- When the server restarts, all in-flight requests are lost
- No prioritization: urgent tasks wait in the same queue as routine ones

### Pattern: Task queue

The asynchronous approach separates submitting a task from getting the result. The caller puts the task into a queue and immediately receives a `task_id`. A worker picks the task from the queue, executes it, and puts the result back.

```
Client → [Task queue] → Worker → [Result queue] → Client
```

In Go, a task queue can be built with channels. In production, people use RabbitMQ, Kafka, or Redis Streams — but the principle is the same.

```go
// Run states — a subset of the FSM from chapter 11
// (full machine: queued/working/waiting/paused/asleep/done/failed/cancelled)
const (
    TaskQueued  = "queued"
    TaskWorking = "working"
    TaskDone    = "done"
    TaskFailed  = "failed"
)

// TaskQueue — a simple task queue built on channels
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

// Submit puts a task into the queue and returns the task_id
func (q *TaskQueue) Submit(userInput string) string {
    task := &Task{
        ID:        generateTaskID(),
        UserInput: userInput,
        State:     TaskQueued,
        CreatedAt: time.Now(),
    }

    // Channel for the result of this specific task
    q.mu.Lock()
    q.results[task.ID] = make(chan *Task, 1)
    q.mu.Unlock()

    q.tasks <- task
    return task.ID
}

// Wait waits for the task result with a timeout
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

// Worker picks tasks from the queue and executes them
func (q *TaskQueue) Worker(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case task := <-q.tasks:
            task.State = TaskWorking
            task.UpdatedAt = time.Now()

            // Execute the run (the agent loop goes here)
            result, err := doWork(task.UserInput)

            if err != nil {
                task.State = TaskFailed
                task.Error = err.Error()
            } else {
                task.State = TaskDone
                task.Result = result
            }
            task.UpdatedAt = time.Now()

            // Send the result
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

    // The client submits a task and immediately gets an ID
    taskID := queue.Submit("Check the disks and clean up the logs")
    fmt.Printf("Task accepted: %s\n", taskID)

    // The client waits for the result (or checks later)
    result, err := queue.Wait(taskID, 5*time.Minute)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    fmt.Printf("Result: %s\n", result.Result)
}
```

### Pattern: Events (Event-Driven)

In an event-driven architecture, the agent does not call services directly. It publishes an event ("disks cleaned"), and other services subscribe to that event and react on their own.

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

Example subscriptions:

```go
bus := NewEventBus()

// The monitoring service subscribes to errors
bus.Subscribe("agent.error", func(e Event) {
    log.Printf("[alert] Agent %s: error — %v", e.TaskID, e.Payload["error"])
})

// The notification service subscribes to completions
bus.Subscribe("task.completed", func(e Event) {
    notifyUser(e.TaskID, e.Payload["result"].(string))
})

// The agent publishes events as it works
bus.Publish(Event{
    Type:    "task.completed",
    TaskID:  "task-123",
    Payload: map[string]any{"result": "Disks cleaned, 20GB freed"},
    Time:    time.Now(),
})
```

### Webhooks for notifications

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

### What to use in production

| Tool           | When to use                             |
|----------------|---------------------------------------|
| Redis Streams  | Simple queues, Redis is already there |
| RabbitMQ       | Classic queues with acknowledgments   |
| Kafka          | High throughput, event log            |
| NATS           | Lightweight pub/sub, microservices    |

Go channels are fine for a prototype and a single process. A distributed system needs an external queue.

## Idempotent Delivery: Ack After Persist

The central thesis of asynchronous agent reliability: **an incoming event is acknowledged (acked) only after the run's state and memory have been successfully saved**. An event can be a human message, a tool result, a timer, a confirmation. The order of operations is always the same:

1. The worker receives the event from the queue (no ack yet).
2. It applies the event to the run state: an FSM transition, an append to the history.
3. It saves the state and memory to the database — in a single transaction.
4. Only now does it ack the event in the queue.

If the worker dies between steps 1 and 4, the queue returns the event (redelivery), and it gets applied again. That is why step 2 must be idempotent: applying the same event twice does not change the outcome — the run state stores the ID of every processed event.

```go
// handleEvent: persist first, ack second — never the other way around
func handleEvent(ev Event) error {
    turn, err := store.LoadTurn(ev.TurnID)
    if err != nil {
        return err // no ack: the event will be redelivered
    }
    if turn.SeenEvent(ev.ID) {
        return queue.Ack(ev.ID) // a duplicate after redelivery — just ack
    }
    turn.Apply(ev) // FSM transition + append to history

    if err := store.Save(turn); err != nil { // state and memory, one transaction
        return err // no ack
    }
    return queue.Ack(ev.ID) // persist succeeded — now it's safe
}
```

The anti-pattern is acking immediately upon receipt: the worker crashes, the event is lost forever, the run hangs in `waiting`, the human gets no answer and duplicates the request.

### Interleaving: a human reply joins a run in progress

The user writes "stop, changed my mind" while the agent is working. The reply does not wait for the next run: it is **appended to the end of the in-progress run's history**, and the current iteration of the agent loop will see it. The ack follows the same rule: after the history is persisted.

```go
// The worker checks for new messages between agent loop iterations
select {
case msg := <-inbox:
    messages = append(messages, Message{Role: "user", Content: msg.Text})
    if err := store.SaveHistory(turn, messages); err != nil {
        return err // no ack — the message will be redelivered
    }
    if err := inbox.Ack(msg.ID); err != nil {
        return err
    }
default:
}
```

> **How the real SDK does it.** The agent FSM lives on top of a mailbox queue: every incoming event is a message in the queue, and the run processes them one at a time. Interleaving is appending a human reply to the end of the in-progress run's history. The event ack is sent only after the state has been successfully persisted. The prompt version is a run constant, fixed when the run starts (more on this in [chapter 22](../22-prompt-program-management/README.md)).

## Agent-to-UI (A2UI)

### The problem: the user is in the dark

The user submits a task. The agent works for 30 seconds. The user sees a spinner and has no idea what is happening. Is the agent stuck? Is it almost done? The user hits "Cancel" at the 25th second — 5 seconds before the result.

The solution is to send real-time progress updates.

### The structure of updates

Before choosing a transport, decide what to send:

```go
// ProgressUpdate — a single progress update for the UI
type ProgressUpdate struct {
    TaskID    string    `json:"task_id"`
    Step      int       `json:"step"`       // Current step number (agent loop iteration)
    TotalStep int       `json:"total_step"` // Expected number of steps (0 — unknown)
    Status    string    `json:"status"`     // "thinking", "tool_call", "tool_result", "completed", "error"
    Tool      string    `json:"tool,omitempty"`    // Which tool is being called
    Message   string    `json:"message"`           // Human-readable description
    Time      time.Time `json:"time"`
}
```

Examples of updates as the agent works:
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
    Tool: "check_disk", Message: "Disk /dev/sda1: 95% full",
}

// Iteration 3: done
update = ProgressUpdate{
    TaskID: "task-123", Step: 3, Status: "completed",
    Message: "Cleaned 20GB of logs, 45% free",
}
```

### Server-sent events (SSE)

SSE is the simplest way to push updates from server to client. It's an ordinary HTTP request that never closes. The server sends `data: ...` lines as updates arrive.

Advantages of SSE:
- Works over plain HTTP (proxying, load balancing)
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

        // Subscribe to task updates
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

                // If the task is finished, close the stream
                if update.Status == "completed" || update.Status == "error" {
                    return
                }
            }
        }
    }
}
```

On the client side (JavaScript in the browser):

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

You need a WebSocket when the client sends data while the task is running. For example, the user wants to clarify the task, cancel it, or change its priority. SSE is server → client only. WebSocket is bidirectional.

When to use a WebSocket instead of SSE:
- The user can cancel a task on the fly
- The user answers the agent's questions (human-in-the-loop)
- The agent needs confirmation before a dangerous operation

### Publishing updates from the agent loop

For the SSE handler to receive updates, the agent loop must send them. Add a callback to the agent loop:

```go
// ProgressCallback — a function for sending updates
type ProgressCallback func(update ProgressUpdate)

// A terminal run boundary — NOT a transient error, do not retry
var ErrLoopLimit = errors.New("agent loop: iteration limit reached")

func runAgentLoop(
    ctx context.Context,
    client Client, // the neutral client from ch. 04: Complete(ctx, Request) (*Response, error)
    task *Task,
    onProgress ProgressCallback,
) error {
    const maxIterations = 25 // iteration limit of the run

    messages := []Message{
        {Role: "system", Content: task.SystemPrompt}, // a constant of the run (ch. 22)
        {Role: "user", Content: task.UserInput},
    }
    tools := task.Tools // the run's []ToolDef (ch. 04)

    for i := 0; i < maxIterations; i++ {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        // Notify: the agent is thinking
        onProgress(ProgressUpdate{
            TaskID: task.ID, Step: i + 1, Status: "thinking",
            Message: fmt.Sprintf("Iteration %d: analyzing...", i+1),
            Time: time.Now(),
        })

        resp, err := client.Complete(ctx, Request{Messages: messages, Tools: tools})
        if err != nil {
            return err // for error classification, see below
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

        // Notify about every tool call
        for _, tc := range msg.ToolCalls {
            onProgress(ProgressUpdate{
                TaskID: task.ID, Step: i + 1, Status: "tool_call",
                Tool: tc.Name,
                Message: fmt.Sprintf("Calling %s...", tc.Name),
                Time: time.Now(),
            })

            result := executeTool(tc)
            messages = append(messages, Message{
                Role: "tool", Content: result, ToolCallID: tc.ID,
            })

            onProgress(ProgressUpdate{
                TaskID: task.ID, Step: i + 1, Status: "tool_result",
                Tool: tc.Name, Message: result, Time: time.Now(),
            })
        }
    }

    // The iteration limit is a terminal boundary: the run goes to failed,
    // with a message to the human. This is not transient: a retry with the
    // same context will reproduce the same loop.
    return ErrLoopLimit
}
```

**Run error classification** — this is what separates a mature system from "retry everything":

- **Transient** (network, LLM timeout, provider 429/5xx) → retry: the sweep restarts the run or repeats the iteration with backoff.
- **Run boundaries** (iteration limit `ErrLoopLimit`, a ~1h run deadline) → terminal: the run goes to `failed`, with a "run interrupted" message to the human. Retrying is pointless — the same context will reproduce the same loop. Diagnostics: look at the trace ([chapter 19](../19-observability-and-tracing/README.md)), fix the cause (prompt, tools), start a new run.

If you return `ErrLoopLimit` as an "ordinary" error, an outer retry mechanism will keep restarting the stuck agent forever — and burn your token budget ([chapter 20](../20-cost-latency-engineering/README.md)).

## Distributed Patterns

### Saga: multi-step operations with compensation

The agent executes a chain of steps. Each step changes the state of the system. If step N fails, steps N-1, N-2, ..., 1 must be rolled back. This is the Saga pattern.

Example: an agent deploys an application.

```
Step 1: Create a DNS record      → Compensation: Delete the DNS record
Step 2: Issue a certificate      → Compensation: Revoke the certificate
Step 3: Deploy the container     → Compensation: Remove the container
Step 4: Configure monitoring     → Compensation: Remove the alerts
```

If step 3 fails, the compensations for steps 2 and 1 must run (in reverse order).

```go
// SagaStep — one step of the saga
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

// Run executes all steps. On error it rolls back the completed ones.
func (s *Saga) Run(ctx context.Context) error {
    for i, step := range s.Steps {
        log.Printf("[saga] Executing step %d: %s", i+1, step.Name)

        if err := step.Execute(ctx); err != nil {
            log.Printf("[saga] Step %d (%s) failed: %v", i+1, step.Name, err)
            log.Printf("[saga] Starting compensation...")

            // Roll back the completed steps in reverse order
            s.compensate(ctx)

            return fmt.Errorf("saga failed at step %d (%s): %w", i+1, step.Name, err)
        }

        s.completed = append(s.completed, i)
    }

    log.Printf("[saga] All %d steps completed", len(s.Steps))
    return nil
}

// compensate rolls back the completed steps in reverse order
func (s *Saga) compensate(ctx context.Context) {
    for i := len(s.completed) - 1; i >= 0; i-- {
        idx := s.completed[i]
        step := s.Steps[idx]

        log.Printf("[saga] Compensating step %d: %s", idx+1, step.Name)

        if err := step.Compensate(ctx); err != nil {
            // Compensation must not fail, but we log it anyway
            log.Printf("[saga] compensation error for step %d: %v", idx+1, err)
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

### Distributed state

When agents run on different machines, in-memory storage no longer works. State has to live in a shared store:

| Store          | When to use                                       |
|----------------|--------------------------------------------------|
| PostgreSQL     | Reliable storage, transactions, complex queries  |
| Redis          | Fast access, TTLs, simple structures             |
| etcd           | Coordination, distributed locks, leader election |

The typical pattern: PostgreSQL for persistent state + Redis for locks and cache.

```go
// DistributedTaskStore — task store backed by PostgreSQL
type DistributedTaskStore struct {
    db *sql.DB
}

// Claim atomically takes a task from the queue (only one worker will get it)
func (s *DistributedTaskStore) Claim(ctx context.Context, workerID string) (*Task, error) {
    var task Task

    // UPDATE ... RETURNING with locking: only one worker will get the task
    err := s.db.QueryRowContext(ctx, `
        UPDATE tasks
        SET state = 'working', worker_id = $1, updated_at = now()
        WHERE id = (
            SELECT id FROM tasks
            WHERE state = 'queued'
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

`FOR UPDATE SKIP LOCKED` is the key construct. It locks the row for the current worker while the other workers skip locked rows. This gives you concurrent access without deadlocks.

### Workflow engines

For complex workflows in production, specialized engines are used:

- **Temporal** — the most popular one. A workflow is described as ordinary code (Go, Java, Python). The engine handles retries, state, and timeouts.
- **Cadence** — Temporal's predecessor (from Uber).

An honest assessment: for most production bots, the combination of "DB queue + run FSM + sweep retries" from this chapter and [chapter 11](../11-state-management/README.md) is enough — fewer moving parts, and everything is observable with a single SQL query. Temporal is justified when complex branching sagas appear and multi-day processes (customer onboarding, migrations), where manually managing timers, retries, and compensations already hurts. For 3–5 steps, Saga is enough.

## Common Mistakes

### Mistake 1: Processing long tasks synchronously

**Symptom:** the HTTP request times out after 30 seconds, the task is not finished. The user repeats the request and gets a duplicate.

**Cause:** the agent loop runs inside the HTTP handler. A long task doesn't fit within the reverse proxy's timeout.

**Solution:**
```go
// BAD: agent loop inside an HTTP handler
func handleTask(w http.ResponseWriter, r *http.Request) {
    result := runAgentLoop(r.Context(), task) // Can run for 10 minutes
    json.NewEncoder(w).Encode(result)
}

// GOOD: accept the task, execute asynchronously
func handleTask(w http.ResponseWriter, r *http.Request) {
    taskID := queue.Submit(task)
    w.WriteHeader(http.StatusAccepted)
    json.NewEncoder(w).Encode(map[string]string{"task_id": taskID})
}
```

### Mistake 2: No concurrency protection when claiming tasks

**Symptom:** Two workers take the same task. Result: duplicated work, conflicts when writing the result, corrupted state.

**Cause:** `SELECT ... WHERE state = 'queued'` without locking. Both workers see the same row and both claim it.

**Fix:**
```sql
-- BAD: two workers can claim the same task
SELECT id FROM tasks WHERE state = 'queued' LIMIT 1;
UPDATE tasks SET state = 'working' WHERE id = $1;

-- GOOD: atomic claim with locking
UPDATE tasks
SET state = 'working', worker_id = $1
WHERE id = (
    SELECT id FROM tasks
    WHERE state = 'queued'
    ORDER BY created_at
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
RETURNING id;
```

### Mistake 3: Compensation in the wrong order

**Symptom:** The Saga rolls back steps in forward order. The DNS record is deleted before the container — the container is left with a dangling certificate and users get errors.

**Cause:** Compensation runs in the order 1, 2, 3 instead of 3, 2, 1.

**Fix:**
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

### Mistake 4: No progress for the user

**Symptom:** The user submitted a task. Two minutes pass. The user has no idea whether the agent is running. They hit "Cancel" or duplicate the request.

**Cause:** No progress updates are sent. The agent works silently.

**Fix:**
```go
// BAD: agent loop without feedback
for i := 0; i < maxIter; i++ {
    resp, _ := llm.Complete(ctx, Request{Messages: messages, Tools: tools})
    // ... processing ...
}

// GOOD: send an update on every step
for i := 0; i < maxIter; i++ {
    onProgress(ProgressUpdate{
        TaskID: task.ID, Step: i + 1, Status: "thinking",
        Message: fmt.Sprintf("Step %d: analyzing...", i+1),
    })
    resp, err := llm.Complete(ctx, Request{Messages: messages, Tools: tools})
    if err != nil {
        return err // progress reporting does not replace error handling (ch. 04)
    }
    // ... processing with onProgress for tool_call/tool_result ...
}
```

### Mistake 5: Unbounded queue

**Symptom:** Under load the service eats all memory and dies with OOM. Or the queue grows without limit and tasks wait for hours.

**Cause:** A channel or queue created without a buffer, or with a huge one. No rejection on overflow.

**Fix:**
```go
// BAD: unlimited queue
tasks := make(chan *Task) // Blocking channel, but no backpressure

// GOOD: bounded buffer + reject on overflow
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

## Exercises

### Exercise 1: Implement an SSE endpoint

Create an HTTP handler that streams agent progress updates via Server-Sent Events:

```go
func SSEProgressHandler(w http.ResponseWriter, r *http.Request) {
    // 1. Get task_id from the query parameter
    // 2. Set the Content-Type: text/event-stream headers
    // 3. Subscribe to the task's updates
    // 4. Stream updates until the task completes
}
```

**Expected result:**
- The client receives updates in real time
- The stream closes when the task completes
- Client disconnects are handled correctly (`r.Context().Done()`)

### Exercise 2: Implement a deploy saga

Create a 3-step Saga with compensation:

```go
func deploySaga(appName string) *Saga {
    return NewSaga(
        // Step 1: create the namespace
        // Step 2: deploy the container
        // Step 3: configure routing
        // Every step has a compensation
    )
}
```

**Expected result:**
- On success, all 3 steps run
- On failure at step 3, steps 2 and 1 roll back in reverse order
- Every step and every compensation is logged

### Exercise 3: Implement concurrent task claiming

Write a `Claim` function that atomically claims a task from PostgreSQL:

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

## Acceptance Criteria / Checklist

**Passing (production-ready):**
- [x] Tasks are processed asynchronously via a queue (not in the HTTP handler)
- [x] The worker pool scales horizontally
- [x] The user sees real-time progress (SSE or WebSocket)
- [x] Multi-step operations use a Saga with compensation
- [x] Concurrent task claiming (`FOR UPDATE SKIP LOCKED`)
- [x] The queue has a size limit and backpressure
- [x] Incoming events are acked only after state is persisted (idempotent delivery)
- [x] Run deadline and iteration limit are terminal boundaries, not retries

**Failing:**
- [ ] Synchronous processing in the HTTP handler
- [ ] No progress for the user
- [ ] Two workers can claim the same task
- [ ] No compensation on multi-step operation failure

## Relation to Other Chapters

- **[Chapter 11: State Management](../11-state-management/README.md)** — Core concepts: idempotency, retries, deadlines, persistence. This chapter builds on them.
- **[Chapter 04: Autonomy and Loops](../04-autonomy-and-loops/README.md)** — The agent loop executed inside the worker
- **[Chapter 07: Multi-Agent Systems](../07-multi-agent/README.md)** — Coordinating multiple agents via queues and events
- **[Chapter 19: Observability and Tracing](../19-observability-and-tracing/README.md)** — Logging and tracing of distributed tasks
- **[Chapter 20: Cost & Latency Engineering](../20-cost-latency-engineering/README.md)** — Cost control when scaling

## What's Next?

Once you understand the production workflow patterns, move on to:
- **[22. Prompt and Program Management](../22-prompt-program-management/README.md)** — Managing prompts and configuration in production

