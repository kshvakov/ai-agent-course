# 07. RAG and Knowledge Base

## Why This Chapter?

A regular agent only knows what it was taught during training (up to the cut-off date). It doesn't know your local instructions like "How to restart Phoenix server according to protocol #5".

**RAG (Retrieval Augmented Generation)** is a mechanism for "peeking at a cheat sheet". The agent first searches for information in the knowledge base, then acts.

Without RAG, an agent cannot use your documentation, protocols, and knowledge base. With RAG, the agent can find the needed information and act according to your instructions.

### Real-World Case Study

**Situation:** User writes: "Restart Phoenix server according to protocol"

**Problem:** Agent doesn't know the Phoenix server restart protocol. It may perform a standard restart that doesn't match your procedures.

**Solution:** RAG allows the agent to find the protocol in the knowledge base before performing the action. Agent finds document "Phoenix server restart protocol: 1. Turn off load balancer 2. Restart server 3. Turn on load balancer" and follows these steps.

## Theory in Simple Terms

### How Does RAG Work?

1. **Agent receives request** from user
2. **Agent searches for information** in knowledge base via search tool
3. **Knowledge base returns** relevant documents
4. **Agent uses information** to perform action

## How Does RAG Work? — Magic vs Reality

**❌ Magic:**
> Agent "knows" it needs to search the knowledge base and finds the needed information itself

**✅ Reality:**

### Full RAG Protocol

**Step 1: User Request**

```go
messages := []openai.ChatCompletionMessage{
    {Role: "system", Content: "You are a DevOps assistant. Always search knowledge base before actions."},
    {Role: "user", Content: "Restart Phoenix server according to protocol"},
}
```

**Step 2: Model Sees Search Tool Description**

```go
tools := []openai.Tool{
    {
        Function: &openai.FunctionDefinition{
            Name: "search_knowledge_base",
            Description: "Search the knowledge base for documentation, protocols, and procedures. Use this BEFORE performing actions that require specific procedures.",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "query": {"type": "string", "description": "Search query"}
                },
                "required": ["query"]
            }`),
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name: "restart_server",
            Description: "Restart a server",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "hostname": {"type": "string"}
                },
                "required": ["hostname"]
            }`),
        },
    },
}
```

**Step 3: Model Generates Tool Call for Search**

```go
resp1, _ := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
    Model:    openai.GPT3Dot5Turbo,
    Messages: messages,
    Tools:    tools,
})

msg1 := resp1.Choices[0].Message
// msg1.ToolCalls = [{
//     Function: {
//         Name: "search_knowledge_base",
//         Arguments: "{\"query\": \"Phoenix restart protocol\"}"
//     }
// }]
```

**Why did model generate tool_call for search?**
- System Prompt says: "Always search knowledge base before actions"
- Tool description says: "Use this BEFORE performing actions"
- Model sees word "protocol" in request and links it to search tool

**Step 4: Runtime Executes Search**

```go
func searchKnowledgeBase(query string) string {
    // Simple keyword search (in production - vector search)
    knowledgeBase := map[string]string{
        "protocols.txt": "Phoenix server restart protocol:\n1. Turn off load balancer\n2. Restart server\n3. Turn on load balancer",
    }
    
    for filename, content := range knowledgeBase {
        if strings.Contains(strings.ToLower(content), strings.ToLower(query)) {
            return fmt.Sprintf("File: %s\nContent: %s", filename, content)
        }
    }
    return "No documents found"
}

result1 := searchKnowledgeBase("Phoenix restart protocol")
// result1 = "File: protocols.txt\nContent: Phoenix server restart protocol:\n1. Turn off load balancer..."
```

**Step 5: Search Result Added to Context**

```go
messages = append(messages, openai.ChatCompletionMessage{
    Role:       "tool",
    Content:    result1,  // All found documentation!
    ToolCallID: msg1.ToolCalls[0].ID,
})
// Now messages contains:
// [system, user, assistant(tool_call: search_kb), tool("File: protocols.txt\nContent: ...")]
```

**Step 6: Model Sees Documentation and Acts**

```go
// Send updated history (with documentation!) to model
resp2, _ := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
    Model:    openai.GPT3Dot5Turbo,
    Messages: messages,  // Model sees found documentation!
    Tools:    tools,
})

msg2 := resp2.Choices[0].Message
// Model sees in context:
// "Phoenix server restart protocol:\n1. Turn off load balancer..."
// Model generates tool calls according to protocol:

// msg2.ToolCalls = [
//     {Function: {Name: "restart_server", Arguments: "{\"hostname\": \"phoenix\"}"}},
//     // Or first turn off load balancer, then server
// ]
```

**Why this is not magic:**

1. **Model doesn't "know" protocol** — it sees it in context after search
2. **Search is a regular tool** — same as `ping` or `restart_service`
3. **Search result added to `messages[]`** — model sees it as a new message
4. **Model generates actions based on context** — it sees documentation and follows it

**Key point:** RAG is not magic "knowledge", but a mechanism for adding relevant information to model context through a regular tool call.

## Simple RAG vs Vector Search

In this lab we implement **simple RAG** (keyword search). In production, **vector search** (Semantic Search) is used, which searches by meaning, not by words.

**Simple RAG (Lab 07):**
```go
// Search by substring match
if strings.Contains(content, query) {
    return content
}
```

**Vector Search (production):**
```go
// 1. Documents split into chunks and converted to vectors (embeddings)
chunks := []Chunk{
    {ID: "chunk_1", Text: "Phoenix restart protocol...", Embedding: [1536]float32{...}},
    {ID: "chunk_2", Text: "Step 2: Turn off load balancer...", Embedding: [1536]float32{...}},
}

// 2. User query also converted to vector
queryEmbedding := embedQuery("Phoenix restart protocol")  // [1536]float32{...}

// 3. Search similar vectors by cosine distance
similarDocs := vectorDB.Search(queryEmbedding, topK=3)
// Returns 3 most similar chunks by meaning (not by words!)

// 4. Result added to model context same as in simple RAG
result := formatChunks(similarDocs)  // "Chunk 1: ...\nChunk 2: ...\nChunk 3: ..."
messages = append(messages, openai.ChatCompletionMessage{
    Role:    "tool",
    Content: result,
})
```

**Why vector search is better:**
- Searches by **meaning**, not by words
- Will find "restart Phoenix" even if document says "Phoenix server restart"
- Works with synonyms and different phrasings

## Chunking

Documents are split into chunks (pieces) for efficient search.

**Example:**
```
Document: "Phoenix server restart protocol..."
Chunk 1: "Phoenix server restart protocol: step 1..."
Chunk 2: "Step 2: Turn off load balancer..."
Chunk 3: "Step 3: Restart server..."
```

## Common Mistakes

### Mistake 1: Agent Doesn't Search Knowledge Base

**Symptom:** Agent performs actions without searching knowledge base, using only general knowledge.

**Cause:** System Prompt doesn't instruct agent to search knowledge base, or search tool description is not clear enough.

**Solution:**
```go
// GOOD: System Prompt requires search
systemPrompt := `... Always search knowledge base before performing actions that require specific procedures.`

// GOOD: Clear tool description
Description: "Search the knowledge base for documentation, protocols, and procedures. Use this BEFORE performing actions that require specific procedures."
```

### Mistake 2: Poor Search Query

**Symptom:** Agent doesn't find needed information in knowledge base.

**Cause:** Search query is too general or doesn't contain keywords from document.

**Solution:**
```go
// BAD: Too general query
query := "server"

// GOOD: Specific query with keywords
query := "Phoenix server restart protocol"
```

### Mistake 3: Chunks Too Large

**Symptom:** Search returns documents too large that don't fit in context.

**Cause:** Chunk size too large (larger than context window).

**Solution:**
```go
// GOOD: Chunk size ~500-1000 tokens
chunkSize := 500  // Tokens
```

## Mini-Exercises

### Exercise 1: Implement Simple Search

Implement a simple keyword search function:

```go
func searchKnowledgeBase(query string) string {
    // Simple keyword search
    // Return relevant documents
}
```

**Expected result:**
- Function finds documents containing keywords from query
- Function returns first N relevant documents

### Exercise 2: Implement Chunking

Implement a function to split document into chunks:

```go
func chunkDocument(text string, chunkSize int) []string {
    // Split document into chunks of chunkSize tokens
    // Return list of chunks
}
```

**Expected result:**
- Function splits document into chunks of specified size
- Chunks don't overlap (or overlap minimally)

## Completion Criteria / Checklist

✅ **Completed:**
- Agent searches knowledge base before performing actions
- Search queries are specific and contain keywords
- Documents split into appropriately sized chunks
- Search tool has clear description
- System Prompt instructs agent to use knowledge base

❌ **Not completed:**
- Agent doesn't search knowledge base (uses only general knowledge)
- Search queries too general (doesn't find needed information)
- Chunks too large (don't fit in context)

## Connection with Other Chapters

- **Tools:** How search tool integrates into agent, see [Chapter 04: Tools](../04-tools-and-function-calling/README.md)
- **Autonomy:** How RAG works in agent loop, see [Chapter 05: Autonomy](../05-autonomy-and-loops/README.md)

## What's Next?

After studying RAG, proceed to:
- **[08. Multi-Agent Systems](../08-multi-agent/README.md)** — how to create a team of agents

---

**Navigation:** [← Safety](../06-safety-and-hitl/README.md) | [Table of Contents](../README.md) | [Multi-Agent →](../08-multi-agent/README.md)
