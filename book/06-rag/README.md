# 06. RAG and Knowledge Base

## Why does this matter?

An ordinary agent only knows what it was taught during training (up to its cut-off date). Your local instructions, like "how to restart the Phoenix server per runbook #5", are not something it knows on its own.

**RAG (Retrieval Augmented Generation)** is a way to "peek at a cheat sheet". The agent first finds the relevant piece in the knowledge base, and only then acts.

Without RAG the agent cannot rely on your documentation, runbooks and knowledge base. With RAG it can: it finds the right material and follows the steps according to your rules.

### A real-world case

**Situation:** The user writes: "Restart the Phoenix server according to the runbook"

**Problem:** The agent doesn't know the Phoenix server restart runbook. It may perform a standard restart that doesn't match your procedures.

**Solution:** With RAG, the agent first finds the runbook in the knowledge base and only then acts. It retrieves the document "Phoenix server restart runbook: 1. Shut down the load balancer 2. Restart the server 3. Bring the load balancer back up" and follows the steps.

## The theory in plain words

### How does RAG work?

1. **The agent receives a request** from the user
2. **The agent searches for information** in the knowledge base via a search tool
3. **The knowledge base returns** relevant documents
4. **The agent uses the information** to perform the action

## How Does RAG Work? — Magic vs Reality

**Magic:**
> The agent "knows" what to search for in the knowledge base and finds the right information by itself

**Reality:**

### The full RAG protocol

**Step 1: The user's request**

```go
messages := []Message{
    {Role: "system", Content: "You are a DevOps assistant. Internal procedures and protocols live in the knowledge base."},
    {Role: "user", Content: "Перезагрузи сервер Phoenix согласно регламенту"},
}
```

**Step 2: The model sees the description of the search tool**

```go
tools := []ToolDef{
    {
        Name:        "search_knowledge_base",
        Description: "Search the knowledge base for documentation, protocols, and procedures.",
        Parameters: json.RawMessage(`{
            "type": "object",
            "properties": {
                "query": {"type": "string", "description": "Search query"}
            },
            "required": ["query"]
        }`),
    },
    {
        Name:        "restart_server",
        Description: "Restart a server",
        Parameters: json.RawMessage(`{
            "type": "object",
            "properties": {
                "hostname": {"type": "string"}
            },
            "required": ["hostname"]
        }`),
    },
}
```

**Step 3: The model generates a tool call to search**

```go
client, err := NewClient(...) // provider from the config (ch. 04)

resp1, err := client.Complete(ctx, Request{
    Model:       "glm-5.3",
    Messages:    messages,
    Tools:       tools,
    Temperature: 0.2, // quality: less variance in tool selection
}) // the error is handled by the loop (ch. 04)

msg1 := resp1.Choices[0].Message
// msg1.ToolCalls = [{
//     ID: "call_1",
//     Name: "search_knowledge_base",
//     Arguments: "{\"query\": \"Phoenix restart protocol\"}"
// }]
```

**Why did the model generate a tool call to search?**
- The model **decided on its own** that for the word "runbook" it needs internal instructions, not general knowledge
- The tool's Description says where to look for documentation and procedures
- The model sees the word "runbook" in the request and links it to the search tool

There is no coercive line in the prompt like "Always search knowledge base before actions": the search is an ordinary tool call, and the decision of "whether to search or not" is made by the model based on the tool description. If in your task the search is always mandatory — that's a runtime invariant (the code does not perform the action without a search), not a line in the system prompt.

**Step 4: The runtime (your code) performs the search**

> **Note:** The runtime is the agent code that you write in Go. See [Chapter 00: Preface](../00-preface/README.md#runtime) for the definition.

```go
func searchKnowledgeBase(query string) string {
    // Simple keyword search (in production - vector search)
    knowledgeBase := map[string]string{
        "protocols.txt": "Регламент перезагрузки сервера Phoenix:\n1. Выключить балансировщик\n2. Перезагрузить сервер\n3. Включить балансировщик",
    }
    
    for filename, content := range knowledgeBase {
        if strings.Contains(strings.ToLower(content), strings.ToLower(query)) {
            return fmt.Sprintf("File: %s\nContent: %s", filename, content)
        }
    }
    return "No documents found"
}

result1 := searchKnowledgeBase("Phoenix restart protocol")
// result1 = "File: protocols.txt\nContent: Регламент перезагрузки сервера Phoenix:\n1. Выключить балансировщик..."
```

**Step 5: The search result is added to the context**

```go
messages = append(messages, Message{
    Role:       "tool",
    Content:    result1,  // The full retrieved documentation!
    ToolCallID: msg1.ToolCalls[0].ID,
})
// Now messages contains:
// [system, user, assistant(tool_call: search_kb), tool("File: protocols.txt\nContent: ...")]
```

**Step 6: The model sees the documentation and acts**

```go
// Send the updated history (with the documentation!) to the model
resp2, err := client.Complete(ctx, Request{
    Model:    "glm-5.3",
    Messages: messages,  // The model sees the retrieved documentation!
    Tools:    tools,
}) // the error is handled by the loop (ch. 04)

msg2 := resp2.Choices[0].Message
// The model sees in its context:
// "Регламент перезагрузки сервера Phoenix:\n1. Выключить балансировщик..."
// The model generates tool calls according to the runbook:

// msg2.ToolCalls = [
//     {ID: "call_1", Name: "restart_server", Arguments: "{\"hostname\": \"phoenix\"}"},
//     // Or first shut down the load balancer, then the server
// ]
```

**What actually happens:**

1. **The model doesn't "know" the runbook** — it sees it in its context after the search
2. **Search is just an ordinary tool** — the same as `ping` or `restart_service`
3. **The search result is appended to `messages[]`** — the model sees it as a new message
4. **The model generates actions based on the context** — it sees the documentation and follows it

**The essence:** RAG is not "knowledge out of thin air" — it's a way to add relevant information to the model's context through an ordinary tool call.

## Simple RAG vs Vector Search

Simple RAG (keyword search) is implemented in [Lab 07](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab07-rag), revised to the neutral client. Production uses **vector search** (Semantic Search), which searches by meaning rather than by words.

**Simple RAG ([Lab 07](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab07-rag)):**
```go
// Search by substring occurrence
if strings.Contains(content, query) {
    return content
}
```

**Vector search (production):**
```go
// 1. Documents are split into chunks and converted to vectors
// by the provider's embedding model (dimensionality d depends on the model)
chunks := []Chunk{
    {ID: "chunk_1", Text: "Регламент перезагрузки Phoenix...", Embedding: embed("Регламент перезагрузки Phoenix...")},
    {ID: "chunk_2", Text: "Шаг 2: Выключить балансировщик...", Embedding: embed("Шаг 2: Выключить балансировщик...")},
}

// 2. The user's query is also converted to a vector
queryEmbedding := embed("Phoenix restart protocol")

// 3. Search for similar vectors by cosine distance
similarDocs := vectorDB.Search(queryEmbedding, topK=3)
// Returns the 3 most semantically similar chunks (not by words!)

// 4. The result is added to the model's context just like in simple RAG
result := formatChunks(similarDocs)  // "Chunk 1: ...\nChunk 2: ...\nChunk 3: ..."
messages = append(messages, Message{
    Role:    "tool",
    Content: result,
})
```

**Why vector search is better:**
- Searches by **meaning**, not by words
- Finds "restart Phoenix" even if the document says "перезагрузка сервера Phoenix"
- Works with synonyms and different phrasings

## Chunking

Documents are split into chunks (pieces) for efficient retrieval.

**Example:**
```
Document: "Регламент перезагрузки сервера Phoenix..."
Chunk 1: "Регламент перезагрузки сервера Phoenix: шаг 1..."
Chunk 2: "Шаг 2: Выключить балансировщик..."
Chunk 3: "Шаг 3: Перезагрузить сервер..."
```

## Advanced RAG Techniques

Basic RAG works like this: user query → search → retrieved documents → answer. In practice, this is not enough. The query may be vague, the search imprecise, and the results irrelevant.

Let's look at techniques that solve these problems.

### The evolution of RAG

```
Basic RAG          Advanced RAG           Agentic RAG
┌──────────┐       ┌──────────────┐       ┌─────────────────┐
│ Query    │       │ Query        │       │ Agent decides:  │
│   ↓      │       │   ↓          │       │ - Search or not │
│ Search   │       │ Transform    │       │ - Where to look │
│   ↓      │       │   ↓          │       │ - Enough info?  │
│ Retrieve │       │ Route        │       │ - Search again  │
│   ↓      │       │   ↓          │       │   ↓             │
│ Generate │       │ Hybrid Search│       │ Iterative       │
│          │       │   ↓          │       │ search in a loop│
│          │       │ Rerank       │       │                 │
│          │       │   ↓          │       │                 │
│          │       │ Generate     │       │                 │
└──────────┘       └──────────────┘       └─────────────────┘
```

### Pre- and post-query processing: when it's worth it

Between the user's query and the search (and between the search and the answer) you can insert extra LLM calls: rephrase the query, generate a hypothetical document, pick a source, judge the relevance of what was found. These are working techniques — but each one carries the same price tag: **an extra LLM call before or after the search**. It adds latency and tokens to every request, even when the basic search would have coped on its own.

| Technique | What it does | When it's justified |
|-------|-----------|----------------|
| **Query Rewriting** | The LLM reformulates the query into the language of the documents: "the server is down" → "server recovery procedure after a failure" | User queries are conversational, documents are formal; search often misses |
| **Sub-Query Decomposition** | A complex query is split into several simple ones: "compare nginx settings on prod and staging" → two independent searches | Queries aggregate several independent questions |
| **HyDE** (Hypothetical Document Embeddings) | The LLM generates a hypothetical answer, and the search is run on its embedding: the embedding of a document is closer to the embedding of another document than the embedding of a short query | Short queries against long canonical documents; rare terminology |
| **Routing** | Classifying the query and choosing a source: wiki, SQL database, metrics API | Several genuinely different sources that can't all be queried |
| **Self-RAG** | The model evaluates the relevance and sufficiency of what was found: answer / refine the query / search again | A noisy corpus where a search miss costs more than an extra iteration |

**General rule:** turn on a technique when it gives you a cheap win specifically on your corpus (verify with evals — [ch. 23](../23-evals-in-cicd/README.md)), not "because the articles say so". Escalation order: first fix the search itself (chunking, hybrid, reranking below), and only then add LLM scaffolding around it.

In agentic mode (the section below), some of these techniques come for free: the model reformulates the query itself, picks the source itself and decides on its own whether to search again — because for it, search is an ordinary tool call, not a pipeline step hardcoded in code.

### Hybrid search

**Problem:** Vector search is good at searching by meaning but poor at exact terms. Keyword search is the opposite. The query "ORA-12154 error" needs to be searched both by the keyword "ORA-12154" and by the meaning "Oracle connection error".

**Solution:** Combine both approaches and merge the results.

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

    // 3. Reciprocal Rank Fusion (RRF) — merging the results
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
    // ... sorting and returning top-K ...
}
```

**When each approach works better:**

| Query type | Keyword | Vector | Hybrid |
|-------------|---------|--------|--------|
| "ORA-12154" | Excellent | Poor | Excellent |
| "can't connect to the database" | Poor | Excellent | Excellent |
| "ORA-12154 not connecting" | Fair | Fair | Excellent |

### Reranking

**Problem:** The search returned 20 results. They are not all equally relevant. Passing all 20 into the context is a waste of tokens.

**Solution:** After the initial search, rerank the results with a more accurate (but slower) model.

```go
func rerankResults(query string, results []SearchResult, topK int) []SearchResult {
    // Use an LLM to score the relevance of each result
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

**The two-stage pipeline:** Fast search (BM25/vector, top-100) → Accurate reranking (cross-encoder, top-5).

### Agentic RAG (RAG as an agent tool)

**Problem:** The pre- and post-processing from the table above cannot make complex decisions: which source to use, whether information from several documents needs to be combined, or solve a multi-hop task (the answer requires a chain of searches). Each such decision there is a pipeline step hardcoded in code.

**Solution:** RAG is embedded into the Agent Loop (a.k.a. the ReAct Loop — see [Chapter 04](../04-autonomy-and-loops/README.md)). The agent itself decides when to search, where to search and whether the information is sufficient.

```go
// Agentic RAG: RAG is just the agent's tools
tools := []ToolDef{
    // Tool 1: Search in documentation
    {
        Name:        "search_docs",
        Description: "Search documentation and runbooks. Use when you need procedures or technical details.",
        Parameters:  searchParamsSchema,
    },
    // Tool 2: SQL query against the metrics database
    {
        Name:        "query_metrics",
        Description: "Query metrics database. Use when you need historical data or statistics.",
        Parameters:  sqlParamsSchema,
    },
    // Tool 3: Search for similar incidents
    {
        Name:        "search_incidents",
        Description: "Search past incidents for similar issues and their resolutions.",
        Parameters:  searchParamsSchema,
    },
}

// The agent decides on its own:
// Iteration 1: search_docs("nginx 502 error troubleshooting")
// Iteration 2: query_metrics("SELECT avg(latency) FROM requests WHERE status=502 AND time > now()-1h")
// Iteration 3: search_incidents("nginx 502 upstream timeout")
// Iteration 4: Final answer combining information from all sources
```

**Multi-hop RAG (a chain of searches):**

```
Query: "Why did the payments service go down yesterday?"

Step 1: search_incidents("payments service outage yesterday")
       → "Incident INC-4521: payments went down due to a database timeout"

Step 2: search_docs("payments database connection configuration")
       → "Payments uses PostgreSQL on db-prod-03, connection pool = 20"

Step 3: query_metrics("SELECT connections FROM pg_stat WHERE time = yesterday")
       → "Connection peak: 150 (limit is 100)"

Step 4: Final answer: "Payments went down because the connection pool was exhausted.
        The peak was 150 connections against a limit of 100. Recommendation: increase the pool to 200."
```

**The difference between the approaches:**

| Approach | Who makes the decisions | Where the logic lives |
|--------|----------------------|-----------|
| Basic RAG | Rigid pipeline | Code (hardcoded) |
| Advanced RAG | Pipeline with branches | Code + config |
| Self-RAG | The model evaluates quality | Model (assessment) |
| Agentic RAG | The agent runs everything | Agent loop |

## Many Tools ≠ You Need a Catalog of Them

So far we've talked about RAG for **documents** (runbooks, procedures). A frequent next thought is: "why not apply RAG to the action space?" — after all, an agent potentially has an endless number of commands and tools. Let's look at why a tool catalog with retrieval over it is almost always over-engineering, and what the right form is.

### The problem: "infinite" commands

**Situation:** The agent has to work with Linux commands for troubleshooting. Linux has thousands of commands (`grep`, `awk`, `sed`, `jq`, `sort`, `uniq`, `head`, `tail`, etc.), and they combine into pipelines.

**The problem:**
- You can't put all the commands into `tools[]` — that's thousands of tokens in every request
- The model chooses worse from a large list (more hallucinations)
- There's no safety control (which commands are dangerous?)

**The naive solution (doesn't work):** one universal `run_shell(command)` tool with no restrictions. No validation (`rm -rf /` can be executed), no audit (it's unclear which commands were run), no control.

### Anti-pattern: a catalog of tool wrappers and tool RAG over shell commands

The first idea: build a **tool catalog** — one wrapper per command (`grep`, `sort`, `head`...) with descriptions and tags — and retrieve the relevant ones before planning. This is called Tool RAG (action-space retrieval), and for shell commands it's over-engineering:

- **A command already knows how to be found.** The knowledge that "filtering logs requires `grep | sort | head`" lives in documentation (README, skill) and in the model's own knowledge — not in a catalog of schemas. Retrieval over command descriptions solves a problem that doesn't exist.
- **Retrieval doesn't check correctness.** Searching the catalog returns commands "similar in description," but doesn't guarantee the command fits or that its arguments are correct. Correctness is still the runtime's responsibility (validation, allowlist) — the safety system doesn't change, and a retrieval layer is added on top as a second subsystem that has to be kept in sync with the actual commands and run through evals.
- **A catalog of 20+ wrappers over one system** (shell, kubectl, docker) is not a "rich action space" — it's one `exec` tool duplicated by twenty schemas, each paying for itself in every request.

A special case of this anti-pattern is **JSON DSL pipelines**: the model generates a JSON plan of steps (`grep` → `sort` → `head`), and the runtime executes it. It looks "more reliable," but in practice it's a second language for describing actions that needs to be validated, debugged, and maintained — instead of ordinary tool calls, which the model already generates well.

### The right form: skills + exec with an allowlist

When there are many actions, their **description** moves out of `tools[]` into files, and **execution** goes into a single `exec` tool with an allowlist. This is the **skills** mechanism ([ch. 09](../09-agent-architecture/README.md)): one `SKILL.md` per area of knowledge + progressive disclosure.

1. **Discovery.** At startup, the runtime reads all `SKILL.md` files and adds one line per skill to the system prompt: name + `description`. 10 skills ≈ 10 lines — that's the entire cost of discovery.
2. **Activation.** When the model decides it needs a skill, it reads the body of `SKILL.md` itself — via the ordinary `read` tool. The text enters the history as a tool_result.
3. **Execution.** The model acts on the skill's instructions through `exec` — with a command allowlist and confirmation of dangerous operations.

**Example: the "k8s-debug" skill**

```
skills/k8s-debug/SKILL.md
---
name: k8s-debug
description: Debug Kubernetes pods — check status, logs, describe.
  Use when user reports pod crashes or restart loops.
---
1. kubectl get pods -n <namespace> — find the problem pod
2. kubectl describe pod <pod> -n <namespace> — events and restart causes
3. kubectl logs <pod> -n <namespace> --tail=100 — the latest logs
Dangerous operations (delete, rollout undo) — only with confirmation.
```

The system prompt has a single line: `k8s-debug: Debug Kubernetes pods...`. From there the model works on its own: `read("skills/k8s-debug/SKILL.md")` → `exec("kubectl", "get", "pods", "-n", "prod")` → `exec("kubectl", "logs", ...)`. No catalog of hundreds of tool wrappers and no retrieval over it — yet the model sees all the diagnostic steps and executes them through a single controlled tool:

```go
// exec with an allowlist — the single "executor" of commands
func execCommand(command string, args ...string) (string, error) {
    allowed := []string{"kubectl", "grep", "sort", "head", "tail", "jq"} // allowlist, not a denylist
    if !slices.Contains(allowed, command) {
        return "", fmt.Errorf("command %q is not in the allowlist", command)
    }
    cmd := exec.Command(command, args...)
    // ... execute, log for audit, cap the output size
    return string(output), nil
}
```

> **How the real SDK does it.** Tool catalogs with retrieval are rare precisely because skills displaced them: one line per skill in the system prompt instead of hundreds of schemas in `tools[]`. The agent's main "retrieval" tool is `read`: the model reads the file it needs when it needs it. Reading is constrained by an FSHooks policy: which directories can be read, what gets logged, and what enters the context — that's decided by the runtime, not the model.

### When there really are many tools: tool retrieval as a necessary evil

There are cases where a tool really is a separate tool with its own schema: dozens of SaaS integrations, hundreds of endpoints behind tool servers ([ch. 18](../18-tool-protocols-and-servers/README.md)). Then dynamically selecting a subset of tools before a request (tool retrieval) is a justified **forced** measure. Remember the cost: the `tools[]` block is sent and paid for in **every** request ([ch. 03](../03-tools-and-function-calling/README.md)); retrieval shrinks the block but doesn't eliminate the catalog, which still needs to be versioned and checked with evals. A reference implementation — [Lab 13](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab13-tool-retrieval).

## Common Mistakes

### Mistake 1: The agent doesn't search the knowledge base

**Symptom:** The agent performs actions without searching the knowledge base, relying only on general knowledge.

**Cause:** The search tool's description doesn't tell the model **what** it contains: it's unclear which tasks require a search.

**Fix:** Don't force it via the prompt ("Always search knowledge base before actions" is an anti-pattern: the line is either ignored or makes the model search even where it isn't needed). Let the model make the decision itself — via an accurate tool description:

```go
// GOOD: the description says WHAT is in the knowledge base — the model decides on its own when it's needed
Description: "Search the internal knowledge base: runbooks, procedures, protocols. Use when the task involves internal systems or company-specific procedures."

// BAD: forcing it via the prompt
systemPrompt := "... Always search knowledge base before performing actions ..."
```

If a search is **always** mandatory in your task (for example, an action cannot be performed without a runbook) — that's a runtime invariant, not a line in the prompt: the code doesn't perform the action until the history contains a search result.

### Mistake 2: A poor search query

**Symptom:** The agent can't find the information it needs in the knowledge base.

**Cause:** The search query is too generic or doesn't contain keywords from the document.

**Fix:**
```go
// BAD: too generic a query
query := "server"

// GOOD: a specific query with keywords
query := "Phoenix server restart protocol"
```

### Mistake 3: Chunks too large

**Symptom:** Search returns documents that are too large and don't fit in the context.

**Cause:** Chunk sizes are too large (larger than the context window).

**Fix:**
```go
// GOOD: chunk size ~500-1000 tokens
chunkSize := 500  // tokens
```

### Mistake 4: A catalog of 20+ tool wrappers over one system

**Symptom:** The agent has dozens of `grep`/`sort`/`head`/`tail` tools — one wrapper per command. The `tools[]` block is huge, the model gets confused, and every request gets more expensive.

**Cause:** The action space is confused with the tool space: one system (shell) is wrapped by twenty schemas.

**Fix:** Move the description of actions into skills (`SKILL.md` + progressive disclosure) and execution into a single `exec` with an allowlist (the "Many Tools ≠ You Need a Catalog of Them" section above). A tool catalog with retrieval only if the tools really are different and numerous (SaaS integrations).

### Mistake 5: A universal `run_shell` with no control

**Symptom:** The agent uses a single `run_shell(command)` tool for all commands. No validation, no audit.

**Cause:** Simplifying the architecture at the expense of safety.

**Fix:**
```go
// BAD: a universal shell with no restrictions
tools := []ToolDef{{
    Name:        "run_shell",
    Description: "Execute any shell command",
}}

// GOOD: exec with an allowlist (not a denylist!) and confirmation of dangerous operations
func execCommand(command string, args ...string) (string, error) {
    allowed := map[string]bool{
        "grep": true, "sort": true, "head": true, "kubectl": true,
        // rm, dd and other dangerous ones are NOT in the allowlist
    }
    if !allowed[command] {
        return "", fmt.Errorf("command %q is not in the allowlist", command)
    }
    // ... execute, log for audit
    return string(output), nil
}
```

### Mistake 6: Dangerous operations without confirmation

**Symptom:** The agent executes destructive commands (`rm`, `kubectl delete`) without checking and human confirmation.

**Cause:** "On average" validation doesn't distinguish dangerous calls from safe ones.

**Fix:** The allowlist answers "is it allowed at all," while confirmation ([ch. 05](../05-safety-and-hitl/README.md)) answers "is it allowed right now, with a human in the loop":

```go
// GOOD: two separate checks before execution
if !allowedCommands[cmd.Command] {
    return fmt.Errorf("command %q is not allowed", cmd.Command)
}
if destructive(cmd.Command, cmd.Args) { // delete, drop, rollout undo...
    return confirmWithHuman(cmd) // without confirmation, it doesn't run
}
return run(cmd)
```

## Exercises

### Exercise 1: Implement simple search

Implement a simple keyword search function:

```go
func searchKnowledgeBase(query string) string {
    // Simple keyword search
    // Return the relevant documents
}
```

**Expected result:**
- The function finds documents containing keywords from the query
- The function returns the first N relevant documents

### Exercise 2: Implement chunking

Implement a function that splits a document into chunks:

```go
func chunkDocument(text string, chunkSize int) []string {
    // Split the document into chunks of chunkSize tokens
    // Return the list of chunks
}
```

**Expected result:**
- The function splits the document into chunks of the given size
- The chunks don't overlap (or overlap minimally)

### Exercise 3: Implement search over the skill catalog

The runtime reads all `SKILL.md` files and needs to pick the relevant ones. Implement a search function:

```go
type SkillDef struct {
    Name        string // "k8s-debug"
    Description string // "Debug Kubernetes pods — check status, logs, describe..."
}

func searchSkills(query string, skills []SkillDef, topK int) []SkillDef {
    // Search over the descriptions
    // Return the top-k most relevant skills
}
```

**Expected result:**
- The function finds skills relevant to the query
- It returns at most topK skills
- It takes skill descriptions into account

**For advanced readers:** Implement vector search over the descriptions (similar to the vector document search above) — useful when there are dozens of skills. The same mechanism underlies tool retrieval for really large sets of genuinely different tools ([Lab 13](https://github.com/kshvakov/ai-agent-course/tree/main/labs/lab13-tool-retrieval)).

### Exercise 4: Validate an exec command

Implement a function that validates a command before execution:

```go
func validateCommand(command string, args []string, allowed map[string]bool) error {
    // Check that command is in the allowlist
    // Return an error if the command is not allowed
}
```

**Expected result:**
- The function returns an error for commands outside the allowlist
- The function returns nil for allowed commands

## Acceptance Criteria / Checklist

**Passing:**
- [x] The agent searches the knowledge base when the task requires internal procedures
- [x] Search queries are specific and contain keywords
- [x] Documents are split into chunks of a suitable size
- [x] The search tool has a clear description (what's in the base — the model decides on its own, without an "Always search" line in the prompt)
- [x] A large action space is organized via skills (`SKILL.md` + progressive disclosure) and `exec` with an allowlist; tool retrieval only for genuinely different and numerous tools
- [x] Commands pass the allowlist before execution; dangerous operations require confirmation
- [x] The search result fits within the tool-result budget (~128 KB); chunks are stored with path/ID so the full document can be moved to an artifact

**Not passing:**
- [ ] The agent doesn't search the knowledge base (relies only on general knowledge)
- [ ] Search queries are too generic (the needed information isn't found)
- [ ] Chunks are too large (they don't fit in the context)
- [ ] A catalog of 20+ tool wrappers over one system instead of skills and `exec`
- [ ] A universal `run_shell` is used without safety controls
- [ ] Commands are executed without an allowlist; dangerous ones without confirmation

## Production Notes

When using RAG in production, keep in mind:

- **Document versioning:** Track document versions and update dates (`updated_at`). This helps understand which version was used in an answer.
- **Freshness:** Filter out stale documents (for example, older than 30 days) before putting them into the context.
- **Grounding:** Require the agent to cite the retrieved documents in its answer. This reduces hallucinations and increases trust.
- **Read permissions:** `read` is the main tool for accessing knowledge, and it must be constrained by a policy: which directories can be read, what gets logged, and what enters the context — that's decided by the runtime (FSHooks policy), not the model.
- **Tool-result budget:** A search result is an ordinary tool result: the guideline is ~128 KB per result ([ch. 13](../13-context-engineering/README.md)). What doesn't fit — an excerpt in the context + the full document as an artifact ([ch. 11](../11-state-management/README.md)); for that, store chunks with `path`/`ID`.

More on production readiness: [Chapter 19: Observability](../19-observability-and-tracing/README.md), [Chapter 23: Evals in CI/CD](../23-evals-in-cicd/README.md).

## Relation to Other Chapters

- **Tools:** How the search tool integrates into the agent — see [Chapter 03: Tools](../03-tools-and-function-calling/README.md). A large action space is organized via skills and `exec` (the "Many Tools ≠ You Need a Catalog of Them" section above; skills — [ch. 09](../09-agent-architecture/README.md)).
- **Autonomy:** How RAG works inside the agent loop — see [Chapter 04: Autonomy](../04-autonomy-and-loops/README.md)
- **Tool Servers:** How to get tools dynamically via tool servers — see [Chapter 18: Tool Protocols](../18-tool-protocols-and-servers/README.md)
- **MCP Resources:** MCP provides a standard mechanism for accessing data (Resources) — [Model Context Protocol](https://modelcontextprotocol.io/)

## What's Next?

After studying RAG, move on to:
- **[07. Multi-Agent Systems](../07-multi-agent/README.md)** — how to build a team of agents
