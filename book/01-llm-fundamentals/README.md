# 01. LLM Physics — How the Agent's "Brain" Works

## Why This Chapter?

To control an agent, it helps to understand how its "brain" works. Without basic LLM physics, you won't be able to:
- Properly configure the model for the agent
- Understand why the agent behaves non-deterministically
- Manage context and conversation history
- Avoid hallucinations and errors

In this chapter, we'll cover the basics of how LLMs work in simple terms, without excessive math.

### Real-World Case Study

**Situation:** You've created a DevOps agent. The user writes: "Check the status of server web-01"

**Problem:** The agent sometimes responds with text "Server is working" and sometimes calls the `check_status` tool. The behavior is unpredictable.

**Solution:** Understanding the probabilistic nature of LLMs and setting `Temperature = 0` makes behavior deterministic. Understanding the context window helps manage conversation history.

## Theory in Simple Terms

### Probabilistic Nature

**Key Fact:** LLM doesn't think, it predicts.

LLM is a function `NextToken(Context) -> Distribution`.  
A sequence of tokens $x_1, ..., x_t$ is fed as input. The model computes a probability distribution for the next token:

$$P(x_{t+1} | x_1, ..., x_t)$$

**What does this mean in practice?**

> **Note:** The examples below use Function Calling — a mechanism for calling tools via the LLM API. See [Chapter 03: Tools and Function Calling](../03-tools-and-function-calling/README.md) for a detailed explanation. For now, it's enough to understand that the model receives tool descriptions (`tools[]`) and selects the right one based on `Description`.

#### Example 1: DevOps — Magic vs Reality

**Magic (as usually explained):**
> Prompt: `"Check server status"`  
> Model processes context and predicts: "I will call the `check_status` tool" (probability 0.85)

**Reality (how it actually works):**

**1. What gets sent to the model:**

```go
// System Prompt (sets role and behavior)
systemPrompt := `You are a DevOps assistant. 
When user asks about server status, use the check_status tool.
When user asks about logs, use the read_logs tool.
When user asks to restart, use the restart_service tool.`

// User Input
userInput := "Check server status"

// Description of available tools (tools schema)
// IMPORTANT: The model receives ALL tools and selects the needed one!
tools := []openai.Tool{
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
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
    },
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
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
    },
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
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
    },
}

// Full API request
messages := []openai.ChatCompletionMessage{
    {Role: "system", Content: systemPrompt},
    {Role: "user", Content: userInput},
}

req := openai.ChatCompletionRequest{
    Model:    "gpt-4o-mini",
    Messages: messages,
    Tools:    tools,  // Note: the model receives tool descriptions!
}
```

**2. What the model returns:**

The model **doesn't return text** like "I will call the tool". Instead, it returns a **structured tool call**:

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

**How does the model choose the tool?**

The model receives **all three tools** and their `Description`:
- `check_status`: "Check the status... Use this when user asks about server status"
- `read_logs`: "Read logs... Use this when user asks about logs"
- `restart_service`: "Restart service... Use this when user explicitly asks to restart"

User request: "Check server status"

The model matches the request with descriptions:
- [x] `check_status` — description contains "server status" → **selects this**
- [ ] `read_logs` — description is about logs, not status
- [ ] `restart_service` — description is about restart, not checking

**Example with a different request:**

```go
userInput := "Show the latest errors in nginx logs"

// Model sees the same 3 tools
// Matches:
// - check_status: about status, not logs → doesn't fit
// - read_logs: "Use this when user asks about logs" → SELECTS THIS
// - restart_service: about restart → doesn't fit

// Model returns:
// tool_calls: [{function: {name: "read_logs", arguments: "{\"service\": \"nginx\", \"lines\": 50}"}}]
```

**Takeaway:** The model selects a tool based on **semantic matching** between the user's request and the tool's `Description`. The more accurate the `Description`, the better the selection.

**3. What Runtime does:**

```go
resp, _ := client.CreateChatCompletion(ctx, req)
msg := resp.Choices[0].Message

// Runtime checks: are there tool_calls?
if len(msg.ToolCalls) > 0 {
    // Parse arguments
    var args struct {
        Hostname string `json:"hostname"`
    }
    json.Unmarshal([]byte(msg.ToolCalls[0].Function.Arguments), &args)
    
    // Execute the real function
    result := checkStatus(args.Hostname)  // "Server is ONLINE"
    
    // Return result back to the model
    messages = append(messages, openai.ChatCompletionMessage{
        Role:       "tool",
        Content:    result,
        ToolCallID: msg.ToolCalls[0].ID,
    })
    
    // Send updated history to the model again
    // Model receives the result and decides what to do next
}
```

**Note about "probabilities":**

Numbers like "probability 0.85" are **illustrations** for understanding. OpenAI/local model APIs usually **do not return** these probabilities directly (unless using `logprobs`). It's important to understand the principle: when there are `tools` with good `Description` in the context, the model will likely choose a tool call instead of text. But this happens **inside the model**, we only see the final choice.

#### Example 2: Support — Magic vs Reality

**Magic:**
> Prompt: `"User reports error 500"`
> Model predicts: "First I'll gather context via `get_ticket_details`" (probability 0.9)

**Reality:** The principle is the same as in the DevOps example. The model receives 4 tools (`get_ticket_details`, `check_account_status`, `search_kb`, `draft_reply`) with descriptions. It selects `get_ticket_details` because its `Description` contains "Use this FIRST when user reports an error" — the best semantic match for the request.

The model selects tools **sequentially**:

```go
// Iteration 1: User reports an error
// Model selects: get_ticket_details (gathers context)

// Iteration 2: Ticket details now appear in context
// Model selects: search_kb("error 500") (searches for solution)

// Iteration 3: KB solution now appears in context
// Model selects: draft_reply(ticket_id, solution) (creates response)
```

Runtime parses the tool call on each iteration, executes the function, and returns the result. More on the tool call protocol in [Chapter 03](../03-tools-and-function-calling/README.md).

#### Example 3: Data Analytics — Magic vs Reality

**Magic:**
> Prompt: `"Show sales for the last month"`
> Model predicts: "I'll formulate SQL query via `sql_select`" (probability 0.95)

**Reality:** The model receives 3 tools (`describe_table`, `sql_select`, `check_data_quality`). The request "Show sales for the last month" best matches `sql_select` with description "when user asks for specific data or reports". The model generates SQL directly in the arguments:

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

For complex questions, the model uses tools **sequentially**: first `describe_table` (learn the structure), then `sql_select` (get data), then `check_data_quality` (check quality). Runtime validates that it's a SELECT (not DELETE/DROP!) and executes the query through a read-only connection. More details in [Chapter 03](../03-tools-and-function-calling/README.md).

### Why Is This Important for Engineers?

#### 1. Non-Determinism

If you run the agent twice with the same prompt, you may get different actions.

**Example:**
```
Request 1: "Check server"
Response 1: [Calls check_status]

Request 2: "Check server" (same prompt)
Response 2: [Responds with text "Server is working"]
```

**Solution:** `Temperature = 0` (Greedy decoding) compresses the distribution, forcing the model to always choose the most probable path.

```go
req := openai.ChatCompletionRequest{
    Temperature: 0,  // Deterministic behavior
    // ...
}
```

#### 2. Hallucinations

The model strives to generate *plausible*, not *true* text.

**DevOps example:** The model may write "use flag `--force`" for a command that doesn't support it.

**Data example:** The model may generate SQL with a non-existent field `user.email` instead of `users.email`.

**Support example:** The model may "invent" a solution to a problem that doesn't exist in the knowledge base.

**Solution:** **Grounding**. We give the agent access to real data (Tools/RAG) and forbid inventing facts.

```go
systemPrompt := `You are a DevOps assistant.
CRITICAL: Never invent facts. Always use tools to get real data.
If you don't know something, say "I don't know" or use a tool.`
```

## Tokens and Context Window

### What Is a Token?

A **token** is a unit of text that the model processes.
- One token ≈ 0.75 words (in English)
- In Russian: one word ≈ 1.5 tokens

**Example:**
```
Text: "Check server status"
Tokens: ["Check", " server", " status"]  // ~3 tokens
```

### Context Window

The **context window** is the model's "working memory".

**Examples of context window sizes (at time of writing):**
- GPT-3.5: 4k tokens (~3000 words)
- GPT-4 Turbo: 128k tokens (~96000 words)
- Llama 3 70B: 8k tokens

> **Note:** Specific models and context sizes may change over time. It's important to understand the principle: the larger the context window, the more information the agent can "remember" within a single request.

**What does this mean for the agent?**

Everything the agent "knows" about the current task is what fits in the context window (Prompt + History).

**Example calculation (approximate):**
```
Context window: 4k tokens
System Prompt: 200 tokens
Conversation history: 3000 tokens
Tool results: 500 tokens
Remaining space: 300 tokens
```

> **Note:** This is an approximate estimate. Exact token counting depends on the model and library used (e.g., `tiktoken` for OpenAI models).

If history overflows, the agent "forgets" the beginning of the conversation. In practice, it happens for one of two reasons: either your runtime trims/summarizes old messages to fit the limit (the model never sees them), or the API rejects the request with a context-length error if you don't handle overflow.

**The Model is Stateless:** It doesn't remember your previous request if you don't pass it again in `messages`.

```go
// Each request must include the full history
messages := []openai.ChatCompletionMessage{
    {Role: "system", Content: systemPrompt},
    {Role: "user", Content: "Check server"},
    {Role: "assistant", Content: "Checking..."},
    {Role: "tool", Content: "Server is ONLINE"},
    {Role: "user", Content: "What about the database?"},  // Agent receives full history!
}
```

## Temperature

**Temperature** is a parameter that controls the entropy of the probability distribution.

```go
Temperature = 0  // Deterministic (for agents!)
Temperature = 0.7  // Balance of creativity and stability
Temperature = 1.0+  // Creative, but unstable
```

### When to Use Which Value?

| Temperature | Usage | Example |
|-------------|-------|---------|
| 0.0 | Agents, JSON generation, Tool Calling | DevOps agent should consistently call `restart_service`, not "create" |
| 0.1-0.3 | Structured responses | Support agent generates response templates |
| 0.7-1.0 | Creative tasks | Product agent writes marketing texts |

**Practical example:**

```go
// BAD: For agent
req := openai.ChatCompletionRequest{
    Temperature: 0.9,  // Too random!
    // ...
}

// GOOD: For agent
req := openai.ChatCompletionRequest{
    Temperature: 0,  // Maximum determinism
    // ...
}
```

## Choosing a Model for Local Deployment

Not all models are equally good for agents.

### Selection Criteria

1. **Function Calling Support:** The model must be able to generate structured tool calls.
    - [x] Good: Models fine-tuned on function calling (e.g., `Hermes-2-Pro`, `Llama-3-Instruct`, `Mistral-7B-Instruct` at time of writing)
    - [ ] Bad: Base models without fine-tuning on tools

    > **Note:** Specific models may change. It's important to verify function calling support through capability benchmark (see [Appendix: Capability Benchmark](../appendix/README.md#capability-benchmark-characterization)).

2. **Context Size:** Complex tasks require large context.
    - Minimum: 4k tokens
    - Recommended: 8k+

3. **Instruction Following Quality:** The model must strictly follow System Prompt.
    - Verified through capability benchmark (see [Appendix: Capability Benchmark](../appendix/README.md#capability-benchmark-characterization))

4. **Prompt Template Compatibility (when using LM Studio):** If you're using LM Studio, make sure the selected prompt template supports all required roles (system, user, assistant, tool). If the template only supports `user` and `assistant`, you'll get an error: `"Only user and assistant roles are supported!"`. To fix this, select the correct template in model settings (e.g., `ChatML` or a model-specific template like `Mistral Instruct` for Mistral models). See more details in [Mistake 3: LM Studio — Wrong Prompt Template](#mistake-3-lm-studio--wrong-prompt-template-role-support-error) in the "Common Mistakes" section.

### How to Test a Model?

**Theory:** See [Appendix: Capability Benchmark](../appendix/README.md#capability-benchmark-characterization) — detailed description of what we test and why it's important.

**Practice:** See [Lab 00: Model Capability Benchmark](../../labs/lab00-capability-check/README.md) — ready tool for testing the model.

## Common Mistakes

### Mistake 1: Model is Non-Deterministic

**Symptom:** The same prompt gives different results. Agent sometimes calls a tool, sometimes responds with text.

**Cause:** `Temperature > 0` makes the model random. It chooses not the most probable token, but a random one from the distribution.

**Solution:**
```go
// BAD
req := openai.ChatCompletionRequest{
    Temperature: 0.7,  // Random behavior!
    // ...
}

// GOOD
req := openai.ChatCompletionRequest{
    Temperature: 0,  // Always use for agents
    // ...
}
```

### Mistake 2: Context Overflow

**Symptom:** Agent "forgets" the beginning of the conversation. After N messages, stops remembering what was discussed at the start.

**Cause:** Conversation history exceeds the model's context window size. Old messages get "pushed out" of context.

**Solution:**

There are two approaches:

**Option 1: History Trimming (simple, but we lose information)**
```go
// BAD: We lose important information from the start of conversation!
if len(messages) > maxHistoryLength {
    messages = append(
        []openai.ChatCompletionMessage{messages[0]},  // System
        messages[len(messages)-maxHistoryLength+1:]...,  // Last ones
    )
}
```

**Option 2: Context Compression via Summarization (better solution)**

Instead of trimming, it's better to **compress** old messages via LLM, preserving important information:

```go
// 1. Split into "old" and "new" messages
systemMsg := messages[0]
oldMessages := messages[1 : len(messages)-10]  // All except last 10
recentMessages := messages[len(messages)-10:]  // Last 10

// 2. Compress old messages via LLM
summary := summarizeMessages(ctx, client, oldMessages)

// 3. Assemble new context: System + Summary + Recent
compressed := []openai.ChatCompletionMessage{
    systemMsg,
    {
        Role:    "system",
        Content: fmt.Sprintf("Summary of previous conversation:\n%s", summary),
    },
}
compressed = append(compressed, recentMessages...)
```

**Why is summarization better than trimming?**

- **Preserves important information:** User name, task context, decisions made
- **Saves tokens:** Compresses 2000 tokens to 200, preserving essence
- **Agent remembers start:** Can answer questions about early messages

**Example:**
```
Original history (2000 tokens):
- User: "My name is Ivan, I'm a DevOps engineer"
- Assistant: "Hello, Ivan!"
- User: "We have a server problem"
- Assistant: "Describe the problem"
... (50 more messages)
```
 
- [ ] After trimming: We lose name and context
- [x] After summarization: "User Ivan, DevOps engineer. Discussed server problem. Current task: diagnostics."

**When to use:**
- **Trimming:** Quick one-time tasks, history not important
- **Summarization:** Long sessions, contextual information important, autonomous agents

See more: section "Context Optimization" in [Chapter 09: Agent Anatomy](../09-agent-architecture/README.md#context-optimization) and [Lab 09: Context Optimization](../../labs/lab09-context-optimization/README.md)

### Mistake 3: LM Studio — Wrong Prompt Template (role support error)

**Symptom:** When trying to use a model through LM Studio, you get an error:

```json
{
  "error": "Error rendering prompt with jinja template: \"Only user and assistant roles are supported!\".\n\nThis is usually an issue with the model's prompt template. If you are using a popular model, you can try to search the model under lmstudio-community, which will have fixed prompt templates. If you cannot find one, you are welcome to post this issue to our discord or issue tracker on GitHub. Alternatively, if you know how to write jinja templates, you can override the prompt template in My Models > model settings > Prompt Template."
}
```

**Cause:** The prompt template in LM Studio only accepts `user` and `assistant` roles and doesn't support `system` or `tool` roles that agents need. This usually happens when an incorrect community template is selected or the default template doesn't match your model.

**Solution:**

Choose the correct prompt template in model settings. Here's how to fix it for `mistralai/mistral-7b-instruct-v0.3`:

1. Open LM Studio
2. Go to **My Models**
3. Find the model `mistralai/mistral-7b-instruct-v0.3`
4. Click the three dots → **Model Settings**
5. Go to the **Prompt Template** tab
6. Select the correct template:
    - For Mistral: **Mistral Instruct**
    - Or try: **ChatML**

For other models, select the appropriate template (e.g., `Llama 3` for Llama models, or `ChatML` as a universal option).

### Mistake 4: Hallucinations

**Symptom:** Model invents facts. For example, says "use flag `--force`" for a command that doesn't support it.

**Cause:** The model strives to generate *plausible* text, not necessarily *true* text. It doesn't know real facts about your system.

**Solution:**
```go
// GOOD: Forbid inventing facts
systemPrompt := `You are a DevOps assistant.
CRITICAL: Never invent facts. Always use tools to get real data.
If you don't know something, say "I don't know" or use a tool.`

// Also use:
// 1. Tools to get real data
// 2. RAG for access to documentation
```

## Completion Criteria / Checklist

**Completed:**
- [x] Understand that LLM predicts tokens, not "thinks"
- [x] Know how to set `Temperature = 0` for deterministic behavior
- [x] Understand context window limitations
- [x] Know how to manage conversation history (summarization or trimming)
- [x] Model supports Function Calling (verified via Lab 00)
- [x] System Prompt forbids hallucinations

**Not completed:**
- [ ] Model behaves non-deterministically (`Temperature > 0`)
- [ ] Agent "forgets" conversation start (context overflow)
- [ ] Model invents facts (no grounding via Tools/RAG)

## Mini-Exercises

### Exercise 1: Token Counting

Write a function that approximately counts the number of tokens in text:

```go
func estimateTokens(text string) int {
    // Approximate estimate: 1 token ≈ 4 characters (for English)
    // For Russian: 1 token ≈ 3 characters
    return len(text) / 4
}
```

**Expected result:**
- Function returns approximate number of tokens
- Accounts for difference between English and Russian text

### Exercise 2: History Trimming

Implement a function to trim message history:

```go
func trimHistory(messages []ChatCompletionMessage, maxTokens int) []ChatCompletionMessage {
    // Keep System Prompt + last messages that fit in maxTokens
    // ...
}
```

**Expected result:**
- System Prompt always remains first
- Last messages are added until they exceed maxTokens
- Function returns trimmed history

## For the Curious

> This section explains the formalization of LLM operation at a deeper level. Can be skipped if you're only interested in practice.

### Formal Definition of LLM

An LLM is a function `NextToken(Context) -> Distribution`:

$$P(x_{t+1} | x_1, ..., x_t)$$

Where:
- $x_1, ..., x_t$ — sequence of tokens (context)
- $P(x_{t+1})$ — probability distribution for the next token
- Model selects token based on this distribution

**Temperature** changes the entropy of the distribution:
- `Temperature = 0`: most probable token is selected (greedy decoding)
- `Temperature > 0`: random token is selected from distribution (sampling)

### Why Does the Model "Choose" a Tool?

When the model receives in context:
- System Prompt: "Use tools when needed"
- Tools Schema: `[{name: "check_status", description: "..."}]`
- User Input: "Check server status"

The model generates a sequence of tokens that matches the tool call format. This isn't "magic" — it's the result of training on function call examples.

## Connection to Other Chapters

- **Function Calling:** More about how the model generates tool calls, see [Chapter 03: Tools](../03-tools-and-function-calling/README.md)
- **Context Window:** How to manage message history, see [Chapter 09: Agent Anatomy](../09-agent-architecture/README.md#context-optimization)
- **Temperature:** Why `Temperature = 0` is used for agents, see [Chapter 03: Tools](../03-tools-and-function-calling/README.md)

## What's Next?

After studying LLM physics, proceed to:
- **[02. Prompting as Programming](../02-prompt-engineering/README.md)** — how to control model behavior through prompts

