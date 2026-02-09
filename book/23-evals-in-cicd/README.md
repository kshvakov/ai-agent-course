# 23. Evals in CI/CD

## Why This Chapter?

You changed a prompt or code, and the agent got worse. But you only find out after deploying. Without evals in CI/CD, bad changes reach production.

In [Chapter 08](../08-evals-and-reliability/README.md) we wrote tests for the agent. Now we integrate them into a CI/CD pipeline and add a four-level evaluation system.

### Real-World Case Study

**Situation:** You updated the system prompt and deployed. A day later, users complain the agent picks wrong tools.

**Problem:** Evals only checked "was the task completed" (Task Level). They didn't check "was the right tool selected" (Tool Level).

**Solution:** A four-level eval system in CI/CD: Task → Tool → Trajectory → Topic. Quality gates block deployment when any level degrades.

## Theory in Simple Terms

### Four-Level Evaluation System

A single "pass/fail" metric is not enough. The agent may complete the task but do it inefficiently (extra tool calls), unsafely (bypassed checks), or incorrectly (right answer by accident).

| Level | What It Evaluates | Example Metric |
|-------|-------------------|----------------|
| **Task Level** | Was the task completed correctly? | Pass rate, answer correctness |
| **Tool Level** | Was the right tool selected? Are arguments valid? | Tool selection accuracy, argument validity |
| **Trajectory Level** | Was the execution path optimal? | Step count, unnecessary tool calls, loops |
| **Topic Level** | Quality in a specific domain | Domain-specific metrics (e.g., SQL validity) |

### Quality Gates

A Quality Gate is a check that blocks deployment when metrics degrade. Each level has its own threshold.

## How It Works (Step by Step)

### Step 1: Eval Case Structure with Levels

```go
type EvalCase struct {
    ID       string `json:"id"`
    Input    string `json:"input"`   // User query
    Topic    string `json:"topic"`   // Domain: "devops", "database", "security"

    // Task Level
    ExpectedOutput   string   `json:"expected_output"`    // Expected final answer (or pattern)
    MustContain      []string `json:"must_contain"`       // Strings that must appear in the answer

    // Tool Level
    ExpectedTools    []string `json:"expected_tools"`     // Which tools should be called
    ForbiddenTools   []string `json:"forbidden_tools"`    // Which tools must NOT be called
    ExpectedArgs     map[string]json.RawMessage `json:"expected_args"` // Expected arguments

    // Trajectory Level
    MaxSteps         int      `json:"max_steps"`          // Maximum number of steps
    MustNotLoop      bool     `json:"must_not_loop"`      // Must not enter a loop
}
```

### Step 2: Recording the Execution Trajectory

To evaluate at all levels, you need to record the agent's full path:

```go
type AgentTrajectory struct {
    RunID    string          `json:"run_id"`
    Steps    []TrajectoryStep `json:"steps"`
    Duration time.Duration   `json:"duration"`
    Tokens   int             `json:"tokens"`
}

type TrajectoryStep struct {
    Iteration int    `json:"iteration"`
    Type      string `json:"type"` // "tool_call", "tool_result", "final_answer"
    ToolName  string `json:"tool_name,omitempty"`
    ToolArgs  string `json:"tool_args,omitempty"`
    Result    string `json:"result,omitempty"`
}

// Record trajectory inside the agent loop
func runAgentWithTracing(input string, tools []openai.Tool) (string, AgentTrajectory) {
    var trajectory AgentTrajectory
    trajectory.RunID = generateRunID()

    for i := 0; i < maxIterations; i++ {
        resp, _ := client.CreateChatCompletion(ctx, req)
        msg := resp.Choices[0].Message

        if len(msg.ToolCalls) == 0 {
            trajectory.Steps = append(trajectory.Steps, TrajectoryStep{
                Iteration: i, Type: "final_answer", Result: msg.Content,
            })
            return msg.Content, trajectory
        }

        for _, tc := range msg.ToolCalls {
            trajectory.Steps = append(trajectory.Steps, TrajectoryStep{
                Iteration: i, Type: "tool_call",
                ToolName: tc.Function.Name, ToolArgs: tc.Function.Arguments,
            })
            result := executeTool(tc)
            trajectory.Steps = append(trajectory.Steps, TrajectoryStep{
                Iteration: i, Type: "tool_result",
                ToolName: tc.Function.Name, Result: result,
            })
        }
    }
    return "", trajectory
}
```

### Step 3: Evaluation at Four Levels

```go
type EvalResult struct {
    CaseID string `json:"case_id"`

    // Task Level
    TaskPass     bool    `json:"task_pass"`
    TaskScore    float64 `json:"task_score"`    // 0.0 - 1.0

    // Tool Level
    ToolPass     bool    `json:"tool_pass"`
    ToolAccuracy float64 `json:"tool_accuracy"` // % of correct tool calls

    // Trajectory Level
    TrajectoryPass bool  `json:"trajectory_pass"`
    StepCount      int   `json:"step_count"`
    HasLoops       bool  `json:"has_loops"`

    // Topic Level
    TopicPass    bool    `json:"topic_pass"`
    TopicScore   float64 `json:"topic_score"`
}

func evaluateCase(c EvalCase, answer string, traj AgentTrajectory) EvalResult {
    result := EvalResult{CaseID: c.ID}

    // --- Task Level ---
    result.TaskPass = checkTaskCompletion(c, answer)
    result.TaskScore = scoreAnswer(c.ExpectedOutput, answer)

    // --- Tool Level ---
    usedTools := extractToolNames(traj)
    result.ToolAccuracy = toolSelectionAccuracy(c.ExpectedTools, usedTools)
    result.ToolPass = result.ToolAccuracy >= 0.8 && !containsForbidden(usedTools, c.ForbiddenTools)

    // --- Trajectory Level ---
    result.StepCount = len(traj.Steps)
    result.HasLoops = detectLoops(traj)
    result.TrajectoryPass = result.StepCount <= c.MaxSteps && !result.HasLoops

    // --- Topic Level ---
    result.TopicPass, result.TopicScore = evaluateTopic(c.Topic, answer, traj)

    return result
}
```

**Tool Level — checking tool selection:**

```go
func toolSelectionAccuracy(expected, actual []string) float64 {
    if len(expected) == 0 {
        return 1.0
    }
    matches := 0
    for _, exp := range expected {
        for _, act := range actual {
            if exp == act {
                matches++
                break
            }
        }
    }
    return float64(matches) / float64(len(expected))
}

func containsForbidden(used, forbidden []string) bool {
    for _, f := range forbidden {
        for _, u := range used {
            if f == u {
                return true // Forbidden tool was used
            }
        }
    }
    return false
}
```

**Trajectory Level — loop detection:**

```go
func detectLoops(traj AgentTrajectory) bool {
    // If the same sequence of tool calls repeats 3+ times, it's a loop
    var calls []string
    for _, step := range traj.Steps {
        if step.Type == "tool_call" {
            calls = append(calls, step.ToolName+":"+step.ToolArgs)
        }
    }

    windowSize := 3
    for i := 0; i <= len(calls)-windowSize*2; i++ {
        pattern := strings.Join(calls[i:i+windowSize], "|")
        next := strings.Join(calls[i+windowSize:min(i+windowSize*2, len(calls))], "|")
        if pattern == next {
            return true
        }
    }
    return false
}
```

### Step 4: Multi-turn Evaluation

Evaluate multi-step dialogues where the agent goes through several rounds of interaction:

```go
type MultiTurnCase struct {
    ID    string      `json:"id"`
    Turns []TurnCase  `json:"turns"`
}

type TurnCase struct {
    UserInput      string   `json:"user_input"`
    ExpectedAction string   `json:"expected_action"` // "tool_call" or "text_response"
    ExpectedTools  []string `json:"expected_tools,omitempty"`
    MustContain    []string `json:"must_contain,omitempty"`
}

func evaluateMultiTurn(mtc MultiTurnCase, client *openai.Client) (float64, error) {
    var messages []openai.ChatCompletionMessage
    passedTurns := 0

    for _, turn := range mtc.Turns {
        messages = append(messages, openai.ChatCompletionMessage{
            Role: openai.ChatMessageRoleUser, Content: turn.UserInput,
        })

        resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
            Model:    model,
            Messages: messages,
            Tools:    tools,
        })
        if err != nil {
            return 0, err
        }

        msg := resp.Choices[0].Message
        messages = append(messages, msg)

        // Check expectations for this turn
        if turn.ExpectedAction == "tool_call" && len(msg.ToolCalls) > 0 {
            passedTurns++
        } else if turn.ExpectedAction == "text_response" && len(msg.ToolCalls) == 0 {
            passedTurns++
        }

        // Execute tool calls if any
        for _, tc := range msg.ToolCalls {
            result := executeTool(tc)
            messages = append(messages, openai.ChatCompletionMessage{
                Role: openai.ChatMessageRoleTool, Content: result, ToolCallID: tc.ID,
            })
        }
    }

    return float64(passedTurns) / float64(len(mtc.Turns)), nil
}
```

### Step 5: RAGAS Metrics for RAG

If your agent uses RAG, you need specialized metrics.

> **Note:** Below is a simplified Go implementation of RAGAS metrics. The real [RAGAS](https://docs.ragas.io/) is a Python library where `isRelevant`, `scoreFaithfulness`, and `scoreRelevance` are implemented via LLM-based evaluation. This example shows the metric structure, not a production implementation.

```go
// RAGAS (Retrieval Augmented Generation Assessment)
type RAGASMetrics struct {
    ContextPrecision float64 `json:"context_precision"` // What fraction of retrieved docs is relevant
    ContextRecall    float64 `json:"context_recall"`    // What fraction of needed docs was found
    Faithfulness     float64 `json:"faithfulness"`      // Answer is grounded in retrieved docs (not hallucinated)
    AnswerRelevance  float64 `json:"answer_relevance"`  // Answer is relevant to the question
}

func evaluateRAGAS(query, answer string, retrievedDocs, groundTruthDocs []string,
    client *openai.Client) RAGASMetrics {

    metrics := RAGASMetrics{}

    // Context Precision: what fraction of retrieved documents is relevant?
    relevantCount := 0
    for _, doc := range retrievedDocs {
        if isRelevant(query, doc, client) {
            relevantCount++
        }
    }
    if len(retrievedDocs) > 0 {
        metrics.ContextPrecision = float64(relevantCount) / float64(len(retrievedDocs))
    }

    // Context Recall: what fraction of needed documents was found?
    foundCount := 0
    for _, gtDoc := range groundTruthDocs {
        for _, retDoc := range retrievedDocs {
            if isSameContent(gtDoc, retDoc) {
                foundCount++
                break
            }
        }
    }
    if len(groundTruthDocs) > 0 {
        metrics.ContextRecall = float64(foundCount) / float64(len(groundTruthDocs))
    }

    // Faithfulness: is the answer grounded in documents, not hallucinated?
    metrics.Faithfulness = scoreFaithfulness(answer, retrievedDocs, client)

    // Answer Relevance: is the answer relevant to the question?
    metrics.AnswerRelevance = scoreRelevance(query, answer, client)

    return metrics
}
```

### Step 6: Quality Gates in CI/CD

```yaml
# .github/workflows/evals.yml
name: Agent Evals
on: [pull_request]
jobs:
  evals:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Run four-level evals
        run: go run cmd/evals/main.go --output=results.json

      - name: Check quality gates
        run: |
          # Parse results
          TASK_PASS=$(jq '.task_pass_rate' results.json)
          TOOL_ACCURACY=$(jq '.tool_accuracy' results.json)
          TRAJECTORY_PASS=$(jq '.trajectory_pass_rate' results.json)
          TOPIC_SCORE=$(jq '.topic_avg_score' results.json)

          echo "Task Pass Rate: $TASK_PASS"
          echo "Tool Accuracy: $TOOL_ACCURACY"
          echo "Trajectory Pass Rate: $TRAJECTORY_PASS"
          echo "Topic Score: $TOPIC_SCORE"

          # Quality gates per level
          FAILED=0
          if (( $(echo "$TASK_PASS < 0.95" | bc -l) )); then
            echo "FAIL: Task pass rate $TASK_PASS < 0.95"
            FAILED=1
          fi
          if (( $(echo "$TOOL_ACCURACY < 0.90" | bc -l) )); then
            echo "FAIL: Tool accuracy $TOOL_ACCURACY < 0.90"
            FAILED=1
          fi
          if (( $(echo "$TRAJECTORY_PASS < 0.85" | bc -l) )); then
            echo "FAIL: Trajectory pass rate $TRAJECTORY_PASS < 0.85"
            FAILED=1
          fi

          if [ "$FAILED" -eq 1 ]; then
            echo "Quality gates FAILED"
            exit 1
          fi
          echo "All quality gates PASSED"
```

```yaml
# .gitlab-ci.yml
stages:
  - evals

agent-evals:
  stage: evals
  image: golang:1.22
  script:
    - go run cmd/evals/main.go --output=results.json
    - |
      TASK_PASS=$(jq '.task_pass_rate' results.json)
      TOOL_ACCURACY=$(jq '.tool_accuracy' results.json)
      echo "Task: $TASK_PASS, Tool: $TOOL_ACCURACY"
      if (( $(echo "$TASK_PASS < 0.95" | bc -l) )); then
        echo "Quality gate failed"
        exit 1
      fi
  only:
    - merge_requests
  artifacts:
    paths:
      - results.json
```

### Step 7: Continuous Evaluation (in Production)

Evals in CI/CD catch problems before deployment. But models get updated and data changes. You need evaluation in production too:

```go
// Background process: runs evals on real data periodically
func continuousEval(interval time.Duration) {
    ticker := time.NewTicker(interval)
    for range ticker.C {
        // Sample from recent runs
        recentRuns := getRecentRuns(100)
        results := evaluateRuns(recentRuns)

        // Check thresholds
        if results.TaskPassRate < 0.90 {
            alert("Task pass rate dropped to %.2f", results.TaskPassRate)
        }
        if results.ToolAccuracy < 0.85 {
            alert("Tool accuracy dropped to %.2f", results.ToolAccuracy)
        }

        // Record metrics for the dashboard
        metrics.Record("eval.task_pass_rate", results.TaskPassRate)
        metrics.Record("eval.tool_accuracy", results.ToolAccuracy)
    }
}
```

### Step 8: Dataset Versioning

Eval datasets are versioned too:

```go
type EvalDataset struct {
    Version   string     `json:"version"`
    CreatedAt time.Time  `json:"created_at"`
    Cases     []EvalCase `json:"cases"`
}

// Dataset is stored in Git alongside the code
// testdata/evals/v1.0.json
// testdata/evals/v1.1.json (new edge cases added)
```

## Minimal Code Example

A minimal eval runner for CI/CD:

```go
package main

import (
    "encoding/json"
    "fmt"
    "os"
)

type EvalSummary struct {
    TaskPassRate      float64 `json:"task_pass_rate"`
    ToolAccuracy      float64 `json:"tool_accuracy"`
    TrajectoryPassRate float64 `json:"trajectory_pass_rate"`
    TopicAvgScore     float64 `json:"topic_avg_score"`
}

func main() {
    dataset := loadDataset("testdata/evals/latest.json")
    var results []EvalResult

    for _, c := range dataset.Cases {
        answer, traj := runAgentWithTracing(c.Input, tools)
        result := evaluateCase(c, answer, traj)
        results = append(results, result)
    }

    summary := summarize(results)

    // Output for CI/CD
    out, _ := json.MarshalIndent(summary, "", "  ")
    os.WriteFile("results.json", out, 0644)
    fmt.Printf("Task: %.2f, Tool: %.2f, Trajectory: %.2f\n",
        summary.TaskPassRate, summary.ToolAccuracy, summary.TrajectoryPassRate)

    // Quality gate
    if summary.TaskPassRate < 0.95 || summary.ToolAccuracy < 0.90 {
        fmt.Println("FAILED: Quality gates not met")
        os.Exit(1)
    }
    fmt.Println("PASSED: All quality gates met")
}
```

## Common Errors

### Error 1: Task Level Evals Only

**Symptom:** The agent passes tests, but in production it picks wrong tools or takes unnecessary steps.

**Cause:** You only check the final answer, not the path to it.

**Solution:**
```go
// BAD: Only "is the answer correct?"
if answer == expected { pass++ }

// GOOD: Four levels of evaluation
result := evaluateCase(c, answer, trajectory)
// Checks task + tool + trajectory + topic
```

### Error 2: Evals Without Trajectory Recording

**Symptom:** A test failed, but you can't tell which step went wrong.

**Cause:** No execution trajectory is recorded.

**Solution:**
```go
// BAD: Run the agent, check only the answer
answer := runAgent(input)

// GOOD: Record the trajectory
answer, trajectory := runAgentWithTracing(input, tools)
// Now you see every step: which tool, which args, which result
```

### Error 3: Rigid Thresholds for All Levels

**Symptom:** CI/CD keeps failing due to flaky evals at the Trajectory Level.

**Cause:** Same strict threshold for all levels. Trajectory Level is inherently unstable — the model can choose different paths to the same result.

**Solution:**
```go
// BAD: Same 0.95 threshold for everything
taskThreshold := 0.95
toolThreshold := 0.95
trajectoryThreshold := 0.95 // Too strict for trajectory!

// GOOD: Different thresholds per level
taskThreshold := 0.95       // Task must be completed
toolThreshold := 0.90       // Correct tool selection
trajectoryThreshold := 0.80 // Path can vary
```

### Error 4: No RAGAS Metrics for RAG Agents

**Symptom:** The RAG agent retrieves irrelevant documents, but evals don't catch it (only the answer is checked).

**Cause:** No evaluation of retrieval quality.

**Solution:**
```go
// BAD: Only check the RAG agent's final answer
if answerCorrect { pass++ }

// GOOD: Check both retrieval and the answer
ragasMetrics := evaluateRAGAS(query, answer, retrievedDocs, groundTruthDocs, client)
if ragasMetrics.Faithfulness < 0.8 {
    log.Printf("Low faithfulness: agent may be hallucinating")
}
```

### Error 5: Evals Only in CI/CD, Not in Production

**Symptom:** Evals pass in CI/CD, but production quality degrades (model updated, data changed).

**Cause:** No continuous evaluation.

**Solution:** Run evals in production too (on a sample of real requests).

## Mini-Exercises

### Exercise 1: Write a Tool Level Eval

Write an eval case that verifies the agent calls `check_status` (not `restart_service`) for the query "What is the server status?":

```go
testCase := EvalCase{
    Input:          "What is the status of server web-01?",
    ExpectedTools:  []string{"check_status"},
    ForbiddenTools: []string{"restart_service"},
    // ...
}
```

### Exercise 2: Implement Loop Detection

Implement the `detectLoops` function for Trajectory Level:

```go
func detectLoops(trajectory AgentTrajectory) bool {
    // Your code: check whether tool calls repeat
}
```

### Exercise 3: Implement a Multi-turn Eval

Write a test where the agent must first check the status, then — if the service is down — restart it:

```go
multiTurnCase := MultiTurnCase{
    Turns: []TurnCase{
        {UserInput: "Check nginx", ExpectedTools: []string{"check_status"}},
        {UserInput: "Service is down, restart it", ExpectedTools: []string{"restart_service"}},
    },
}
```

## Completion Criteria / Checklist

**Completed:**
- [x] Evals integrated into CI/CD pipeline
- [x] Quality gates block deployment when metrics degrade
- [x] Evaluation at four levels (Task, Tool, Trajectory, Topic)
- [x] Execution trajectory is recorded for analysis
- [x] RAG agents have RAGAS metrics
- [x] Eval datasets are versioned

**Not completed:**
- [ ] Evals not integrated in CI/CD
- [ ] Only the final answer is checked (no Tool/Trajectory Level)
- [ ] No trajectory recording (impossible to debug failures)
- [ ] RAG agents are evaluated only by the final answer

## Connection with Other Chapters

- **[Chapter 08: Evals and Reliability](../08-evals-and-reliability/README.md)** — basic eval concepts
- **[Chapter 06: RAG](../06-rag/README.md)** — RAGAS metrics for RAG agents
- **[Chapter 19: Observability and Tracing](../19-observability-and-tracing/README.md)** — tracing tied to evals
- **[Chapter 22: Prompt and Program Management](../22-prompt-program-management/README.md)** — testing prompts

## What's Next?

After learning about evals in CI/CD, move on to:
- **[24. Data and Privacy](../24-data-and-privacy/README.md)** — data protection and privacy