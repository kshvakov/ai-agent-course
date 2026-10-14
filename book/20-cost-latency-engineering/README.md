# 20. Cost & Latency Engineering

## Why does this matter?

The agent works, but the LLM API bill is growing uncontrollably. One request costs $5, another $0.10. Why? Without cost control and latency optimization you cannot:
- Predict the monthly budget
- Optimize expensive requests
- Guarantee response time to the user

Cost & Latency Engineering is budget and performance control. Without it, it's easy to spend thousands of dollars on simple requests or ship a slow agent nobody will use.

### A real-world case

**Situation:** A DevOps agent has been running in production for a week. The LLM API bill is $5000 per month instead of the expected $500.

**Problem:** The agent uses the most expensive model for all requests, even simple status checks. There are no token limits, the system prefix changes between iterations (so the provider cache never hits), and there is no iteration limit. A single "check the server status" request consumes 50,000 tokens because of the large context.

**Solution:** Token budgets, iteration limits, an immutable system prefix for the provider's prompt cache, and a cheap model on auxiliary nodes outside the loop. Auxiliary requests now go to a lightweight model (GLM-5.3 Flash), while the main loop runs on a single model (GLM-5.3 or Fable 5.1). The bill dropped to $600 per month.

> **Note:** LLM API prices change frequently. Check the current rates on the providers' websites and keep them in your config. This chapter contains no specific prices — only formulas into which you plug your actual rates.

## The theory in plain words

### What is cost engineering?

Cost Engineering is the control and optimization of LLM API usage costs. The main levers:
1. **Model choice** — powerful models (Fable 5.1, Opus 5.5) are many times more expensive than lightweight ones (GLM-5.3 Flash); the lightweight one is for auxiliary nodes outside the loop
2. **Token count** — the larger the context, the more expensive; the `tools[]` block is billed on every request
3. **The provider's prompt cache** — an immutable system prefix (system + the tools block) is cached by the provider and billed at a fraction of the price
4. **Request count** — every LLM call costs money

### What is latency engineering?

Latency Engineering is the control of response time. The main factors:
1. **The model** — powerful models are slower than lightweight ones
2. **Context size** — more tokens = more processing time
3. **Iteration count** — more ReAct loops = more time
4. **Timeouts** — protection against hangs

## How it works (step by step)

### Step 1: Token budgets

Set a limit on paid tokens per run (cumulative, with the cache discount accounted for — see below):

```go
const (
    MaxTokensPerTurn = 10000
    MaxIterations    = 10
)

type TokenBudget struct {
    Used  int
    Limit int
}

func checkTokenBudget(budget TokenBudget) error {
    if budget.Used > budget.Limit {
        return fmt.Errorf("token budget exceeded: %d > %d", budget.Used, budget.Limit)
    }
    return nil
}
```

### Step 2: Iteration limits

Cap the number of ReAct loop iterations — the canon of the loop and limits is in [chapter 04](../04-autonomy-and-loops/README.md); the [`lab04-autonomy`](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab04-autonomy) lab is revised to the same neutral client. The examples use the neutral client from chapter 04: `client.Complete(ctx, Request{...})`:

```go
func runAgent(ctx context.Context, client Client, userInput string) (string, error) {
    maxIterations := 10
    tokenBudget := TokenBudget{Limit: MaxTokensPerTurn}

    messages := []Message{
        {Role: "system", Content: systemPrompt}, // immutable prefix — cache-friendly
        {Role: "user", Content: userInput},
    }
    toolDefs := []ToolDef{/* run tools (ch. 03) */}

    for i := 0; i < maxIterations; i++ {
        resp, err := client.Complete(ctx, Request{Messages: messages, Tools: toolDefs})
        if err != nil {
            return "", err // transient errors are retried by the client itself (ch. 04)
        }
        
        // The budget comes from the provider's Usage, in paid tokens, not TotalTokens:
        // the cache prefix is cheaper than generation and is subtracted from prompt (ch. 04, 13)
        tokenBudget.Used += resp.Usage.PromptTokens - resp.Usage.CacheReadTokens + resp.Usage.CompletionTokens
        
        // Check the budget
        if err := checkTokenBudget(tokenBudget); err != nil {
            return "", fmt.Errorf("stopping: %v", err)
        }
        
        // ... the rest of the code ...
    }
    
    return "", fmt.Errorf("max iterations (%d) exceeded", maxIterations)
}
```

### Step 2.5: Artifacts for large tool results

One of the fastest ways to reduce cost/latency is to stop stuffing "fat" tool results straight into `messages[]`.

The pattern:

- store the raw tool result in external storage as an **artifact**,
- add only a short excerpt to the dialog (e.g., the top 20 lines) + `artifact_id`,
- if needed, fetch the artifact in chunks on demand (`range`/`offset`/`limit`).

This is often cheaper and more reliable than summarization: you don't lose data, you just move it out of the context into storage.

### Step 3: Prompt cache — the main savings lever

The classic idea of "caching LLM answers" — `map[hash(messages)] → answer` — **does not work** for agents. The key is almost always unique: on every iteration the history grows (the model's reply was added, tool results were added), so the hash changes, so there are no cache hits. We reached exactly the same conclusion for the context cache in [chapter 13](../13-context-engineering/README.md).

The real lever is the **provider's prompt cache**. The provider caches the **immutable prefix** of the request: the system message + the `tools[]` block. The rules:

- **The prefix does not change** between iterations and requests: the same system prompt, the same list of tools. Everything dynamic goes at the end of the history (`user`, `tool` messages).
- **The history is append-only**: request N+1 starts with the same prefix + the entire history of request N. The provider does not recompute the matched prefix.
- **A hit is visible in the metrics**: `cache-read > 0` in the response's `Usage` (field names differ across providers — `cache_read` / `cached_tokens` / `CacheRead` — but the meaning is the same).
- **The cache is tied to the model**: switching models between requests breaks the cache — each model has its own.

```go
// A prompt cache hit is visible right in the response's Usage:
resp, err := client.Complete(ctx, Request{Messages: messages, Tools: toolDefs})
if err != nil {
    return "", err
}
// resp.Usage.PromptTokens      — all prompt tokens (including cached ones)
// resp.Usage.CacheReadTokens   — how many of them came from the provider's cache
hitRate := float64(resp.Usage.CacheReadTokens) / float64(resp.Usage.PromptTokens)
```

The savings are double:

1. **Cost**: a cache read is billed at a fraction of the full prompt-token price (the coefficients live in the config, see [chapter 19](../19-observability-and-tracing/README.md): the per-model price table).
2. **Latency**: the cached prefix is not recomputed — time to first token drops noticeably. The prompt cache is a **latency** optimization as much as a money one.

What kills the cache outright: a "smart" system prompt with the date or the user's name inside, reordering tools between iterations, and rotating models within a single loop.

### Step 4: Model choice — once, at the boundary

The model is chosen **before the loop starts** and does not change inside it. The criteria are the task class and quality requirements, and the decision is made by code at the system boundary: explicit scenarios in the config, a separate classifier (see "A cheap model on auxiliary nodes" below), or a human.

```go
// selectModel is called once — before the agent loop, not inside it.
// The criteria come from the outside (scenario config / classifier), not from
// heuristics over substrings of the user's input: substring magic is fragile
// and does not survive rewording.
func selectModel(profile TaskProfile) string {
    switch profile {
    case ProfileAuxiliary: // auxiliary nodes outside the loop
        return "glm-5.3-flash"
    default: // the main loop — one model for the whole run
        return "glm-5.3"
    }
}
```

Inside the loop the model is fixed: switching models between iterations breaks the prompt cache (each model has its own) and wipes out all the savings from Step 3.

### Step 5: Fallback models

A fallback chain exists for **fault tolerance**, not for savings: "if the primary model is unavailable — the run continues on the backup instead of failing". Switching happens only on transient errors and never for price reasons. The mechanism — [ch. 03 → Fallbacks instead of routing](../03-tools-and-function-calling/README.md#fallbacks-instead-of-routing) and [ch. 04](../04-autonomy-and-loops/README.md):

```go
// Fallbacks live in the client's configuration (Resolve); the loop knows nothing about them.
cfg := ModelConfig{ // the ch. 03 shape: primary + explicit fallbacks
    Primary:   "glm-5.3",
    Fallbacks: []string{"fable-5.1"}, // availability only, not "savings"
}
// The client itself retries the request against the backup model on a transient error;
// the switch counter lands in RunResult (ch. 04) and in the failover metrics (ch. 19).
```

### Step 6: Timeouts

Set a timeout for the whole agent run and for every tool call:

```go
import "context"
import "time"

func runAgentWithTimeout(ctx context.Context, client Client, userInput string) (string, error) {
    // Timeout for the whole agent run (5 minutes)
    ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
    defer cancel()

    maxIterations := 10 // a terminal run boundary (ch. 04)

    messages := []Message{
        {Role: "system", Content: systemPrompt},
        {Role: "user", Content: userInput},
    }
    toolDefs := []ToolDef{/* run tools (ch. 03) */}

    // ... agent loop ...
    
    for i := 0; i < maxIterations; i++ {
        // Timeout for each LLM call (30 seconds)
        callCtx, callCancel := context.WithTimeout(ctx, 30*time.Second)
        resp, err := client.Complete(callCtx, Request{Messages: messages, Tools: toolDefs})
        callCancel()

        if err != nil {
            if errors.Is(err, context.DeadlineExceeded) {
                return "", fmt.Errorf("LLM call timeout")
            }
            return "", err
        }

        // ... the rest of the code ...
    }

    // Exhausting the iteration limit is a terminal boundary (ch. 04)
    return "", fmt.Errorf("agent exceeded max iterations (%d)", maxIterations)
}
```

## Where to plug this into our code

### Integration point 1: Agent loop

In your runtime from chapter 04, add the budget check and the iteration limit (the revised lab04-autonomy uses the same loop shape):

```go
const MaxIterations = 10
const MaxTokensPerTurn = 10000

// In the loop: the budget is cumulative, for the whole run (ch. 04), not per request
var usedPaidTokens int
for i := 0; i < MaxIterations; i++ {
    resp, err := client.Complete(ctx, Request{Messages: messages, Tools: toolDefs})
    if err != nil {
        return "", err // before reading Usage: resp == nil on error
    }
    // Accumulate paid tokens (the cache prefix is cheaper, ch. 04/13) and check against the run limit
    usedPaidTokens += resp.Usage.PromptTokens - resp.Usage.CacheReadTokens + resp.Usage.CompletionTokens
    if usedPaidTokens > MaxTokensPerTurn {
        return "", fmt.Errorf("token limit exceeded")
    }
    // ... the rest of the code ...
}
```

### Integration point 2: Context optimization

In your runtime, add condense triggered by the context-window threshold (ch. 12/13), not by the run budget (the revised lab09-context-optimization uses the same client):

```go
func adaptiveContextManagement(ctx context.Context, client Client, lastResp *Response, messages []Message, condenseAt float64) []Message {
    // The condense trigger is the context-window threshold (ch. 12/13), not the run budget:
    // the budget (MaxTokensPerTurn) is about money, condenseAt is about space in the window.
    // The threshold is computed from the last provider response's usage.PromptTokens (ch. 12);
    // the "~3 characters per token" pre-estimate is only a fallback while there is no response yet
    var usedTokens int
    if lastResp != nil {
        usedTokens = lastResp.Usage.PromptTokens // primary source — the provider
    } else {
        usedTokens = estimateTokens(messages) // fallback until the first response (ch. 12)
    }
    window := contextWindowTokens() // the model's window size (ch. 01)

    if float64(usedTokens)/float64(window) >= condenseAt { // typically ~0.85
        return compressOldMessages(ctx, client, messages)
    }

    // ... the rest of the logic ...
}
```

## Mini code example

A compact example with cost control built on the loop from chapter 04 — budgets, an iteration limit, timeouts, one model inside the loop (the client is the neutral one from [chapter 04](../04-autonomy-and-loops/README.md); steps 1–2 and 6 above, assembled into a single listing):

```go
const (
    MaxTokensPerTurn    = 10000
    MaxIterations       = 10
    LLMTimeout          = 30 * time.Second
    AgentTimeout        = 5 * time.Minute
)

type TokenBudget struct {
    Used  int
    Limit int
}

func checkTokenBudget(budget TokenBudget) error {
    if budget.Used > budget.Limit {
        return fmt.Errorf("token budget exceeded: %d > %d", budget.Used, budget.Limit)
    }
    return nil
}

// runAgentWithBudgets — the chapter 04 loop with a budget, an iteration limit and timeouts.
func runAgentWithBudgets(ctx context.Context, client Client, userInput string) (string, error) {
    ctx, cancel := context.WithTimeout(ctx, AgentTimeout) // the deadline for the whole run
    defer cancel()

    // One primary model for the entire loop (set on the client, ch. 04) —
    // the prompt cache works; switching happens only through the fallback chain.

    tokenBudget := TokenBudget{Limit: MaxTokensPerTurn}

    messages := []Message{
        {Role: "system", Content: systemPrompt}, // immutable prefix — cache-friendly for the provider
        {Role: "user", Content: userInput},
    }
    toolDefs := []ToolDef{/* run tools (ch. 03) */}

    for i := 0; i < MaxIterations; i++ {
        // Check the budget before the request
        if err := checkTokenBudget(tokenBudget); err != nil {
            log.Printf("budget exceeded: %v", err)
            return "", fmt.Errorf("token budget exceeded: %w", err) // terminal (ch. 04)
        }

        // Timeout for each LLM call
        callCtx, callCancel := context.WithTimeout(ctx, LLMTimeout)
        resp, err := client.Complete(callCtx, Request{Messages: messages, Tools: toolDefs})
        callCancel()

        if err != nil {
            if errors.Is(err, context.DeadlineExceeded) {
                log.Printf("LLM call timeout")
                return "", err
            }
            log.Printf("API error: %v", err)
            return "", err
        }

        msg := resp.Choices[0].Message
        messages = append(messages, msg)

        // Update the budget — in paid tokens, not TotalTokens (ch. 04, 13).
        // Lower-bound simplification: a cache read is not free, it is billed
        // at a discount — exact accounting uses the coefficient from the price config (ch. 19).
        tokenBudget.Used += resp.Usage.PromptTokens - resp.Usage.CacheReadTokens + resp.Usage.CompletionTokens
        log.Printf("tokens: %d/%d (cache_read=%d)",
            tokenBudget.Used, tokenBudget.Limit, resp.Usage.CacheReadTokens)

        if len(msg.ToolCalls) == 0 {
            return msg.Content, nil
        }

        for _, toolCall := range msg.ToolCalls {
            result, err := executeTool(ctx, toolCall)
            if err != nil {
                result = "error: " + err.Error()
            }
            messages = append(messages, Message{
                Role: "tool", Content: result, ToolCallID: toolCall.ID,
            })
        }
    }
    // Exhausting the iteration limit is a terminal boundary (ch. 04), not a silent return
    log.Printf("agent exceeded max iterations (%d), tokens: %d/%d", MaxIterations, tokenBudget.Used, tokenBudget.Limit)
    return "", fmt.Errorf("agent exceeded max iterations (%d)", MaxIterations)
}
```

## A Cheap Model on Auxiliary Nodes

The idea of "choosing the model by complexity at each iteration" looks tempting, but inside the main loop it does not work: **switching models between requests breaks the prompt cache** (each model has its own cache, Step 3), and the "50% savings" from marketing examples accounts for neither the lost cache nor the extra latency of recomputing the prefix. On top of that, a cheap model choosing tools in the middle of a run can steer the reasoning in the wrong direction — and then the "saved" call turns into extra iterations.

The right place for a cheap model is **auxiliary nodes outside the main loop**:

| Node | What it does | Model |
|------|-----------|--------|
| Input classification | simple/complex request, scenario routing | GLM-5.3 Flash |
| Structured output | field extraction, parsing a response into a structure | GLM-5.3 Flash |
| **The main loop** | the ReAct run: choosing tools, analysis, the answer | **one model**: GLM-5.3 or Fable 5.1 |

**Condense is an exception, not an auxiliary node.** The summarization request carries the **same cacheable prefix** (system + tools) and goes to the same model as the main loop: the head is read from the warm cache instead of being paid in full ([ch. 13](../13-context-engineering/README.md)). A cheap model here is false economy: its own prompt and a "trimmed" history wipe out the cache hit across the entire old history, and condense ends up more expensive than a call to the main model.

```go
// condense — per the ch. 12/13 canon: the same model, the same cacheable prefix
// (system + tools), the input is the entire old history (head), at most once per Run.
func condense(ctx context.Context, ep Client, msgs []Message) ([]Message, error) {
    // 1. Split into head (old) and tail (the last 2 user steps)
    // 2. Build the head summary: the request carries the same system + tools prefix
    //    — the head is read from the warm cache (ch. 13)
    // 3. Return [summary as user message] + tail
}
```

The key properties of auxiliary nodes: a fixed output format, a short input (no growing history), and errors handled by falling back to the main model — quality degrades gracefully instead of failing.

**Fallback chains are a separate story:** they exist for fault tolerance (the primary model is unavailable — the run continues on the backup), not for savings. The mechanism — [ch. 03](../03-tools-and-function-calling/README.md) and [ch. 04](../04-autonomy-and-loops/README.md).

> **Connection to other chapters:** Function Calling and the anatomy of tools — [Chapter 03: Tools and Function Calling](../03-tools-and-function-calling/README.md). The agent loop itself — [Chapter 04: Autonomy and Loops](../04-autonomy-and-loops/README.md).

## The Context Budget Is Money

Every agent request pays not only for the history but also for the **entire `tools[]` block** — it is sent on every call. Four tools with detailed descriptions amount to roughly ~700+ tokens in every request; over 10 iterations and thousands of runs per day the bill grows faster than you'd think.

What to do about it:

- **Write short tool descriptions**: the name, what it does, when to call it, constraints. A 500-token "documentation" blurb in the description is a permanent tax.
- **Don't haul the whole catalog around**: give the agent the tools list that fits the task (a narrow ToolRegistry), not "everything there is". The link — [ch. 03: Tools and Function Calling](../03-tools-and-function-calling/README.md).
- **Skills instead of catalogs**: instead of 50 tools with long descriptions — a few meta-tools ("find the right skill", "run the skill") that load instructions on demand. The same prefix, but you pay only for what is actually used.
- **Watch the metric**: the `cache_read / prompt` share in Usage (ch. 19) shows whether the prefix is actually being cached rather than billed anew.

A detailed treatment of context management — [Chapter 13: Context Engineering](../13-context-engineering/README.md).

## Streaming to Reduce Perceived Latency

Latency comes in two kinds:
- **Actual** — the time from the request to the last token of the response
- **Perceived** — the time the user *feels* like waiting

Streaming does not speed up generation. The model spends just as much time on the whole response. But the user sees the first words after 200–500 ms instead of waiting 5–10 seconds for the full response. That fundamentally changes the UX.

### How streaming works

Without streaming: request → 5 seconds of waiting → the whole response at once.

With streaming: request → 200 ms → the first token → tokens arrive one by one → the response is complete.

Technically this is Server-Sent Events (SSE) — the HTTP connection stays open and the server sends data as it becomes ready.

### Streaming in the agent loop

```go
func streamAgentResponse(ctx context.Context, client Client, messages []Message) error {
    stream, err := client.Stream(ctx, Request{Messages: messages}) // the neutral streaming call
    if err != nil {
        return fmt.Errorf("stream error: %w", err)
    }
    defer stream.Close()

    for {
        delta, err := stream.Recv()
        if errors.Is(err, io.EOF) {
            fmt.Println()
            return nil
        }
        if err != nil {
            return fmt.Errorf("stream recv error: %w", err)
        }
        fmt.Print(delta.Text) // Print the chunk immediately, without buffering
    }
}
```

### Streaming via SSE for web clients

If the agent runs as an HTTP server, streaming is delivered to the client via Server-Sent Events: the connection is not closed, the server writes `data: ...` lines and calls `flusher.Flush()` after every chunk. A full treatment of the SSE transport (headers, `[DONE]`, reconnects, WebSocket vs SSE) is in [Chapter 21: Workflow and State Management](../21-workflow-state-management/README.md).

### When streaming doesn't help

Streaming reduces perceived latency only for the final answer to the user. Intermediate LLM calls (tool selection, analyzing results) don't need to be streamed — the user never sees them. For intermediate calls what matters is actual latency. It is reduced by choosing a fast model and optimizing the context.

## Parallel Execution of Tool Calls

The LLM can return several tool calls in one iteration (see [Chapter 03: Tools and Function Calling](../03-tools-and-function-calling/README.md)). If the tools are independent of each other, they can be executed in parallel.

Parallel execution **does not save tokens** — the LLM generated all the tool calls in a single request anyway. But it **shortens wall-clock time** — the time from the start to the end of the iteration.

### When it's safe to parallelize

| Safe | Unsafe |
|-----------|-------------|
| `check_disk` + `check_memory` — read different resources | `create_file` → `write_to_file` — the second depends on the first |
| `get_logs("app-1")` + `get_logs("app-2")` — different servers | `stop_service` → `deploy` → `start_service` — strict order |
| `dns_lookup` + `ping` — independent checks | `read_config` → `apply_config` — read first, then apply |

The rule: if one tool's result is needed as input for another — execute sequentially. If not — in parallel.

### Implementation: executeToolsParallel

```go
// ToolResult holds the result of executing one tool
type ToolResult struct {
    ToolCallID string
    Content    string
    Err        error
}

// executeToolsParallel runs all tool calls concurrently.
// Each tool is executed in its own goroutine.
func executeToolsParallel(ctx context.Context, toolCalls []ToolCall) []Message {
    results := make([]ToolResult, len(toolCalls))

    var wg sync.WaitGroup
    for i, tc := range toolCalls {
        wg.Add(1)
        go func(idx int, call ToolCall) {
            defer wg.Done()

            // A timeout for each tool individually
            toolCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
            defer cancel()

            content, err := executeTool(toolCtx, call)
            results[idx] = ToolResult{
                ToolCallID: call.ID,
                Content:    content,
                Err:        err,
            }
        }(i, tc)
    }
    wg.Wait()

    // Assemble the results into the messages format
    var messages []Message
    for _, r := range results {
        content := r.Content
        if r.Err != nil {
            content = fmt.Sprintf("error: %v", r.Err)
        }
        messages = append(messages, Message{
            Role:       "tool",
            Content:    content,
            ToolCallID: r.ToolCallID,
        })
    }
    return messages
}
```

Integration into the agent loop — replace sequential execution with parallel:

```go
for i := 0; i < MaxIterations; i++ {
    resp, err := client.Complete(ctx, Request{Messages: messages, Tools: toolDefs})
    if err != nil {
        return "", err
    }

    msg := resp.Choices[0].Message
    messages = append(messages, msg)

    if len(msg.ToolCalls) == 0 {
        return msg.Content, nil
    }

    // Before: sequential execution
    // for _, tc := range msg.ToolCalls { ... }

    // After: parallel execution
    toolMessages := executeToolsParallel(ctx, msg.ToolCalls)
    messages = append(messages, toolMessages...)
}
```

### How much time this saves

Suppose the agent calls 3 tools in one iteration. Each tool takes ~2 seconds.

- **Sequentially:** 2 + 2 + 2 = 6 seconds
- **In parallel:** max(2, 2, 2) = 2 seconds

Over 8 iterations with 2–3 tool calls each — that's the difference between 48 seconds and 16 seconds. The user waits three times less.

> **Connection to Chapter 04:** In [Chapter 04: Autonomy and Loops](../04-autonomy-and-loops/README.md) tool calls are executed sequentially. Parallel execution is a natural optimization for when the agent works reliably and you start optimizing latency.

## Common Mistakes

A short summary (detailed breakdowns below):

| # | Mistake | Symptom | Fix |
|---|--------|---------|---------|
| 1 | No token limits | The bill grows uncontrollably, requests of 100k tokens each | TokenBudget based on the provider's Usage |
| 2 | No iteration limit | The agent loops, 50+ iterations | `MaxIterations` — a terminal boundary (ch. 04) |
| 3 | One expensive model for everything | Auxiliary calls cost more than they should | GLM-5.3 Flash on auxiliary nodes outside the loop |
| 4 | Switching models inside the loop | The prompt cache breaks, the prefix is billed anew | One model in the loop (breakdown below) |
| 5 | Your own result-cache instead of the prompt cache | No hits, no savings | An immutable system prefix (Step 3, breakdown below) |
| 6 | No timeouts | The agent hangs for 10+ minutes | A timeout for the run and for every LLM call |

### Breakdown 1: switching models inside the loop

**Symptom:** the `cache_read / prompt` share in Usage is nearly zero even though the system prompt doesn't change; the latency of every call is noticeably higher than the first one.

**Cause:** the model alternates between iterations ("the cheap one picks the tool, the expensive one answers"). Each model has its own prompt cache — on a switch the prefix is not reused and is billed at full rate. The "50% savings" from the stage-routing examples accounts for neither this nor the loss of reasoning quality when the model changes mid-run.

**Fix:** one model for the whole loop; the lightweight model (GLM-5.3 Flash) only on auxiliary nodes outside the loop, where there is almost no history and the main run's cache is not involved.

### Breakdown 2: your own result-cache instead of the prompt cache

**Symptom:** you implemented `map[hash(messages)] → answer`, but the hit rate is near zero and there are no savings.

**Cause:** the cache key is almost always unique — the dialog history grows on every iteration (the model's reply, tool results), the hash changes, hits never happen.

**Fix:** cache not the answers but the **prefix** — keep the system message + the `tools[]` block immutable and only append to the history. Then the provider's cache works: a hit is visible as `CacheReadTokens > 0` in Usage (Step 3). Your own map for answer caching only makes sense for genuinely identical requests with no history (e.g., the embedding cache in RAG, ch. 06) — for the agent loop it's dead weight.

## Exercises

### Exercise 1: Implement the token budget check

Add the budget check to your runtime from chapter 04 (the revised lab04-autonomy uses the same client):

```go
func checkTokenBudget(used int, limit int) error {
    // Your code here
    // Return an error if the limit is exceeded
}
```

**Expected result:**
- The function returns an error if more tokens were used than the limit allows
- The function returns nil if the limit is not exceeded

### Exercise 2: Condense per the ch. 12/13 canon

Implement condense summarization per the canon: the same model, the same cacheable prefix, at most once per Run:

```go
func condense(ctx context.Context, ep Client, msgs []Message) ([]Message, error) {
    // Your code here
    // 1. Split into head/tail  2. Summary: a request with the same prefix
    // system + tools (ch. 13)  3. [summary as user message] + tail
}
```

**Expected result:**
- The summarization request carries the same cacheable prefix — the head is read from the warm cache
- At most 1 condense per Run; if summarization fails, the run continues with the uncondensed history (ch. 13)

### Exercise 3: Prompt cache metrics

Compute the provider cache hit rate from the Usage in the responses:
```go
func cacheHitRate(usages []Usage) float64 {
    // Your code here
    // Sum of CacheReadTokens / sum of PromptTokens across all calls
}
```

**Expected result:**
- With an immutable system prefix, the hit rate is noticeably above zero
- When the model changes between requests, the hit rate drops to zero

## Acceptance Criteria / Checklist

**Done (production-ready):**
- [x] Token budgets based on the provider's Usage (prompt/completion) implemented, with a check before every request
- [x] An iteration limit for the ReAct loop is in place
- [x] An immutable system prefix (system + tools) — the provider's prompt cache works, cache_read metrics are tracked
- [x] A cheap model (GLM-5.3 Flash) on auxiliary nodes outside the loop; one model inside the loop
- [x] A fallback chain configured for fault tolerance, not for savings
- [x] Timeouts set for LLM calls and for the agent run

**Not done:**
- [ ] No token limits
- [ ] No iteration limit
- [ ] The system prefix changes between iterations / the model changes inside the loop
- [ ] The budget is computed from TotalTokens or your own estimates instead of the provider's Usage
- [ ] No timeouts

## Relation to Other Chapters

- **Observability:** Logging token usage — [Chapter 19: Observability and Tracing](../19-observability-and-tracing/README.md)
- **Context Optimization:** Managing context size — [Chapter 13: Context Engineering](../13-context-engineering/README.md)
- **Agent Loop:** The basic agent loop — [Chapter 04: Autonomy and Loops](../04-autonomy-and-loops/README.md)
