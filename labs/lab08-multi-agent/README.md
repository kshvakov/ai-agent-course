# Lab 08: Agent Team (Multi-Agent)

## Goal
Create a system of multiple specialized agents managed by a main agent (Supervisor). Implement the Supervisor/Worker pattern with context isolation.

## Theory

### Problem: One Agent "Jack of All Trades"

One agent with many tools often gets confused. Context overflows, the agent may mix up tools or make wrong decisions.

**Solution:** Divide responsibility among specialized agents.

### Supervisor Pattern (Boss-Subordinate)

**Architecture:**

- **Supervisor:** The main brain. Doesn't have tools for infrastructure work, but knows who can do what.
- **Workers:** Specialized agents with a narrow set of tools.

**Context isolation:** A Worker doesn't see all of the Supervisor's conversation, only its own task. This saves tokens and focuses attention.

**Example:**

```
Supervisor receives: "Check if DB server is accessible, and if yes — find out the version"

Supervisor thinks:
- First need to check network → delegate to Network Specialist
- Then need to check DB → delegate to DB Specialist

Network Specialist receives: "Check accessibility of db-host.example.com"
→ Calls ping("db-host.example.com")
→ Returns: "Host is reachable"

DB Specialist receives: "What PostgreSQL version on db-host?"
→ Calls sql_query("SELECT version()")
→ Returns: "PostgreSQL 15.2"

Supervisor collects results and responds to user
```

## Assignment

In `main.go`, implement a Multi-Agent system.

### Part 1: Define Tools for Supervisor

The Supervisor has tools for calling specialists:

```go
supervisorTools := []openai.Tool{
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name: "ask_network_expert",
            Description: "Ask the network specialist about connectivity, pings, ports.",
            // ...
        },
    },
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name: "ask_database_expert",
            Description: "Ask the DB specialist about SQL, schemas, data.",
            // ...
        },
    },
}
```

### Part 2: Define Tools for Workers

```go
netTools := []openai.Tool{
    {Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: "ping", Description: "Ping a host"}},
}

dbTools := []openai.Tool{
    {Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: "run_sql", Description: "Run SQL query"}},
}
```

### Part 3: Worker Launch Function

Implement the `runWorkerAgent` function that:
- Creates a **new** dialogue context for the worker (isolation!)
- Runs a simple agent loop (usually 1-2 steps)
- Returns the worker's final answer

### Part 4: Supervisor Loop

Implement the Supervisor loop that:
- Receives a task from the user
- Decides which specialist to delegate to
- Calls the corresponding Worker
- Receives the answer and adds it to its history
- Collects results and responds to the user

### Testing Scenario

Run the system with the prompt: *"Check if DB server db-host.example.com is accessible, and if yes — find out PostgreSQL version"*

**Expected:**
- Supervisor delegates to Network Specialist → checks accessibility
- Supervisor delegates to DB Specialist → finds out version
- Supervisor collects results and responds to user

## Important

- **Context isolation:** Worker should not see Supervisor's context
- **Returning results:** Worker responses should be added to Supervisor's history (role: "tool")
- **Limits:** Set iteration limits for Workers (usually 3-5)

## Completion Criteria

✅ **Completed:**
- Supervisor delegates tasks to Workers
- Workers work in isolation (don't see Supervisor's context)
- Worker responses are returned to Supervisor
- Supervisor collects results and responds to user
- Code compiles and works

❌ **Not completed:**
- Supervisor doesn't delegate tasks
- Workers see Supervisor's context
- Worker responses aren't returned
- System loops infinitely

---

**Congratulations!** You've completed the course. Now you can build production AI agents.
