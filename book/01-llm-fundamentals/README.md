# 01. LLM Physics — How the Agent's "Brain" Works

## Why Is This Needed?

To control an agent, you need to understand how its "brain" works. Without understanding LLM physics, you won't be able to:
- Properly configure the model for the agent
- Understand why the agent behaves non-deterministically
- Manage context and conversation history
- Avoid hallucinations and errors

This chapter explains the basics of how LLMs work in simple terms, without excessive mathematics.

### Real-World Case Study

**Situation:** You've created a DevOps agent. The user writes: "Check the status of server web-01"

**Problem:** The agent sometimes responds with text "Server is working", and sometimes calls the `check_status` tool. Behavior is unpredictable.

**Solution:** Understanding the probabilistic nature of LLMs and setting `Temperature = 0` makes behavior deterministic. Understanding the context window helps manage conversation history.

## Theory in Simple Terms

### Probabilistic Nature

**Key Fact:** LLM doesn't think, it predicts.

LLM is a function `NextToken(Context) -> Distribution`.  
A sequence of tokens $x_1, ..., x_t$ is fed as input. The model computes a probability distribution for the next token:

$$P(x_{t+1} | x_1, ..., x_t)$$

**What does this mean in practice?**

#### Example 1: DevOps — Magic vs Reality

**❌ Magic (as usually explained):**
> Prompt: `"Check server status"`  
> Model processes context and predicts: "I will call the `check_status` tool" (probability 0.85)

**✅ Reality (how it actually works):**

**1. What is sent to the model:**

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
    Model:    openai.GPT3Dot5Turbo,
    Messages: messages,
    Tools:    tools,  // Key point: the model receives tool descriptions!
}
```

**2. What the model returns:**

The model **does not return text** "I will call the tool". It returns a **structured tool call**:

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
- ✅ `check_status` — description contains "server status" → **selects this**
- ❌ `read_logs` — description is about logs, not status
- ❌ `restart_service` — description is about restart, not checking

**Example with a different request:**

```go
userInput := "Show the latest errors in nginx logs"

// Model sees the same 3 tools
// Matches:
// - check_status: about status, not logs → doesn't fit
// - read_logs: "Use this when user asks about logs" → ✅ SELECTS THIS
// - restart_service: about restart → doesn't fit

// Model returns:
// tool_calls: [{function: {name: "read_logs", arguments: "{\"service\": \"nginx\", \"lines\": 50}"}}]
```

**Key point:** The model selects a tool based on **semantic matching** between the user's request and the tool's `Description`. The more accurate the `Description`, the better the selection.

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

**❌ Magic:**
> Prompt: `"User reports error 500"`  
> Model predicts: "First I'll gather context via `get_ticket_details`" (probability 0.9)

**✅ Reality:**

**What is sent:**

```go
systemPrompt := `You are a Customer Support agent.
When user reports an error, first gather context using get_ticket_details.
When user asks about account, use check_account_status.
When you find a solution, use draft_reply to create a response.`

tools := []openai.Tool{
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name:        "get_ticket_details",
            Description: "Get ticket details including user info, error logs, and history. Use this FIRST when user reports an error or problem.",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "ticket_id": {"type": "string"}
                },
                "required": ["ticket_id"]
            }`),
        },
    },
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name:        "check_account_status",
            Description: "Check if a user account is active, locked, or suspended. Use this when user asks about account status or login issues.",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "user_id": {"type": "string"}
                },
                "required": ["user_id"]
            }`),
        },
    },
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name:        "search_kb",
            Description: "Search knowledge base for solutions to common problems. Use this after gathering ticket details to find similar cases.",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "query": {"type": "string"}
                },
                "required": ["query"]
            }`),
        },
    },
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name:        "draft_reply",
            Description: "Draft a reply message to the ticket. Use this when you have a solution or need to ask user for more information.",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "ticket_id": {"type": "string"},
                    "message": {"type": "string"}
                },
                "required": ["ticket_id", "message"]
            }`),
        },
    },
}

messages := []openai.ChatCompletionMessage{
    {Role: "system", Content: systemPrompt},
    {Role: "user", Content: "User reports error 500"},
}
```

**What the model returns:**

```json
{
  "role": "assistant",
  "tool_calls": [
    {
      "id": "call_xyz789",
      "function": {
        "name": "get_ticket_details",
        "arguments": "{\"ticket_id\": \"TICKET-12345\"}"
      }
    }
  ]
}
```

**How did the model choose `get_ticket_details`?**

The model saw **4 tools**:
- `get_ticket_details`: "Use this FIRST when user reports an error" ✅
- `check_account_status`: "Use this when user asks about account status" ❌
- `search_kb`: "Use this after gathering ticket details" ❌ (too early)
- `draft_reply`: "Use this when you have a solution" ❌ (no solution yet)

Request: "User reports error 500"

The model matched:
- ✅ `get_ticket_details` — description says "FIRST when user reports an error" → **selects this**
- Others don't fit the context

**Example of sequential tool selection:**

```go
// Iteration 1: User reports an error
userInput := "User reports error 500"
// Model selects: get_ticket_details (gathers context)

// Iteration 2: After receiving ticket details
// Model receives in context: "Error 500, user_id: 12345"
// Model selects: search_kb("error 500") (searches for solution)

// Iteration 3: After searching KB
// Model sees solution in context
// Model selects: draft_reply(ticket_id, solution) (creates response)
```

**Key point:** The model selects tools sequentially, based on:
1. **Current user request**
2. **Results of previous tools** (in context)
3. **Tool descriptions** (`Description`)

**Runtime:**
- Parses `ticket_id` from JSON
- Calls real function `getTicketDetails("TICKET-12345")`
- Returns result to model as a message with role `tool`
- Model receives result and continues work

#### Example 3: Data Analytics — Magic vs Reality

**❌ Magic:**
> Prompt: `"Show sales for the last month"`  
> Model predicts: "I'll formulate SQL query via `sql_select`" (probability 0.95)

**✅ Reality:**

**What is sent:**

```go
systemPrompt := `You are a Data Analyst.
When user asks for data, first check table schema using describe_table.
Then formulate SQL query and use sql_select tool.
If data quality is questionable, use check_data_quality.`

tools := []openai.Tool{
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name:        "describe_table",
            Description: "Get table schema including column names, types, and constraints. Use this FIRST when user asks about data structure or before writing SQL queries.",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "table_name": {"type": "string", "description": "Name of the table"}
                },
                "required": ["table_name"]
            }`),
        },
    },
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name:        "sql_select",
            Description: "Execute a SELECT query on the database. ONLY SELECT queries allowed. Use this when user asks for specific data or reports.",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "query": {"type": "string", "description": "SQL SELECT query"}
                },
                "required": ["query"]
            }`),
        },
    },
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name:        "check_data_quality",
            Description: "Check for data quality issues: nulls, duplicates, outliers. Use this when user asks about data quality or before analysis.",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "table_name": {"type": "string"}
                },
                "required": ["table_name"]
            }`),
        },
    },
}
```

**What the model returns:**

```json
{
  "role": "assistant",
  "tool_calls": [
    {
      "function": {
        "name": "sql_select",
        "arguments": "{\"query\": \"SELECT region, SUM(amount) FROM sales WHERE date >= NOW() - INTERVAL '1 month' GROUP BY region\"}"
      }
    }
  ]
}
```

**How did the model choose `sql_select`?**

The model saw **3 tools**:
- `describe_table`: "Use this FIRST when user asks about data structure" ❌ (user is not asking about structure)
- `sql_select`: "Use this when user asks for specific data or reports" ✅
- `check_data_quality`: "Use this when user asks about data quality" ❌ (not about quality)

Request: "Show sales for the last month"

The model matched:
- ✅ `sql_select` — description says "when user asks for specific data" → **selects this**
- Others don't fit

**Example with a different request:**

```go
userInput := "What fields are in the sales table?"

// Model sees the same 3 tools
// Matches:
// - describe_table: "Use this FIRST when user asks about data structure" → ✅ SELECTS THIS
// - sql_select: about executing queries → doesn't fit
// - check_data_quality: about data quality → doesn't fit

// Model returns:
// tool_calls: [{function: {name: "describe_table", arguments: "{\"table_name\": \"sales\"}"}}]
```

**Example of sequential selection:**

```go
// Iteration 1: User asks about sales
userInput := "Why did sales drop in region X?"
// Model selects: describe_table("sales") (need to understand structure first)

// Iteration 2: After receiving table schema
// Model receives in context: "columns: date, region, amount"
// Model selects: sql_select("SELECT region, SUM(amount) FROM sales WHERE region='X' GROUP BY date")

// Iteration 3: After receiving data
// Model analyzes results and may select: check_data_quality("sales")
// if data quality needs to be checked before output
```

**Key point:** The model selects tools based on:
1. **Semantic matching** of request and `Description`
2. **Sequence** (schema first, then query)
3. **Context** of previous results

**Runtime:**
- Validates that it's a SELECT (not DELETE/DROP!)
- Executes SQL through a secure connection (read-only)
- Returns results to model
- Model formats results for user

### Why Is This Important for Engineers?

#### 1. Non-Determinism

Running the agent twice with the same prompt, you may get different actions.

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

**Token** is a unit of text that the model processes.
- One token ≈ 0.75 words (in English)
- In Russian: one word ≈ 1.5 tokens

**Example:**
```
Text: "Check server status"
Tokens: ["Check", " server", " status"]  // ~3 tokens
```

### Context Window

**Context window** is the model's "working memory".

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

If history overflows, the agent "forgets" the beginning of the conversation. In practice, this happens either because your runtime trims/summarizes old messages to fit the limit (the model doesn't receive them), or because the API rejects the request with a context-length error if you don't handle overflow.

**Model is Stateless:** It doesn't remember your previous request if you don't pass it again in `messages`.

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

**Temperature** is a parameter of probability distribution entropy.

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
   - ✅ Good: Models fine-tuned on function calling (e.g., `Hermes-2-Pro`, `Llama-3-Instruct`, `Mistral-7B-Instruct` at time of writing)
   - ❌ Bad: Base models without fine-tuning on tools
   
   > **Note:** Specific models may change. It's important to verify function calling support through capability benchmark (see [Appendix: Capability Benchmark](../appendix/README.md#capability-benchmark-characterization)).

2. **Context Size:** Complex tasks require large context.
   - Minimum: 4k tokens
   - Recommended: 8k+

3. **Instruction Following Quality:** The model must strictly follow System Prompt.
   - Verified through capability benchmark (see [Appendix: Capability Benchmark](../appendix/README.md#capability-benchmark-characterization))

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

**Cause:** Conversation history exceeds the model's context window size. Old messages are "pushed out" of context.

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

- ✅ **Preserves important information:** User name, task context, decisions made
- ✅ **Saves tokens:** Compresses 2000 tokens to 200, preserving essence
- ✅ **Agent remembers start:** Can answer questions about early messages

**Example:**
```
Original history (2000 tokens):
- User: "My name is Ivan, I'm a DevOps engineer"
- Assistant: "Hello, Ivan!"
- User: "We have a server problem"
- Assistant: "Describe the problem"
... (50 more messages)

After trimming: We lose name and context ❌
After summarization: "User Ivan, DevOps engineer. Discussed server problem. Current task: diagnostics." ✅
```

**When to use:**
- **Trimming:** Quick one-time tasks, history not important
- **Summarization:** Long sessions, contextual information important, autonomous agents

See more: section "Context Optimization" in [Chapter 09: Agent Anatomy](../09-agent-architecture/README.md#context-optimization) and [Lab 09: Context Optimization](../../labs/lab09-context-optimization/README.md)

### Mistake 3: Hallucinations

**Symptom:** Model invents facts. For example, says "use flag `--force`" for a command that doesn't support it.

**Cause:** Model strives to generate *plausible* text, not *true*. It doesn't know real facts about your system.

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

✅ **Completed:**
- Understand that LLM predicts tokens, not "thinks"
- Know how to set `Temperature = 0` for deterministic behavior
- Understand context window limitations
- Know how to manage conversation history (summarization or trimming)
- Model supports Function Calling (verified via Lab 00)
- System Prompt forbids hallucinations

❌ **Not completed:**
- Model behaves non-deterministically (`Temperature > 0`)
- Agent "forgets" conversation start (context overflow)
- Model invents facts (no grounding via Tools/RAG)

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

LLM is a function `NextToken(Context) -> Distribution`:

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

The model generates a sequence of tokens that matches the tool call format. This is not "magic" — it's the result of training on function call examples.

## Connection to Other Chapters

- **Function Calling:** More about how the model generates tool calls, see [Chapter 03: Tools](../03-tools-and-function-calling/README.md)
- **Context Window:** How to manage message history, see [Chapter 09: Agent Anatomy](../09-agent-architecture/README.md#context-optimization)
- **Temperature:** Why `Temperature = 0` is used for agents, see [Chapter 03: Tools](../03-tools-and-function-calling/README.md)

## What's Next?

After studying LLM physics, proceed to:
- **[02. Prompting as Programming](../02-prompt-engineering/README.md)** — how to control model behavior through prompts

---

**Navigation:** [← Preface](../00-preface/README.md) | [Table of Contents](../README.md) | [Prompting →](../02-prompt-engineering/README.md)
