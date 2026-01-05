# Method Guide: Lab 05 — Human-in-the-Loop

## Why Is This Needed?

Autonomy doesn't mean permissiveness. There are two scenarios when an agent **must** return control to a human:

1. **Clarification:** User set a task unclearly
2. **Confirmation:** Action is too dangerous

### Real-World Case Study

**Situation 1 (Clarification):**
- User: "Send email to boss"
- Agent: "What's the subject and email text?" ← Agent asks for clarification
- User: "Subject: Report, text: All done"
- Agent: [Sends email]

**Situation 2 (Confirmation):**
- User: "Delete prod database"
- Agent: "Are you sure? This will delete all data in production. Type 'yes' to confirm." ← Agent requests confirmation
- User: "yes"
- Agent: [Deletes database]

**Without Human-in-the-Loop:** Agent could delete the database without confirmation, leading to disaster.

## Theory in Simple Terms

### Nested Loops

We use a nested loop structure:

- **Outer loop (`While True`):** Handles communication with user. Reads `stdin`.
- **Inner loop (Agent Loop):** Handles "thinking". Loops while agent calls tools. As soon as agent outputs text — we exit to outer loop.

**Diagram:**

```
Outer loop (Chat):
  Read user input
  Inner loop (Agent):
    While agent calls tools:
      Execute tool
      Continue inner loop
    If agent responded with text:
      Show to user
      Exit inner loop
  Wait for next user input
```

### System Prompt as Safety Rules

We explicitly write in the system prompt: *"Always ask for explicit confirmation before deleting anything"*.

LLM (especially GPT-4) follows this rule well. Instead of generating `ToolCall("delete_db")` it generates text *"Are you sure you want to delete...?"*.

Since this is text, the inner loop breaks, and the question is shown to the user.

### Continuing Conversation

When the user responds *"Yes"*, we add it to history and run the agent again. Now it has in context:
1. User: "Delete DB"
2. Assistant: "Are you sure?"
3. User: "Yes"

The agent sees confirmation and this time generates `ToolCall("delete_db")`.

## Execution Algorithm

### Step 1: Defining Critical Tools

```go
tools := []openai.Tool{
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name:        "delete_db",
            Description: "Delete a database by name. DANGEROUS ACTION.",
            // ...
        },
    },
}
```

**Important:** In `Description` indicate that the action is dangerous.

### Step 2: System Prompt with Safety Rules

```go
systemPrompt := `You are a helpful assistant.
IMPORTANT:
1. Always ask for explicit confirmation before deleting anything.
2. If user parameters are missing, ask clarifying questions.`
```

### Step 3: Nested Loops

```go
// Outer loop (Chat)
for {
    // Read user input
    input := readUserInput()
    messages = append(messages, userMessage)
    
    // Inner loop (Agent)
    for {
        resp := client.CreateChatCompletion(...)
        msg := resp.Choices[0].Message
        messages = append(messages, msg)
        
        if len(msg.ToolCalls) == 0 {
            // Agent responded with text (question or final answer)
            fmt.Println("Agent:", msg.Content)
            break  // Exit inner loop
        }
        
        // Execute tools
        for _, toolCall := range msg.ToolCalls {
            result := executeTool(toolCall)
            messages = append(messages, toolResult)
        }
        // Continue inner loop
    }
    // Wait for next user input
}
```

## Common Errors

### Error 1: Agent Doesn't Ask for Confirmation

**Symptom:** Agent immediately deletes database without confirmation.

**Cause:** System Prompt not strict enough or model ignores it.

**Solution:**
1. Strengthen System Prompt: "CRITICAL: Never delete without explicit confirmation"
2. Use a stronger model (GPT-4 instead of GPT-3.5)
3. Add Few-Shot examples to prompt

### Error 2: Agent Doesn't Clarify Parameters

**Symptom:** Agent tries to call tool with incomplete arguments.

**Example:**
```
User: "Send email"
Agent: [Tries to call send_email without subject and text]
```

**Solution:**
```go
// In System Prompt:
"If required parameters are missing, ask the user for them. Do not guess."
```

### Error 3: Confirmation Doesn't Work

**Symptom:** User confirmed, but agent asks again.

**Cause:** Confirmation not added to history or added incorrectly.

**Solution:**
```go
// After user confirmation:
messages = append(messages, ChatCompletionMessage{
    Role:    "user",
    Content: "yes",  // Confirmation
})
// Now agent will see confirmation in context
```

## Mini-Exercises

### Exercise 1: Add Risk Scoring

Implement a function that determines action risk level:

```go
func calculateRisk(toolName string) float64 {
    risks := map[string]float64{
        "delete_db": 0.9,
        "restart_service": 0.3,
        "read_logs": 0.0,
    }
    return risks[toolName]
}
```

### Exercise 2: Different Confirmation Levels

Implement different confirmation types for different risk levels:

```go
if risk > 0.8 {
    // Critical action: require explicit "yes"
} else if risk > 0.5 {
    // Medium risk: simple confirmation is enough
} else {
    // Low risk: can execute without confirmation
}
```

## Completion Criteria

✅ **Completed:**
- Agent asks for confirmation before dangerous actions
- Agent clarifies parameters if they're missing
- Confirmation works correctly
- Nested loops implemented correctly

❌ **Not completed:**
- Agent doesn't ask for confirmation
- Agent doesn't clarify parameters
- Confirmation doesn't work

---

**Next step:** After successfully completing Lab 05, proceed to [Lab 06: Incident Management](../lab06-incident/README.md)
