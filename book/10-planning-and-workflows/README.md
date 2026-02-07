# 10. Planning and Workflow Patterns

## Why This Chapter?

Simple ReAct loops work well for straightforward tasks. Once the task becomes multi-step, you usually need planning: break the work into steps, respect dependencies, handle failures, and keep track of progress.

This chapter covers planning patterns that help agents handle complex, long-running work without getting lost.

### Real-World Case Study

**Situation:** User asks: "Deploy new microservice: create VM, install dependencies, configure network, deploy application, configure monitoring."

**Problem:** A simple ReAct loop may:
- Jump between steps randomly
- Skip dependencies (try to deploy before creating VM)
- Not track which steps are completed
- Fail and start from scratch

**Solution:** Use a planning pattern: first create a plan (steps + dependencies), then execute it while tracking state and handling failures.

## Theory in Simple Terms

### What Is Planning?

Planning is the process of breaking down a complex task into smaller, manageable steps with clear dependencies and execution order.

**Key components:**
1. **Task decomposition** — Break down large tasks into steps
2. **Dependency graph** — Understand which steps depend on others
3. **Execution order** — Determine the sequence (or parallel execution)
4. **State tracking** — Know what's done, what's in progress, and what failed
5. **Failure handling** — Retry, skip, or abort on errors

### Planning Patterns

**Pattern 1: Plan→Execute**
- Agent creates complete plan upfront
- Executes steps sequentially
- Simple, but inflexible

**Pattern 2: Plan-and-Revise**
- Agent creates initial plan
- Revises plan as it learns (e.g., step failed, new information discovered)
- More adaptive, but more complex

**Pattern 3: DAG/Workflow**
- Steps form a directed acyclic graph
- Some steps can execute in parallel
- Handles complex dependencies

## How It Works (Step by Step)

### Step 1: Task Decomposition

Agent receives a high-level task and breaks it into steps:

```go
type Plan struct {
    Steps []Step
}

type Step struct {
    ID          string
    Description string
    Dependencies []string  // IDs of steps that must complete first
    Status      StepStatus
    Result      any
    Error       error
}

type StepStatus string

const (
    StepStatusPending   StepStatus = "pending"
    StepStatusRunning   StepStatus = "running"
    StepStatusCompleted StepStatus = "completed"
    StepStatusFailed    StepStatus = "failed"
    StepStatusSkipped   StepStatus = "skipped"
)
```

**Example:** "Deploy microservice" is broken into:
1. Create VM (no dependencies)
2. Install dependencies (depends on: Create VM)
3. Configure network (depends on: Create VM)
4. Deploy application (depends on: Install dependencies, Configure network)
5. Configure monitoring (depends on: Deploy application)

### Step 2: Create Plan

Agent uses LLM for task decomposition:

```go
func createPlan(ctx context.Context, client *openai.Client, task string) (*Plan, error) {
    prompt := fmt.Sprintf(`Break this task into steps with dependencies:
Task: %s

Return JSON with array of steps. Each step has: id, description, dependencies (array of step IDs).

Example:
{
  "steps": [
    {"id": "step1", "description": "Create VM", "dependencies": []},
    {"id": "step2", "description": "Install dependencies", "dependencies": ["step1"]}
  ]
}`, task)

    messages := []openai.ChatCompletionMessage{
        {Role: "system", Content: "You are a planning agent. Break tasks into steps."},
        {Role: "user", Content: prompt},
    }

    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model:    openai.GPT3Dot5Turbo,
        Messages: messages,
        Temperature: 0, // Deterministic planning
    })
    if err != nil {
        return nil, err
    }

    // Parse JSON response into Plan
    var plan Plan
    json.Unmarshal([]byte(resp.Choices[0].Message.Content), &plan)
    return &plan, nil
}
```

### Step 3: Execute Plan

Execute steps considering dependencies:

```go
func executePlan(ctx context.Context, plan *Plan, executor StepExecutor) error {
    for {
        // Find steps ready to execute (all dependencies completed)
        readySteps := findReadySteps(plan)
        
        if len(readySteps) == 0 {
            // Check if all completed or stuck
            if allStepsCompleted(plan) {
                return nil
            }
            if allRemainingStepsBlocked(plan) {
                return fmt.Errorf("plan blocked: some steps failed")
            }
            // Wait for async steps or retry failed steps
            continue
        }

        // Execute ready steps (can be parallel)
        for _, step := range readySteps {
            step.Status = StepStatusRunning
            result, err := executor.Execute(ctx, step)
            
            if err != nil {
                step.Status = StepStatusFailed
                step.Error = err
                // Decide: retry, skip, or abort
                if shouldRetry(step) {
                    step.Status = StepStatusPending
                    continue
                }
            } else {
                step.Status = StepStatusCompleted
                step.Result = result
            }
        }
    }
}

func findReadySteps(plan *Plan) []*Step {
    ready := make([]*Step, 0, len(plan.Steps))
    for i := range plan.Steps {
        step := &plan.Steps[i]
        if step.Status != StepStatusPending {
            continue
        }
        
        // Check if all dependencies are completed
        allDepsDone := true
        for _, depID := range step.Dependencies {
            dep := findStep(plan, depID)
            if dep == nil || dep.Status != StepStatusCompleted {
                allDepsDone = false
                break
            }
        }
        
        if allDepsDone {
            ready = append(ready, step)
        }
    }
    return ready
}
```

### Step 4: Failure Handling

Implement retry logic with exponential backoff:

```go
type StepExecutor interface {
    Execute(ctx context.Context, step *Step) (any, error)
}

func executeWithRetry(ctx context.Context, executor StepExecutor, step *Step, maxRetries int) (any, error) {
    var lastErr error
    backoff := time.Second
    
    for attempt := 0; attempt <= maxRetries; attempt++ {
        if attempt > 0 {
            // Exponential backoff
            time.Sleep(backoff)
            backoff *= 2
        }
        
        result, err := executor.Execute(ctx, step)
        if err == nil {
            return result, nil
        }
        
        lastErr = err
        // Check if error is retryable
        if !isRetryableError(err) {
            return nil, err
        }
    }
    
    return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}
```

### Step 5: Plan State Persistence

**IMPORTANT:** State persistence for resuming execution is described in [State Management](../11-state-management/README.md). Here, only the plan state structure is described.

```go
// Plan state is used to track progress
// Persistence and resumption described in State Management
type PlanState struct {
    PlanID    string
    Steps     []Step
    UpdatedAt time.Time
}
```

## Common Errors

### Error 1: No Dependency Tracking

**Symptom:** Agent tries to execute steps out of order, causing failures.

**Cause:** Dependencies between steps are not tracked.

**Solution:**
```go
// BAD: Execute steps in order without checking dependencies
for _, step := range plan.Steps {
    executor.Execute(ctx, step)
}

// GOOD: Check dependencies first
readySteps := findReadySteps(plan)
for _, step := range readySteps {
    executor.Execute(ctx, step)
}
```

### Error 2: No State Persistence

**Symptom:** Agent starts from scratch after failure, losing progress.

**Cause:** Plan state is not persisted.

**Solution:** Use techniques from [State Management](../11-state-management/README.md) to persist and resume plan execution.

### Error 3: Infinite Retries

**Symptom:** Agent retries failed step forever, wasting resources.

**Cause:** No retry limits or backoff.

**Solution:** Implement maximum retry count and exponential backoff.

### Error 4: No Parallel Execution

**Symptom:** Agent executes independent steps sequentially, wasting time.

**Cause:** Steps that can execute in parallel are not identified.

**Solution:** Use `findReadySteps` to get all ready steps, execute them concurrently:

```go
// Execute ready steps in parallel
var wg sync.WaitGroup
for _, step := range readySteps {
    wg.Add(1)
    go func(s *Step) {
        defer wg.Done()
        executor.Execute(ctx, s)
    }(step)
}
wg.Wait()
```

## Pattern: Controller + Processor (orchestrator + normalizer)

When a workflow grows, it's useful to separate two concerns:

- **Controller (orchestrator)** selects the next step: call a tool or respond to the user.
- **Processor (analyzer/normalizer)** turns tool results and user answers into a structured state update (for example: "append facts", "replace plan", "add open questions").

This reduces noise in the agent loop. The controller does not get buried in large outputs. The processor does not decide on side effects.

Mini-trace (read-only search + file read):

1) Controller calls search.

```json
{
  "tool_call": {
    "name": "search_code",
    "arguments": { "query": "type ClientError struct" }
  }
}
```

2) ToolRunner stores the raw output as an artifact and returns a short payload (top-k matches).

3) Processor returns a `state_patch`:

```json
{
  "replace_plan": [
    "Read the file with the best match",
    "Write a short explanation for the user"
  ],
  "append_known_facts": [
    {
      "key": "client_error_candidate",
      "value": "pkg/errors/client_error.go:12",
      "source": "tool",
      "artifact_id": "srch_123",
      "confidence": 0.9
    }
  ]
}
```

4) Controller reads the file and produces the final answer.

## Mini-Exercises

### Exercise 1: Task Decomposition

Implement a function that breaks a task into steps:

```go
func decomposeTask(task string) (*Plan, error) {
    // Use LLM to create plan
    // Return Plan with steps and dependencies
}
```

**Expected result:**
- Plan contains logical steps
- Dependencies correctly defined
- Steps can execute in valid order

### Exercise 2: Dependency Resolution

Implement `findReadySteps` that returns steps whose all dependencies are completed:

```go
func findReadySteps(plan *Plan) []*Step {
    // Your implementation
}
```

**Expected result:**
- Returns only steps with all satisfied dependencies
- Handles cyclic dependencies (detects and errors)

### Exercise 3: Plan Execution with Retries

Implement plan execution with retry logic:

```go
func executePlanWithRetries(ctx context.Context, plan *Plan, executor StepExecutor, maxRetries int) error {
    // Execute plan with retry logic
    // Handle failures correctly
}
```

**Expected result:**
- Steps execute considering dependencies
- Failed steps retry up to maxRetries
- Plan completes or fails correctly

## Completion Criteria / Checklist

**Completed:**
- [x] Can break complex tasks into steps
- [x] Understand dependency graphs
- [x] Can execute plans considering dependencies
- [x] Handle failures with retries
- [x] Persist plan state for resumption

**Not completed:**
- [ ] Step execution without dependency checks
- [ ] No state persistence
- [ ] Infinite retries without limits
- [ ] Sequential execution when parallel is possible

## Connection with Other Chapters

- **[Chapter 04: Autonomy and Loops](../04-autonomy-and-loops/README.md)** — Planning extends ReAct loop for complex tasks
- **[Chapter 07: Multi-Agent Systems](../07-multi-agent/README.md)** — Planning can coordinate multiple agents
- **[Chapter 11: State Management](../11-state-management/README.md)** — Reliable plan execution (idempotency, retries, persist)
- **[Chapter 21: Workflow and State Management in Production](../21-workflow-state-management/README.md)** — Production workflow patterns

**IMPORTANT:** Planning focuses on **task decomposition** and **dependency graphs**. Execution reliability (persist, retries, deadlines) is described in [State Management](../11-state-management/README.md).

## What's Next?

After mastering planning patterns, proceed to:
- **[11. State Management](../11-state-management/README.md)** — Learn how to guarantee reliable plan execution

