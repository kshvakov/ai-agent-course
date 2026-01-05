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

See more: [Chapter 18: Tool Protocols and Tool Servers](../../book/18-tool-protocols-and-servers/README.md)

## Task

In `main.go` implement a tool server with protocol support.

### Part 1: stdio Protocol

Implement `StdioToolServer`:
- Read requests from stdin
- Execute tools
- Write responses to stdout

### Part 2: HTTP Protocol

Implement `HTTPToolServer`:
- HTTP endpoint for executing tools
- JSON request/response format
- Error handling

### Part 3: Schema Versioning

Implement `ToolDefinition` with versioning:
- Version field
- List of compatible versions
- Version checking

### Part 4: Agent Integration

Implement tool client on agent side:
- Call tool server via protocol
- Handle version compatibility
- Error handling

## Completion Criteria

✅ **Completed:**
- stdio protocol implemented
- HTTP protocol implemented
- Schema versioning works
- Agent can call tool server
- Version compatibility checked

❌ **Not completed:**
- No versioning, updates break compatibility
- Protocol not implemented
- Agent cannot call tool server

---

**Next step:** After completing Lab 12 you've mastered advanced agent patterns! Consider optional advanced topics from the textbook: [Chapter 17: Security and Governance](../../book/17-security-and-governance/README.md) or [Chapter 23: Evals in CI/CD](../../book/23-evals-in-cicd/README.md).
