# Lab 10: Planning and Workflows

## Goal
Learn to implement planning patterns: task decomposition, dependency resolution, plan execution with retries, and state persistence.

## Theory

### Planning Patterns

When agents handle complex multi-step tasks, they need planning:
- **Task decomposition** — Break large task into steps
- **Dependency graph** — Understand dependencies between steps
- **Execution order** — Execute steps considering dependencies
- **State tracking** — Know what's done, what's pending
- **Error handling** — Retry, skip, or abort on errors

See more: [Chapter 10: Planning and Workflow Patterns](../../book/10-planning-and-workflows/README.md)

## Task

In `main.go` implement a planning system for complex tasks.

### Part 1: Task Decomposition

Implement function `createPlan(task string) (*Plan, error)`:
- Use LLM to decompose task into steps
- Define dependencies between steps
- Return Plan with steps and dependencies

### Part 2: Dependency Resolution

Implement function `findReadySteps(plan *Plan) []*Step`:
- Return steps whose all dependencies are completed
- Handle cyclic dependencies (detect and return error)

### Part 3: Plan Execution with Retries

Implement function `executePlanWithRetries(plan *Plan, executor StepExecutor, maxRetries int) error`:
- Execute steps considering dependencies
- Retry failed steps up to maxRetries
- Handle errors correctly

### Part 4: State Persistence

Implement function `savePlanState(planID string, plan *Plan) error`:
- Save plan state to file
- Include plan resumption after interruption

## Completion Criteria

✅ **Completed:**
- Task decomposition implemented
- Dependency resolution works correctly
- Plan execution with retries
- State persistence implemented
- Can resume interrupted plans

❌ **Not completed:**
- Steps executed without checking dependencies
- No state persistence
- Infinite retries without limits

---

**Next step:** After completing Lab 10, proceed to [Lab 11: Memory and Context Management](../lab11-memory-context/README.md).
