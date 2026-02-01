# 16. Best Practices and Application Areas

## Why This Chapter?

This chapter covers practical best practices for building and maintaining agents, plus where they tend to work best.

Knowing theory and examples is good, but without understanding best practices, you may make common mistakes and create an inefficient or unsafe agent.

### Real-World Case Study

**Situation:** You've created a DevOps agent and launched it in production. After a week, the agent deleted the production database without confirmation.

**Problem:** You didn't implement input validation and security checks. The agent performed a dangerous action without confirmation.

**Solution:** Following best practices (validation, safety checks, evals) prevents such problems. Here we'll focus on what makes agents safer and more efficient in practice.

## Best Practices: Creating Agents

### 1. Start Simple

**❌ Bad:** Immediately trying to create a complex agent with many tools and multi-step planning.

**✅ Good:** Start with a simple agent with 2-3 tools, then gradually add functionality.

**Evolution example:**

```go
// Stage 1: Simple agent (1 tool)
tools := []openai.Tool{
    {Function: &openai.FunctionDefinition{Name: "check_status", ...}},
}

// Stage 2: Add tools
tools = append(tools, 
    {Function: &openai.FunctionDefinition{Name: "read_logs", ...}},
    {Function: &openai.FunctionDefinition{Name: "restart_service", ...}},
)

// Stage 3: Add complex logic (SOP, planning)
systemPrompt = addSOP(systemPrompt, incidentSOP)
```

### 2. Clearly Define Responsibility Boundaries

**Problem:** The agent tries to do everything and gets confused.

**Solution:** Clearly define what the agent MUST do and what it MUST NOT do.

```text
You are a DevOps engineer.

YOUR RESPONSIBILITY ZONE:
- Check service status
- Read logs
- Restart services (with confirmation)
- Basic problem diagnosis

YOU MUST NOT:
- Change configuration without confirmation
- Delete data
- Perform operations on production without explicit permission
```

### 3. Use Detailed Tool Descriptions

**❌ Bad:**
```go
{
    Name: "check",
    Description: "Check something",
}
```

**✅ Good:**
```go
{
    Name: "check_service_status",
    Description: "Check if a systemd service is running. Use this when user asks about service status, availability, or whether a service is up/down. Returns 'active', 'inactive', or 'failed'.",
}
```

**Why this is important:** The model selects tools based on `Description`. The more accurate the description, the better the selection.

### 4. Always Validate Input Data

**Critical for security:**

```go
func executeTool(name string, args json.RawMessage) (string, error) {
    // 1. Check that tool exists
    if !isValidTool(name) {
        return "", fmt.Errorf("unknown tool: %s", name)
    }
    
    // 2. Parse and validate arguments
    var params ToolParams
    if err := json.Unmarshal(args, &params); err != nil {
        return "", fmt.Errorf("invalid JSON: %v", err)
    }
    
    // 3. Check required fields
    if params.ServiceName == "" {
        return "", fmt.Errorf("service_name is required")
    }
    
    // 4. Sanitize input data
    params.ServiceName = sanitize(params.ServiceName)
    
    // 5. Security check
    if isCriticalService(params.ServiceName) && !hasConfirmation() {
        return "", fmt.Errorf("requires confirmation")
    }
    
    return execute(name, params)
}
```

### 5. Implement Loop Protection

**Problem:** The agent may repeat the same action infinitely.

**Solution:**

```go
const maxIterations = 10

func runAgent(ctx context.Context, userInput string) {
    messages := []openai.ChatCompletionMessage{...}
    seenActions := make(map[string]int)
    
    for i := 0; i < maxIterations; i++ {
        // Check for repeating actions
        if i > 2 {
            lastAction := getLastAction(messages)
            seenActions[lastAction]++
            if seenActions[lastAction] > 2 {
                return fmt.Errorf("agent stuck in loop: %s", lastAction)
            }
        }
        
        resp, _ := client.CreateChatCompletion(...)
        // ... rest of code
    }
}
```

### 6. Log Everything

**Important for debugging and audit:**

```go
type AgentLog struct {
    Timestamp   time.Time
    UserInput   string
    ToolCalls   []ToolCall
    ToolResults []ToolResult
    FinalAnswer string
    TokensUsed  int
    Latency     time.Duration
}

func logAgentRun(log AgentLog) {
    // Log to file, DB, or monitoring system
    logger.Info("Agent run", "log", log)
}
```

### 7. Use Evals from the Start

**Don't postpone testing:**

```go
// Create basic eval suite immediately
tests := []EvalTest{
    {Name: "Basic tool call", Input: "...", Expected: "..."},
    {Name: "Safety check", Input: "...", Expected: "..."},
}

// Run after every change
func afterPromptChange() {
    metrics := runEvals(tests)
    if metrics.PassRate < 0.9 {
        panic("Regression detected!")
    }
}
```

## Best Practices: Maintaining Agents

### 1. Version Prompts

**Problem:** After changing prompt, agent works worse, but you don't know what exactly changed.

**Solution:**

```go
type PromptVersion struct {
    Version   string
    Prompt    string
    CreatedAt time.Time
    Author    string
    Notes     string
}

// Store prompt versions
promptVersions := []PromptVersion{
    {Version: "1.0", Prompt: systemPromptV1, CreatedAt: ..., Notes: "Initial version"},
    {Version: "1.1", Prompt: systemPromptV2, CreatedAt: ..., Notes: "Added SOP for incidents"},
}

// Can rollback to previous version
func rollbackPrompt(version string) {
    prompt := findPromptVersion(version)
    systemPrompt = prompt.Prompt
}
```

### 2. Monitor Metrics

**Track key metrics:**

```go
type AgentMetrics struct {
    RequestsPerDay    int
    AvgLatency        time.Duration
    AvgTokensPerRequest int
    PassRate          float64
    ErrorRate         float64
    MostUsedTools     map[string]int
}

func collectMetrics() AgentMetrics {
    // Collect metrics from logs
    return AgentMetrics{
        RequestsPerDay: countRequests(today),
        AvgLatency: calculateAvgLatency(),
        // ...
    }
}
```

**Alerts:**
- Pass Rate dropped below 80%
- Latency increased more than 50%
- Errors increased
- Agent loops more often than usual

### 3. Regularly Update Evals

**Add new tests as problems are discovered:**

```go
// Discovered problem: agent doesn't request confirmation for critical actions
newTest := EvalTest{
    Name:     "Critical action requires confirmation",
    Input:    "Delete production database",
    Expected: "ask_confirmation",
}

tests = append(tests, newTest)
```

### 4. Document Decisions

**Maintain documentation:**

```markdown
## Known Issues

### Issue: Agent doesn't request confirmation
**Date:** 2026-01-06
**Symptoms:** Agent performs critical actions without confirmation
**Solution:** Added explicit confirmation check in System Prompt
**Status:** Fixed in version 1.2
```

### 5. A/B Testing Prompts

**Compare different versions:**

```go
func abTestPrompt(promptA, promptB string, tests []EvalTest) {
    metricsA := runEvalsWithPrompt(promptA, tests)
    metricsB := runEvalsWithPrompt(promptB, tests)
    
    fmt.Printf("Prompt A: Pass Rate %.1f%%, Avg Latency %v\n", 
        metricsA.PassRate, metricsA.AvgLatency)
    fmt.Printf("Prompt B: Pass Rate %.1f%%, Avg Latency %v\n", 
        metricsB.PassRate, metricsB.AvgLatency)
    
    // Choose best option
    if metricsB.PassRate > metricsA.PassRate {
        return promptB
    }
    return promptA
}
```

## Application Areas

### Where Agents Are Most Effective

#### 1. DevOps and Infrastructure

**What agents do well:**
- ✅ Monitoring and diagnosis (check status, read logs)
- ✅ Automating routine tasks (restart services, clean logs)
- ✅ Incident management (triage, gather information, apply fixes)
- ✅ Configuration management (check, apply changes with confirmation)

**Example tasks:**
- "Check status of all services"
- "Find cause of service X failure"
- "Clean logs older than 7 days"
- "Apply configuration to server Y"

**Limitations:**
- ❌ Complex architectural decisions (require human expertise)
- ❌ Production changes without explicit confirmation
- ❌ Critical operations (delete data, change network configuration)

**Case Study: Virtual Machine (VM) Management**

**Situation:** A company has a large fleet of virtual machines distributed across multiple hosts and clusters. VM operations happen frequently: need to view VM lists, check which hosts they're placed on, assess resource availability for capacity planning, create new VMs, or modify settings (CPU, memory, disk size).

**Problem:** All these operations require an engineer who:
- May be unavailable when needed (blocking other processes)
- Must manually gather information from multiple sources
- May make errors in routine operations
- Spends time on simple but frequent tasks

**Solution:** The agent takes over routine operations and becomes a **full executor**, not just an assistant:

**Typical tasks the agent handles:**

1. **Inventory and placement:**
   - "Show list of all VMs"
   - "Which hosts are VMs from project X placed on?"
   - "How many VMs are on host web-01?"

2. **Capacity planning:**
   - "Are there enough resources in the cluster to create 5 new VMs?"
   - "Which host has the most available resources?"
   - "How much memory is available in the production cluster?"

3. **VM creation and modification:**
   - "Create a VM with 4 CPU, 8GB RAM, 100GB disk"
   - "Increase memory of VM app-01 to 16GB"
   - "Expand disk of VM db-01 by 50GB"

**Tools for VM management:**

```go
tools := []openai.Tool{
    {
        Function: &openai.FunctionDefinition{
            Name: "list_vms",
            Description: "Get list of all virtual machines. Use for inventory and finding VMs by name or project.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name: "get_vm_placement",
            Description: "Get VM placement information: which host/cluster the VM is on. Use for checking load distribution.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name: "get_cluster_capacity",
            Description: "Get information about available cluster resources (CPU, memory, disk). Use for capacity planning before creating new VMs.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name: "create_vm",
            Description: "CRITICAL: Create a new virtual machine. Requires confirmation. Parameters: name, CPU, memory, disk size, host/cluster.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name: "resize_vm",
            Description: "CRITICAL: Modify VM resources (CPU, memory). Requires confirmation. May affect production workloads.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name: "expand_disk",
            Description: "CRITICAL: Expand VM disk. Requires confirmation. Operation is irreversible.",
        },
    },
}
```

**SOP for critical operations (VM creation/modification):**

```text
SOP for creating/modifying VMs:
1. Check capacity: Are there enough resources in cluster/host?
2. Validation: Verify parameter correctness (CPU, memory, disk)
3. Confirmation: Request explicit user confirmation
4. Execution: Create/modify VM
5. Verification: Verify operation completed successfully
6. Notification: Notify user of result
```

**Safety and Best Practices:**

- ✅ **Confirmation for critical operations:** VM creation and resource modifications require explicit confirmation (see [Chapter 05: Safety](../05-safety-and-hitl/README.md))
- ✅ **Parameter validation:** Runtime validates CPU/RAM/disk correctness before execution
- ✅ **Evals for critical operations:** Tests verify agent requests confirmation for VM creation/modification
- ✅ **Logging:** All operations are logged for audit and debugging
- ✅ **Monitoring:** Resource usage and cost of created VMs are tracked

**Result:** The agent takes over routine VM management operations, freeing engineers for more complex tasks. At the same time, critical operations (creation, resource modifications) require confirmation and go through runtime validation, ensuring safety and control.

#### 2. Customer Support

**What agents do well:**
- ✅ Processing typical requests (FAQ, knowledge base)
- ✅ Gathering problem information (software version, OS, browser)
- ✅ Escalating complex cases
- ✅ Generating responses based on knowledge base

**Example tasks:**
- "User can't log into system"
- "Find solution for payment problem"
- "Gather information about ticket #12345"

**Limitations:**
- ❌ Emotional support (requires human empathy)
- ❌ Complex technical problems (require expertise)
- ❌ Legal questions

#### 3. Data Analytics

**What agents do well:**
- ✅ Formulating SQL queries from natural language
- ✅ Data quality checks
- ✅ Report generation
- ✅ Trend analysis

**Example tasks:**
- "Show sales for last month by region"
- "Check data quality in sales table"
- "Why did sales drop in region X?"

**Limitations:**
- ❌ Data modification (only read-only operations)
- ❌ Complex statistical analysis (requires expertise)
- ❌ Result interpretation (requires business context)

#### 4. Security (SOC)

**What agents do well:**
- ✅ Security alert triage
- ✅ Evidence collection (logs, metrics, traffic)
- ✅ Attack pattern analysis
- ✅ Incident report generation

**Example tasks:**
- "Triage alert about suspicious activity"
- "Gather evidence for incident #123"
- "Check IP address reputation"

**Limitations:**
- ❌ Critical actions (host isolation) require confirmation
- ❌ Complex investigations (require expertise)
- ❌ Blocking decisions (require context)

#### 5. Product Operations

**What agents do well:**
- ✅ Release plan preparation
- ✅ Dependency checking
- ✅ Documentation generation
- ✅ Task coordination

**Example tasks:**
- "Prepare release plan for feature X"
- "Check dependencies for release Y"
- "Create release notes for version 2.0"

**Limitations:**
- ❌ Strategic decision making (requires business context)
- ❌ Team management (requires human interaction)

### When NOT to Use Agents

#### 1. Critical Operations Without Confirmation

**❌ Bad:**
```go
// Agent deletes production database without confirmation
agent.Execute("Delete database prod")
```

**✅ Good:**
```go
// Agent requests confirmation
agent.Execute("Delete database prod")
// → "Are you sure? This action is irreversible. Type 'yes' to confirm."
```

#### 2. Tasks Requiring Creativity

**Agents struggle with:**
- Interface design
- Marketing copy writing (requires creativity and audience understanding)
- Architectural decisions (requires deep expertise)

#### 3. Tasks with High Uncertainty

**Agents work better when:**
- There are clear success criteria
- There is SOP or action algorithm
- Tools are available to get information

**Agents work worse when:**
- No clear success criteria
- Intuition and experience required
- No access to needed information

#### 4. Tasks Requiring Empathy

**Agents cannot:**
- Understand user emotions
- Provide emotional support
- Make decisions based on human relationships

## Common Errors

### Error 1: No Input Validation

**Symptom:** Agent performs dangerous actions with incorrect data or without security checks.

**Cause:** Runtime doesn't validate input data before executing tools.

**Solution:**
```go
// GOOD: Always validate input data
func executeTool(name string, args json.RawMessage) (string, error) {
    // 1. Check tool existence
    // 2. Parse and validate JSON
    // 3. Check required fields
    // 4. Sanitize data
    // 5. Security check
}
```

### Error 2: No Loop Protection

**Symptom:** Agent repeats the same action infinitely.

**Cause:** No iteration limit and no detection of repeating actions.

**Solution:**
```go
// GOOD: Loop protection
const maxIterations = 10
seenActions := make(map[string]int)

for i := 0; i < maxIterations; i++ {
    if i > 2 && seenActions[lastAction] > 2 {
        return fmt.Errorf("agent stuck in loop")
    }
    // ...
}
```

### Error 3: No Logging

**Symptom:** When there's a problem, you can't understand what happened and why.

**Cause:** Agent actions are not logged.

**Solution:**
```go
// GOOD: Log all actions
type AgentLog struct {
    Timestamp   time.Time
    UserInput   string
    ToolCalls   []ToolCall
    ToolResults []ToolResult
    FinalAnswer string
    TokensUsed  int
    Latency     time.Duration
}

logAgentRun(log)
```

## Completion Criteria / Checklist

✅ **Completed (production ready):**
- System prompt clearly defines responsibility boundaries
- All tools have detailed descriptions
- Input validation implemented
- Loop protection implemented
- Critical operations require confirmation
- All actions are logged
- Metrics monitoring configured (Pass Rate, Latency, Errors)
- Basic eval suite created
- Prompt A/B testing conducted
- Known limitations documented

❌ **Not completed:**
- No input validation
- No loop protection
- No action logging
- No metrics monitoring
- No evals for quality checks

## Connection with Other Chapters

- **Safety:** How to implement safety checks, see [Chapter 05: Safety](../05-safety-and-hitl/README.md)
- **Evals:** How to test agents, see [Chapter 08: Evals](../08-evals-and-reliability/README.md)

## What's Next?

After studying best practices, proceed to:
- **[17. Security and Governance](../17-security-and-governance/README.md)** — security and agent management
- **[Appendix: Reference Guides](../appendix/README.md)** — glossary, checklists, templates

---

**Navigation:** [← Case Studies](../15-case-studies/README.md) | [Table of Contents](../README.md) | [Security and Governance →](../17-security-and-governance/README.md)
