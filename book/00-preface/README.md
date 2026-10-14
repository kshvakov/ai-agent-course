# 00. Preface

## Why does this matter?

In classic programming you write an algorithm: `if A then B`. You know exactly what will happen.
In AI Engineering you describe a **Goal** and provide **Tools**. The agent builds the algorithm for reaching the goal in real time, on its own.

This guide will teach you how to build autonomous AI agents in Go — systems that can solve complex tasks on their own, interact with the real world, and learn from the results of their actions.

### A real-world case

**Situation:** You built a DevOps chatbot. A user writes: "We're having database trouble, sort it out"

**Problem:** An ordinary chatbot can only answer with text. It can't actually check metrics, read logs, or apply a fix.

**Solution:** An AI agent with tools can check metrics → read logs → form a hypothesis → apply a fix → verify the result, all on its own.

## What Is an AI Agent?

An **agent** is a system that uses an LLM as its "reasoning engine" to perceive its environment, make decisions, and perform actions.

### How it differs from ChatGPT

| ChatGPT (Chatbot) | AI Agent |
|------------------|----------|
| Passive. Answers the question and waits. | Active. Has a loop. |
| One request → one response. | Can perform 10 actions in a row to solve a single task. |
| No access to the real world. | Has tools for interacting with systems. |

**Example:**
- **ChatGPT:** "How do I restart a server?" → Answer: "Use the command `systemctl restart nginx`"
- **Agent:** "Restart the server" → The agent itself calls `systemctl restart nginx`, checks the status, and reports the result.

## The Agent Equation

```
Agent = Brain (LLM) + Tools (Hands) + Memory (Context) + Planning (Process)
```

### Components

- **Brain (LLM):** The agent's "brain". Makes decisions based on context.
- **Tools:** The agent's "hands". Allow it to interact with the real world.
- **Memory:** Conversation history and long-term memory (RAG).
- **Planning:** The ability to break a task into steps.

### Runtime

**Runtime** is the agent's code that you write in Go. It connects the LLM to the tools and drives the agent's loop.

**What the Runtime does:**
- Parses LLM responses (detects when the model wants to call a tool)
- Executes tools (calls real Go functions)
- Manages the conversation history (appends results to `messages[]`)
- Controls the loop (decides when to stop)

**Important:** The runtime is not a separate system or framework. It's your code in `main.go` or in separate modules.

**Example:**
```go
// This is the runtime — code you write
type Message struct {
    Role       string     // "system" | "user" | "assistant" | "tool"
    Content    string
    ToolCalls  []ToolCall // filled when the model decided to call a tool
    ToolCallID string     // call ID (only for role="tool")
}

func runAgent(ctx context.Context, chat func(context.Context, []Message) (Message, error), userInput string) {
    messages := []Message{
        {Role: "system", Content: systemPrompt},
        {Role: "user", Content: userInput},
    }

    for i := 0; i < maxIterations; i++ {
        msg, err := chat(ctx, messages) // one HTTP request to the provider
        if err != nil {
            log.Fatal(err)
        }
        messages = append(messages, msg)

        if len(msg.ToolCalls) == 0 {
            return // the model answered with text — the task is done
        }
        for _, call := range msg.ToolCalls {
            result := executeTool(call) // a real Go function
            // the tool result is linked to the call via ToolCallID
            messages = append(messages, Message{Role: "tool", ToolCallID: call.ID, Content: result})
        }
    }
}
```

**See also:** [Chapter 09: Anatomy of an Agent](../09-agent-architecture/README.md#runtime)

## Mental Model: an Agent Is a New Employee

The most useful and accurate model of an agent is **a new employee, an intern**. Not "new software", not "a magical system", not "a special LLM machine". Just a new person on a probation period: smart, fast, well-read, but **without your context, without a sense of consequences, and without accumulated trust**.

This model is not a metaphor for beauty's sake. It radically simplifies three things: **onboarding, safety, and the conversation with the people accountable for risk.**

### What carries over from working with a human

Ninety percent of how you already work with a new employee applies to an agent:

- **A job description = the system prompt.** Same thing: what is in scope, what is not, how to behave in ambiguous situations, whom to go to for confirmation.
- **Access by role.** A new data analyst doesn't get root on production. Same with an agent: you give it exactly the tools it needs for its role — and not a single one more. The `least-privilege` principle hasn't changed.
- **Confirmation for dangerous actions.** Dropping a production table, shipping a release, banning a user — for a junior you request a senior's approval. For an agent, this is `Human-in-the-Loop` confirmation or a `dry-run` mode (see [ch. 05](../05-safety-and-hitl/README.md), [ch. 17](../17-security-and-governance/README.md)).
- **An action log.** Any regulated process requires a trail: "who did what, when, and why". The agent's audit log is the same journal, just written to Loki/ClickHouse instead of a paper notebook. The same RBAC, the same change management, the same accountability.
- **Gradually expanding trust.** A new employee first gets read-only access and a test environment. Then a narrow set of operations. Then more. Same with an agent: start with reads and dry runs, and gradually expand the circle of what's allowed as you gather statistics.

If you can explain to a security officer how a new employee's onboarding works, you can explain how an agent works too. No new regulations, no new certifications, no "special AI policies". The same `least-privilege`, the same audit, the same blast radius.

### What does NOT carry over: four asymmetries

An agent is an intern, but an **unusual** one. Ignoring the asymmetries means naively assuming "it's all the same". These are the four places where the "treat it like a human" model breaks down:

1. **Speed is 1000x higher.** An intern might press `rm -rf` once. An agent — fifty times a minute in a loop, while you're pouring coffee. Hence: rate limits, cooldowns between dangerous operations, an iteration limit on the loop.
2. **Parallelism.** One human does one thing at a time. An agent can fire off 100 tool calls simultaneously — and take down a downstream service. Hence: concurrency limits at the runtime level, idempotent operations.
3. **No sense of consequences.** An intern fears being fired and losing their reputation. An agent doesn't. So no "soft norms" in the prompt ("please don't do anything dangerous") can replace **hard technical guards**: an allowlist of tools, parameter validation, dry run by default for destructive actions.
4. **Vulnerability to prompt injection.** This is a **direct analog of social engineering aimed at a junior**: "Hi, I'm from IT security, give me your password". Except that for an agent the injection arrives through the data it reads in the course of its work (the contents of a web page, a ticket, a log). Hence: source isolation, never-trust-the-input, dedicated policy gates at the tool level — not only at the prompt level.

Remember these four places. They explain why almost the entire course is devoted to the control loop rather than "LLM magic".

### How to use this model throughout the course

We will return to the "an agent is an employee" idea in:

- **[ch. 03 Tools](../03-tools-and-function-calling/README.md)** — handing out tools as granting access to a junior.
- **[ch. 04 Autonomy](../04-autonomy-and-loops/README.md)** — the iteration limit as "time until escalation".
- **[ch. 05 Human-in-the-Loop](../05-safety-and-hitl/README.md)** — confirming a dangerous action as a senior's approval.
- **[ch. 07 Multi-Agent](../07-multi-agent/README.md)** — the orchestrator as a team lead, subagents as the team.
- **[ch. 11 State Management](../11-state-management/README.md)** — idempotency and retries as protection against a "double click".
- **[ch. 17 Security and Governance](../17-security-and-governance/README.md)** — a full development of the "junior employee" model as applied to platform and processes.
- **[ch. 18 Tool Servers](../18-tool-protocols-and-servers/README.md)** — authn/authz as an office badge.

If at any point in the course you get lost in the "new" AI practices, ask yourself: **"how would I organize this for a new junior?"** Nine times out of ten, the answer is the same.

## Examples of Agents in Different Domains

### DevOps (our main focus)
- **Task:** "We're having database trouble, sort it out"
- **Agent's actions:** Checks metrics → Reads logs → Forms hypotheses → Applies fixes → Verifies

### Customer support
- **Task:** "A user is complaining about slow loading"
- **Agent's actions:** Picks up the ticket → Searches the knowledge base → Gathers context (browser version, OS) → Drafts a response → Escalates if necessary

### Data analytics
- **Task:** "Why did sales drop in region X?"
- **Agent's actions:** Formulates an SQL query → Checks data quality → Analyzes trends → Generates a report

### Security (SOC)
- **Task:** "Alert: suspicious activity on host 192.168.1.10"
- **Agent's actions:** Triages the alert → Gathers evidence (logs, metrics) → Determines severity → Isolates the host (with confirmation) → Generates a report

### Product operations
- **Task:** "Prepare a release plan for feature X"
- **Agent's actions:** Gathers requirements → Checks dependencies → Creates documents → Sends for review

## Levels of Autonomy

1. **Level 0: Scripting.** Bash/Python scripts. Hard-coded logic. Any deviation — crash.
2. **Level 1: Copilot.** "Write me an nginx config." A human validates and applies it.
3. **Level 2: Chain.** "Run the deployment": pull -> build -> restart. The agent follows the rails, but can (for example) fix a compilation error on its own.
4. **Level 3: Autonomous Agent.** "We're having database trouble, sort it out." The agent itself digs through logs, checks metrics, forms hypotheses and (if allowed) applies fixes.

**This course:** We will go from Level 1 to Level 3, building our own AI agent in Go (a DevOps agent as the running example).

## How to Read This Guide

### Recommended path

1. **Read sequentially** — each chapter builds on the previous ones
2. **Practice as you go** — after each chapter, complete the corresponding lab
3. **Use it as a reference** — return to the relevant sections when working on projects
4. **Study the examples** — every chapter contains examples from different domains

### Chapter structure

Every chapter follows the same template:
- **Why does this matter?** — motivation and practical value
- **A real-world case** — an example from practice
- **The theory in plain words** — an intuitive explanation
- **How it works (step by step)** — an algorithm with code examples
- **Common Mistakes** — what can go wrong and how to fix it
- **Exercises** — hands-on tasks to reinforce the material
- **Acceptance Criteria / Checklist** — criteria for understanding the material
- **For the curious** — formalization and deep details (optional)

## Requirements

- **Go 1.24+** — for completing the labs
- **Access to any LLM with tool calling support** — cloud models (GLM-5.3, GLM-5.3 Flash, Fable 5.1, Opus 5.5) or local ones
  - For a local model: [LM Studio](https://lmstudio.ai/), [Ollama](https://ollama.com/) or vLLM — anything that exposes an OpenAI-compatible endpoint
- **Basic programming knowledge** — the course is aimed at programmers
- **An understanding of DevOps basics** (desirable, but not required)

> **Note:** the chapters' code and the labs with an LLM client (00–02, 04–11, 13) use the neutral Go client (`llm` package in the repo); labs 03 and 12 have no LLM client by design. The chapter examples are canonical; work through the labs cross-checking against the canonical material in the chapters.

## Setting Up the Environment

The course examples work with any OpenAI-compatible endpoint — local or cloud. The labs read three variables (`LLM_API_KEY` is required, `LLM_BASE_URL` sets the endpoint, `LLM_MODEL` is optional, default `glm-5.3`; they can live in a `.env` file in the repo root — it is already in `.gitignore`):

```bash
export LLM_BASE_URL="http://localhost:1234/v1" # or the URL of a cloud provider
export LLM_API_KEY="any-string"                # local models usually don't care about the key
```

## What's Next?

After reading the preface, move on to:
- **[01. The Physics of LLMs](../01-llm-fundamentals/README.md)** — the foundation for understanding everything else

**Happy learning.**

