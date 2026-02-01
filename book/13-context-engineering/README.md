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

!!! warning "Context as an anchor (Anchoring Bias)"
    If **user preferences** ("user thinks X", "we need answer Y") or **unverified hypotheses** enter the facts layer, they become a strong anchor for the model. The model may shift answers toward these preferences, even if actual data points elsewhere.
    
    **Problem:** Preferences and hypotheses included in context as facts can distort objective analysis.
    
    **Solution:** Separate entry types: **Fact** (verified data), **Preference** (user preferences), **Hypothesis** (hypotheses). Include preferences and hypotheses in context only when appropriate (personalization), and exclude them for analytical tasks requiring objectivity.

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
        Model: openai.GPT3Dot5Turbo,
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

✅ **Completed:**
- Understand context layers
- Can summarize conversations
- Extract and store facts
- Manage context within token limits

❌ **Not completed:**
- No summarization, context grows infinitely
- Too aggressive summarization, facts lost
- No fact selection, token waste

## Connection with Other Chapters

- **[Chapter 11: State Management](../11-state-management/README.md)** — Task state is used when assembling context
- **[Chapter 12: Agent Memory Systems](../12-agent-memory/README.md)** — Facts from memory are used in context (storage/retrieval described there)
- **[Chapter 20: Cost & Latency Engineering](../20-cost-latency-engineering/README.md)** — Token budgets control context selection policies

**IMPORTANT:** Context Engineering focuses on **assembling context** from various sources (memory, state, retrieval). Data storage is described in respective chapters (Memory, State Management, RAG).

## What's Next?

After mastering context engineering, proceed to:
- **[14. Ecosystem and Frameworks](../14-ecosystem-and-frameworks/README.md)** — Learn about agent frameworks

---

**Navigation:** [← Agent Memory Systems](../12-agent-memory/README.md) | [Table of Contents](../README.md) | [Ecosystem and Frameworks →](../14-ecosystem-and-frameworks/README.md)
