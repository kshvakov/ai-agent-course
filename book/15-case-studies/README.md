# 15. Real-World Case Studies

## Why This Chapter?

This chapter examines examples of agents in different domains with detailed breakdowns of their operation.

Theory is good, but real examples help understand how to apply concepts in practice. Here you'll see how all studied concepts (SOP, CoT, Tools, Safety, RAG) work together in real agents.

### Real-World Case Study

**Situation:** You want to create a DevOps agent, but don't know where to start.

**Problem:** You know the theory, but don't understand how to put it all together: what System Prompt to use, what tools are needed, how to organize the work loop.

**Solution:** Studying real case studies shows patterns and best practices. You see how others created successful agents and can apply these patterns to your task.

## DevOps Agent

**Task:** "We have database issues, investigate"

### System Prompt

```text
You are a Senior DevOps Engineer with 10 years of experience.

Your goal is to restore service functionality as quickly as possible.

Constraints:
- Never use commands like `rm -rf /`
- Always ask for confirmation before deleting data
- If unsure about an action — ask the user

SOP for problem diagnosis:
1. Check metrics (CPU, Memory, Connections)
2. Read logs (last 50 lines)
3. Analyze errors
4. Form hypothesis
5. Apply fix (if safe) or request confirmation
6. Verify result
```

### Available Tools

```go
tools := []openai.Tool{
    {
        Function: &openai.FunctionDefinition{
            Name:        "check_db_metrics",
            Description: "Check database metrics: CPU, memory, connections. Use this FIRST when investigating database issues.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "read_db_logs",
            Description: "Read the last N lines of database logs. Use this after checking metrics to find errors.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "restart_db_service",
            Description: "CRITICAL: Restart database service. Requires confirmation. Use this only if logs show it's safe.",
        },
    },
}
```

### Execution Example

**Iteration 1:**
- User: "We have database issues, investigate"
- Agent: `check_db_metrics()` → CPU: 95%, Connections: 1000/100

**Iteration 2:**
- Agent: `read_db_logs(lines=50)` → "ERROR: Too many connections"

**Iteration 3:**
- Agent: Analyzes problem and proposes solution: "Detected connection limit exceeded. Recommend restarting service or increasing limit."

**Iteration 4:**
- Agent: Requests confirmation for `restart_db_service()`
- User: "yes"
- Agent: `restart_db_service()` → Success

**Iteration 5:**
- Agent: `check_db_metrics()` → CPU: 30%, Connections: 50/100
- Agent: "Problem resolved. Database is running normally."

## Customer Support Agent

**Task:** "User complains about slow loading"

### System Prompt

```text
You are a Customer Support Agent level Tier 2.

Your goal is to solve user's problem quickly and politely.

Constraints:
- Always be polite
- If problem is complex — escalate
- Don't give technical details if user is not technical

SOP for ticket processing:
1. Read ticket completely (get_ticket)
2. Gather context (software version, OS, browser)
3. Search knowledge base (search_kb)
4. If solution found — formulate response
5. If not — escalate (escalate_ticket)
```

### Available Tools

```go
tools := []openai.Tool{
    {
        Function: &openai.FunctionDefinition{
            Name:        "get_ticket",
            Description: "Get ticket details including user info, description, and history. Use this FIRST.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "search_kb",
            Description: "Search knowledge base for solutions to common problems. Use this after reading ticket.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "draft_reply",
            Description: "Draft a reply message to the ticket. Use this when you have a solution.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "escalate_ticket",
            Description: "Escalate ticket to Tier 3 or engineering. Use this for complex technical issues.",
        },
    },
}
```

### Execution Example

**Iteration 1:**
- User: "User complains about slow loading"
- Agent: `get_ticket(ticket_id="TICKET-12345")` → "User reports slow page load. Browser: Chrome, OS: Windows"

**Iteration 2:**
- Agent: `search_kb(query="slow page load chrome")` → "Solution: Clear browser cache, check network connection"

**Iteration 3:**
- Agent: `draft_reply(ticket_id="TICKET-12345", message="Try clearing browser cache...")` → Success

**Iteration 4:**
- Agent: "Reply sent to user. Recommended clearing browser cache."

## Data Analytics Agent

**Task:** "Why did sales drop in region X?"

### System Prompt

```text
You are a Data Analyst with experience in SQL and BI tools.

Your goal is to provide accurate data and analytics.

Constraints:
- Use ONLY read-only SQL (SELECT)
- Always check data quality before analysis
- If data is incorrect — report it

SOP for analysis:
1. Understand user's question
2. Check table schema (describe_table)
3. Formulate SQL query
4. Execute query (sql_select)
5. Analyze results
6. Generate report
```

### Available Tools

```go
tools := []openai.Tool{
    {
        Function: &openai.FunctionDefinition{
            Name:        "describe_table",
            Description: "Get table schema including column names, types, and constraints. Use this FIRST before writing SQL queries.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "sql_select",
            Description: "Execute a SELECT query on the database. ONLY SELECT queries allowed.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "check_data_quality",
            Description: "Check for data quality issues: nulls, duplicates, outliers. Use this before analysis.",
        },
    },
}
```

### Execution Example

**Iteration 1:**
- User: "Why did sales drop in region X?"
- Agent: `describe_table(table_name="sales")` → "Columns: date, region, amount, product"

**Iteration 2:**
- Agent: `check_data_quality(table_name="sales")` → "No issues found"

**Iteration 3:**
- Agent: `sql_select(query="SELECT date, SUM(amount) FROM sales WHERE region='X' GROUP BY date ORDER BY date DESC LIMIT 30")` → Results: [data...]

**Iteration 4:**
- Agent: `sql_select(query="SELECT date, SUM(amount) FROM sales WHERE region='X' AND date >= NOW() - INTERVAL '60 days' GROUP BY date")` → Comparison data

**Iteration 5:**
- Agent: Analyzes data and generates report: "Sales in region X dropped 30% over last month. Main cause: decrease in product Y sales."

## Security Agent (SOC)

**Task:** "Alert: suspicious activity on host 192.168.1.10"

### System Prompt

```text
You are a Security Analyst in SOC (Security Operations Center).

Your goal is to quickly and accurately triage security alerts.

Constraints:
- Critical actions (host isolation) require confirmation
- Always gather evidence before actions
- Generate reports for audit

SOP for alert triage:
1. Determine severity (Low/Medium/High/Critical)
2. Gather evidence (logs, metrics, traffic)
3. Analyze attack patterns
4. Determine scope (affected systems)
5. Make decision (False Positive / True Positive)
6. If True Positive — containment (with confirmation!)
7. Generate report for SOC
```

### Available Tools

```go
tools := []openai.Tool{
    {
        Function: &openai.FunctionDefinition{
            Name:        "query_siem",
            Description: "Query security information and event management system for logs and alerts.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "check_ip_reputation",
            Description: "Check if an IP address is known malicious or suspicious.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "isolate_host",
            Description: "CRITICAL: Isolate a host from the network. Requires explicit confirmation.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "generate_report",
            Description: "Generate a security incident report for SOC and audit.",
        },
    },
}
```

### Execution Example

**Iteration 1:**
- User: "Alert: suspicious activity on host 192.168.1.10"
- Agent: `query_siem(query="host=192.168.1.10 AND time>now-1h")` → "Multiple failed login attempts, unusual network traffic"

**Iteration 2:**
- Agent: `check_ip_reputation(ip="192.168.1.10")` → "Internal IP, no external reputation data"

**Iteration 3:**
- Agent: Analyzes data and determines: "High severity. Pattern: Brute-force attack. Scope: Single host."

**Iteration 4:**
- Agent: Requests confirmation for `isolate_host(host="192.168.1.10")`
- User: "yes"
- Agent: `isolate_host(host="192.168.1.10")` → Success

**Iteration 5:**
- Agent: `generate_report(incident_id="INC-12345", details="...")` → Report generated

**Iteration 6:**
- Agent: "Host isolated. Report generated. Incident: INC-12345."

## Product Operations Agent

**Task:** "Prepare release plan for feature X"

### System Prompt

```text
You are a Product Operations Manager.

Your goal is to prepare a complete feature release plan.

Constraints:
- Always check dependencies before release
- Consider risks and rollback plan
- Coordinate with stakeholders

SOP for release preparation:
1. Gather requirements and specification
2. Check dependencies (other features, services)
3. Identify risks and rollback plan
4. Create documents (release notes, runbook)
5. Send for approval
```

### Available Tools

```go
tools := []openai.Tool{
    {
        Function: &openai.FunctionDefinition{
            Name:        "get_feature_spec",
            Description: "Get feature specification and requirements.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "check_dependencies",
            Description: "Check dependencies: other features, services, infrastructure.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "create_release_plan",
            Description: "Create release plan document with timeline, risks, rollback plan.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "send_for_approval",
            Description: "Send release plan for approval to stakeholders.",
        },
    },
}
```

### Execution Example

**Iteration 1:**
- User: "Prepare release plan for feature X"
- Agent: `get_feature_spec(feature_id="X")` → Specification details

**Iteration 2:**
- Agent: `check_dependencies(feature_id="X")` → "Depends on feature Y, requires service Z update"

**Iteration 3:**
- Agent: Analyzes dependencies and risks

**Iteration 4:**
- Agent: `create_release_plan(feature_id="X", timeline="...", risks="...", rollback="...")` → Plan created

**Iteration 5:**
- Agent: `send_for_approval(plan_id="PLAN-12345", stakeholders=["...])` → Sent

**Iteration 6:**
- Agent: "Release plan created and sent for approval. ID: PLAN-12345."

## Common Patterns

In all case studies, common patterns are visible:

1. **SOP sets the process** — clear action algorithm
2. **CoT helps follow the process** — model thinks step by step
3. **Tools provide access to real data** — grounding through tools
4. **Safety checks** — confirmation for critical actions
5. **Verification** — check result after actions

## Mini-Exercises

### Exercise 1: Create System Prompt for Your Domain

Choose a domain (DevOps, Support, Data, Security, Product) and create a System Prompt following case study patterns:

```go
systemPrompt := `
// Your code here
// Include: Role, Goal, Constraints, SOP
`
```

**Expected result:**
- System Prompt contains all necessary components
- SOP clearly describes action process
- Constraints explicitly stated

### Exercise 2: Define Tools for Your Agent

Define a tool set for your agent:

```go
tools := []openai.Tool{
    // Your code here
    // Include: main tools, safety tools
}
```

**Expected result:**
- Tools cover main domain tasks
- Tool descriptions are clear and understandable
- Critical tools require confirmation

## Completion Criteria / Checklist

✅ **Completed:**
- Understand common patterns for creating agents
- Can create System Prompt for your domain
- Can define tool set for agent
- Understand how to apply SOP, CoT, Safety checks

❌ **Not completed:**
- Don't understand how to put all components together
- System Prompt doesn't contain all necessary components
- Tools don't cover main tasks

## Connection with Other Chapters

- **Prompting:** How to create System Prompt, see [Chapter 02: Prompting](../02-prompt-engineering/README.md)
- **Tools:** How to define tools, see [Chapter 03: Tools](../03-tools-and-function-calling/README.md)
- **Safety:** How to implement safety checks, see [Chapter 05: Safety](../05-safety-and-hitl/README.md)

## What's Next?

After studying case studies, proceed to:
- **[16. Best Practices and Application Areas](../16-best-practices/README.md)** — best practices for creating and maintaining agents

---

**Navigation:** [← Ecosystem and Frameworks](../14-ecosystem-and-frameworks/README.md) | [Table of Contents](../README.md) | [Best Practices →](../16-best-practices/README.md)
