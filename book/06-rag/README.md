# 06. RAG and Knowledge Base

## Why This Chapter?

A regular agent only knows what it was trained on (up to the cut-off date). It doesn't know your local instructions like "How to restart Phoenix server according to protocol #5".

**RAG (Retrieval Augmented Generation)** is a "cheat sheet lookup" mechanism. The agent first searches for information in the knowledge base, then acts.

Without RAG, an agent can't use your documentation, protocols, and knowledge base. With RAG, an agent can find the needed information and act according to your instructions.

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

**Why this isn't magic:**

1. **Model doesn't "know" the protocol** — it receives it in context after search
2. **Search is a regular tool** — same as `ping` or `restart_service`
3. **Search result is added to `messages[]`** — model receives it as a new message
4. **Model generates actions based on context** — it processes documentation and follows it

**Takeaway:** RAG is not magic "knowledge", but a mechanism for adding relevant information to the model's context through a regular tool call.

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

## RAG for Action Space (Tool Retrieval)

So far we've discussed RAG for **documents** (protocols, instructions). But RAG is also needed for **action space** — when an agent has a potentially infinite number of tools.

### The Problem: "Infinite" Tools

**Situation:** An agent needs to work with Linux commands for troubleshooting. Linux has thousands of commands (`grep`, `awk`, `sed`, `jq`, `sort`, `uniq`, `head`, `tail`, etc.), and they can be combined into pipelines.

**Problem:**
- Can't pass all commands in `tools[]` — that's thousands of tokens
- Model performs worse with large lists (more hallucinations)
- Latency increases (more tokens = slower)
- No security control (which commands are dangerous?)

**Naive solution (doesn't work):**
```go
// BAD: One universal tool for everything
tools := []openai.Tool{
    {
        Function: &openai.FunctionDefinition{
            Name: "run_shell",
            Description: "Execute any shell command",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "command": {"type": "string"}
                }
            }`),
        },
    },
}
```

**Why this is bad:**
- No validation (could execute `rm -rf /`)
- No audit trail (unclear which commands were used)
- No control (model can call anything)

### Solution: Tool RAG (Action-Space Retrieval)

**Idea:** Store a **tool catalog** and retrieve only relevant tools before planning.

**How it works:**

1. **Tool catalog** stores metadata for each tool:
   - Name and description
   - Tags/categories (e.g., "text-processing", "network", "filesystem")
   - Parameters and their types
   - Risk level (safe/moderate/dangerous)
   - Usage examples

2. **Before planning**, agent searches for relevant tools:
   - Based on user query ("find errors in logs")
   - Retrieves top-k tools (e.g., `grep`, `tail`, `jq`)
   - Adds only their schemas to `tools[]`

3. **For pipelines**, use a two-level contract:
   - **JSON DSL** describes the pipeline plan (steps, stdin/stdout, expectations)
   - **Runtime** maps DSL to tool calls or executes via single `execute_pipeline`

### Example: Tool RAG for Linux Commands

**Step 1: Tool Catalog**

```go
type ToolDefinition struct {
    Name        string
    Description string
    Tags        []string  // "text-processing", "filtering", "sorting"
    RiskLevel   string    // "safe", "moderate", "dangerous"
    Schema      json.RawMessage
}

var toolCatalog = []ToolDefinition{
    {
        Name:        "grep",
        Description: "Search for patterns in text. Use for filtering lines matching a pattern.",
        Tags:        []string{"text-processing", "filtering", "search"},
        RiskLevel:   "safe",
        Schema: json.RawMessage(`{
            "type": "object",
            "properties": {
                "pattern": {"type": "string"},
                "input": {"type": "string"}
            }
        }`),
    },
    {
        Name:        "sort",
        Description: "Sort lines of text. Use for ordering output.",
        Tags:        []string{"text-processing", "sorting"},
        RiskLevel:   "safe",
        Schema: json.RawMessage(`{
            "type": "object",
            "properties": {
                "input": {"type": "string"}
            }
        }`),
    },
    {
        Name:        "head",
        Description: "Show first N lines. Use for limiting output.",
        Tags:        []string{"text-processing", "filtering"},
        RiskLevel:   "safe",
        Schema: json.RawMessage(`{
            "type": "object",
            "properties": {
                "lines": {"type": "number"},
                "input": {"type": "string"}
            }
        }`),
    },
    // ... hundreds more tools
}
```

**Step 2: Search for Relevant Tools**

For tool search, you can use two approaches: simple keyword search (for learning) and vector search (for production).

**Simple search (Lab 13):**
```go
func searchToolCatalog(query string, topK int) []ToolDefinition {
    // Simple search by description and tags
    var results []ToolDefinition
    
    queryLower := strings.ToLower(query)
    for _, tool := range toolCatalog {
        // Search by description
        if strings.Contains(strings.ToLower(tool.Description), queryLower) {
            results = append(results, tool)
            continue
        }
        // Search by tags
        for _, tag := range tool.Tags {
            if strings.Contains(strings.ToLower(tag), queryLower) {
                results = append(results, tool)
                break
            }
        }
    }
    
    // Return top-k
    if len(results) > topK {
        return results[:topK]
    }
    return results
}
```

**Vector search (production):**
```go
// 1. Tools are converted to vectors (embeddings)
toolEmbeddings := []ToolEmbedding{
    {
        Tool:      toolCatalog[0], // grep
        Embedding: embedText("Search for patterns in text. Use for filtering lines matching a pattern."), // [1536]float32{...}
    },
    {
        Tool:      toolCatalog[1], // sort
        Embedding: embedText("Sort lines of text. Use for ordering output."), // [1536]float32{...}
    },
    // ... all tools
}

// 2. User query is also converted to vector
queryEmbedding := embedQuery("find errors in logs")  // [1536]float32{...}

// 3. Search for similar vectors by cosine distance
similarTools := vectorDB.Search(queryEmbedding, topK=5)
// Returns 5 most similar tools by meaning (not by words!)

// 4. Result is used the same way as in simple search
relevantTools := extractTools(similarTools)  // [grep, tail, jq, ...]
```

**Why vector search is better for tools:**
- Searches by **meaning**, not by words
- Will find `grep` even if query is "filter lines by pattern" (without word "grep")
- Works with synonyms and different phrasings
- Especially important for large catalogs (1000+ tools)

**Example usage:**
```go
userQuery := "find errors in logs"
relevantTools := searchToolCatalog("error log filter", 5)
// Returns: [grep, tail, jq, ...] - only relevant ones!
```

**Step 3: Add Only Relevant Tools to Context**

```go
// Instead of passing all 1000+ tools
relevantTools := searchToolCatalog(userQuery, 5)

// Convert to OpenAI format
tools := make([]openai.Tool, 0, len(relevantTools))
for _, toolDef := range relevantTools {
    tools = append(tools, openai.Tool{
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name:        toolDef.Name,
            Description: toolDef.Description,
            Parameters:  toolDef.Schema,
        },
    })
}

// Now tools contains only 5 relevant tools instead of 1000+
```

### Pipelines: JSON DSL + Runtime

For complex tasks (e.g., "find top 10 errors in logs"), the agent must build **pipelines** from multiple commands.

**Approach 1: JSON DSL Pipeline**

Agent generates a formalized pipeline plan:

```go
type PipelineStep struct {
    Tool    string                 `json:"tool"`
    Args    map[string]interface{} `json:"args"`
    Input   string                 `json:"input,omitempty"`  // stdin from previous step
    Output  string                 `json:"output,omitempty"` // expected format
}

type Pipeline struct {
    Steps         []PipelineStep `json:"steps"`
    ExpectedOutput string        `json:"expected_output"`
    RiskLevel     string         `json:"risk_level"` // "safe", "moderate", "dangerous"
}

// Example: Agent generates this JSON
pipelineJSON := `{
    "steps": [
        {
            "tool": "grep",
            "args": {"pattern": "ERROR"},
            "input": "logs.txt"
        },
        {
            "tool": "sort",
            "args": {},
            "input": "{{step_0.output}}"
        },
        {
            "tool": "head",
            "args": {"lines": 10},
            "input": "{{step_1.output}}"
        }
    ],
    "expected_output": "Top 10 error lines, sorted",
    "risk_level": "safe"
}`
```

**Approach 2: Runtime Executes Pipeline**

```go
func executePipeline(pipelineJSON string, inputData string) (string, error) {
    var pipeline Pipeline
    if err := json.Unmarshal([]byte(pipelineJSON), &pipeline); err != nil {
        return "", err
    }
    
    // Validation: check risk
    if pipeline.RiskLevel == "dangerous" {
        return "", fmt.Errorf("dangerous pipeline requires confirmation")
    }
    
    // Execute steps sequentially
    currentInput := inputData
    for i, step := range pipeline.Steps {
        // Substitute result from previous step
        if strings.Contains(step.Input, "{{step_") {
            step.Input = currentInput
        }
        
        // Execute step (in reality - call corresponding tool)
        result, err := executeToolStep(step.Tool, step.Args, step.Input)
        if err != nil {
            return "", fmt.Errorf("step %d failed: %v", i, err)
        }
        
        currentInput = result
    }
    
    return currentInput, nil
}
```

**Approach 3: `execute_pipeline` Tool**

Agent calls a single tool with pipeline JSON:

```go
tools := []openai.Tool{
    {
        Function: &openai.FunctionDefinition{
            Name: "execute_pipeline",
            Description: "Execute a pipeline of tools. Provide pipeline JSON with steps, expected output, and risk level.",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "pipeline": {"type": "string", "description": "JSON pipeline definition"},
                    "input_data": {"type": "string", "description": "Input data (e.g., log file content)"}
                },
                "required": ["pipeline", "input_data"]
            }`),
        },
    },
}

// Agent generates tool call:
// execute_pipeline({
//     "pipeline": "{\"steps\":[...], \"risk_level\": \"safe\"}",
//     "input_data": "log content here"
// })
```

### Practical Patterns

**Tool Discovery via Tool Servers:**

In production, tools are often provided via [Tool Servers](../18-tool-protocols-and-servers/README.md). The catalog can be retrieved dynamically:

```go
// Tool Server provides ListTools()
toolServer := connectToToolServer("http://localhost:8080")
allTools, _ := toolServer.ListTools()

// Filter by task
relevantTools := filterToolsByQuery(allTools, userQuery, topK=5)
```

**Validation and Security:**

```go
func validatePipeline(pipeline Pipeline) error {
    // Check risk
    if pipeline.RiskLevel == "dangerous" {
        return fmt.Errorf("dangerous pipeline requires human approval")
    }
    
    // Check tool allowlist
    allowedTools := map[string]bool{
        "grep": true, "sort": true, "head": true,
        // rm, dd and other dangerous ones - NOT in allowlist
    }
    
    for _, step := range pipeline.Steps {
        if !allowedTools[step.Tool] {
            return fmt.Errorf("tool %s not allowed", step.Tool)
        }
    }
    
    return nil
}
```

**Observability:**

```go
// Log selected tools and reasons
log.Printf("Tool retrieval: query=%s, selected=%v, reason=%s",
    userQuery,
    []string{"grep", "sort", "head"},
    "matched tags: text-processing, filtering")

// Store pipeline JSON for audit
auditLog.StorePipeline(userID, pipelineJSON, result)
```

## Common Errors

### Error 1: Agent Doesn't Search Knowledge Base

**Symptom:** Agent performs actions without searching the knowledge base, using only general knowledge.

**Cause:** The System Prompt doesn't instruct the agent to search the knowledge base, or the search tool description isn't clear enough.

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

### Error 4: Passing All Tools to Context

**Symptom:** Agent receives a list of 1000+ tools, model performs worse, latency increases.

**Cause:** All tools are passed in `tools[]` without filtering.

**Solution:**
```go
// BAD: All tools
tools := getAllTools()  // 1000+ tools

// GOOD: Only relevant ones
userQuery := "find errors in logs"
relevantTools := searchToolCatalog(userQuery, topK=5)  // Only 5 relevant tools
tools := convertToOpenAITools(relevantTools)
```

### Error 5: Universal `run_shell` Without Control

**Symptom:** Agent uses a single `run_shell(command)` tool for all commands. No validation, no audit trail.

**Cause:** Simplifying architecture at the expense of security.

**Solution:**
```go
// BAD: Universal shell
tools := []openai.Tool{{
    Function: &openai.FunctionDefinition{
        Name: "run_shell",
        Description: "Execute any shell command",
    },
}}

// GOOD: Specific tools + pipeline DSL
tools := []openai.Tool{
    {Function: &openai.FunctionDefinition{Name: "grep", ...}},
    {Function: &openai.FunctionDefinition{Name: "sort", ...}},
    {Function: &openai.FunctionDefinition{Name: "execute_pipeline", ...}},
}

// Pipeline JSON is validated before execution
if err := validatePipeline(pipeline); err != nil {
    return err
}
```

### Error 6: No Pipeline Validation

**Symptom:** Agent generates a pipeline with dangerous commands (`rm -rf`, `dd`), which execute without checks.

**Cause:** No risk level check and allowlist validation before execution.

**Solution:**
```go
// GOOD: Validate before execution
func executePipeline(pipelineJSON string) error {
    var pipeline Pipeline
    json.Unmarshal([]byte(pipelineJSON), &pipeline)
    
    // Check risk
    if pipeline.RiskLevel == "dangerous" {
        return fmt.Errorf("dangerous pipeline requires confirmation")
    }
    
    // Check allowlist
    allowedTools := map[string]bool{"grep": true, "sort": true}
    for _, step := range pipeline.Steps {
        if !allowedTools[step.Tool] {
            return fmt.Errorf("tool %s not allowed", step.Tool)
        }
    }
    
    // Execute only after validation
    return runValidatedPipeline(pipeline)
}
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

### Exercise 3: Implement Tool Search

Implement a function to search for relevant tools in the catalog:

```go
func searchToolCatalog(query string, catalog []ToolDefinition, topK int) []ToolDefinition {
    // Search by description and tags
    // Return top-k most relevant tools
}
```

**Expected result:**
- Function finds tools relevant to the query
- Returns no more than topK tools
- Considers tool descriptions and tags

**Advanced:** Implement vector search for tools (similar to vector search for documents above). This is especially useful for large catalogs (1000+ tools).

### Exercise 4: Pipeline Validation

Implement a function to validate a pipeline before execution:

```go
func validatePipeline(pipeline Pipeline, allowedTools map[string]bool) error {
    // Check risk level
    // Check tool allowlist
    // Return error if pipeline is unsafe
}
```

**Expected result:**
- Function returns error for dangerous pipelines
- Function returns error if disallowed tools are used
- Function returns nil for safe pipelines

## Completion Criteria / Checklist

✅ **Completed:**
- Agent searches knowledge base before performing actions
- Search queries are specific and contain keywords
- Documents are split into appropriately sized chunks
- Search tool has clear description
- System Prompt instructs agent to use knowledge base
- For large tool spaces, tool retrieval is used (only relevant tools in context)
- Pipelines are validated before execution (risk level, allowlist)
- Dangerous operations require confirmation

❌ **Not completed:**
- Agent doesn't search knowledge base (uses only general knowledge)
- Search queries are too general (doesn't find needed information)
- Chunks are too large (don't fit in context)
- All tools are passed to context without filtering (1000+ tools)
- Universal `run_shell` is used without security controls
- Pipelines execute without validation

## Production Notes

When using RAG in production, consider:

- **Document versioning:** Track document versions and update date (`updated_at`). This helps understand which version was used in the response.
- **Freshness (currency):** Filter outdated documents (e.g., older than 30 days) before using in context.
- **Grounding:** Require agent to reference found documents in response. This reduces hallucinations and increases trust.

More on production readiness: [Chapter 19: Observability](../19-observability-and-tracing/README.md), [Chapter 23: Evals in CI/CD](../23-evals-in-cicd/README.md).

## Connection with Other Chapters

- **Tools:** How search tool integrates into agent, see [Chapter 03: Tools](../03-tools-and-function-calling/README.md). The problem of large tool lists is solved via tool retrieval (see "RAG for Action Space" section above).
- **Autonomy:** How RAG works in the agent loop, see [Chapter 04: Autonomy](../04-autonomy-and-loops/README.md)
- **Tool Servers:** How to retrieve tool catalogs dynamically via tool servers, see [Chapter 18: Tool Protocols](../18-tool-protocols-and-servers/README.md)

## What's Next?

After studying RAG, proceed to:
- **[07. Multi-Agent Systems](../07-multi-agent/README.md)** — how to create a team of agents

---

**Navigation:** [← Safety](../05-safety-and-hitl/README.md) | [Table of Contents](../README.md) | [Multi-Agent →](../07-multi-agent/README.md)
