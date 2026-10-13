# 10. Planning and Workflow Patterns

## Why does this matter?

Simple ReAct loops work fine for straightforward tasks. But as soon as a task becomes multi-step, you usually need planning: break the work into steps, account for dependencies, survive failures without losing progress.

In this chapter we'll look at planning patterns that help agents handle complex and long-running tasks.

### A real-world case

**Situation:** The user asks: "Deploy a new microservice: create a VM, install dependencies, set up networking, deploy the app, configure monitoring."

**Problem:** A simple ReAct loop may:
- Jump between steps at random
- Skip dependencies (try to deploy before creating the VM)
- Fail to track which steps are complete
- Crash and start over from scratch

**Solution:** The planning pattern: the agent first builds a plan (steps + dependencies), then executes it in order, tracking state and handling failures properly.

## The theory in plain words

### What is planning?

Planning is the process of breaking a complex task into smaller, manageable steps with clear dependencies and an execution order.

**Key components:**
1. **Task decomposition** — Break a large task into steps
2. **Dependency graph** — Understand which steps depend on others
3. **Execution order** — Determine the sequence (or parallel execution)
4. **State tracking** — Know what's done, what's in progress, what failed
5. **Failure handling** — Retry, skip, or abort on errors

### Planning patterns

**Pattern 1: Plan→Execute**
- The agent builds a full plan upfront
- Executes the steps sequentially
- Simple, but inflexible

**Pattern 2: Plan-and-Revise**
- The agent builds an initial plan
- Revises the plan as it learns (e.g., a step failed, new information was discovered)
- More adaptive, but more complex

**Pattern 3: DAG/Workflow**
- Steps form a directed acyclic graph
- Some steps can run in parallel
- Handles complex dependencies
- This is an option for batch pipelines (CI, ETL, mass processing), not the default for an interactive agent

## How it works (step by step)

### Step 1: Task decomposition

The agent receives a high-level task and breaks it into steps:

```go
type Plan struct {
    Steps []Step
}

type Step struct {
    ID          string
    Description string
    Dependencies []string  // IDs of steps that must finish first
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

**Example:** "Deploy a microservice" breaks down into:
1. Create a VM (no dependencies)
2. Install dependencies (depends on: Create a VM)
3. Set up networking (depends on: Create a VM)
4. Deploy the app (depends on: Install dependencies, Set up networking)
5. Configure monitoring (depends on: Deploy the app)

### Step 2: Create the plan

The agent uses the LLM to decompose the task — **in a single structured-output request**. The JSON Schema is generated from the Go `Plan` struct, and broken JSON from the model is fixed by a repair pass:

```go
func createPlan(ctx context.Context, llm *llm.Client, task string) (*Plan, error) {
    return llm.CompleteStructured(ctx, llm.StructuredRequest{
        System: "You are a planning agent. Break tasks into steps with dependencies.",
        User:   "Task: " + task,
        // The response schema is generated from the Go Plan struct (json tags):
        // steps[{id, description, dependencies}]
        Into:        new(Plan),
        Temperature: 0.2, // decomposition quality; "0 for determinism" is a myth
        MaxRepair:   2,   // up to two repair attempts for broken JSON
    })
}
```

What matters here:

- **One request → a typed object.** No manual `json.Unmarshal` of the raw response text: without repairing broken JSON (unclosed quotes, a ```json wrapper) parsing fails for no good reason.
- **Schema from the Go struct.** Change `Plan` and the schema updates itself; there is no separate JSON contract that can drift out of sync.
- **Temperature 0.2.** A low temperature is about the quality of repeatable decomposition, not about "determinism": an LLM is nondeterministic by nature.

### Step 3: Execute the plan

We execute the steps while respecting dependencies:

```go
func executePlan(ctx context.Context, plan *Plan, executor StepExecutor) error {
    for {
        // Find steps ready to run (all dependencies completed)
        readySteps := findReadySteps(plan)
        
        if len(readySteps) == 0 {
            // Check whether everything is done or stuck
            if allStepsCompleted(plan) {
                return nil
            }
            if allRemainingStepsBlocked(plan) {
                return fmt.Errorf("plan blocked: some steps failed")
            }
            // Wait for async steps or retry failed ones
            continue
        }

        // Execute ready steps (they may be parallelizable)
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
        
        // Check whether all dependencies are completed
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

### Step 4: Failure handling

Transient errors (network, API limits) are not the planner's concern: the LLM client retries them itself, with exponential backoff and jitter (see [Chapter 04](../04-autonomy-and-loops/README.md)). At the plan level, what remains is a policy for a step that failed **after** all client-side retries:

```go
type StepExecutor interface {
    Execute(ctx context.Context, step *Step) (any, error)
}

// Plan-level policy: terminal errors (deadline, iteration limit)
// are never retried — the run stops and is handed to a human.
func onStepError(step *Step, err error) Decision {
    if isTerminal(err) {
        return Abort
    }
    step.Retries++
    if step.Retries <= maxStepRetries {
        return Retry // retry the step; already-completed results are kept
    }
    return SkipOrAbort
}
```

Boundaries — the deadline for the whole run and the iteration limit — are terminal: planning does not override them. When they are exceeded, the run ends with an error and a message to the human, instead of being masked by endless retries. How to store progress for resumption — [Chapter 11](../11-state-management/README.md).

### Step 5: Persisting plan state

**IMPORTANT:** Saving state for resuming execution is covered in [State Management](../11-state-management/README.md). This section only describes the structure of the plan state.

```go
// Plan state is used to track progress
// Saving and resuming is covered in State Management
type PlanState struct {
    PlanID    string
    Steps     []Step
    UpdatedAt time.Time
}
```

## LivePlan: The Plan as Program State

In a typical implementation, the plan is stored in messages — as text generated by the model. The problem: during condense, the plan is lost along with the other old messages.

In practice, the plan lives **in a Go struct**, not in messages:

```go
type LivePlan struct {
    Goal  string
    Steps []PlanStep
    Notes string
}

type PlanStep struct {
    ID          int
    Description string
    Status      StepStatus // pending, in_progress, completed, cancelled
    Result      string
}

type StepStatus string

const (
    StepPending    StepStatus = "pending"
    StepInProgress StepStatus = "in_progress"
    StepCompleted  StepStatus = "completed"
    StepCancelled  StepStatus = "cancelled"
)
```

Where does the plan go in the context? **Not in the system prompt.** The system prompt must stay unchanged from iteration to iteration — otherwise the prompt cache breaks. The plan is dynamic, so it lives in the tail of the history:

1. When the plan is created, its text is added to the history as a regular message (or a file plus a link to it).
2. From then on, the plan is updated via a tool: the model calls `update_plan`, and the tool call's result — a fresh render of the plan — is inserted into the history before the next iteration.

```go
func (p *LivePlan) Render() string {
    var sb strings.Builder
    sb.WriteString("Plan (current):\n")
    sb.WriteString("Goal: " + p.Goal + "\n")
    for _, step := range p.Steps {
        sb.WriteString(fmt.Sprintf("%d. [%s] %s", step.ID, step.Status, step.Description))
        if step.Result != "" {
            sb.WriteString(" → " + step.Result)
        }
        sb.WriteString("\n")
    }
    return sb.String()
}
```

A plan update as a tool result in the history:

```json
{"role": "assistant", "tool_calls": [{"name": "update_plan",
  "arguments": {"step": 2, "status": "completed", "result": "401 on /api/auth"}}]}
{"role": "tool", "name": "update_plan",
 "content": "Plan (current):\nGoal: ...\n1. [completed] ...\n2. [completed] ...\n3. [pending] ..."}
```

A step's status changes automatically when a subagent runs:
- Before execution → `in_progress`
- After success → `completed`
- After an error → `pending` (rollback)

Rule: **only one step in_progress at a time**. When all steps are `completed` or `cancelled`, the plan is automatically cleared, freeing up space in the history.

**Anti-pattern:** inserting the LivePlan into the system prompt on every iteration. The system prompt must stay immutable — that's the condition for the prompt cache to work. Anything that changes from step to step (the plan, intermediate results) is appended to the end of the history as a message or via tool results.

> **Comparison with Crush (Charmbracelet):** Crush stores the plan in `todos` — a UI checklist that lives in messages. It gets lost during summarization. Here the plan lives in a Go struct that survives condensation, and is rendered into the tail of the history.

## Task Routing: Choosing a Planning Strategy

Not every task needs a plan. A simple question like "Which Go version is installed?" needs no decomposition. The decision rule:

- **Trivial question** (a single answer, no side effects) — answer right away, without a plan.
- **Multi-step task** (tools, changes, dependencies) — build a plan.

The decision itself can be delegated to the model (structured classification in a single request) or skipped entirely: an agent with `update_plan` will build a plan on its own once the task turns out to be complex. But hard heuristics like "len(words) <= 15 && no word 'refactoring'" are fragile — they break depending on phrasing and silently route complex tasks to `direct`.

| Routing result | What the agent does |
|-------------------|------------------|
| `direct` | Executes the task immediately, without planning |
| `plan` | Calls the `plan` tool first, then executes |
| `plan_and_subagent` | Builds a plan, runs a subagent for each step |

## Two Planning Modes

| Mode | Who is in control | When to use |
|-------|---------------|-------------------|
| **Soft (guided loop)** | The LLM follows the plan in the tail of the history | Default for an agent: mid-size tasks, adaptivity |
| **Hard (orchestrator)** | Go code iterates over the steps | Batch pipelines (CI, ETL) — not the default for an agent |

**Soft mode:** the plan lives at the end of the message history. The LLM itself decides which step to execute and updates statuses via `update_plan`.

**Hard mode:** Go code takes the next `pending` step, builds the task, launches a subagent, gets the result, updates the status. The LLM does not control the order — the runtime does. This is reliable for reproducible batch pipelines, but for an interactive agent a hard orchestrator takes away adaptivity — don't make it the default.

Task Routing picks the mode: `plan` → soft, `plan_and_subagent` → hard.

## Common Mistakes

### Mistake 1: No dependency tracking

**Symptom:** The agent tries to execute steps out of order, causing failures.

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

### Mistake 2: No state persistence

**Symptom:** The agent starts from scratch after a failure, losing progress.

**Cause:** Plan state is not saved.

**Solution:** Use the techniques from [State Management](../11-state-management/README.md) to save and resume plan execution.

### Mistake 3: Endless retries

**Symptom:** The agent retries a failed step forever, wasting resources.

**Cause:** No retry limits or backoff.

**Solution:** Transient errors are retried by the LLM client (backoff with jitter, see [Chapter 04](../04-autonomy-and-loops/README.md)); at the plan level — a per-step retry limit, and the deadline and iteration limit are terminal: the run stops and is handed to a human.

### Mistake 4: No parallel execution

**Symptom:** The agent executes independent steps sequentially, wasting time.

**Cause:** Steps that could run in parallel are not identified.

**Solution:** Use `findReadySteps` to get all ready steps and execute them concurrently:

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

### Mistake 5: The plan as free-form text

**Symptom:** The model skips steps, starts Step 3 without finishing Step 2. There is no objective criterion for "step completed".

**Cause:** The plan is generated as free-form text without a machine-readable structure. Step statuses are not tracked.

**Solution:** Use a structured plan with explicit statuses:

```go
// BAD: plan as text in messages
plan := "1. Check the logs\n2. Find the error\n3. Fix it\n4. Test"

// GOOD: a LivePlan with tracking
plan := &LivePlan{
    Goal: "Fix the authorization error",
    Steps: []PlanStep{
        {ID: 1, Description: "Check the nginx logs", Status: StepCompleted, Result: "401 on /api/auth"},
        {ID: 2, Description: "Find the cause in the code", Status: StepInProgress},
        {ID: 3, Description: "Fix the middleware", Status: StepPending},
        {ID: 4, Description: "Test", Status: StepPending},
    },
}
```

## Pattern: Controller + Processor (Orchestrator + Normalizer)

When a workflow is complex and there are many tools, it's tempting to split the roles: the **Controller** picks the next step, while the **Processor** normalizes tool results into "state patches" — `state_patch`, `known_facts`, `confidence: 0.9` — right inside the LLM context.

This is an anti-pattern. Fact scoring and state patches inside the context are hidden logic that bypasses the model: it can't be tested, it silently degrades under condensation, and it encourages manually "picking the relevant state." The agent's state should live outside the context — in Go structs and the database ([Chapter 11](../11-state-management/README.md)), and large tool results should live in artifacts outside the history, with only a brief excerpt in the context. Then the Controller becomes the ordinary agent loop from [Chapter 04](../04-autonomy-and-loops/README.md), and a separate Processor isn't needed.

## Exercises

### Exercise 1: Task decomposition

Implement a function that breaks a task into steps:

```go
func decomposeTask(task string) (*Plan, error) {
    // Use the LLM to create a plan
    // Return a Plan with steps and dependencies
}
```

**Expected result:**
- The plan contains logical steps
- Dependencies are identified correctly
- Steps can be executed in a valid order

### Exercise 2: Dependency resolution

Implement `findReadySteps` yourself, without peeking at the implementation from "Step 3" (signature: `func findReadySteps(plan *Plan) []*Step`). Then compare your version with the one from the chapter.

**Expected result:**
- Returns only steps whose dependencies are all satisfied
- Handles cyclic dependencies (detects them and errors out)

### Exercise 3: Plan execution with retries

Implement plan execution with retry logic:

```go
func executePlanWithRetries(ctx context.Context, plan *Plan, executor StepExecutor, maxRetries int) error {
    // Execute the plan with retry logic
    // Handle failures gracefully
}
```

**Expected result:**
- Steps are executed with dependencies taken into account
- Failed steps are retried up to maxRetries times
- The plan either completes or fails gracefully

## Acceptance Criteria / Checklist

**Pass:**
- [x] You can break complex tasks into steps
- [x] You understand dependency graphs
- [x] You can execute plans respecting dependencies
- [x] You handle failures with retries
- [x] You persist plan state for resumption
- [x] You understand why the plan lives in the history tail, not in the system prompt
- [x] You know when routing is needed to choose a strategy

**Fail:**
- [ ] Executing steps without checking dependencies
- [ ] No state persistence
- [ ] Endless retries without limits
- [ ] Sequential execution when parallel is possible
- [ ] A plan as free text with no status tracking

## Relation to Other Chapters

- **[Chapter 04: Autonomy and Loops](../04-autonomy-and-loops/README.md)** — Planning extends the ReAct loop for complex tasks
- **[Chapter 07: Multi-Agent Systems](../07-multi-agent/README.md)** — Planning can coordinate multiple agents
- **[Chapter 11: State Management](../11-state-management/README.md)** — Reliable plan execution (idempotency, retries, persistence)
- **[Chapter 21: Workflow and State Management in Production](../21-workflow-state-management/README.md)** — Production workflow patterns

**IMPORTANT:** Planning focuses on **task decomposition** and **dependency graphs**. Execution reliability (persistence, retries, deadlines) is covered in [State Management](../11-state-management/README.md).

## What's Next?

Once you've mastered the planning patterns, move on to:
- **[11. State Management](../11-state-management/README.md)** — Learn how to guarantee reliable plan execution

