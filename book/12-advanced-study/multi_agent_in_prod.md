# Multi-Agent in Production

## Why This Chapter?

Multi-Agent system works, but you cannot understand where error occurred in chain Supervisor → Worker → Tool. Without production-ready Multi-Agent, you cannot debug complex systems.

### Real-World Case Study

**Situation:** Supervisor routed task to Network Admin Worker, but task didn't complete. You cannot understand where error occurred.

**Problem:** No tracing through agent chain, no context isolation, no log correlation.

**Solution:** Tracing through agent chain, context isolation, log correlation via `run_id`, safety circuits for each Worker.

## Theory in Simple Terms

### What Is Context Isolation?

Context isolation is separate context for each Worker. Supervisor has its context, each Worker has its own. This prevents context "overflow".

### What Is Chain Tracing?

Chain tracing is tracking full request path through Supervisor → Worker → Tool. Each step has its span in trace.

## How It Works (Step-by-Step)

### Step 1: Chain Tracing

Trace chain Supervisor → Worker → Tool:

```go
func (s *Supervisor) ExecuteTaskWithTracing(ctx context.Context, task string) (string, error) {
    span := trace.StartSpan("supervisor.route_task")
    defer span.End()
    
    worker, err := s.RouteTask(task)
    if err != nil {
        span.RecordError(err)
        return "", err
    }
    
    span.SetAttributes(
        attribute.String("worker.name", worker.name),
    )
    
    // Worker executes with tracing
    result, err := worker.ExecuteWithTracing(ctx, task)
    if err != nil {
        span.RecordError(err)
        return "", err
    }
    
    return result, nil
}
```

### Step 2: Context Isolation

Each Worker has its isolated context:

```go
type Worker struct {
    name         string
    systemPrompt string
    tools        []openai.Tool
    // Isolated context for this Worker
}

func (w *Worker) Execute(task string) (string, error) {
    // Worker uses its systemPrompt and tools
    messages := []openai.ChatCompletionMessage{
        {Role: "system", Content: w.systemPrompt},
        {Role: "user", Content: task},
    }
    // ... execution ...
}
```

## Where to Integrate in Our Code

### Integration Point: Multi-Agent System

In `labs/lab08-multi-agent/main.go`, add tracing and context isolation:

```go
func (s *Supervisor) ExecuteTask(ctx context.Context, task string) (string, error) {
    runID := generateRunID()
    
    // Supervisor tracing
    log.Printf("SUPERVISOR_START: run_id=%s task=%s", runID, task)
    
    worker := s.RouteTask(task)
    
    // Worker tracing
    log.Printf("WORKER_START: run_id=%s worker=%s", runID, worker.name)
    
    result, err := worker.Execute(task)
    
    log.Printf("WORKER_END: run_id=%s worker=%s result=%s", runID, worker.name, result)
    log.Printf("SUPERVISOR_END: run_id=%s", runID)
    
    return result, err
}
```

## Common Mistakes

### Mistake 1: No Chain Tracing

**Symptom:** Cannot understand where error occurred in chain Supervisor → Worker → Tool.

**Solution:** Trace each chain step with `run_id`.

### Mistake 2: No Context Isolation

**Symptom:** Context overflows, agents confuse between tasks.

**Solution:** Each Worker has its isolated context.

## Completion Criteria / Checklist

✅ **Completed:**
- Tracing through agent chain
- Context isolation for each Worker
- Log correlation via `run_id`

❌ **Not completed:**
- No chain tracing
- No context isolation

## Connection with Other Chapters

- **Multi-Agent:** Basic concepts — [Chapter 08: Multi-Agent Systems](../08-multi-agent/README.md)
- **Observability:** Tracing as part of observability — [Observability and Tracing](observability.md)

---

**Navigation:** [← Evals in CI/CD](evals_in_cicd.md) | [Chapter 12 Table of Contents](README.md) | [Model and Decoding →](model_and_decoding.md)
