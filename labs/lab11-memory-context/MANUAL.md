# Manual: Lab 11 — Memory and Context Management

## Why this matters

In this lab you'll implement a **minimum viable** memory system for an agent: one `condense` when the model window fills up, and three tools for long-term notes. No `LayeredContext`, no automatic fact extraction, no "importance" scoring.

### Real-world case

**Situation:** an assistant agent works with a user in long sessions.

**Without memory:**

- User: "My name is Ivan."
- 50 messages later: "What's my name?" → "I don't know" (we hit the window, history was truncated).

**With memory (done right):**

- User: "Remember that my name is Ivan and I'm responsible for the prod cluster."
- The agent decides on its own: "this is a stable fact" → calls `memory.save("user.name", "Ivan, responsible for the prod cluster")`.
- 50 messages later / in a new session: "Who am I on this project?" → the agent calls `memory.recall("who is the user")` → answers.

**Difference from the "trendy" scheme:** it's not an agent-server automatically pushing the whole dialogue through an extraction-LLM — the agent itself decides what to write. Less noise, a clear log, cheaper.

## Theory in simple terms

### Two memory horizons

```text
                    one Run                       many Runs
              ┌────────────────────────┐    ┌──────────────────┐
   In-Run     │  []Message (linear)    │    │   forgotten      │
              │  + one condense        │    │   on exit        │
              └────────────────────────┘    └──────────────────┘

   Long-term  ┌────────────────────────────────────────────────┐
              │   Store: save / recall / delete (tool calls)   │
              │   lives in a file / DB; agent writes itself    │
              └────────────────────────────────────────────────┘
```

| Horizon | Lifetime | Who writes | Where |
|---|---|---|---|
| In-Run | one run | runtime | `[]openai.ChatCompletionMessage` |
| Long-term | survives restart | agent via tool | JSON / SQLite |

### One threshold, one reaction

The window-management loop is the simplest possible:

1. After every provider response, remember `usage.PromptTokens`.
2. Before the next request, check: `lastTokens > contextMax * 0.80`?
3. Yes → one `condense` (if not already done in this Run).
4. If the provider still returns `ContextOverflowError` → reactive `condense` + retry exactly once. Another overflow → bubble the error up; decompose the task.

No 4-level strategies, no budget trackers, no message scoring. See [Ch. 13: Budget](../../book/13-context-engineering/README.md).

### Why not truncate

A naive `messages = messages[len(messages)-N:]` breaks `tool_call ↔ tool_result` pairs — the provider returns 400. On top of that, you lose context with no trace.

Condense:

1. Preserves the system prompt **byte-for-byte** (important for prompt cache).
2. Replaces the middle with a single `user` message: "Context of previous work: …".
3. The tail (last N messages) stays intact, with pair checking.

### Long-term memory as a tool, not a "layer"

Courses often teach "Final context = System + Facts layer + Summary layer + Working memory". This is a bad idea:

- Every change to "Facts layer" invalidates prompt cache → you pay for a full re-encode on every step.
- Content in the `system` role gets mixed with the rules for the model — the model starts to confuse what's an instruction and what's data.
- Auto-extracting facts from every message = noise like "user said thanks".

The right way: long-term memory is just **tools** in the catalog (like `read_file` or `bash`) that the agent calls deliberately. In the system prompt we say only: "these tools exist, use them for stable facts". Memory contents enter the context only when the agent itself called `memory.recall(...)`.

## Step-by-step

### Step 1: Run skeleton + token tracking

```go
type Run struct {
    messages     []openai.ChatCompletionMessage
    lastTokens   int
    contextMax   int
    condenseDone bool
    client       *openai.Client
    model        string
    tools        []openai.Tool
}

func (r *Run) Step(ctx context.Context, userInput string) (string, error) {
    r.messages = append(r.messages, openai.ChatCompletionMessage{
        Role: openai.ChatMessageRoleUser, Content: userInput,
    })

    if err := r.beforeRequest(ctx); err != nil {
        return "", err
    }

    resp, err := r.callLLM(ctx)
    if err != nil {
        if isContextOverflow(err) {
            if cerr := r.condense(ctx); cerr != nil {
                return "", cerr
            }
            resp, err = r.callLLM(ctx)
            if err != nil {
                return "", err
            }
        } else {
            return "", err
        }
    }

    r.lastTokens = resp.Usage.PromptTokens
    return r.handleResponse(ctx, resp)
}

func (r *Run) beforeRequest(ctx context.Context) error {
    if r.lastTokens > 0 && float64(r.lastTokens) > float64(r.contextMax)*0.80 {
        return r.condense(ctx)
    }
    return nil
}
```

### Step 2: condense

```go
func (r *Run) condense(ctx context.Context) error {
    if r.condenseDone || len(r.messages) < 6 {
        return nil
    }

    system := r.messages[0]
    tail := safeTail(r.messages, 4)
    head := r.messages[1 : len(r.messages)-len(tail)]

    summary, err := r.summarize(ctx, head)
    if err != nil {
        return err
    }

    next := make([]openai.ChatCompletionMessage, 0, 2+len(tail))
    next = append(next, system)
    next = append(next, openai.ChatCompletionMessage{
        Role:    openai.ChatMessageRoleUser,
        Content: "Context of previous work:\n\n" + summary,
    })
    next = append(next, tail...)

    r.messages = next
    r.condenseDone = true
    return nil
}

// safeTail returns >=N trailing messages, expanding the boundary to the left
// if the first element is a tool result without its tool_call in the tail.
func safeTail(msgs []openai.ChatCompletionMessage, n int) []openai.ChatCompletionMessage {
    if n > len(msgs)-1 {
        n = len(msgs) - 1
    }
    start := len(msgs) - n
    for start > 1 && msgs[start].Role == openai.ChatMessageRoleTool {
        start--
    }
    return msgs[start:]
}
```

Prompt for `summarize`:

```text
You are compressing the agent's working transcript into a brief handoff for the next step.
Preserve:
1. The user's original task.
2. Decisions already made and the reasoning behind them.
3. Which files / resources have been read and what's relevant in them.
4. What still needs to be done.
Drop pleasantries and chatter.
```

### Step 3: long-term memory as Store + tools

Storage — a simple JSON file:

```go
type Entry struct {
    Key       string    `json:"key"`
    Value     string    `json:"value"`
    CreatedAt time.Time `json:"created_at"`
}

type FileStore struct {
    mu      sync.Mutex
    path    string
    entries []Entry
}

func (s *FileStore) Save(_ context.Context, key, value string) error {
    s.mu.Lock(); defer s.mu.Unlock()
    for i, e := range s.entries {
        if e.Key == key {
            s.entries[i].Value = value
            s.entries[i].CreatedAt = time.Now()
            return s.flush()
        }
    }
    s.entries = append(s.entries, Entry{key, value, time.Now()})
    return s.flush()
}

func (s *FileStore) Recall(_ context.Context, query string) ([]Entry, error) {
    s.mu.Lock(); defer s.mu.Unlock()
    q := strings.ToLower(query)
    var hits []Entry
    for _, e := range s.entries {
        if q == "" || strings.Contains(strings.ToLower(e.Key+" "+e.Value), q) {
            hits = append(hits, e)
        }
    }
    if len(hits) > 5 {
        hits = hits[:5]
    }
    return hits, nil
}

func (s *FileStore) Delete(_ context.Context, key string) error {
    s.mu.Lock(); defer s.mu.Unlock()
    out := s.entries[:0]
    for _, e := range s.entries {
        if e.Key != key {
            out = append(out, e)
        }
    }
    s.entries = out
    return s.flush()
}
```

Tool registration:

```go
tools := []openai.Tool{
    {Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{
        Name: "memory_save",
        Description: "Save a long-term note. Use for stable facts about the user or project.",
        Parameters: jsonSchema(`{"type":"object","properties":{
            "key":{"type":"string"},"value":{"type":"string"}
        },"required":["key","value"]}`),
    }},
    {Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{
        Name: "memory_recall",
        Description: "Search long-term notes by query (substring).",
        Parameters: jsonSchema(`{"type":"object","properties":{
            "query":{"type":"string"}
        },"required":["query"]}`),
    }},
    {Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{
        Name: "memory_delete",
        Description: "Delete a note by key.",
        Parameters: jsonSchema(`{"type":"object","properties":{
            "key":{"type":"string"}
        },"required":["key"]}`),
    }},
}
```

System prompt — once and only about role/rules:

```text
You are an assistant.
You have memory_save / memory_recall / memory_delete tools that persist between sessions.
Use them for stable facts about the user and project. Don't store transient statuses.
```

### Step 4: tool dispatching loop

```go
for {
    resp, err := r.callLLM(ctx)
    if err != nil { return "", err }
    r.lastTokens = resp.Usage.PromptTokens

    msg := resp.Choices[0].Message
    r.messages = append(r.messages, msg)

    if len(msg.ToolCalls) == 0 {
        return msg.Content, nil
    }

    for _, tc := range msg.ToolCalls {
        result := dispatchTool(ctx, store, tc)
        r.messages = append(r.messages, openai.ChatCompletionMessage{
            Role:       openai.ChatMessageRoleTool,
            ToolCallID: tc.ID,
            Content:    result,
        })
    }
}
```

## Common errors

### Error 1: a dynamic system prompt with the memory list

**Symptom:** expensive and slow — every request recomputes the prefix.

**Cause:** the long-term memory contents are stitched into `messages[0]` on every request.

**Solution:** the agent uses memory through `memory.recall`; the system prompt only mentions that the tools exist.

### Error 2: truncate without checking tool pairs

**Symptom:** the provider returns 400 with a message about `tool_calls without matching tool messages`.

**Cause:** you cut `messages` exactly between an `assistant`-tool_call and a `tool` result.

**Solution:** `condense` or `safeTail` that expands the boundary to the left until a complete pair.

### Error 3: tokens counted via `len(content)/3`

**Symptom:** condense fires either too early or too late — the real cost doesn't match the prediction.

**Cause:** `usage.PromptTokens` from the provider isn't being used.

**Solution:** record `r.lastTokens = resp.Usage.PromptTokens` after every response. Your own estimates are good only for a pre-send check.

### Error 4: condense fires every step

**Symptom:** the agent slows down, the history is constantly "shrinking", fresh context is lost.

**Cause:** no "one condense per Run" limit.

**Solution:** a `condenseDone bool` flag. If you overflow again after the first condense — that's a signal the task isn't decomposable, and you should bubble the error up.

### Error 5: auto-extracting "facts" from every message

**Symptom:** memory fills up with junk like `user_said_hi=true`.

**Cause:** every step runs a separate "extract facts" LLM pass.

**Solution:** drop it entirely. The agent decides what to record, through `memory_save`. Cheaper and easier to read in the logs.

## Completion criteria

✅ **Done:**

- `messages[0]` (system) is stable for the whole Run — verified by byte-for-byte comparison.
- `condense` fires on `usage.PromptTokens > contextMax * 0.80` or reactively on overflow.
- In one Run — at most one `condense` (plus exactly one reactive retry on overflow).
- After `condense`, `tool_call ↔ tool_result` pairs are intact.
- Long-term memory works through 3 tools and survives process restart.
- The system prompt contains no long-term memory contents.

❌ **Not done:**

- `LayeredContext` (Facts/Summary/Working layers stitched into one prompt).
- Auto-extraction of facts from every message.
- Truncate without checking tool pairs.
- Multiple condense calls in a row with no limit.
- Window accounting via `len(content)/3` instead of `usage.PromptTokens`.
