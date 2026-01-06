# Lab 13: Tool Retrieval & Pipeline Building (Optional)

## Goal
Teach the agent to search for relevant tools from a large catalog and build pipelines (combinations of tools) for complex tasks. Implement tool retrieval and pipeline execution.

## Theory

### Problem: Infinite Tool Space

In Linux, there are thousands of commands (`grep`, `awk`, `sed`, `jq`, `sort`, `uniq`, `head`, `tail`, etc.) that can be combined into pipelines. You can't pass all of them to the agent — it's too many tokens, and the model performs worse with large tool lists.

**Solution:** Tool RAG (Retrieval Augmented Generation for action space):
1. Store a **tool catalog** with metadata (name, description, tags, risk level)
2. Before planning, **retrieve top-k relevant tools** based on the task
3. Pass only relevant tools to the model
4. For complex tasks, build **pipelines** (JSON DSL) that combine multiple tools

### How Does Tool Retrieval Work?

1. **Task:** "Find top 10 errors in logs"
2. **Agent thought:** "I need tools for filtering, sorting, and limiting"
3. **Action:** `search_tool_catalog("error log filter sort", top_k=5)`
4. **Result:** Returns relevant tools: `[grep, sort, head, ...]`
5. **Agent thought:** "I'll build a pipeline: grep → sort → head"
6. **Action:** `execute_pipeline(pipeline_json)`

### Pipeline JSON DSL

A pipeline is a formalized plan that describes:
- **Steps:** Sequence of tool calls
- **Input/Output:** Data flow between steps
- **Expected Output:** What the pipeline should produce
- **Risk Level:** Safety assessment

## Task

In `main.go` implement tool retrieval and pipeline execution.

### Part 1: Tool Catalog

Create a tool catalog with sample Linux-like commands:

```go
type ToolDefinition struct {
    Name        string
    Description string
    Tags        []string
    RiskLevel   string
}

var toolCatalog = []ToolDefinition{
    {Name: "grep", Description: "Search for patterns in text", Tags: []string{"filter", "search"}},
    {Name: "sort", Description: "Sort lines of text", Tags: []string{"sort"}},
    // ... more tools
}
```

### Part 2: Tool Search

Implement `searchToolCatalog(query string, topK int) []ToolDefinition`, which:
- Searches tools by description and tags (simple keyword matching)
- Returns top-k most relevant tools

### Part 3: Pipeline Execution

Implement `executePipeline(pipelineJSON string, inputData string) (string, error)`, which:
- Parses pipeline JSON
- Validates risk level (reject "dangerous" pipelines)
- Executes steps sequentially
- Returns final result

### Part 4: Agent Integration

1. Add tool `search_tool_catalog` to agent's tools
2. Add tool `execute_pipeline` to agent's tools
3. Configure System Prompt to use tool retrieval before building pipelines
4. Implement agent loop

### Test Scenario

Run agent with prompt: *"Find top 5 error lines from the logs, sorted by frequency"*

**Expected:**
- Agent calls `search_tool_catalog("error filter sort")`
- Gets relevant tools: `[grep, sort, uniq, head]`
- Builds pipeline JSON: `grep("ERROR") → sort() → uniq(-c) → head(5)`
- Calls `execute_pipeline(pipeline_json, log_data)`
- Returns top 5 error lines

## Important

- Tool retrieval should return only relevant tools (not all 100+ tools)
- Pipeline JSON must be validated before execution
- Dangerous pipelines should be rejected
- Pipeline steps execute sequentially (each step's output is next step's input)

## Completion Criteria

✅ **Completed:**
- Tool catalog search finds relevant tools
- Agent builds pipeline JSON from multiple tools
- Pipeline executes correctly with sequential steps
- Dangerous pipelines are rejected
- Code compiles and works

❌ **Not completed:**
- Tool search doesn't work
- Pipeline JSON is malformed
- Pipeline execution fails
- No validation of dangerous operations

---

**Note:** This is an optional lab. After Lab 12, you can proceed to production topics or complete this lab for deeper understanding of tool retrieval.

**Next step:** After successfully completing Lab 13, proceed to production topics or [Lab 14: Evals in CI](../lab14-evals-in-ci/README.md) (if available)

