# 06. RAG and Knowledge Base

## Why This Chapter?

A regular agent only knows what it was taught during training (up to the cut-off date). It doesn't know your local instructions like "How to restart Phoenix server according to protocol #5".

**RAG (Retrieval Augmented Generation)** is a "cheat sheet lookup" mechanism. The agent first searches for information in the knowledge base, then acts.

Without RAG, an agent cannot use your documentation, protocols, and knowledge base. With RAG, an agent can find the needed information and act according to your instructions.

### Real-World Case Study

**Situation:** A user writes: "Restart Phoenix server according to protocol"

**Problem:** The agent doesn't know the Phoenix server restart protocol. It may perform a standard restart that doesn't match your procedures.

**Solution:** RAG allows the agent to find the protocol in the knowledge base before performing the action. The agent finds the document "Phoenix server restart protocol: 1. Turn off load balancer 2. Restart server 3. Turn on load balancer" and follows these steps.

## Theory in Simple Terms

### How Does RAG Work?

1. **Agent receives a request** from the user
2. **Agent searches for information** in the knowledge base via a search tool
3. **Knowledge base returns** relevant documents
4. **Agent uses the information** to perform the action

## How Does RAG Work? — Magic vs Reality

**❌ Magic:**
> Agent "knows" it needs to search the knowledge base and finds the needed information on its own

**✅ Reality:**

### Complete RAG Protocol

**Step 1: User Request**

```go
messages := []openai.ChatCompletionMessage{
    {Role: "system", Content: "You are a DevOps assistant. Always search knowledge base before actions."},
    {Role: "user", Content: "Restart Phoenix server according to protocol"},
}
```

**Step 2: Model Receives Search Tool Description**

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

**Why did the model generate a tool_call for search?**
- System Prompt says: "Always search knowledge base before actions"
- Tool Description says: "Use this BEFORE performing actions"
- The model identifies the word "protocol" in the request and links it to the search tool

**Step 4: Runtime (Your Code) Executes Search**

> **Note:** Runtime is the agent code you write in Go. See [Chapter 00: Preface](../00-preface/README.md#runtime-execution-environment) for definition.

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

**Step 6: Model Receives Documentation and Acts**

```go
// Send updated history (with documentation!) to model
resp2, _ := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
    Model:    openai.GPT3Dot5Turbo,
    Messages: messages,  // Model receives found documentation!
    Tools:    tools,
})

msg2 := resp2.Choices[0].Message
// Model receives in context:
// "Phoenix server restart protocol:\n1. Turn off load balancer..."
// Model generates tool calls according to protocol:

// msg2.ToolCalls = [
//     {Function: {Name: "restart_server", Arguments: "{\"hostname\": \"phoenix\"}"}},
//     // Or first turn off load balancer, then server
// ]
```

**Why this is not magic:**

1. **Model doesn't "know" the protocol** — it receives it in context after search
2. **Search is a regular tool** — same as `ping` or `restart_service`
3. **Search result is added to `messages[]`** — model receives it as a new message
4. **Model generates actions based on context** — it processes documentation and follows it

**Key point:** RAG is not magic "knowledge", but a mechanism for adding relevant information to the model's context through a regular tool call.

## Simple RAG vs Vector Search

In this lab, we implement **simple RAG** (keyword search). In production, **vector search** (Semantic Search) is used, which searches by meaning, not by words.

**Simple RAG (Lab 07):**
```go
// Search by substring match
if strings.Contains(content, query) {
    return content
}
```

**Vector search (production):**
```go
// 1. Documents are split into chunks and converted to vectors (embeddings)
chunks := []Chunk{
    {ID: "chunk_1", Text: "Phoenix restart protocol...", Embedding: [1536]float32{...}},
    {ID: "chunk_2", Text: "Step 2: Turn off load balancer...", Embedding: [1536]float32{...}},
}

// 2. User query is also converted to vector
queryEmbedding := embedQuery("Phoenix restart protocol")  // [1536]float32{...}

// 3. Search for similar vectors by cosine distance
similarDocs := vectorDB.Search(queryEmbedding, topK=3)
// Returns 3 most similar chunks by meaning (not by words!)

// 4. Result is added to model context the same way as in simple RAG
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

## Common Errors

### Error 1: Agent Doesn't Search Knowledge Base

**Symptom:** Agent performs actions without searching the knowledge base, using only general knowledge.

**Cause:** System Prompt doesn't instruct agent to search knowledge base, or search tool description is not clear enough.

**Solution:**
```go
// GOOD: System Prompt requires search
systemPrompt := `... Always search knowledge base before performing actions that require specific procedures.`

// GOOD: Clear tool description
Description: "Search the knowledge base for documentation, protocols, and procedures. Use this BEFORE performing actions that require specific procedures."
```

### Error 2: Poor Search Query

**Symptom:** Agent doesn't find needed information in the knowledge base.

**Cause:** Search query is too general or doesn't contain keywords from the document.

**Solution:**
```go
// BAD: Too general query
query := "server"

// GOOD: Specific query with keywords
query := "Phoenix server restart protocol"
```

### Error 3: Chunks Too Large

**Symptom:** Search returns documents that are too large and don't fit in context.

**Cause:** Chunk size is too large (larger than context window).

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

Implement a function to split a document into chunks:

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
- Documents are split into appropriately sized chunks
- Search tool has clear description
- System Prompt instructs agent to use knowledge base

❌ **Not completed:**
- Agent doesn't search knowledge base (uses only general knowledge)
- Search queries are too general (doesn't find needed information)
- Chunks are too large (don't fit in context)

## Production Notes

When using RAG in production, consider:

- **Document versioning:** Track document versions and update date (`updated_at`). This helps understand which version was used in the response.
- **Freshness (currency):** Filter outdated documents (e.g., older than 30 days) before using in context.
- **Grounding:** Require agent to reference found documents in response. This reduces hallucinations and increases trust.

More on production readiness: [Chapter 19: Observability](../19-observability-and-tracing/README.md), [Chapter 23: Evals in CI/CD](../23-evals-in-cicd/README.md).

## Connection with Other Chapters

- **Tools:** How search tool integrates into agent, see [Chapter 03: Tools](../03-tools-and-function-calling/README.md)
- **Autonomy:** How RAG works in the agent loop, see [Chapter 04: Autonomy](../04-autonomy-and-loops/README.md)

## What's Next?

After studying RAG, proceed to:
- **[07. Multi-Agent Systems](../07-multi-agent/README.md)** — how to create a team of agents

---

**Navigation:** [← Safety](../05-safety-and-hitl/README.md) | [Table of Contents](../README.md) | [Multi-Agent →](../07-multi-agent/README.md)
