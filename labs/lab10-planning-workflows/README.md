# Lab 10: Planning and Workflows

## Goal
Learn how to implement planning patterns: task decomposition, dependency resolution, plan execution with retries, and state persistence.

## Theory

### Planning Patterns

When agents handle complex multi-step tasks, they need planning:
- **Task decomposition** — Break large task into steps
- **Dependency graph** — Understand dependencies between steps
- **Execution order** — Execute steps considering dependencies
- **State tracking** — Know what's done, what's pending
- **Error handling** — Retry, skip, or abort on errors

### Explicit Planning vs Implicit Planning

**Implicit planning (ReAct) — Lab 04:**
- Agent plans "on the fly" during execution
- Suitable for simple tasks (2-4 steps)
- Example: "Check disk" → "Clean logs" → "Check again"

**Explicit planning (Plan-and-Solve) — Lab 10:**
- Agent first creates a complete plan
- Then executes plan step by step
- Suitable for complex tasks (5+ steps)
- Example: "Plan: 1. Check HTTP 2. Read logs 3. Analyze 4. Fix 5. Verify"

### Dependency Resolution

Steps may depend on each other:
- Step B requires Step A to complete first
- Steps can run in parallel if no dependencies
- Cyclic dependencies must be detected and rejected

### State Persistence

Plans should be saved to disk:
- Resume after interruption
- Track progress
- Handle failures gracefully

See more: [Chapter 10: Planning and Workflow Patterns](../../book/10-planning-and-workflows/README.md)

## Task

In `main.go` implement a planning system for complex tasks.

### Part 1: Task Decomposition

Implement function `createPlan(task string) (*Plan, error)`:
- Use LLM to decompose task into steps
- Define dependencies between steps
- Return Plan with steps and dependencies

**Example:**
```
Task: "Deploy new version of service"
Plan:
  Step 1: Backup database (no dependencies)
  Step 2: Build new version (depends on: Step 1)
  Step 3: Deploy to staging (depends on: Step 2)
  Step 4: Run tests (depends on: Step 3)
  Step 5: Deploy to production (depends on: Step 4)
```

### Part 2: Dependency Resolution

Implement function `findReadySteps(plan *Plan) []*Step`:
- Return steps whose all dependencies are completed
- Handle cyclic dependencies (detect and return error)
- Support parallel execution of independent steps

### Part 3: Plan Execution with Retries

Implement function `executePlanWithRetries(plan *Plan, executor StepExecutor, maxRetries int) error`:
- Execute steps considering dependencies
- Retry failed steps up to maxRetries
- Handle errors correctly (skip, abort, or retry)
- Track step status (pending, running, completed, failed)

### Part 4: State Persistence

Implement function `savePlanState(planID string, plan *Plan) error`:
- Save plan state to file (JSON format)
- Include plan resumption after interruption
- Load plan state from file

## Important

- Always check dependencies before executing steps
- Handle errors gracefully (retry, skip, or abort)
- Save state after each step completion
- Detect and reject cyclic dependencies

## Completion Criteria

✅ **Completed:**
- Task decomposition implemented
- Dependency resolution works correctly
- Plan execution with retries
- State persistence implemented
- Can resume interrupted plans
- Cyclic dependencies detected

❌ **Not completed:**
- Steps executed without checking dependencies
- No state persistence
- Infinite retries without limits
- Cyclic dependencies not detected

---

**Next step:** After completing Lab 10, proceed to [Lab 11: Memory and Context Management](../lab11-memory-context/README.md).
