# Lab 06: Incident Management (Advanced Planning)

## Goal
Create an SRE-level agent capable of independently investigating and resolving failures using **explicit planning** (Plan-and-Solve) and SOP (Standard Operating Procedure).

## Theory

### Planning — Breaking Down Tasks into Steps

In this lab, we use **explicit planning** (Plan-and-Solve), unlike implicit planning (ReAct) from Lab 04.

**Difference:**

**Implicit planning (ReAct) — Lab 04:**
- Agent plans "on the fly"
- Suitable for simple tasks (2-4 steps)
- Example: "Check disk" → "Clean logs" → "Check again"

**Explicit planning (Plan-and-Solve) — Lab 06:**
- Agent first creates a full plan
- Then executes plan step by step
- Suitable for complex tasks (5+ steps)
- Example: "Plan: 1. Check HTTP 2. Read logs 3. Analyze 4. Fix 5. Verify"

### SOP (Standard Operating Procedure)

**SOP** is an action algorithm encoded in the prompt. It's like a manual for a soldier: clear instructions on what to do in each situation.

**Example SOP for incident:**
```
SOP for service failure:
1. Check Status: Check HTTP response code
2. Check Logs: If 500/502 — read last 20 lines of logs
3. Analyze: Find keywords:
   - "Syntax error" → Rollback
   - "Connection refused" → Check Database
   - "Out of memory" → Restart
4. Action: Apply fix according to analysis
5. Verify: Check HTTP status again
```

**Why is SOP important?**

Without SOP, the model sees: `User: Fix it`. Its probabilistic mechanism may output: `Call: restart_service`. This is the most "popular" action.

With SOP, the model is forced to generate text:
- "Step 1: I need to check HTTP status." → This increases probability of calling `check_http`
- "HTTP is 502. Step 2: I need to check logs." → This increases probability of calling `read_logs`

We **direct the model's attention** along the right path.

### Task Decomposition

The task "Investigate the incident" is broken down into subtasks:

1. **Diagnostics:** What happened?
   - Check service status
   - Read logs
   - Analyze errors

2. **Solution:** How to fix?
   - Determine cause
   - Choose correct action (rollback/restart/scale)
   - Apply fix

3. **Verification:** Did the solution help?
   - Check status again
   - Ensure problem is resolved

**Decomposition principles:**
- **Atomicity:** Each step is executable with one action
- **Dependencies:** Steps execute in correct order
- **Verifiability:** Each step has a clear success criterion

## Assignment
In `main.go` — large skeleton.

1. **Tools:** You have 4 tools:
   - `check_http` — HTTP status check
   - `read_logs` — service log reading
   - `restart_service` — service restart
   - `rollback_deploy` — rollback to previous version

2. **SOP in prompt:** Add detailed SOP for incident handling to System Prompt.

3. **The Loop:** Implement agent loop that strictly follows SOP.

4. **Scenario:** Run the agent with prompt: *"Payment Service is down (502). Fix it."*
   - Expected: Agent follows SOP:
     - Checks HTTP → 502
     - Reads logs → "Syntax error"
     - Does rollback (not restart!)
     - Verifies → 200 OK

## Important
- Agent must **strictly follow SOP**, not guess
- Agent must **read logs before action**, not immediately restart
- Agent must **verify result** after fix
