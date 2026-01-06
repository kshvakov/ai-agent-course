# Manual: Lab 04 — The Agent Loop (Autonomy)

## Why Is This Needed?

In this lab you'll implement the **ReAct (Reason + Act)** pattern — the heart of an autonomous agent. The agent will learn to independently make decisions, execute actions, and analyze results in a loop, without human intervention.

### Real-World Case Study

**Situation:** User writes: "I'm out of space on the server. Fix it."

**Without autonomous loop:**
- Agent: "I'll check disk usage" → calls `check_disk` → gets "95%"
- Agent: [Stops, waits for next user command]

**With autonomous loop:**
- Agent: "I'll check disk usage" → calls `check_disk` → gets "95%"
- Agent: "Disk is full. Cleaning logs" → calls `clean_logs` → gets "Freed 20GB"
- Agent: "Checking again" → calls `check_disk` → gets "40%"
- Agent: "Done! Freed 20GB."

**Difference:** Agent decides what to do next based on results of previous actions.

## Theory in Simple Terms

### ReAct Loop (Autonomy Loop)

An autonomous agent works in a loop:

```
While (Task not solved):
  1. Send history to LLM
  2. Get response
  3. IF it's text → Show user and wait for new input
  4. IF it's a tool call →
       a. Execute tool
       b. Add result to history
       c. GOTO 1 (without asking user!)
```

**Key point:** Point 4.c provides the "magic" — the agent looks at the result and decides what to do next.

### Closing the Loop

After executing a tool, **don't ask the user** what to do next. Send the result back to the LLM. The model receives the result of its actions and decides what to do next.

**Example dialogue in memory:**

1. **User:** "Out of space."
2. **Assistant (ToolCall):** `check_disk()`
3. **Tool (Result):** "95% usage."
4. **Assistant (ToolCall):** `clean_logs()` ← Agent decided this itself!
5. **Tool (Result):** "Freed 20GB."
6. **Assistant (Text):** "I cleaned logs, now there's enough space."

## Execution Algorithm

### Step 1: Tool Definition

```go
tools := []openai.Tool{
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name:        "check_disk",
            Description: "Check current disk usage",
        },
    },
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name:        "clean_logs",
            Description: "Delete old logs to free space",
        },
    },
}
```

### Step 2: Initial History

```go
messages := []openai.ChatCompletionMessage{
    {
        Role:    openai.ChatMessageRoleSystem,
        Content: "You are an autonomous DevOps agent.",
    },
    {
        Role:    openai.ChatMessageRoleUser,
        Content: "I'm out of space. Fix it.",
    },
}
```

### Step 3: Agent Loop

```go
for i := 0; i < maxIterations; i++ {
    // 1. Send request
    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model:    openai.GPT3Dot5Turbo,
        Messages: messages,
        Tools:    tools,
    })
    
    msg := resp.Choices[0].Message
    messages = append(messages, msg)  // Save response
    
    // 2. Check response type
    if len(msg.ToolCalls) == 0 {
        // This is final text response
        fmt.Println("Agent:", msg.Content)
        break
    }
    
    // 3. Execute tools
    for _, toolCall := range msg.ToolCalls {
        result := executeTool(toolCall.Function.Name, toolCall.Function.Arguments)
        
        // 4. Add result to history
        messages = append(messages, openai.ChatCompletionMessage{
            Role:       openai.ChatMessageRoleTool,
            Content:    result,
            ToolCallID: toolCall.ID,
        })
    }
    // Loop continues automatically!
}
```

## Common Errors

### Error 1: Infinite Loop

**Symptom:** Agent repeats the same action infinitely.

**Example:**
```
Action: check_disk()
Result: "95%"
Action: check_disk()  // Again!
Result: "95%"
Action: check_disk()  // And again!
```

**Solution:**
1. Add iteration limit
2. Detect repeating actions
3. Improve prompt: "If action didn't help, try a different approach"

### Error 2: Tool Result Not Added to History

**Symptom:** Agent doesn't see tool result and continues executing the same action.

**Cause:** You forgot to add result to `messages`.

**Solution:**
```go
// MUST add result!
messages = append(messages, openai.ChatCompletionMessage{
    Role:       openai.ChatMessageRoleTool,
    Content:    result,
    ToolCallID: toolCall.ID,  // Important for linking!
})
```

### Error 3: Agent Doesn't Stop

**Symptom:** Agent continues calling tools even when task is solved.

**Solution:**
```go
// Add to System Prompt:
"If task is solved, respond with text to user. Don't call tools unnecessarily."
```

## Mini-Exercises

### Exercise 1: Add Loop Detection

Implement a check that the last 3 actions are the same:

```go
func isStuck(history []ChatCompletionMessage) bool {
    if len(history) < 3 {
        return false
    }
    lastActions := getLastActions(history, 3)
    return allEqual(lastActions)
}
```

### Exercise 2: Add Logging

Log each loop iteration:

```go
fmt.Printf("[Iteration %d] Agent decided: %s\n", i, action)
fmt.Printf("[Iteration %d] Tool result: %s\n", i, result)
```

## Completion Criteria

✅ **Completed:**
- Agent executes loop autonomously
- Tool results added to history
- Agent stops when task is solved
- Loop protection exists

❌ **Not completed:**
- Agent doesn't continue loop after executing tool
- Tool results not visible to agent
- Agent loops infinitely

---

**Next step:** After successfully completing Lab 04, proceed to [Lab 05: Human-in-the-Loop](../lab05-human-interaction/README.md)
