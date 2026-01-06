# 25. Production Readiness Index

## Why This Chapter?

You completed the main course and understand agent basics. Now you need to transition from a "learning agent" to a "production agent" that works reliably, safely, and efficiently in real environments.

This chapter is a **prioritization guide** and quick reference to production topics. It helps you understand what to implement first and provides quick links to detailed chapters.

### Real-World Case Study

**Situation:** You built a working agent locally. Want to deploy it to production, but don't know where to start.

**Problem:**
- Too many production topics to study at once
- Don't know what's critical vs. nice-to-have
- Need prioritization guide

**Solution:** This index provides a prioritized roadmap: start with mandatory blocks (observability, cost control, security), then add topics as you grow.

## Prioritization Guide

### Mode 1: Urgent to Production in 1 Day (Minimal Set)

If you need to launch an agent to production **right now**, start with these three topics:

1. **[19. Observability and Tracing](../19-observability-and-tracing/README.md)** — Without this you're blind. Needed immediately.
2. **[20. Cost & Latency Engineering](../20-cost-latency-engineering/README.md)** — Critical for budget control.
3. **[17. Security and Governance](../17-security-and-governance/README.md)** — Mandatory production block.

These three topics give you basic production readiness: you'll see what's happening, control costs, and protect the system.

### Mode 2: Planned Refinement in 1–2 Weeks (Extended Set)

If you have time for planned refinement, add topics as you grow:

**Week 1:**
- **[21. Workflow and State Management](../21-workflow-state-management/README.md)** — When agents execute long tasks
- **[22. Prompt and Program Management](../22-prompt-program-management/README.md)** — When prompts change frequently
- **[23. Evals in CI/CD](../23-evals-in-cicd/README.md)** — Automatic quality checking

**Week 2:**
- **[24. Data and Privacy](../24-data-and-privacy/README.md)** — If working with personal data

## Production Topics Overview

### Mandatory Production Blocks (Needed Immediately)

#### [19. Observability and Tracing](../19-observability-and-tracing/README.md)
**When needed:** Immediately, as soon as agent goes to production.

**What's inside:** Structured logging, tracing agent runs and tool calls, metrics (latency, token usage, error rate), log correlation via `run_id`.

#### [20. Cost & Latency Engineering](../20-cost-latency-engineering/README.md)
**When needed:** When agent is used actively or works with large contexts.

**What's inside:** Token budgets, iteration limits, caching, fallback models, batching, timeouts.

#### [17. Security and Governance](../17-security-and-governance/README.md)
**When needed:** Immediately, as soon as agent goes to production.

**What's inside:** Threat modeling for tool agents, risk scoring, prompt injection protection, RBAC for tools, dry-run modes, audit.

### Topics as You Grow

#### [21. Workflow and State Management in Production](../21-workflow-state-management/README.md)
**When needed:** When agents execute long tasks (minutes or hours), need idempotency or error handling with retry.

**What's inside:** Queues and asynchrony, scaling, distributed state. Basic concepts (idempotency, retries, deadlines) described in [Chapter 11: State Management](../11-state-management/README.md).

#### [22. Prompt and Program Management](../22-prompt-program-management/README.md)
**When needed:** When prompts change frequently, there are multiple versions, or need A/B testing.

**What's inside:** Prompt versioning, prompt regressions via evals, configs and feature flags, A/B testing.

#### [23. Evals in CI/CD](../23-evals-in-cicd/README.md)
**When needed:** When prompts or code change frequently and need automatic quality checking.

**What's inside:** Quality gates in CI/CD, dataset versioning, handling flaky cases, security tests.

### Specialized Topics

#### [24. Data and Privacy](../24-data-and-privacy/README.md)
**When needed:** When agent works with personal data (PII) or secrets.

**What's inside:** PII detection and masking, secret protection, log redaction, log storage and TTL.

**Note:** Production aspects of RAG and Multi-Agent are described in basic chapters [06](../06-rag/README.md) and [07](../07-multi-agent/README.md). Model selection and decoding configuration are described in [Chapter 01](../01-llm-fundamentals/README.md) and [Lab 00](../../labs/lab00-capability-check/README.md).

## Prioritization Algorithm

Don't try to study everything at once. Use this algorithm:

1. **Start with mandatory production blocks:**
   - Observability (logging, tracing) — needed immediately, without this you're blind
   - Cost & latency engineering — critical if agent is used actively
   - Security and Governance — mandatory production block

2. **Add topics as you grow:**
   - Workflow/state — when agents execute long tasks or need idempotency
   - Prompt/program management — when prompts change frequently or there are multiple versions
   - Evals in CI/CD — when need automatic quality checking

3. **Specialized topics:**
   - Data/privacy — if working with personal data
   - RAG/Multi-Agent production aspects — see production notes in [Chapter 06](../06-rag/README.md) and [Chapter 07](../07-multi-agent/README.md)

## Connection with Other Chapters

- **Basics:** Basic concepts studied in [Chapters 01-12](../README.md)
- **Advanced Patterns:** Ecosystem and patterns studied in [Chapters 13-18](../README.md)
- **Production Topics:** Detailed production guides in [Chapters 20-24](../README.md)

## What's Next?

After implementing production readiness, your agent is ready for deployment in the real world. Continue monitoring, iterations, and improvements based on production metrics and user feedback.

---

**Navigation:** [← Data and Privacy](../24-data-and-privacy/README.md) | [Table of Contents](../README.md) | [Appendix →](../appendix/README.md)
