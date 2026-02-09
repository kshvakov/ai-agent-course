# 06. RAG and Knowledge Base

## Why This Chapter?

A regular agent only knows what it was trained on (up to the cut-off date). It won't know your local instructions like "How to restart Phoenix server according to protocol #5" unless you provide them at runtime.

**RAG (Retrieval Augmented Generation)** is a "cheat sheet lookup" pattern. The agent first pulls relevant text from a knowledge base, then acts.

Without RAG, an agent can't reliably use your documentation, protocols, and knowledge base. With RAG, it can: fetch what's relevant and act according to your instructions.

### Real-World Case Study

**Situation:** A user writes: "Restart Phoenix server according to protocol"

**Problem:** The agent doesn't know the Phoenix server restart protocol. It may perform a standard restart that doesn't match your procedures.

**Solution:** With RAG, the agent looks up the protocol in the knowledge base before acting. It finds the document "Phoenix server restart protocol: 1. Turn off load balancer 2. Restart server 3. Turn on load balancer" and follows the steps.

## Theory in Simple Terms

### How Does RAG Work?

1. **Agent receives a request** from the user
2. **Agent searches for information** in the knowledge base via a search tool
3. **Knowledge base returns** relevant documents
4. **Agent uses the information** to perform the action

## How Does RAG Work? — Magic vs Reality

**Magic:**
> Agent "knows" it needs to search the knowledge base and finds the needed information on its own

**Reality:**

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
    Model:    "gpt-4o-mini",
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
    Model:    "gpt-4o-mini",
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

**Simple RAG ([Lab 07](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab07-rag)):**
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

## Advanced RAG Techniques

Basic RAG works like this: user query → search → retrieved documents → response. In practice, this isn't enough. The query can be vague, search can be imprecise, and results can be irrelevant.

Let's look at techniques that solve these problems.

### RAG Evolution

```
Basic RAG          Advanced RAG           Agentic RAG
┌──────────┐       ┌──────────────┐       ┌──────────────────┐
│ Query    │       │ Query        │       │ Agent decides:   │
│   ↓      │       │   ↓          │       │ - Search or not  │
│ Search   │       │ Transform    │       │ - Where to search│
│   ↓      │       │   ↓          │       │ - Is it enough   │
│ Retrieve │       │ Route        │       │ - Search more    │
│   ↓      │       │   ↓          │       │   ↓              │
│ Generate │       │ Hybrid Search│       │ Iterative        │
│          │       │   ↓          │       │ search loop      │
│          │       │ Rerank       │       │                  │
│          │       │   ↓          │       │                  │
│          │       │ Generate     │       │                  │
└──────────┘       └──────────────┘       └──────────────────┘
```

### Query Transformation

**Problem:** A user writes "server is down". Searching for this query won't find the document "Server failure recovery procedure".

**Solution:** Transform the query before searching — rephrase, expand, or break it into sub-queries.

**Technique 1: Query Rewriting**

```go
// Rewrite the query before search for better retrieval
func rewriteQuery(originalQuery string, client *openai.Client) (string, error) {
    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: "gpt-4o-mini", // Cheap model — simple task
        Messages: []openai.ChatCompletionMessage{
            {
                Role: openai.ChatMessageRoleSystem,
                Content: `Rewrite the user query to be more specific for document search.
Return ONLY the rewritten query, nothing else.
Examples:
- "server is down" → "server failure recovery procedure"
- "DB is slow" → "PostgreSQL performance diagnostics high latency"`,
            },
            {Role: openai.ChatMessageRoleUser, Content: originalQuery},
        },
        Temperature: 0,
    })
    if err != nil {
        return originalQuery, err // Fall back to original
    }
    return resp.Choices[0].Message.Content, nil
}
```

**Technique 2: Sub-Query Decomposition**

A complex query is broken into several simpler ones. Results are merged.

```go
// "Compare nginx config on prod and staging" breaks into:
// 1. "nginx configuration production"
// 2. "nginx configuration staging"
subQueries := decomposeQuery(userQuery)
var allResults []SearchResult
for _, sq := range subQueries {
    results := searchKnowledgeBase(sq)
    allResults = append(allResults, results...)
}
```

**Technique 3: HyDE (Hypothetical Document Embeddings)**

Instead of searching by the query, ask the model to generate a **hypothetical answer**. Then search for documents similar to that answer.

```go
// Step 1: Model generates a hypothetical document
hypothetical := generateHypotheticalAnswer(query)
// query: "how to set up SSL"
// hypothetical: "To set up SSL on nginx: 1. Obtain a certificate... 2. Add to config..."

// Step 2: Search for documents similar to the hypothetical answer
embedding := embedText(hypothetical)
results := vectorDB.Search(embedding, topK=5)
// Finds real documents about SSL setup, even if written differently
```

**Why this works:** The embedding of a hypothetical document is closer to the embedding of the real document than the embedding of a short query.

### Routing (Query Routing)

**Problem:** You have multiple data sources: a wiki, SQL database, monitoring API. The query "server response time for the last hour" won't be found in the wiki — you need the metrics database.

**Solution:** Classify the query and route it to the right source.

```go
type QueryRoute struct {
    Source   string // "wiki", "sql", "metrics_api", "vector_db"
    Query    string // Original or transformed query
    Reason   string // Why this source
}

func routeQuery(query string, client *openai.Client) (QueryRoute, error) {
    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: "gpt-4o-mini",
        Messages: []openai.ChatCompletionMessage{
            {
                Role: openai.ChatMessageRoleSystem,
                Content: `Classify the query and route to the correct data source.
Available sources:
- "wiki": documentation, procedures, runbooks
- "sql": structured data, tables, historical records
- "metrics_api": real-time metrics, monitoring data
- "vector_db": semantic search in knowledge base

Return JSON: {"source": "...", "query": "...", "reason": "..."}`,
            },
            {Role: openai.ChatMessageRoleUser, Content: query},
        },
        Temperature: 0,
    })
    if err != nil {
        return QueryRoute{Source: "vector_db", Query: query}, err
    }

    var route QueryRoute
    json.Unmarshal([]byte(resp.Choices[0].Message.Content), &route)
    return route, nil
}

// Usage:
route, _ := routeQuery("server response time for the last hour")
// route.Source = "metrics_api"
// route.Reason = "query about real-time metrics"
```

### Hybrid Search

**Problem:** Vector search finds meaning well but struggles with exact terms. Keyword search is the opposite. The query "error ORA-12154" needs both keyword search for "ORA-12154" and semantic search for "Oracle connection error".

**Solution:** Combine both approaches and merge results.

```go
type SearchResult struct {
    ChunkID string
    Text    string
    Score   float64
}

func hybridSearch(query string, topK int) []SearchResult {
    // 1. Keyword search (BM25)
    keywordResults := bm25Search(query, topK)

    // 2. Vector search (Semantic)
    embedding := embedQuery(query)
    vectorResults := vectorDB.Search(embedding, topK)

    // 3. Reciprocal Rank Fusion (RRF) — merge results
    return reciprocalRankFusion(keywordResults, vectorResults, topK)
}

// RRF: combines ranks from different lists
func reciprocalRankFusion(lists ...[]SearchResult) []SearchResult {
    scores := make(map[string]float64) // chunkID → combined score
    k := 60.0 // RRF constant (standard value)

    for _, list := range lists {
        for rank, result := range list {
            scores[result.ChunkID] += 1.0 / (k + float64(rank+1))
        }
    }

    // Sort by combined score (descending)
    // ... sort and return top-K ...
}
```

**When each approach works best:**

| Query Type | Keyword | Vector | Hybrid |
|------------|---------|--------|--------|
| "ORA-12154" | Excellent | Poor | Excellent |
| "can't connect to database" | Poor | Excellent | Excellent |
| "ORA-12154 won't connect" | Medium | Medium | Excellent |

### Reranking

**Problem:** Search returned 20 results. Not all are equally relevant. Passing all 20 into context wastes tokens.

**Solution:** After initial retrieval, rerank results using a more accurate (but slower) model.

```go
func rerankResults(query string, results []SearchResult, topK int) []SearchResult {
    // Use LLM to score relevance of each result
    type scored struct {
        Result SearchResult
        Score  float64
    }
    var scored []scored

    for _, r := range results {
        score := scoreRelevance(query, r.Text) // cross-encoder or LLM
        scored = append(scored, scored{Result: r, Score: score})
    }

    // Sort by score (descending)
    sort.Slice(scored, func(i, j int) bool {
        return scored[i].Score > scored[j].Score
    })

    // Return top-K
    result := make([]SearchResult, 0, topK)
    for i := 0; i < topK && i < len(scored); i++ {
        result = append(result, scored[i].Result)
    }
    return result
}
```

**Two-stage pipeline:** Fast retrieval (BM25/vector, top-100) → Precise reranking (cross-encoder, top-5).

### Self-RAG (Self-Assessment of Retrieval Quality)

**Problem:** The agent found documents, but they may be irrelevant, outdated, or incomplete. Basic RAG doesn't notice this.

**Solution:** The model assesses retrieved document quality and decides: answer, refine the query, or search more.

```go
type RetrievalAssessment struct {
    IsRelevant  bool   `json:"is_relevant"`  // Are documents relevant to the query?
    IsSufficient bool  `json:"is_sufficient"` // Is there enough information to answer?
    Action      string `json:"action"`        // "answer", "refine_query", "search_more"
    RefinedQuery string `json:"refined_query,omitempty"` // Refined query (if needed)
}

func assessRetrieval(query string, docs []SearchResult, client *openai.Client) (RetrievalAssessment, error) {
    docsText := formatDocs(docs)
    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: "gpt-4o-mini",
        Messages: []openai.ChatCompletionMessage{
            {
                Role: openai.ChatMessageRoleSystem,
                Content: `Assess if the retrieved documents are relevant and sufficient to answer the query.
Return JSON: {"is_relevant": bool, "is_sufficient": bool, "action": "answer"|"refine_query"|"search_more", "refined_query": "..."}`,
            },
            {
                Role: openai.ChatMessageRoleUser,
                Content: fmt.Sprintf("Query: %s\n\nRetrieved documents:\n%s", query, docsText),
            },
        },
        Temperature: 0,
    })
    if err != nil {
        return RetrievalAssessment{IsRelevant: true, IsSufficient: true, Action: "answer"}, err
    }

    var assessment RetrievalAssessment
    json.Unmarshal([]byte(resp.Choices[0].Message.Content), &assessment)
    return assessment, nil
}
```

**Self-RAG in a loop:**

```go
func selfRAG(query string, maxAttempts int) ([]SearchResult, error) {
    currentQuery := query

    for attempt := 0; attempt < maxAttempts; attempt++ {
        docs := hybridSearch(currentQuery, 10)
        assessment, _ := assessRetrieval(currentQuery, docs)

        switch assessment.Action {
        case "answer":
            return docs, nil // Documents are sufficient — answer
        case "refine_query":
            currentQuery = assessment.RefinedQuery // Refine the query
        case "search_more":
            // Expand search (more top-K or different source)
            moreDocs := hybridSearch(currentQuery, 20)
            docs = append(docs, moreDocs...)
            return docs, nil
        }
    }
    return nil, fmt.Errorf("could not find sufficient documents after %d attempts", maxAttempts)
}
```

### Agentic RAG (RAG as an Agent Tool)

**Problem:** Self-RAG assesses retrieval quality but can't make complex decisions: which source to use, whether to combine information from multiple documents, or solve multi-hop tasks (where the answer requires a chain of searches).

**Solution:** RAG is embedded into the Agent Loop (a.k.a. ReAct Loop — see [Chapter 04](../04-autonomy-and-loops/README.md)). The agent decides when to search, where to search, and whether it has enough information.

```go
// Agentic RAG: RAG is simply agent tools
tools := []openai.Tool{
    // Tool 1: Search documentation
    {
        Function: &openai.FunctionDefinition{
            Name:        "search_docs",
            Description: "Search documentation and runbooks. Use when you need procedures or technical details.",
            Parameters:  searchParamsSchema,
        },
    },
    // Tool 2: SQL query to metrics database
    {
        Function: &openai.FunctionDefinition{
            Name:        "query_metrics",
            Description: "Query metrics database. Use when you need historical data or statistics.",
            Parameters:  sqlParamsSchema,
        },
    },
    // Tool 3: Search similar incidents
    {
        Function: &openai.FunctionDefinition{
            Name:        "search_incidents",
            Description: "Search past incidents for similar issues and their resolutions.",
            Parameters:  searchParamsSchema,
        },
    },
}

// Agent decides on its own:
// Iteration 1: search_docs("nginx 502 error troubleshooting")
// Iteration 2: query_metrics("SELECT avg(latency) FROM requests WHERE status=502 AND time > now()-1h")
// Iteration 3: search_incidents("nginx 502 upstream timeout")
// Iteration 4: Final answer combining information from all sources
```

**Multi-hop RAG (search chains):**

```
Query: "Why did the payments service go down yesterday?"

Step 1: search_incidents("payments service outage yesterday")
       → "Incident INC-4521: payments went down due to DB timeout"

Step 2: search_docs("payments database connection configuration")
       → "Payments uses PostgreSQL on db-prod-03, connection pool = 20"

Step 3: query_metrics("SELECT connections FROM pg_stat WHERE time = yesterday")
       → "Peak connections: 150 (with limit of 100)"

Step 4: Final answer: "Payments went down due to connection pool exhaustion.
        Peak was 150 connections with a limit of 100. Recommendation: increase pool to 200."
```

**Differences between approaches:**

| Approach | Who makes decisions | Where's the logic |
|----------|-------------------|------------------|
| Basic RAG | Rigid pipeline | Code (hardcoded) |
| Advanced RAG | Pipeline with branching | Code + config |
| Self-RAG | Model assesses quality | Model (assessment) |
| Agentic RAG | Agent controls everything | Agent Loop |

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

**Simple search ([Lab 13](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab13-tool-retrieval)):**
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

**Completed:**
- [x] Agent searches knowledge base before performing actions
- [x] Search queries are specific and contain keywords
- [x] Documents are split into appropriately sized chunks
- [x] Search tool has clear description
- [x] System Prompt instructs agent to use knowledge base
- [x] For large tool spaces, tool retrieval is used (only relevant tools in context)
- [x] Pipelines are validated before execution (risk level, allowlist)
- [x] Dangerous operations require confirmation

**Not completed:**
- [ ] Agent doesn't search knowledge base (uses only general knowledge)
- [ ] Search queries are too general (doesn't find needed information)
- [ ] Chunks are too large (don't fit in context)
- [ ] All tools are passed to context without filtering (1000+ tools)
- [ ] Universal `run_shell` is used without security controls
- [ ] Pipelines execute without validation

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
- **MCP Resources:** MCP provides a standard mechanism for data access (Resources) — [Model Context Protocol](https://modelcontextprotocol.io/)

## What's Next?

After studying RAG, proceed to:
- **[07. Multi-Agent Systems](../07-multi-agent/README.md)** — how to create a team of agents

