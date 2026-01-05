# AI Agent Course 🤖

## Translations

- **English (EN)** — `main` (this branch)
- **Русский (RU)** — [Russian version](./translations/ru/README.md)

## Why This Course?

Welcome to a practical course on building autonomous AI agents in Go!

This course is designed for programmers who want to understand how modern LLM agents work "under the hood" and learn to apply them to solve real-world problems.

### Real-World Case Study

**Situation:** You've created a chatbot for DevOps. A user writes: "We have database issues, investigate"

**Problem:** A regular chatbot can only respond with text. It cannot actually check metrics, read logs, or apply fixes.

**Solution:** An AI agent with tools can independently check metrics → read logs → form hypotheses → apply fixes → verify results. This course will teach you to build such agents.

## Theory

**📚 Required Reading Before Starting:**

👉 **[TEXTBOOK: Designing Autonomous AI Agents](./book/README.md)**  
A complete textbook with theory, examples from different domains (DevOps, Support, Data, Security, Product), diagrams, and practical advice. The textbook is divided into chapters for easy study.

**What You'll Find in the Textbook:**
- LLM Physics — how the agent's "brain" works
- Prompting as Programming — controlling behavior through prompts
- Agent Architecture — components and their interactions
- Tools and Function Calling — the agent's "hands"
- Autonomy and Loops — ReAct Loop
- Safety and Human-in-the-Loop — protection from dangerous actions
- RAG and Knowledge Base — working with documentation
- Multi-Agent Systems — a team of specialized agents
- Evals and Reliability — testing agents
- Real-World Case Studies — examples of successful agents
- Best Practices — best practices for creating and maintaining agents

## Advanced Study

If you want to transition from a "learning agent" to a "production agent," study **[Chapter 12: Advanced Study](./book/12-advanced-study/README.md)** in the textbook.

There you'll find a practical guide to production readiness: observability and tracing, cost & latency engineering, workflow and state management, security and governance, prompt management, data/privacy, RAG in production, evals in CI/CD, and other practical topics with step-by-step implementation recipes tied to your code from the laboratory assignments.

## How to Take the Course

**Recommended Path:**

1. **Read the textbook:** Open [`book/README.md`](./book/README.md) and study the theory. This will take 1-2 hours but will give you a fundamental understanding.

2. **Start with Lab 00:** Check if your model is suitable for the course.
   ```bash
   cd labs/lab00-capability-check
   # Read METHOD.md before starting
   go run main.go
   ```

3. **Complete labs in order:**
   - Navigate to the lab folder (e.g., `cd labs/lab01-basics`)
   - **Read `METHOD.md`** — this is a study guide with theory, algorithms, and common mistakes
   - Read `README.md` with the assignment
   - Open `main.go`, where the code skeleton is already written
   - Implement missing parts marked with `// TODO` comments
   - If stuck — check `SOLUTION.md` (but try yourself first!)

4. **Run and test:** After each lab, run the code and verify it works.

**Important:** Each laboratory assignment has its own `METHOD.md` — a study guide that must be read before starting. It contains:
- Why this is needed (real-world case)
- Theory in simple terms
- Execution algorithm
- Common mistakes and solutions
- Mini-exercises
- Completion criteria

**Theory and Practice Connection:**
- After each textbook chapter, complete the corresponding laboratory assignment
- Use the textbook as a reference when working on projects
- Return to relevant sections when questions arise

## Course Structure

The course consists of a preparatory stage (Lab 00) and 9 main laboratory assignments (Lab 01-09).

### 🔬 [Lab 00: Model Benchmark](./labs/lab00-capability-check)
**Diagnostics.** Before starting, we'll verify whether your model (especially a local one) is suitable for the course. We'll run a series of tests on JSON, Instruction Following, and Function Calling.

### [Lab 01: Hello, LLM!](./labs/lab01-basics)
**Basics and Memory.** You'll learn to programmatically communicate with LLMs, manage context (memory), and configure the agent's role through system prompts.

### [Lab 02: Agent's Hands](./labs/lab02-tools)
**Function Calling.** Learn how to turn Go functions into tools that an LLM can call. Implement the Tool Execution mechanism.

### [Lab 03: Real-World Tools](./labs/lab03-real-world)
**Infrastructure as Code Integration.** We'll connect real APIs (Proxmox, Ansible) to our agent.

### [Lab 04: Autonomy Loop](./labs/lab04-autonomy)
**The Agent Loop.** We'll implement the ReAct pattern (Reason + Act). The agent will learn to independently make decisions, execute actions, and analyze results in a loop.

### [Lab 05: Human-in-the-Loop](./labs/lab05-human-interaction)
**Dialogue and Safety.** The agent will learn to ask clarifying questions ("In which region should I create the server?") and request confirmation before dangerous actions ("Are you sure you want to delete the database?").

### [Lab 06: Incident Management](./labs/lab06-incident)
**Complex Scenarios.** We'll create an agent capable of independently investigating and resolving failures (e.g., service crashes) using a toolkit of tools.

### [Lab 07: RAG & Knowledge Base](./labs/lab07-rag)
**Working with Knowledge.** The agent will learn to read internal documentation (Wiki/Man pages) before executing actions. We'll implement simple document search.

### [Lab 08: Agent Team (Multi-Agent)](./labs/lab08-multi-agent)
**Supervisor Pattern.** We'll create a system where a main agent (Orchestrator) manages highly specialized sub-agents (Network Admin, DB Admin).

### [Lab 09: Context Optimization](./labs/lab09-context-optimization)
**Context Window Management.** Learn to count tokens, apply optimization techniques (truncation, summarization), and implement adaptive context management for long-lived agents.

### 📋 Laboratory Assignments Table

| Lab | Topic | Key Skills | Study Guide |
| :--- | :--- | :--- | :--- |
| **Lab 00** | **Capability Check** | Model testing. Unit tests for LLMs. | [METHOD.md](./labs/lab00-capability-check/METHOD.md) |
| **Lab 01** | **Basics** | OpenAI API, Chat Loop, Memory Management. | [METHOD.md](./labs/lab01-basics/METHOD.md) |
| **Lab 02** | **Tools** | Function definitions (JSON Schema), parsing ToolCalls. | [METHOD.md](./labs/lab02-tools/METHOD.md) |
| **Lab 03** | **Architecture** | Go interfaces, Registry pattern, Mocking. | [METHOD.md](./labs/lab03-real-world/METHOD.md) |
| **Lab 04** | **Autonomy (ReAct)** | `Think-Act-Observe` loop. Result processing. | [METHOD.md](./labs/lab04-autonomy/METHOD.md) |
| **Lab 05** | **Human-in-the-Loop** | Interactivity. Clarifying questions. Safety. | [METHOD.md](./labs/lab05-human-interaction/METHOD.md) |
| **Lab 06** | **Incident (SOP)** | Advanced planning. SOP integration in prompts. | [METHOD.md](./labs/lab06-incident/METHOD.md) |
| **Lab 07** | **RAG** | Working with documentation. Knowledge search before action. | [METHOD.md](./labs/lab07-rag/METHOD.md) |
| **Lab 08** | **Multi-Agent** | Orchestration. Task delegation. Context isolation. | [METHOD.md](./labs/lab08-multi-agent/METHOD.md) |
| **Lab 09** | **Context Optimization** | Token counting, summarization, adaptive context management. | [METHOD.md](./labs/lab09-context-optimization/METHOD.md) |

## Requirements

*   Go 1.21+
*   **Local LLM** (recommended) or OpenAI API Key.
    *   Install [LM Studio](https://lmstudio.ai/) or [Ollama](https://ollama.com/).
    *   Start a local server (usually on port 1234 or 11434).
*   Docker (optional)

## Environment Setup

To work with a local model (e.g., in LM Studio):
```bash
export OPENAI_BASE_URL="http://localhost:1234/v1"
export OPENAI_API_KEY="any-string" # Local models usually don't need a key, but it shouldn't be empty
```

## Project Structure

```
ai-agent-course/
├── book/               # Textbook on agent design (English)
│   ├── 01-llm-fundamentals/
│   ├── 02-prompt-engineering/
│   ├── ...             # Other chapters
│   └── README.md
├── translations/       # Translations
│   └── ru/             # Russian translation
│       ├── README.md   # Russian README
│       ├── book/       # Russian textbook
│       └── labs/       # Russian labs
├── labs/               # Laboratory assignments (English)
│   ├── lab00-capability-check/
│   ├── lab01-basics/
│   └── ...             # Other labs
└── README.md           # This file
```

## What's Next?

After completing the course, you'll be able to:
- ✅ Create autonomous AI agents in Go
- ✅ Manage agent context and memory
- ✅ Implement Function Calling and tools
- ✅ Create safe agents with Human-in-the-Loop
- ✅ Apply RAG for working with documentation
- ✅ Create Multi-Agent systems
- ✅ Test and optimize agents

**Happy Learning! 🚀**
