# 05. Autonomy and Loops — ReAct Loop

## Why This Chapter?

In this chapter, we implement the **ReAct (Reason + Act)** pattern — the heart of an autonomous agent.

Without an autonomous loop, an agent works like a chatbot: one request → one response. With an autonomous loop, an agent can perform 10 actions in a row to solve one task, independently making decisions based on results of previous actions.

### Real-World Case Study

**Situation:** User writes: "I'm out of space on the server. Fix it."

**Without autonomous loop:**
- Agent: "I'll check disk usage" → calls `check_disk` → gets "95%"
- Agent: [Stops, waits for next user command]

**With autonomous loop:**
- Agent: "I'll check disk usage" → calls `check_disk` → gets "95%"
- Agent: "Disk is full. I'll clean logs" → calls `clean_logs` → gets "Freed 20GB"
- Agent: "I'll check again" → calls `check_disk` → gets "40%"
- Agent: "Done! Freed 20GB."

**Difference:** Agent decides what to do next based on results of previous actions.

## Theory in Simple Terms

### ReAct Loop (Autonomy Cycle)

An autonomous agent works in a loop:

```
While (Task not solved):
  1. Send history to LLM
  2. Get response
  3. IF it's text → Show to user and wait for new input
  4. IF it's a tool call →
       a. Execute tool
       b. Add result to history
       c. GOTO 1 (without asking user!)
```

**Key point:** Point 4.c provides the "magic" — the agent itself looks at the result and decides what to do next. But this is not real magic: the model sees the tool result in context (`messages[]`) and generates the next step based on this context.

### Closing the Loop

After executing a tool, we **don't ask the user** what to do next. We send the result back to the LLM. The model sees the result of its actions and decides what to do next.

**Example dialogue in memory:**

### Magic vs Reality: How the Loop Works

**❌ Magic (as usually explained):**
> Agent itself decided to call `clean_logs()` after checking disk

**✅ Reality (how it actually works):**

**Iteration 1: First Request**

```go
// messages before first iteration
messages := []openai.ChatCompletionMessage{
    {Role: "system", Content: "You are an autonomous DevOps agent."},
    {Role: "user", Content: "Out of space."},
}

// Send to model
resp1, _ := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
    Model:    openai.GPT3Dot5Turbo,
    Messages: messages,
    Tools:    tools,
})

msg1 := resp1.Choices[0].Message
// msg1.ToolCalls = [{ID: "call_1", Function: {Name: "check_disk_usage", Arguments: "{}"}}]

// Add assistant response to history
messages = append(messages, msg1)
// Now messages contains:
// [system, user, assistant(tool_call: check_disk_usage)]
```

**Iteration 2: Tool Execution and Result Return**

```go
// Execute tool
result1 := checkDiskUsage()  // "95% usage"

// Add result as message with role "tool"
messages = append(messages, openai.ChatCompletionMessage{
    Role:       "tool",
    Content:    result1,  // "95% usage"
    ToolCallID: "call_1",
})
// Now messages contains:
// [system, user, assistant(tool_call), tool("95% usage")]

// Send UPDATED history to model again
resp2, _ := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
    Model:    openai.GPT3Dot5Turbo,
    Messages: messages,  // Model sees check_disk_usage result!
    Tools:    tools,
})

msg2 := resp2.Choices[0].Message
// msg2.ToolCalls = [{ID: "call_2", Function: {Name: "clean_logs", Arguments: "{}"}}]

messages = append(messages, msg2)
// Now messages contains:
// [system, user, assistant(tool_call_1), tool("95%"), assistant(tool_call_2)]
```

**Iteration 3: Second Tool**

```go
// Execute second tool
result2 := cleanLogs()  // "Freed 20GB"

messages = append(messages, openai.ChatCompletionMessage{
    Role:       "tool",
    Content:    result2,  // "Freed 20GB"
    ToolCallID: "call_2",
})
// Now messages contains:
// [system, user, assistant(tool_call_1), tool("95%"), assistant(tool_call_2), tool("Freed 20GB")]

// Send again
resp3, _ := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
    Model:    openai.GPT3Dot5Turbo,
    Messages: messages,  // Model sees both results!
    Tools:    tools,
})

msg3 := resp3.Choices[0].Message
// msg3.ToolCalls = []  // Empty! Model decided to respond with text
// msg3.Content = "I cleaned logs, now there's enough space."

// This is final answer - exit loop
```

**Why this is not magic:**

1. **Model sees full history** — it doesn't "remember" the past, it sees it in `messages[]`
2. **Model sees tool result** — result is added as new message with role `tool`
3. **Model decides based on context** — seeing "95% usage", model understands that space needs to be freed
4. **Runtime manages loop** — code checks `len(msg.ToolCalls)` and decides whether to continue loop

**Key point:** Model didn't "decide itself" — it saw the result of `check_disk_usage` in context and generated the next tool call based on this context.

## Loop Implementation

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
        // This is final text answer
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
    // But this is not magic: we send updated history (with tool result)
    // to model again, and model sees result and decides what to do next
}
```

### Error Handling in Loop

**Important:** Don't forget to handle errors and add them to history! If a tool fails, the LLM should know about it and try something else.

**Proper error handling:**

```go
for _, toolCall := range msg.ToolCalls {
    result, err := executeTool(toolCall.Function.Name, toolCall.Function.Arguments)
    
    if err != nil {
        // Error is also a result! Add it to history
        result = fmt.Sprintf("Error: %v", err)
    }
    
    // Add result (or error) to history
    messages = append(messages, openai.ChatCompletionMessage{
        Role:       openai.ChatMessageRoleTool,
        Content:    result,  // Model will see error!
        ToolCallID: toolCall.ID,
    })
}
```

**What happens:**

1. Tool returns error: `Error: connection refused`
2. Error is added to history as tool result
3. Model sees error in context
4. Model can:
   - Try another tool
   - Report problem to user
   - Escalate problem

**Example:**

```
Iteration 1:
Action: check_database_status("prod")
Observation: Error: connection refused

Iteration 2 (model sees error):
Thought: "Database unavailable. I'll check network connectivity"
Action: ping_host("db-prod.example.com")
Observation: "Host is unreachable"

Iteration 3:
Thought: "Network unavailable. I'll report problem to user"
Action: [Final answer] "Database is unavailable. Check network connectivity."
```

**Anti-pattern:** Don't hide errors from the model!

```go
// BAD: Hide error
if err != nil {
    log.Printf("Error: %v", err)  // Only to log
    continue  // Skip tool
}

// GOOD: Show error to model
if err != nil {
    result := fmt.Sprintf("Error: %v", err)
    messages = append(messages, ...)  // Add to history
}
```

## Common Mistakes

### Mistake 1: Infinite Loop

**Symptom:** Agent repeats the same action infinitely.

**Cause:** No iteration limit and detection of repeating actions.

**Solution:**
```go
// GOOD: Iteration limit + stuck detection
for i := 0; i < maxIterations; i++ {
    // ...
    
    // Detection of repeating actions
    if lastNActionsAreSame(history, 3) {
        break
    }
}

// GOOD: Improve prompt
systemPrompt := `... If action doesn't help, try a different approach.`
```

### Mistake 2: Tool Result Not Added to History

**Symptom:** Agent doesn't see tool result and continues executing the same action.

**Cause:** Tool execution result not added to `messages[]`.

**Solution:**
```go
// BAD: Result not added
result := executeTool(toolCall)
// History not updated!

// GOOD: MUST add result!
messages = append(messages, openai.ChatCompletionMessage{
    Role:       openai.ChatMessageRoleTool,
    Content:    result,
    ToolCallID: toolCall.ID,  // Important for linking!
})
```

### Mistake 3: Agent Doesn't Stop

**Symptom:** Agent continues calling tools even when task is solved.

**Cause:** System Prompt doesn't instruct agent to stop when task is solved.

**Solution:**
```go
// GOOD: Add to System Prompt
systemPrompt := `... If task is solved, answer user with text. Don't call tools unnecessarily.`
```

## Mini-Exercises

### Exercise 1: Add Loop Detection

Implement a check that the last 3 actions are the same:

```go
func isStuck(history []ChatCompletionMessage) bool {
    // Check that last 3 actions are the same
    // ...
}
```

**Expected result:**
- Function returns `true` if last 3 actions are the same
- Function returns `false` otherwise

### Exercise 2: Add Logging

Log each loop iteration:

```go
fmt.Printf("[Iteration %d] Agent decided: %s\n", i, action)
fmt.Printf("[Iteration %d] Tool result: %s\n", i, result)
```

**Expected result:**
- Each iteration logged with number and action
- Tool results logged

## Completion Criteria / Checklist

✅ **Completed:**
- Agent executes loop autonomously
- Tool results added to history
- Agent stops when task is solved
- Has protection against infinite loops (iteration limit + detection)
- Tool errors handled and added to history

❌ **Not completed:**
- Agent doesn't continue loop after tool execution
- Tool results not visible to agent (not added to history)
- Agent loops infinitely (no protection)
- Agent doesn't stop when task is solved

## Connection with Other Chapters

- **Tools:** How tools are called and return results, see [Chapter 04: Tools](../04-tools-and-function-calling/README.md)
- **Memory:** How message history (`messages[]`) grows and is managed, see [Chapter 03: Agent Anatomy](../03-agent-architecture/README.md)
- **Safety:** How to stop loop for confirmation, see [Chapter 06: Safety](../06-safety-and-hitl/README.md)

## What's Next?

After studying autonomy, proceed to:
- **[06. Safety and Human-in-the-Loop](../06-safety-and-hitl/README.md)** — how to protect agent from dangerous actions

---

**Navigation:** [← Tools](../04-tools-and-function-calling/README.md) | [Table of Contents](../README.md) | [Safety →](../06-safety-and-hitl/README.md)
