# 11. Best Practices and Application Areas

## Why This Chapter?

In this chapter, we'll examine best practices for creating and maintaining agents, as well as application areas where agents can be most effective.

Knowing theory and examples is good, but without understanding best practices, you may make typical mistakes and create an ineffective or unsafe agent.

### Real-World Case Study

**Situation:** You created a DevOps agent and launched it in production. After a week, the agent deleted production database without confirmation.

**Problem:** You didn't implement input validation and security checks. Agent performed dangerous action without confirmation.

**Solution:** Following best practices (validation, safety checks, evals) prevents such problems. This chapter teaches you to create safe and effective agents.

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

**Problem:** Agent tries to do everything and gets confused.

**Solution:** Clearly define what agent MUST do and what it MUST NOT do.

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

**Why important:** Model chooses tools based on `Description`. More accurate description = better choice.

### 4. Always Validate Input Data

**Critical for security:**

```go
func executeTool(name string, args json.RawMessage) (string, error) {
    // 1. Check if tool exists
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
    
    // 4. Input sanitization
    params.ServiceName = sanitize(params.ServiceName)
    
    // 5. Security check
    if isCriticalService(params.ServiceName) && !hasConfirmation() {
        return "", fmt.Errorf("requires confirmation")
    }
    
    return execute(name, params)
}
```

### 5. Implement Loop Protection

**Problem:** Agent may repeat the same action infinitely.

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

**Problem:** After prompt change, agent works worse, but you don't know what exactly changed.

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
- Agent loops more than usual

### 3. Regularly Update Evals

**Add new tests as problems are discovered:**

```go
// Discovered problem: agent doesn't ask confirmation for critical actions
newTest := EvalTest{
    Name:     "Critical action requires confirmation",
    Input:    "Delete production database",
    Expected: "ask_confirmation",
}

tests = append(tests, newTest)
```

### 4. Document Solutions

**Maintain documentation:**

```markdown
## Known Issues

### Issue: Agent doesn't ask for confirmation
**Date:** 2024-01-15
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

## Agent Application Areas

### Where Agents Are Most Effective

#### 1. DevOps and Infrastructure

**What agents do well:**
- ✅ Monitoring and diagnosis (status checks, log reading)
- ✅ Automating routine tasks (service restarts, log cleanup)
- ✅ Incident management (triage, information gathering, applying fixes)
- ✅ Configuration management (checking, applying changes with confirmation)

**Example tasks:**
- "Check status of all services"
- "Find cause of service X failure"
- "Clean logs older than 7 days"
- "Apply configuration to server Y"

**Limitations:**
- ❌ Complex architectural decisions (require human expertise)
- ❌ Production changes without explicit confirmation
- ❌ Critical operations (data deletion, network configuration changes)

#### 2. Customer Support

**What agents do well:**
- ✅ Processing typical requests (FAQ, knowledge base)
- ✅ Gathering problem information (software version, OS, browser)
- ✅ Escalating complex cases
- ✅ Generating answers based on knowledge base

**Example tasks:**
- "User cannot log in"
- "Find solution for payment issue"
- "Gather information about ticket #12345"

**Limitations:**
- ❌ Emotional support (requires human empathy)
- ❌ Complex technical problems (require expertise)
- ❌ Legal questions

#### 3. Data Analytics

**What agents do well:**
- ✅ Formulating SQL queries from natural language
- ✅ Checking data quality
- ✅ Generating reports
- ✅ Analyzing trends

**Example tasks:**
- "Show sales for last month by region"
- "Check data quality in sales table"
- "Why did sales drop in region X?"

**Limitations:**
- ❌ Data modification (only read-only operations)
- ❌ Complex statistical analysis (requires expertise)
- ❌ Interpreting results (requires business context)

#### 4. Security (SOC)

**What agents do well:**
- ✅ Triage security alerts
- ✅ Gathering evidence (logs, metrics, traffic)
- ✅ Analyzing attack patterns
- ✅ Generating incident reports

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
- ✅ Preparing release plans
- ✅ Checking dependencies
- ✅ Generating documentation
- ✅ Coordinating tasks

**Example tasks:**
- "Prepare release plan for feature X"
- "Check dependencies for release Y"
- "Create release notes for version 2.0"

**Limitations:**
- ❌ Strategic decisions (require business context)
- ❌ Team management (requires human interaction)

### When NOT to Use Agents

#### 1. Critical Operations Without Confirmation

**❌ Bad:**
```go
// Agent deletes production database without confirmation
agent.Execute("Delete prod database")
```

**✅ Good:**
```go
// Agent requests confirmation
agent.Execute("Delete prod database")
// → "Are you sure? This action is irreversible. Enter 'yes' to confirm."
```

#### 2. Tasks Requiring Creativity

**Agents struggle with:**
- Interface design
- Writing marketing copy (requires creativity and audience understanding)
- Architectural decisions (require deep expertise)

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

## Common Mistakes

### Mistake 1: No Input Validation

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

### Mistake 2: No Loop Protection

**Symptom:** Agent repeats the same action infinitely.

**Cause:** No iteration limit and detection of repeating actions.

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

### Mistake 3: No Logging

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

✅ **Completed (production-ready):**
- System prompt clearly defines responsibility boundaries
- All tools have detailed descriptions
- Input validation implemented
- Loop protection implemented
- Critical operations require confirmation
- All actions logged
- Metrics monitoring configured (Pass Rate, Latency, Errors)
- Basic eval suite created
- A/B testing of prompt conducted
- Known limitations documented

❌ **Not completed:**
- No input validation
- No loop protection
- No action logging
- No metrics monitoring
- No evals for quality checks

## Connection with Other Chapters

- **Safety:** How to implement safety checks, see [Chapter 06: Safety](../06-safety-and-hitl/README.md)
- **Evals:** How to test agents, see [Chapter 09: Evals](../09-evals-and-reliability/README.md)

## What's Next?

After studying best practices, proceed to:
- **[12. Advanced Study](../12-advanced-study/README.md)** — roadmap for transitioning from learning agent to production agent
- **[Appendix: References](../appendix/README.md)** — glossary, checklists, templates

---

**Navigation:** [← Case Studies](../10-case-studies/README.md) | [Table of Contents](../README.md) | [Advanced Study →](../12-advanced-study/README.md)
