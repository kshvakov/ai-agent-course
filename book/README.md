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
- **[12. Agent Memory Systems](./12-agent-memory/README.md)** — Short/long-term memory, episodic/semantic memory, forgetting/TTL, memory verification, storage/retrieval
- **[13. Context Engineering](./13-context-engineering/README.md)** — Context layers, fact selection policies, summarization, token budgets, context assembly from state+memory+retrieval
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

1. **Start with [Preface](./00-preface/README.md)** — learn what an agent is and how to use this handbook
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
   - [Agent Memory Systems](./12-agent-memory/README.md) — long-term memory
   - [Context Engineering](./13-context-engineering/README.md) — context management
7. **Practice:** Complete laboratory assignments alongside reading chapters

### For Experienced Programmers

You can skip basic chapters and go directly to:
- [Tools and Function Calling](./03-tools-and-function-calling/README.md)
- [Autonomy and Loops](./04-autonomy-and-loops/README.md)
- [Case Studies](./15-case-studies/README.md) — for understanding real-world applications

### Quick Track: Core Concepts in 10 Minutes

If you're an experienced developer and want to quickly understand the essence:

1. **What is an agent?**
   - Agent = LLM + Tools + Memory + Planning
   - LLM is the "brain" that makes decisions
   - Tools are the "hands" that perform actions
   - Memory is history and long-term storage
   - Planning is the ability to break down a task into steps

2. **How does the agent loop work?**
   ```
   While (task not solved):
     1. Send history to LLM
     2. Get response (text or tool_call)
     3. If tool_call → execute tool → add result to history → repeat
     4. If text → show user and stop
   ```

3. **Key points:**
   - LLM doesn't execute code. It generates JSON with an execution request.
   - Runtime (your code) executes real Go functions.
   - LLM doesn't "remember" the past. It processes it in `messages[]`, which Runtime collects.
   - Temperature = 0 for deterministic agent behavior.

4. **Minimal example:**
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

5. **What to read next:**
   - [Chapter 03: Tools](./03-tools-and-function-calling/README.md) — detailed protocol
   - [Chapter 04: Autonomy](./04-autonomy-and-loops/README.md) — agent loop
   - [Chapter 09: Agent Anatomy](./09-agent-architecture/README.md) — architecture

### After Completing the Main Course

After studying chapters 1-16, proceed to:
- **[Part V: Platform Infrastructure/Security](./17-security-and-governance/README.md)** — security, governance, tool protocols
- **[Part VI: Production Readiness](./25-production-readiness-index/README.md)** — practical guide to production readiness with step-by-step implementation recipes

---

## Connection with laboratory assignments

| Handbook Chapter | Corresponding Laboratory Assignments |
|----------------|-------------------------------------|
| [01. LLM Physics](./01-llm-fundamentals/README.md) | Lab 00 (Capability Check) |
| [02. Prompting](./02-prompt-engineering/README.md) | Lab 01 (Basics) |
| [03. Tools](./03-tools-and-function-calling/README.md) | Lab 02 (Tools), Lab 03 (Architecture) |
| [04. Autonomy](./04-autonomy-and-loops/README.md) | Lab 04 (Autonomy) |
| [05. Safety](./05-safety-and-hitl/README.md) | Lab 05 (Human-in-the-Loop) |
| [02. Prompting (SOP)](./02-prompt-engineering/README.md) | Lab 06 (Incident) |
| [06. RAG](./06-rag/README.md) | Lab 07 (RAG) |
| [07. Multi-Agent](./07-multi-agent/README.md) | Lab 08 (Multi-Agent) |
| [09. Agent Anatomy](./09-agent-architecture/README.md) | Lab 01 (Basics), Lab 09 (Context Optimization) |
| [10. Planning and Workflow Patterns](./10-planning-and-workflows/README.md) | Lab 10 (Planning & Workflow) |
| [11. State Management](./11-state-management/README.md) | Lab 10 (Planning & Workflow) — partially |
| [12. Agent Memory Systems](./12-agent-memory/README.md), [13. Context Engineering](./13-context-engineering/README.md) | Lab 11 (Memory & Context Engineering) |
| [18. Tool Protocols and Tool Servers](./18-tool-protocols-and-servers/README.md) | Lab 12 (Tool Server Protocol) |
| [17. Security and Governance](./17-security-and-governance/README.md) | Lab 13 (Agent Security Hardening) — Optional |
| [22. Prompt and Program Management](./22-prompt-program-management/README.md) | Lab 01 (Basics) — partially |
| [23. Evals in CI/CD](./23-evals-in-cicd/README.md) | Lab 14 (Evals in CI) — Optional |

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
