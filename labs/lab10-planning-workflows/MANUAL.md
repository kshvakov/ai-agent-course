# Method Guide: Lab 10 — Planning and Workflows

## Why Is This Needed?

In this lab you'll implement **explicit planning** (Plan-and-Solve) — a pattern for complex multi-step tasks. The agent first creates a complete plan, then executes it step by step.

### Real-World Case Study

**Situation:** Need to deploy a new version of the service.

**Without planning:**
- Agent: [Immediately starts deployment]
- Result: Database backup skipped
- Result: Deployment to production without testing

**With planning:**
- Agent creates plan:
  1. Create database backup
  2. Build new version
  3. Deploy to staging
  4. Run tests
  5. Deploy to production
- Agent executes plan step by step
- Result: All steps executed in correct order

**Difference:** Planning guarantees execution of all necessary steps.

## Theory in Simple Terms

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

### Task Decomposition

Complex tasks need to be broken down into subtasks.

**Decomposition principles:**
- **Atomicity:** Each step is executed with one action
  - ❌ Bad: "Check and fix server"
  - ✅ Good: "Check status" → "Read logs" → "Apply fix"

- **Dependencies:** Steps executed in correct order
  - ❌ Bad: "Apply fix" → "Read logs"
  - ✅ Good: "Read logs" → "Analyze" → "Apply fix"

- **Verifiability:** Each step has a clear success criterion
  - ❌ Bad: "Improve performance"
  - ✅ Good: "Reduce CPU from 95% to 50%"

### Dependency Resolution

Steps may depend on each other:
- Step B requires Step A to complete first
- Steps can run in parallel if no dependencies
- Cyclic dependencies must be detected and rejected

**Example dependency graph:**
```
Step 1 (no dependencies) → can execute immediately
Step 2 (depends on Step 1) → waits for Step 1
Step 3 (depends on Step 1) → waits for Step 1, but can run in parallel with Step 2
Step 4 (depends on Step 2 and Step 3) → waits for both
```

### State Persistence

Plans should be saved to disk:
- Resume after interruption
- Track progress
- Handle failures gracefully

## Execution Algorithm

### Step 1: Creating Plan via LLM

```go
prompt := fmt.Sprintf(`Break down the task into steps with dependencies.
Task: %s

Return plan in JSON format:
{
  "steps": [
    {"id": "step1", "description": "...", "dependencies": []},
    {"id": "step2", "description": "...", "dependencies": ["step1"]}
  ]
}`, task)

resp, _ := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
    Model: openai.GPT3Dot5Turbo,
    Messages: []openai.ChatCompletionMessage{
        {Role: "user", Content: prompt},
    },
})

// Parse JSON response into Plan structure
```

### Step 2: Finding Ready Steps

```go
func findReadySteps(plan *Plan) []*Step {
    var ready []*Step
    for _, step := range plan.Steps {
        if step.Status != "pending" {
            continue
        }
        
        // Check if all dependencies are completed
        allDepsCompleted := true
        for _, depID := range step.Dependencies {
            dep := findStep(plan, depID)
            if dep.Status != "completed" {
                allDepsCompleted = false
                break
            }
        }
        
        if allDepsCompleted {
            ready = append(ready, step)
        }
    }
    return ready
}
```

### Step 3: Plan Execution

```go
func executePlan(plan *Plan, executor StepExecutor) error {
    for {
        ready := findReadySteps(plan)
        if len(ready) == 0 {
            // Check if all steps are completed
            if allStepsCompleted(plan) {
                return nil
            }
            return fmt.Errorf("deadlock: no ready steps, but plan not completed")
        }
        
        // Execute ready steps
        for _, step := range ready {
            step.Status = "running"
            result, err := executor.Execute(step)
            if err != nil {
                step.Status = "failed"
                return err
            }
            step.Status = "completed"
            step.Result = result
        }
    }
}
```

### Step 4: State Persistence

```go
func savePlanState(planID string, plan *Plan) error {
    data, err := json.Marshal(plan)
    if err != nil {
        return err
    }
    return os.WriteFile(fmt.Sprintf("plan_%s.json", planID), data, 0644)
}
```

## Common Mistakes

### Mistake 1: Dependencies Not Checked

**Symptom:** Steps executed in wrong order.

**Cause:** Dependencies not checked before execution.

**Solution:** Always call `findReadySteps()` before execution.

### Mistake 2: Cyclic Dependencies Not Detected

**Symptom:** Plan hangs, no ready steps.

**Cause:** Cyclic dependencies not detected during plan creation.

**Solution:** Check for cycles when creating plan or before execution.

### Mistake 3: No State Persistence

**Symptom:** All progress lost on interruption.

**Cause:** State not saved.

**Solution:** Save plan after each completed step.

## Completion Criteria

✅ **Completed:**
- Plan created via LLM
- Dependencies checked before execution
- Cyclic dependencies detected
- State saved to file
- Plan can be resumed after interruption

❌ **Not completed:**
- Steps executed without checking dependencies
- Cyclic dependencies not detected
- No state persistence
- Plan cannot be resumed
