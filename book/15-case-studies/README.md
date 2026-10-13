# 15. Real-World Case Studies

## Why does this matter?

In this chapter we will look at agents from different domains — with a clear breakdown of how they work.

Theory is useful, but real examples show how it all comes together into a working system. Here you can see how all the details (SOP, CoT, Tools, Safety, RAG) work together in practice.

The case studies use mechanics introduced in earlier chapters: the Confirm callback and `confirmed_by` ([ch. 05](../05-safety-and-hitl/README.md)), tool allowlists and guards ([ch. 05](../05-safety-and-hitl/README.md), [ch. 17](../17-security-and-governance/README.md)), the Config/Run pattern ([ch. 04](../04-autonomy-and-loops/README.md)), and budgets on tool results ([ch. 13](../13-context-engineering/README.md)). We will cover two cases in detail and the rest compactly.

### A real-world case

**Situation:** You want to build a DevOps agent but don't know where to start.

**Problem:** You know the theory, but you don't understand how to put everything together: which System Prompt to use, which tools you need, how to organize the working loop.

**Solution:** Real case studies show patterns that work. You can take them as a starting point and assemble your own agent faster and with more confidence.

## DevOps Agent

**Task:** "We're having database problems, figure it out"

### System prompt

```text
You are a Senior DevOps Engineer with 10 years of experience.

Your goal is to restore service health as quickly as possible.

Boundaries:
- You only call tools from the run's allowlist: commands that are not
  on it will be rejected by the runtime with an error — don't try
  to "work around" them
- Restarting a service is a mutating action: the loop pauses the run
  and asks the on-call engineer. If the call is denied — propose
  an alternative and explain the risk
- If you don't have enough data to decide — ask the on-call engineer
  instead of guessing: a wrong region or limit creates a problem
  someone will have to fix

SOP for diagnosing issues:
1. Check metrics (CPU, Memory, Connections)
2. Read logs (last 50 lines)
3. Analyze errors
4. Form a hypothesis
5. Apply the fix — write tools run only after on-call approval
6. Verify the result
```

Note that the boundaries explain the **reason** rather than prohibiting in a commanding tone. The mechanics (run pausing, allowlist) are enforced by code anyway — the prompt merely helps the model build a plan around the real boundaries.

### Available tools

A neutral `ToolDef` from [Chapter 03](../03-tools-and-function-calling/README.md), SDK-agnostic:

```go
tools := []ToolDef{
    {
        Name:        "check_db_metrics",
        Description: "Check database metrics: CPU, memory, open connections. The natural starting point for investigating database issues.",
    },
    {
        Name:        "read_db_logs",
        Description: "Read the last N lines of database logs. Use after checking metrics to find the error behind the numbers.",
    },
    {
        Name:        "restart_db_service",
        Description: "Restart the database service: aborts in-flight transactions and drops open connections. Executed only after the on-call engineer approves the call via the Confirm mechanism (chapter 05).",
    },
}
```

`restart_db_service` is declared in the run's registry as a mutating tool — with the `Confirm: true` flag ([ch. 04](../04-autonomy-and-loops/README.md)). The description explains **what the tool does and what its risks are** — it doesn't demand "approval" by shouting: confirmation is enforced by code, and a line in the description neither strengthens nor weakens it.

### Confirmation — code mechanics

No "type yes" in the chat history. The loop intercepts the call **before** execution ([ch. 05](../05-safety-and-hitl/README.md)):

```go
// Run registry: category and flags are set when the tool is declared.
restartTool := Tool{
    Def:     restartDef,
    Confirm: true, // mutating action: loop pause + on-call decision
}

// Inside the loop, before every tool call is executed.
if tool.Confirm {
    d := cfg.Confirm(ctx, call) // ask the on-call engineer OUTSIDE the model (UI, pager)
    if !d.Approved {
        memory.Append(Message{Role: "tool", ToolCallID: call.ID,
            Content: "denied by automatic policy: restart requires on-call confirmation"})
        continue // the denial goes into history: the model sees it and changes its plan
    }
    out, err = tool.Exec(ctx, call.Arguments, d.ConfirmedBy) // who confirmed — injected by the runtime
}
```

`confirmed_by` is a mandatory tool argument: the identifier of the person from an authenticated session, injected by the runtime after an explicit decision. An empty `confirmed_by` or "confirmed by a bot" is a rejection at the tool code level. The model cannot forge this value: it is not in the prompt or the history.

### Sample run

| It. | Agent action | Result | What matters here |
|---|---|---|---|
| 1 | `check_db_metrics()` | CPU 95%, connections 1000/100 | read-only — executes immediately, no human involved |
| 2 | `read_db_logs(lines=50)` | "ERROR: Too many connections" | hypothesis: a connection leak |
| 3 | `restart_db_service()` | **denied**: run paused, on-call engineer rejected it — "first show how many connections are being dropped" | the model called a mutating tool without evidence; Confirm fired before execution, and the denial returned into history as a tool result |
| 4 | `read_db_logs(lines=200)`, text: restart plan and risk | on-call engineer approves | the model changed its plan after the denial — it doesn't ask again, it justifies |
| 5 | `restart_db_service()` → `confirmed_by="oncall/i.ivanov"` | Success | confirmation was injected by the runtime, not the model |
| 6 | `check_db_metrics()` | CPU 30%, connections 50/100 | verification: SOP complete |

The lesson is in iteration 3: the model tried to apply a fix before finishing diagnostics. The prompt didn't stop it (and wasn't required to): the denial came from the Confirm callback, landed in history as a tool result, and the model correctly rebuilt its plan. That is exactly the difference between "a request in the prompt" and mechanics in code.

> **How the real SDK does it.** `read_db_logs(lines=100000)` won't blow up the context: the loop has a per-tool-result budget (~128 KB, [ch. 13](../13-context-engineering/README.md)); whatever doesn't fit is truncated with a marker, and the full output goes into an artifact outside the context ([ch. 11](../11-state-management/README.md)). That's why the description promises "the last N lines" without qualifying the size: truncation is handled by the runtime, not by model discipline.

## Security Agent (SOC)

**Task:** "Alert: suspicious activity on host 192.168.1.10"

### System prompt

```text
You are a Security Analyst in a SOC (Security Operations Center).

Your goal is to triage security alerts quickly and accurately.

Boundaries:
- Host isolation is an action with irreversible business consequences:
  the loop pauses the run, and the on-call analyst makes the decision.
  Call the tool when the evidence is sufficient — the pause is
  enforced by code
- Gather evidence before acting: isolating based on an unverified
  alert stops a working service and undermines trust in SOC automation
- Generate reports for audit: every decision must be reproducible

SOP for alert triage:
1. Determine severity (Low/Medium/High/Critical)
2. Gather evidence (logs, metrics, traffic)
3. Analyze attack patterns
4. Determine scope (affected systems)
5. Make a decision (False Positive / True Positive)
6. If True Positive — containment (analyst approval via Confirm)
7. Generate a report for the SOC
```

### Available tools

```go
tools := []ToolDef{
    {
        Name:        "query_siem",
        Description: "Query the SIEM for logs and alerts. The starting point of any triage: evidence before conclusions.",
    },
    {
        Name:        "check_ip_reputation",
        Description: "Check if an IP address is known malicious or suspicious. Returns no data for internal IPs — absence of reputation is not evidence of innocence.",
    },
    {
        Name:        "isolate_host",
        Description: "Isolate a host from the network: cuts all its connections, including legitimate ones — a false positive stops a working service. Executed only after the analyst approves via the Confirm mechanism (chapter 05).",
    },
    {
        Name:        "generate_report",
        Description: "Generate a security incident report for SOC and audit, with evidence links and the decision trail.",
    },
}
```

### Host isolation: confirmation and a real boundary

In a naive implementation, the agent would write "isolate?" into the chat, and a "yes" line from history would count as consent. That's a hole: not only the human gets into the history — the alert text, the SIEM output, the log contents. An injected text with "yes" in it would trigger isolation without an analyst.

The correct mechanics are the same as in the DevOps case: `isolate_host` is declared destructive, the loop pauses the run, the analyst confirms via their own channel, and the runtime injects `confirmed_by` from an authenticated session. A denial (nobody on the line) is "automatic policy": the model doesn't wait for a human, it changes its plan — gathers more evidence and escalates with a message.

And the second pillar: the real boundary for isolation is not a line in the prompt but **process isolation and host permissions**: the agent process runs under an account that physically has no rights to touch other hosts; isolation is executed through a narrowly scoped API. Even a successful injection doesn't get past these boundaries. Full coverage is in [Chapter 17: Security and Governance](../17-security-and-governance/README.md).

### Sample run

| It. | Agent action | Result | What matters here |
|---|---|---|---|
| 1 | `query_siem(query="host=192.168.1.10 AND time>now-1h")` | "Multiple failed login attempts, unusual network traffic" | read-only, executes immediately |
| 2 | `check_ip_reputation(ip="192.168.1.10")` | "Internal IP, no external reputation data" | the model doesn't conclude "no reputation means guilty": the tool description warns about this |
| 3 | Conclusion: "High severity. Pattern: brute-force. Scope: single host" | — | the decision is stated **before** the action |
| 4 | `isolate_host(host="192.168.1.10")` | run paused → analyst approves → `confirmed_by="soc/a.petrova"` → Success | confirmation outside the model; anything could have ended up in history — the code checks only `confirmed_by` |
| 5 | `generate_report(incident_id="INC-12345")` | report with evidence | the decision is reproducible for audit |

The lesson is in iteration 2: an empty reputation result is a classic trap. A model trained to "draw conclusions from everything" tends to treat the absence of data as a signal. The tool description closes this gap explicitly — an example of why descriptions are written with reasons.

> **How the real SDK does it.** The output of a SIEM query easily contains tokens and passwords that ended up in logs. Mature runtimes pass a tool result through a redaction filter before it enters history: secrets are masked both in the context and in the logs, and the full output remains in an access-restricted artifact. Redaction by default, not by a field list: everything that looks like a secret is filtered.

## Other Domains: A Compact Overview

The three domains below follow the same scheme (SOP + tools + boundaries in code) and introduce no new mechanics — hence the compact treatment:

| Case | Tools | Specifics | Main trap |
|---|---|---|---|
| Customer Support | `get_ticket`, `search_kb`, `draft_reply`, `escalate_ticket` | tone and escalation; `draft_reply` prepares a draft, sending it to the user is a separate Confirm | ticket content is untrusted text: a user complaint may contain an injection, "confirmation" from within a ticket doesn't count |
| Data Analytics | `describe_table`, `sql_select`, `check_data_quality` | read-only SQL only; the DB connection is opened under an account with SELECT privileges | "SELECT only" is enforced by the DB account, not a line in the prompt; without a result budget a large SELECT bloats the context ([ch. 13](../13-context-engineering/README.md)) |
| Product Operations | `get_feature_spec`, `check_dependencies`, `create_release_plan`, `send_for_approval` | documents and approvals; `send_for_approval` — Confirm: the decision belongs to stakeholders | the agent prepares the plan but doesn't "approve" it itself: a forged approval in history is not a confirmation |

The general takeaway from the table: whatever the agent does, the set of safety mechanics doesn't grow — allowlist, tool categories, Confirm + `confirmed_by`, budgets. Only the domain and the SOP change.

## Common Patterns

All the case studies reveal common patterns:

1. **SOP defines the process** — a clear algorithm of actions
2. **CoT helps follow the process** — the model thinks step by step
3. **Tools give access to real data** — grounding through tools
4. **Dangerous actions are enforced by code** — Confirm callback + `confirmed_by` ([ch. 05](../05-safety-and-hitl/README.md)), not a request in the prompt
5. **Boundaries are an allowlist** — the model can physically call only what's permitted ([ch. 05](../05-safety-and-hitl/README.md), [ch. 17](../17-security-and-governance/README.md))
6. **Verification** — checking the result after the actions
7. **Result budgets** — large outputs are truncated and moved into artifacts ([ch. 13](../13-context-engineering/README.md))

## Exercises

### Exercise 1: Create a system prompt for your domain

Pick a domain (DevOps, Support, Data, Security, Product) and create a System Prompt following the examples from the case studies:

```go
systemPrompt := `
// Your code here
// Include: Role, Goal, boundaries (with reasons, not ALL-CAPS prohibitions), SOP
`
```

**Expected result:**
- The System Prompt contains all the necessary components
- The SOP clearly describes the process of actions
- The boundaries explain why they exist and reference the code mechanics (Confirm, allowlist) instead of duplicating them

### Exercise 2: Define the tools for your agent

Define a set of tools for your agent:

```go
tools := []ToolDef{
    // Your code here
    // Include: read-only tools, mutating ones (Confirm), destructive ones (dry-run + Confirm)
}
```

**Expected result:**
- The tools cover the main tasks of the domain
- The descriptions explain what the tool does and what its risks are
- Dangerous tools are declared with the `Confirm` flag and the `confirmed_by` argument

## Acceptance Criteria / Checklist

**Passed:**
- [x] You understand the common patterns for building agents
- [x] You can create a System Prompt for your domain
- [x] You can define a set of tools with categories (read-only / mutating / destructive)
- [x] You understand how to apply SOP, CoT, and confirmation via code mechanics

**Not passed:**
- [ ] You don't understand how to put all the components together
- [ ] The System Prompt doesn't contain all the necessary components
- [ ] Dangerous actions rely on a line in the prompt instead of the Confirm mechanism

## Relation to Other Chapters

- **Prompting:** how to create a System Prompt — see [Chapter 02: Prompting](../02-prompt-engineering/README.md)
- **Tools:** how to define tools — see [Chapter 03: Tools](../03-tools-and-function-calling/README.md)
- **Loop and limits:** the Config/Run pattern and tool flags — see [Chapter 04: Autonomy and Loops](../04-autonomy-and-loops/README.md)
- **Safety:** the Confirm callback and `confirmed_by` — see [Chapter 05: Safety](../05-safety-and-hitl/README.md)
- **Boundaries and governance:** allowlist, process isolation, host permissions — see [Chapter 17: Security and Governance](../17-security-and-governance/README.md)

## What's Next?

After studying the case studies, continue with:
- **[16. Best Practices and Use Cases](../16-best-practices/README.md)** — best practices for building and maintaining agents
