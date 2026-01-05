# Lab 12: Tool Server Protocol

## Goal
Learn to implement tool servers: stdio/HTTP protocols, schema versioning, and tool server architecture.

## Theory

### Tool Servers

As agents grow, tools become services:
- **Separate processes** — Tools run independently
- **Protocol communication** — stdio or HTTP
- **Schema versioning** — Handling tool updates
- **Isolation** — Tools isolated for security

### Why Tool Servers?

**Monolithic agent (Lab 01-09):**
- All tools in one process
- Tools compiled with agent
- Hard to update tools independently

**Tool server architecture (Lab 12):**
- Tools in separate processes
- Tools can be updated without rebuilding agent
- Better security (tools isolated)
- Tools can be written in different languages

### Protocols

**stdio Protocol:**
- Simple: read from stdin, write to stdout
- JSON request/response format
- Good for local tools

**HTTP Protocol:**
- REST API for tool execution
- Better for distributed systems
- Can be called from anywhere

### Schema Versioning

Tools evolve over time:
- Version 1.0: `check_status(hostname)`
- Version 2.0: `check_status(hostname, timeout)` — added parameter

**Versioning strategy:**
- Each tool has version
- Agent specifies required version
- Tool server checks compatibility
- Return error if versions don't match

See more: [Chapter 18: Tool Protocols and Tool Servers](../../book/18-tool-protocols-and-servers/README.md)

## Task

In `main.go` implement a tool server with protocol support.

### Part 1: stdio Protocol

Implement `StdioToolServer`:
- Read requests from stdin (JSON format)
- Execute tools
- Write responses to stdout (JSON format)

**Request format:**
```json
{
  "tool": "check_status",
  "version": "1.0",
  "arguments": {"hostname": "web-01"}
}
```

**Response format:**
```json
{
  "success": true,
  "result": "Server is ONLINE",
  "error": null
}
```

### Part 2: HTTP Protocol

Implement `HTTPToolServer`:
- HTTP endpoint for executing tools (POST /execute)
- JSON request/response format
- Error handling

**Endpoint:** `POST /execute`
**Request body:**
```json
{
  "tool": "check_status",
  "version": "1.0",
  "arguments": {"hostname": "web-01"}
}
```

**Response:**
```json
{
  "success": true,
  "result": "Server is ONLINE",
  "error": null
}
```

### Part 3: Schema Versioning

Implement `ToolDefinition` with versioning:
- Version field (e.g., "1.0", "2.0")
- List of compatible versions
- Version checking before execution

**Example:**
```go
type ToolDefinition struct {
    Name            string
    Version         string
    CompatibleWith  []string  // ["1.0", "1.1", "2.0"]
    Description     string
    Parameters      json.RawMessage
}
```

### Part 4: Agent Integration

Implement tool client on agent side:
- Call tool server via protocol (stdio or HTTP)
- Handle version compatibility
- Error handling

**Agent workflow:**
1. Agent needs to call tool
2. Agent sends request to tool server (with version)
3. Tool server checks version compatibility
4. Tool server executes tool
5. Tool server returns result
6. Agent receives result and continues

## Important

- Always check version compatibility
- Handle protocol errors gracefully
- Support both stdio and HTTP protocols
- Tools should be isolated (separate processes)

## Completion Criteria

✅ **Completed:**
- stdio protocol implemented
- HTTP protocol implemented
- Schema versioning works
- Agent can call tool server
- Version compatibility checked
- Error handling implemented

❌ **Not completed:**
- No versioning, updates break compatibility
- Protocol not implemented
- Agent cannot call tool server
- Version compatibility not checked

---

**Next step:** After completing Lab 12 you've mastered advanced agent patterns! Consider optional advanced topics from the textbook: [Chapter 17: Security and Governance](../../book/17-security-and-governance/README.md) or [Chapter 23: Evals in CI/CD](../../book/23-evals-in-cicd/README.md).
