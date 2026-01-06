# Manual: Lab 13 — Tool Retrieval & Pipeline Building

## Why This Lab?

In real-world scenarios, agents need to work with large tool spaces. Linux has thousands of commands, and they can be combined into infinite pipelines. You can't pass all tools to the model — it's inefficient and leads to worse tool selection.

**Tool RAG (Retrieval Augmented Generation for action space)** solves this by:
- Storing a catalog of tools with metadata
- Retrieving only relevant tools before planning
- Building pipelines for complex multi-step tasks

### Real-World Case Study

**Situation:** Agent needs to troubleshoot logs. User asks: "Find top 10 most frequent errors"

**Without Tool Retrieval:**
- Agent receives 1000+ tools in context
- Model struggles to choose (too many options)
- High latency (too many tokens)
- Risk of hallucinations (calling non-existent tools)

**With Tool Retrieval:**
- Agent searches catalog: "error filter sort"
- Gets 5 relevant tools: `[grep, sort, uniq, head]`
- Builds pipeline: `grep("ERROR") → sort() → uniq(-c) → head(10)`
- Executes pipeline safely

**Difference:** Tool retrieval filters the action space, making agent more efficient and safer.

## Theory in Simple Terms

### How Does Tool Retrieval Work?

1. **Tool Catalog:** Store metadata for each tool (name, description, tags, risk level)
2. **Search:** Before planning, search catalog for relevant tools
3. **Filter:** Return only top-k most relevant tools
4. **Execute:** Pass filtered tools to model, model builds pipeline

### Pipeline DSL

A pipeline is a JSON structure that describes:
- **Steps:** List of tool calls in sequence
- **Input/Output:** How data flows between steps
- **Risk Level:** Safety assessment

**Example Pipeline:**
```json
{
    "steps": [
        {"tool": "grep", "args": {"pattern": "ERROR"}},
        {"tool": "sort", "args": {}},
        {"tool": "head", "args": {"lines": 10}}
    ],
    "risk_level": "safe"
}
```

## Execution Algorithm

### Step 1: Create Tool Catalog

```go
type ToolDefinition struct {
    Name        string
    Description string
    Tags        []string  // "filter", "sort", "search"
    RiskLevel   string    // "safe", "moderate", "dangerous"
}

var toolCatalog = []ToolDefinition{
    {
        Name:        "grep",
        Description: "Search for patterns in text. Use for filtering lines.",
        Tags:        []string{"filter", "search", "text"},
        RiskLevel:   "safe",
    },
    {
        Name:        "sort",
        Description: "Sort lines of text alphabetically or numerically.",
        Tags:        []string{"sort", "order", "text"},
        RiskLevel:   "safe",
    },
    {
        Name:        "head",
        Description: "Show first N lines. Use for limiting output.",
        Tags:        []string{"limit", "filter", "text"},
        RiskLevel:   "safe",
    },
    // ... more tools
}
```

### Step 2: Implement Tool Search

```go
func searchToolCatalog(query string, topK int) []ToolDefinition {
    var results []ToolDefinition
    queryLower := strings.ToLower(query)
    
    // Simple keyword matching (in production - use embeddings)
    for _, tool := range toolCatalog {
        // Match in description
        if strings.Contains(strings.ToLower(tool.Description), queryLower) {
            results = append(results, tool)
            continue
        }
        // Match in tags
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

### Step 3: Pipeline Structure

```go
type PipelineStep struct {
    Tool string                 `json:"tool"`
    Args map[string]interface{} `json:"args"`
}

type Pipeline struct {
    Steps         []PipelineStep `json:"steps"`
    RiskLevel     string          `json:"risk_level"`
    ExpectedOutput string         `json:"expected_output,omitempty"`
}
```

### Step 4: Pipeline Execution

```go
func executePipeline(pipelineJSON string, inputData string) (string, error) {
    var pipeline Pipeline
    if err := json.Unmarshal([]byte(pipelineJSON), &pipeline); err != nil {
        return "", err
    }
    
    // Validate risk level
    if pipeline.RiskLevel == "dangerous" {
        return "", fmt.Errorf("dangerous pipeline requires confirmation")
    }
    
    // Execute steps sequentially
    currentData := inputData
    for i, step := range pipeline.Steps {
        result, err := executeToolStep(step.Tool, step.Args, currentData)
        if err != nil {
            return "", fmt.Errorf("step %d (%s) failed: %v", i, step.Tool, err)
        }
        currentData = result
    }
    
    return currentData, nil
}

func executeToolStep(toolName string, args map[string]interface{}, input string) (string, error) {
    switch toolName {
    case "grep":
        pattern := args["pattern"].(string)
        return grepPattern(input, pattern), nil
    case "sort":
        return sortLines(input), nil
    case "head":
        lines := int(args["lines"].(float64))
        return headLines(input, lines), nil
    default:
        return "", fmt.Errorf("unknown tool: %s", toolName)
    }
}
```

### Step 5: Agent Integration

```go
tools := []openai.Tool{
    {
        Function: &openai.FunctionDefinition{
            Name: "search_tool_catalog",
            Description: "Search tool catalog for relevant tools. Use this BEFORE building pipelines.",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "query": {"type": "string"},
                    "top_k": {"type": "number", "default": 5}
                },
                "required": ["query"]
            }`),
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name: "execute_pipeline",
            Description: "Execute a pipeline of tools. Provide pipeline JSON with steps and risk_level.",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "pipeline": {"type": "string"},
                    "input_data": {"type": "string"}
                },
                "required": ["pipeline", "input_data"]
            }`),
        },
    },
}
```

## Common Errors

### Error 1: Tool Search Returns Too Many Tools

**Symptom:** `search_tool_catalog` returns 50+ tools, defeating the purpose.

**Cause:** Search is too broad or topK is too large.

**Solution:**
```go
// ХОРОШО: Limit to top 5-10 most relevant
relevantTools := searchToolCatalog(query, topK=5)
```

### Error 2: Pipeline JSON Malformed

**Symptom:** `execute_pipeline` fails with JSON parsing error.

**Cause:** Model generates invalid JSON or missing required fields.

**Solution:**
```go
// ХОРОШО: Validate JSON before parsing
if !json.Valid([]byte(pipelineJSON)) {
    return "", fmt.Errorf("invalid JSON")
}

var pipeline Pipeline
if err := json.Unmarshal([]byte(pipelineJSON), &pipeline); err != nil {
    return "", fmt.Errorf("failed to parse pipeline: %v", err)
}

// Validate required fields
if len(pipeline.Steps) == 0 {
    return "", fmt.Errorf("pipeline has no steps")
}
```

### Error 3: Pipeline Steps Don't Chain

**Symptom:** Each step receives original input instead of previous step's output.

**Cause:** Not passing output from step N to step N+1.

**Solution:**
```go
// ХОРОШО: Chain steps correctly
currentData := inputData
for _, step := range pipeline.Steps {
    result, err := executeToolStep(step.Tool, step.Args, currentData)
    if err != nil {
        return "", err
    }
    currentData = result  // Use output as next input
}
```

### Error 4: No Risk Validation

**Symptom:** Dangerous pipelines execute without confirmation.

**Cause:** Not checking `risk_level` before execution.

**Solution:**
```go
// ХОРОШО: Validate risk before execution
if pipeline.RiskLevel == "dangerous" {
    return "", fmt.Errorf("dangerous pipeline requires human approval")
}
```

## Mini-Exercises

### Exercise 1: Improve Tool Search

Implement search that ranks tools by relevance (count matches in description + tags):

```go
func searchToolCatalog(query string, topK int) []ToolDefinition {
    // Score each tool by number of matches
    // Sort by score
    // Return top-k
}
```

### Exercise 2: Add Pipeline Validation

Implement validation that checks:
- All tools in steps exist in catalog
- Risk level is set
- Steps are not empty

```go
func validatePipeline(pipeline Pipeline, catalog []ToolDefinition) error {
    // Check tools exist
    // Check risk level
    // Check steps not empty
}
```

## Completion Criteria

✅ **Completed:**
- Tool catalog search finds relevant tools (top-k)
- Agent builds valid pipeline JSON
- Pipeline executes with correct step chaining
- Risk validation works
- Code compiles and works

❌ **Not completed:**
- Tool search returns all tools
- Pipeline JSON is malformed
- Steps don't chain (each gets original input)
- No risk validation
- Code doesn't compile

---

**Next step:** After successfully completing Lab 13, proceed to production topics or [Lab 14: Evals in CI](../lab14-evals-in-ci/README.md) (if available)

