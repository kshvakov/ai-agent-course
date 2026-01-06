# 17. Security and Governance

## Why This Chapter?

An agent performs critical operations without confirmation. User writes "delete database", and the agent immediately deletes it. Without security and governance, you cannot:
- Protect against dangerous actions
- Control who can do what
- Audit agent actions
- Protect against prompt injection

Security isn't optional—it's a mandatory requirement for production agents. Without it, an agent can cause irreparable damage.

### Real-World Case Study

**Situation:** A DevOps agent has access to `delete_database` tool. User writes "delete old database test_db", and the agent immediately deletes it.

**Problem:** The database contained important data. No confirmation, no risk assessment, no audit. Impossible to understand who and when deleted the database.

**Solution:** Threat modeling, risk scoring for tools, prompt injection protection, sandboxing, allowlists, RBAC for access control, audit of all operations. Now critical actions require confirmation, and all operations are logged for audit.

## Theory in Simple Terms

### What Is Threat Modeling?

Threat Modeling is risk assessment for each tool. Tools are categorized into risk levels:
- **Low risk:** reading logs, checking status
- **Medium risk:** restarting services, changing settings
- **High risk:** deleting data, changing critical configs

### What Is RBAC?

RBAC (Role-Based Access Control) is role-based access control. Different users have access to different tools:
- **Viewer:** read-only
- **Operator:** read + safe actions
- **Admin:** all actions

### Security Threats

**1. Prompt Injection:**
- Attacker manipulates agent through input
- Bypasses security checks
- Performs unauthorized actions

**2. Tool Abuse:**
- Agent calls dangerous tools
- Without proper validation
- Causes system damage

**3. Data Leakage:**
- Agent reveals sensitive data
- In logs or responses
- Privacy violations

## How It Works (Step by Step)

### Step 1: Threat Modeling and Risk Scoring

Assess risk for each tool:

```go
type ToolRisk string

const (
    RiskLow    ToolRisk = "low"
    RiskMedium ToolRisk = "medium"
    RiskHigh   ToolRisk = "high"
)

type ToolDefinition struct {
    Name                string
    Description         string
    Risk                ToolRisk
    RequiresConfirmation bool
}

func assessRisk(tool ToolDefinition) ToolRisk {
    // Assess risk based on name and description
    if strings.Contains(tool.Name, "delete") || strings.Contains(tool.Name, "remove") {
        return RiskHigh
    }
    if strings.Contains(tool.Name, "restart") || strings.Contains(tool.Name, "update") {
        return RiskMedium
    }
    return RiskLow
}
```

### Step 2: Prompt Injection Protection

**IMPORTANT:** This is the canonical definition of prompt injection protection. In other chapters (e.g., [Chapter 05: Safety and Human-in-the-Loop](../05-safety-and-hitl/README.md)), a simplified approach is used for basic scenarios.

Validate and sanitize user input data:

```go
func sanitizeUserInput(input string) string {
    dangerous := []string{
        "Ignore previous instructions",
        "You are now",
        "System:",
        "Assistant:",
        "ignore previous",
        "forget all",
        "execute:",
    }
    
    sanitized := input
    for _, pattern := range dangerous {
        sanitized = strings.ReplaceAll(sanitized, pattern, "[REDACTED]")
    }
    
    return sanitized
}

func validateInput(input string) error {
    // Check injection patterns
    injectionPatterns := []string{
        "ignore previous",
        "forget all",
        "execute:",
        "system:",
    }
    
    inputLower := strings.ToLower(input)
    for _, pattern := range injectionPatterns {
        if strings.Contains(inputLower, pattern) {
            return fmt.Errorf("potential injection detected: %s", pattern)
        }
    }
    
    return nil
}

func buildMessages(userInput string, systemPrompt string) []openai.ChatCompletionMessage {
    // Validate input data
    if err := validateInput(userInput); err != nil {
        return []openai.ChatCompletionMessage{
            {Role: "system", Content: systemPrompt},
            {Role: "user", Content: "Invalid input detected."},
        }
    }
    
    return []openai.ChatCompletionMessage{
        {Role: "system", Content: systemPrompt},
        {Role: "user", Content: sanitizeUserInput(userInput)},
    }
}
```

**Why this is important:**
- System Prompt is never changed by user
- Input data is validated and sanitized
- Context separation (system vs user) prevents injection

### Step 3: Tool Allowlists

Allow only safe tools:

```go
type ToolAllowlist struct {
    allowedTools map[string]bool
    dangerousTools map[string]bool
}

func (a *ToolAllowlist) IsAllowed(toolName string) bool {
    return a.allowedTools[toolName]
}

func (a *ToolAllowlist) IsDangerous(toolName string) bool {
    return a.dangerousTools[toolName]
}

func (a *ToolAllowlist) RequireConfirmation(toolName string) bool {
    return a.IsDangerous(toolName)
}
```

### Step 4: Tool Sandboxing

Isolate tool execution:

```go
func executeToolSandboxed(toolName string, args map[string]any) (any, error) {
    // Create isolated environment
    sandbox := &Sandbox{
        WorkDir: "/tmp/sandbox",
        MaxMemory: 100 * 1024 * 1024, // 100MB
        Timeout: 30 * time.Second,
    }
    
    // Execute in sandbox
    result, err := sandbox.Execute(toolName, args)
    if err != nil {
        return nil, fmt.Errorf("sandbox execution failed: %w", err)
    }
    
    return result, nil
}
```

### Step 5: Confirmations for Critical Actions

Require confirmation before executing critical operations (see also [Chapter 05: Safety and Human-in-the-Loop](../05-safety-and-hitl/README.md) for basic concepts):

```go
func executeToolWithConfirmation(toolCall openai.ToolCall, userID string) (string, error) {
    tool := getToolDefinition(toolCall.Function.Name)
    
    if tool.RequiresConfirmation {
        // Request confirmation
        confirmed := requestConfirmation(userID, toolCall)
        if !confirmed {
            return "Operation cancelled by user", nil
        }
    }
    
    return executeTool(toolCall)
}
```

### Step 6: RBAC for Tools

Control access to tools based on user role:

```go
type UserRole string

const (
    RoleViewer  UserRole = "viewer"
    RoleOperator UserRole = "operator"
    RoleAdmin   UserRole = "admin"
)

func canUseTool(userRole UserRole, toolName string) bool {
    toolPermissions := map[string][]UserRole{
        "read_logs":      {RoleViewer, RoleOperator, RoleAdmin},
        "restart_service": {RoleOperator, RoleAdmin},
        "delete_database": {RoleAdmin},
    }
    
    roles, exists := toolPermissions[toolName]
    if !exists {
        return false
    }
    
    for _, role := range roles {
        if role == userRole {
            return true
        }
    }
    
    return false
}
```

### Step 7: Policy-as-Code (Policy Enforcement)

Define security policies and enforce them automatically:

```go
type SecurityPolicy struct {
    MaxToolCallsPerRequest int
    AllowedTools []string
    RequireConfirmationFor []string
}

func (p *SecurityPolicy) ValidateRequest(toolCalls []ToolCall) error {
    if len(toolCalls) > p.MaxToolCallsPerRequest {
        return fmt.Errorf("too many tool calls: %d > %d", len(toolCalls), p.MaxToolCallsPerRequest)
    }
    
    for _, call := range toolCalls {
        if !contains(p.AllowedTools, call.Name) {
            return fmt.Errorf("tool not allowed: %s", call.Name)
        }
    }
    
    return nil
}
```

### Step 8: Dry-Run Modes

Implement a mode where tools don't execute for real:

```go
type ToolExecutor struct {
    dryRun bool
}

func (e *ToolExecutor) Execute(toolName string, args map[string]any) (string, error) {
    if e.dryRun {
        return fmt.Sprintf("[DRY RUN] Would execute %s with args: %v", toolName, args), nil
    }
    
    return executeTool(toolName, args)
}
```

### Step 9: Audit

Log all tool calls for audit:

```go
type AuditLog struct {
    Timestamp  time.Time              `json:"timestamp"`
    UserID     string                 `json:"user_id"`
    ToolName   string                 `json:"tool_name"`
    Arguments  map[string]any `json:"arguments"`
    Result     string                 `json:"result"`
    Error      string                 `json:"error,omitempty"`
}

func logAudit(log AuditLog) {
    // Send to separate audit system
    auditJSON, _ := json.Marshal(log)
    // Send to separate audit service (not regular logs)
    fmt.Printf("AUDIT: %s\n", string(auditJSON))
}
```

## Where to Integrate This in Our Code

### Integration Point 1: Tool Execution

In `labs/lab02-tools/main.go` add access check and confirmation:

```go
func executeTool(toolCall openai.ToolCall, userRole UserRole) (string, error) {
    // Check access
    if !canUseTool(userRole, toolCall.Function.Name) {
        return "", fmt.Errorf("access denied for tool: %s", toolCall.Function.Name)
    }
    
    // Check risk and request confirmation
    tool := getToolDefinition(toolCall.Function.Name)
    if tool.RequiresConfirmation {
        if !requestConfirmation(toolCall) {
            return "Operation cancelled", nil
        }
    }
    
    // Log for audit
    logAudit(AuditLog{
        ToolName: toolCall.Function.Name,
        Arguments: parseArguments(toolCall.Function.Arguments),
        Timestamp: time.Now(),
    })
    
    // Execute tool (with sandboxing for dangerous operations)
    if tool.Risk == RiskHigh {
        return executeToolSandboxed(toolCall.Function.Name, parseArguments(toolCall.Function.Arguments))
    }
    
    return executeToolImpl(toolCall)
}
```

### Integration Point 2: Human-in-the-Loop

In `labs/lab05-human-interaction/main.go` confirmations already exist. Extend them for risk scoring:

```go
func requestConfirmation(toolCall openai.ToolCall) bool {
    tool := getToolDefinition(toolCall.Function.Name)
    
    if tool.Risk == RiskHigh {
        fmt.Printf("⚠️  WARNING: High-risk operation: %s\n", toolCall.Function.Name)
        fmt.Printf("Type 'yes' to confirm: ")
        // ... request confirmation ...
    }
    
    return true
}
```

## Mini Code Example

Complete example with security based on `labs/lab05-human-interaction/main.go`:

```go
package main

import (
    "bufio"
    "context"
    "encoding/json"
    "fmt"
    "os"
    "strings"
    "time"

    "github.com/sashabaranov/go-openai"
)

type ToolRisk string

const (
    RiskLow    ToolRisk = "low"
    RiskMedium ToolRisk = "medium"
    RiskHigh   ToolRisk = "high"
)

type ToolDefinition struct {
    Name                string
    Description         string
    Risk                ToolRisk
    RequiresConfirmation bool
}

var toolDefinitions = map[string]ToolDefinition{
    "delete_db": {
        Name:                "delete_db",
        Description:         "Delete a database",
        Risk:                RiskHigh,
        RequiresConfirmation: true,
    },
    "send_email": {
        Name:                "send_email",
        Description:         "Send an email",
        Risk:                RiskLow,
        RequiresConfirmation: false,
    },
}

type AuditLog struct {
    Timestamp time.Time              `json:"timestamp"`
    ToolName  string                 `json:"tool_name"`
    Arguments map[string]any `json:"arguments"`
    Result    string                 `json:"result"`
}

func logAudit(log AuditLog) {
    auditJSON, _ := json.Marshal(log)
    fmt.Printf("AUDIT: %s\n", string(auditJSON))
}

func sanitizeUserInput(input string) string {
    dangerous := []string{
        "Ignore previous instructions",
        "You are now",
        "System:",
    }
    
    sanitized := input
    for _, pattern := range dangerous {
        sanitized = strings.ReplaceAll(sanitized, pattern, "[REDACTED]")
    }
    
    return sanitized
}

func requestConfirmation(toolCall openai.ToolCall) bool {
    tool, exists := toolDefinitions[toolCall.Function.Name]
    if !exists || !tool.RequiresConfirmation {
        return true
    }
    
    fmt.Printf("⚠️  WARNING: High-risk operation: %s\n", toolCall.Function.Name)
    fmt.Printf("Type 'yes' to confirm: ")
    
    reader := bufio.NewReader(os.Stdin)
    confirmation, _ := reader.ReadString('\n')
    confirmation = strings.TrimSpace(confirmation)
    
    return confirmation == "yes"
}

func deleteDB(name string) string {
    return fmt.Sprintf("Database '%s' has been DELETED.", name)
}

func sendEmail(to, subject, body string) string {
    return fmt.Sprintf("Email sent to %s", to)
}

func main() {
    token := os.Getenv("OPENAI_API_KEY")
    if token == "" {
        token = "dummy"
    }
    
    config := openai.DefaultConfig(token)
    if baseURL := os.Getenv("OPENAI_BASE_URL"); baseURL != "" {
        config.BaseURL = baseURL
    }
    client := openai.NewClientWithConfig(config)
    
    ctx := context.Background()
    
    tools := []openai.Tool{
        {
            Type: openai.ToolTypeFunction,
            Function: &openai.FunctionDefinition{
                Name:        "delete_db",
                Description: "Delete a database by name. DANGEROUS ACTION.",
                Parameters: json.RawMessage(`{
                    "type": "object",
                    "properties": { "name": { "type": "string" } },
                    "required": ["name"]
                }`),
            },
        },
        {
            Type: openai.ToolTypeFunction,
            Function: &openai.FunctionDefinition{
                Name:        "send_email",
                Description: "Send an email",
                Parameters: json.RawMessage(`{
                    "type": "object",
                    "properties": {
                        "to": { "type": "string" },
                        "subject": { "type": "string" },
                        "body": { "type": "string" }
                    },
                    "required": ["to", "subject", "body"]
                }`),
            },
        },
    }
    
    messages := []openai.ChatCompletionMessage{
        {
            Role:    openai.ChatMessageRoleSystem,
            Content: "You are a helpful assistant. IMPORTANT: Always ask for explicit confirmation before deleting anything.",
        },
    }
    
    reader := bufio.NewReader(os.Stdin)
    fmt.Println("Agent is ready. (Try: 'Delete prod_db' or 'Send email to bob')")
    
    for {
        fmt.Print("\nUser > ")
        input, _ := reader.ReadString('\n')
        input = strings.TrimSpace(input)
        if input == "exit" {
            break
        }
        
        // Sanitize input data
        sanitizedInput := sanitizeUserInput(input)
        
        messages = append(messages, openai.ChatCompletionMessage{
            Role:    openai.ChatMessageRoleUser,
            Content: sanitizedInput,
        })
        
        for {
            req := openai.ChatCompletionRequest{
                Model:    openai.GPT4,
                Messages: messages,
                Tools:    tools,
            }
            
            resp, err := client.CreateChatCompletion(ctx, req)
            if err != nil {
                fmt.Printf("Error: %v\n", err)
                break
            }
            
            msg := resp.Choices[0].Message
            messages = append(messages, msg)
            
            if len(msg.ToolCalls) == 0 {
                fmt.Printf("Agent > %s\n", msg.Content)
                break
            }
            
            for _, toolCall := range msg.ToolCalls {
                fmt.Printf("  [System] Executing tool: %s\n", toolCall.Function.Name)
                
                // Check risk and request confirmation
                if !requestConfirmation(toolCall) {
                    result := "Operation cancelled by user"
                    messages = append(messages, openai.ChatCompletionMessage{
                        Role:       openai.ChatMessageRoleTool,
                        Content:    result,
                        ToolCallID: toolCall.ID,
                    })
                    continue
                }
                
                var result string
                var args map[string]any
                json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
                
                if toolCall.Function.Name == "delete_db" {
                    result = deleteDB(args["name"].(string))
                } else if toolCall.Function.Name == "send_email" {
                    result = sendEmail(
                        args["to"].(string),
                        args["subject"].(string),
                        args["body"].(string),
                    )
                }
                
                // Log for audit
                logAudit(AuditLog{
                    Timestamp: time.Now(),
                    ToolName:  toolCall.Function.Name,
                    Arguments: args,
                    Result:    result,
                })
                
                messages = append(messages, openai.ChatCompletionMessage{
                    Role:       openai.ChatMessageRoleTool,
                    Content:    result,
                    ToolCallID: toolCall.ID,
                })
            }
        }
    }
}
```

## Common Errors

### Error 1: No Risk Assessment

**Symptom:** All tools are handled the same way, critical actions don't require confirmation.

**Cause:** No risk scoring for tools.

**Solution:**
```go
// BAD
func executeTool(toolCall openai.ToolCall) {
    // All tools execute the same way
}

// GOOD
tool := getToolDefinition(toolCall.Function.Name)
if tool.Risk == RiskHigh && tool.RequiresConfirmation {
    if !requestConfirmation(toolCall) {
        return
    }
}
```

### Error 2: No Prompt Injection Protection

**Symptom:** User can inject prompt through input data.

**Cause:** Input data is not sanitized.

**Solution:**
```go
// BAD
messages = append(messages, openai.ChatCompletionMessage{
    Role: "user",
    Content: userInput, // Not sanitized
})

// GOOD
messages = append(messages, openai.ChatCompletionMessage{
    Role: "user",
    Content: sanitizeUserInput(userInput),
})
```

### Error 3: No RBAC

**Symptom:** All users have access to all tools.

**Cause:** No access rights check.

**Solution:**
```go
// BAD
func executeTool(toolCall openai.ToolCall) {
    // No access check
}

// GOOD
if !canUseTool(userRole, toolCall.Function.Name) {
    return fmt.Errorf("access denied")
}
```

### Error 4: No Sandboxing

**Symptom:** Tool execution affects the system, causing damage.

**Cause:** Tools execute with full system access.

**Solution:**
```go
// BAD
result := executeTool(toolCall) // Direct execution

// GOOD
if tool.Risk == RiskHigh {
    result = executeToolSandboxed(toolCall.Function.Name, args)
} else {
    result = executeTool(toolCall)
}
```

### Error 5: No Audit

**Symptom:** Impossible to understand who and when performed a critical operation.

**Cause:** Operations are not logged for audit.

**Solution:**
```go
// BAD
result := executeTool(toolCall)
// No logging

// GOOD
result := executeTool(toolCall)
logAudit(AuditLog{
    ToolName: toolCall.Function.Name,
    Arguments: args,
    Result: result,
    Timestamp: time.Now(),
})
```

## Mini-Exercises

### Exercise 1: Implement Risk Scoring

Create a function to assess tool risk:

```go
func assessRisk(toolName string, description string) ToolRisk {
    // Your code here
    // Return RiskLow, RiskMedium, or RiskHigh
}
```

**Expected result:**
- Tools with "delete", "remove" → RiskHigh
- Tools with "restart", "update" → RiskMedium
- Others → RiskLow

### Exercise 2: Implement RBAC

Create an access check function:

```go
func canUseTool(userRole UserRole, toolName string) bool {
    // Your code here
    // Return true if user has access to tool
}
```

**Expected result:**
- RoleViewer → only read_logs
- RoleOperator → read_logs + restart_service
- RoleAdmin → all tools

### Exercise 3: Implement Sandboxing

Create a function to execute tool in sandbox:

```go
func executeToolSandboxed(toolName string, args map[string]any) (any, error) {
    // Your code here
    // Isolate tool execution
}
```

**Expected result:**
- Tool executes in isolated environment
- Resources limited (memory, time)
- System protected from damage

## Completion Criteria / Checklist

✅ **Completed (production ready):**
- Threat modeling and risk scoring implemented for tools
- Critical actions require confirmation
- Prompt injection protection implemented (validation and sanitization)
- RBAC implemented for access control
- Sandboxing implemented for dangerous operations
- Tool allowlists implemented
- Policy-as-code implemented (policy enforcement)
- All operations logged for audit
- Dry-run mode implemented for testing

❌ **Not completed:**
- No risk assessment
- No prompt injection protection
- No RBAC
- No sandboxing
- No audit
- No allowlists

## Connection with Other Chapters

- **[Chapter 05: Safety and Human-in-the-Loop](../05-safety-and-hitl/README.md)** — Basic concepts of confirmations and clarifications (security UX)
- **[Chapter 19: Observability and Tracing](../19-observability-and-tracing/README.md)** — Audit as part of observability
- **[Chapter 24: Data and Privacy](../24-data-and-privacy/README.md)** — Personal data protection

## What's Next?

After understanding security and governance, proceed to:
- **[18. Tool Protocols and Tool Servers](../18-tool-protocols-and-servers/README.md)** — Learn about tool communication protocols

---

**Navigation:** [← Best Practices](../16-best-practices/README.md) | [Table of Contents](../README.md) | [Tool Protocols and Tool Servers →](../18-tool-protocols-and-servers/README.md)
