# 01. LLM Physics — How the Agent's "Brain" Works

## Why does this matter?

To steer an agent, it helps to understand how its "brain" works. Without a basic grasp of LLM physics, you won't be able to:
- Configure the model for your agent correctly
- Understand why the agent behaves nondeterministically
- Manage the context and conversation history
- Avoid hallucinations and errors

In this chapter we'll cover the fundamentals of how LLMs work in plain language, without unnecessary math.

### A real-world case

**Situation:** You built a DevOps agent. The user writes: "Check the status of server web-01"

**Problem:** Sometimes the agent replies with the text "Server is running", and sometimes it calls the `check_status` tool. The behavior is unpredictable.

**Solution:** Understanding the probabilistic nature of LLMs helps you manage generation variety: a low `Temperature` and clear tool `Description`s. But what makes the agent process predictable is not the model's "determinism" — it's structure: strict call schemas, response validation, and guards in code. Understanding the context window helps you manage conversation history.

## The theory in plain words

### The probabilistic nature

**Key fact:** An LLM doesn't think — it predicts.

An LLM is a function `NextToken(Context) -> Distribution`.  
Its input is a sequence of tokens $x_1, ..., x_t$. The model computes a probability distribution over the next token:

$$P(x_{t+1} | x_1, ..., x_t)$$

**What does this mean in practice?**

> **Note:** The examples below use Function Calling — the mechanism for calling tools via the LLM API. For a detailed explanation, see [Chapter 03: Tools and Function Calling](../03-tools-and-function-calling/README.md). For now it's enough to understand that the model receives tool descriptions (`tools[]`) and picks one based on its `Description`.

#### Example 1: DevOps — magic vs reality

**Magic (the way it's usually explained):**
> Prompt: `"Check the server status"`  
> The model sees the context and predicts: "I will call the `check_status` tool" (probability 0.85)

**Reality (what actually happens):**

**1. What gets sent to the model:**

```go
// Neutral structures for a direct tool-calling protocol.
// Most providers (cloud and local) use a protocol that looks the same —
// for details and the full cycle, see Chapter 03.
type Message struct {
    Role       string     // "system" | "user" | "assistant" | "tool"
    Content    string
    ToolCalls  []ToolCall // populated by the model for role="assistant"
    ToolCallID string     // for role="tool": ID of the call this result belongs to
}

type ToolCall struct {
    ID        string // e.g., "call_abc123"
    Name      string // e.g., "check_status"
    Arguments string // JSON string, e.g. {"hostname":"web-01"}
}

type ToolDef struct {
    Name        string
    Description string
    Parameters  json.RawMessage // JSON Schema of the arguments
}

type Request struct {
    Model       string
    Messages    []Message
    Tools       []ToolDef
    Temperature float64
}

// System Prompt (sets the role and behavior)
systemPrompt := `You are a DevOps assistant. 
When user asks about server status, use the check_status tool.
When user asks about logs, use the read_logs tool.
When user asks to restart, use the restart_service tool.`

// User Input
userInput := "Check the server status"

// Descriptions of the available tools (tools schema)
// IMPORTANT: The model sees ALL the tools and picks the right one!
tools := []ToolDef{
    {
        Name:        "check_status",
        Description: "Check the status of a server by hostname. Use this when user asks about server status or availability.",
        Parameters: json.RawMessage(`{
            "type": "object",
            "properties": {
                "hostname": {"type": "string", "description": "Server hostname"}
            },
            "required": ["hostname"]
        }`),
    },
    {
        Name:        "read_logs",
        Description: "Read the last N lines of service logs. Use this when user asks about logs, errors, or troubleshooting.",
        Parameters: json.RawMessage(`{
            "type": "object",
            "properties": {
                "service": {"type": "string", "description": "Service name"},
                "lines": {"type": "number", "description": "Number of lines to read"}
            },
            "required": ["service"]
        }`),
    },
    {
        Name:        "restart_service",
        Description: "Restart a systemd service. Use this when user explicitly asks to restart a service. WARNING: This causes downtime.",
        Parameters: json.RawMessage(`{
            "type": "object",
            "properties": {
                "service_name": {"type": "string", "description": "Service name to restart"}
            },
            "required": ["service_name"]
        }`),
    },
}

// The full API request
req := Request{
    Model: "glm-5.3", // any current model with native tool calling
    Messages: []Message{
        {Role: "system", Content: systemPrompt},
        {Role: "user", Content: userInput},
    },
    Tools: tools,  // Important: the model sees the tool descriptions!
}
```

**2. What the model returns:**

The model does **not return the text** "I will call the tool". It returns a **structured tool call**:

```json
{
  "role": "assistant",
  "content": null,
  "tool_calls": [
    {
      "id": "call_abc123",
      "type": "function",
      "function": {
        "name": "check_status",
        "arguments": "{\"hostname\": \"web-01\"}"
      }
    }
  ]
}
```

**How does the model choose a tool?**

The model sees **all three tools** and their `Description`s:
- `check_status`: "Check the status... Use this when user asks about server status"
- `read_logs`: "Read logs... Use this when user asks about logs"
- `restart_service`: "Restart service... Use this when user explicitly asks to restart"

The user's request: "Check the server status"

The model matches the request against the descriptions:
- [x] `check_status` — the description mentions "server status" → **this one is picked**
- [ ] `read_logs` — the description is about logs, not status
- [ ] `restart_service` — the description is about restarting, not checking

**An example with a different request:**

```go
userInput := "Show me the latest errors in the nginx logs"

// The model sees the same 3 tools
// It matches:
// - check_status: about status, not logs → not a fit
// - read_logs: "Use this when user asks about logs" → THIS ONE IS PICKED
// - restart_service: about restarting → not a fit

// The model returns:
// tool_calls: [{function: {name: "read_logs", arguments: "{\"service\": \"nginx\", \"lines\": 50}"}}]
```

**The point:** The model picks a tool based on the **semantic match** between the user's request and the tool's `Description`. The more precise the `Description`, the better the choice.

**3. What the Runtime does:**

```go
resp, err := client.Complete(ctx, req) // neutral call: one protocol, any provider
if err != nil {
    return err // client errors are not for the model: retry/logging in code (ch. 04)
}
msg := resp.Choices[0].Message // the model's response

// The Runtime checks: are there any tool_calls?
if len(msg.ToolCalls) > 0 {
    call := msg.ToolCalls[0]

    // Parse the arguments (Arguments is a JSON string)
    var args struct {
        Hostname string `json:"hostname"`
    }
    json.Unmarshal([]byte(call.Arguments), &args)

    // Execute the real function
    result := checkStatus(args.Hostname)  // "Server is ONLINE"

    // Feed the result back into the model:
    // the assistant message with tool_calls + the result with role="tool"
    req.Messages = append(req.Messages, msg,
        Message{Role: "tool", Content: result, ToolCallID: call.ID},
    )

    // Send the updated history to the model again
    // The model sees the result and decides what to do next
}
```

**A note about "probabilities":**

Numbers like "probability 0.85" are an **illustration**. Provider APIs (cloud and local) usually do **not** return these probabilities directly (unless you use `logprobs`). What matters is this: when the context contains `tools` with good `Description`s, the model will pick a tool call over text with high probability. But that happens **inside the model** — from the outside we only see the final choice.

#### Example 2: Support — magic vs reality

**Magic:**
> Prompt: `"The user is complaining about a 500 error"`
> The model predicts: "First I'll gather context via `get_ticket_details`" (probability 0.9)

**Reality:** The same principle as in the DevOps example. The model receives 4 tools (`get_ticket_details`, `check_account_status`, `search_kb`, `draft_reply`) with descriptions. It picks `get_ticket_details` because its `Description` contains "Use this FIRST when user reports an error" — the best semantic match for the request.

The model picks tools **sequentially**:

```go
// Iteration 1: The user is complaining about an error
// The model picks: get_ticket_details (gathers context)

// Iteration 2: The ticket details are now in the context
// The model picks: search_kb("error 500") (looks for a solution)

// Iteration 3: The KB solution is now in the context
// The model picks: draft_reply(ticket_id, solution) (drafts a reply)
```

The runtime on each iteration parses the tool call, executes the function, and returns the result. For more on the tool-call protocol, see [Chapter 03](../03-tools-and-function-calling/README.md).

#### Example 3: Data analytics — magic vs reality

**Magic:**
> Prompt: `"Show sales for the last month"`
> The model predicts: "I'll formulate an SQL query via `sql_select`" (probability 0.95)

**Reality:** The model receives 3 tools (`describe_table`, `sql_select`, `check_data_quality`). The request "Show sales for the last month" best matches `sql_select` with its description "when user asks for specific data or reports". The model generates the SQL directly in the arguments:

```json
{
  "tool_calls": [{
    "function": {
      "name": "sql_select",
      "arguments": "{\"query\": \"SELECT region, SUM(amount) FROM sales WHERE date >= NOW() - INTERVAL '1 month' GROUP BY region\"}"
    }
  }]
}
```

For complex questions, the model uses tools **sequentially**: first `describe_table` (learn the schema), then `sql_select` (fetch the data), then `check_data_quality` (check quality). The runtime validates that it is a SELECT (not DELETE/DROP!) and executes the query over a read-only connection. For more, see [Chapter 03](../03-tools-and-function-calling/README.md).

### Why does this matter for an engineer?

#### 1. Nondeterminism

If you run the agent twice with the same prompt, you may get different actions.

**Example:**
```
Request 1: "Check the server"
Response 1: [Calls check_status]

Request 2: "Check the server" (same prompt)
Response 2: [Replies with the text "Server is running"]
```

**Solution:** Lower the `Temperature` (0.0–0.3 is typical for agent loops) — this reduces generation variety and makes tool selection more stable. But don't expect "determinism": for many providers, `T=0` does not technically guarantee greedy decoding (request batching, MoE architectures). Reproducibility of the agent process comes from structure (JSON Schema, validation), guards in code, and evals — more on this in the "Temperature" section below.

```go
req := Request{
    Model:       "glm-5.3",
    Temperature: 0.2, // less spread in tool selection
    // ...
}
```

#### 2. Hallucinations

The model strives to generate *plausible* text, not *true* text.

**DevOps example:** The model may suggest "use the `--force` flag" for a command that doesn't support it.

**Data example:** The model may generate SQL with a nonexistent field `user.email` instead of `users.email`.

**Support example:** The model may "invent" a solution to the problem that isn't in the knowledge base.

**Solution:** **Grounding**. We give the agent access to real data (Tools/RAG) and forbid it from inventing facts.

```go
systemPrompt := `You are a DevOps assistant.
Do not invent facts: you have tools that return real data, and real data
is always more reliable than a plausible guess. If you don't know
something, say "I don't know" or use a tool.`
```

## Tokens and the Context Window

### What is a token?

A **token** is a unit of text that the model processes.
- One token ≈ 0.75 words (in English)
- In Russian: one word ≈ 1.5 tokens

**Example:**
```
Text: "Check the server status"
Tokens: ["Check", " the", " server", " status"]  // ~4 tokens
```

### The context window

The **context window** is the model's "working memory".

**Orders of magnitude (no exact figures, which go stale quickly):**
- Current models have context windows from ~128k to ~1M tokens (GLM-5.3, Fable 5.1, Opus 5.5)
- Check the provider's documentation for exact values — windows change more often than book editions come out

> **Note:** What matters is the principle: the larger the context window, the more information the agent can "remember" in a single request. But a large window doesn't eliminate the need for context management: a long history costs money and degrades in quality (see [Chapter 13: Context Engineering](../13-context-engineering/README.md)).

**What does this mean for an agent?**

Everything the agent "knows" about the current task is what fits into the context window (Prompt + History).

**Sample calculation (approximate):**
```
Context window: 128k tokens
System Prompt + tools: 2k tokens
Conversation history: 60k tokens
Tool results: 30k tokens
Space left: 36k tokens
```

> **Important for the agent (the rule of chapter 13).** In the agent runtime, **the primary source of the token count is `usage.PromptTokens` / `usage.CompletionTokens` from the provider's response**, not your own counter: the provider already knows its tokenizer and has applied it. For a rough **pre-flight estimate before sending**, the heuristic **~3 characters/token** is enough (closer to 4 for English); tokenizers like `tiktoken` are more accurate, but that precision is usually unnecessary when deciding "is it time to condense the history". Basing the agent's token budget only on your own estimates is a recipe for a 10–30% discrepancy with reality, which hurts most with a large window. For more, see [Chapter 13: Context Engineering](../13-context-engineering/README.md).

If the history overflows, the agent "forgets" the beginning of the conversation. In practice this means: either the earliest messages are truncated/condensed (and the model genuinely can't see them), or, if the overflow isn't handled, the API returns a context-window-exceeded error.

**The model is stateless:** It doesn't remember your previous request unless you send it again in `messages`.

```go
// Every request must include the full history
messages := []Message{
    {Role: "system", Content: systemPrompt},
    {Role: "user", Content: "Check the server"},
    {Role: "assistant", Content: "Checking..."},
    {Role: "tool", Content: "Server is ONLINE"},
    {Role: "user", Content: "And what about the database?"},  // The agent sees the whole history!
}
```

## Temperature

**Temperature** is a setting for the quality and variety of generation: it controls the entropy of the probability distribution when picking the next token.

```go
Temperature: 0.0–0.3  // typical for agent loops and structured calls
Temperature: 0.7      // a balance of creativity and stability
Temperature: 1.0+     // creative tasks, brainstorming
```

### When to use which value?

| Temperature | Use | Example |
|-------------|-----|---------|
| 0.0–0.3 | Agent loops, structured calls, JSON | A DevOps agent consistently calls `restart_service` instead of "getting creative"; tool and argument selection is additionally pinned down by JSON Schema and validation |
| 0.3–0.7 | Template replies, drafts | A support agent generates reply templates |
| 0.7–1.0 | Creative tasks | A product agent writes marketing copy |

### What temperature does not give you

A common misconception: "`Temperature = 0` makes the agent deterministic." It does not:

- **Technically**, `T=0` at many providers does not guarantee greedy determinism: request batching and MoE architectures introduce nondeterminism regardless of temperature.
- **Reproducibility of the agent process** comes not from sampling settings but from structure: strict JSON Schema for tool arguments, response validation, guards in code (controlling permitted actions), and evals that pin down behavior.

A low temperature reduces the spread and "noise" in tool selection — that is useful. But it is a generation-quality setting, not an on/off switch for "determinism".

**A practical example:**

```go
// BAD: for the agent loop
req := Request{
    Temperature: 0.9,  // too much variance: the model will "get creative"
    // ...
}

// GOOD: for the agent loop
req := Request{
    Temperature: 0.2,  // less variance; process predictability comes from schema + guards + evals
    // ...
}
```

## Choosing a Model

Not all models are equally good for agents.

### Current models (as of this revision)

- **Fable 5.1** and **Opus 5.5** — flagship models for complex agent scenarios
- **GLM-5.3** — a strong model with native tool calling, a good price/quality balance
- **GLM-5.3 Flash** — a cheap and fast option for mass and simple calls

> **Note:** Model names and versions live for months, not years. Trust not lists from books, but the provider's up-to-date documentation and your own capability test (see below).

### Running locally

For local runs (Ollama, vLLM, LM Studio), pick a current instruct model with declared function calling support — and be sure to run a capability benchmark: how tools behave in local runtimes depends on the model's chat template, not just its weights.

### Selection criteria

1. **Function Calling support (native tool calling):** The model must be able to generate structured tool calls.
    - [x] Good: models with native tool calling (GLM-5.3, GLM-5.3 Flash, Fable 5.1, Opus 5.5)
    - [ ] Bad: base models not trained on tools

    > **Note:** Specific models change. Verify function calling support via a capability benchmark (see [Appendix: Capability Benchmark](../appendix/README.md#capability-benchmark-characterization)).

2. **Context size:** The context window must fit the system prompt, tools, history, and tool results. Modern models offer ~128k tokens and up — with context management (see [Chapter 13](../13-context-engineering/README.md)) that is usually enough.

3. **Instruction-following quality:** The model must strictly follow the System Prompt.
    - Verified via a capability benchmark (see [Appendix: Capability Benchmark](../appendix/README.md#capability-benchmark-characterization))

4. **Chat template compatibility (for local runtimes):** The message roles (system, user, assistant, tool) must match the model's chat template, otherwise the tool protocol won't work. See more in [Mistake 3: LM Studio — Wrong Prompt Template (Role Error)](#mistake-3-lm-studio-wrong-prompt-template-role-error) in the "Common Mistakes" section. Cloud providers with native tool calling don't have this problem.

### How do you check a model?

**Theory:** See [Appendix: Capability Benchmark](../appendix/README.md#capability-benchmark-characterization) — a detailed description of what we check and why it matters.

**Practice:** See [Lab 00: Model Capability Benchmark](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab00-capability-check) — a ready-made tool for checking a model.

## Common Mistakes

### Mistake 1: The model behaves differently on identical requests

**Symptom:** The same prompt gives different results. The agent sometimes calls a tool, sometimes replies with text.

**Cause:** An LLM samples from a probability distribution. At high temperature the variance grows, and "gray zones" — situations where both text and a tool call are plausible — become unstable.

**Fix:**

```go
// BAD: high temperature in the agent loop
req := Request{
    Temperature: 0.9,  // high variance: tool selection is unstable
    // ...
}

// GOOD: low temperature + structural guarantees
req := Request{
    Temperature: 0.2,  // less variance
    // ...
}
// And most importantly: validate tool calls against JSON Schema, reject
// unparseable arguments, restrict the allowed tools.
// Agent process reproducibility comes from guards and evals, not temperature.
```

### Mistake 2: The context overflows

**Symptom:** The agent "forgets" the beginning of the conversation. After N messages it stops remembering what was discussed at the start. Or the API returns a context window exceeded error.

**Cause:** The dialogue history grows and exceeds the model's context window size.

**Fix: Condense — a single condense as you approach the limit**

The canonical mechanism (more in [Chapter 09: Memory](../09-agent-architecture/README.md#memory) and [Chapter 13: Context Engineering](../13-context-engineering/README.md)):

1. **The threshold is ~0.85 of the context window**, computed from `usage.PromptTokens` in the provider's response (not from your own counter).
2. **At most one condense per run:** when the threshold is exceeded, the old part of the history (the head) is summarized with a separate LLM call, and the fresh part (the tail) is kept as is.
3. **`tool_call` ↔ `tool_result` pairs are never split:** the condense boundary passes only between complete pairs, otherwise the history becomes invalid.
4. **The cacheable prefix is unchanged:** `messages[0]` (the system prompt) and the `tools` list stay intact — this is exactly the prefix the provider caches.

```go
// Condense: threshold based on provider data, head -> summary, tail as is
const condenseThreshold = 0.85

if float64(resp.Usage.PromptTokens) > condenseThreshold*float64(ctxWindow) && !condensed {
    // 1. Boundary: the index where the last tool_call <-> tool_result
    //    pair completed (never split a pair!)
    cut := lastCompleteToolBoundary(req.Messages)

    // 2. Head (everything after system) is summarized with a separate LLM call
    summary := summarize(ctx, req.Messages[1:cut])

    // 3. New context: system (unchanged!) + summary + tail
    req.Messages = append([]Message{
        req.Messages[0], // system: must not be mutated — it would break the prompt cache
        {Role: "user", Content: "Summary of earlier conversation:\n" + summary},
    }, req.Messages[cut:]...)
    condensed = true // at most once per run
}
```

**Why NOT "insert the summary as a second system message"?**

A common temptation is to glue the condensed history right after the system message:

```go
// BAD: mutates the cacheable prefix
compressed := []Message{
    systemMsg,
    {Role: "system", Content: "Summary of previous conversation:\n" + summary},
}
```

This breaks the **prompt cache**: the provider caches an immutable prefix (system + tools). Inserting a new message right after the system message changes the prefix — the cache is invalidated, and every request after the condense is paid for in full. The summary should go into the body of the history (for example, as a `user` message), and `messages[0]` must not be touched.

**Why not truncation?**

Naive truncation ("System + the last N messages") loses important information from the beginning: the user's name, task context, decisions already made. Condense preserves the essence: 2000 tokens shrink to ~200 while keeping the key facts.

**Example:**
```
Original history (60k tokens):
- User: "My name is Ivan, I'm a DevOps engineer"
- Assistant: "Hi, Ivan!"
- User: "We have a problem with our server"
- Assistant: "Describe the problem"
... (50 more messages and tool results)
```

- [ ] After truncation: we lose the name and context
- [x] After condense: "The user is Ivan, a DevOps engineer. We discussed a server problem. Current task: diagnostics." + recent messages as is

See more: the "Memory" section in [Chapter 09: Agent Anatomy](../09-agent-architecture/README.md#memory) and [Lab 09: Context Optimization](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab09-context-optimization)

### Mistake 3: LM Studio — wrong prompt template (role error)

**Symptom:** When running locally via LM Studio you get an error like `"Only user and assistant roles are supported!"`.

**Cause and fix:** The model's chat template only accepts the `user`/`assistant` roles and doesn't know the `system`/`tool` roles the agent needs. In the model settings, choose a template that matches the model (ChatML-compatible or specific to the model family), or take a model from the repository that already has a correct template. Cloud providers with native tool calling don't have this problem.

## Acceptance Criteria / Checklist

**Passed:**
- [x] You understand that an LLM predicts tokens rather than "thinking"
- [x] You understand that temperature is a generation-quality setting; agent process reproducibility comes from JSON Schema, validation, guards, and evals
- [x] You understand the limits of the context window
- [x] You know how to manage dialogue history (condense at ~0.85 of the window based on `usage.PromptTokens`)
- [x] The model supports Function Calling (verified via [Lab 00](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab00-capability-check))
- [x] The System Prompt gives a motivated ban on hallucinations, and grounding via Tools/RAG is in place

**Not passed:**
- [ ] The variance in agent behavior is uncontrolled (high temperature, no tool call validation)
- [ ] The agent "forgets" the beginning of the conversation (context overflows, condense not implemented)
- [ ] The model invents facts (no grounding via Tools/RAG)

## Exercises

### Exercise 1: Counting tokens

> **Context.** This function is a **pre-estimate**, used to gauge whether it's time to condense the history **before** sending a request. After the request, we use the provider's `resp.Usage.PromptTokens` as the authoritative source. See the note in the "Context Window" section above and [Chapter 13](../13-context-engineering/README.md).

Write a function that approximately counts the number of tokens in a text:

```go
func estimateTokens(text string) int {
    // Rough estimate: 1 token ≈ 4 characters (for English)
    // For Russian: 1 token ≈ 3 characters
    return len(text) / 4
}
```

**Expected result:**
- The function returns an approximate token count
- It accounts for the difference between English and Russian text

### Exercise 2: Managing history

Implement a function that truncates the message history:

```go
func trimHistory(messages []Message, maxTokens int) []Message {
    // Keep the System Prompt + the latest messages that fit into maxTokens
    // The truncation boundary must not split a tool_call <-> tool_result pair
    // ...
}
```

**Expected result:**
- The System Prompt always stays first (messages[0] is not mutated)
- Recent messages are added as long as maxTokens is not exceeded
- The truncation boundary does not split `tool_call` ↔ `tool_result` pairs
- The function returns the truncated history

## For the curious

> A dive into the formalism. Feel free to skip if you only care about practice.

### Temperature — formally

Temperature $T$ scales the logits of the distribution $P(x_{t+1} | x_1, ..., x_t)$ before the softmax: as $T \to 0$ the distribution degenerates into an argmax (greedy), at $T = 1$ it stays "as is", and at $T > 1$ it flattens and becomes more random. Hence the practical effect from the "Temperature" section: a low temperature narrows the generation spread but does not guarantee reproducibility at the provider level (batching, MoE).

### Why does the model "choose" a tool?

Formally, the model doesn't "choose" — it generates a sequence of tokens that matches the tool call format, because it was trained on examples of function calls. The System Prompt and `tools[]` bias the distribution toward that format: the better the tool's `Description` matches the request, the higher the probability of the corresponding tokens. No magic — just the result of training.

## Relation to Other Chapters

- **Function Calling:** For more on how the model generates tool calls, see [Chapter 03: Tools](../03-tools-and-function-calling/README.md)
- **Context window:** For how to manage message history, see [Chapter 09: Agent Anatomy](../09-agent-architecture/README.md) and [Chapter 13: Context Engineering](../13-context-engineering/README.md)
- **Temperature:** For why temperature is a generation-quality setting while structure, guards, and evals provide reproducibility, see the "Temperature" section in this chapter and [Chapter 13: Context Engineering](../13-context-engineering/README.md)

## What's Next?

After learning the physics of LLMs, move on to:
- **[02. Prompting as Programming](../02-prompt-engineering/README.md)** — how to steer model behavior through prompts
