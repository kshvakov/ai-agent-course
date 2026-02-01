# Appendix: Reference Guides

## Why This Chapter?

This section contains reference information: glossary of terms, checklists, SOP templates, and decision tables. Use it as a reference when working on projects.

## Glossary

### Core Concepts

**Agent** — a system that uses an LLM as a "reasoning engine" to perceive its environment, make decisions, and perform actions. Consists of: LLM (brain) + Tools (hands) + Memory (memory) + Planning (planning).

**See also:** [Chapter 00: Preface](../00-preface/README.md#what-is-an-ai-agent)

**Runtime (Execution Environment)** — the agent code you write in Go. It connects the LLM with tools and manages the agent work loop. It performs LLM response parsing, validation, tool execution, and dialogue history management.

**Important:** Runtime is not a separate system or framework. It's your code in `main.go` or separate modules.

**See also:** [Chapter 09: Agent Anatomy](../09-agent-architecture/README.md#runtime-execution-environment)

**Tool** — a Go function, API call, or command that an agent can execute to interact with the real world. It's described in JSON Schema format and passed to the model in the `tools[]` field.

**Synonyms:** Function (in Function Calling context)

**See also:** [Chapter 03: Tools and Function Calling](../03-tools-and-function-calling/README.md)

**Tool Call / Function Call** — a structured JSON request that the LLM generates to call a tool. It contains the tool name and arguments in JSON format. It's returned in the `tool_calls` field of the model response.

**Note:** "Tool Call" and "Function Call" are the same. "Function Calling" is the mechanism name in API, "Tool Call" is a specific tool invocation.

**See also:** [Chapter 03: Tools and Function Calling](../03-tools-and-function-calling/README.md#step-3-model-response-tool-call)

**Artifact / artifact_id** — a large tool result (file, logs, JSON, HTML, etc.) that the runtime stores outside `messages[]`. The model only receives a short excerpt plus an `artifact_id`. This reduces cost/latency and helps avoid context overflows.

**See also:** [Chapter 20: Cost & Latency Engineering](../20-cost-latency-engineering/README.md)

**AgentState** — a canonical agent run state structure: goal, constraints (including HITL), budgets, plan, known facts, open questions, artifacts, and risk flags. It is a contract between agent loop iterations and between components (for example, an "orchestrator" and an "analyzer").

**StatePatch** — a structured update to `AgentState` (for example: append facts, replace plan, add open questions). It's useful when one component normalizes observations and another decides the next action.

**Risk Level** — classification of side effects used by policy and HITL gates. A typical minimal set is: `read_only`, `write_local`, `external_action`.

**ReAct Loop (Reasoning and Action Loop)** — autonomous agent work pattern: Reason (reasons) → Act (acts) → Observe (observes) → repeats. Agent analyzes situation, performs action, receives result, and decides what to do next.

**Etymology:** ReAct = Reason + Act

**See also:** [Chapter 04: Autonomy and Loops](../04-autonomy-and-loops/README.md#react-loop-autonomy-loop)

### LLM and Context

**Context Window** — maximum number of tokens a model can process in one request. Limits dialogue history size. Examples: GPT-3.5: 4k tokens, GPT-4 Turbo: 128k tokens.

**See also:** [Chapter 01: LLM Physics](../01-llm-fundamentals/README.md#context-window)

**Token** — unit of text processed by the model. One token ≈ 0.75 words (English) or ≈ 1.5 tokens per word (Russian). Everything the agent "knows" about the current task is limited by the number of tokens in the context window.

**See also:** [Chapter 01: LLM Physics](../01-llm-fundamentals/README.md#what-is-a-token)

**System Prompt** — instructions for the model that set agent role, goal, constraints, and work process. Passed in `messages[0].content` with role `"system"`. Consists of: Role (Persona), Goal, Constraints, Format, SOP.

**See also:** [Chapter 02: Prompting](../02-prompt-engineering/README.md#system-prompt-structure)

**Temperature** — entropy parameter of token probability distribution. `Temperature = 0` — deterministic behavior (for agents), `Temperature > 0` — random behavior (for creative tasks).

**Rule:** For all agents, set `Temperature = 0`.

**See also:** [Chapter 01: LLM Physics](../01-llm-fundamentals/README.md#temperature)

### Prompting Techniques

**Chain-of-Thought (CoT)** — prompting technique "think step by step", forcing the model to generate intermediate reasoning before final answer. Critical for agents solving complex multi-step tasks.

**See also:** [Chapter 02: Prompting](../02-prompt-engineering/README.md#chain-of-thought-cot-think-step-by-step)

**Few-Shot Learning / Few-Shot** — prompting technique where examples of desired behavior are added to the prompt. Model adapts to format based on examples in context.

**Antonym:** Zero-Shot (instruction only, no examples)

**See also:** [Chapter 02: Prompting](../02-prompt-engineering/README.md#few-shot-instruction--examples)

**Zero-Shot Learning / Zero-Shot** — prompting technique where only instruction is given to the model without examples. Saves tokens, but requires precise instructions.

**Antonym:** Few-Shot (with examples)

**See also:** [Chapter 02: Prompting](../02-prompt-engineering/README.md#zero-shot-instruction-only)

**In-Context Learning (ICL)** — model's ability to adapt behavior based on examples within the prompt, without changing model weights. Works in Zero-shot and Few-shot modes.

**See also:** [Chapter 02: Prompting](../02-prompt-engineering/README.md#in-context-learning-icl-zero-shot-and-few-shot)

**SOP (Standard Operating Procedure)** — action algorithm encoded in the prompt. Sets sequence of steps to solve a task. CoT helps follow SOP step by step.

**See also:** [Chapter 02: Prompting](../02-prompt-engineering/README.md#sop-and-task-decomposition-agent-process)

### Memory and Context

**Memory (Agent Memory)** — system for storing and retrieving information between conversations. Includes short-term memory (current conversation history) and long-term memory (persistent fact storage).

**See also:** [Chapter 12: Agent Memory Systems](../12-agent-memory/README.md)

**Working Memory** — recent conversation turns that are always included in context. Most relevant for current task. Managed through Context Engineering.

**See also:** [Chapter 13: Context Engineering](../13-context-engineering/README.md#context-layers)

**Long-term Memory** — persistent storage of facts, preferences, and past decisions. Stored in database/files and persists between conversations. Can use vector database (RAG) for semantic search.

**See also:** [Chapter 12: Agent Memory Systems](../12-agent-memory/README.md#long-term-memory)

**Episodic Memory** — memory of specific events: "User asked about disk space on 2026-01-06". Useful for debugging and learning.

**See also:** [Chapter 12: Agent Memory Systems](../12-agent-memory/README.md#episodic-memory)

**Semantic Memory** — general knowledge extracted from episodes: "User prefers JSON responses". More abstract than episodic memory.

**See also:** [Chapter 12: Agent Memory Systems](../12-agent-memory/README.md#semantic-memory)

**Context Engineering** — techniques for efficient context management: context layers (working memory, summaries, facts), summarization of old conversations, selection of relevant facts, adaptive context management.

**See also:** [Chapter 13: Context Engineering](../13-context-engineering/README.md)

**RAG (Retrieval Augmented Generation)** — technique for augmenting agent context with relevant documents from knowledge base via vector search. Documents are split into chunks, converted to vectors (embeddings), similar vectors are searched on query.

**See also:** [Chapter 06: RAG and Knowledge Base](../06-rag/README.md)

### Planning and Architecture

**Planning** — the agent's ability to break down a complex task into a sequence of simple steps and execute them in the correct order. Levels: implicit (ReAct), explicit (Plan-and-Solve), hierarchical.

**See also:** [Chapter 09: Agent Anatomy](../09-agent-architecture/README.md#planning)

**State Management** — managing task execution state: progress, what's done, what's pending, resumption capability. Includes tool idempotency, retries with exponential backoff, deadlines, persist state.

**See also:** [Chapter 11: State Management](../11-state-management/README.md)

**Reflexion** — agent self-correction technique through error analysis. Cycle: Act → Observe → Fail → REFLECT → Plan Again. Agent analyzes why action didn't work and plans again.

**See also:** [Chapter 09: Agent Anatomy](../09-agent-architecture/README.md#reflexion-self-correction)

### Security and Reliability

**Grounding** — anchoring agent to real data through Tools/RAG to avoid hallucinations. Agent must use tools to get facts, not invent them.

**See also:** [Chapter 01: LLM Physics](../01-llm-fundamentals/README.md#hallucinations)

**Human-in-the-Loop (HITL)** — mechanism for human confirmation of critical actions before execution. Includes Confirmation (confirmation), Clarification (clarification), Risk Scoring (risk assessment).

**See also:** [Chapter 05: Safety and Human-in-the-Loop](../05-safety-and-hitl/README.md)

**Prompt Injection** — attack on agent through input manipulation. Attacker tries to "trick" the prompt to make agent perform unwanted action.

**See also:** [Chapter 05: Safety and Human-in-the-Loop](../05-safety-and-hitl/README.md#prompt-injection)

**Eval (Evaluation)** — test to check agent work quality. Can check answer correctness, tool selection, SOP following, safety. In production systems used in CI/CD.

**See also:** [Chapter 08: Evals and Reliability](../08-evals-and-reliability/README.md)

### Multi-Agent Systems

**Multi-Agent System (MAS)** — system of multiple agents working together. Can use Supervisor/Worker patterns, context isolation, task routing between specialized agents.

**See also:** [Chapter 07: Multi-Agent Systems](../07-multi-agent/README.md)

## Checklists

### Checklist: Model Setup for Agent

- [ ] Model supports Function Calling (checked via Lab 00)
- [ ] `Temperature = 0` set
- [ ] Context window large enough (minimum 4k tokens)
- [ ] System Prompt prohibits hallucinations
- [ ] Dialogue history managed (truncated on overflow)

### Checklist: Creating System Prompt

- [ ] Role (Persona) clearly defined
- [ ] Goal (Goal) specific and measurable
- [ ] Constraints (Constraints) explicitly stated
- [ ] Response format (Format) described
- [ ] SOP (if applicable) detailed
- [ ] CoT included for complex tasks
- [ ] Few-Shot examples added (if needed)

## Capability Benchmark (Characterization)

Before building agents, you must **scientifically confirm** that the selected model has necessary capabilities. In engineering, this is called **Characterization**.

### Why Is This Needed?

We don't trust labels ("Super-Pro-Max Model"). We trust tests.

**Problem without checking:** You downloaded "Llama-3-8B-Instruct" model and started building an agent. After an hour of work, discovered that the model doesn't call tools, only writes text. You wasted time debugging code, though the problem was in the model.

**Solution:** Run capability benchmark **before** starting work. This saves hours.

### What Do We Check?

#### 1. Basic Sanity
- Model responds to requests
- No critical API errors
- Basic response coherence

#### 2. Instruction Following
- Model can strictly adhere to constraints
- Important for agents: they must return strictly defined formats
- **Test:** "Write a poem, but don't use letter 'a'"
- **Why:** Agent must return strictly defined formats, not "thoughts"

#### 3. JSON Generation
- Model can generate valid syntax
- All tool interaction is built on JSON
- If model forgets to close bracket `}`, agent crashes
- **Test:** "Return JSON with fields name and age"

#### 4. Function Calling
- Specific model skill to recognize function definitions and generate special call token
- Without this, tools are impossible (see [Chapter 03: Tools](../03-tools-and-function-calling/README.md))
- **Why:** This is the foundation for Lab 02 and all subsequent labs

### Why Don't All Models Know Tools?

LLM (Large Language Model) is a probabilistic text generator. It doesn't "know" about functions out of the box.

**Function Calling** mechanism is a result of special training (Fine-Tuning). Model developers add thousands of examples to training set:

```
User: "Check weather"
Assistant: <special_token>call_tool{"name": "weather"}<end_token>
```

If you downloaded "bare" Llama 3 (Base model), it hasn't seen these examples. It will just continue dialogue with text.

**How to check:** Run Lab 00 before starting work with tools.

### Why Is `Temperature = 0` Critical for Agents?

Temperature regulates "randomness" of next token selection:
- **High Temp (0.8+):** Model chooses less probable words. Good for poems, creative tasks.
- **Low Temp (0):** Model always chooses most probable word (ArgMax). Maximum determinism.

For agents that must output strict JSON or function calls, maximum determinism is needed. Any "creative" error in JSON breaks the parser.

**Rule:** For all agents, set `Temperature = 0`.

### How to Interpret Results?

#### ✅ All Tests Passed
Model is ready for the course. Can continue work.

#### ⚠️ 3 out of 4 Tests Passed
Can continue, but with caution. Problems possible in edge cases.

#### ❌ Function Calling Failed
**Critical:** Model is not suitable for Lab 02-08. Need different model.

**What to do:**
1. Download model with tools support:
   - `Hermes-2-Pro-Llama-3-8B`
   - `Mistral-7B-Instruct-v0.2`
   - `Llama-3-8B-Instruct` (some versions)
   - `Gorilla OpenFunctions`
2. Restart tests

#### ❌ JSON Generation Failed
Model generates broken JSON (missing brackets, quotes).

**What to do:**
1. Try different model
2. Or use `Temperature = 0` (but this doesn't always help)

### Connection with Evals

Capability Benchmark is a primitive **Eval** (Evaluation). In production systems (LangSmith, PromptFoo), there are hundreds of such tests.

**Topic development:** See [Chapter 08: Evals and Reliability](../08-evals-and-reliability/README.md) to understand how to build comprehensive evals for checking agent work quality.

### Practice

To perform capability benchmark, see [Lab 00: Model Capability Benchmark](../../labs/lab00-capability-check/README.md).

## SOP Templates

### SOP for Incident (DevOps)

```
SOP for service failure:
1. Check Status: Check HTTP response code
2. Check Logs: If 500/502 — read last 20 log lines
3. Analyze: Find keywords:
   - "Syntax error" → Rollback
   - "Connection refused" → Check Database
   - "Out of memory" → Restart
4. Action: Apply fix according to analysis
5. Verify: Check HTTP status again
```

### SOP for Ticket Processing (Support)

```
SOP for ticket processing:
1. Read: Read ticket completely
2. Context: Gather context (version, OS, browser)
3. Search: Search knowledge base for similar cases
4. Decide:
   - If solution found → Draft reply
   - If complex problem → Escalate
5. Respond: Send response to user
```

## Decision Tables

### Decision Table for Incident

| Symptom | Hypothesis | Check | Action | Verification |
|---------|------------|-------|--------|--------------|
| HTTP 502 | Service down | `check_http()` → 502 | - | - |
| HTTP 502 | Error in logs | `read_logs()` → "Syntax error" | `rollback_deploy()` | `check_http()` → 200 |
| HTTP 502 | Error in logs | `read_logs()` → "Connection refused" | `restart_service()` | `check_http()` → 200 |

## FAQ: Why Is This Not Magic?

### Q: Agent decides what to do itself. Is this magic?

**A:** No. Agent works by a simple algorithm:
1. LLM receives tool descriptions in `tools[]`
2. LLM generates JSON with tool name and arguments
3. Your code (Runtime) parses JSON and executes real function
4. Result is added to history
5. LLM receives result in context and generates next step

There's no magic here — it's just a loop where the model receives results of previous actions.

**See also:** [Chapter 04: Autonomy](../04-autonomy-and-loops/README.md#magic-vs-reality-how-the-loop-works)

### Q: How does the model "know" which tool to call?

**A:** Model doesn't "know". It selects tool based on:
1. **Tool description** (`Description` in JSON Schema)
2. **User query** (semantic match)
3. **Context of previous results** (if any)

The more accurate the `Description`, the better the selection.

**See also:** [Chapter 03: Tools](../03-tools-and-function-calling/README.md#how-does-the-model-choose-between-multiple-tools)

### Q: Does the model "remember" past conversations?

**A:** No. Model is stateless. It only processes the past in `messages[]` that you pass in each request. If you don't pass history, the model remembers nothing.

**See also:** [Chapter 01: LLM Physics](../01-llm-fundamentals/README.md#model-is-stateless)

### Q: Why does the agent sometimes do different actions on the same request?

**A:** This happens due to probabilistic nature of LLM. If `Temperature > 0`, the model selects random token from distribution. For agents, always use `Temperature = 0` for deterministic behavior.

**See also:** [Chapter 01: LLM Physics](../01-llm-fundamentals/README.md#temperature)

### Q: Model invents facts. How to fix this?

**A:** Use **Grounding**:
1. Prohibit inventing facts in System Prompt
2. Give agent access to real data through Tools
3. Use RAG for documentation access

**See also:** [Chapter 01: LLM Physics](../01-llm-fundamentals/README.md#hallucinations)

### Q: Agent "forgets" beginning of conversation. What to do?

**A:** This happens when dialogue history exceeds context window size. Solutions:
1. **Summarization:** Compress old messages through LLM
2. **Fact selection:** Extract important facts and store separately
3. **Context layers:** Working memory + summary + facts

**See also:** [Chapter 13: Context Engineering](../13-context-engineering/README.md)

### Q: Model doesn't call tools. Why?

**A:** Possible causes:
1. Model doesn't support Function Calling (check via Lab 00)
2. Poor tool description (`Description` unclear)
3. `Temperature > 0` (too random)

**Solution:** Use model with tools support, improve `Description`, set `Temperature = 0`.

**See also:** [Chapter 03: Tools](../03-tools-and-function-calling/README.md#error-1-model-does-not-generate-tool_call)

## Mini-Exercises

### Exercise 1: Create Your SOP

Create an SOP for your domain following the "SOP Templates" section:

```text
SOP for [your task]:
1. [Step 1]
2. [Step 2]
3. [Step 3]
```

**Expected result:**
- SOP clearly describes action process
- Steps are sequential and logical
- Checks and verification included

### Exercise 2: Create Decision Table

Create a decision table for your task following the "Decision Tables" section:

| Symptom | Hypothesis | Check | Action | Verification |
|---------|------------|-------|--------|--------------|
| ...     | ...        | ...   | ...    | ...          |

**Expected result:**
- Table covers main scenarios
- For each symptom there is hypothesis, check, action, and verification

## Connection with Other Chapters

- **Prompting:** How to use SOP in prompts, see [Chapter 02: Prompting](../02-prompt-engineering/README.md)
- **Case Studies:** Examples of SOP usage in real agents, see [Chapter 15: Case Studies](../15-case-studies/README.md)

---

**Navigation:** [← Production Readiness Index](../25-production-readiness-index/README.md) | [Table of Contents](../README.md)
