# Lab 04: The Agent Loop (Autonomy)

## Goal
Put it all together: Brain + Tools + Memory. Implement the **ReAct** pattern (Reason + Act). The agent should autonomously solve a task in a loop.

## Theory

### Planning — ability to break a task into steps

**Planning** is the agent's ability to break a complex task into a sequence of simple steps and execute them in the correct order.

**Example:**
- **Without planning:** Agent receives "Out of space" and may immediately try to delete files without checking how much space is used.
- **With planning:** Agent first checks disk usage → sees 95% → decides to clean logs → checks again → reports result.

### ReAct (Reason + Act) — implicit planning

In this lab we implement **implicit planning** through the ReAct pattern. The agent doesn't create an explicit plan, but plans "on the fly" during execution.

An autonomous agent works in a loop:
```
While (Task not solved):
  1. Send history to LLM.
  2. Get response.
  3. IF it's text -> Show user and wait for new input.
  4. IF it's a tool call ->
       a. Execute tool.
       b. Add result to history.
       c. GOTO 1 (without asking user!).
```
Point 4.c provides the "magic" — the agent looks at the result and decides what to do next.

**How it works:**
1. Agent receives task: "Out of space"
2. Agent thinks: "What do I need to do first? Check disk usage"
3. Agent executes: `check_disk_usage()`
4. Agent sees result: "95%"
5. Agent thinks: "Disk is full. What to do next? Clean logs"
6. Agent executes: `clean_logs()`
7. Agent sees result: "Freed 20GB"
8. Agent thinks: "Check again to make sure"
9. Agent executes: `check_disk_usage()`
10. Agent sees result: "40%"
11. Agent thinks: "Task solved. Report to user"

**Note:** For more complex tasks (5+ steps), **explicit planning** (Plan-and-Solve) is used, which we'll study in Lab 06.

## Task
In `main.go` — large template.

1.  **Tools:** You have 2 tools: `check_disk_usage` (returns 95%) and `clean_logs` (frees space).
2.  **The Loop:** Implement the `RunAgentLoop` function. It should loop until LLM responds with regular text.
3.  **Scenario:** Run the agent with prompt: *"I'm out of space on the server. Fix it."*
    *   Expected: Agent calls `check_disk_usage` -> sees 95% -> decides to call `clean_logs` -> checks again -> says "Done".

## Important
Don't forget to handle errors and add them to history! If a tool fails, LLM should know and try something else.
