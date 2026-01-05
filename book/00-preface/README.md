# 00. Preface

## Why Is This Needed?

In classical programming you write an algorithm: `if A then B`. You know exactly what will happen.  
In AI Engineering you describe a **Goal** and provide **Tools**. The agent builds the algorithm to achieve the goal in real time.

This textbook will teach you to create autonomous AI agents in Go — systems that can independently solve complex tasks, interact with the real world, and learn from the results of their actions.

### Real-World Case Study

**Situation:** You created a chatbot for DevOps. User writes: "We have database issues, investigate"

**Problem:** A regular chatbot can only respond with text. It cannot actually check metrics, read logs, or apply fixes.

**Solution:** An AI agent with tools can independently check metrics → read logs → form hypotheses → apply fixes → verify results.

## What Is an AI Agent?

**Agent** is a system using LLM as a "reasoning engine" (Reasoning Engine) to perceive the environment, make decisions, and perform actions.

### Difference from ChatGPT

| ChatGPT (Chatbot) | AI Agent |
|------------------|----------|
| Passive. Answers question and waits. | Active. Has a loop. |
| One request → one response. | Can perform 10 actions in a row to solve one task. |
| No access to real world. | Has tools for interacting with systems. |

**Example:**
- **ChatGPT:** "How to restart server?" → Answer: "Use command `systemctl restart nginx`"
- **Agent:** "Restart server" → Agent itself calls `systemctl restart nginx`, checks status, reports result.

## Agent Equation

```
Agent = Brain (LLM) + Tools (Hands) + Memory (Context) + Planning (Process)
```

### Components

- **🧠 Brain (LLM):** Agent's "brain". Makes decisions based on context.
- **🛠 Tools:** Agent's "hands". Allow interaction with the real world.
- **📝 Memory:** Dialogue history and long-term memory (RAG).
- **📋 Planning:** Ability to break task into steps.

### Runtime (Execution Environment)

**Runtime** is the agent code you write in Go. It connects LLM with tools and manages the agent's work loop.

**What Runtime does:**
- Parses LLM responses (determines if model wants to call a tool)
- Executes tools (calls real Go functions)
- Manages dialogue history (adds results to `messages[]`)
- Manages loop (determines when to stop)

**Important:** Runtime is not a separate system or framework. It's your code that you write in `main.go` or in separate modules.

**Example:**
```go
// This is Runtime — code you write
func runAgent(ctx context.Context, client *openai.Client, userInput string) {
    messages := []openai.ChatCompletionMessage{...}
    
    for i := 0; i < maxIterations; i++ {
        resp, _ := client.CreateChatCompletion(ctx, ...)
        msg := resp.Choices[0].Message
        
        if len(msg.ToolCalls) > 0 {
            // Runtime executes tool
            result := executeTool(msg.ToolCalls[0])
            // Runtime adds result to history
            messages = append(messages, ...)
        }
    }
}
```

**See also:** [Chapter 09: Agent Anatomy](../09-agent-architecture/README.md#runtime-execution-environment)

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

1. **Level 0: Scripting.** Bash/Python scripts. Rigid logic. Any deviation — crash.
2. **Level 1: Copilot.** "Write me nginx config". Human validates and applies.
3. **Level 2: Chain.** "Execute deployment": pull -> build -> restart. Agent follows rails, but can (for example) fix compilation error itself.
4. **Level 3: Autonomous Agent.** "We have database issues, investigate". Agent itself searches logs, checks metrics, builds hypotheses, and (if allowed) applies fixes.

**This course:** We'll go from Level 1 to Level 3, creating our AI agent in Go (using DevOps agent as main example).

## How to Read This Textbook

### Recommended Path

1. **Read sequentially** — each chapter builds on previous ones
2. **Practice in parallel** — after each chapter complete corresponding lab
3. **Use as reference** — return to needed sections when working on projects
4. **Study examples** — each chapter has examples from different domains

### Chapter Structure

Each chapter follows a unified template:
- **Why is this needed?** — motivation and practical value
- **Real-world case** — practical example
- **Theory in simple terms** — intuitive explanation
- **How it works** — step-by-step algorithm with code examples
- **Common errors** — what can go wrong and how to fix it
- **Mini-exercises** — practical assignments for reinforcement
- **Checklist** — criteria for understanding the material
- **For the curious** — formalization and deep details (optional)

## Requirements

- **Go 1.21+** — for completing labs
- **Local LLM** (recommended) or OpenAI API Key
  - Install [LM Studio](https://lmstudio.ai/) or [Ollama](https://ollama.com/)
  - Start local server (usually on port 1234 or 11434)
- **Basic programming knowledge** — course is designed for programmers
- **Understanding of DevOps basics** (desirable but not required)

## Environment Setup

To work with local model (e.g., in LM Studio):

```bash
export OPENAI_BASE_URL="http://localhost:1234/v1"
export OPENAI_API_KEY="any-string" # Local models usually don't care about key
```

## What's Next?

After reading the preface, proceed to:
- **[01. LLM Physics](../01-llm-fundamentals/README.md)** — foundation for understanding everything else

---

**Navigation:** [Table of Contents](../README.md) | [LLM Physics →](../01-llm-fundamentals/README.md)

**Happy learning! 🚀**
