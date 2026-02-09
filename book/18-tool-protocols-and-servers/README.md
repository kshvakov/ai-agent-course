# 18. Tool Protocols and Tool Servers

## Why This Chapter?

As agents grow, tools often turn into services. Instead of embedding everything into one binary, you can run tools as separate processes or services. That requires protocols for communication, versioning, and security.

This chapter covers tool server patterns: stdio, HTTP, gRPC, schema versioning, and authentication/authorization. It also covers two standard protocols: MCP (Model Context Protocol) for connecting agents to tools, and A2A (Agent-to-Agent) for agent-to-agent communication.

### Real-World Case Study

**Situation:** You have 50+ tools. Some are written in Go, some in Python, some are external services. Embedding all in one agent binary is impractical.

**Problem:**
- Different languages require different integration approaches
- Tool updates require agent redeployment
- No isolation between tools
- Hard to scale individual tools

**Solution:** Tool servers: each tool runs as a separate process/service. The agent talks to tools through a standard protocol (stdio, HTTP, or gRPC). Tools can be updated and scaled independently, and isolated for security.

## Theory in Simple Terms

### Tool Server Architecture

**Agent Runtime:**
- Manages conversation flow
- Calls tools through protocol
- Processes tool responses

**Tool Server:**
- Implements tool logic
- Provides protocol interface
- Can be a separate process/service

**Protocol:**
- Communication contract
- Request/response format
- Error handling

### Protocol Types

**1. stdio Protocol:**
- Tool runs as subprocess
- Communication through stdin/stdout
- Simple, good for local tools

**2. HTTP Protocol:**
- Tool runs as HTTP service
- REST API interface
- Good for distributed systems

**3. gRPC Protocol:**
- Tool runs as a gRPC service
- Strict contract via Protobuf (IDL)
- Type safety and backward compatibility of schemas
- Rich Go ecosystem: client/server code generation, interceptors, reflection
- Built-in mechanisms: TLS/mTLS, authentication via metadata, deadlines, retries, load balancing
- Observability: integration with tracing/metrics/logging
- A practical choice for production tool servers

## How It Works (Step by Step)

### Step 1: Tool Protocol Interface

```go
type ToolServer interface {
    ListTools() ([]ToolDefinition, error)
    ExecuteTool(name string, args map[string]any) (any, error)
}

type ToolDefinition struct {
    Name        string
    Description string
    Schema      json.RawMessage
    Version     string
}
```

### Step 2: stdio Protocol

```go
// Tool server reads from stdin, writes to stdout
type StdioToolServer struct {
    tools map[string]Tool
}

func (s *StdioToolServer) Run() {
    scanner := bufio.NewScanner(os.Stdin)
    for scanner.Scan() {
        var req ToolRequest
        json.Unmarshal(scanner.Bytes(), &req)
        
        result, err := s.ExecuteTool(req.Name, req.Args)
        
        resp := ToolResponse{
            Result: result,
            Error:  errString(err),
        }
        
        json.NewEncoder(os.Stdout).Encode(resp)
    }
}

type ToolRequest struct {
    Name string
    Args map[string]any
}

type ToolResponse struct {
    Result any
    Error  string
}
```

### Step 3: Agent Calls Tool Server

```go
func executeToolViaStdio(toolName string, args map[string]any) (any, error) {
    cmd := exec.Command("tool-server")
    stdin, _ := cmd.StdinPipe()
    stdout, _ := cmd.StdoutPipe()
    
    cmd.Start()
    
    req := ToolRequest{Name: toolName, Args: args}
    json.NewEncoder(stdin).Encode(req)
    stdin.Close()
    
    var resp ToolResponse
    json.NewDecoder(stdout).Decode(&resp)
    
    cmd.Wait()
    
    if resp.Error != "" {
        return nil, fmt.Errorf(resp.Error)
    }
    return resp.Result, nil
}
```

### Step 4: HTTP Protocol

```go
// Tool server as HTTP service
func (s *HTTPToolServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    var req ToolRequest
    json.NewDecoder(r.Body).Decode(&req)
    
    result, err := s.ExecuteTool(req.Name, req.Args)
    
    resp := ToolResponse{
        Result: result,
        Error:  errString(err),
    }
    
    json.NewEncoder(w).Encode(resp)
}
```

### Step 5: gRPC Protocol

gRPC provides a strict contract via Protocol Buffers. Service definition:

```protobuf
syntax = "proto3";

package tools.v1;

service ToolServer {
  rpc ListTools(ListToolsRequest) returns (ListToolsResponse);
  rpc ExecuteTool(ExecuteToolRequest) returns (ExecuteToolResponse);
}

message ListToolsRequest {
  string version = 1; // Protocol version
}

message ListToolsResponse {
  repeated ToolDefinition tools = 1;
}

message ExecuteToolRequest {
  string tool_name = 1;
  string version = 2;
  bytes arguments = 3; // JSON-serialized arguments
}

message ExecuteToolResponse {
  bytes result = 1;
  string error = 2;
}

message ToolDefinition {
  string name = 1;
  string description = 2;
  string schema = 3; // JSON Schema
  string version = 4;
  repeated string compatible_versions = 5;
}
```

**Advantages of gRPC for tool servers:**
- **Strict contract**: Protobuf guarantees type safety and schema evolution without breaking changes
- **Go ecosystem**: Automatic client/server generation, interceptors for authn/authz, health checks
- **Security**: Built-in TLS/mTLS support, authentication via metadata (tokens, API keys)
- **Reliability**: Deadlines, retries, load balancing at client level or via service mesh
- **Observability**: Integration with OpenTelemetry, gRPC metrics, structured logging

### Step 6: Schema Versioning

```go
type ToolDefinition struct {
    Name        string
    Version     string
    Schema      json.RawMessage
    CompatibleVersions []string
}

func (s *ToolServer) GetToolDefinition(name string, version string) (*ToolDefinition, error) {
    tool := s.tools[name]
    if tool.Version == version {
        return &tool, nil
    }
    
    // Check compatibility
    for _, v := range tool.CompatibleVersions {
        if v == version {
            return &tool, nil
        }
    }
    
    return nil, fmt.Errorf("incompatible version")
}
```

## MCP (Model Context Protocol)

### What Is MCP

MCP (Model Context Protocol) is an open standard from Anthropic for connecting LLM agents to tools and data. Instead of every agent writing its own integration with every service, MCP defines a single protocol. A tool written as an MCP server works with any agent that supports MCP.

Analogy: USB for AI tools. One connector — any device.

### Architecture

```
Host (IDE / chatbot / CI pipeline)
  └── MCP Client ──── JSON-RPC 2.0 ──── MCP Server
                                           ├── Resources  (read-only data)
                                           ├── Tools      (actions)
                                           └── Prompts    (prompt templates)
```

**Host** — the application running the agent (IDE, chatbot, CI/CD pipeline). A single host can connect to multiple MCP servers simultaneously.

**MCP Client** — a component of the host. Establishes a connection with the MCP server using JSON-RPC 2.0.

**MCP Server** — a separate process or service. Exposes tools, data, and templates through the standard protocol.

### Three Primitives

| Primitive | Analogy | What It Does | Example |
|-----------|---------|--------------|---------|
| **Resources** | GET | Read-only data. Agent requests — server returns | File contents, SQL query result, metrics |
| **Tools** | POST | Actions with side effects. Require confirmation | Create ticket, trigger deploy, send alert |
| **Prompts** | Template | Ready-made prompts for common tasks | Code review template, incident analysis |

### Transport

MCP supports two transport types:

**stdio** — the host launches the MCP server as a subprocess. Communication via stdin/stdout. Simple, no network required. Good for local tools: CLI utilities, file operations, IDE plugins.

**Streamable HTTP** — the client sends JSON-RPC requests over HTTP POST. The server can respond via SSE (Server-Sent Events) for streaming results. Good for remote servers and production environments.

### Example: MCP Server in Go

A minimal MCP server that exposes a tool for deploying services.

JSON-RPC 2.0 types — the foundation of the MCP protocol:

```go
type JSONRPCRequest struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      any             `json:"id,omitempty"`
    Method  string          `json:"method"`
    Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
    JSONRPC string    `json:"jsonrpc"`
    ID      any       `json:"id"`
    Result  any       `json:"result,omitempty"`
    Error   *RPCError `json:"error,omitempty"`
}

type RPCError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}
```

Define a tool via JSON Schema. The LLM uses the description and schema to generate arguments:

```go
type MCPTool struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    InputSchema json.RawMessage `json:"inputSchema"`
}

var deployTool = MCPTool{
    Name:        "deploy_service",
    Description: "Deploy a service to a specified environment",
    InputSchema: json.RawMessage(`{
        "type": "object",
        "properties": {
            "service": {"type": "string", "description": "Service name"},
            "env":     {"type": "string", "enum": ["staging", "production"]}
        },
        "required": ["service", "env"]
    }`),
}
```

Request handling. The MCP server responds to three key methods — `initialize`, `tools/list`, and `tools/call`:

```go
func handleRequest(req JSONRPCRequest) JSONRPCResponse {
    switch req.Method {
    case "initialize":
        // Handshake: server reports its capabilities
        return JSONRPCResponse{
            JSONRPC: "2.0", ID: req.ID,
            Result: map[string]any{
                "protocolVersion": "2025-03-26",
                "capabilities":    map[string]any{"tools": map[string]any{}},
                "serverInfo": map[string]any{
                    "name": "deploy-server", "version": "1.0.0",
                },
            },
        }

    case "tools/list":
        // Agent calls this on connect to discover available tools
        return JSONRPCResponse{
            JSONRPC: "2.0", ID: req.ID,
            Result:  map[string]any{"tools": []MCPTool{deployTool}},
        }

    case "tools/call":
        // Tool invocation: agent passes name and arguments
        var params struct {
            Name      string         `json:"name"`
            Arguments map[string]any `json:"arguments"`
        }
        json.Unmarshal(req.Params, &params)

        result, err := executeDeploy(params.Arguments)
        if err != nil {
            return JSONRPCResponse{
                JSONRPC: "2.0", ID: req.ID,
                Result: map[string]any{
                    "content": []map[string]any{{"type": "text", "text": err.Error()}},
                    "isError": true,
                },
            }
        }
        return JSONRPCResponse{
            JSONRPC: "2.0", ID: req.ID,
            Result: map[string]any{
                "content": []map[string]any{{"type": "text", "text": result}},
            },
        }

    default:
        return JSONRPCResponse{
            JSONRPC: "2.0", ID: req.ID,
            Error:   &RPCError{Code: -32601, Message: "method not found"},
        }
    }
}
```

Running via stdio. The server reads JSON-RPC messages from stdin and writes responses to stdout:

```go
func main() {
    scanner := bufio.NewScanner(os.Stdin)
    scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

    for scanner.Scan() {
        var req JSONRPCRequest
        if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
            continue
        }
        resp := handleRequest(req)
        json.NewEncoder(os.Stdout).Encode(resp)
    }
}
```

### Example: MCP Client

The client launches an MCP server as a subprocess and calls tools:

```go
type MCPClient struct {
    cmd    *exec.Cmd
    stdin  io.WriteCloser
    stdout *bufio.Scanner
    nextID int
}

func NewMCPClient(serverPath string) (*MCPClient, error) {
    cmd := exec.Command(serverPath)
    stdin, _ := cmd.StdinPipe()
    stdoutPipe, _ := cmd.StdoutPipe()

    if err := cmd.Start(); err != nil {
        return nil, fmt.Errorf("failed to start MCP server: %w", err)
    }

    client := &MCPClient{
        cmd:    cmd,
        stdin:  stdin,
        stdout: bufio.NewScanner(stdoutPipe),
    }

    // Initialization — mandatory first step
    _, err := client.call("initialize", map[string]any{
        "protocolVersion": "2025-03-26",
        "clientInfo":      map[string]any{"name": "my-agent", "version": "1.0.0"},
    })
    return client, err
}

func (c *MCPClient) call(method string, params any) (json.RawMessage, error) {
    c.nextID++
    req := JSONRPCRequest{
        JSONRPC: "2.0",
        ID:      c.nextID,
        Method:  method,
    }
    if params != nil {
        req.Params, _ = json.Marshal(params)
    }

    // Send request to server's stdin
    data, _ := json.Marshal(req)
    fmt.Fprintf(c.stdin, "%s\n", data)

    // Read response from server's stdout
    if !c.stdout.Scan() {
        return nil, fmt.Errorf("server did not respond")
    }
    var resp JSONRPCResponse
    json.Unmarshal(c.stdout.Bytes(), &resp)

    if resp.Error != nil {
        return nil, fmt.Errorf("RPC error %d: %s", resp.Error.Code, resp.Error.Message)
    }
    result, _ := json.Marshal(resp.Result)
    return result, nil
}
```

Using the client:

```go
func main() {
    client, err := NewMCPClient("./deploy-server")
    if err != nil {
        log.Fatal(err)
    }
    defer client.cmd.Process.Kill()

    // Get available tools
    tools, _ := client.call("tools/list", nil)
    fmt.Println("Tools:", string(tools))

    // Call a tool
    result, _ := client.call("tools/call", map[string]any{
        "name":      "deploy_service",
        "arguments": map[string]any{"service": "api-gateway", "env": "staging"},
    })
    fmt.Println("Result:", string(result))
}
```

### When to Use MCP vs Direct HTTP/gRPC

| Criterion | MCP | Direct HTTP/gRPC |
|-----------|-----|-------------------|
| **Agent uses a tool** | Yes — MCP is designed for this | Requires manual integration |
| **Tools for multiple agents** | One server — any MCP client | Each agent writes its own client |
| **High-load API** | No — not optimized for RPS | Yes — gRPC/HTTP is the better choice |
| **Complex business logic** | No — better as a separate service | Yes |
| **IDE and developer tools** | Yes — most IDEs support MCP | No |

**Rule:** If a tool is needed by an LLM agent — use MCP. If it's an API for other services — use HTTP/gRPC.

## A2A (Agent-to-Agent Protocol)

### What Is A2A

A2A (Agent-to-Agent) is an open protocol from Google for agent-to-agent communication. MCP solves "agent ↔ tool". A2A solves a different problem — "agent ↔ agent".

Why does this matter? When you have many agents built by different teams, you need a standard way to discover each other and delegate tasks. A2A provides this out of the box.

### Key Concepts

**Agent Card** — a JSON document describing an agent's capabilities. Published at `/.well-known/agent.json`. Any agent can fetch the card and understand what tasks another agent can handle.

**Task** — a unit of work. One agent creates a task, another executes it. A task has a lifecycle with clear statuses.

**Message** — communication within a task. Contains parts (Part): text, files, structured data.

**Artifact** — the result of task execution. The executing agent returns artifacts as they become ready.

### Task Lifecycle

```
submitted ──→ working ──→ completed
                │      ──→ failed
                │      ──→ canceled
                ▼
          input-required ──→ working ──→ ...
```

- **submitted** — task created, waiting to be processed
- **working** — agent is working on the task
- **input-required** — agent needs additional information from the caller
- **completed** — task done, artifacts ready
- **failed** — task ended with an error
- **canceled** — task canceled by the caller

### Example: Agent Card and A2A Server

Agent Card — describes the agent's capabilities:

```go
// Agent Card — published at /.well-known/agent.json
type AgentCard struct {
    Name         string       `json:"name"`
    Description  string       `json:"description"`
    URL          string       `json:"url"`
    Version      string       `json:"version"`
    Capabilities Capabilities `json:"capabilities"`
    Skills       []Skill      `json:"skills"`
}

type Capabilities struct {
    Streaming         bool `json:"streaming"`
    PushNotifications bool `json:"pushNotifications"`
}

// Skill — a specific task the agent can handle
type Skill struct {
    ID          string   `json:"id"`
    Name        string   `json:"name"`
    Description string   `json:"description"`
    InputModes  []string `json:"inputModes"`  // "text", "file", "data"
    OutputModes []string `json:"outputModes"`
}
```

Types for working with tasks:

```go
type Task struct {
    ID        string     `json:"id"`
    Status    TaskStatus `json:"status"`
    Messages  []Message  `json:"messages,omitempty"`
    Artifacts []Artifact `json:"artifacts,omitempty"`
}

type TaskStatus struct {
    State   string `json:"state"` // submitted, working, input-required, completed, failed
    Message string `json:"message,omitempty"`
}

type Message struct {
    Role  string `json:"role"` // "user" or "agent"
    Parts []Part `json:"parts"`
}

type Part struct {
    Type string `json:"type"` // "text", "file", "data"
    Text string `json:"text,omitempty"`
    Data any    `json:"data,omitempty"`
}

type Artifact struct {
    Name  string `json:"name"`
    Parts []Part `json:"parts"`
}
```

The A2A server processes tasks from other agents:

```go
type A2AServer struct {
    card  AgentCard
    tasks sync.Map // taskID → *Task
}

func (s *A2AServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    switch r.URL.Path {
    case "/.well-known/agent.json":
        // Discovery: any agent can fetch the card
        json.NewEncoder(w).Encode(s.card)

    case "/tasks/send":
        // Receive a task from another agent
        var req struct {
            ID      string  `json:"id"`
            Message Message `json:"message"`
        }
        json.NewDecoder(r.Body).Decode(&req)

        task := &Task{
            ID:       req.ID,
            Status:   TaskStatus{State: "working"},
            Messages: []Message{req.Message},
        }
        s.tasks.Store(req.ID, task)

        // Process the task asynchronously
        go s.processTask(task)

        json.NewEncoder(w).Encode(task)

    case "/tasks/get":
        // Check task status
        taskID := r.URL.Query().Get("id")
        if task, ok := s.tasks.Load(taskID); ok {
            json.NewEncoder(w).Encode(task)
        } else {
            http.Error(w, "task not found", http.StatusNotFound)
        }
    }
}

func (s *A2AServer) processTask(task *Task) {
    // Extract task text
    input := task.Messages[0].Parts[0].Text

    // Agent does its work...
    result := fmt.Sprintf("Analyzed: %s", input)

    // Update task with result
    task.Artifacts = []Artifact{
        {Name: "result", Parts: []Part{{Type: "text", Text: result}}},
    }
    task.Status = TaskStatus{State: "completed"}
}
```

Usage — the client discovers an agent and sends a task:

```go
func discoverAndDelegate(agentURL string, taskText string) (*Task, error) {
    // 1. Fetch Agent Card
    resp, _ := http.Get(agentURL + "/.well-known/agent.json")
    var card AgentCard
    json.NewDecoder(resp.Body).Decode(&card)
    resp.Body.Close()

    fmt.Printf("Agent: %s — %s\n", card.Name, card.Description)

    // 2. Send the task
    taskReq := map[string]any{
        "id": "task-001",
        "message": Message{
            Role:  "user",
            Parts: []Part{{Type: "text", Text: taskText}},
        },
    }
    body, _ := json.Marshal(taskReq)
    resp, _ = http.Post(agentURL+"/tasks/send", "application/json", bytes.NewReader(body))

    var task Task
    json.NewDecoder(resp.Body).Decode(&task)
    resp.Body.Close()

    // 3. Poll status until completion
    for task.Status.State == "working" {
        time.Sleep(time.Second)
        resp, _ = http.Get(agentURL + "/tasks/get?id=" + task.ID)
        json.NewDecoder(resp.Body).Decode(&task)
        resp.Body.Close()
    }

    return &task, nil
}
```

### When to Use A2A vs Supervisor/Worker Pattern

| Criterion | A2A | Supervisor/Worker |
|-----------|-----|-------------------|
| **Agents from different teams** | Yes — standard discovery protocol | No — requires shared code |
| **Different frameworks** | Yes — protocol is implementation-agnostic | No — shared framework |
| **Simple orchestration** | Overkill | Yes — simpler to implement |
| **Dynamic discovery** | Yes — via Agent Card | No — agents are statically defined |
| **One team, one repo** | Overkill | Yes — direct calls are simpler |

**Rule:** Use A2A when agents are built by independent teams and need to discover each other dynamically. For agents within one system, direct orchestration is enough (see [Chapter 07: Multi-Agent Systems](../07-multi-agent/README.md)).

## Protocol Comparison Table

| | stdio | HTTP | gRPC | MCP | A2A |
|---|---|---|---|---|---|
| **Use case** | Local tools | Distributed services | Production API | Tools for LLM agents | Agent-to-agent communication |
| **Latency** | Minimal (IPC) | Medium (network + HTTP/1.1) | Low (HTTP/2 + binary format) | Depends on transport (stdio or HTTP) | Medium (HTTP) |
| **Implementation complexity** | Low | Medium | High (protobuf, codegen) | Medium (JSON-RPC 2.0) | Medium-high |
| **Tool discovery** | None | Swagger / OpenAPI | Reflection, service mesh | `tools/list`, `resources/list` | Agent Card (`/.well-known/agent.json`) |
| **Streaming** | Line-by-line via stdout | SSE, WebSocket | Bidirectional streams | SSE (Streamable HTTP) | SSE |
| **Contract typing** | None (free-form JSON) | JSON Schema / OpenAPI | Strict (Protobuf IDL) | JSON Schema (for tools) | JSON Schema |
| **When to choose** | CLI utilities, IDE plugins | External APIs, webhooks | Microservices, high load | Tools for LLM agents | Multi-agent systems |

### Authentication: Examples

**HTTP — Bearer Token:**

```go
func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        if !strings.HasPrefix(token, "Bearer ") {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }

        apiKey := strings.TrimPrefix(token, "Bearer ")
        if !isValidKey(apiKey) {
            http.Error(w, "invalid token", http.StatusForbidden)
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

**MCP — Authentication via OAuth 2.1:**

MCP uses [OAuth 2.1](https://modelcontextprotocol.io/specification/2025-03-26/basic/authorization) (IETF draft-ietf-oauth-v2-1) for remote servers (Streamable HTTP transport). The client obtains a token through the standard OAuth flow and passes it in the `Authorization` header:

```go
// Authenticated MCP client for HTTP transport
type AuthenticatedMCPClient struct {
    serverURL string
    token     string
    client    *http.Client
}

func (c *AuthenticatedMCPClient) call(method string, params any) (json.RawMessage, error) {
    body, _ := json.Marshal(JSONRPCRequest{
        JSONRPC: "2.0",
        ID:      1,
        Method:  method,
        Params:  mustMarshal(params),
    })

    req, _ := http.NewRequest("POST", c.serverURL, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+c.token)

    resp, err := c.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusUnauthorized {
        return nil, fmt.Errorf("token expired, re-authentication required")
    }

    var rpcResp JSONRPCResponse
    json.NewDecoder(resp.Body).Decode(&rpcResp)

    if rpcResp.Error != nil {
        return nil, fmt.Errorf("RPC error: %s", rpcResp.Error.Message)
    }
    result, _ := json.Marshal(rpcResp.Result)
    return result, nil
}
```

For stdio transport, authentication is typically unnecessary — the server runs locally as a subprocess of the host.

## Common Errors

### Error 1: No Versioning

**Symptom:** Tool updates break the agent.

**Cause:** No versioning, agent expects old interface.

**Solution:** Version tool schemas, check compatibility.

### Error 2: No Authentication

**Symptom:** Unauthorized access to tools.

**Cause:** No authn/authz for tool servers.

**Solution:** Implement authentication (API keys, tokens).

## Mini-Exercises

### Exercise 1: Implement stdio Tool Server

Create a tool server that communicates via stdio:

```go
func main() {
    server := NewStdioToolServer()
    server.Run()
}
```

**Expected result:**
- Reads requests from stdin
- Executes tools
- Writes responses to stdout

## Completion Criteria / Checklist

**Completed:**
- [x] Understand tool server architecture
- [x] Can implement stdio protocol
- [x] Can implement HTTP protocol
- [x] Understand advantages of gRPC for production tool servers
- [x] Understand schema versioning
- [x] Understand MCP: architecture, three primitives, transport
- [x] Can write an MCP server and MCP client in Go
- [x] Understand A2A: Agent Card, task lifecycle
- [x] Know when to choose MCP, A2A, HTTP, or gRPC

**Not completed:**
- [ ] No versioning, updates break compatibility
- [ ] No authentication, security risk
- [ ] Using MCP for high-load APIs instead of gRPC
- [ ] Using A2A where direct orchestration would suffice

## Connection with Other Chapters

- **[Chapter 03: Tools and Function Calling](../03-tools-and-function-calling/README.md)** — Basics of tool execution
- **[Chapter 07: Multi-Agent Systems](../07-multi-agent/README.md)** — Agent orchestration patterns (Supervisor/Worker and others)
- **[Chapter 17: Security and Governance](../17-security-and-governance/README.md)** — Security for tool servers

## What's Next?

After understanding tool protocols, proceed to:
- **[19. Observability and Tracing](../19-observability-and-tracing/README.md)** — Learn production observability

