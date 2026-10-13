# 25. Production Readiness Index

## Why does this matter?

You have completed the main course and understand the basics of agents. It is now time to move from a "learning agent" to a "production agent" that runs reliably, safely and efficiently in real environments.

This chapter is a **prioritization guide** and a quick reference for production topics. It helps you decide what to implement first and provides quick links to the detailed chapters.

### A real-world case

**Situation:** You built a working agent locally. You want to deploy it to production but don't know where to start.

**Problem:**
- Too many production topics to learn at once
- You don't know what is critical vs. nice-to-have
- You need a prioritization guide

**Solution:** This index gives you a prioritized roadmap: start with the mandatory blocks (observability, cost control, security), then add topics as you grow.

## Prioritization Guide

### Mode 1: Urgent production launch in 1 day (minimal set)

If you need to get an agent into production **right now**, start with these three topics and one rule:

1. **[19. Observability and Tracing](../19-observability-and-tracing/README.md)** — Without it you are blind. Needed immediately.
2. **[20. Cost & Latency Engineering](../20-cost-latency-engineering/README.md)** — Critical for budget control.
3. **[17. Security and Governance](../17-security-and-governance/README.md)** — A mandatory production block.

And one rule — the **context budget**: the `tools` block is sent with every request and paid for by every request — 4 compact tools ≈ 742 tokens, and that is schemas only, before the system prompt and history. The source of truth on spending is `PromptTokens` from the response usage ([ch. 03](../03-tools-and-function-calling/README.md), [ch. 13](../13-context-engineering/README.md), [ch. 20](../20-cost-latency-engineering/README.md)).

These three topics give you basic production readiness: you will see what is happening, control costs, and protect the system.

### Mode 2: Planned hardening in 1–2 weeks (extended set)

If you have time for planned hardening, add topics as you grow:

**Week 1:**
- **[21. Workflow and State Management](../21-workflow-state-management/README.md)** — When agents run long tasks
- **[22. Prompt and Program Management](../22-prompt-program-management/README.md)** — When prompts change often
- **[23. Evals in CI/CD](../23-evals-in-cicd/README.md)** — Automated quality checks

**Week 2:**
- **[24. Data and Privacy](../24-data-and-privacy/README.md)** — If you handle personal data

## Overview of Production Topics

### Mandatory production blocks (needed immediately)

#### [19. Observability and Tracing](../19-observability-and-tracing/README.md)
**When needed:** Immediately, as soon as the agent goes to production.

**What's inside:** Structured logging, tracing of agent runs and tool calls, metrics (latency, token usage, error rate), log correlation via `run_id`.

#### [20. Cost & Latency Engineering](../20-cost-latency-engineering/README.md)
**When needed:** When the agent is used heavily or works with large contexts.

**What's inside:** Token budgets, iteration limits, caching, fallback models, timeouts.

#### [17. Security and Governance](../17-security-and-governance/README.md)
**When needed:** Immediately, as soon as the agent goes to production.

**What's inside:** Threat modeling for tool agents, tool categorization, protection against prompt injection, RBAC for tools, dry-run modes, auditing.

### Topics to add as you grow

#### [21. Workflow and State Management in Production](../21-workflow-state-management/README.md)
**When needed:** When agents run long tasks (minutes or hours), or you need idempotency or error handling with retry.

**What's inside:** Queues and asynchrony, scaling, distributed state. Basic concepts (idempotency, retries, deadlines) are covered in [Chapter 11: State Management](../11-state-management/README.md).

#### [22. Prompt and Program Management](../22-prompt-program-management/README.md)
**When needed:** When prompts change often, there are multiple versions, or A/B testing is needed.

**What's inside:** Prompt versioning, prompt regressions via evals, configs and feature flags, A/B testing.

#### [23. Evals in CI/CD](../23-evals-in-cicd/README.md)
**When needed:** When prompts or code change often and automated quality checks are needed.

**What's inside:** Quality gates in CI/CD, threshold calibration against a baseline, cost/latency gates, live evals in production, dataset versioning, handling flaky cases, security tests.

### Specialized topics

#### [24. Data and Privacy](../24-data-and-privacy/README.md)
**When needed:** When the agent handles personal data (PII) or secrets.

**What's inside:** PII masking at the logging/tracing boundary and at tool output (the model's input is untouched), redaction by default, secret protection, vault at build time, storage with TTL, GDPR and correlation ID.

**Note:** Production aspects of RAG and Multi-Agent are covered in the base chapters [06](../06-rag/README.md) and [07](../07-multi-agent/README.md). Model selection and decoding settings are covered in [Chapter 01](../01-llm-fundamentals/README.md) and [Lab 00](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab00-capability-check).

## Prioritization Algorithm

Don't try to learn everything at once. Use this algorithm:

1. **Start with the mandatory production blocks:**
    - Observability (logging, tracing) — needed immediately, without it you are blind
    - Cost & latency engineering — critical if the agent is used heavily
    - Security and Governance — a mandatory production block

2. **Add topics as you grow:**
    - Workflow/state — when agents run long tasks or idempotency is needed
    - Prompt/program management — when prompts change often or there are multiple versions
    - Evals in CI/CD — when automated quality checks are needed

3. **Specialized topics:**
    - Data/privacy — if you handle personal data
    - RAG/Multi-Agent production aspects — see the production notes in [Chapter 06](../06-rag/README.md) and [Chapter 07](../07-multi-agent/README.md)

## Relation to Other Chapters

- **Fundamentals:** Basic concepts are covered in [Chapters 01-12](../README.md)
- **Advanced patterns:** Ecosystem and patterns are covered in [Chapters 13-18](../README.md)
- **Production topics:** Detailed production guides — [chapters 19-24](../README.md); this chapter is their navigator

## What's Next?

After implementing production readiness, your agent is ready for deployment in the real world. Keep monitoring, iterating and improving based on production metrics and user feedback.

[Appendix](../appendix/README.md) — use it as a set of checklists when bringing an agent to production: glossary, checklists, SOP templates and decision tables.


