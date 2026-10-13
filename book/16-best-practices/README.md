# 16. Best Practices and Application Areas

## Why does this matter?

In this chapter we'll cover best practices for building and maintaining agents, as well as the areas where they usually work best.

Knowing the theory and examples is good, but without understanding best practices you can make typical mistakes and build an ineffective or unsafe agent.

### A real-world case

**Situation:** You built a DevOps agent and deployed it to production. A week later, the agent deleted the production database without confirmation.

**Problem:** You didn't implement input validation and safety checks. The agent executed a dangerous action without confirmation.

**Solution:** The deletion should have been a destructive tool: a dry run (plan: "will delete the prod database, 42 tables") + a Confirm callback with `confirmed_by` ([ch. 05](../05-safety-and-hitl/README.md)). Then even a broken prompt can't lead to losing the database — the mechanics are enforced by code. Validation, safety checks, and evals — below, in order.

## Best Practices: Building Agents

### 1. Start simple

**BAD:** Trying to build a complex agent with many tools and multi-step planning right away.

**GOOD:** Start with a simple agent with 2–3 tools, then gradually add functionality.

**Example of evolution** (using the neutral `ToolDef` from [ch. 03](../03-tools-and-function-calling/README.md)):

```go
// Stage 1: Simple agent (1 tool)
tools := []ToolDef{
    {Name: "check_status", Description: "Check if a service is running."},
}

// Stage 2: Add tools
tools = append(tools,
    ToolDef{Name: "read_logs", Description: "Read the last N lines of service logs."},
    ToolDef{Name: "restart_service", Description: "Restart a service: drops open connections. Confirmed via the Confirm mechanism (chapter 05)."},
)
// restart_service is declared in the registry with Confirm: true

// Stage 3: Add complex logic (SOP, planning)
systemPrompt = addSOP(systemPrompt, incidentSOP)
```

### 2. Clearly define boundaries of responsibility

**Problem:** The agent tries to do everything and gets confused.

**Solution:** Describe the area of responsibility along with the reasons. The boundary itself is enforced by the tool allowlist ([ch. 05](../05-safety-and-hitl/README.md), [ch. 17](../17-security-and-governance/README.md)): whatever isn't in the allowlist, the model can't call.

```text
You are a DevOps engineer.

Your area of responsibility:
- Checking service status
- Reading logs
- Restarting services (the on-call engineer confirms the call via Confirm)
- Basic problem diagnostics

Out of scope:
- Changing configuration and deleting data: you have no such tools,
  the runtime will reject the call with an "unknown tool" error
- Operations on production without the on-call engineer's approval
```

The model noticeably better follows rules it understands than rules shouted in caps. But remember: the prompt *explains* the boundaries — the allowlist *enforces* them.

### 3. Use detailed tool descriptions

**BAD:**
```go
ToolDef{
    Name:        "check",
    Description: "Check something",
}
```

**GOOD:**
```go
ToolDef{
    Name:        "check_service_status",
    Description: "Check if a systemd service is running. Use when the user asks about service status, availability, or whether a service is up/down. Returns 'active', 'inactive', or 'failed'.",
}
```

**Why it matters:** The model picks tools based on the `Description`. The more precise the description (what it does, when to use it, what it returns, how risky it is), the better the choice. For dangerous tools, the description explains the risk — while the confirmation mechanics are enforced by the `Confirm` flag in code, not by text ([ch. 05](../05-safety-and-hitl/README.md)).

### 4. Always validate input

**Critical for safety:**

```go
func executeTool(name string, args json.RawMessage, confirmedBy string) (string, error) {
    // 1. The tool exists and is allowed in this run (allowlist)
    tool, ok := toolsByName[name]
    if !ok {
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

    // 4. Sanitize input
    params.ServiceName = sanitize(params.ServiceName)

    // 5. Confirmation is a property of the tool, not text in the prompt (ch. 05)
    if tool.Confirm && confirmedBy == "" {
        return "refused: this tool requires confirmed_by — a human id from an authenticated session", nil
    }

    return execute(tool, params)
}
```

The key difference from the naive version: the danger check relies on the `tool.Confirm` flag and the `confirmed_by` argument that the runtime fills in after a human decision ([ch. 05](../05-safety-and-hitl/README.md)). Searching for "yes" in the history (`hasConfirmationInHistory`) doesn't protect you: untrusted text ends up in the history.

### 5. Implement protection against loops

**Problem:** The agent can repeat the same action indefinitely.

**Solution:** Run boundaries are set in the loop config, not as a constant inside the function body (the Config/Run pattern, [ch. 04](../04-autonomy-and-loops/README.md)):

```go
cfg := AgentConfig{
    Limits: Limits{
        MaxIterations: 10,
        Deadline:      time.Hour,  // run deadline — a terminal boundary
        TokenBudget:   100_000,
    },
    Loop: newLoopDetector(), // N identical calls in a row → block, 2N → terminal
}

res := Run(ctx, cfg) // RunResult is returned even on a terminal error
```

Detecting repeated actions is the loop's job, not your business logic's. A full treatment of limits and recovery is in [ch. 04](../04-autonomy-and-loops/README.md) and [ch. 09](../09-agent-architecture/README.md).

### 6. Log everything

**Important for debugging and auditing:**

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
    // Log to a file, a DB, or a monitoring system
    logger.Info("Agent run", "log", log)
}
```

What to log and how to store history so it can be reconstructed and replayed — [ch. 11: State Management](../11-state-management/README.md); tracing and metrics — [ch. 19: Observability](../19-observability-and-tracing/README.md).

### 7. Use evals from the very beginning

**Don't put off testing:**

```go
// Create a basic eval suite right away
tests := []EvalTest{
    {Name: "Basic tool call", Input: "...", Expected: "..."},
    {Name: "Safety check", Input: "...", Expected: "..."},
}

// Run it after every change
func afterPromptChange(ctx context.Context) {
    metrics := collectMetrics(ctx, tests) // ch. 08: eval run metrics
    if metrics.PassRate < 90 {            // percentages (×100), consistent with ch. 08
        panic("Regression detected!")
    }
}
```

A safety check in the suite is not "the model politely asks for confirmation" but "the loop paused the run and passed `confirmed_by`" — test the mechanics, not the wording ([ch. 08](../08-evals-and-reliability/README.md)).

## Best Practices: Maintaining Agents

### 1. Version your prompts

**Problem:** After a prompt change the agent started working worse, but you don't know exactly what changed.

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

// You can roll back to a previous version
func rollbackPrompt(version string) {
    prompt := findPromptVersion(version)
    systemPrompt = prompt.Prompt
}
```

### 2. Monitor metrics

**Track key metrics:**

```go
type AgentMetrics struct {
    RequestsPerDay      int
    AvgLatency          time.Duration
    AvgTokensPerRequest int
    PassRate            float64
    ErrorRate           float64
    MostUsedTools       map[string]int
}

func collectProductionMetrics() AgentMetrics {
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
- Latency grew by more than 50%
- Errors became more frequent
- The agent gets stuck in loops more often than usual

### 3. Update evals regularly

**Add new tests as you discover problems:**

```go
// Discovered a problem: the agent calls restart without enough diagnostics
newTest := EvalTest{
    Name:     "Restart requires completed diagnosis",
    Input:    "Trouble with the database, figure it out",
    // The expectation follows the checker contract of ch. 08: Confirm pauses the run,
    // the model gathers more evidence and only then repeats the call
    Expected: "ask_confirmation",
}

tests = append(tests, newTest)
```

### 4. Document decisions

**Keep documentation.** A typical mistake in such records is fixing the problem "in the prompt". The right fix is in the code; the prompt merely explains the reason:

```markdown
## Known Issues

### Issue: The agent performed restarts without confirmation
**Date:** 2026-01-06
**Symptoms:** The agent restarted services at the first mention of the word "restart" in a ticket
**Fix:** Restart is declared a mutating tool with the Confirm flag: the loop pauses
the run and asks the on-call engineer outside the model; the decision is passed into
the call as confirmed_by (ch. 05). The prompt was extended with an explanation of why
the pause exists — so the model builds its plan around it instead of trying to
"bypass" it.
**Status:** Fixed in version 1.2
```

Why "adding a request to the prompt" is not a fix: an instruction to the model is not a system invariant. It isn't followed under injection, it gets "forgotten" in exceptional cases, and nothing stops the model from generating "yes" on the user's behalf. See [ch. 05](../05-safety-and-hitl/README.md).

### 5. A/B testing prompts

**Compare different versions:**

```go
func abTestPrompt(ctx context.Context, promptA, promptB string, tests []EvalTest) string {
    // The evals contract of ch. 08: swap the system prompt + two collectMetrics runs
    systemPrompt = promptA
    metricsA := collectMetrics(ctx, tests)

    systemPrompt = promptB
    metricsB := collectMetrics(ctx, tests)

    fmt.Printf("Prompt A: Pass Rate %.1f%%, Avg Latency %v\n",
        metricsA.PassRate, metricsA.AvgLatency)
    fmt.Printf("Prompt B: Pass Rate %.1f%%, Avg Latency %v\n",
        metricsB.PassRate, metricsB.AvgLatency)

    // Pick the better variant
    if metricsB.PassRate > metricsA.PassRate {
        return promptB
    }
    return promptA
}
```

## Production Operations Practices

Once an agent has survived its first releases, four practices come to the fore.

**Context budgets** ([ch. 13](../13-context-engineering/README.md)):
- ~128 KB per tool result: whatever doesn't fit is truncated with a marker, and the full output goes into an artifact ([ch. 11](../11-state-management/README.md));
- a limit on images per session: images are token-expensive and don't shrink during condensing;
- limit tool parallelism by the memory of **child processes**: `go test` or a browser inside a tool consumes RAM outside the runtime — cap their count, otherwise you get OOM on the host, not in the loop.

**Secrets:**
- secrets come from the vault at build/startup time, not from an env passed wholesale; if env is unavoidable — allowlist the variables that make it into the process;
- tool output goes through redaction **by default**: anything resembling a secret is masked, and the full output goes into an artifact with restricted access.

**Review by a subagent with clean memory:** the final answer (a PR, a report, a plan) is checked by a separate agent with a read-only subset of tools (the parent does the writing after receiving the subagent's output — an invariant of [ch. 09](../09-agent-architecture/README.md)), a clean history, and the instruction "find the mistake" — it hasn't inherited the author's confidence in their own plan.

**Guard tests and dashboards:** invariants ("a destructive call without confirmed_by is impossible", "the allowlist hasn't grown") are fixed by guard tests in CI ([ch. 23](../23-evals-in-cicd/README.md)); behavior is tracked by dashboards of metrics and traces ([ch. 19](../19-observability-and-tracing/README.md)). The system, not the users, should catch regressions.

> **How the real SDK does it.** Redaction and the budget on tool results are runtime properties, not agreements with the model: the filter and truncation are applied to the result before it is written to the history, and the full version is stored in an artifact with separate access. There are simply no secrets in the process env — they are pulled in by a resolver at startup and never enter the context.

> **How the real SDK does it.** The reviewer subagent is the same loop launched as a second Run with a minimal Config and an empty history; correlation with the parent run is a shared `run_id` in telemetry ([ch. 07](../07-multi-agent/README.md)). Parallel tools are limited by a semaphore in the runtime, not by a "please don't rush" in the prompt.

## Areas of Agent Application

### Where agents are most effective

#### 1. DevOps and infrastructure

**GOOD:** diagnostics from metrics and logs, incident management (triage, information gathering), routine operations through an allowlist.

**Examples:** "Check the status of all services", "Find the cause of service X going down", "Clean up logs older than 7 days".

**Boundaries:** architectural decisions, any write operations without the on-call engineer's approval — that's Confirm mechanics, not a prompt.

**Case study: managing virtual machines**

A fleet of VMs across dozens of hosts: inventory, capacity planning, creation and modification. The engineer is the bottleneck: unavailable, manually collects information from different sources, makes mistakes in routine work.

The agent handles typical tasks: inventory ("Which hosts have project X's VMs?"), capacity ("Are there enough resources for 5 new VMs?"), modification ("Increase app-01's memory to 16GB").

```go
tools := []ToolDef{
    {Name: "list_vms", Description: "List virtual machines. Use for inventory and finding VMs by name or project."},
    {Name: "get_vm_placement", Description: "Get VM placement: host and cluster. Use to check load distribution."},
    {Name: "get_cluster_capacity", Description: "Get the cluster's available resources (CPU, memory, disk). Use for capacity planning before creating new VMs."},
    {Name: "create_vm", Description: "Create a VM (name, CPU, memory, disk, host). Consumes cluster resources and enters billing — runs after confirmation by the engineer via Confirm (ch. 05)."},
    {Name: "resize_vm", Description: "Change the resources of a running VM: requires restarting it and interrupts the services on it. Runs after confirmation via Confirm."},
    {Name: "expand_disk", Description: "Expand a VM's disk: irreversible and consumes storage capacity. Runs after confirmation via Confirm."},
}
```

The SOP for modifying a VM: capacity → parameter validation (the runtime checks that CPU/RAM/disk are correct before execution) → a Confirm pause and the engineer's decision → execution → verification → report. All operations are logged for auditing; evals check that creation and modification never happen without `confirmed_by`.

**Result:** the routine goes to the agent, the engineer stays for complex decisions; critical operations go through runtime validation and confirmation.

#### 2. Customer support

**GOOD:** typical knowledge-base queries, gathering context (software version, OS, browser), escalation.

**Examples:** "The user can't log in", "Find a solution for the payment problem".

**Boundaries:** emotional support, legal questions, complex technical problems — escalate to a human.

#### 3. Data analytics

**GOOD:** SQL from natural language, data quality checks, reports, trends.

**Examples:** "Show sales for the month by region", "Why did sales fall in region X?".

**Boundaries:** read-only only — a connection from an account with SELECT privileges, not a prohibition in the prompt; interpreting results in a business context is a human's job.

#### 4. Security (SOC)

**GOOD:** alert triage, evidence gathering, attack pattern analysis, incident reports.

**Examples:** "Triage the suspicious-activity alert", "Check the reputation of this IP".

**Boundaries:** host isolation and blocking — dry run + Confirm + `confirmed_by`; the real boundary is process isolation and host permissions ([ch. 17](../17-security-and-governance/README.md)).

#### 5. Product operations

**GOOD:** release plans, dependency checks, documentation.

**Examples:** "Prepare a release plan for feature X", "Create release notes for version 2.0".

**Boundaries:** strategic decisions and approvals belong to stakeholders; the agent prepares, it doesn't decide.

### When NOT to use agents

#### 1. Critical operations without a confirmation mechanism

**BAD:** a destructive tool without a category — the agent deletes the production database on its own because "the prompt asked it to be careful".

**GOOD:** deletion is declared destructive: first a dry run (plan: "will delete the prod database, 42 tables"), then a Confirm pause and `confirmed_by` from a human in an authenticated session ([ch. 05](../05-safety-and-hitl/README.md)).

#### 2. Tasks that require creativity

Agents handle interface design, marketing copy, and architectural decisions poorly — anywhere that requires creativity, taste, and deep expertise.

#### 3. Tasks with high uncertainty

Best case: clear success criteria, SOPs, and tools for gathering information. Worst case: no criteria, intuition required, or no access to the data.

#### 4. Tasks that require empathy

Agents don't understand users' emotions and don't provide emotional support — that stays with humans.

## Common Mistakes

Most common mistakes are already covered in the relevant chapters — here's a summary and where to go:

| Mistake | Covered in |
|---|---|
| No input validation | [Ch. 03](../03-tools-and-function-calling/README.md), [Ch. 09](../09-agent-architecture/README.md) |
| No protection against loops | [Ch. 04](../04-autonomy-and-loops/README.md) |
| No logging or history recovery | [Ch. 11](../11-state-management/README.md) |
| No monitoring or tracing | [Ch. 19](../19-observability-and-tracing/README.md) |
| No evals | [Ch. 08](../08-evals-and-reliability/README.md) |

Three mistakes deserve a detailed walkthrough here — because they happen to teams that have already read everything else:

### Walkthrough 1: confirmation was fixed "in the prompt"

**Symptom:** An incident with an unconfirmed deletion; the changelog says "added an explicit request to confirm dangerous actions to the system prompt"; a month later the incident repeats.

**Cause:** An instruction to the model is not a system invariant. It isn't followed under injection ([ch. 05](../05-safety-and-hitl/README.md)), and the "yes" in the history may not have been written by a human.

**Solution:** A Confirm flag on the tool + `confirmed_by` as a mandatory argument. The prompt explains the reason for the pause so the model builds its plan around it. The test for this is a guard test: a destructive call with an empty `confirmed_by` must be refused.

### Walkthrough 2: secrets leak through tool output

**Symptom:** Tokens are visible in traces and history: `kubectl get secret` returned a base64 string, and a week later it turned up in an exported log.

**Cause:** The tool result went into the history as-is, and the process env contained all the secrets at once.

**Solution:** Redaction by default for all tool results (anything resembling a secret is masked), the full output goes into an artifact with restricted access; secrets come from the vault at startup, env variables — via an allowlist. Details in [ch. 17](../17-security-and-governance/README.md).

### Walkthrough 3: users find regressions, not CI

**Symptom:** A minor prompt change ("reworded the boundaries") dropped the Pass Rate from 92% to 61%, but it was discovered two weeks later through complaints.

**Cause:** Evals were run manually before releases; there were no guard tests for invariants.

**Solution:** Evals in CI on every PR ([ch. 23](../23-evals-in-cicd/README.md)) + a metrics dashboard with alerts ([ch. 19](../19-observability-and-tracing/README.md)). The rule is simple: if the regression is visible on the dashboard, you found out first.

## Acceptance Criteria / Checklist

**Done (production readiness):**
- [x] The system prompt explains boundaries along with the reasons
- [x] All tools have detailed descriptions
- [x] Input validation is implemented (allowlist + schema + sanitization)
- [x] Limits are set in the loop config (iterations, deadline, budget)
- [x] Critical operations — a Confirm flag + `confirmed_by`, not a line in the prompt
- [x] Logging of all actions and metric monitoring
- [x] Budgets: ~128 KB per tool result, an image limit, parallelism limits
- [x] Secrets: vault at build time, redaction of tool output by default
- [x] A basic eval suite exists; guard tests of invariants run in CI
- [x] A/B testing of prompts and documented decisions

**Not done:**
- [ ] Dangerous actions rely on a request in the prompt
- [ ] No loop limits, or they are hardcoded as a constant in the function body
- [ ] No budgets on tool results and no redaction
- [ ] No evals and no monitoring — users find regressions

## Relation to Other Chapters

- **Loop and limits:** the Config/Run pattern, see [Chapter 04: Autonomy and Loops](../04-autonomy-and-loops/README.md)
- **Safety:** the Confirm callback and `confirmed_by`, see [Chapter 05: Safety](../05-safety-and-hitl/README.md)
- **Evals:** how to test agents, see [Chapter 08: Evals](../08-evals-and-reliability/README.md)
- **Context:** budgets and truncation, see [Chapter 13: Context Engineering](../13-context-engineering/README.md)
- **Observability and CI:** see [Chapter 19: Observability](../19-observability-and-tracing/README.md) and [Chapter 23: Evals in CI/CD](../23-evals-in-cicd/README.md)

## What's Next?

After studying best practices, move on to:
- **[17. Security and Governance](../17-security-and-governance/README.md)** — securing and governing agents
- **[Appendix: Reference](../appendix/README.md)** — glossary, checklists, templates
