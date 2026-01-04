# 00. Preface

## Why This Course?

In classical programming, you write an algorithm: `if A then B`. You know exactly what will happen.  
In AI Engineering, you describe a **Goal** and provide **Tools**. The agent builds the algorithm to achieve the goal in real-time.

This textbook will teach you to create autonomous AI agents in Go — systems that can independently solve complex problems, interact with the real world, and learn from the results of their actions.

### Real-World Case Study

**Situation:** You've created a chatbot for DevOps. A user writes: "We have database issues, investigate"

**Problem:** A regular chatbot can only respond with text. It cannot actually check metrics, read logs, or apply fixes.

**Solution:** An AI agent with tools can independently check metrics → read logs → form hypotheses → apply fixes → verify results.

## What is an AI Agent?

**An Agent** is a system that uses an LLM as a "reasoning engine" to perceive the environment, make decisions, and execute actions.

### Difference from ChatGPT

| ChatGPT (Chatbot) | AI Agent |
|------------------|----------|
| Passive. Answers a question and waits. | Active. Has a loop. |
| One request → one response. | Can execute 10 actions in a row to solve one task. |
| No access to the real world. | Has tools for interacting with systems. |

**Example:**
- **ChatGPT:** "How do I restart the server?" → Answer: "Use the command `systemctl restart nginx`"
- **Agent:** "Restart the server" → The agent itself calls `systemctl restart nginx`, checks status, reports the result.

## The Agent Equation

```
Agent = Brain (LLM) + Tools (Hands) + Memory (Context) + Planning (Process)
```

### Components

- **🧠 Brain (LLM):** The agent's "brain". Makes decisions based on context.
- **🛠 Tools:** The agent's "hands". Allow interaction with the real world.
- **📝 Memory:** Dialogue history and long-term memory (RAG).
- **📋 Planning:** The ability to break down a task into steps.

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

1. **Level 0: Scripting.** Bash/Python scripts. Rigid logic. Any deviation → crash.
2. **Level 1: Copilot.** "Write me an nginx config". Human validates and applies.
3. **Level 2: Chain.** "Deploy": pull -> build -> restart. The agent follows rails but can (e.g.) fix compilation errors itself.
4. **Level 3: Autonomous Agent.** "We have database issues, investigate". The agent itself searches logs, checks metrics, builds hypotheses, and (if permitted) applies fixes.

**This course:** We'll progress from Level 1 to Level 3, building our AI agent in Go (using a DevOps agent as the main example).

## How to Read This Textbook

### Recommended Path

1. **Read sequentially** — each chapter builds on previous ones
2. **Practice in parallel** — after each chapter, complete the corresponding laboratory assignment
3. **Use as a reference** — return to relevant sections when working on projects
4. **Study examples** — each chapter has examples from different domains

### Chapter Structure

Each chapter follows a unified template:
- **Why is this needed?** — motivation and practical value
- **Real-world case** — practical example
- **Theory in simple terms** — intuitive explanation
- **How it works** — step-by-step algorithm with code examples
- **Common mistakes** — what can go wrong and how to fix it
- **Mini-exercises** — practical assignments for reinforcement
- **Checklist** — criteria for understanding the material
- **For the curious** — formalization and deep details (optional)

## Requirements

- **Go 1.21+** — for completing laboratory assignments
- **Local LLM** (recommended) or OpenAI API Key
  - Install [LM Studio](https://lmstudio.ai/) or [Ollama](https://ollama.com/)
  - Start a local server (usually on port 1234 or 11434)
- **Basic programming knowledge** — the course is designed for programmers
- **Understanding of DevOps basics** (desirable but not required)

## Environment Setup

To work with a local model (e.g., in LM Studio):
```bash
export OPENAI_BASE_URL="http://localhost:1234/v1"
export OPENAI_API_KEY="any-string" # Local models usually don't need a key
```

## What's Next?

After reading the preface, proceed to:
- **[01. LLM Physics](../01-llm-fundamentals/README.md)** — the foundation for understanding everything else

---

**Navigation:** [Table of Contents](../README.md) | [LLM Physics →](../01-llm-fundamentals/README.md)

**Happy Learning! 🚀**
