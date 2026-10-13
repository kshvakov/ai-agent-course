# Appendix: Reference Guides

## Why does this matter?

This section contains reference material: a glossary of terms, checklists, SOP templates, and decision tables. Use it as a reference while working on projects.

## Glossary

### Core concepts

**Agent** — a system that uses an LLM as its "reasoning engine" to perceive the environment, make decisions, and take actions. Consists of: LLM (brain) + Tools (hands) + Memory + Planning.

**See also:** [Chapter 00: Preface](../00-preface/README.md#what-is-an-ai-agent)

**Runtime** — the agent code you write in Go. It connects the LLM to tools and drives the agent loop. It parses LLM responses, validates them, executes tools, and manages the conversation history.

**Important:** Runtime is not a separate system or framework. It is your code in `main.go` or in individual modules.

**See also:** [Chapter 09: Agent Anatomy](../09-agent-architecture/README.md#runtime)

**Tool** — a Go function, API call, or command the agent can execute to interact with the real world. Described in JSON Schema format and passed to the model in the `tools[]` field.

**Synonyms:** Function (in the Function Calling context)

**See also:** [Chapter 03: Tools and Function Calling](../03-tools-and-function-calling/README.md)

**Tool Call / Function Call** — a structured JSON request the LLM generates to invoke a tool. Contains the tool name and JSON arguments. Returned in the `tool_calls` field of the model response.

**Note:** "Tool Call" and "Function Call" are the same thing. "Function Calling" is the name of the mechanism in the API; "Tool Call" is a specific tool invocation.

**See also:** [Chapter 03: Tools and Function Calling](../03-tools-and-function-calling/README.md#step-3-the-models-response-tool-call)

**MCP (Model Context Protocol)** — a protocol for connecting tools and data to LLM agents (Anthropic). Provides a standard interface for Resources, Tools, and Prompts.

**Artifact / artifact_id** — a large tool result (file, log, JSON, HTML, etc.) that the runtime stores outside `messages[]`, returning only a short excerpt + `artifact_id` into the model context. This cuts cost/latency and reduces the risk of context overflow.

**See also:** [Chapter 20: Cost & Latency Engineering](../20-cost-latency-engineering/README.md)

**AgentState** — the state of a run, living OUTSIDE the LLM context: goal, constraints (including HITL), budgets, plan, artifacts (files/DB + paths/IDs). The agent context is a linear message history; "known facts" and similar structures are not injected into the context (see [Chapter 11](../11-state-management/README.md)).

**Risk Level** — a classification of a tool's side effects, used by policy and HITL gates. The course's single scale: `read_only` (auto) / locally writing (Confirm per policy) / destructive or external (dry-run + Confirm + `confirmed_by`). See [Chapter 05](../05-safety-and-hitl/README.md).

**ReAct Loop** — the operating pattern of an autonomous agent: Reason → Act → Observe → repeat. The agent analyzes the situation, takes an action, sees the result, and decides what to do next. In its modern form, "reasoning" is just the model's content alongside tool_calls — no separate text format is needed.

**Etymology:** ReAct = Reason + Act

**See also:** [Chapter 04: Autonomy and Loops](../04-autonomy-and-loops/README.md#the-react-loop-the-loop-of-autonomy)

**Skills** — reusable agent knowledge modules in SKILL.md format (agentskills.io): a directory with instructions and scripts. Only names/descriptions go into the system prompt (~1 line per skill — progressive disclosure); the model reads a skill's text itself with the `read` tool when it needs it. More in [Chapter 09](../09-agent-architecture/README.md#skills).

### LLM and context

**Context Window** — the maximum number of tokens a model can process in a single request. It limits the size of the conversation history. Current models (GLM-5.3, Fable 5.1, Opus 5.5) range from ~128k to ~1M tokens; check the provider's docs for the exact value — it changes more often than books get reprinted.

**See also:** [Chapter 01: LLM Physics](../01-llm-fundamentals/README.md#the-context-window)

**Token** — a unit of text the model processes. One token ≈ 0.75 words (English) or ≈ 1.5 tokens per word (Russian). Everything the agent "knows" about the current task is limited by the token count of the context window.

**See also:** [Chapter 01: LLM Physics](../01-llm-fundamentals/README.md#what-is-a-token)

**System Prompt** — instructions for the model that define the agent's role, goal, constraints, and working process. Passed as `messages[0].content` with the `"system"` role. Consists of: Role (Persona), Goal, Constraints, Format, SOP.

**See also:** [Chapter 02: Prompting](../02-prompt-engineering/README.md#structure-of-the-system-prompt)

**Temperature** — a parameter controlling the entropy of the token probability distribution. A quality/diversity knob: low values (0.0–0.3) reduce variance, high values increase it.

**Rule:** for agent loops and structured calls, `0.0–0.3` is typical. But remember: temperature is not a "determinism switch". For many providers, `T=0` does not technically guarantee greedy determinism (batching, MoE); reproducibility of the agent process comes from structure (JSON Schema, validation), guards in code, and evals.

**See also:** [Chapter 01: LLM Physics](../01-llm-fundamentals/README.md#temperature)

**Prompt Cache** — an optimization on the LLM provider's side: an identical `messages[]` prefix (system prompt + first messages) is cached and billed at a 50–90% discount. To hit the cache, the prefix must be **byte-for-byte stable** across requests within a single agent run. Any mutation of the system prompt (inserting time, dynamic facts) or replacing early messages evicts the cache — iterations become several times more expensive and slower. That's why dynamic state (current time, facts) goes into the **last** user message or into tool results, not into `messages[0]`.

**Rule:** `messages[0]` (system prompt) is append-only and is not mutated within a single run.

**See also:** [Chapter 12: Agent Memory Systems](../12-agent-memory/README.md), [Chapter 13: Context Engineering](../13-context-engineering/README.md)

### Prompting techniques

**Chain-of-Thought (CoT)** — the "think step by step" prompting technique, making the model generate intermediate reasoning before the final answer. Critical for agents solving complex multi-step problems.

**See also:** [Chapter 02: Prompting](../02-prompt-engineering/README.md#chain-of-thought-cot-think-step-by-step)

**Few-shot learning / Few-shot** — a prompting technique where examples of the desired behavior are added to the prompt. The model adapts to the format based on the examples in context.

**Antonym:** Zero-shot (instruction only, no examples)

**See also:** [Chapter 02: Prompting](../02-prompt-engineering/README.md#few-shot-instruction-examples)

**Zero-shot learning / Zero-shot** — a prompting technique where the model gets only an instruction, no examples. Saves tokens but requires precise instructions.

**Antonym:** Few-shot (with examples)

**See also:** [Chapter 02: Prompting](../02-prompt-engineering/README.md#zero-shot-instruction-only)

**In-Context Learning (ICL)** — the model's ability to adapt its behavior based on examples inside the prompt, without changing model weights. Works in both Zero-shot and Few-shot modes.

**See also:** [Chapter 02: Prompting](../02-prompt-engineering/README.md#in-context-learning-icl-zero-shot-and-few-shot)

**SOP (Standard Operating Procedure)** — an algorithm of actions encoded in the prompt. It sets the sequence of steps for solving a task. CoT helps the model follow the SOP step by step.

**See also:** [Chapter 02: Prompting](../02-prompt-engineering/README.md#sop-and-task-decomposition-the-agents-process)

### Memory and context

**Memory** — the set of mechanisms that let an agent retain and use information. This course splits it **by horizon**, not by "levels":

- **Within a run** — the linear `messages[]` in the LLM context +, if needed, a single `condense`. Everything is in RAM and lives until the end of the run.
- **Between runs in one session** — session-level state (plan, files read, recent actions) passed between REPL iterations.
- **Between sessions** — long-term memory the agent manages itself via tools (`search_memory` / `read_note` — reading; fact writing goes to the store); stored in DB/files.

There is no separate "short-term memory" structure — `messages[]` itself plays that role.

**See also:** [Chapter 12: Agent Memory Systems](../12-agent-memory/README.md)

**Working Memory** — in this course the term refers to **two different things**; don't confuse them:

1. **The LLM context within a single run** — just a **linear `messages[]`** (system + dialog + tool results) up to the next `condense`. No "layers" (Working/Summary/Facts) on top — that's an outdated pattern, see ch. 13 and [Lab 11](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab11-memory-context).
2. **Session-level state** — the plan, files read, and recent actions that survive across REPL cycles. This is not LLM context but a structure serialized to disk and loaded on restart. See ch. 11.

**See also:** [Chapter 11: State Management](../11-state-management/README.md), [Chapter 12: Agent Memory Systems](../12-agent-memory/README.md), [Chapter 13: Context Engineering](../13-context-engineering/README.md)

**Long-term Memory** — persistent storage across sessions: facts, preferences, past decisions. **The principle from this course:** the agent manages long-term memory **itself** via tools (`search_memory` / `read_note` — read on demand; writing — facts into the store), not via auto-extraction of facts from every message and not by mutating the system prompt. Stored in DB/files; for a large corpus — vector search (RAG).

**See also:** [Chapter 12: Agent Memory Systems](../12-agent-memory/README.md#long-term-memory-between-sessions), [Lab 11](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab11-memory-context)

**Episodic Memory** — memory of specific events: "the user asked about disk space on 2026-01-06". For most agents this is just dated entries in the long-term memory store — no separate infrastructure needed. Useful for debugging.

**See also:** [Chapter 12: Agent Memory Systems](../12-agent-memory/README.md)

**Semantic Memory** — general knowledge about the environment/user ("prefers JSON responses"). In practice, the same long-term memory entries, just more generalized. There is no need to build a separate "semantic store".

**See also:** [Chapter 12: Agent Memory Systems](../12-agent-memory/README.md)

**Context Engineering** — techniques for effective context management: token counting via `usage.PromptTokens`, a single condense threshold (~0.85 of the context window), one `condense` per run, and protecting `tool_call ↔ tool_result` pairs during truncation.

**See also:** [Chapter 13: Context Engineering](../13-context-engineering/README.md)

**Condense** — an on-demand operation that summarizes the older part of `messages[]` via a separate LLM call to free up space in the context window. Triggered by a single threshold (`usage.PromptTokens > contextWindow * 0.85`) or reactively on `ContextOverflowError`. Principles of a correct `condense`: (1) **the cacheable prefix (system + tools block) stays unchanged** — only the head of the history is summarized, otherwise the prompt cache is lost; (2) `safeTail` preserves `tool_call ↔ tool_result` pairs — you must not leave a tool_call without its tool_result or vice versa; (3) the limit is one `condense` per run, otherwise it loops forever. The course does not recommend compact copies or block recall catalogs (an anti-pattern, see [Chapter 12](../12-agent-memory/README.md)).

**See also:** [Chapter 13: Context Engineering](../13-context-engineering/README.md), [Lab 09](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab09-context-optimization), [Lab 11](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab11-memory-context)

**RAG (Retrieval Augmented Generation)** — a technique for augmenting the agent's context with relevant documents from a knowledge base via vector search. Documents are split into chunks, converted to vectors (embeddings), and similar vectors are looked up at query time.

**See also:** [Chapter 06: RAG and the Knowledge Base](../06-rag/README.md)

**Agentic RAG** — RAG embedded in the Agent Loop. The agent decides itself when and where to search, evaluates result quality, and can search again.

**Self-RAG** — RAG with quality self-assessment. The model evaluates the relevance of retrieved documents and decides: answer, clarify the query, or search more.

**Hybrid Search** — a combination of keyword search (BM25) and vector search. Results are merged via Reciprocal Rank Fusion.

### Planning and architecture

**Planning** — the agent's ability to break a complex task into a sequence of simple steps and execute them in the right order. Levels: implicit (ReAct), explicit (Plan-and-Solve), hierarchical.

**See also:** [Chapter 09: Agent Anatomy](../09-agent-architecture/README.md#planning)

**State Management** — managing the state of task execution: progress, what's done, what's pending, the ability to resume. Includes tool idempotency, retries with exponential backoff, deadlines, and state persistence.

**See also:** [Chapter 11: State Management](../11-state-management/README.md)

**Checkpoint** — a snapshot of the agent's state at a given moment. Allows resuming after a failure without losing progress.

**Reflexion** — a technique for agent self-correction through error analysis. The loop: Act → Observe → Fail → REFLECT → Plan Again. The agent analyzes why an action failed and plans again.

**See also:** [Chapter 09: Agent Anatomy](../09-agent-architecture/README.md#reflexion-self-correction)

### Safety and reliability

**Junior-Employee Model** — the basic frame from the Preface: the agent is a new intern — well-read and fast, but without your context and without a sense of consequences. Most security/governance rules are ordinary HR practices (role, access, senior approval, audit, probation period) translated into code. **Four asymmetries** make an agent more dangerous than a human in the same role: (1) 1000× speed, (2) parallelism across N sessions, (3) no psychological brake on destructive actions, (4) prompt injection as social engineering via data. To counter these asymmetries: rate limits, max parallel tools, idempotency, sanitization of external data.

**See also:** [Preface → Mental Model](../00-preface/README.md#mental-model-an-agent-is-a-new-employee), [Chapter 17: Security and Governance](../17-security-and-governance/README.md)

**Blast Radius** — the scale of consequences of an agent's erroneous action. Minimizing blast radius is the core principle of tool design: instead of `delete_files(pattern string)` prefer `move_to_trash(paths []string)` with an explicit list; instead of `run_sql(query string)` prefer `select_users(filter UserFilter)`. Protective layers that reduce blast radius: dry-run, idempotency, path/table allowlists, pagination/limit caps, mandatory HITL for destructive operations, sandboxing.

**See also:** [Chapter 03: Tools and Function Calling](../03-tools-and-function-calling/README.md), [Chapter 17: Security and Governance](../17-security-and-governance/README.md)

**Recovery State Machine** — a structured approach to handling LLM call errors: instead of a single `if err != nil { return err }`, the agent classifies the error (`RateLimit`, `ContextOverflow`, `InvalidToolCall`, `ProviderUnavailable`, `MalformedJSON`) and applies a specific response (backoff + retry, on-demand `condense`, a repeated call with a format hint, fallback to another model, a recovery parser to extract tool calls from raw text). Without a recovery state machine, the agent "falls at the first bump" in production.

**See also:** [Chapter 11: State Management](../11-state-management/README.md), [Chapter 13: Context Engineering](../13-context-engineering/README.md)

**Grounding** — anchoring the agent to real data via Tools/RAG to avoid hallucinations. The agent must use tools to obtain facts, not invent them.

**See also:** [Chapter 01: LLM Physics](../01-llm-fundamentals/README.md#2-hallucinations)

**Human-in-the-Loop (HITL)** — a mechanism for human confirmation of critical actions before execution. Includes Confirmation, Clarification, and tool categories (risk as an explicit category, not a number).

**See also:** [Chapter 05: Safety and Human-in-the-Loop](../05-safety-and-hitl/README.md)

**Prompt Injection** — an attack on the agent by manipulating input data. The attacker tries to "trick" the prompt so the agent performs an unwanted action.

**See also:** [Chapter 05: Safety and Human-in-the-Loop](../05-safety-and-hitl/README.md#mistake-3-prompt-injection)

**Defense in Depth** — a multi-layered security strategy. Each layer (input validation, runtime checks, output filtering, monitoring) protects against different attack types.

**Red Teaming** — systematic testing of the agent for vulnerabilities. An "attacking" team tries to make the agent break the rules.

**Eval (Evaluation)** — a test that checks the quality of the agent's work. Can verify answer correctness, tool selection, SOP adherence, and safety. Used in CI/CD in production systems.

**See also:** [Chapter 08: Evals and Reliability](../08-evals-and-reliability/README.md)

**DeepEval** — a framework for evaluating LLM application quality. Provides metrics: faithfulness, answer relevancy, context precision.

**Multi-turn Evaluation** — evaluating the agent in multi-step dialogs. Checks context coherence and correctness of actions at each step.

**RAGAS** — a metrics framework for RAG systems. Measures context precision, context recall, faithfulness, and answer relevance.

### Multi-agent systems

**Multi-Agent System (MAS)** — a system of several agents working together. Can use Supervisor/Worker patterns, context isolation, and routing of tasks between specialized agents.

**See also:** [Chapter 07: Multi-Agent Systems](../07-multi-agent/README.md)

**A2A (Agent-to-Agent)** — an inter-agent communication protocol (Google). Standardizes task exchange between agents via Agent Card, Task lifecycle, and Message/Artifact.

**Handoff** — transferring control from one agent to another along with part of the context. Used for escalation or domain change.

**Router Agent** — a routing agent. Classifies the request and directs it to the appropriate specialist.

**Subagent** — a subagent created dynamically for a subtask. Unlike a Worker, it is created "on the fly" for a specific task.

## Checklists

### Checklist: Configuring the model for an agent

- [ ] The model supports Function Calling (verified via [Lab 00](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab00-capability-check))
- [ ] `Temperature` in the 0.0–0.3 range
- [ ] Context window ≥ ~128k tokens (current models range from ~128k to ~1M)
- [ ] Conversation history is managed: condense at ~0.85 of the window by `usage.PromptTokens`, at most 1 per run (see [Chapter 13](../13-context-engineering/README.md))
- [ ] Call reliability is ensured by structure: JSON Schema, validation, repair of broken JSON, guards in code (not hope in low temperature)

### Checklist: Writing a system prompt

- [ ] Role (Persona) is clearly defined
- [ ] Goal is specific and measurable
- [ ] Constraints are stated explicitly
- [ ] Response format is described
- [ ] SOP (if applicable) is written out in detail
- [ ] CoT is enabled for complex tasks
- [ ] Few-shot examples added (if needed)

## Capability Benchmark (Characterization)

Before building agents, you must **scientifically verify** that the chosen model has the required capabilities. In engineering this is called **Characterization**.

### Why does this matter?

We don't trust labels ("Super-Pro-Max Model"). We trust tests.

**The problem without verification:** You picked a local instruct model that claims tool support and started building an agent. An hour into work you discover the model doesn't call tools — it only writes text. You spent time debugging your code when the problem was in the model.

**The fix:** Run the capability benchmark **before** starting work. It saves hours.

### What do we check?

#### 1. Basic sanity
- The model responds to requests
- No critical API errors
- Basic response coherence

#### 2. Instruction following
- The model can strictly adhere to constraints
- Important for agents: they must return strictly defined formats
- **Test:** "Write a poem but don't use the letter 'a'"
- **Why:** The agent must return strictly defined formats, not "musings"

#### 3. JSON generation
- The model can generate valid syntax
- All tool interaction is built on JSON
- If the model forgets to close a bracket `}`, the agent crashes
- **Test:** "Return JSON with name and age fields"

#### 4. Function calling
- A specific model skill: recognizing function definitions and emitting a special call token
- Without it, tools are impossible (see [Chapter 03: Tools](../03-tools-and-function-calling/README.md))
- **Why:** This is the foundation for [Lab 02](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab02-tools) and all subsequent labs

### Why can't all models do tools?

An LLM (Large Language Model) is a probabilistic text generator. It doesn't "know" about functions out of the box.

The **Function Calling** mechanism is the result of special training (Fine-Tuning). The model developers add thousands of examples like this to the training set:

```
User: "Check weather"
Assistant: <special_token>call_tool{"name": "weather"}<end_token>
```

If you take a model without instruction fine-tuning (a base model), it has never seen these examples. It will just continue the dialog with text.

**How to check:** Run [Lab 00](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab00-capability-check) before starting work with tools.

### Why does temperature matter for agents?

Temperature controls the "randomness" of next-token selection:
- **High Temp (0.8+):** The model picks less probable words. Good for poems and creative tasks.
- **Low Temp (0.0–0.3):** The model picks the most probable word more often — less variance in tool selection and output format.

Agents that must emit strict JSON or a function call need low variance — a "creative" error in JSON breaks the parser. But process determinism comes not from temperature but from structure: call schemas, validation, repair of broken JSON, and guards in code.

**Rule:** for agent loops — `0.0–0.3`; reliability is achieved with schema/repair/guards, not by hoping for `T=0`.

### How to interpret the results?

#### All tests pass
The model is ready for the course. You can continue.

#### Warning: 3 of 4 tests pass
You can continue, but with caution. Edge-case problems are possible.

#### Function calling fails
**Critical:** The model is unsuitable for [Lab 02](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab02-tools)-08. You need a different model.

**What to do:**
1. Take a model with tool support: cloud (GLM-5.3, GLM-5.3 Flash, Fable 5.1, Opus 5.5) or local via Ollama/vLLM/LM Studio — any that claims function calling and offers an OpenAI-compatible endpoint.
2. Re-run the tests

#### JSON generation fails
The model generates broken JSON (missing brackets, quotes).

**What to do:**
1. Try a different model
2. Or lower `Temperature` (0.0–0.3) and add repair of broken JSON — temperature doesn't guarantee valid JSON, repair does

### Relation to evals

The Capability Benchmark is a primitive **Eval** (Evaluation). Production systems (LangSmith, PromptFoo) run hundreds of such tests.

**Going deeper:** See [Chapter 08: Evals and Reliability](../08-evals-and-reliability/README.md) to learn how to build comprehensive evals for checking agent quality.

### Practice

To run the capability benchmark, see [Lab 00: Model Capability Benchmark](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab00-capability-check).

## SOP Templates

### SOP for an incident (DevOps)

```
Service outage SOP:
1. Check Status: Check the HTTP response code
2. Check Logs: If 500/502 — read the last 20 log lines
3. Analyze: Look for keywords:
   - "Syntax error" → Rollback
   - "Connection refused" → Check Database
   - "Out of memory" → Restart
4. Action: Apply the fix according to the analysis
5. Verify: Check the HTTP status again
```

### SOP for ticket handling (support)

```
Ticket-handling SOP:
1. Read: Read the ticket in full
2. Context: Gather context (version, OS, browser)
3. Search: Look for similar cases in the knowledge base
4. Decide:
   - If a solution is found → Draft reply
   - If it's a hard problem → Escalate
5. Respond: Send the reply to the user
```

## Decision Tables

### Decision table for an incident

| Symptom | Hypothesis | Check | Action | Verification |
|---------|----------|----------|----------|-------------|
| HTTP 502 | Service is down | `check_http()` → 502 | - | - |
| HTTP 502 | Error in the logs | `read_logs()` → "Syntax error" | `rollback_deploy()` | `check_http()` → 200 |
| HTTP 502 | Error in the logs | `read_logs()` → "Connection refused" | `restart_service()` | `check_http()` → 200 |

## FAQ: What Actually Happens?

### Q: The agent decides what to do on its own. Is that magic?

**A:** No. The agent follows a simple algorithm:
1. The LLM sees tool descriptions in `tools[]`
2. The LLM generates JSON with the tool name and arguments
3. Your code (the Runtime) parses the JSON and calls the real function
4. The result is appended to the history
5. The LLM sees the result in context and generates the next step

No magic here — it's just a loop where the model sees the results of its previous actions.

**See also:** [Chapter 04: Autonomy](../04-autonomy-and-loops/README.md#magic-vs-reality-how-the-loop-works)

### Q: How does the model "know" which tool to call?

**A:** The model doesn't "know". It picks a tool based on:
1. **The tool description** (`Description` in the JSON Schema)
2. **The user request** (semantic match)
3. **The context of previous results** (if any)

The more precise the `Description`, the better the choice.

**See also:** [Chapter 03: Tools](../03-tools-and-function-calling/README.md)

### Q: Does the model "remember" past conversations?

**A:** No. The model is stateless. It sees the past only in the `messages[]` you pass with each request. If you don't pass the history, the model remembers nothing.

**See also:** [Chapter 01: LLM Fundamentals](../01-llm-fundamentals/README.md#the-probabilistic-nature)

### Q: Why does the agent sometimes take different actions on the same request?

**A:** That's the probabilistic nature of LLMs. At high temperature the model picks less likely tokens. For agent loops use a low temperature (0.0–0.3) — but remember that process reproducibility comes from structure (schema, validation, repair), guards in code, and evals, not from temperature itself.

**See also:** [Chapter 01: LLM Fundamentals](../01-llm-fundamentals/README.md#temperature)

### Q: The model makes up facts. How do I fix that?

**A:** Use **Grounding**:
1. Forbid inventing facts in the System Prompt
2. Give the agent access to real data via Tools
3. Use RAG for access to documentation

**See also:** [Chapter 01: LLM Fundamentals](../01-llm-fundamentals/README.md#2-hallucinations)

### Q: The agent "forgets" the start of the conversation. What do I do?

**A:** This happens when the dialogue history exceeds the context window. The right fix is **one, simple**: `condense`.

1. Count tokens via `usage.PromptTokens` from the provider's response (not your own counter).
2. When `usage.PromptTokens > contextWindow * 0.85`, do **one** `condense`: a separate LLM request summarizes the old part of the history (head) with the same cacheable prefix (system + tools), the tail is kept intact — with protection of `tool_call ↔ tool_result` pairs (`safeTail`); the system prompt is unchanged.
3. One `condense` per Run; if that doesn't help, the task should be split into sub-Runs.

What **not** to do: importance scoring of messages, reordering history, adaptive ladders `prioritize → summarize → truncate`, Working/Summary/Facts layers, a dynamic system prompt. These are classic forms of overengineering — expensive, fragile, and they wreck the prompt cache.

Long-term knowledge that must survive a `condense` is **no longer context** — it's long-term memory: the agent **itself** saves facts to a store and reads them on demand via `search_memory` / `read_note` ([ch. 12](../12-agent-memory/README.md)).

**See also:** [Chapter 12: Agent Memory Systems](../12-agent-memory/README.md), [Chapter 13: Context Engineering](../13-context-engineering/README.md), [Lab 09](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab09-context-optimization), [Lab 11](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab11-memory-context)

### Q: The model doesn't call tools. Why?

**A:** Possible reasons:
1. The model doesn't support Function Calling (check via [Lab 00](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab00-capability-check))
2. Poor tool description (unclear `Description`)
3. Too high `Temperature` (well above the 0.0–0.3 range)

**Fix:** Use a model with tools support, improve the `Description`, lower the `Temperature` (0.0–0.3), and add repair for broken JSON.

**See also:** [Chapter 03: Tools](../03-tools-and-function-calling/README.md#mistake-1-the-model-doesnt-generate-a-tool_call)

## Exercises

### Exercise 1: Write your own SOP

Write an SOP for your domain following the template from the "SOP templates" section:

```text
SOP for [your task]:
1. [Step 1]
2. [Step 2]
3. [Step 3]
```

**Expected result:**
- The SOP clearly describes the sequence of actions
- The steps are ordered and logical
- Checks and verification are included

### Exercise 2: Build a decision table

Build a decision table for your task following the example in the "Decision tables" section:

| Symptom | Hypothesis | Check | Action | Verification |
|---------|----------|----------|----------|-------------|
| ...     | ...      | ...      | ...      | ...         |

**Expected result:**
- The table covers the main scenarios
- Each symptom has a hypothesis, check, action, and verification

## Production View: Atlas SDK

> Atlas SDK is a real production SDK on which autonomous agents running in prod are built. The SDK is **closed-source**, so there is no SDK code here and no API promises: this is a map of contracts **as of October 2026** — meant to connect the course principles to how they look in a battle-tested runtime. Signatures will age; invariants won't: all the invariants from the "How the real SDK does it." sidebars across chapters will remain true even after the API changes.

### Module map

| Module | Role |
|---|---|
| `agent` | The `Run`/`Stream` loop: iterations, events, provider failover, parallel tools, auto-condense, loop detection |
| `agent/recovery` | Parsers for tool calls that "leaked" into plain text (XML/bracket formats) — a fallback path, not the primary protocol |
| `llm` | Facade: `Model` (model description) → `Open()` → long-lived `Client` (Chat/Stream); retry of transient errors inside the client; provider drivers are registered via blank import, like `database/sql` |
| `llm/catalog` | Catalog of providers and models: ready-made model descriptions, a separate package so it doesn't land in the binary |
| `memory` | Linear message history: a concrete structure (Append/Messages/Replace/Snapshot/Restore) with built-in sanitization |
| `condense` | LLM-based condense of history: head → summary with the same cacheable prefix, tail intact |
| `tool` | Tool interface, registry, typed constructor, options (read-only/parallel/confirm), StreamingTool, FSHooks host policy, a reference tool set (read/write/edit/exec/fetch/...) |
| `skill` | Loading SKILL.md skills (agentskills.io), PromptSection for the system prompt, progressive disclosure |
| `genobject` | Structured output: one request → a typed Go object (schema derived from the type, repair of broken JSON) |
| `schema`, `jsonrepair` | JSON Schema generation from Go structs; robust parsing of broken JSON |

### Key contracts

```go
// The loop: one function, config is a flat struct, events are a closed set
func (a *Agent) Run(ctx context.Context, in RunInput) (*RunResult, error)
func (a *Agent) Stream(ctx context.Context, in RunInput) iter.Seq2[Event, error]

type Config struct {
    Resolve   llm.Resolver   // where to get an llm.Client
    Memory    *memory.History
    Tools     []tool.Tool
    Prompt    string         // immutable cacheable prefix
    Inject    func(LoopState) []llm.Message // dynamics: injecting messages into the run (interleaving)
    Stop      func(LoopState, llm.Response) bool
    Confirm   ConfirmFunc    // UX confirmation, NOT a security boundary
    Condense  CondenseFunc   // one threshold ~0.85, at most 1 per Run
    Recovery  RecoveryFunc   // parsing "leaked" tool calls
    Fallbacks []llm.Resolver // each fallback used at most once per Run
    Limits    Limits         // iterations, deadline — terminal boundaries
    On        func(Event)    // EvDelta / EvResponse / EvToolStart / EvToolUpdate / EvToolEnd / EvCondenseErr / EvDone
    Streaming bool
}

// The book's teaching canon (ch. 09) uses neutral names for the same events:
// DeltaEvent / ResponseEvent / ToolStartEvent / ToolUpdateEvent / ToolEndEvent /
// CondenseErrorEvent (EvDone remained only in the SDK).

// RunResult is returned even on error — partial data is useful
type RunResult struct {
    ID, Output        string
    Iterations        int
    Usage             llm.Usage // PromptTokens is authoritative; cache metrics are parsed
    CalledTools       []string
    Failovers         int
    FinalModel        string // redacted: keys stripped by default
    FinishReason      string
}

// Tool: typed constructor — the schema is generated from a Go struct
func New[TInput any](name, description string,
    fn func(ctx context.Context, input TInput) (*Result, error), opts ...Option) tool.Tool
// Options: WithReadOnly() / WithParallel() / WithConfirm() / WithFormatCall(fn)
// Result{Content — for UI/audit, FilteredContent — seen only by the model, Images}

// Structured output: one request → a typed object
func Generate[T any](ctx context.Context, r llm.Resolver, prompt string, opts Options) (T, error)
```

### Invariants that survive API changes

1. One condense threshold (~0.85 of the window per `usage.PromptTokens`), at most one condensation per Run.
2. System prompt + tools block — an immutable prefix; dynamics go through tool results and message injection at the end.
3. Confirm is UX; the security boundary is process isolation, host permissions, allowlist.
4. Human confirmation is a mandatory tool argument (`confirmed_by`); "a bot confirmed" = refusal.
5. Errors: transient → retry in the client; loop-limit/deadline → terminal + a message to the human.
6. Budget on tool results (~128 KB) — otherwise the context gets eaten; the model gets a filtered version (`FilteredContent`).
7. Skills instead of catalogs of 20+ tool wrappers: 10 skills = ~10 prefix lines.
8. A subagent is the same loop with clean memory: review and offloading noisy reading, without externally writing tools.
9. Configuration errors (threshold outside (0,1), nil fallback) — at startup, not in production.
10. Redaction by default: model/client never appear in logs with keys.

---

## Relation to Other Chapters

- **Prompting:** For how to use SOPs in prompts, see [Chapter 02: Prompting](../02-prompt-engineering/README.md)
- **Case studies:** For examples of SOPs in real agents, see [Chapter 15: Case Studies](../15-case-studies/README.md)
- **SDK:** How the contracts above map to the chapters — ch. 04 (Config/events), ch. 03 (tools), ch. 12/13 (memory/context), ch. 05/17 (confirm/boundaries)


