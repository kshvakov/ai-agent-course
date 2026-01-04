# Security and Governance

## Why This Chapter?

Agent performs critical operations without confirmation. User writes "delete database", and agent immediately deletes it. Without security and governance, you cannot:
- Protect from dangerous actions
- Control who can do what
- Audit agent actions
- Protect from prompt injection

Security is not an option, but a mandatory requirement for production agents. Without it, agent can cause irreparable damage.

### Real-World Case Study

**Situation:** DevOps agent has access to `delete_database` tool. User writes "delete old test_db database", and agent immediately deletes it.

**Problem:** Database contained important data. No confirmation, no risk assessment, no audit. Cannot understand who and when deleted database.

**Solution:** Risk scoring for tools, confirmations for critical actions, RBAC for access control, audit of all operations. Now critical actions require confirmation, and all operations are logged for audit.

## Theory in Simple Terms

### What Is Threat Modeling?

Threat Modeling is risk assessment for each tool. Tools divided into risk levels:
- **Low risk:** reading logs, checking status
- **Medium risk:** restarting services, changing settings
- **High risk:** deleting data, changing critical configs

### What Is RBAC?

RBAC (Role-Based Access Control) is role-based access control. Different users have access to different tools:
- **Viewer:** read-only
- **Operator:** read + safe actions
- **Admin:** all actions

## How It Works (Step-by-Step)

### Step 1: Risk Scoring for Tools

Assess risk of each tool:

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

### Step 2: Confirmations for Critical Actions

Require confirmation before executing critical operations (see `labs/lab05-human-interaction/main.go`):

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

### Step 3: RBAC for Tools

Control tool access based on user role:

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

### Step 4: Prompt Injection Protection

Validate and sanitize user input:

```go
func sanitizeUserInput(input string) string {
    dangerous := []string{
        "Ignore previous instructions",
        "You are now",
        "System:",
        "Assistant:",
    }
    
    sanitized := input
    for _, pattern := range dangerous {
        sanitized = strings.ReplaceAll(sanitized, pattern, "[REDACTED]")
    }
    
    return sanitized
}

func buildMessages(userInput string, systemPrompt string) []openai.ChatCompletionMessage {
    return []openai.ChatCompletionMessage{
        {Role: "system", Content: systemPrompt},
        {Role: "user", Content: sanitizeUserInput(userInput)},
    }
}
```

### Step 5: Dry-Run Modes

Implement mode where tools don't execute for real:

```go
type ToolExecutor struct {
    dryRun bool
}

func (e *ToolExecutor) Execute(toolName string, args map[string]interface{}) (string, error) {
    if e.dryRun {
        return fmt.Sprintf("[DRY RUN] Would execute %s with args: %v", toolName, args), nil
    }
    
    return executeTool(toolName, args)
}
```

### Step 6: Audit

Log all tool calls for audit:

```go
type AuditLog struct {
    Timestamp  time.Time              `json:"timestamp"`
    UserID     string                 `json:"user_id"`
    ToolName   string                 `json:"tool_name"`
    Arguments  map[string]interface{} `json:"arguments"`
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

## Where to Integrate in Our Code

### Integration Point 1: Tool Execution

In `labs/lab02-tools/main.go`, add access check and confirmation:

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
    
    // Execute tool
    return executeToolImpl(toolCall)
}
```

### Integration Point 2: Human-in-the-Loop

In `labs/lab05-human-interaction/main.go`, confirmations already exist. Extend them for risk scoring:

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
    Arguments map[string]interface{} `json:"arguments"`
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
        
        // Sanitize input
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
                var args map[string]interface{}
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

## Common Mistakes

### Mistake 1: No Risk Assessment

**Symptom:** All tools handled the same, critical actions don't require confirmation.

**Cause:** No risk scoring for tools.

**Solution:**
```go
// BAD
func executeTool(toolCall openai.ToolCall) {
    // All tools executed the same
}

// GOOD
tool := getToolDefinition(toolCall.Function.Name)
if tool.Risk == RiskHigh && tool.RequiresConfirmation {
    if !requestConfirmation(toolCall) {
        return
    }
}
```

### Mistake 2: No RBAC

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

### Mistake 3: No Prompt Injection Protection

**Symptom:** User can inject prompt via input data.

**Cause:** Input data not sanitized.

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

### Mistake 4: No Audit

**Symptom:** Cannot understand who and when executed critical operation.

**Cause:** Operations not logged for audit.

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

Create function to assess tool risk:

```go
func assessRisk(toolName string, description string) ToolRisk {
    // Your code here
    // Return RiskLow, RiskMedium or RiskHigh
}
```

**Expected result:**
- Tools with "delete", "remove" → RiskHigh
- Tools with "restart", "update" → RiskMedium
- Others → RiskLow

### Exercise 2: Implement RBAC

Create function to check access:

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

## Completion Criteria / Checklist

✅ **Completed (production-ready):**
- Risk scoring implemented for tools
- Critical actions require confirmation
- RBAC implemented for access control
- Input data sanitized from prompt injection
- All operations logged for audit
- Dry-run mode implemented for testing

❌ **Not completed:**
- No risk assessment
- No confirmations for critical actions
- No RBAC
- No prompt injection protection
- No audit

## Connection with Other Chapters

- **Human-in-the-Loop:** Basic confirmation concepts — [Chapter 06: Safety and Human-in-the-Loop](../06-safety-and-hitl/README.md)
- **Observability:** Audit as part of observability — [Observability and Tracing](observability.md)
- **Data Privacy:** Personal data protection — [Data and Privacy](data_privacy.md)

---

**Navigation:** [← Workflow and State Management](workflow_state.md) | [Chapter 12 Table of Contents](README.md) | [Prompt and Program Management →](prompt_program_mgmt.md)
