# 02. Prompting as an Engineering Discipline

## Why does this matter?

A prompt is code for a neural network. The difference from ordinary code is that it works by steering probabilities.

Without proper prompting an agent will:
- Act randomly, without logic
- Skip important analysis steps
- Perform dangerous actions without confirmation
- Reply in the wrong format

In this chapter we'll look at how to write prompts that actually control agent behavior.

### A real-world case

**Situation:** You built an incident-response agent. The user writes: "Service is down, fix it"

**Problem:** The agent immediately restarts the service without checking the logs. Or the opposite: it spends a long time analyzing and never applies a fix.

**Solution:** A proper System Prompt with an SOP (Standard Operating Procedure) defines the order of actions: status → logs → analysis → fix → verification.

## TL;DR: What to Remember

- **The System Prompt** is a specification of agent behavior. It consists of: Role, Goal, Constraints, Format, SOP.
- **In-Context Learning (ICL)**: Zero-shot (instruction-only) — compact and flexible, but the model may misinterpret the format. Few-shot (demonstration) — instruction plus examples, more accurate for complex formats. In practice — a combination.
- **CoT (Chain-of-Thought)**: makes the model think step by step. Critical for agents.
- **SOP**: an action algorithm encoded in the prompt. It defines the process; CoT helps the model follow it.
- **Task Decomposition**: break complex tasks into subtasks.
- **The Tools Schema** is passed in a separate `tools[]` field, not inside the System Prompt.
- **Few-shot examples** must be consistent — one format, otherwise the model gets confused.

## Structure of the System Prompt

A good System Prompt consists of five blocks:

```text
1. Role (Persona)      — who you are
2. Goal                — what you need to achieve
3. Constraints         — what you must not do
4. Format              — how to format your reply
5. SOP                 — the action algorithm
```

### System prompt template

```text
You are [Role] with [experience/qualifications].

Your goal is [Goal].

Constraints:
- [Constraint 1]
- [Constraint 2]

Reply format:
- [Format rule 1]
- [Format rule 2]

SOP for [task type]:
1. [Step 1]
2. [Step 2]
3. [Step 3]
```

### A DevOps example

```text
You are a Senior DevOps Engineer. Your goal is to restore service
health without breaking what still works.

Invariants:
- Destructive actions (restart, rollback, data deletion) — only after
  confirmation: the system may actually be alive and the alert false,
  and a restart would make things worse.
- Do not run commands that wipe data outside the incident zone:
  recovering from them costs more than the incident itself.

Usual incident investigation path (not a rigid order — if there is not
enough data, gather more; the main rule is: never apply a fix before
analysis):
- check the service status (check_http)
- if the status is not 200 — read the latest logs (read_logs)
- use the logs to identify the likely cause (deploy, DB, resources)
- apply the minimal fix (restart/rollback)
- check the status again (check_http)
```

> **Important: delimiter strings in a prompt are a hint to the model, not protection.**
> The model can ignore them or interpret them differently. The real boundary is enforced by code: the tool allowlist, access rights, and mandatory confirmation of dangerous actions via a code mechanism (a Confirm callback or a required tool argument). The prompt explains to the model *why* the invariant exists — this lowers the chance of spontaneous violations, but it does not replace enforcement. How this is done — see [Chapter 05: Safety and HITL](../05-safety-and-hitl/README.md) and [Chapter 17: Security and Governance](../17-security-and-governance/README.md).

### A support example

```text
You are a Tier 2 Customer Support Agent.

Your goal is to resolve the user's problem quickly and politely.

Constraints:
- Always stay polite
- If the problem is complex — escalate
- Do not give technical details if the user is not technical

Reply format:
- Use a structured format: {"action": "...", "user_id": "..."}

SOP for handling a ticket:
1. Read the ticket in full (get_ticket)
2. Collect context (software version, OS, browser)
3. Search the knowledge base (search_kb)
4. If a solution is found — compose the answer
5. If not — escalate (escalate_ticket)
```

### A data analytics example

```text
You are a Data Analyst with experience in SQL and BI tools.

Your goal is to provide accurate data and analytics.

Constraints:
- Work only with read-only SQL (SELECT): a mutating query can corrupt production data, and it cannot be restored from an analytics session
- Check data quality before analysis: on dirty data your conclusions will mislead
- If the data is wrong — say so instead of "fudging" conclusions

Reply format:
- First show the SQL query
- Then the results and the analysis

SOP for analysis:
1. Understand the user's question
2. Formulate the SQL query
3. Verify the table schema (describe_table)
4. Run the query (sql_select)
5. Analyze the results
6. Generate the report
```

### The system prompt is a cacheable prefix

LLM APIs have a prompt cache: an unchanging prefix of the request is cached and is noticeably cheaper and faster. This gives us practical rules:

- The system prompt must be **immutable** from request to request. Any change — a timestamp, a counter, a "personalized" insertion — breaks the cache and makes every request more expensive.
- All dynamics (current time, tool results, new facts) go **at the end of the `messages` history** — via tool results and appended messages. Never mutate `messages[0]`.
- `time.Now()` and other changing values in a system prompt are an anti-pattern.
- Don't pile bulk knowledge into the prompt: large instructions live in skill files that the model reads itself as needed (progressive disclosure) — 10 skills take ~10 lines in the prompt, not 10 pages.

More detail in [Chapter 12: Agent Memory](../12-agent-memory/README.md) and [Chapter 13: Context Engineering](../13-context-engineering/README.md).

## In-Context Learning (ICL): Zero-shot and Few-shot

Now that we've defined the System Prompt structure, the question arises: **what is the best way to convey the desired behavior to the model?**

**In-Context Learning (ICL)** is the model's ability to adapt its behavior based on examples *inside the prompt*, without changing the model's weights.

ICL works in two modes:

### Zero-shot (instruction only)

We give the model only the **instruction**, no examples:

```text
System Prompt: "You are a DevOps engineer. When the user asks you to check logs, use the read_logs tool."

User: "Check the nginx logs"
Assistant: [The model performs the task based on the instruction]
```

**When to use:**
- Simple tasks where the model understands instructions well
- Limited context (tokens need to be saved)
- The model is already trained on similar tasks

### Few-shot (instruction + examples)

We give the model the **instruction plus examples** of the desired behavior:

```text
System Prompt: "You are a DevOps engineer. Examples:

Example 1:
User: "Check the nginx logs"
Assistant: {"tool": "read_logs", "args": {"service": "nginx"}}

Example 2:
User: "Restart the web-01 server"
Assistant: {"tool": "restart", "args": {"name": "web-01"}}

User: "Check the server status"
Assistant: {"tool": "check_status", "args": {"hostname": "web-01"}}
```

> **Important: teaching the model the tool-call format via text in the prompt is a historical technique (recovery).**
> The mainstream approach today is **native tool-calling**: the model is pre-trained on tool calls, and it is enough to pass the JSON Schema in a separate `tools[]` field (see [Chapter 03: Tools](../03-tools-and-function-calling/README.md)) — the call comes back in a separate `tool_calls` field, not as text. Few-shot with textual `{"tool": ...}` examples remains useful in two cases: non-standard *textual* reply formats (not tool calls) and recovery when the model or the environment does not support native tool-calling.

**When to use:**
- A complex reply format (JSON, structured data)
- High format-compliance accuracy is required
- The model may misinterpret the instruction

### Terminology: instruction vs demonstration

The terms **Instruction-only** and **Demonstration** (or **Few-shot**) are alternative ways of describing the same two ICL modes:

- **Instruction-only** = **Zero-shot**: we **describe** to the model what to do (compact, flexible, but the model may misinterpret the format)
- **Demonstration/Few-shot** = **Few-shot**: we **show** the model examples of the desired behavior (the model copies the format precisely, but it takes more tokens)

**Default rule:** use a **combination** — instruction + 1–2 examples for complex formats or edge cases.

### Table for choosing an ICL mode

| Mode | What we give the model | When to use | Example |
|-------|-------------------|-------------------|--------|
| **Zero-shot** (Instruction-only) | Instruction only | Simple tasks, the model understands instructions well, limited context, saving tokens | "You are a DevOps engineer. Check logs via read_logs." |
| **Few-shot** (Demonstration) | Instruction + examples | Complex reply format (JSON, structured data), high format-compliance accuracy required | Instruction + 2–3 examples of the reply format |

### A practical example: Few-shot for support

```go
// System Prompt with Few-shot examples
systemPrompt := `You are a Support agent.

Examples of the reply format:

Example 1:
User: "My account is locked"
Assistant: {"action": "check_account", "user_id": "extract_from_ticket"}

Example 2:
User: "I can't log in"
Assistant: {"action": "check_login", "user_id": "extract_from_ticket"}`

messages := []Message{
    {Role: "system", Content: systemPrompt},
    {Role: "user", Content: "Access problem"},
}

// Message — the neutral struct from Chapter 03:
// type Message struct {
//     Role       string
//     Content    string
//     ToolCalls  []ToolCall
// }

// The model sees the examples in the System Prompt and generates a reply in the same format:
// {"action": "check_account", "user_id": "..."}
```

### Anti-example: inconsistent Few-shot examples

**Bad:** examples in different formats

```text
Example 1:
User: "Check the logs"
Assistant: {"tool": "read_logs", "service": "nginx"}

Example 2:
User: "Restart the server"
Assistant: {"action": "restart", "target": "web-01"}  // A different format!

Example 3:
User: "Status"
Assistant: check_status("web-01")  // Yet another format!
```

**Problem:** The model sees three different formats and doesn't know which one to use. The result is unpredictable.

**Good:** all examples in one format

```text
Example 1:
User: "Check the logs"
Assistant: {"tool": "read_logs", "args": {"service": "nginx"}}

Example 2:
User: "Restart the server"
Assistant: {"tool": "restart", "args": {"name": "web-01"}}  // Same format

Example 3:
User: "Status"
Assistant: {"tool": "check_status", "args": {"hostname": "web-01"}}  // Same format
```

**Result:** The model clearly sees the pattern and follows it.

> **Important: few-shot sets not only the format but also hidden hints.**
> Few-shot examples can do more than show the reply format — they can **implicitly suggest the desired output**. The model may "pick up" a pattern from the examples and drift toward what the demonstrations show, even when that doesn't match the real data or the task.
>
> **Problem:** If the correct answer is always in position "A" in the examples, or all examples lead to one type of solution, the model may adopt that positional or topical bias.
>
> **Solution:** Use diverse examples, shuffle the positions of correct answers, include counterexamples. See the section ["How to Avoid Suggesting the Answer to the Model"](#how-to-avoid-suggesting-the-answer-to-the-model) below.

### How ICL relates to other techniques

- **ICL → CoT:** Demonstrations can define not only the reply format but also the **format of reasoning**. For example, examples with "Thought: ... Action: ... Observation: ..." (the historical ReAct format — more in the CoT section below).
- **CoT → SOP:** An SOP is a **process** fixed by instructions and/or examples. CoT helps the model follow this process step by step.

See also:
- **[Lab 01: Basics](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab01-basics)** — working with context and memory
- **[Lab 02: Tools](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab02-tools)** — reply format via Function Calling
- **[Lab 06: Incident (SOP)](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab06-incident)** — an SOP as an action algorithm

## End-to-end Example: What Exactly the Agent Sends to the LLM

In brief — what lives where in a request to the LLM API:

- **System Prompt** — the text in `messages[0]` with `role = "system"`: instructions (Role, Goal, Constraints), the SOP, and few-shot examples, if used.
- **Tools Schema** — a separate `tools[]` field (NOT inside the prompt): JSON Schema descriptions of the tools. The model sees them together with the prompt.
- **User Input** — `role = "user"` messages in the `messages` history: the current request plus previous user/assistant messages.
- **Tool Results** — your runtime appends a `role = "tool"` message to the end of the history with a `tool_call_id` linking the result to the call.

A compact request skeleton in the middle of a loop (one tool call has already happened). The shape matches the course's neutral Go structs (`Request`/`Message`/`ToolCall`, ch. 04); the exact wire format of the HTTP request (`"type": "function"` + the `function` wrapper) is in [Chapter 03](../03-tools-and-function-calling/README.md):

```json
{
  "model": "glm-5.3",
  "messages": [
    { "role": "system", "content": "You are a DevOps engineer..." },
    { "role": "user", "content": "Check the nginx status" },
    { "role": "assistant", "content": "",
      "tool_calls": [
        { "id": "call_1", "name": "check_status", "arguments": "{\"service\": \"nginx\"}" }
      ] },
    { "role": "tool", "tool_call_id": "call_1", "content": "nginx is ONLINE" }
  ],
  "tools": [
    { "name": "check_status", "description": "Check service status",
      "parameters": {
        "type": "object",
        "properties": { "service": { "type": "string" } },
        "required": ["service"]
      } }
  ]
}
```

The loop: the model returns `tool_calls` → your code (the runtime) validates the call, executes the tool and appends the result to the end of `messages` → a new request is sent with the extended history. No magic: the model simply sees the results of previous tools in its context and generates the next step based on them.

A full walkthrough of the tool-calling protocol — formats, argument validation, error handling — in **[Chapter 03: Tools and Function Calling](../03-tools-and-function-calling/README.md)**.

## Chain-of-Thought (CoT): Think Step by Step

**Chain-of-Thought (CoT)** is the "think step by step" technique. For agents, CoT is critical.

### When is CoT needed?

**Bad:** "Fix the server" (one step)  
**Good:** "Analyze the situation, form a hypothesis, test it, propose a solution" (a chain)

**CoT is needed for:**
- Complex tasks that require analysis
- Tasks with multiple steps
- Tasks where a hypothesis must be tested before acting

**CoT is not needed for:**
- Simple one-step tasks
- Tasks where the reply format matters more than the process

### How do you set CoT?

CoT can be set via an **instruction** (Zero-shot) or via **examples** (Few-shot):

**Zero-shot CoT:**
```text
Think step by step:
1. Analyze the situation
2. Form a hypothesis
3. Test the hypothesis
4. Apply the solution
```

**Few-shot CoT (the historical ReAct format):**
```text
Example of reasoning (this shows how the agent works in a loop):
Thought: The user complains about slowness. I'll start by checking the metrics.
Action: get_cpu_metrics()
Observation: CPU 95%, process: ffmpeg
Thought: ffmpeg is eating resources. I'll check what this process is.
Action: get_process_info(pid=12345)
```

The model sees an example of the reasoning format and follows it. In practice this happens in a loop: each `Observation` is appended to the history by the runtime (your code), and the model sees it on the next iteration.

> **Context: where this format comes from.**
> "Thought / Action / Observation" is the ReAct text protocol (2022): back then models could not call tools natively, and the whole agent loop had to be expressed as text in the prompt. Today a separate textual format is unnecessary: "Thought" is simply the `content` of the model's reply alongside `tool_calls`, and "Observation" is a `role = "tool"` message. Knowing ReAct is useful as the origin of the agent loop and as a recovery technique when the model or the environment does not support native tool-calling — but not as the primary protocol.

### CoT and the agent loop

The **"Thought-Action-Observation"** format is a formalization of the **Agent Loop** (the agent's iterative process) proposed in the ReAct paper. Back then the whole loop lived in text; today the same loop is expressed through native API fields.

**How it works (mapped to the ReAct format):**

1. **Thought** — the `content` of the model's reply: a brief analysis of the situation and the plan for the next step, written by the model alongside `tool_calls`
2. **Action** — the `tool_calls`: the model picks a tool and its parameters (generates a `tool_call`)
3. **Observation** — the runtime (your code) executes the tool and appends the result to the history (`role = "tool"`)
4. **The loop repeats:** the model sees the Observation in its context, thinks again (Thought), and picks the next Action

**Example of an iteration:**

```
Iteration 1:
  Thought: "I need to check CPU metrics"
  Action: get_cpu_metrics()
  Observation: "CPU 95%, process: ffmpeg"  ← added to history by the runtime

Iteration 2 (the model sees the Observation in context):
  Thought: "ffmpeg is eating resources. Let me check the process details"
  Action: get_process_info(pid=12345)
  Observation: "It's a video conversion"  ← added to history by the runtime

Iteration 3:
  Thought: "Legitimate process, but it's choking the system. I'll suggest throttling its priority"
  Action: [final answer to the user]
```

**Important:** There is no magic here — the model simply sees the results of previous tool calls in its context (`messages[]`) and generates the next step based on that context. (The notation above is an educational pseudo-notation for ReAct; in a real model response the same steps appear as `content` + `tool_calls`.)

> **Important: CoT can be post-hoc.**
> The chain of thought (CoT) a model generates does **not necessarily reflect its actual inference process**. The model may first "solve" the task (via its probabilistic mechanism) and only then produce a "nice explanation" of that solution.
>
> **The problem:** Polished CoT is not proof that the answer is correct. The model can confidently rationalize a wrong conclusion.
>
> **The solution:** Don't rely on CoT as the sole source of truth. Verify answers with tools, evals, and runtime validation. For critical decisions, use Human-in-the-Loop regardless of how good the explanation sounds.

See also: **[Chapter 04: Autonomy and Loops](../04-autonomy-and-loops/README.md)** — how the ReAct loop and the agent's iterative process work.

### CoT examples in different domains

Below are educational reasoning trajectories in ReAct notation. Read them as an illustration of the agent's train of thought; in a real loop with native tool calling, the same steps are `content` (the reasoning) and `tool_calls` (the action) in the model's responses.

#### DevOps
```
Thought: The user complains about slowness. I'll start by checking metrics.
Action: get_cpu_metrics()
Observation: CPU 95%, process: ffmpeg
Thought: ffmpeg is eating resources. Let me check what this process is.
Action: get_process_info(pid=12345)
Observation: It's a video conversion started by the user
Thought: This is a legitimate process, but it's blocking the system. I'll suggest throttling its priority.
```

#### Security
```
Thought: A suspicious-activity alert came in. I'll start with triage.
Action: query_siem(query="host=192.168.1.10 AND time>now-1h")
Observation: Many failed login attempts
Thought: This looks like brute force. Let me check the source.
Action: get_source_ip()
Observation: IP from an unfamiliar country
Thought: High risk. I'll isolate the host, but first I'll request confirmation.
```

#### Data analytics
```
Thought: The user is asking about sales. I need to see what data is available.
Action: describe_table("sales")
Observation: The table contains fields: date, region, amount
Thought: Now I'll write an SQL query for last month's sales.
Action: sql_select("SELECT region, SUM(amount) FROM sales WHERE date > NOW() - INTERVAL '1 month' GROUP BY region")
Observation: Results: Region A: 100k, Region B: 150k
Thought: I'll analyze the results and draw conclusions.
```

## SOP and Task Decomposition: The Agent's Process

An **SOP (Standard Operating Procedure)** is an algorithm of actions encoded in a prompt. It defines the process; CoT helps the model follow it step by step.

### The chain: goal → constraints → SOP → CoT

- **Goal/Constraints** set the frame (what to achieve, what must not be done)
- **SOP** sets the process (which steps to perform)
- **CoT** helps follow the steps (thinking at each stage)

### Why is this necessary?

Without an SOP, the agent can "flail": restart immediately, then read logs, then restart again.

**Why an SOP matters: it directs the model's attention**

Without an SOP, the model sees: `User: Fix it`. Its probabilistic mechanism may output: `Call: restart_service`. That's the most "popular" action.

With an SOP, the model is forced to generate text like:
- "Step 1: I need to check the HTTP status." → This raises the probability of calling `check_http`
- "HTTP is 502. Step 2: I need to check logs." → This raises the probability of calling `read_logs`

We **steer the model's attention** down the right channel by explicitly spelling out the steps in the prompt.

### Example SOP for incidents (DevOps)

```text
SOP for a service outage:
1. Check Status: Check the HTTP response code
2. Check Logs: If 500/502 — read the last 20 lines of logs
3. Analyze: Look for keywords:
   - "Syntax error" → Rollback
   - "Connection refused" → Check Database
   - "Out of memory" → Restart
4. Action: Apply the fix according to your analysis
5. Verify: Check the HTTP status again
```

### Example SOP for support

```text
SOP for handling a ticket:
1. Read: Read the ticket in full
2. Context: Gather context (version, OS, browser)
3. Search: Look for similar cases in the knowledge base
4. Decide:
   - If a solution is found → Draft reply
   - If it's a complex issue → Escalate
5. Respond: Send the answer to the user
```

### Example SOP for security

```text
SOP for alert triage:
1. Severity: Determine the criticality (Low/Medium/High/Critical)
2. Evidence: Gather evidence (logs, metrics, network traffic)
3. Triage:
   - If False Positive → Close alert
   - If True Positive → Containment (with confirmation!)
4. Report: Generate a report for the SOC
```

### Structuring tasks (task decomposition)

Complex tasks should be broken into subtasks.

**Without decomposition:**
```
User: "Sort out the database problem"
Assistant: [May try to do everything at once and get lost]
```

**With decomposition:**
```
User: "Sort out the database problem"
Assistant:
1. Check DB availability (ping_db)
2. Check metrics (cpu, memory, connections)
3. Read the logs (read_db_logs)
4. Analyze the errors
5. Propose a solution
```

## Common Prompting Mistakes

### Mistake 1: A prompt that is too generic

**Symptom:** The model doesn't understand what to do and answers in generalities.

**Cause:** The prompt doesn't define a specific role, goal, or process.

**Solution:**
```text
// BAD:
You are a helpful assistant.

// GOOD:
You are a Senior DevOps Engineer with 10 years of experience.
Your goal is to restore services to a working state.
Follow the SOP: 1. Check Status 2. Check Logs 3. Analyze 4. Fix 5. Verify
```

### Mistake 2: No CoT

**Symptom:** The model tries to do everything at once and skips analysis steps.

**Cause:** The prompt doesn't specify a reasoning format (Chain-of-Thought), so the model doesn't "think" step by step.

**Solution:**
```text
// BAD:
Fix the server.

// GOOD:
Think step by step:
1. Analyze the situation
2. Form a hypothesis
3. Test the hypothesis
4. Apply the fix
```

### Mistake 3: No constraints

**Symptom:** The model may perform dangerous actions without confirmation.

**Cause:** The prompt doesn't constrain the agent's actions — so the model follows the "fix it fast" principle.

**Solution:**
```text
// BAD:
You are a DevOps engineer. Do whatever is needed.

// GOOD:
You are a DevOps engineer. Your goal is to restore the service without creating new damage.
Constraints:
- Delete data only after confirmation: the system may still be alive
  and the alert may be false, and deleted data cannot be rolled back
- Gather facts (status, logs) before any action: acting without a diagnosis
  usually hurts more than it helps
```

Remember: these lines are hints to the model that reduce the probability of a mistake. The guarantee comes from code: a tool allowlist and confirmation of dangerous actions as a runtime mechanism (ch. 05).

### Mistake 4: A suggested answer (anchoring bias)

**Symptom:** The model biases its answer toward whatever is explicitly suggested in the prompt or the user's request, even if it's wrong.

**Cause:** The prompt or request contains explicit hints about the desired answer ("I think the correct answer is A", "it's probably X"). A model trained to maximize human preferences (RLHF) may "flatter" and agree.

**Solution:**
```text
// BAD:
User: "I think the problem is in the database. Check the logs."
System Prompt: "The user believes the problem is in the DB. Check the logs."

// GOOD:
User: "I think the problem is in the database. Check the logs."
System Prompt: "Check the logs and analyze all possible causes. Don't limit yourself to the user's assumptions."
```

**Practice:** Separate facts (data, logs, metrics) from the user's preferences and hypotheses. Don't put preferences into the context as facts.

### Mistake 5: Few-shot bias ("the answer is always a")

**Symptom:** The model picks up a positional pattern from the examples: if the correct answer in few-shot examples is always in position "A", the model starts choosing "A" more often than it should.

**Cause:** The few-shot examples lack variety; correct answers always sit in the same position or follow the same pattern.

**Solution:**
```text
// BAD:
Example 1: Question → Answer A (correct)
Example 2: Question → Answer A (correct)
Example 3: Question → Answer A (correct)

// GOOD:
Example 1: Question → Answer A (correct)
Example 2: Question → Answer B (correct)
Example 3: Question → Answer C (correct)
// Or shuffle the positions and add counterexamples
```

**Practice:** Shuffle the positions of correct answers, include varied examples, and add counterexamples (wrong answers with an explanation of why they are wrong).

### Mistake 6: Blind faith in CoT

**Symptom:** The developer takes the model's "nice explanation" as proof that the answer is correct, without checking the answer itself.

**Cause:** The CoT looks logical and convincing, but may be a post-hoc rationalization of a wrong conclusion.

**Solution:**
```text
// BAD:
Model: "Thought: The problem is in the DB. Action: restart_database()"
Developer: "Great, that's logical!" → Executes the action without checking

// GOOD:
Model: "Thought: The problem is in the DB. Action: restart_database()"
Developer: Verifies with tools (check_db_status, read_logs),
           validates with evals, requests confirmation for critical actions
```

**Practice:** CoT is a tool for improving the model's reasoning, not a source of truth. Always verify answers with tools and evals, especially for critical actions.

## How to Avoid Suggesting the Answer to the Model

Few-shot and CoT can inadvertently "suggest" the desired conclusion to the model. Here is a checklist for avoiding this:

### Checklist: Neutral wording

**Good:**
- Use neutral wording in requests
- Separate facts from the user's preferences
- Don't include assumptions as facts in the context

**Bad:**
- "I think the correct answer is A, verify it"
- "The user believes X; treat it as a fact"
- "The problem is most likely Y; check only Y"

### Checklist: Variety in Few-shot examples

**Good:**
- Shuffle the positions of correct answers (A, B, C, D)
- Include diverse task types
- Add counterexamples (wrong answers with an explanation)

**Bad:**
- All correct answers in position "A"
- All examples of the same type
- Only positive examples

### Checklist: Separating preferences from facts

**Good:**
```text
Facts (include in context):
- Logs show a 500 error
- CPU metrics: 95%

Preferences/hypotheses (separately, not as facts):
- The user suspects a DB problem
- Hypothesis: the problem is in the network
```

**Bad:**
```text
Facts:
- The problem is in the database (this is an assumption, not a fact!)
- Logs show a 500 error
```

### Checklist: Verification via tools

**Good:**
- Verify answers with tools (read_logs, check_status)
- Use evals to check resistance to leading hints
- Don't rely on CoT alone as proof

**Bad:**
- Accepting the model's answer just because the CoT looks logical
- Not checking answers with tools
- No evals for anchoring resistance

### Example: An Anti-Prompt and a fixed version

**Bad (suggests the answer):**
```text
You are a DevOps engineer. The user believes the problem is in the database.
Check the logs and confirm that the problem is in the DB.
```

**Problems:**
- Includes the user's assumption as a fact
- Suggests the desired conclusion ("confirm that the problem is in the DB")
- The model may agree even if the problem is not in the DB

**Good (neutral):**
```text
You are a DevOps engineer. The user reported an issue.
Check the logs, metrics, and all possible causes.
Analyze the data objectively, without limiting yourself to assumptions.
```

**Improvements:**
- Doesn't include the assumption as a fact
- Doesn't suggest the desired conclusion
- Requires objective analysis of all causes

## Acceptance Criteria / Checklist

**Passed:**
- [x] Role (Persona) is clearly defined
- [x] Goal is specific and measurable
- [x] Constraints are stated explicitly
- [x] Response format is described
- [x] SOP (if applicable) is spelled out in detail
- [x] CoT is enabled for complex tasks
- [x] Few-shot examples are added (if a complex format is needed)
- [x] Few-shot examples are consistent (one format)
- [x] Few-shot examples are varied (no positional bias)
- [x] The prompt doesn't suggest the desired answer (neutral wording)
- [x] The user's preferences are separated from facts
- [x] Answers are verified with tools and evals, not just via CoT

**Not passed:**
- [ ] The prompt is too generic (no specific role or goal)
- [ ] No CoT for complex tasks
- [ ] No constraints on dangerous actions
- [ ] Few-shot examples use different formats
- [ ] Few-shot examples have positional bias (all answers in position "A")
- [ ] The prompt suggests the desired answer ("confirm that X")
- [ ] The user's preferences are included as facts
- [ ] Blind faith in CoT without verification via tools

## Exercises

### Exercise 1: Write a system prompt

Write a System Prompt for an agent that handles support tickets:

```go
systemPrompt := `
// Your code here
`
```

**Expected result:**
- Role: Support Agent
- Goal: resolve the user's issue
- Constraints: politeness, escalation of complex cases
- Format: structured JSON with `action` and `user_id` fields
- SOP: Read → Context → Search → Decide → Respond

### Exercise 2: Add CoT

Improve the prompt from Exercise 1 by adding Chain-of-Thought:

```go
cotPrompt := `
// Your code here
`
```

**Expected result:**
- A reasoning format has been added: the model first analyzes and formulates a hypothesis, then acts
- In native tool calling, the reasoning lives in the `content` of the response next to `tool_calls` — a separate text format (Thought/Action/Observation) is not needed
- Or an instruction to "Think step by step" (if zero-shot)

## For the curious

> This section explains the mechanics of how few-shot prompting and CoT work at the model level. Feel free to skip it if you only care about the practice.

### Why Few-shot works

### The self-attention mechanism

The **self-attention** mechanism in a transformer lets the model "see" connections between tokens in the context. When the model sees examples:

1. **Intuitively:** the model "adapts" to the pattern it sees in the context
2. **Mechanically:** the vector representations of tokens shift toward the expected format, because the preceding tokens (the examples) have set that context
3. **Practically:** the quality of the result depends heavily on the quality of the examples and their consistency

### Why CoT works

Imagine the task: "What is 23 * 41 + 12?"

**Without CoT:**
- The model has to produce the answer "955" immediately
- That requires an enormous amount of computation in a single step
- The probability of error is high (the model may "guess" wrong)

**With CoT:**
- The model generates: "23 * 40 = 920... 23 * 1 = 23... sum 943... plus 12... answer 955"
- By generating intermediate tokens ("920", "943"), the model **offloads computation into the context**
- The next token is predicted from the *extended* context, which now contains the intermediate results
- This turns a hard problem into a series of easy ones

**For agents, CoT is critical**, because agents solve complex multi-step problems where each step depends on the previous one.

## Relation to Other Chapters

- **LLM physics:** understanding the probabilistic nature of LLMs helps you write effective prompts — see [Chapter 01: LLM Physics](../01-llm-fundamentals/README.md)
- **Tools:** how the prompt affects tool selection — see [Chapter 03: Tools](../03-tools-and-function-calling/README.md)
- **Autonomy:** how SOPs and CoT work in the agent loop — see [Chapter 04: Autonomy](../04-autonomy-and-loops/README.md)

## What's Next?

Once you've finished with prompting, move on to:
- **[03. Tools and Function Calling](../03-tools-and-function-calling/README.md)** — how the agent interacts with the real world
