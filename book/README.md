# Textbook: Designing Autonomous AI Agents

**Version:** 2.0  
**For Course:** AI Agent Course  
**Target Audience:** Programmers who want to build production AI agents

## Translations

- **English (EN)** — `main` (this branch)
- **Русский (RU)** — [Russian version](../translations/ru/book/README.md)

---

## 📚 Table of Contents

### Part I: Fundamentals

- **[00. Preface](./00-preface/README.md)** — How to read the textbook, requirements, what is an agent
- **[01. LLM Physics](./01-llm-fundamentals/README.md)** — Tokens, context, temperature, determinism, probabilistic nature
- **[02. Prompting as Programming](./02-prompt-engineering/README.md)** — ICL, Few-Shot, CoT, task structuring, SOP

### Part II: Agent Architecture

- **[03. Agent Anatomy](./03-agent-architecture/README.md)** — Memory, Tools, Planning, Runtime
- **[04. Tools and Function Calling](./04-tools-and-function-calling/README.md)** — JSON Schema, validation, error handling, tool↔runtime contract
- **[05. Autonomy and Loops](./05-autonomy-and-loops/README.md)** — ReAct loop, stopping, anti-loops, observability

### Part III: Advanced Topics

- **[06. Safety and Human-in-the-Loop](./06-safety-and-hitl/README.md)** — Confirmation, Clarification, Risk Scoring, Prompt Injection
- **[07. RAG and Knowledge Base](./07-rag/README.md)** — Chunking, Retrieval, Grounding, search modes, limits
- **[08. Multi-Agent Systems](./08-multi-agent/README.md)** — Supervisor/Worker, context isolation, task routing
- **[09. Evals and Reliability](./09-evals-and-reliability/README.md)** — Evals, prompt regressions, quality metrics, test datasets

### Part IV: Practice

- **[10. Real-World Case Studies](./10-case-studies/README.md)** — Examples of agents in different domains (DevOps, Support, Data, Security, Product)
- **[11. Best Practices and Application Areas](./11-best-practices/README.md)** — Best practices for creating and maintaining agents, application areas
- **[12. Advanced Study](./12-advanced-study/README.md)** — Practical guide to production readiness: observability, cost engineering, workflow, governance, and other production topics with step-by-step recipes

### Appendices

- **[Appendix: Reference Guides](./appendix/README.md)** — Glossary, checklists, SOP templates, decision tables, Capability Benchmark

---

## 🗺️ Reading Path

### For Beginners (recommended path)

1. **Start with [Preface](./00-preface/README.md)** — learn what an agent is and how to work with the textbook
2. **Study [LLM Physics](./01-llm-fundamentals/README.md)** — the foundation for understanding everything else
3. **Master [Prompting](./02-prompt-engineering/README.md)** — this is the foundation of working with agents
4. **Move to [Architecture](./03-agent-architecture/README.md)** — how an agent is structured
5. **Practice:** Complete laboratory assignments in parallel with reading chapters

### For Experienced Programmers

You can skip basic chapters and go directly to:
- [Tools and Function Calling](./04-tools-and-function-calling/README.md)
- [Autonomy and Loops](./05-autonomy-and-loops/README.md)
- [Case Studies](./10-case-studies/README.md) — for understanding real-world applications

### After Completing the Main Course

After studying chapters 1-11, proceed to:
- **[12. Advanced Study](./12-advanced-study/README.md)** — practical guide to production readiness: observability, cost engineering, workflow, governance, and other production topics with step-by-step implementation recipes

---

## 🔗 Connection with Laboratory Assignments

| Textbook Chapter | Corresponding Laboratory Assignments |
|----------------|-------------------------------------|
| [01. LLM Physics](./01-llm-fundamentals/README.md) | Lab 00 (Capability Check) |
| [02. Prompting](./02-prompt-engineering/README.md) | Lab 01 (Basics) |
| [03. Agent Anatomy](./03-agent-architecture/README.md) | Lab 01 (Basics), Lab 09 (Context Optimization) |
| [04. Tools](./04-tools-and-function-calling/README.md) | Lab 02 (Tools), Lab 03 (Architecture) |
| [05. Autonomy](./05-autonomy-and-loops/README.md) | Lab 04 (Autonomy) |
| [06. Safety](./06-safety-and-hitl/README.md) | Lab 05 (Human-in-the-Loop) |
| [02. Prompting (SOP)](./02-prompt-engineering/README.md) | Lab 06 (Incident) |
| [07. RAG](./07-rag/README.md) | Lab 07 (RAG) |
| [08. Multi-Agent](./08-multi-agent/README.md) | Lab 08 (Multi-Agent) |
| [03. Agent Anatomy (Optimization)](./03-agent-architecture/README.md) | Lab 09 (Context Optimization) |

---

## 📖 How to Use This Textbook

1. **Read sequentially** — each chapter builds on previous ones
2. **Practice in parallel** — after each chapter, complete the corresponding laboratory assignment
3. **Use as a reference** — return to relevant sections when working on projects
4. **Study examples** — each chapter has examples from different domains (DevOps, Support, Data, Security, Product)
5. **Complete exercises** — mini-exercises in each chapter help reinforce the material
6. **Check yourself** — use checklists for self-assessment

---

**Happy Learning! 🚀**
