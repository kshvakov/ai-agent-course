# Designing Autonomous AI Agents

For programmers who want to build production AI agents

---

## Table of contents

### Part I: Fundamentals

- **[00. Preface](./00-preface/README.md)** — How to use this handbook, requirements, and what an agent is
- **[01. LLM Physics](./01-llm-fundamentals/README.md)** — Tokens, context, temperature, determinism, probabilistic nature
- **[02. Prompting as Programming](./02-prompt-engineering/README.md)** — ICL, Few-Shot, CoT, task structuring, SOP

### Part II: Practice-first (build an agent)

- **[03. Tools and Function Calling](./03-tools-and-function-calling/README.md)** — JSON Schema, validation, error handling, tool↔runtime contract
- **[04. Autonomy and Loops](./04-autonomy-and-loops/README.md)** — ReAct loop, stopping, anti-loops, observability
- **[05. Safety and Human-in-the-Loop](./05-safety-and-hitl/README.md)** — Confirmation, Clarification, Risk Scoring, Prompt Injection
- **[06. RAG and Knowledge Base](./06-rag/README.md)** — Chunking, Retrieval, Grounding, search modes, limits
- **[07. Multi-Agent Systems](./07-multi-agent/README.md)** — Supervisor/Worker, context isolation, task routing
- **[08. Evals and Reliability](./08-evals-and-reliability/README.md)** — Evals, prompt regressions, quality metrics, test datasets

### Part III: Architecture and Runtime Core

- **[09. Agent Anatomy](./09-agent-architecture/README.md)** — Memory, Tools, Planning, Runtime
- **[10. Planning and Workflow Patterns](./10-planning-and-workflows/README.md)** — Plan→Execute, Plan-and-Revise, task decomposition, DAG/workflow, stop conditions
- **[11. State Management](./11-state-management/README.md)** — Tool idempotency, retries with exponential backoff, deadlines, persist state, task resumption
- **[12. Agent Memory Systems](./12-agent-memory/README.md)** — Two memory horizons (in-Run working memory vs cross-session long-term), linear history as an immutable log, prompt cache and why the system prompt stays stable, compact vs condense vs recall, optional block-based memory
- **[13. Context Engineering](./13-context-engineering/README.md)** — Stable system prefix, single threshold and single reaction (`condense` with a 1/Run cap), `usage.PromptTokens` as the primary source, common over-engineering traps (LayeredContext, message scoring, dynamic system prompt)
- **[14. Ecosystem and Frameworks](./14-ecosystem-and-frameworks/README.md)** — Choosing between custom runtime and frameworks, portability, avoiding vendor lock-in

### Part IV: Practice (case studies/practices)

- **[15. Real-World Case Studies](./15-case-studies/README.md)** — Examples of agents in different domains (DevOps, Support, Data, Security, Product)
- **[16. Best Practices and Application Areas](./16-best-practices/README.md)** — Best practices for creating and maintaining agents, application areas

### Part V: Platform Infrastructure/Security

- **[17. Security and Governance](./17-security-and-governance/README.md)** — Threat modeling, risk scoring, prompt injection protection (canonical), tool sandboxing, allowlists, policy-as-code, RBAC, dry-run modes, audit
- **[18. Tool Protocols and Tool Servers](./18-tool-protocols-and-servers/README.md)** — Tool↔runtime contract at process/service level, schema versioning, authn/authz

### Part VI: Production Readiness

- **[19. Observability and Tracing](./19-observability-and-tracing/README.md)** — Structured logging, tracing agent runs and tool calls, metrics, log correlation
- **[20. Cost & Latency Engineering](./20-cost-latency-engineering/README.md)** — Token budgets, iteration limits, caching, fallback models, batching, timeouts
- **[21. Workflow and State Management in Production](./21-workflow-state-management/README.md)** — Queues and asynchrony, scaling, distributed state
- **[22. Prompt and Program Management](./22-prompt-program-management/README.md)** — Prompt versioning, prompt regressions via evals, configs and feature flags, A/B testing
- **[23. Evals in CI/CD](./23-evals-in-cicd/README.md)** — Quality gates in CI/CD, dataset versioning, handling flaky cases, security tests
- **[24. Data and Privacy](./24-data-and-privacy/README.md)** — PII detection and masking, secret protection, log redaction, log storage and TTL
- **[25. Production Readiness Index](./25-production-readiness-index/README.md)** — Prioritization guide (1 day / 1–2 weeks) and quick links to production topics

### Appendices

- **[Appendix: Reference Guides](./appendix/README.md)** — Glossary, checklists, SOP templates, decision tables, Capability Benchmark

---

## Reading path

### For Beginners (recommended path — practice-first)

1. **Start with [Preface](./00-preface/README.md)** — what an agent is, the `Brain + Tools + Memory + Planning` equation, and **don't skip** the section [«Mental Model: an Agent Is a New Employee»](./00-preface/README.md#mental-model-an-agent-is-a-new-employee). Without it the security chapters read as "special LLM machinery" instead of "common sense you already know".
2. **Study [LLM Physics](./01-llm-fundamentals/README.md)** — the foundation for understanding everything else
3. **Master [Prompting](./02-prompt-engineering/README.md)** — the foundation of working with agents
4. **Build a working agent:**
    - [Tools and Function Calling](./03-tools-and-function-calling/README.md) — the agent's "hands"
    - [Autonomy and Loops](./04-autonomy-and-loops/README.md) — how agents work in loops
    - [Safety and Human-in-the-Loop](./05-safety-and-hitl/README.md) — protecting against dangerous actions
5. **Expand capabilities:**
    - [RAG and Knowledge Base](./06-rag/README.md) — working with documentation
    - [Multi-Agent Systems](./07-multi-agent/README.md) — teams of specialized agents
    - [Evals and Reliability](./08-evals-and-reliability/README.md) — testing agents
6. **Dive deeper into architecture:**
    - [Agent Anatomy](./09-agent-architecture/README.md) — components and their interactions
    - [Planning and Workflow Patterns](./10-planning-and-workflows/README.md) — planning complex tasks
    - [State Management](./11-state-management/README.md) — execution reliability
    - [Agent Memory Systems](./12-agent-memory/README.md) — linear in-Run memory, cross-session long-term memory, prompt cache
    - [Context Engineering](./13-context-engineering/README.md) — stable prefix, `condense` on overflow, no over-engineering
7. **Practice:** Complete laboratory assignments alongside reading chapters

### For Experienced Programmers

You can skip basic chapters and go directly to:
- [Preface → Mental Model: an Agent Is a New Employee](./00-preface/README.md#mental-model-an-agent-is-a-new-employee) — 5 minutes; explains the entire security part of the course with one idea
- [Tools and Function Calling](./03-tools-and-function-calling/README.md)
- [Autonomy and Loops](./04-autonomy-and-loops/README.md)
- [Agent Memory Systems](./12-agent-memory/README.md) and [Context Engineering](./13-context-engineering/README.md) — memory and context without over-engineering
- [Case Studies](./15-case-studies/README.md) — for understanding real-world applications

### Quick Track: Core Concepts in 10 Minutes

If you're an experienced developer and want to quickly understand the essence:

1. **What is an agent?**
    - Agent = LLM + Tools + Memory + Planning
    - LLM is the "brain" that makes decisions
    - Tools are the "hands" that perform actions
    - Memory is history and long-term storage
    - Planning is the ability to break down a task into steps

2. **Mental model (more important than it sounds right now):**
    - An agent is **a new employee on probation**, not "new software".
    - Role-based access, sign-off for dangerous actions, audit log, gradual trust expansion — same as for a person.
    - Four asymmetries (where the "like with a person" model breaks): 1000× higher speed, parallelism, no sense of consequences, prompt injection as social engineering.
    - Full version: [Preface → Mental Model](./00-preface/README.md#mental-model-an-agent-is-a-new-employee).

3. **How does the agent loop work?**
    ```
    While (task not solved):
      1. Send history to LLM
      2. Get response (text or tool_call)
      3. If tool_call → execute tool → add result to history → repeat
      4. If text → show user and stop
    ```

4. **Key points:**
    - LLM doesn't execute code. It generates JSON with an execution request.
    - Runtime (your code) executes real Go functions.
    - LLM doesn't "remember" the past. It processes it in `messages[]`, which Runtime collects.
    - Temperature = 0 for deterministic agent behavior.
    - History is an immutable log; the system prompt stays stable across iterations (otherwise prompt cache is lost — see [Ch. 12](./12-agent-memory/README.md), [Ch. 13](./13-context-engineering/README.md)).
    - Take token counts from `usage.PromptTokens` in the provider's response, not from your own counters.

5. **Minimal example:**
    ```go
    // 1. Define tool
    tools := []openai.Tool{{
        Function: &openai.FunctionDefinition{
            Name: "check_status",
            Description: "Check server status",
        },
    }}
    
    // 2. Request to model
    resp, _ := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: "gpt-4o-mini",
        Messages: []openai.ChatCompletionMessage{
            {Role: "system", Content: "You are a DevOps engineer"},
            {Role: "user", Content: "Check server status"},
        },
        Tools: tools,
    })
    
    // 3. Check tool_call
    if len(resp.Choices[0].Message.ToolCalls) > 0 {
        // 4. Execute tool (Runtime)
        result := checkStatus()
        // 5. Add result to history
        messages = append(messages, openai.ChatCompletionMessage{
            Role: "tool",
            Content: result,
        })
        // 6. Send updated history back to model
    }
    ```

6. **What to read next:**
    - [Chapter 03: Tools](./03-tools-and-function-calling/README.md) — detailed protocol
    - [Chapter 04: Autonomy](./04-autonomy-and-loops/README.md) — agent loop
    - [Chapter 09: Agent Anatomy](./09-agent-architecture/README.md) — architecture
    - [Chapter 12: Agent Memory](./12-agent-memory/README.md) and [Chapter 13: Context Engineering](./13-context-engineering/README.md) — without over-engineering

### After Completing the Main Course

After studying chapters 1-16, proceed to:
- **[Part V: Platform Infrastructure/Security](./17-security-and-governance/README.md)** — security, governance, tool protocols
- **[Part VI: Production Readiness](./25-production-readiness-index/README.md)** — practical guide to production readiness with step-by-step implementation recipes

---

## Connection with laboratory assignments

| Handbook Chapter | Corresponding Laboratory Assignments |
|----------------|-------------------------------------|
| [01. LLM Physics](./01-llm-fundamentals/README.md) | [Lab 00](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab00-capability-check) (Capability Check) |
| [02. Prompting](./02-prompt-engineering/README.md) | [Lab 01](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab01-basics) (Basics) |
| [03. Tools](./03-tools-and-function-calling/README.md) | [Lab 02](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab02-tools) (Tools), [Lab 03](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab03-real-world) (Architecture) |
| [04. Autonomy](./04-autonomy-and-loops/README.md) | [Lab 04](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab04-autonomy) (Autonomy) |
| [05. Safety](./05-safety-and-hitl/README.md) | [Lab 05](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab05-human-interaction) (Human-in-the-Loop) |
| [02. Prompting (SOP)](./02-prompt-engineering/README.md) | [Lab 06](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab06-incident) (Incident) |
| [06. RAG](./06-rag/README.md) | [Lab 07](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab07-rag) (RAG) |
| [07. Multi-Agent](./07-multi-agent/README.md) | [Lab 08](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab08-multi-agent) (Multi-Agent) |
| [09. Agent Anatomy](./09-agent-architecture/README.md) | [Lab 01](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab01-basics) (Basics), [Lab 09](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab09-context-optimization) (Context Optimization) |
| [10. Planning and Workflow Patterns](./10-planning-and-workflows/README.md) | [Lab 10](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab10-planning-workflows) (Planning & Workflow) |
| [11. State Management](./11-state-management/README.md) | [Lab 10](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab10-planning-workflows) (Planning & Workflow) — partially |
| [12. Agent Memory Systems](./12-agent-memory/README.md), [13. Context Engineering](./13-context-engineering/README.md) | [Lab 11](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab11-memory-context) (Memory & Context Engineering) |
| [18. Tool Protocols and Tool Servers](./18-tool-protocols-and-servers/README.md) | [Lab 12](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab12-tool-server) (Tool Server Protocol), [Lab 13](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab13-tool-retrieval) (Tool Retrieval & Pipelines) — Optional |
| [17. Security and Governance](./17-security-and-governance/README.md) | — (read as a theory capstone; security practice is embedded in Lab 02 / Lab 05 / Lab 12) |
| [22. Prompt and Program Management](./22-prompt-program-management/README.md) | [Lab 01](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab01-basics) (Basics) — partially |

---

## How to use this handbook

1. **Read sequentially** — each chapter builds on previous ones
2. **Practice alongside reading** — complete the corresponding laboratory assignment after each chapter
3. **Use as a reference** — return to relevant sections when working on projects
4. **Study examples** — each chapter includes examples from different domains (DevOps, Support, Data, Security, Product)
5. **Complete exercises** — mini-exercises in each chapter help reinforce the material
6. **Check your understanding** — use checklists for self-assessment

---

**Happy learning.**
