# 18. Tool Protocols and Tool Servers

## Why does this matter?

As agents grow, tools often turn into services. Instead of embedding tool code directly into the agent, you can run tools as separate processes or services. That requires protocols for communication, versioning, and security.

In this chapter we'll look at tool server patterns: stdio, HTTP APIs, gRPC, schema versioning, and authentication/authorization. We'll also cover two standard protocols: MCP (Model Context Protocol) for connecting agents to tools, and A2A (Agent-to-Agent) for agents talking to each other.

### A real-world case

**Situation:** You have 50+ tools. Some are written in Go, some in Python, and some are external services. Embedding all of them into a single agent binary is impractical.

**Problems:**
- Different languages require different integration approaches
- Tool updates require redeploying the agent
- No isolation between tools
- It's hard to scale individual tools

**Solution:** Tool servers: each tool runs as a separate process/service. The agent communicates over a standard protocol (stdio, HTTP, or gRPC). Tools can be updated and scaled independently, and isolated for security.

> **Junior employee model.** Authn/authz at the tool-server level is an "office badge." The mere fact that the agent knows the service URL doesn't give it the right to use it: you need a token, a restricted set of operations, and auditing. One tool server = one department: its own access level, its own logs, its own area of responsibility. MCP/A2A don't override these rules — they standardize how they are applied. See [Preface → Mental Model](../00-preface/README.md#mental-model-an-agent-is-a-new-employee) and [Ch. 17: Security and Governance](../17-security-and-governance/README.md).

## The theory in plain words

### Tool server architecture

**Agent Runtime:**
- Manages the conversation flow
- Calls tools via a protocol
- Handles tool responses

**Tool Server:**
- Implements the tool logic
- Exposes a protocol interface
- Can be a separate process/service

**Protocol:**
- The communication contract
- Request/response format
- Error handling

### Protocol types

**1. stdio Protocol:**
- The tool runs as a subprocess
- Communication over stdin/stdout
- Simple; good for local tools

**2. HTTP Protocol:**
- The tool runs as an HTTP service
- REST API interface
- Good for distributed systems

**3. gRPC Protocol:**
- The tool runs as a gRPC service
- A strict contract via Protobuf (IDL)
- Type safety and backward-compatible schema evolution
- A rich ecosystem in Go: client/server code generation, interceptors, reflection
- Built-in mechanisms: TLS/mTLS, authentication via metadata, deadlines, retries, load balancing
- Observability: integration with tracing/metrics/logging
- A practical choice for production tool servers

## How it works (step by step)

### Step 1: The tool protocol interface

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

### Step 2: Stdio protocol

```go
// The tool server reads from stdin, writes to stdout
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

### Step 3: The agent calls the tool server

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

### Step 4: HTTP protocol

```go
// The tool server as an HTTP service
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

### Step 5: gRPC protocol

gRPC provides a strict contract via Protocol Buffers. The service definition:

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
- **Go ecosystem**: automatic client/server generation, interceptors for authn/authz, health checks
- **Security**: built-in TLS/mTLS support, authentication via metadata (tokens, API keys)
- **Reliability**: deadlines, retries, load balancing at the client level or via a service mesh
- **Observability**: integration with OpenTelemetry, gRPC metrics, structured logging

### Step 6: Schema versioning

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

### What is MCP

MCP (Model Context Protocol) is an open standard from Anthropic for connecting LLM agents to tools and data. Instead of every agent writing its own integration with every service, MCP defines a single protocol. A tool written as an MCP server works with any agent that supports MCP.

Analogy: USB for AI tools. One connector — any device.

### Architecture

```
Host (IDE / chatbot / CI pipeline)
  └── MCP Client ──── JSON-RPC 2.0 ──── MCP Server
                                           ├── Resources  (data to read)
                                           ├── Tools      (actions)
                                           └── Prompts    (prompt templates)
```

**Host** — the application the agent runs in (an IDE, a chatbot, a CI/CD pipeline). One host can connect to several MCP servers at once.

**MCP Client** — a component of the host. It establishes a connection to the MCP server over JSON-RPC 2.0.

**MCP Server** — a separate process or service. It exposes tools, data, and templates through a standard protocol.

### The three primitives

| Primitive | Analogue | What it does | Example |
|-----------|----------|--------------|---------|
| **Resources** | GET | Data to read. The agent requests it — the server returns it | File contents, a SQL query result, metrics |
| **Tools** | POST | Actions with side effects. They require confirmation | Create a ticket, start a deploy, send an alert |
| **Prompts** | Template | Ready-made prompts for common tasks | A code review template, incident analysis |

### Transport

MCP supports two types of transport:

**stdio** — the host launches the MCP server as a subprocess. Communication over stdin/stdout. Simple, no network required. Suitable for local tools: CLI utilities, file operations, IDE plugins.

**Streamable HTTP** — the client sends JSON-RPC requests over HTTP POST. The server can respond via SSE (Server-Sent Events) to stream results. Suitable for remote servers and production environments.

### Example: An MCP server in Go

A minimal MCP server that exposes a tool for deploying services.

The JSON-RPC 2.0 types — the foundation of the MCP protocol:

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

We define the tool via JSON Schema. The LLM uses the description and schema to generate arguments:

```go
type MCPTool struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    InputSchema json.RawMessage `json:"inputSchema"`
}

var deployTool = MCPTool{
    Name:        "deploy_service",
    Description: "Deploy a service to the specified environment",
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

Handling requests. The MCP server answers three key methods — `initialize`, `tools/list`, and `tools/call`:

```go
func handleRequest(req JSONRPCRequest) JSONRPCResponse {
    switch req.Method {
    case "initialize":
        // Handshake: the server reports the protocol version and its capabilities.
        // The spec version is negotiated during the handshake — use the current one
        return JSONRPCResponse{
            JSONRPC: "2.0", ID: req.ID,
            Result: map[string]any{
                "protocolVersion": "YYYY-MM-DD",
                "capabilities":    map[string]any{"tools": map[string]any{}},
                "serverInfo": map[string]any{
                    "name": "deploy-server", "version": "1.0.0",
                },
            },
        }

    case "tools/list":
        // The agent calls this on connection to discover the available tools
        return JSONRPCResponse{
            JSONRPC: "2.0", ID: req.ID,
            Result:  map[string]any{"tools": []MCPTool{deployTool}},
        }

    case "tools/call":
        // Tool call: the agent passes the name and arguments
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

Running over stdio. The loop here is the same as in the stdio Tool Server from the start of the chapter: the scanner reads a line from stdin, `json.Unmarshal` parses the request, the handler builds a response, and the encoder writes it to stdout. MCP doesn't invent a new transport — it merely wraps messages in JSON-RPC 2.0. The differences from a hand-rolled protocol: a mandatory handshake (`initialize`) with negotiation of the spec version before the first call, standard methods (`tools/list`, `tools/call`, `resources/list`) instead of a free-form format, and a content-based response format (`content` + `isError` instead of the result/error pair).

### Example: An MCP client

The client launches the MCP server as a subprocess and calls tools:

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

    // Initialization is a mandatory first step.
    // The protocol version is negotiated in the handshake — substitute the current spec version
    _, err := client.call("initialize", map[string]any{
        "protocolVersion": "YYYY-MM-DD",
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

    // Send the request to the server's stdin
    data, _ := json.Marshal(req)
    fmt.Fprintf(c.stdin, "%s\n", data)

    // Read the response from the server's stdout
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

    // Get the list of tools
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

### When to use MCP vs direct HTTP/gRPC

| Criterion | MCP | Direct HTTP/gRPC |
|----------|-----|-------------------|
| **Agent consumes the tool** | Yes — MCP is built for this | Requires manual integration |
| **Tools for multiple agents** | One server — any MCP client | Each agent writes its own client |
| **Network, scale, language agnosticism** | Conditional: stdio — local and simple, Streamable HTTP — for remote servers | Yes — HTTP/gRPC when you need a full-fledged API for other services |
| **Complex business logic** | No — better as a separate service | Yes |
| **IDEs and developer tools** | Yes — most IDEs support MCP | No |

**Rule of thumb:** If an LLM agent needs the tool — use MCP. If it's an API for other services — use HTTP/gRPC.

### MCP vs skills vs regular tools

Three mechanisms solve different problems — don't substitute one for another:

- **Regular tools** ([ch. 03](../03-tools-and-function-calling/README.md)) — the default for your own operations: deploys, queries against your own database, internal APIs. You control the code, the schema and the result.
- **MCP** — when integrating third-party systems or when you want a ready-made ecosystem of servers: browser, file system, ticket tracker. One protocol — any server works with any agent.
- **Skills** ([ch. 09](../09-agent-architecture/README.md#skills)) — when knowledge matters more than code: instruction packages in `SKILL.md` format with progressive disclosure; the agent loads them only when the task calls for it.

A word of caution: **20+ connected MCP servers is the same disease as 20+ hand-written wrappers**. Each server brings tools whose names and descriptions eat into the context budget ([ch. 03](../03-tools-and-function-calling/README.md), [ch. 13](../13-context-engineering/README.md)). Connect only the servers whose tools the agent actually needs; for everything else look toward skills and progressive disclosure — load on demand, not all at once.

> **The FilteredContent invariant for large results.** MCP tool results can be bulky: file contents, table dumps, search output. The invariant from [ch. 09](../09-agent-architecture/README.md) applies here too: the full result goes into `Content` (for UI and audit), while the model's context receives a filtered version. An MCP server doesn't know your context budget — the host does the filtering.

## A2A (Agent-to-Agent Protocol)

### What is A2A

A2A (Agent-to-Agent) is an open protocol from Google for agents to interact with each other. MCP solves the problem "agent ↔ tool". A2A solves a different problem — "agent ↔ agent".

Why is this needed? When there are many agents built by different teams, you need a standard way to discover each other and delegate tasks. A2A provides this out of the box.

### Key concepts

**Agent Card** — a discovery document in JSON format describing an agent's capabilities: name, description, supported input/output modes, skills. It is published according to the rules of the current A2A version (see [a2a-protocol.org](https://a2a-protocol.org/)); any agent can fetch the card and learn what tasks another agent can solve.

**Task** — a unit of work. One agent creates the task, another executes it. A task has a life cycle with well-defined statuses.

**Message** — communication within a task. It contains parts (Part): text, files, structured data.

**Artifact** — the result of executing a task. The executing agent returns artifacts as they become ready.

### Task life cycle

```
submitted ──→ working ──→ completed
                │      ──→ failed
                │      ──→ canceled
                ▼
          input-required ──→ working ──→ ...
```

- **submitted** — the task has been created and is waiting to be processed
- **working** — the agent is working on the task
- **input-required** — the agent needs additional information from the caller
- **completed** — the task is done, artifacts are ready
- **failed** — the task ended with an error
- **canceled** — the task was canceled by the caller

### Example: Agent card and an A2A server

The Agent Card — a description of the agent's capabilities:

```go
// The Agent Card is a discovery document; the publication path follows the rules of the current A2A version
const agentCardPath = "/.well-known/agent-card.json"

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

// Skill is a specific task the agent knows how to solve
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
    State   string `json:"state"` // submitted, working, input-required, completed, failed, canceled
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

An A2A server processes tasks from other agents:

```go
type A2AServer struct {
    card  AgentCard
    tasks sync.Map // taskID → *Task
}

func (s *A2AServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    switch r.URL.Path {
    case agentCardPath:
        // Discovery: any agent can fetch the card
        json.NewEncoder(w).Encode(s.card)

    case "/tasks/send":
        // Receiving a task from another agent
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

        // Execute the task asynchronously
        go s.processTask(task)

        json.NewEncoder(w).Encode(task)

    case "/tasks/get":
        // Check the task status
        taskID := r.URL.Query().Get("id")
        if task, ok := s.tasks.Load(taskID); ok {
            json.NewEncoder(w).Encode(task)
        } else {
            http.Error(w, "task not found", http.StatusNotFound)
        }
    }
}

func (s *A2AServer) processTask(task *Task) {
    // Extract the task text
    input := task.Messages[0].Parts[0].Text

    // The agent does its work...
    result := fmt.Sprintf("Analyzed: %s", input)

    // Update the task with the result
    task.Artifacts = []Artifact{
        {Name: "result", Parts: []Part{{Type: "text", Text: result}}},
    }
    task.Status = TaskStatus{State: "completed"}
}
```

Usage — a client discovers the agent and sends a task:

```go
func discoverAndDelegate(agentURL string, taskText string) (*Task, error) {
    // 1. Fetch the Agent Card
    resp, err := http.Get(agentURL + agentCardPath)
    if err != nil {
        return nil, fmt.Errorf("discovery failed: %w", err)
    }
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
    body, err := json.Marshal(taskReq)
    if err != nil {
        return nil, err
    }
    resp, err = http.Post(agentURL+"/tasks/send", "application/json", bytes.NewReader(body))
    if err != nil {
        return nil, fmt.Errorf("task submission failed: %w", err)
    }

    var task Task
    json.NewDecoder(resp.Body).Decode(&task)
    resp.Body.Close()

    // 3. Poll the status until completion
    for task.Status.State == "working" {
        time.Sleep(time.Second)
        resp, err = http.Get(agentURL + "/tasks/get?id=" + task.ID)
        if err != nil {
            return nil, fmt.Errorf("poll failed: %w", err)
        }
        json.NewDecoder(resp.Body).Decode(&task)
        resp.Body.Close()
    }

    return &task, nil
}
```

### When to use A2A vs the Supervisor/Worker pattern

| Criterion | A2A | Supervisor/Worker |
|----------|-----|-------------------|
| **Agents from different teams** | Yes — a standard discovery protocol | No — requires shared code |
| **Different frameworks** | Yes — the protocol is implementation-agnostic | No — a shared framework |
| **Simple orchestration** | Overkill | Yes — easier to implement |
| **Dynamic discovery** | Yes — via the Agent Card | No — agents are defined statically |
| **One team, one repository** | Overkill | Yes — direct calls are simpler |

**Rule of thumb:** A2A is needed when agents are built by independent teams and must discover each other dynamically. For agents inside a single system, direct orchestration is enough (see [Chapter 07: Multi-Agent Systems](../07-multi-agent/README.md)).

## Protocol Comparison Table

| | stdio | HTTP | gRPC | MCP | A2A |
|---|---|---|---|---|---|
| **Use case** | Local tools | Distributed services | Production APIs | Tools for LLM agents | Agent-to-agent interaction |
| **Latency** | Minimal (IPC) | Moderate (network + HTTP/1.1) | Low (HTTP/2 + binary format) | Depends on transport (stdio or HTTP) | Moderate (HTTP) |
| **Implementation complexity** | Low | Moderate | High (protobuf, codegen) | Moderate (JSON-RPC 2.0) | Moderate-high |
| **Tool discovery** | None | Swagger / OpenAPI | Reflection, service mesh | `tools/list`, `resources/list` | Agent Card (discovery document) |
| **Streaming** | Line-by-line via stdout | SSE, WebSocket | Bidirectional streams | SSE (Streamable HTTP) | SSE |
| **Contract typing** | None (free-form JSON) | JSON Schema / OpenAPI | Strict (Protobuf IDL) | JSON Schema (for tools) | JSON Schema |
| **When to choose** | CLI utilities, IDE plugins | External APIs, webhooks | Microservices, high load | Tools for LLM agents | Multi-agent systems |

### Authentication: examples

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

**MCP — authentication via OAuth 2.1:**

MCP uses OAuth 2.1 (IETF draft-ietf-oauth-v2-1) for remote servers (the Streamable HTTP transport); the exact flow and requirements are in the current version of the specification ([modelcontextprotocol.io](https://modelcontextprotocol.io/specification)). The client obtains a token through the standard OAuth flow and passes it in the `Authorization` header:

```go
// MCP client with authentication for the HTTP transport
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

For the stdio transport, authentication is usually unnecessary — the server runs locally as a subprocess of the host.

## Common Mistakes

### Mistake 1: No versioning

**Symptom:** Tool updates break the agent.

**Cause:** There is no versioning, and the agent expects the old interface.

**Solution:** Version your tool schemas and check compatibility.

### Mistake 2: No authentication

**Symptom:** Unauthorized access to tools.

**Cause:** No authn/authz for tool servers.

**Solution:** Implement authentication (API keys, tokens).

## Exercises

### Exercise 1: Implement a stdio tool server

Create a tool server that communicates over stdio:

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

## Acceptance Criteria / Checklist

**Passing:**
- [x] You understand the tool server architecture
- [x] You can implement the stdio protocol
- [x] You can implement the HTTP protocol
- [x] You understand the advantages of gRPC for production tool servers
- [x] You understand schema versioning
- [x] You understand MCP: architecture, the three primitives, transports
- [x] You can write an MCP server and an MCP client in Go
- [x] You understand A2A: the Agent Card and the task lifecycle
- [x] You know when to choose MCP, A2A, HTTP, or gRPC

**Failing:**
- [ ] No versioning — updates break compatibility
- [ ] No authentication — a security risk
- [ ] Using MCP as a public high-load API instead of HTTP/gRPC
- [ ] Using A2A where direct orchestration is enough

## Relation to Other Chapters

- **[Chapter 03: Tools and Function Calling](../03-tools-and-function-calling/README.md)** — Tool execution basics
- **[Chapter 07: Multi-Agent Systems](../07-multi-agent/README.md)** — Agent orchestration patterns (Supervisor/Worker and others)
- **[Chapter 17: Security and Governance](../17-security-and-governance/README.md)** — Security for tool servers

## What's Next?

Once you understand tool protocols, move on to:
- **[19. Observability and Tracing](../19-observability-and-tracing/README.md)** — Learn production observability
