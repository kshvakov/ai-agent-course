# Solution: Lab 13 — Tool Retrieval & Pipeline Building

## Complete Implementation

Here's the complete solution with all TODOs implemented:

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// ... (toolCatalog, sampleLogs, types remain the same) ...

// searchToolCatalog implementation
func searchToolCatalog(query string, topK int) []ToolDefinition {
	var results []ToolDefinition
	queryLower := strings.ToLower(query)
	queryWords := strings.Fields(queryLower)

	// Score each tool by relevance
	type scoredTool struct {
		tool  ToolDefinition
		score int
	}
	var scored []scoredTool

	for _, tool := range toolCatalog {
		score := 0
		toolDescLower := strings.ToLower(tool.Description)

		// Count matches in description
		for _, word := range queryWords {
			if strings.Contains(toolDescLower, word) {
				score += 2 // Description matches are more important
			}
		}

		// Count matches in tags
		for _, tag := range tool.Tags {
			tagLower := strings.ToLower(tag)
			for _, word := range queryWords {
				if strings.Contains(tagLower, word) {
					score += 1
				}
			}
		}

		if score > 0 {
			scored = append(scored, scoredTool{tool: tool, score: score})
		}
	}

	// Sort by score (descending)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Return top-k
	if len(scored) > topK {
		scored = scored[:topK]
	}

	results = make([]ToolDefinition, len(scored))
	for i, s := range scored {
		results[i] = s.tool
	}

	return results
}

// Tool execution implementations
func executeGrep(input string, pattern string) string {
	lines := strings.Split(input, "\n")
	var result []string
	for _, line := range lines {
		if strings.Contains(line, pattern) {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

func executeSort(input string) string {
	lines := strings.Split(input, "\n")
	// Remove empty lines
	var nonEmpty []string
	for _, line := range lines {
		if line != "" {
			nonEmpty = append(nonEmpty, line)
		}
	}
	sort.Strings(nonEmpty)
	return strings.Join(nonEmpty, "\n")
}

func executeHead(input string, lines int) string {
	allLines := strings.Split(input, "\n")
	if lines > len(allLines) {
		lines = len(allLines)
	}
	return strings.Join(allLines[:lines], "\n")
}

func executeUniq(input string, count bool) string {
	lines := strings.Split(input, "\n")
	// Remove empty lines
	var nonEmpty []string
	for _, line := range lines {
		if line != "" {
			nonEmpty = append(nonEmpty, line)
		}
	}

	if count {
		// Count occurrences (like uniq -c)
		counts := make(map[string]int)
		for _, line := range nonEmpty {
			counts[line]++
		}

		var result []string
		for line, cnt := range counts {
			result = append(result, fmt.Sprintf("%d %s", cnt, line))
		}
		// Sort by count (descending)
		sort.Slice(result, func(i, j int) bool {
			return result[i] > result[j]
		})
		return strings.Join(result, "\n")
	} else {
		// Simple deduplication
		seen := make(map[string]bool)
		var result []string
		for _, line := range nonEmpty {
			if !seen[line] {
				seen[line] = true
				result = append(result, line)
			}
		}
		return strings.Join(result, "\n")
	}
}

// executeToolStep remains the same
func executeToolStep(toolName string, args map[string]interface{}, input string) (string, error) {
	switch toolName {
	case "grep":
		pattern, ok := args["pattern"].(string)
		if !ok {
			return "", fmt.Errorf("grep requires 'pattern' argument")
		}
		return executeGrep(input, pattern), nil
	case "sort":
		return executeSort(input), nil
	case "head":
		lines, ok := args["lines"].(float64)
		if !ok {
			return "", fmt.Errorf("head requires 'lines' argument")
		}
		return executeHead(input, int(lines)), nil
	case "uniq":
		count := false
		if c, ok := args["count"].(bool); ok {
			count = c
		}
		return executeUniq(input, count), nil
	default:
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}
}

// executePipeline implementation
func executePipeline(pipelineJSON string, inputData string) (string, error) {
	// Validate JSON
	if !json.Valid([]byte(pipelineJSON)) {
		return "", fmt.Errorf("invalid JSON")
	}

	// Parse JSON
	var pipeline Pipeline
	if err := json.Unmarshal([]byte(pipelineJSON), &pipeline); err != nil {
		return "", fmt.Errorf("failed to parse pipeline: %v", err)
	}

	// Validate risk level
	if pipeline.RiskLevel == "dangerous" {
		return "", fmt.Errorf("dangerous pipeline requires human approval")
	}

	// Validate steps
	if len(pipeline.Steps) == 0 {
		return "", fmt.Errorf("pipeline has no steps")
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

// main remains the same
```

## Key Points

1. **Tool Search:** Scores tools by matching query words in description (weight 2) and tags (weight 1), then returns top-k sorted by score.

2. **Tool Execution:** Each tool operates on string data (simulating Linux commands):
   - `grep`: Filters lines containing pattern
   - `sort`: Sorts lines alphabetically
   - `head`: Returns first N lines
   - `uniq`: Deduplicates (with optional counting)

3. **Pipeline Execution:** 
   - Validates JSON and required fields
   - Rejects dangerous pipelines
   - Executes steps sequentially (each output becomes next input)

4. **Agent Flow:**
   - Agent calls `search_tool_catalog("error filter sort")`
   - Gets relevant tools: `[grep, sort, uniq, head]`
   - Builds pipeline JSON
   - Calls `execute_pipeline(pipeline_json, log_data)`
   - Returns result

## Expected Output

For query "Find top 5 most frequent error lines":
- Pipeline: `grep("ERROR") → uniq(count=true) → sort() → head(5)`
- Result: Top 5 error lines sorted by frequency

