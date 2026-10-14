# Manual: Lab 08 — Multi-Agent Systems

## Why This Lab?

A single "jack-of-all-trades" agent often gets confused with tools. It's more efficient to divide responsibilities: create a team of narrow specialists managed by a main agent (Supervisor).

### Real-World Case Study

**Situation:** User asks: "Check if DB server is accessible, and if yes — find out database version."

**Without Multi-Agent:**
- One agent must know both `ping` and SQL
- Context overflows
- Agent may mix up tools

**With Multi-Agent:**
- Supervisor: Distributes tasks to specialists
- Network Specialist: Knows only `ping`
- DB Specialist: Knows only SQL
- Each specialist focuses on their task

**Difference:** Context isolation and specialization improve reliability.

## Theory in Simple Terms

### Supervisor Pattern (Boss-Subordinate)

**Architecture:**

- **Supervisor:** Main brain. Doesn't have tools, but knows who can do what.
- **Workers:** Specialized agents with narrow tool sets.

**Context isolation:** A Worker doesn't see the entire Supervisor conversation, only its task. This saves tokens and focuses attention.

**Example:**

```
Supervisor receives: "Check if DB server is accessible, and if yes — find out version"

Supervisor thinks:
- First need to check network → delegate to Network Specialist
- Then need to check DB → delegate to DB Specialist

Network Specialist receives: "Check reachability of db-host.example.com"
→ Calls ping("db-host.example.com")
→ Returns: "Host is reachable"

DB Specialist receives: "What PostgreSQL version on db-host?"
→ Calls sql_query("SELECT version()")
→ Returns: "PostgreSQL 15.2"

Supervisor collects results and responds to user
```

### Recursion and Isolation

Technically, calling an agent is just a function call. Inside function `runWorkerAgent` we create a **new** dialogue context (new `messages` array). A Worker has its own short memory and doesn't see the Supervisor-user conversation (context encapsulation).

## Execution Algorithm

### Step 1: Defining Tools for Supervisor

```go
supervisorTools := []llm.ToolDef{
    {
        Name: "ask_network_expert",
        Description: "Ask the network specialist about connectivity, pings, ports.",
        Parameters: json.RawMessage(`{
            "type": "object",
            "properties": {"question": {"type": "string"}},
            "required": ["question"]
        }`),
    },
    {
        Name: "ask_database_expert",
        Description: "Ask the DB specialist about SQL, schemas, data.",
        // ...
    },
}
```

**Important:** Supervisor tools are functions for calling other agents!

### Step 2: Defining Tools for Workers

```go
netTools := []llm.ToolDef{
    {Name: "ping", Description: "Ping a host"},
}

dbTools := []llm.ToolDef{
    {Name: "run_sql", Description: "Run SQL query"},
}
```

### Step 3: Worker Launch Function

```go
func runWorkerAgent(role, prompt, question string, tools []llm.ToolDef, client llm.Client, model string) string {
    // Create NEW context for worker
    messages := []llm.Message{
        {Role: "system", Content: prompt},
        {Role: "user", Content: question},
    }
    
    // Simple loop for worker (usually 1-2 steps)
    for i := 0; i < 5; i++ {
        resp, _ := client.Complete(ctx, llm.Request{
            Model: model, // From LLM_MODEL (default glm-5.3)
            Messages: messages,
            Tools: tools,
        })
        msg := resp.Choices[0].Message
        messages = append(messages, msg)
        
        if len(msg.ToolCalls) == 0 {
            return msg.Content  // Return worker's final answer
        }
        
        // Execute worker tools
        for _, tc := range msg.ToolCalls {
            result := executeWorkerTool(tc.Name)
            messages = append(messages, toolResult)
        }
    }
    return "Worker failed."
}
```

### Step 4: Supervisor Loop

```go
for i := 0; i < 10; i++ {
    resp, err := client.Complete(ctx, supervisorRequest)
    msg := resp.Choices[0].Message
    messages = append(messages, msg)
    
    if len(msg.ToolCalls) == 0 {
        fmt.Printf("Supervisor: %s\n", msg.Content)
        break
    }
    
    for _, tc := range msg.ToolCalls {
        var workerResponse string
        if tc.Name == "ask_network_expert" {
            workerResponse = runWorkerAgent("NetworkAdmin", "You are a Network Admin", question, netTools, client, model)
        } else if tc.Name == "ask_database_expert" {
            workerResponse = runWorkerAgent("DBAdmin", "You are a DBA", question, dbTools, client, model)
        }
        
        // Return worker answer to Supervisor
        messages = append(messages, llm.Message{
            Role: "tool",
            Content: workerResponse,
            ToolCallID: tc.ID,
        })
    }
}
```

## Common Errors

### Error 1: Worker Receives Supervisor Context

**Symptom:** The Worker receives all Supervisor history, causing context overflow.

**Cause:** You're passing Supervisor `messages` to Worker.

**Solution:**
```go
// BAD
runWorkerAgent(supervisorMessages, ...)  // Worker receives all history!

// GOOD
runWorkerAgent([]llm.Message{systemMsg, questionMsg}, ...)  // Only its task
```

### Error 2: Supervisor Doesn't Receive Worker Answer

**Symptom:** Supervisor calls Worker but doesn't receive the result.

**Cause:** Worker answer not added to Supervisor history.

**Solution:**
```go
// MUST add worker answer:
messages = append(messages, llm.Message{
    Role: "tool",
    Content: workerResponse,  // Supervisor must see this!
    ToolCallID: tc.ID,
})
```

### Error 3: Worker Loops Infinitely

**Symptom:** Worker executes loop infinitely.

**Cause:** No iteration limit for Worker.

**Solution:**
```go
for i := 0; i < 5; i++ {  // Limit for Worker
    // ...
}
```

## Mini-Exercises

### Exercise 1: Add Third Specialist

Create Security Specialist with tools `query_siem` and `check_ip_reputation`.

### Exercise 2: Add Logging

Log who does what:

```go
fmt.Printf("[Supervisor] Delegating to: %s\n", workerName)
fmt.Printf("[Worker: %s] Executing: %s\n", workerName, action)
fmt.Printf("[Worker: %s] Result: %s\n", workerName, result)
```

## Completion Criteria

✅ **Completed:**
- Supervisor delegates tasks to Workers
- Workers work in isolation
- Worker answers returned to Supervisor
- Supervisor collects results and responds to user

❌ **Not completed:**
- Supervisor doesn't delegate tasks
- Workers see Supervisor context
- Worker answers not returned

---

**Congratulations!** You've completed the course. Now you can build production AI agents.
