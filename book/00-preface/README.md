# 00. Preface

## Why This Chapter?

In classical programming, you write an algorithm: `if A then B`. You know exactly what will happen.  
In AI engineering, you describe a **goal** and provide **tools**. The agent builds the algorithm to achieve the goal in real time.

This handbook teaches you how to create autonomous AI agents in Go — systems that can independently solve complex tasks, interact with the real world, and learn from the results of their actions.

### Real-World Case Study

**Situation:** You've created a chatbot for DevOps. A user writes: "We have database issues, investigate"

**Problem:** A regular chatbot can only respond with text. It cannot actually check metrics, read logs, or apply fixes.

**Solution:** An AI agent with tools can independently check metrics → read logs → form hypotheses → apply fixes → verify results.

## What Is an AI Agent?

An **agent** is a system that uses an LLM as a "reasoning engine" to perceive the environment, make decisions, and perform actions.

### Difference from ChatGPT

| ChatGPT (Chatbot) | AI Agent |
|------------------|----------|
| Passive. Answers questions and waits. | Active. Operates in a loop. |
| One request → one response. | Can perform 10 actions in a row to solve one task. |
| No access to real world. | Has tools for interacting with systems. |

**Example:**
- **ChatGPT:** "How to restart server?" → Answer: "Use command `systemctl restart nginx`"
- **Agent:** "Restart server" → The agent calls `systemctl restart nginx`, checks status, and reports the result.

## Agent Equation

```
Agent = Brain (LLM) + Tools (Hands) + Memory (Context) + Planning (Process)
```

### Components

- **Brain (LLM):** The agent's "brain" that makes decisions based on context.
- **Tools:** The agent's "hands" that enable interaction with the real world.
- **Memory:** Dialogue history and long-term memory (RAG).
- **Planning:** The ability to break tasks into steps.

### Runtime (Execution Environment)

**Runtime** is the agent code you write in Go. It connects the LLM with tools and manages the agent's work loop.

**What Runtime does:**
- Parses LLM responses (determines whether the model wants to call a tool)
- Executes tools (calls real Go functions)
- Manages dialogue history (adds results to `messages[]`)
- Manages the loop (determines when to stop)

**Important:** Runtime is not a separate system or framework. It's your code that you write in `main.go` or in separate modules.

**Example:**
```go
// This is Runtime — code you write
func runAgent(ctx context.Context, client *openai.Client, userInput string) {
    messages := []openai.ChatCompletionMessage{...}
    
    for i := 0; i < maxIterations; i++ {
        resp, _ := client.CreateChatCompletion(ctx, ...)
        msg := resp.Choices[0].Message
        
        if len(msg.ToolCalls) > 0 {
            // Runtime executes tool
            result := executeTool(msg.ToolCalls[0])
            // Runtime adds result to history
            messages = append(messages, ...)
        }
    }
}
```

**See also:** [Chapter 09: Agent Anatomy](../09-agent-architecture/README.md#runtime-execution-environment)

## Mental Model: an Agent Is a New Employee

The most useful and accurate model of an agent is **a new employee, an intern**. Not "new software", not "a magic system", not "a special LLM machine". Just a new person on probation: capable, fast, well-read — but **without your context, without a sense of consequences, without accumulated trust**.

This model is not a metaphor for decoration. It radically simplifies three things: **onboarding, security, and the conversation with people who own risk.**

### What Carries Over from Working with People

The way you already know how to work with a new employee applies to an agent about 90% of the time:

- **Job description = system prompt.** Same idea: what's in scope, what isn't, how to behave in ambiguous cases, who to escalate to.
- **Role-based access.** A new data analyst doesn't get root in production. Same with the agent: you give it exactly the tools it needs for its role and not one more. The `least-privilege` principle hasn't changed.
- **Approval for dangerous actions.** Drop a table in prod, ship a release, ban a user — for a junior you require sign-off from a senior. For an agent it's `Human-in-the-Loop` confirmation or `dry-run` mode (see [Ch. 05](../05-safety-and-hitl/README.md), [Ch. 17](../17-security-and-governance/README.md)).
- **Action log.** Any regulated process needs a trail: "who did what, when, why". The agent's audit log is the same journal — it just lives in Loki/ClickHouse instead of a paper notebook. Same RBAC, same change management, same accountability.
- **Trust expanded gradually.** A new employee starts with read-only access and a staging environment. Then a narrow set of operations. Then more. Same with the agent: start with reads and dry-run, broaden the allowed scope as you accumulate evidence that it's safe.

If you can explain to your security team how onboarding a new employee works, you can explain how the agent works. No new regulations, no new certifications, no "special AI policies" required. Same `least-privilege`, same audit, same blast radius.

### What Does NOT Carry Over: Four Asymmetries

The agent is an intern, but **an unusual one**. Ignoring the asymmetries means naively assuming "it's all the same". These are the four places where the "like with a person" model breaks:

1. **Speed is 1000× higher.** An intern manages to hit `rm -rf` once. An agent runs it fifty times in a minute inside a loop while you grab coffee. Hence: rate limits, cooldowns between dangerous operations, iteration limits on the loop.
2. **Parallelism.** A person does one thing at a time. An agent can fire 100 tool calls in parallel and overload a downstream service. Hence: concurrency limits at the runtime level, idempotent operations.
3. **No sense of consequences.** An intern fears being fired and damage to their reputation. An agent does not. So no "soft norms" in the prompt ("please don't do anything dangerous") replace **hard technical guards**: tool allowlists, parameter validation, dry-run-by-default for destructive actions.
4. **Vulnerability to prompt injection.** This is **the direct analog of social engineering against a junior**: "Hi, I'm from IT security, give me your password." Only for the agent the injection arrives through data (web pages, tickets, log lines) it reads as part of its job. Hence: source isolation, never-trust-the-input, separate policy gates at the tool layer — not just at the prompt layer.

Remember these four spots. They explain why almost the whole course is about the control loop and not about "LLM magic".

### How to Use This Model Throughout the Course

We will return to the "agent is an employee" idea in:

- **[Ch. 03 Tools](../03-tools-and-function-calling/README.md)** — granting tools = granting access to a junior.
- **[Ch. 04 Autonomy](../04-autonomy-and-loops/README.md)** — iteration limits = "time before escalation".
- **[Ch. 05 Human-in-the-Loop](../05-safety-and-hitl/README.md)** — confirmation for dangerous actions = sign-off from a senior.
- **[Ch. 07 Multi-Agent](../07-multi-agent/README.md)** — orchestrator as a tech lead, subagents as the team.
- **[Ch. 11 State Management](../11-state-management/README.md)** — idempotency and retries as protection against the "double click".
- **[Ch. 17 Security and Governance](../17-security-and-governance/README.md)** — full development of the "junior employee" model applied to the platform and processes.
- **[Ch. 18 Tool Servers](../18-tool-protocols-and-servers/README.md)** — authn/authz as a building badge.

If at any point in the course the "new" AI practices feel confusing — ask yourself: **"how would I set this up for a new junior?"** Nine times out of ten, the answer is the same.

## Agent Examples in Different Domains

### DevOps (our main focus)
- **Task:** "We have database issues, investigate"
- **Agent actions:** Checks metrics → Reads logs → Forms hypotheses → Applies fixes → Verifies

### Customer Support
- **Task:** "User complains about slow loading"
- **Agent actions:** Receives ticket → Searches knowledge base → Gathers context (browser version, OS) → Formulates response → Escalates if needed

### Data Analytics
- **Task:** "Why did sales drop in region X?"
- **Agent actions:** Formulates SQL query → Checks data quality → Analyzes trends → Generates report

### Security (SOC)
- **Task:** "Alert: suspicious activity on host 192.168.1.10"
- **Agent actions:** Triages alert → Gathers evidence (logs, metrics) → Determines severity → Isolates host (with confirmation) → Generates report

### Product Operations
- **Task:** "Prepare release plan for feature X"
- **Agent actions:** Gathers requirements → Checks dependencies → Creates documents → Sends for approval

## Autonomy Levels

1. **Level 0: Scripting.** Bash/Python scripts with rigid logic. Any deviation causes a crash.
2. **Level 1: Copilot.** "Write me nginx config". The human validates and applies.
3. **Level 2: Chain.** "Execute deployment": pull -> build -> restart. The agent follows a predefined path but can (for example) fix compilation errors itself.
4. **Level 3: Autonomous Agent.** "We have database issues, investigate". The agent searches logs, checks metrics, builds hypotheses, and (if allowed) applies fixes.

**This course:** You'll progress from Level 1 to Level 3, creating your AI agent in Go (using a DevOps agent as the main example).

## How to Read This Handbook

### Recommended Path

1. **Read sequentially** — each chapter builds on previous ones
2. **Practice alongside reading** — complete the corresponding lab after each chapter
3. **Use as a reference** — return to relevant sections when working on projects
4. **Study examples** — each chapter includes examples from different domains

### Chapter Structure

Each chapter follows a unified template:
- **Why is this needed?** — motivation and practical value
- **Real-world case** — practical example
- **Theory in simple terms** — intuitive explanation
- **How it works** — step-by-step algorithm with code examples
- **Common errors** — what can go wrong and how to fix it
- **Mini-exercises** — practical assignments for reinforcement
- **Checklist** — criteria for understanding the material
- **For the curious** — formalization and deep details (optional)

## Requirements

- **Go 1.21+** — for completing labs
- **Local LLM** (recommended) or OpenAI API Key
  - Install [LM Studio](https://lmstudio.ai/) or [Ollama](https://ollama.com/)
  - Start local server (usually on port 1234 or 11434)
- **Basic programming knowledge** — this course is designed for programmers
- **Understanding of DevOps basics** (helpful but not required)

## Environment Setup

To work with a local model (e.g., in LM Studio):

```bash
export OPENAI_BASE_URL="http://localhost:1234/v1"
export OPENAI_API_KEY="any-string" # Local models usually don't care about key
```

## What's Next?

After reading the preface, proceed to:
- **[01. LLM Physics](../01-llm-fundamentals/README.md)** — the foundation for understanding everything else

**Happy learning.**
