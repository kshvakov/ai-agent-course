# Lab 08: Agent Team (Multi-Agent)

## Goal
Create a system of multiple specialized agents managed by a main agent (Supervisor). Implement Supervisor/Worker pattern with context isolation.

## Theory

### Problem: One Agent "Jack of All Trades"

One agent with many tools often gets confused. Context overflows, agent may mix up tools or make wrong decisions.

**Solution:** Divide responsibilities among specialized agents.

### Supervisor Pattern (Boss-Subordinate)

**Architecture:**

- **Supervisor:** Main brain. Doesn't have infrastructure tools, but knows who can do what.
- **Workers:** Specialized agents with narrow tool sets.

**Context isolation:** Worker doesn't see all Supervisor conversation, only its task. This saves tokens and focuses attention.

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

## Task

In `main.go` implement a Multi-Agent system.

### Part 1: Defining Tools for Supervisor

Supervisor has tools for calling specialists:

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

### Part 2: Defining Tools for Workers

```go
netTools := []openai.Tool{
    {Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: "ping", Description: "Ping a host"}},
}

dbTools := []openai.Tool{
    {Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: "run_sql", Description: "Run SQL query"}},
}
```

### Part 3: Worker Launch Function

Implement function `runWorkerAgent`, which:
- Creates **new** dialogue context for worker (isolation!)
- Runs simple agent loop (usually 1-2 steps)
- Returns worker's final answer

### Part 4: Supervisor Loop

Implement Supervisor loop, which:
- Receives task from user
- Decides which specialist to delegate to
- Calls corresponding Worker
- Gets answer and adds it to its history
- Collects results and responds to user

### Test Scenario

Run system with prompt: *"Check if DB server db-host.example.com is accessible, and if yes — find out PostgreSQL version"*

**Expected:**
- Supervisor delegates to Network Specialist → checks reachability
- Supervisor delegates to DB Specialist → finds out version
- Supervisor collects results and responds to user

## Important

- **Context isolation:** Worker must not see Supervisor context
- **Returning results:** Worker answers must be added to Supervisor history (role: "tool")
- **Limits:** Set iteration limit for Workers (usually 3-5)

## Completion Criteria

✅ **Completed:**
- Supervisor delegates tasks to Workers
- Workers work in isolation (don't see Supervisor context)
- Worker answers returned to Supervisor
- Supervisor collects results and responds to user
- Code compiles and works

❌ **Not completed:**
- Supervisor doesn't delegate tasks
- Workers see Supervisor context
- Worker answers not returned
- System loops infinitely

---

**Congratulations!** You've completed the course. Now you can build production AI agents.
