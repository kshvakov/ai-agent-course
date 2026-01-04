# Appendix: Reference Guides

## Why This Chapter?

This section contains reference information: glossary of terms, checklists, SOP templates, and decision tables. Use it as a reference when working on projects.

## Glossary

**Agent** — system using LLM for perception, decision-making, and action execution.

**Chain-of-Thought (CoT)** — prompting technique forcing model to generate intermediate reasoning.

**Context Window** — maximum number of tokens model can process in one request.

**Eval (Evaluation)** — test for checking agent quality.

**Few-Shot Learning** — prompting technique with examples in context.

**Function Calling** — mechanism for LLM to call tools via structured JSON.

**Grounding** — binding agent to real data via Tools/RAG to avoid hallucinations.

**Human-in-the-Loop (HITL)** — mechanism for human confirmation of critical actions.

**In-Context Learning (ICL)** — model's ability to learn from examples within prompt.

**Multi-Agent System (MAS)** — system of multiple agents working together.

**Prompt Injection** — attack on agent via input data manipulation.

**RAG (Retrieval Augmented Generation)** — technique for augmenting agent context with relevant documents from knowledge base.

**ReAct (Reason + Act)** — agent architecture: Thought → Action → Observation → Loop.

**Reflexion** — agent self-correction technique through error analysis.

**SOP (Standard Operating Procedure)** — action algorithm encoded in prompt.

**Temperature** — entropy parameter of token probability distribution.

**Token** — text unit processed by model (~0.75 words).

**Tool** — function that agent can call to interact with external world.

**Zero-Shot Learning** — prompting technique without examples.

## Checklists

### Checklist: Model Setup for Agent

- [ ] Model supports Function Calling (checked via Lab 00)
- [ ] `Temperature = 0` set
- [ ] Context window large enough (minimum 4k tokens)
- [ ] System Prompt prohibits hallucinations
- [ ] Dialogue history managed (truncated on overflow)

### Checklist: Creating System Prompt

- [ ] Role (Persona) clearly defined
- [ ] Goal concrete and measurable
- [ ] Constraints explicitly stated
- [ ] Response format (Format) described
- [ ] SOP (if applicable) detailed
- [ ] CoT enabled for complex tasks
- [ ] Few-Shot examples added (if needed)

## Capability Benchmark (Characterization)

Before building agents, you must **scientifically confirm** that selected model has necessary capabilities. In engineering, this is called **Characterization**.

### Why This Chapter?

We don't trust labels ("Super-Pro-Max Model"). We trust tests.

**Problem without check:** You downloaded model "Llama-3-8B-Instruct" and started building agent. After an hour of work, discovered that model doesn't call tools, only writes text. You spent time debugging code, though problem was in model.

**Solution:** Run capability benchmark **before** starting work. This saves hours.

### What Do We Check?

#### 1. Basic Sanity
- Model responds to requests
- No critical API errors
- Basic answer coherence

#### 2. Instruction Following
- Model can strictly adhere to constraints
- Important for agents: they must return strictly defined formats
- **Test:** "Write a poem, but don't use letter 'a'"
- **Why:** Agent must return strictly defined formats, not "reflections"

#### 3. JSON Generation
- Model can generate valid syntax
- All tool interaction built on JSON
- If model forgets closing brace `}`, agent crashes
- **Test:** "Return JSON with fields name and age"

#### 4. Function Calling (Tool Usage)
- Specific model skill to recognize function definitions and form special call token
- Without this, tools are impossible (see [Chapter 04: Tools](../04-tools-and-function-calling/README.md))
- **Why:** This is foundation for Lab 02 and all subsequent labs

### Why Don't All Models Know Tools?

LLM (Large Language Model) is a probabilistic text generator. It doesn't "know" about functions out of the box.

**Function Calling** mechanism is result of special training (Fine-Tuning). Model developers add thousands of examples to training data like:

```
User: "Check weather"
Assistant: <special_token>call_tool{"name": "weather"}<end_token>
```

If you downloaded "bare" Llama 3 (Base model), it hasn't seen these examples. It will simply continue dialogue with text.

**How to check:** Run Lab 00 before starting work with tools.

### Why Is `Temperature = 0` Critical for Agents?

Temperature regulates "randomness" of next token selection:
- **High Temp (0.8+):** Model chooses less probable words. Good for poetry, creative tasks.
- **Low Temp (0):** Model always chooses most probable word (ArgMax). Maximum determinism.

For agents that must output strict JSON or function calls, maximum determinism is needed. Any "creative" error in JSON will break parser.

**Rule:** For all agents, set `Temperature = 0`.

### How to Interpret Results?

#### ✅ All Tests Passed
Model ready for course. Can continue work.

#### ⚠️ 3 out of 4 Tests Passed
Can continue, but with caution. Problems possible in edge cases.

#### ❌ Function Calling Failed
**Critical:** Model not suitable for Lab 02-08. Need different model.

**What to do:**
1. Download model with tool support:
   - `Hermes-2-Pro-Llama-3-8B`
   - `Mistral-7B-Instruct-v0.2`
   - `Llama-3-8B-Instruct` (some versions)
   - `Gorilla OpenFunctions`
2. Restart tests

#### ❌ JSON Generation Failed
Model generates broken JSON (missing braces, quotes).

**What to do:**
1. Try different model
2. Or use `Temperature = 0` (but this doesn't always help)

### Connection with Evals

Capability Benchmark is a primitive **Eval** (Evaluation). In industrial systems (LangSmith, PromptFoo), there are hundreds of such tests.

**Topic development:** See [Chapter 09: Evals and Reliability](../09-evals-and-reliability/README.md) for understanding how to build complex evals for checking agent quality.

### Practice

For performing capability benchmark, see [Lab 00: Model Capability Benchmark](../../../labs/lab00-capability-check/README.md).

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
5. Respond: Send answer to user
```

## Decision Tables

### Decision Table for Incident

| Symptom | Hypothesis | Check | Action | Verification |
|---------|------------|-------|--------|--------------|
| HTTP 502 | Service down | `check_http()` → 502 | - | - |
| HTTP 502 | Error in logs | `read_logs()` → "Syntax error" | `rollback_deploy()` | `check_http()` → 200 |
| HTTP 502 | Error in logs | `read_logs()` → "Connection refused" | `restart_service()` | `check_http()` → 200 |

## Mini-Exercises

### Exercise 1: Create Your SOP

Create SOP for your domain following template from "SOP Templates" section:

```text
SOP for [your task]:
1. [Step 1]
2. [Step 2]
3. [Step 3]
```

**Expected result:**
- SOP clearly describes action process
- Steps sequential and logical
- Checks and verification included

### Exercise 2: Create Decision Table

Create decision table for your task following template from "Decision Tables" section:

| Symptom | Hypothesis | Check | Action | Verification |
|---------|------------|-------|--------|--------------|
| ...     | ...      | ...      | ...      | ...         |

**Expected result:**
- Table covers main scenarios
- For each symptom, there is hypothesis, check, action, and verification

## Connection with Other Chapters

- **Prompting:** How to use SOP in prompts, see [Chapter 02: Prompt Engineering](../02-prompt-engineering/README.md)
- **Case Studies:** Examples of SOP usage in real agents, see [Chapter 10: Case Studies](../10-case-studies/README.md)

---

**Navigation:** [← Advanced Study](../12-advanced-study/README.md) | [Table of Contents](../README.md)
