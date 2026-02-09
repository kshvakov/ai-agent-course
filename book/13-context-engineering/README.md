# 13. Context Engineering

## Why This Chapter?

Context windows are limited. As conversations grow, you need to decide what stays in context and what gets summarized or dropped. Poor context management wastes tokens, loses important details, and confuses the agent.

This chapter covers practical context management techniques: layers, summarization, fact selection, and adaptive context building.

### Real-World Case Study

**Situation:** Long-running conversation with agent. After 50 turns, context is 50K tokens. New request needs recent information, but it's buried in history.

**Problem:**
- Include full history: Exceeds context limit, expensive
- Include only recent: Loses important context from early
- No strategy: Agent gets confused or misses critical information

**Solution:** Context engineering uses layers (working memory, summaries, facts), selective retrieval, and summarization of old turns while preserving key facts.

## Theory in Simple Terms

### Context Layers

**Working memory (recent turns):**
- The last N conversation turns
- Always included
- Most relevant for the current task

**Summary layer:**
- Summarized old conversations
- Preserves key facts
- Reduces token usage

**Facts layer:**
- Extracted important facts from [long-term memory](../12-agent-memory/README.md)
- User preferences, decisions, constraints
- Persistent between conversations
- **Note:** Storing and retrieving facts is described in [Memory](../12-agent-memory/README.md), here only their use in context is described

> **Important: Context as an Anchor (Anchoring Bias).**
> If **user preferences** ("user thinks X", "we need answer Y") or **unverified hypotheses** enter the facts layer, they become a strong anchor for the model. The model may shift answers toward these preferences, even if actual data points elsewhere.
>
> **Problem:** Preferences and hypotheses included in context as facts can distort objective analysis.
>
> **Solution:** Separate entry types: **Fact** (verified data), **Preference** (user preferences), **Hypothesis** (hypotheses). Include preferences and hypotheses in context only when appropriate (personalization), and exclude them for analytical tasks requiring objectivity.

**Task state:**
- Current task progress
- What's done, what's pending
- Allows resumption

### Context Operations

1. **Select** — Choose what to include
2. **Summarize** — Compress old information
3. **Extract** — Extract key facts
4. **Layer** — Organize by importance/freshness

## How It Works (Step by Step)

### Step 1: Context Manager Interface

```go
type ContextManager interface {
    AddMessage(msg openai.ChatCompletionMessage) error
    GetContext(maxTokens int) ([]openai.ChatCompletionMessage, error)
    Summarize() error
    ExtractFacts() ([]Fact, error)
}

type Fact struct {
    Key        string
    Value      string
    Source     string // Which conversation
    Importance int    // 1-10
    Type       string // "fact", "preference", "hypothesis", "constraint"
}
```

### Step 2: Layered Context

```go
type LayeredContext struct {
    workingMemory []openai.ChatCompletionMessage // Recent turns
    summary       string                          // Summarized history
    facts         []Fact                          // Extracted facts
    maxWorking    int                             // Max turns in working memory
}

func (c *LayeredContext) GetContext(maxTokens int) ([]openai.ChatCompletionMessage, error) {
    var messages []openai.ChatCompletionMessage
    
    // Add system prompt with facts
    if len(c.facts) > 0 {
        factsContext := "Important facts:\n"
        for _, fact := range c.facts {
            factsContext += fmt.Sprintf("- %s: %s\n", fact.Key, fact.Value)
        }
        messages = append(messages, openai.ChatCompletionMessage{
            Role:    "system",
            Content: factsContext,
        })
    }
    
    // Add summary if exists
    if c.summary != "" {
        messages = append(messages, openai.ChatCompletionMessage{
            Role:    "system",
            Content: "Previous conversation summary: " + c.summary,
        })
    }
    
    // Add working memory (recent turns)
    messages = append(messages, c.workingMemory...)
    
    // Truncate if exceeds maxTokens
    return truncateToTokenLimit(messages, maxTokens), nil
}
```

### Step 3: Summarization

```go
func (c *LayeredContext) Summarize(ctx context.Context, client *openai.Client) error {
    if len(c.workingMemory) <= c.maxWorking {
        return nil // Not needed yet
    }
    
    // Get old messages for summarization
    oldMessages := c.workingMemory[:len(c.workingMemory)-c.maxWorking]
    
    // Create summarization prompt
    prompt := "Summarize this conversation, preserving key facts and decisions:\n\n"
    for _, msg := range oldMessages {
        prompt += fmt.Sprintf("%s: %s\n", msg.Role, msg.Content)
    }
    
    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: "gpt-4o-mini",
        Messages: []openai.ChatCompletionMessage{
            {Role: "system", Content: "You are a summarization agent. Extract key facts and decisions."},
            {Role: "user", Content: prompt},
        },
        Temperature: 0,
    })
    if err != nil {
        return err
    }
    
    c.summary = resp.Choices[0].Message.Content
    
    // Keep only recent messages in working memory
    c.workingMemory = c.workingMemory[len(c.workingMemory)-c.maxWorking:]
    
    return nil
}
```

### Step 4: Using Facts from Memory

**IMPORTANT:** Fact extraction and storage happens in [Memory](../12-agent-memory/README.md). Here we only use already extracted facts when assembling context.

```go
func (c *LayeredContext) GetContext(maxTokens int, memory Memory, includePreferences bool) ([]openai.ChatCompletionMessage, error) {
    var messages []openai.ChatCompletionMessage
    
    // Get facts from memory (don't extract here!)
    facts, _ := memory.Retrieve("user_preferences", 10)
    
    // Filter facts by type depending on task
    var filteredFacts []Fact
    for _, fact := range facts {
        if fact.Type == "fact" || fact.Type == "constraint" {
            // Always include verified facts and constraints
            filteredFacts = append(filteredFacts, fact)
        } else if includePreferences && (fact.Type == "preference" || fact.Type == "hypothesis") {
            // Include preferences and hypotheses only when appropriate (personalization)
            filteredFacts = append(filteredFacts, fact)
        }
        // Otherwise exclude preferences/hypotheses for objective analysis
    }
    
    // Add system prompt with facts
    if len(filteredFacts) > 0 {
        factsContext := "Important facts:\n"
        for _, fact := range filteredFacts {
            // Mark type for clarity
            prefix := ""
            if fact.Type == "preference" {
                prefix = "[User preference] "
            } else if fact.Type == "hypothesis" {
                prefix = "[Hypothesis] "
            }
            factsContext += fmt.Sprintf("- %s%s: %v\n", prefix, fact.Key, fact.Value)
        }
        messages = append(messages, openai.ChatCompletionMessage{
            Role:    "system",
            Content: factsContext,
        })
    }
    
    // Add summary if exists
    if c.summary != "" {
        messages = append(messages, openai.ChatCompletionMessage{
            Role:    "system",
            Content: "Previous conversation summary: " + c.summary,
        })
    }
    
    // Add working memory (recent turns)
    messages = append(messages, c.workingMemory...)
    
    // Truncate if exceeds maxTokens
    return truncateToTokenLimit(messages, maxTokens), nil
}
```

## Token Counting and truncateToTokenLimit

In previous examples we called `truncateToTokenLimit` but never implemented it. Let's cover token counting and context truncation.

### Why Count Tokens?

Every model has a hard context window limit. Exceed it — you get an error. Undershoot — you waste money on empty space. Precise token counting lets you use context as efficiently as possible.

### Simple Counting: Words vs Tokens

Precise counting requires the model's tokenizer (e.g., `tiktoken` for OpenAI). For quick estimates, an approximation works: 1 token ≈ 0.75 words for English text, closer to 0.5 words for Russian (Cyrillic encodes less efficiently).

```go
// TokenCounter — token counting interface.
// Swap implementations: approximate for tests, precise for production.
type TokenCounter interface {
    Count(text string) int
}

// WordBasedCounter — approximate word-based counting.
// Good for quick estimates without external dependencies.
type WordBasedCounter struct {
    TokensPerWord float64 // English ≈ 1.33, Russian ≈ 2.0
}

func (c *WordBasedCounter) Count(text string) int {
    words := len(strings.Fields(text))
    return int(float64(words) * c.TokensPerWord)
}

// TiktokenCounter — precise counting via tiktoken.
// Use in production for accurate budgeting.
type TiktokenCounter struct {
    encoding *tiktoken.Encoding
}

func NewTiktokenCounter(model string) (*TiktokenCounter, error) {
    enc, err := tiktoken.EncodingForModel(model)
    if err != nil {
        return nil, fmt.Errorf("encoding for model %s: %w", model, err)
    }
    return &TiktokenCounter{encoding: enc}, nil
}

func (c *TiktokenCounter) Count(text string) int {
    return len(c.encoding.Encode(text, nil, nil))
}
```

### Model Limits

Context limits depend on the model. Keep them in configuration, not in code:

```go
// ModelLimits stores limits for a specific model.
var ModelLimits = map[string]int{
    "gpt-4o":      128_000,
    "gpt-4o-mini": 128_000,
    "gpt-4-turbo": 128_000,
    "gpt-3.5-turbo": 16_385,
    "claude-3-5-sonnet": 200_000,
}

// SafeLimit returns the limit with room for the model's response.
// Leaves space for generation (maxOutputTokens).
func SafeLimit(model string, maxOutputTokens int) int {
    limit, ok := ModelLimits[model]
    if !ok {
        return 4096 // Safe default
    }
    return limit - maxOutputTokens
}
```

### Implementing truncateToTokenLimit

Truncate context from the end, but always keep system messages and the user's last request:

```go
func truncateToTokenLimit(
    messages []openai.ChatCompletionMessage,
    maxTokens int,
    counter TokenCounter,
) []openai.ChatCompletionMessage {
    total := countMessages(messages, counter)
    if total <= maxTokens {
        return messages
    }

    // Split: system messages, middle, last user message
    var system []openai.ChatCompletionMessage
    var middle []openai.ChatCompletionMessage
    var last openai.ChatCompletionMessage

    for i, msg := range messages {
        if msg.Role == "system" {
            system = append(system, msg)
        } else if i == len(messages)-1 {
            last = msg
        } else {
            middle = append(middle, msg)
        }
    }

    // Count fixed parts (system + last request)
    reserved := countMessages(system, counter) + counter.Count(last.Content) + 4 // +4 for metadata

    // Trim middle from the start (remove oldest messages)
    budget := maxTokens - reserved
    var kept []openai.ChatCompletionMessage
    runningTotal := 0

    for i := len(middle) - 1; i >= 0; i-- {
        msgTokens := counter.Count(middle[i].Content) + 4
        if runningTotal+msgTokens > budget {
            break
        }
        runningTotal += msgTokens
        kept = append([]openai.ChatCompletionMessage{middle[i]}, kept...)
    }

    result := append(system, kept...)
    result = append(result, last)
    return result
}

func countMessages(messages []openai.ChatCompletionMessage, counter TokenCounter) int {
    total := 0
    for _, msg := range messages {
        total += counter.Count(msg.Content) + 4 // +4 tokens for role and delimiters
    }
    return total
}
```

**Why +4?** Each message in the API is encoded with metadata: role, start and end delimiters. For OpenAI this is roughly 4 tokens per message.

## Advanced Compression Strategies

Basic LLM summarization is just one way to compress context. Let's look at more precise approaches.

### Semantic Compression

The idea: keep the meaning, drop the filler. Instead of retelling the entire conversation — extract only what affects future decisions.

### Key-Value Extraction

The idea: turn a long narrative into structured key-value pairs. More compact than a summary, easier for the model to use.

### Implementation

```go
// CompressionStrategy defines the compression method.
type CompressionStrategy string

const (
    StrategySummarize CompressionStrategy = "summarize" // Standard summarization
    StrategySemantic  CompressionStrategy = "semantic"   // Semantic compression
    StrategyKeyValue  CompressionStrategy = "keyvalue"   // Key-Value extraction
)

// compressContext compresses messages using the chosen strategy.
func compressContext(
    ctx context.Context,
    client *openai.Client,
    messages []openai.ChatCompletionMessage,
    strategy CompressionStrategy,
) (string, error) {
    conversation := formatMessages(messages)

    prompts := map[CompressionStrategy]string{
        StrategySummarize: "Summarize this conversation. Preserve key facts and decisions:\n\n" + conversation,

        StrategySemantic: `Compress this conversation to the minimum.
Rules:
- Keep ONLY facts, decisions, and open questions
- Remove greetings, thanks, repetitions
- Remove reasoning if a final decision exists
- Format: one statement per line

Conversation:
` + conversation,

        StrategyKeyValue: `Extract key facts from the conversation in "key: value" format.
Key categories:
- decision: a decision made
- constraint: a constraint or requirement
- action: an action taken
- open: an unresolved question

Example:
decision:database: Using PostgreSQL
constraint:budget: No more than $100/month

Conversation:
` + conversation,
    }

    prompt, ok := prompts[strategy]
    if !ok {
        return "", fmt.Errorf("unknown strategy: %s", strategy)
    }

    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: "gpt-4o-mini",
        Messages: []openai.ChatCompletionMessage{
            {Role: "system", Content: "You compress context. Be as brief as possible."},
            {Role: "user", Content: prompt},
        },
        Temperature: 0,
    })
    if err != nil {
        return "", err
    }
    return resp.Choices[0].Message.Content, nil
}

func formatMessages(messages []openai.ChatCompletionMessage) string {
    var b strings.Builder
    for _, msg := range messages {
        fmt.Fprintf(&b, "[%s]: %s\n", msg.Role, msg.Content)
    }
    return b.String()
}
```

### Choosing the Right Strategy

| Strategy | Compression Ratio | Information Loss | When to Use |
|---|---|---|---|
| `summarize` | Medium (~3x) | Low | Need context to continue dialog |
| `semantic` | High (~5-10x) | Medium | Long discussions, need the gist |
| `keyvalue` | Very high (~10-20x) | High (facts only) | Long-term storage, cross-session |

## Incremental Summarization

### The Problem

Summarizing the entire history every time is expensive. If a conversation has 100 messages and you summarize every 10, by the 10th iteration you're reprocessing everything from scratch. That's O(n²) in tokens.

### Solution: Update the Existing Summary

Instead of summarizing the full history, take the previous summary and augment it with new messages. That's O(n) in tokens.

```go
// incrementalSummarize updates the existing summary with new messages.
// Instead of re-summarizing the full history — augments the current summary.
func incrementalSummarize(
    ctx context.Context,
    client *openai.Client,
    currentSummary string,
    newMessages []openai.ChatCompletionMessage,
) (string, error) {
    if len(newMessages) == 0 {
        return currentSummary, nil
    }

    newConversation := formatMessages(newMessages)

    var prompt string
    if currentSummary == "" {
        // First summarization
        prompt = "Summarize this conversation. Preserve key facts, decisions, and open questions:\n\n" + newConversation
    } else {
        // Update existing summary
        prompt = fmt.Sprintf(`Update the conversation summary with new messages.

Current summary:
%s

New messages:
%s

Rules:
- Include ALL important information from the current summary
- Add new facts and decisions from new messages
- If new messages contradict the summary — use the new information
- Remove outdated items if they were resolved in new messages
- Keep the format compact`, currentSummary, newConversation)
    }

    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: "gpt-4o-mini",
        Messages: []openai.ChatCompletionMessage{
            {Role: "system", Content: "You update conversation summaries. Be precise and concise."},
            {Role: "user", Content: prompt},
        },
        Temperature: 0,
    })
    if err != nil {
        return currentSummary, err // On error, keep the old summary
    }
    return resp.Choices[0].Message.Content, nil
}
```

### Using in LayeredContext

```go
func (c *LayeredContext) SummarizeIncremental(ctx context.Context, client *openai.Client) error {
    if len(c.workingMemory) <= c.maxWorking {
        return nil
    }

    // Take only messages that overflow working memory
    overflow := c.workingMemory[:len(c.workingMemory)-c.maxWorking]

    // Update summary incrementally (don't re-summarize everything)
    updated, err := incrementalSummarize(ctx, client, c.summary, overflow)
    if err != nil {
        return err
    }

    c.summary = updated
    c.workingMemory = c.workingMemory[len(c.workingMemory)-c.maxWorking:]
    return nil
}
```

**Cost comparison:**

| Approach | Tokens at 100th message | Growth |
|---|---|---|
| Full summarization | ~50K (entire history) | O(n²) |
| Incremental | ~2K (summary + 10 new) | O(n) |

## Context Prioritization

When the token budget is tight, you need to decide: which data matters more. Not everything is equally valuable — recent messages matter more than old ones, errors matter more than successful results.

### Budget by Layer

Divide available tokens across context layers. A fixed ratio guarantees no single layer consumes the entire budget:

```go
// TokenBudget distributes available tokens across context layers.
type TokenBudget struct {
    Total          int     // Total budget (model maxTokens - maxOutputTokens)
    SystemRatio    float64 // Share for system prompt (0.10-0.15)
    FactsRatio     float64 // Share for facts (0.10-0.15)
    SummaryRatio   float64 // Share for summary (0.15-0.20)
    WorkingRatio   float64 // Share for working memory (0.50-0.65)
}

func (b TokenBudget) SystemBudget() int  { return int(float64(b.Total) * b.SystemRatio) }
func (b TokenBudget) FactsBudget() int   { return int(float64(b.Total) * b.FactsRatio) }
func (b TokenBudget) SummaryBudget() int { return int(float64(b.Total) * b.SummaryRatio) }
func (b TokenBudget) WorkingBudget() int { return int(float64(b.Total) * b.WorkingRatio) }
```

### Message Scoring

Not all messages are equally useful. Score importance and select within the budget:

```go
// ScoredMessage — a message with an importance score.
type ScoredMessage struct {
    Message    openai.ChatCompletionMessage
    Score      float64
    TokenCount int
}

// scoreMessage scores message importance.
// High score = message should be kept.
func scoreMessage(msg openai.ChatCompletionMessage, position, total int) float64 {
    score := 0.0

    // 1. Recency: recent messages are more important (0.0–0.4)
    recency := float64(position) / float64(total)
    score += recency * 0.4

    // 2. Role: assistant messages with tool_calls are more important than plain text
    if msg.Role == "tool" {
        score += 0.2 // Tool call results are important
    }

    // 3. Content: errors and important decisions
    content := strings.ToLower(msg.Content)
    if strings.Contains(content, "error") {
        score += 0.3 // Errors are more important than regular messages
    }
    if strings.Contains(content, "decision") || strings.Contains(content, "chose") {
        score += 0.2 // Decisions are worth remembering
    }

    return score
}

// prioritizeContext assembles context respecting budget and priorities.
func prioritizeContext(
    messages []openai.ChatCompletionMessage,
    facts []Fact,
    summary string,
    budget TokenBudget,
    counter TokenCounter,
) []openai.ChatCompletionMessage {
    var result []openai.ChatCompletionMessage

    // 1. Facts — within budget
    if len(facts) > 0 {
        factsText := buildFactsText(facts, budget.FactsBudget(), counter)
        result = append(result, openai.ChatCompletionMessage{
            Role:    "system",
            Content: factsText,
        })
    }

    // 2. Summary — truncate if it doesn't fit
    if summary != "" {
        if counter.Count(summary) > budget.SummaryBudget() {
            // Summary too long — truncate by sentence
            summary = truncateText(summary, budget.SummaryBudget(), counter)
        }
        result = append(result, openai.ChatCompletionMessage{
            Role:    "system",
            Content: "Previous conversation summary:\n" + summary,
        })
    }

    // 3. Working memory — select by score
    scored := make([]ScoredMessage, len(messages))
    for i, msg := range messages {
        scored[i] = ScoredMessage{
            Message:    msg,
            Score:      scoreMessage(msg, i, len(messages)),
            TokenCount: counter.Count(msg.Content) + 4,
        }
    }

    // Always include the last user message
    workingBudget := budget.WorkingBudget()
    if len(scored) > 0 {
        last := scored[len(scored)-1]
        workingBudget -= last.TokenCount
    }

    // Remaining messages — by descending score, while they fit
    sort.Slice(scored[:len(scored)-1], func(i, j int) bool {
        return scored[i].Score > scored[j].Score
    })

    var selected []ScoredMessage
    used := 0
    for _, sm := range scored[:len(scored)-1] {
        if used+sm.TokenCount > workingBudget {
            continue
        }
        selected = append(selected, sm)
        used += sm.TokenCount
    }

    // Restore chronological order
    sort.Slice(selected, func(i, j int) bool {
        return indexOfMessage(messages, selected[i].Message) <
            indexOfMessage(messages, selected[j].Message)
    })

    for _, sm := range selected {
        result = append(result, sm.Message)
    }

    // Last message — always at the end
    if len(messages) > 0 {
        result = append(result, messages[len(messages)-1])
    }

    return result
}
```

### Budget Example

For a model with 128K context and `maxOutputTokens = 4096`:

| Layer | Share | Tokens |
|---|---|---|
| System prompt | 10% | ~12,400 |
| Facts | 10% | ~12,400 |
| Summary | 20% | ~24,800 |
| Working memory | 60% | ~74,300 |
| **Total input** | 100% | **~123,900** |
| Model response | — | 4,096 |

## Common Errors

### Error 1: No Summarization

**Symptom:** Context grows infinitely, reaching token limits.

**Cause:** Old conversations are never summarized.

**Solution:** Implement periodic summarization when working memory exceeds threshold.

### Error 2: Too Aggressive Summarization

**Symptom:** Important details lost in summary, agent makes mistakes.

**Cause:** Summary too compressed, facts not extracted.

**Solution:** Extract facts before summarization, save them separately.

### Error 3: No Fact Selection

**Symptom:** Including irrelevant facts wastes tokens.

**Cause:** Including all facts regardless of relevance.

**Solution:** Score facts by importance, include only highly scored facts.

### Error 4: Preferences Included as Facts

**Symptom:** Model shifts answer toward user preferences, even if actual data points elsewhere.

**Cause:** User preferences or hypotheses included in context as facts without distinguishing types.

**Solution:**
```go
// GOOD: Distinguish types
fact := Fact{
    Key:   "user_thinks_db_problem",
    Value: "User assumes problem is in DB",
    Type:  "hypothesis", // Not "fact"!
}

// When assembling context for analytical task:
if !includePreferences {
    // Exclude hypotheses and preferences
    if fact.Type == "fact" || fact.Type == "constraint" {
        includeInContext(fact)
    }
}
```

**Practice:** For analytical tasks (incidents, diagnostics), exclude preferences and hypotheses from context. Include them only for personalized responses (e.g., recommendations based on user preferences).

## Mini-Exercises

### Exercise 1: Implement Summarization

Create a function that summarizes conversation history:

```go
func summarizeConversation(messages []openai.ChatCompletionMessage) (string, error) {
    // Use LLM to create summary
}
```

**Expected result:**
- Summary preserves key facts
- Significantly reduces token count
- Can recover main points

## Completion Criteria / Checklist

**Completed:**
- [x] Understand context layers
- [x] Can summarize conversations
- [x] Extract and store facts
- [x] Manage context within token limits

**Not completed:**
- [ ] No summarization, context grows infinitely
- [ ] Too aggressive summarization, facts lost
- [ ] No fact selection, token waste

## Connection with Other Chapters

- **[Chapter 11: State Management](../11-state-management/README.md)** — Task state is used when assembling context
- **[Chapter 12: Agent Memory Systems](../12-agent-memory/README.md)** — Facts from memory are used in context (storage/retrieval described there)
- **[Chapter 20: Cost & Latency Engineering](../20-cost-latency-engineering/README.md)** — Token budgets control context selection policies

**IMPORTANT:** Context Engineering focuses on **assembling context** from various sources (memory, state, retrieval). Data storage is described in respective chapters (Memory, State Management, RAG).

## What's Next?

After mastering context engineering, proceed to:
- **[14. Ecosystem and Frameworks](../14-ecosystem-and-frameworks/README.md)** — Learn about agent frameworks

