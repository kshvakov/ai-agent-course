# Manual: Lab 12 — Tool Server Protocol

## Why Is This Needed?

In this lab you'll implement a tool server — architecture where tools run in separate processes and communicate with agent via protocols (stdio or HTTP). In production, gRPC is also commonly used due to its strict contract via Protobuf and built-in security and observability mechanisms.

### Real-World Case Study

**Situation:** Agent uses many tools written in different languages.

**Without tool server:**
- All tools compiled with agent
- Updating tool requires rebuilding agent
- Tools in different languages hard to integrate

**With tool server:**
- Tools in separate processes
- Updating tool doesn't require rebuilding agent
- Tools can be written in any language
- Better isolation and security

**Difference:** Tool server allows independent development and updates of tools.

## Theory in Simple Terms

### Tool Server Architecture

**Monolithic agent (Lab 01-09):**
```
[Agent Process]
  ├── Tool 1 (Go)
  ├── Tool 2 (Go)
  └── Tool 3 (Go)
```
All in one process, all in one language.

**Tool Server architecture (Lab 12):**
```
[Agent Process]  ←→  [Tool Server Process]
                        ├── Tool 1 (Go)
                        ├── Tool 2 (Python)
                        └── Tool 3 (Bash)
```
Tools in separate process, can be in different languages.

### Communication Protocols

**stdio Protocol:**
- Simple: read from stdin, write to stdout
- JSON request/response format
- Good for local tools
- Example: `echo '{"tool":"check_status"}' | tool-server`

**HTTP Protocol:**
- REST API for tool execution
- Better for distributed systems
- Can be called from anywhere
- Example: `POST http://localhost:8080/execute`

**gRPC Protocol (production):**
- Strict contract via Protocol Buffers (Protobuf)
- Type safety and automatic client/server generation
- Built-in mechanisms: TLS/mTLS, authentication via metadata, deadlines, retries
- Rich Go ecosystem: interceptors, health checks, reflection
- Integration with observability (tracing, metrics, logging)
- Practical choice for production tool servers, especially in Go ecosystem

### Schema Versioning

Tools evolve over time:
- Version 1.0: `check_status(hostname)`
- Version 2.0: `check_status(hostname, timeout)` — added parameter

**Versioning strategy:**
- Each tool has version
- Agent specifies required version
- Tool server checks compatibility
- Return error if versions don't match

**Example:**
```go
ToolDefinition{
    Name:           "check_status",
    Version:        "2.0",
    CompatibleWith: []string{"1.0", "1.1", "2.0"},
}
```

## Execution Algorithm

### Step 1: stdio Protocol

```go
func (s *StdioToolServer) Start() error {
    scanner := bufio.NewScanner(os.Stdin)
    encoder := json.NewEncoder(os.Stdout)
    
    for scanner.Scan() {
        var req ToolRequest
        if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
            continue
        }
        
        // Check version
        tool := s.tools[req.Tool]
        if !checkVersionCompatibility(tool, req.Version) {
            encoder.Encode(ToolResponse{
                Success: false,
                Error:  "Version mismatch",
            })
            continue
        }
        
        // Execute tool
        result, err := executeTool(req.Tool, req.Arguments)
        if err != nil {
            encoder.Encode(ToolResponse{
                Success: false,
                Error:  err.Error(),
            })
            continue
        }
        
        // Return result
        encoder.Encode(ToolResponse{
            Success: true,
            Result:  result,
        })
    }
    
    return scanner.Err()
}
```

### Step 2: HTTP Protocol

```go
func (s *HTTPToolServer) Start(port string) error {
    http.HandleFunc("/execute", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != "POST" {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }
        
        var req ToolRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }
        
        // Check version
        tool := s.tools[req.Tool]
        if !checkVersionCompatibility(tool, req.Version) {
            json.NewEncoder(w).Encode(ToolResponse{
                Success: false,
                Error:  "Version mismatch",
            })
            return
        }
        
        // Execute tool
        result, err := executeTool(req.Tool, req.Arguments)
        if err != nil {
            json.NewEncoder(w).Encode(ToolResponse{
                Success: false,
                Error:  err.Error(),
            })
            return
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(ToolResponse{
            Success: true,
            Result:  result,
        })
    })
    
    return http.ListenAndServe(":"+port, nil)
}
```

### Step 3: Version Compatibility Check

```go
func checkVersionCompatibility(tool *ToolDefinition, requestedVersion string) bool {
    // Exact match
    if tool.Version == requestedVersion {
        return true
    }
    
    // Check compatible versions
    for _, compatible := range tool.CompatibleWith {
        if compatible == requestedVersion {
            return true
        }
    }
    
    return false
}
```

### Step 4: Tool Client for Agent

```go
type HTTPToolClient struct {
    baseURL string
    client  *http.Client
}

func (c *HTTPToolClient) CallTool(tool string, version string, arguments json.RawMessage) (string, error) {
    req := ToolRequest{
        Tool:      tool,
        Version:   version,
        Arguments: arguments,
    }
    
    data, _ := json.Marshal(req)
    resp, err := c.client.Post(c.baseURL+"/execute", "application/json", bytes.NewBuffer(data))
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    
    var toolResp ToolResponse
    if err := json.NewDecoder(resp.Body).Decode(&toolResp); err != nil {
        return "", err
    }
    
    if !toolResp.Success {
        return "", fmt.Errorf("tool error: %s", toolResp.Error)
    }
    
    return toolResp.Result, nil
}
```

## Common Mistakes

### Mistake 1: Version Not Checked

**Symptom:** Tool called with incompatible version.

**Cause:** Version compatibility not checked.

**Solution:** Always call `checkVersionCompatibility()` before execution.

### Mistake 2: Protocol Not Implemented

**Symptom:** Agent cannot call tool server.

**Cause:** Protocol not implemented or implemented incorrectly.

**Solution:** Follow JSON request/response format strictly.

### Mistake 3: Tools Not Isolated

**Symptom:** Error in one tool crashes entire server.

**Cause:** Tools executed in same process.

**Solution:** Use separate processes for each tool or handle panic.

## Completion Criteria

✅ **Completed:**
- stdio protocol implemented
- HTTP protocol implemented
- Versioning works
- Agent can call tool server
- Version compatibility checked

❌ **Not completed:**
- Protocol not implemented
- Version not checked
- Agent cannot call tool server
