# 18. Tool Protocols and Tool Servers

## Why This Chapter?

As agents grow, tools become complex services. Instead of embedding tool code directly, you can run tools as separate processes or services. This requires protocols for communication, versioning, and security.

This chapter covers tool server patterns: stdio protocols, HTTP API, schema versioning, and authentication/authorization.

### Real-World Case Study

**Situation:** You have 50+ tools. Some are written in Go, some in Python, some are external services. Embedding all in one agent binary is impractical.

**Problem:**
- Different languages require different integration approaches
- Tool updates require agent redeployment
- No isolation between tools
- Hard to scale individual tools

**Solution:** Tool servers: Each tool runs as a separate process/service. Agent communicates through a standard protocol (stdio or HTTP). Tools can be updated independently, scaled separately, and isolated for security.

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

### Step 5: Schema Versioning

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

✅ **Completed:**
- Understand tool server architecture
- Can implement stdio protocol
- Can implement HTTP protocol
- Understand schema versioning

❌ **Not completed:**
- No versioning, updates break compatibility
- No authentication, security risk

## Connection with Other Chapters

- **[Chapter 03: Tools and Function Calling](../03-tools-and-function-calling/README.md)** — Basics of tool execution
- **[Chapter 17: Security and Governance](../17-security-and-governance/README.md)** — Security for tool servers

## What's Next?

After understanding tool protocols, proceed to:
- **[19. Observability and Tracing](../19-observability-and-tracing/README.md)** — Learn production observability

---

**Navigation:** [← Security and Governance](../17-security-and-governance/README.md) | [Table of Contents](../README.md) | [Observability and Tracing →](../19-observability-and-tracing/README.md)
