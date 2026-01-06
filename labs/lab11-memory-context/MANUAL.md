# Method Guide: Lab 11 — Memory and Context Management

## Why Is This Needed?

In this lab you'll implement a memory system for the agent: long-term storage, fact extraction, and efficient context management.

### Real-World Case Study

**Situation:** Agent works with user over extended period.

**Without memory:**
- User: "My name is Ivan"
- Agent: "Hello, Ivan!"
- [After 20 messages]
- User: "What's my name?"
- Agent: "I don't know" (forgot)

**With memory:**
- User: "My name is Ivan"
- Agent extracts fact: "User name: Ivan" → saves to memory
- [After 20 messages]
- User: "What's my name?"
- Agent retrieves from memory: "User name: Ivan"
- Agent: "Your name is Ivan"

**Difference:** Memory allows agent to remember important information between sessions.

## Theory in Simple Terms

### Types of Memory

**Working Memory (Short-term):**
- Recent conversation turns (last 5-10 messages)
- Current task context
- Limited by model's context window

**Long-term Memory:**
- Important facts extracted from conversations
- User preferences
- Past decisions and results
- Stored separately from context

### Fact Extraction

Not all information is equally important:
- User name, preferences → important (store)
- Temporary server status → less important (can forget)
- Decisions made → important (store)

**Example:**
```
Conversation: "Hi, I'm Ivan. I work at TechCorp. Server is running now."
Extracted facts:
  - User name: Ivan (importance: 10)
  - Company: TechCorp (importance: 8)
  - Server status: running (importance: 2) → don't save, this is temporary
```

### Context Layers

Efficient context management uses layers:

```
Final context = 
  System Prompt (always first)
  + Facts Layer (relevant facts from memory)
  + Summary Layer (compressed history)
  + Working Memory (last 5-10 messages)
```

**Example:**
```
System Prompt: "You are a DevOps assistant"
Facts Layer: "User: Ivan, company: TechCorp"
Summary Layer: "Discussed server problem. Decided to restart."
Working Memory: 
  - User: "Check status"
  - Assistant: "Checking..."
```

### Summarization

When context overflows, compress old messages:
- Preserve important information
- Reduce token count
- Maintain context continuity

**Example summary:**
```
Original history (2000 tokens):
- User: "My name is Ivan"
- Assistant: "Hello, Ivan!"
- User: "We have a server problem"
- Assistant: "Describe the problem"
... (50 more messages)

Summary (200 tokens):
"User Ivan, DevOps engineer from TechCorp. Discussed server problem. 
Current task: diagnostics. Important decisions: decided to restart service."
```

## Execution Algorithm

### Step 1: Memory Storage

```go
type FileMemory struct {
    items []MemoryItem
    file  string
}

func (m *FileMemory) Store(key string, value any, importance int) error {
    item := MemoryItem{
        Key:        key,
        Value:      fmt.Sprintf("%v", value),
        Importance: importance,
        Timestamp:  time.Now().Unix(),
    }
    m.items = append(m.items, item)
    return m.save()
}

func (m *FileMemory) Retrieve(query string, limit int) ([]MemoryItem, error) {
    var results []MemoryItem
    queryLower := strings.ToLower(query)
    
    for _, item := range m.items {
        if strings.Contains(strings.ToLower(item.Value), queryLower) {
            results = append(results, item)
        }
    }
    
    // Sort by importance
    sort.Slice(results, func(i, j int) bool {
        return results[i].Importance > results[j].Importance
    })
    
    if len(results) > limit {
        results = results[:limit]
    }
    
    return results, nil
}
```

### Step 2: Fact Extraction

```go
func extractFacts(ctx context.Context, client *openai.Client, conversation string) ([]Fact, error) {
    prompt := fmt.Sprintf(`Extract important facts from this conversation.
    
Conversation:
%s

Return facts in JSON format:
{
  "facts": [
    {"key": "user_name", "value": "Ivan", "importance": 10},
    {"key": "company", "value": "TechCorp", "importance": 8}
  ]
}

Importance: 1-10, where 10 is very important (user name, preferences),
1-3 is temporary information (server status, temporary events).`, conversation)

    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: openai.GPT3Dot5Turbo,
        Messages: []openai.ChatCompletionMessage{
            {Role: "user", Content: prompt},
        },
        Temperature: 0,
    })
    if err != nil {
        return nil, err
    }
    
    // Parse JSON response
    var data struct {
        Facts []Fact `json:"facts"`
    }
    json.Unmarshal([]byte(resp.Choices[0].Message.Content), &data)
    
    return data.Facts, nil
}
```

### Step 3: Summarization

```go
func summarizeConversation(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage) (string, error) {
    // Collect text of all messages (except System)
    var textParts []string
    for _, msg := range messages {
        if msg.Role != openai.ChatMessageRoleSystem && msg.Content != "" {
            textParts = append(textParts, fmt.Sprintf("%s: %s", msg.Role, msg.Content))
        }
    }
    conversationText := strings.Join(textParts, "\n")
    
    prompt := fmt.Sprintf(`Create a brief summary of this conversation, keeping only:
1. Important decisions made
2. Key facts discovered (user name, preferences)
3. Current state of the task

Conversation:
%s

Summary (maximum 200 words):`, conversationText)

    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: openai.GPT3Dot5Turbo,
        Messages: []openai.ChatCompletionMessage{
            {Role: "user", Content: prompt},
        },
        Temperature: 0,
    })
    if err != nil {
        return "", err
    }
    
    return resp.Choices[0].Message.Content, nil
}
```

### Step 4: Layered Context Assembly

```go
func buildLayeredContext(
    systemPrompt string,
    memory Memory,
    summary string,
    workingMemory []openai.ChatCompletionMessage,
    query string,
) []openai.ChatCompletionMessage {
    messages := []openai.ChatCompletionMessage{
        {Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
    }
    
    // Facts layer
    facts, _ := memory.Retrieve(query, 5)
    if len(facts) > 0 {
        var factTexts []string
        for _, fact := range facts {
            factTexts = append(factTexts, fmt.Sprintf("- %s: %s", fact.Key, fact.Value))
        }
        messages = append(messages, openai.ChatCompletionMessage{
            Role: openai.ChatMessageRoleSystem,
            Content: "Important facts:\n" + strings.Join(factTexts, "\n"),
        })
    }
    
    // Summary layer
    if summary != "" {
        messages = append(messages, openai.ChatCompletionMessage{
            Role: openai.ChatMessageRoleSystem,
            Content: "Summary of previous conversation:\n" + summary,
        })
    }
    
    // Working memory
    messages = append(messages, workingMemory...)
    
    return messages
}
```

## Common Mistakes

### Mistake 1: All Facts Extracted Without Filtering

**Symptom:** Memory fills with unimportant facts.

**Cause:** Facts not filtered by importance.

**Solution:** Only save facts with importance >= 5.

### Mistake 2: Summarization Loses Important Information

**Symptom:** After summarization agent forgets user name.

**Cause:** Summary doesn't include important facts.

**Solution:** Specify in summarization prompt to preserve important facts.

### Mistake 3: Facts Not Retrieved by Relevance

**Symptom:** Agent doesn't find relevant facts for current query.

**Cause:** Memory search doesn't consider relevance.

**Solution:** Use semantic search or improve keywords.

## Completion Criteria

✅ **Completed:**
- Memory saved to file
- Facts extracted via LLM
- Summarization reduces tokens
- Context assembled from layers
- Agent remembers important facts between sessions

❌ **Not completed:**
- Memory not saved
- Facts not extracted
- Summarization loses important information
- Context not layered
