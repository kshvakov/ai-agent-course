# Designing Autonomous AI Agents

For programmers who want to build production-grade AI agents

---

## Table of Contents

### Part I: Fundamentals

- **[00. Preface](./00-preface/README.md)** — How to read this guide, prerequisites, what an agent is
- **[01. The Physics of LLMs](./01-llm-fundamentals/README.md)** — Tokens, context, temperature and its limits, probabilistic nature
- **[02. Prompting as Programming](./02-prompt-engineering/README.md)** — ICL, Few-shot, CoT, structuring tasks, SOP

### Part II: Practice-first (Build an Agent)

- **[03. Tools and Function Calling](./03-tools-and-function-calling/README.md)** — JSON Schema, validation, error handling, the tool↔runtime contract
- **[04. Autonomy and Loops](./04-autonomy-and-loops/README.md)** — ReAct loop, termination, loop prevention, observability
- **[05. Safety and Human-in-the-Loop](./05-safety-and-hitl/README.md)** — Confirmation, clarification, tool categories, prompt injection
- **[06. RAG and the Knowledge Base](./06-rag/README.md)** — Chunking, retrieval, grounding, search modes, limits
- **[07. Multi-Agent Systems](./07-multi-agent/README.md)** — Supervisor/Worker, context isolation, task routing
- **[08. Evals and Reliability](./08-evals-and-reliability/README.md)** — Evals, prompt regressions, quality metrics, test datasets

### Part III: Architecture and Runtime Core

- **[09. Anatomy of an Agent](./09-agent-architecture/README.md)** — Memory, tools, planning, runtime
- **[10. Planning and Workflow Patterns](./10-planning-and-workflows/README.md)** — Plan→Execute, Plan-and-Revise, task decomposition, DAG/workflow, stopping conditions
- **[11. State Management](./11-state-management/README.md)** — Tool idempotency, retries with exponential backoff, deadlines, persisting state, task resumption
- **[12. Agent Memory Systems](./12-agent-memory/README.md)** — Two memory horizons (working memory within a run vs. long-term memory across sessions), linear history as an immutable log, prompt cache and why the system prompt is stable, compact vs. condense vs. recall, optional block-based memory
- **[13. Context Engineering](./13-context-engineering/README.md)** — A stable system prefix, a single threshold and a single response (condense with a 1/run limit), `usage.PromptTokens` as the primary source, over-engineering pitfalls (LayeredContext, message scoring, a dynamic system prompt)
- **[14. Ecosystem and Frameworks](./14-ecosystem-and-frameworks/README.md)** — Choosing between your own runtime and frameworks, portability, avoiding vendor lock-in

### Part IV: Practice (Case Studies)

- **[15. Real-World Case Studies](./15-case-studies/README.md)** — Examples of agents across domains (DevOps, Support, Data, Security, Product)
- **[16. Best Practices and Areas of Application](./16-best-practices/README.md)** — Best practices for building and maintaining agents, areas of application

### Part V: Infrastructure / Platform Security

- **[17. Security and Governance](./17-security-and-governance/README.md)** — Threat modeling, tool categorization, prompt injection defense (canonical), tool sandboxing, allowlists, policy-as-code, RBAC, dry-run modes, auditing
- **[18. Tool Protocols and Tool Servers](./18-tool-protocols-and-servers/README.md)** — The tool↔runtime contract at the process/service level, schema versioning, authn/authz

### Part VI: Production Readiness

- **[19. Observability and Tracing](./19-observability-and-tracing/README.md)** — Structured logging, tracing agent runs and tool calls, metrics, log correlation
- **[20. Cost & Latency Engineering](./20-cost-latency-engineering/README.md)** — Token budgets, iteration limits, caching, fallback models, timeouts
- **[21. Workflow and State Management in Production](./21-workflow-state-management/README.md)** — Queues and asynchrony, scaling, distributed state
- **[22. Prompt and Program Management](./22-prompt-program-management/README.md)** — Prompt versioning, prompt regressions through evals, configs and feature flags, A/B testing
- **[23. Evals in CI/CD](./23-evals-in-cicd/README.md)** — Quality gates in CI/CD, dataset versioning, handling flaky cases, security tests
- **[24. Data and Privacy](./24-data-and-privacy/README.md)** — PII detection and redaction, secret protection, log redaction, log retention and TTL
- **[25. Production Readiness Index](./25-production-readiness-index/README.md)** — A prioritization guide (1 day / 1–2 weeks) and quick links to production topics

### Appendices

- **[Appendix: Reference Material](./appendix/README.md)** — Glossary, checklists, SOP templates, decision tables, capability benchmark

---

## Reading Path

### For beginners (recommended path — practice-first)

1. **Start with the [Preface](./00-preface/README.md)** — what an agent is, the equation `Brain + Tools + Memory + Planning`, and **make sure to read** the section [“Mental Model: The Agent Is a New Employee”](./00-preface/README.md#mental-model-an-agent-is-a-new-employee). Without it, the security chapters will read like “special LLM machinery” instead of “the common sense you already know.”
2. **Study [The Physics of LLMs](./01-llm-fundamentals/README.md)** — the foundation for understanding everything else
3. **Master [Prompting](./02-prompt-engineering/README.md)** — this is the foundation of working with agents
4. **Build a working agent:**
    - [Tools and Function Calling](./03-tools-and-function-calling/README.md) — the agent's "hands"
    - [Autonomy and Loops](./04-autonomy-and-loops/README.md) — how the agent works in a loop
    - [Safety and Human-in-the-Loop](./05-safety-and-hitl/README.md) — protection against dangerous actions
5. **Extend its capabilities:**
    - [RAG and the Knowledge Base](./06-rag/README.md) — working with documentation
    - [Multi-Agent Systems](./07-multi-agent/README.md) — a team of specialized agents
    - [Evals and Reliability](./08-evals-and-reliability/README.md) — testing agents
6. **Dive deeper into the architecture:**
    - [Anatomy of an Agent](./09-agent-architecture/README.md) — components and how they interact
    - [Planning and Workflow Patterns](./10-planning-and-workflows/README.md) — planning complex tasks
    - [State Management](./11-state-management/README.md) — execution reliability
    - [Agent Memory Systems](./12-agent-memory/README.md) — linear memory within a run, long-term memory across sessions, prompt cache
    - [Context Engineering](./13-context-engineering/README.md) — a stable prefix, condense on overflow, avoiding over-engineering
7. **Practice:** Work through the labs in parallel with reading the chapters

### For experienced programmers

You can skip the basic chapters and go straight to:
- [Preface → Mental Model: The Agent Is a New Employee](./00-preface/README.md#mental-model-an-agent-is-a-new-employee) — 5 minutes; it explains the entire security part of the course with a single idea
- [Tools and Function Calling](./03-tools-and-function-calling/README.md)
- [Autonomy and Loops](./04-autonomy-and-loops/README.md)
- [Agent Memory Systems](./12-agent-memory/README.md) and [Context Engineering](./13-context-engineering/README.md) — memory and context without over-engineering
- [Case Studies](./15-case-studies/README.md) — to understand real-world applications

### Fast track: the core concepts in 10 minutes

If you are an experienced developer and want to grasp the essentials quickly:

1. **What is an agent?**
    - Agent = LLM + Tools + Memory + Planning
    - The LLM is the "brain" that makes decisions
    - Tools are the "hands" that carry out actions
    - Memory is the history and the long-term store
    - Planning is the ability to break a task into steps

2. **The mental model (more important than it seems right now):**
    - An agent is a **new employee on probation**, not "new software".
    - Role-based access, approval for dangerous actions, an audit log, gradual expansion of trust — the same as for a person.
    - Four asymmetries (where the "like a human" model breaks): speed 1000× faster, parallelism, no sense of consequences, prompt injection as social engineering.
    - In detail: [Preface → Mental Model](./00-preface/README.md#mental-model-an-agent-is-a-new-employee).

3. **How does the agent loop work?**
    ```
    While (task not solved):
      1. Send the history to the LLM
      2. Get a response (text or tool_call)
      3. If tool_call → execute the tool → add the result to the history → repeat
      4. If text → show it to the user and stop
    ```

4. **Key takeaways:**
    - The LLM does not execute code. It generates JSON with a request to execute.
    - The runtime (your code) runs the actual Go functions.
    - The LLM does not "remember" the past. It sees it in `messages[]`, which the runtime assembles.
    - Agent loops typically use a low `Temperature` (0.0–0.3); reproducibility comes from the schema, validation, and guards in code, not from temperature "determinism".
    - The history is an immutable log; the system prompt is stable across iterations (otherwise the prompt cache is lost — see [Ch. 12](./12-agent-memory/README.md), [Ch. 13](./13-context-engineering/README.md)).
    - Take the token count from the provider's `usage.PromptTokens` response, not from your own counters.

5. **A minimal example:**
    ```go
    // 1. Define the tools (the schema is a JSON Schema of the parameters)
    tools := []ToolDef{{
        Name:        "check_status",
        Description: "Check the server status by hostname",
        Parameters:  json.RawMessage(`{"type":"object","properties":{"host":{"type":"string"}},"required":["host"]}`),
    }}

    // 2. Request to the model (a single HTTP call to any provider with tool calling)
    resp, err := client.Complete(ctx, Request{
        Model: "glm-5.3",
        Messages: []Message{
            {Role: "system", Content: "You are a DevOps engineer"},
            {Role: "user", Content: "Check the status of the web-01 server"},
        },
        Tools: tools,
    })
    if err != nil {
        return err // client errors are handled by code, not by the model (Ch. 04)
    }

    // 3. Check for a tool_call
    if len(resp.Choices[0].Message.ToolCalls) > 0 {
        // 4. Execute the tool (runtime)
        result := checkStatus(resp.Choices[0].Message.ToolCalls[0].Arguments)
        // 5. Add the result to the history (referencing the call ID)
        messages = append(messages,
            resp.Choices[0].Message,
            Message{Role: "tool", ToolCallID: resp.Choices[0].Message.ToolCalls[0].ID, Content: result},
        )
        // 6. Send the updated history back to the model
    }
    ```

6. **What to read next:**
    - [Chapter 03: Tools](./03-tools-and-function-calling/README.md) — the detailed protocol
    - [Chapter 04: Autonomy](./04-autonomy-and-loops/README.md) — the agent loop
    - [Chapter 09: Anatomy of an Agent](./09-agent-architecture/README.md) — the architecture
    - [Chapter 12: Agent Memory](./12-agent-memory/README.md) and [Chapter 13: Context Engineering](./13-context-engineering/README.md) — without over-engineering

### After finishing the core course

After studying chapters 1–16, move on to:
- **[Part V: Infrastructure / Platform Security](./17-security-and-governance/README.md)** — security, governance, tool protocols
- **[Part VI: Production Readiness](./25-production-readiness-index/README.md)** — a practical guide to production readiness with step-by-step adoption recipes

---

## Mapping to the Labs

| Guide chapter | Corresponding labs |
|----------------|-------------------------------------|
| [01. The Physics of LLMs](./01-llm-fundamentals/README.md) | [Lab 00](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab00-capability-check) (Capability Check) |
| [02. Prompting](./02-prompt-engineering/README.md) | [Lab 01](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab01-basics) (Basics) |
| [03. Tools](./03-tools-and-function-calling/README.md) | [Lab 02](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab02-tools) (Tools), [Lab 03](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab03-real-world) (Architecture) |
| [04. Autonomy](./04-autonomy-and-loops/README.md) | [Lab 04](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab04-autonomy) (Autonomy) |
| [05. Safety](./05-safety-and-hitl/README.md) | [Lab 05](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab05-human-interaction) (Human-in-the-Loop) |
| [02. Prompting (SOP)](./02-prompt-engineering/README.md) | [Lab 06](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab06-incident) (Incident) |
| [06. RAG](./06-rag/README.md) | [Lab 07](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab07-rag) (RAG) |
| [07. Multi-Agent](./07-multi-agent/README.md) | [Lab 08](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab08-multi-agent) (Multi-Agent) |
| [09. Anatomy of an Agent](./09-agent-architecture/README.md) | [Lab 01](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab01-basics) (Basics), [Lab 09](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab09-context-optimization) (Context Optimization) |
| [10. Planning and Workflow Patterns](./10-planning-and-workflows/README.md) | [Lab 10](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab10-planning-workflows) (Planning & Workflow) |
| [11. State Management](./11-state-management/README.md) | [Lab 10](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab10-planning-workflows) (Planning & Workflow) — in part |
| [12. Agent Memory Systems](./12-agent-memory/README.md), [13. Context Engineering](./13-context-engineering/README.md) | [Lab 11](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab11-memory-context) (Memory & Context Engineering) |
| [18. Tool Protocols and Tool Servers](./18-tool-protocols-and-servers/README.md) | [Lab 12](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab12-tool-server) (Tool Server Protocol), [Lab 13](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab13-tool-retrieval) (Tool Retrieval & Pipelines) — optional |
| [17. Security and Governance](./17-security-and-governance/README.md) | — (read as a theory capstone; security practice is built into Lab 02 / Lab 05 / Lab 12) |
| [19–21, 23–25. Production Part](./25-production-readiness-index/README.md) | — (labs for production topics are planned; the theory is self-sufficient) |
| [22. Prompt and Program Management](./22-prompt-program-management/README.md) | [Lab 01](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab01-basics) (Basics) — in part |

> **Note:** labs 00–04 are revised to the neutral client; labs 05–13 are still on the old stack (the old client and old models) and will be updated in a separate pass. The examples in the chapters are canonical; work through the labs cross-checking against the canonical material in the chapters.

---

## How to Use This Guide

1. **Read sequentially** — each chapter builds on the previous ones
2. **Practice in parallel** — after each chapter, complete the corresponding lab
3. **Use it as a reference** — come back to the sections you need when working on projects
4. **Study the examples** — each chapter includes examples from different domains (DevOps, Support, Data, Security, Product)
5. **Do the exercises** — the exercises in each chapter help reinforce the material
6. **Test yourself** — use the checklists for self-assessment

---

**Happy learning.**
