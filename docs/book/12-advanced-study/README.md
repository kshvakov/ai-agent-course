# 12. Advanced Study: Production-Ready Agents

## Why This Chapter?

You've completed the basic course and created a working agent. But what's next? How to transition from a "learning agent" to a "production agent" that works reliably, safely, and efficiently in a real environment?

This chapter is a **practical guide to production readiness**. Here are topics that almost always appear when creating production agents, but aren't always obvious at the start. Each topic contains:
- **"When needed" criteria** — to understand if you need this topic right now
- **Step-by-step implementation recipes** — what exactly to do and where to integrate it into your code
- **Common mistakes and solutions** — to avoid stepping on the same rakes
- **Readiness checklists** — to verify everything works

### Real-World Case Study

**Situation:** You created a DevOps agent, it works locally. You launch it in production, and after a week:
- Agent performed an operation that cost $500 due to large token usage
- You can't understand why agent made wrong decision — no logs
- Agent hung on a task and didn't respond for 10 minutes
- User complained that agent didn't ask for confirmation before deleting data

**Problem:** Learning agent works, but isn't production-ready. No observability, cost control, error handling, security policies.

**Solution:** This chapter shows how to implement all necessary production blocks. Each topic is a complete document with recipes tied to your code from lab assignments.

## How to Use This Chapter?

### Mode 1: Urgent Production in 1 Day (Minimal Set)

If you need to launch agent in production **right now**, start with these three topics:

1. **[Observability and Tracing](observability.md)** — without this you're blind. Needed immediately.
2. **[Cost & Latency Engineering](cost_latency.md)** — critical for budget control.
3. **[Security and Governance](security_governance.md)** — mandatory production block.

These three topics give you basic production readiness: you'll see what's happening, control costs, and protect the system.

### Mode 2: Planned Refinement Over 1–2 Weeks (Extended Set)

If you have time for planned refinement, add topics as you grow:

**Week 1:**
- [Workflow and State Management](workflow_state.md) — when agents perform long tasks
- [Prompt and Program Management](prompt_program_mgmt.md) — when prompts change frequently
- [Evals in CI/CD](evals_in_cicd.md) — automatic quality checks

**Week 2:**
- [Data and Privacy](data_privacy.md) — if working with personal data
- [RAG in Production](rag_in_prod.md) — if using RAG
- [Multi-Agent in Production](multi_agent_in_prod.md) — if using Multi-Agent systems
- [Model and Decoding](model_and_decoding.md) — model selection and parameter tuning

## Topics for Advanced Study

### Mandatory Production Blocks (Needed Immediately)

#### [Observability and Tracing](observability.md)
**When needed:** Immediately, as soon as agent goes to production. Without observability, you can't understand what's happening with the agent.

**What's inside:** Structured logging, tracing agent runs and tool calls, metrics (latency, token usage, error rate), log correlation via `run_id`.

**Code connection:** Tied to `labs/lab04-autonomy/main.go` (agent loop) and `labs/lab02-tools/main.go` (tool execution).

#### [Cost & Latency Engineering](cost_latency.md)
**When needed:** When agent is used actively or works with large contexts. Critical for budget and performance control.

**What's inside:** Token budgets, iteration limits, caching, fallback models, batching, timeouts.

**Code connection:** Tied to `labs/lab09-context-optimization/main.go` (token counting) and `labs/lab04-autonomy/main.go` (ReAct loop).

#### [Security and Governance](security_governance.md)
**When needed:** Immediately, as soon as agent goes to production. Security is not an option, but a mandatory requirement.

**What's inside:** Threat modeling for tool-agents, risk scoring, prompt injection protection, RBAC for tools, dry-run modes, audit.

**Code connection:** Tied to `labs/lab05-human-interaction/main.go` (confirmations) and `labs/lab06-incident/SOLUTION.md` (determinism via SOP).

### Topics as You Grow

#### [Workflow and State Management](workflow_state.md)
**When needed:** When agents perform long tasks (minutes or hours), need idempotency or error handling with retry.

**What's inside:** Tool idempotency, retries with exponential backoff, deadlines, queues and asynchrony, persist state.

**Code connection:** Tied to `labs/lab04-autonomy/main.go` (agent loop) and `labs/lab06-incident/main.go` (long tasks).

#### [Prompt and Program Management](prompt_program_mgmt.md)
**When needed:** When prompts change frequently, there are multiple versions or need A/B testing.

**What's inside:** Prompt versioning, prompt regressions via evals, configs and feature flags, A/B testing.

**Code connection:** Tied to `labs/lab06-incident/SOLUTION.md` (SOP in prompts) and `labs/lab09-context-optimization/main.go` (model parameters).

#### [Evals in CI/CD](evals_in_cicd.md)
**When needed:** When prompts or code change frequently and need automatic quality checks.

**What's inside:** Quality gates in CI/CD, dataset versioning, handling flaky cases, security tests.

**Code connection:** Tied to [Chapter 09: Evals and Reliability](../09-evals-and-reliability/README.md).

### Specialized Topics

#### [Data and Privacy](data_privacy.md)
**When needed:** When agent works with personal data (PII) or secrets.

**What's inside:** PII detection and masking, secret protection, log redaction, log storage and TTL.

**Code connection:** Tied to `labs/lab05-human-interaction/main.go` (user input handling).

#### [RAG in Production](rag_in_prod.md)
**When needed:** If using RAG (see [Chapter 07](../07-rag/README.md)) and need reliability in production.

**What's inside:** Document versioning, freshness (currency), reranking, grounding, retrieval fault tolerance.

**Code connection:** Tied to `labs/lab07-rag/main.go` (RAG implementation).

#### [Multi-Agent in Production](multi_agent_in_prod.md)
**When needed:** If tools are many (20+) or tasks are heterogeneous (DevOps + Security + Data). Multi-Agent systems help divide responsibility and isolate contexts.

**What's inside:** Supervisor/Worker and context isolation, task routing, safety circuits, chain observability.

**Code connection:** Tied to `labs/lab08-multi-agent/main.go` (Multi-Agent system).

#### [Model and Decoding](model_and_decoding.md)
**When needed:** Immediately, at model selection and parameter tuning stage. Wrong model choice or decoding parameters — common cause of problems.

**What's inside:** Capability benchmark before development, determinism (Temperature = 0), structured outputs / JSON mode, model selection for task.

**Code connection:** Tied to `labs/lab00-capability-check/main.go` (benchmark) and `labs/lab06-incident/SOLUTION.md` (determinism).

## Prioritization Algorithm

Don't try to study everything at once. Use this algorithm:

1. **Start with mandatory production blocks:**
   - Observability (logging, tracing) — needed immediately, without this you're blind
   - Cost & latency engineering — critical if agent is used actively
   - Security and Governance — mandatory production block

2. **Add topics as you grow:**
   - Workflow/state — when agents perform long tasks or need idempotency
   - Access policies — when multiple users or different access levels
   - Prompt/program management — when prompts change frequently or multiple versions

3. **Specialized topics:**
   - RAG in production — if using RAG (see [Chapter 07](../07-rag/README.md))
   - Data/privacy — if working with personal data
   - Multi-Agent in production — if using Multi-Agent systems


## Connection with Other Chapters

- **Safety:** Basic safety concepts studied in [Chapter 06: Safety and Human-in-the-Loop](../06-safety-and-hitl/README.md)
- **RAG:** Basic RAG concepts studied in [Chapter 07: RAG and Knowledge Base](../07-rag/README.md)
- **Multi-Agent:** Basic Multi-Agent concepts studied in [Chapter 08: Multi-Agent Systems](../08-multi-agent/README.md)
- **Evals:** Basic eval concepts studied in [Chapter 09: Evals and Reliability](../09-evals-and-reliability/README.md)
- **Best Practices:** General practices studied in [Chapter 11: Best Practices](../11-best-practices/README.md)
- **LLM Physics:** Fundamental concepts studied in [Chapter 01: LLM Physics](../01-llm-fundamentals/README.md)
- **Tools:** Function Calling studied in [Chapter 04: Tools and Function Calling](../04-tools-and-function-calling/README.md)
- **Appendix:** References, templates and Capability Benchmark in [Appendix](../appendix/README.md)

---

**Navigation:** [← Best Practices](../11-best-practices/README.md) | [Table of Contents](../README.md) | [Appendix →](../appendix/README.md)
