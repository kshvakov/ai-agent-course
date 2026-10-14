# Manual: Lab 06 — Incident Management (SOP)

## Why This Lab?

In this lab you'll create an **SRE (Site Reliability Engineer)** level agent. Unlike simple tasks, incidents require **strategic thinking** and following a strict algorithm (SOP).

### Real-World Case Study

**Situation:** Payment service is unavailable (502 Bad Gateway).

**Without SOP:**
- Agent: [Immediately restarts service]
- Result: Service doesn't start (problem in config)
- Agent: [Restarts again]
- Result: Same error
- Agent: [Loops]

**With SOP:**
- Agent: Checks HTTP status → 502
- Agent: Reads logs → Receives "Config syntax error"
- Agent: Understands restart won't help
- Agent: Does rollback → Service recovers

**Difference:** SOP forces agent to follow algorithm, not guess.

## Theory in Simple Terms

### Planning — breaking task into steps

In this lab, you'll use **explicit planning** (Plan-and-Solve), unlike implicit planning (ReAct) from Lab 04.

**Difference:**

| Implicit (ReAct) | Explicit (Plan-and-Solve) |
|---------------------|------------------------|
| Plans "on the fly" | Creates plan first |
| Suitable for simple tasks (2-4 steps) | Suitable for complex tasks (5+ steps) |
| Flexible, adapts to results | Structured, guarantees all steps executed |

**How explicit planning works:**

1. **Plan generation:** Agent receives task and creates complete plan
   ```
   Plan:
   1. Check HTTP status
   2. Read logs
   3. Analyze errors
   4. Apply fix
   5. Verify
   ```

2. **Plan execution:** Agent executes steps in order, tracking progress

3. **Adaptation:** If step didn't help, agent can replan

### Task Decomposition

The task "Investigate incident" is broken into subtasks:

**Decomposition principles:**
- **Atomicity:** Each step is executable with one action
  - ❌ Bad: "Check and fix server"
  - ✅ Good: "Check status" → "Read logs" → "Apply fix"

- **Dependencies:** Steps executed in correct order
  - ❌ Bad: "Apply fix" → "Read logs"
  - ✅ Good: "Read logs" → "Analyze" → "Apply fix"

- **Verifiability:** Each step has a clear success criterion
  - ❌ Bad: "Improve performance"
  - ✅ Good: "Reduce CPU from 95% to 50%"

**Example decomposition for incident:**
```
Original task: "Service unavailable (502). Fix it."

Decomposition:
1. Check service HTTP status
   - Success criterion: Got HTTP code (200/502/500)
   
2. Read service logs
   - Success criterion: Got last 20 log lines
   
3. Analyze errors in logs
   - Success criterion: Cause identified (Syntax error / Connection error / Memory)
   
4. Apply fix according to analysis
   - Success criterion: Fix applied (rollback/restart executed)
   
5. Verify recovery
   - Success criterion: HTTP status = 200 OK
```

### SOP (Standard Operating Procedure)

**SOP** is an action algorithm encoded in the prompt. It's like a manual for a soldier: clear instructions on what to do in each situation.

**Example SOP for incident:**

```
SOP for service failure:
1. Check Status: Check HTTP response code
2. Check Logs: If 500/502 — read last 20 log lines
3. Analyze: Find keywords:
   - "Syntax error" → Rollback
   - "Connection refused" → Check Database
   - "Out of memory" → Restart
4. Action: Apply fix according to analysis
5. Verify: Check HTTP status again
```

**Why does this work?**

Without SOP, the model receives: `User: Fix it`. Its probabilistic mechanism may output: `Call: restart_service`. This is the most "popular" action.

With SOP, the model is forced to generate text:
- "Step 1: I need to check HTTP status." → This increases probability of calling `check_http`
- "HTTP is 502. Step 2: I need to check logs." → This increases probability of calling `read_logs`

We **direct the model's attention** in the right direction.

### Chain-of-Thought in Action

Note the System Prompt in the solution:
`"Think step by step following this SOP: 1. Check HTTP... 2. Check Logs..."`

Why is this needed?

Without this prompt, the model receives: `User: Fix it`.  
Its probabilistic mechanism may output: `Call: restart_service`. This is the most "popular" action.

With this prompt, the model is forced to generate text:
- "Step 1: I need to check HTTP status." → This increases probability of calling `check_http`
- "HTTP is 502. Step 2: I need to check logs." → This increases probability of calling `read_logs`

We **direct the model's attention** in the right direction.

## Execution Algorithm

### Step 1: Tool Definition

```go
tools := []llm.ToolDef{
    {Name: "check_http", Description: "Check service HTTP status"},
    {Name: "read_logs", Description: "Read service logs. Do this if HTTP is 500/502."},
    {Name: "restart_service", Description: "Restart the service. Use ONLY if logs show transient error."},
    {Name: "rollback_deploy", Description: "Rollback to previous version. Use if logs show Config/Syntax error."},
}
```

**Important:** In `Description` specify when to use the tool. This helps the model choose correctly.

### Step 2: SOP in System Prompt

```go
sopPrompt := `You are a Site Reliability Engineer (SRE).
Your goal is to fix the Payment Service.
Follow this Standard Operating Procedure (SOP) strictly:
1. Check HTTP status first.
2. If status is not 200, READ LOGS immediately. Do not guess.
3. Analyze logs:
   - If "Syntax Error" or "Config Error" -> ROLLBACK.
   - If "Connection Error" -> RESTART.
4. Verify fix by checking HTTP status again.

ALWAYS Think step by step. Output your thought process before calling a tool.`
```

### Step 3: Agent Loop with Logging

```go
for i := 0; i < 15; i++ {
    resp, err := client.Complete(ctx, req)
    msg := resp.Choices[0].Message
    messages = append(messages, msg)
    
    // Log agent thoughts
    if msg.Content != "" {
        fmt.Printf("🧠 Thought: %s\n", msg.Content)
    }
    
    if len(msg.ToolCalls) == 0 {
        fmt.Printf("🤖 Agent: %s\n", msg.Content)
        break
    }
    
    for _, toolCall := range msg.ToolCalls {
        fmt.Printf("🔧 Call: %s\n", toolCall.Name)
        result := executeTool(toolCall)
        fmt.Printf("📦 Result: %s\n", result)
        
        messages = append(messages, toolResult)
    }
}
```

## Common Errors

### Error 1: Agent Doesn't Follow SOP

**Symptom:** Agent immediately restarts without reading logs.

**Cause:** SOP not strict enough or model ignores it.

**Solution:**
1. Strengthen SOP: "CRITICAL: Never restart without reading logs first"
2. Add Few-Shot examples to prompt
3. Use a stronger model (set a higher-tier model via LLM_MODEL)

### Error 2: Agent Loops on One Step

**Symptom:** Agent repeats `check_http` multiple times in a row.

**Cause:** No explicit instruction to move to next step.

**Solution:**
```go
// In SOP:
"After checking HTTP status, move to step 2. Do not repeat step 1."
```

### Error 3: Wrong Action Choice

**Symptom:** Agent does rollback instead of restart (or vice versa).

**Cause:** Tool descriptions unclear.

**Solution:**
```go
// Improve Description:
Description: "Rollback to previous version. Use ONLY if logs show 'Syntax error' or 'Config error'."
```

## Mini-Exercises

### Exercise 1: Add Decision Table

Create a table "symptom → hypothesis → check → action":

```go
decisionTable := map[string]string{
    "502 + Syntax error": "rollback",
    "502 + Connection refused": "check_database",
    "502 + Out of memory": "restart",
}
```

### Exercise 2: Add Verification

After each action, check if it helped:

```go
// After rollback:
verifyResult := checkHttp()
if verifyResult == "200 OK" {
    fmt.Println("✅ Incident resolved!")
} else {
    fmt.Println("⚠️ Rollback didn't help. Trying next step...")
}
```

## Completion Criteria

✅ **Completed:**
- Agent strictly follows SOP
- Agent reads logs before action
- Agent chooses correct action (rollback vs restart)
- Agent verifies result

❌ **Not completed:**
- Agent doesn't follow SOP
- Agent doesn't read logs
- Agent chooses wrong action
- Agent doesn't verify result

---

**Next step:** After successfully completing Lab 06, proceed to [Lab 07: RAG](../lab07-rag/README.md)
