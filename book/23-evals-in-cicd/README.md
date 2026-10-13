# 23. Evals in CI/CD

## Why does this matter?

You changed a prompt or some code, and the agent got worse. But you only find out after deployment. Without evals in CI/CD, bad changes reach production.

In [Chapter 08](../08-evals-and-reliability/README.md) we wrote tests for the agent. Now we'll wire them into a CI/CD pipeline and add a four-level evaluation system.

### A real-world case

**Situation:** You updated the system prompt and deployed. A day later users complain that the agent picks the wrong tools.

**Problem:** Evals only checked "task completed" (Task Level). They didn't check "was the right tool selected" (Tool Level).

**Solution:** A four-level eval system in CI/CD: Task → Tool → Trajectory → Topic. Quality gates block deployment on any level's regression.

## The theory in plain words

### The four-level evaluation system

A single "pass/fail" metric isn't enough. An agent can complete a task but do it inefficiently (extra tool calls), unsafely (bypassed checks), or incorrectly (right answer by accident).

| Level | What it evaluates | Example metric |
|-------|-------------------|----------------|
| **Task Level** | Was the task completed correctly? | Pass rate, answer correctness |
| **Tool Level** | Was the right tool selected? Are the arguments valid? | Tool selection accuracy, argument validity |
| **Trajectory Level** | Is the execution path optimal? | Step count, unnecessary tool calls, loops |
| **Topic Level** | Quality in a specific domain | Domain-specific metrics (e.g., SQL validity) |

### Quality gates

A Quality Gate is a check that blocks deployment when metrics degrade. Each level has its own threshold.

## How it works (step by step)

### Step 1: Eval case structure with levels

```go
type EvalCase struct {
    ID       string `json:"id"`
    Input    string `json:"input"`   // User request
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
    MustNotLoop      bool     `json:"must_not_loop"`      // Must not loop
}
```

### Step 2: The execution trajectory is RunResult

No need to invent a second trajectory structure. The agent loop already returns **RunResult** ([ch. 04](../04-autonomy-and-loops/README.md)): iterations, tool calls with arguments and results, usage. [Chapter 08](../08-evals-and-reliability/README.md) builds basic evals on top of it, [chapter 19](../19-observability-and-tracing/README.md) traces the same events. The eval runner simply reads the ready-made trajectory:

```go
// Eval wrapper over the canonical RunResult (ch. 04): production fields
// (Iterations, Usage, Output) plus eval specifics — RunID for correlation
// with the trace (ch. 19), run latency, and tool call records
type RunResult struct {
    RunID      string        `json:"run_id"`
    Iterations int           `json:"iterations"`
    ToolCalls  []ToolCallRec `json:"tool_calls"`
    Usage      Usage         `json:"usage"` // prompt/completion tokens
    LatencyMs  float64       `json:"latency_ms"`
    Output     string        `json:"output"`
    Err        error         `json:"-"` // populated from RunResult.Err (ch. 04)
}

type ToolCallRec struct {
    Iteration int    `json:"iteration"`
    ToolName  string `json:"tool_name"`
    ToolArgs  string `json:"tool_args"`
    Result    string `json:"result"`
}

// runEvalCase runs the agent with the same Run as production and fills
// the eval wrapper from the canonical RunResult; call records and latency
// come from Config.On events (ToolEndEvent — ch. 09) and timing
func runEvalCase(ctx context.Context, input string) (string, RunResult) {
    start := time.Now()

    cfg := evalConfig(input)
    var rec RunResult
    rec.RunID = "eval-" + time.Now().UTC().Format("20060102T150405.000000000") // trace correlation (ch. 19)
    cfg.On = func(ev Event) {
        if end, ok := ev.(ToolEndEvent); ok {
            recTool := ToolCallRec{
                ToolName: end.Call.Name, ToolArgs: end.Call.Arguments,
                Iteration: len(rec.ToolCalls) + 1,
            }
            // A failing tool is no panic: the error goes into the call record
            if end.Err != nil || end.Res == nil {
                recTool.Result = "tool error"
            } else {
                recTool.Result = end.Res.Content
            }
            rec.ToolCalls = append(rec.ToolCalls, recTool)
        }
    }

    res := Run(ctx, cfg)
    rec.Output, rec.Iterations = res.Output, res.Iterations
    rec.Usage = res.Usage
    rec.Err = res.Err // terminal run error: a failed run must not look successful
    rec.LatencyMs = float64(time.Since(start).Milliseconds())

    return rec.Output, rec
}
```

### Step 3: Evaluation on four levels

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

func evaluateCase(c EvalCase, answer string, res RunResult) EvalResult {
    result := EvalResult{CaseID: c.ID}

    // Terminal run error (ch. 04) — the case failed, no scores are computed
    if res.Err != nil {
        result.TaskPass, result.ToolPass, result.TrajectoryPass = false, false, false
        result.TaskScore = 0
        return result
    }

    // --- Task Level ---
    result.TaskPass = checkTaskCompletion(c, answer)
    result.TaskScore = scoreAnswer(c.ExpectedOutput, answer)

    // --- Tool Level ---
    usedTools := extractToolNames(res)
    result.ToolAccuracy = toolSelectionAccuracy(c.ExpectedTools, usedTools)
    result.ToolPass = result.ToolAccuracy >= 0.8 && !containsForbidden(usedTools, c.ForbiddenTools)

    // --- Trajectory Level ---
    result.StepCount = res.Iterations
    result.HasLoops = detectLoops(res)
    result.TrajectoryPass = result.StepCount <= c.MaxSteps && !result.HasLoops

    // --- Topic Level ---
    result.TopicPass, result.TopicScore = evaluateTopic(c.Topic, answer, res)

    return result
}
```

**Tool Level — tool selection checks:**

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
                return true // A forbidden tool was used
            }
        }
    }
    return false
}
```

**Trajectory Level — loop detection:**

```go
func detectLoops(res RunResult) bool {
    // If the same sequence of tool calls repeats 3+ times — it's a loop
    var calls []string
    for _, tc := range res.ToolCalls {
        calls = append(calls, tc.ToolName+":"+tc.ToolArgs)
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

### Step 4: Multi-turn evaluation

Evaluating multi-step dialogs where the agent goes through several rounds of conversation:

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

func evaluateMultiTurn(mtc MultiTurnCase, client *llm.Client) (float64, error) {
    var messages []llm.Message
    passedTurns := 0

    for _, turn := range mtc.Turns {
        messages = append(messages, llm.Message{
            Role: "user", Content: turn.UserInput,
        })

        resp, err := client.Complete(ctx, llm.Request{
            Model:    model,
            Messages: messages,
            Tools:    tools,
        })
        if err != nil {
            return 0, err
        }

        msg := resp.Choices[0].Message
        messages = append(messages, msg)

        // Check the expectations for this turn
        if turn.ExpectedAction == "tool_call" && len(msg.ToolCalls) > 0 {
            passedTurns++
        } else if turn.ExpectedAction == "text_response" && len(msg.ToolCalls) == 0 {
            passedTurns++
        }

        // Execute tool calls if any
        for _, tc := range msg.ToolCalls {
            result := executeTool(tc)
            messages = append(messages, llm.Message{
                Role: "tool", Content: result, ToolCallID: tc.ID,
            })
        }
    }

    return float64(passedTurns) / float64(len(mtc.Turns)), nil
}
```

### Step 5: RAGAS metrics for RAG

If the agent uses RAG, specialized metrics are needed.

> **Note:** Below is a simplified Go implementation of RAGAS metrics. The real [RAGAS](https://docs.ragas.io/) is a Python library where `isRelevant`, `scoreFaithfulness` and `scoreRelevance` are implemented via LLM-based evaluation. The example shows the structure of the metrics, not a production implementation.

```go
// RAGAS (Retrieval Augmented Generation Assessment)
type RAGASMetrics struct {
    ContextPrecision float64 `json:"context_precision"` // Fraction of retrieved docs that are relevant
    ContextRecall    float64 `json:"context_recall"`    // Fraction of needed docs that were found
    Faithfulness     float64 `json:"faithfulness"`      // Answer is grounded in retrieved docs (no hallucination)
    AnswerRelevance  float64 `json:"answer_relevance"`  // Answer is relevant to the question
}

func evaluateRAGAS(query, answer string, retrievedDocs, groundTruthDocs []string,
    client *llm.Client) RAGASMetrics {

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

    // Faithfulness: is the answer based on the documents, not on hallucinations?
    metrics.Faithfulness = scoreFaithfulness(answer, retrievedDocs, client)

    // Answer Relevance: is the answer relevant to the question?
    metrics.AnswerRelevance = scoreRelevance(query, answer, client)

    return metrics
}
```

### Step 6: Quality gates in CI/CD

```yaml
# .github/workflows/evals.yml
name: Agent Evals
on: [pull_request]
jobs:
  evals:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v6
        with:
          go-version-file: go.mod

      - name: Run four-level evals
        run: go run cmd/evals/main.go --output=results.json

      - name: Check quality gates
        run: |
          # Thresholds are NOT universal constants: each threshold is
          # calibrated on a baseline — run the current (reference) version
          # of the agent N>=3-5 times and set the threshold slightly below
          # the median. The values below are a calibration example, not a
          # "production standard".
          TASK_THRESHOLD=0.90        # calibrated on its own baseline of task evals
          TOOL_THRESHOLD=0.85        # its own baseline for tool evals
          TRAJECTORY_THRESHOLD=0.75  # trajectory varies the most

          # Parse the results
          TASK_PASS=$(jq '.task_pass_rate' results.json)
          TOOL_ACCURACY=$(jq '.tool_accuracy' results.json)
          TRAJECTORY_PASS=$(jq '.trajectory_pass_rate' results.json)
          TOPIC_SCORE=$(jq '.topic_avg_score' results.json)

          echo "Task Pass Rate: $TASK_PASS"
          echo "Tool Accuracy: $TOOL_ACCURACY"
          echo "Trajectory Pass Rate: $TRAJECTORY_PASS"
          echo "Topic Score: $TOPIC_SCORE"

          # Quality gates per level — each with its own threshold
          FAILED=0
          if (( $(echo "$TASK_PASS < $TASK_THRESHOLD" | bc -l) )); then
            echo "FAIL: Task pass rate $TASK_PASS < $TASK_THRESHOLD"
            FAILED=1
          fi
          if (( $(echo "$TOOL_ACCURACY < $TOOL_THRESHOLD" | bc -l) )); then
            echo "FAIL: Tool accuracy $TOOL_ACCURACY < $TOOL_THRESHOLD"
            FAILED=1
          fi
          if (( $(echo "$TRAJECTORY_PASS < $TRAJECTORY_THRESHOLD" | bc -l) )); then
            echo "FAIL: Trajectory pass rate $TRAJECTORY_PASS < $TRAJECTORY_THRESHOLD"
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
  image: golang:1
  script:
    - go run cmd/evals/main.go --output=results.json
    - |
      TASK_PASS=$(jq '.task_pass_rate' results.json)
      TOOL_ACCURACY=$(jq '.tool_accuracy' results.json)
      echo "Task: $TASK_PASS, Tool: $TOOL_ACCURACY"
      # Threshold calibrated on a baseline (N>=3 runs of the reference version)
      if (( $(echo "$TASK_PASS < $TASK_THRESHOLD" | bc -l) )); then
        echo "Quality gate failed"
        exit 1
      fi
  only:
    - merge_requests
  artifacts:
    paths:
      - results.json
```

#### Cost/latency gate — next to the quality gate

Quality is not the only axis of regression. A prompt tweak may keep the pass rate intact yet double tokens per run or the latency. That's why a **cost/latency gate** sits next to the quality gate: the same runs, but instead of pass rate we check tokens, cost and latency against a baseline ([ch. 20](../20-cost-latency-engineering/README.md)):

```go
// Cost/latency gate: average over N>=3 runs (a single run is not reliable)
// and compare against a baseline taken on the reference agent version
type CostBaseline struct {
    AvgTokensPerRun float64       // from RunResult.Usage, not a manual count
    AvgLatencyMs    float64
}

func checkCostGate(runs []RunResult, base CostBaseline) error {
    var tokens, latency float64
    for _, r := range runs {
        tokens += float64(r.Usage.PromptTokens + r.Usage.CompletionTokens)
        latency += r.LatencyMs
    }
    if tokens/float64(len(runs)) > base.AvgTokensPerRun*1.2 {
        return fmt.Errorf("token regression: %.0f vs baseline %.0f", tokens/float64(len(runs)), base.AvgTokensPerRun)
    }
    if latency/float64(len(runs)) > base.AvgLatencyMs*1.3 {
        return fmt.Errorf("latency regression: %.0fms vs baseline %.0fms", latency/float64(len(runs)), base.AvgLatencyMs)
    }
    return nil
}
```

The baseline is re-measured when you deliberately change the pricing/model — it is not tuned to fit each PR.

#### Live evals: a smoke set against the real system

CI runs evals on stubs — that's not enough. When changing prompts or skills, run a **smoke set of key scenarios against the real production system** — manually or "with a button" (a separate mode with confirmation of dangerous actions, [ch. 05](../05-safety-and-hitl/README.md)). This is a mandatory part of the process, not "someday later": the CI run may be green, while production has a different model ranking, different data, different latency. How to build live evals — [chapter 08](../08-evals-and-reliability/README.md); metrics from live runs go into the same dashboard as CI ([ch. 19](../19-observability-and-tracing/README.md)).

### Step 7: Continuous evaluation (in production)

Evals in CI/CD catch problems before deployment. But models get updated, data changes. You need evaluation in production too:

```go
// Background process: runs evals on real data periodically
func continuousEval(interval time.Duration) {
    ticker := time.NewTicker(interval)
    for range ticker.C {
        // Take a random sample of recent runs
        recentRuns := getRecentRuns(100)
        results := evaluateRuns(recentRuns)

        // Thresholds are deviations from the baseline of the same model,
        // not universal constants
        if results.TaskPassRate < baseline.TaskPassRate-0.05 {
            alert("Task pass rate dropped to %.2f (baseline %.2f)",
                results.TaskPassRate, baseline.TaskPassRate)
        }
        if results.ToolAccuracy < baseline.ToolAccuracy-0.05 {
            alert("Tool accuracy dropped to %.2f (baseline %.2f)",
                results.ToolAccuracy, baseline.ToolAccuracy)
        }

        // Record metrics for the dashboard
        metrics.Record("eval.task_pass_rate", results.TaskPassRate)
        metrics.Record("eval.tool_accuracy", results.ToolAccuracy)
    }
}
```

### Step 8: Dataset versioning

Eval datasets are versioned too:

```go
type EvalDataset struct {
    Version   string     `json:"version"`
    CreatedAt time.Time  `json:"created_at"`
    Cases     []EvalCase `json:"cases"`
}

// The dataset is stored in Git next to the code
// testdata/evals/v1.0.json
// testdata/evals/v1.1.json (new edge cases added)
```

## Mini code example

A minimal eval runner for CI/CD:

```go
package main

import (
    "context"
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
    ctx := context.Background()
    dataset := loadDataset("testdata/evals/latest.json")
    var results []EvalResult

    for _, c := range dataset.Cases {
        answer, res := runEvalCase(ctx, c.Input)
        result := evaluateCase(c, answer, res)
        results = append(results, result)
    }

    summary := summarize(results)

    // Output for CI/CD
    out, _ := json.MarshalIndent(summary, "", "  ")
    os.WriteFile("results.json", out, 0644)
    fmt.Printf("Task: %.2f, Tool: %.2f, Trajectory: %.2f\n",
        summary.TaskPassRate, summary.ToolAccuracy, summary.TrajectoryPassRate)

    // Quality gates: thresholds are read from config, calibrated on the baseline
    // of the current version (N>=3-5 runs); they are not universal constants
    gates := loadGates("gates.yaml")
    if summary.TaskPassRate < gates.TaskThreshold || summary.ToolAccuracy < gates.ToolThreshold {
        fmt.Println("FAILED: Quality gates not met")
        os.Exit(1)
    }
    fmt.Println("PASSED: All quality gates met")
}
```

## Common Mistakes

### Mistake 1: Task level evals only

**Symptom:** The agent passes tests, but in production it picks the wrong tools or takes extra steps.

**Cause:** Only the final answer is checked, not the path to it.

**Fix:**
```go
// BAD: Only "is the answer correct?"
if answer == expected { pass++ }

// GOOD: Four levels of evaluation
result := evaluateCase(c, answer, trajectory)
// Check task + tool + trajectory + topic
```

### Mistake 2: Evals without trajectory recording

**Symptom:** A test failed, but it's unclear at which step things went wrong.

**Cause:** No recording of the execution trajectory.

**Fix:**
```go
// BAD: Run the agent, check only the answer
answer := runAgent(input)

// GOOD: Run via Run — RunResult already carries the trajectory
answer, res := runEvalCase(ctx, input)
// Now we see every step: which tool, which args, what result
```

### Mistake 3: Universal thresholds "as a production standard"

**Symptom:** CI/CD keeps failing due to flaky Trajectory Level evals. Or the opposite — the gate is green, but quality in production is worse than before.

**Cause:** Thresholds were made up "off the top of the head" (0.95 for everything) or copied from someone else's project. You have a different model, a different domain, different variability. Trajectory Level is inherently unstable — the model may choose different paths to the same result.

**Fix:** Each eval level is calibrated against its own baseline: run the current (reference) version of the agent N≥3–5 times, take the median of each metric, and set the threshold slightly below it. A single run is not a reliable baseline — model nondeterminism will introduce a random shift.
```go
// BAD: one threshold "0.95 as the standard" for all levels
threshold := 0.95 // where does this number come from? nowhere

// GOOD: each level's threshold comes from its own baseline
// (median of N>=3 runs of the reference version minus a tolerance)
taskThreshold       := baseline.TaskPassRate - 0.05
toolThreshold       := baseline.ToolAccuracy - 0.05
trajectoryThreshold := baseline.TrajectoryPassRate - 0.10 // the path varies more
```

The baseline is re-measured after a deliberate improvement of the agent — and then the threshold grows with quality instead of staying a constant.

### Mistake 4: No RAGAS metrics for RAG agents

**Symptom:** A RAG agent retrieves irrelevant documents, but evals don't catch it (only the answer is checked).

**Cause:** No retrieval quality evaluation.

**Fix:**
```go
// BAD: Check only the RAG agent's final answer
if answerCorrect { pass++ }

// GOOD: Check both retrieval and the answer
ragasMetrics := evaluateRAGAS(query, answer, retrievedDocs, groundTruthDocs, client)
if ragasMetrics.Faithfulness < 0.8 {
    log.Printf("Low faithfulness: agent may be hallucinating")
}
```

### Mistake 5: Evals only in CI/CD, not in production

**Symptom:** Evals pass in CI/CD, but quality degrades in production (model updated, data changed).

**Cause:** No continuous evaluation.

**Fix:** Run evals in production too (on a sample of real requests).

## Exercises

### Exercise 1: Write a tool level eval

Write an eval case that checks that the agent calls `check_status` (not `restart_service`) for the query "Какой статус сервера?":

```go
testCase := EvalCase{
    Input:          "Какой статус сервера web-01?",
    ExpectedTools:  []string{"check_status"},
    ForbiddenTools: []string{"restart_service"},
    // ...
}
```

### Exercise 2: Implement loop detection

Implement a `detectLoops` function for Trajectory Level:

```go
func detectLoops(res RunResult) bool {
    // Your code: check whether tool calls repeat
}
```

### Exercise 3: Implement a multi-turn eval

Write a test where the agent must first check the status, and then — if the service is down — restart it:

```go
multiTurnCase := MultiTurnCase{
    Turns: []TurnCase{
        {UserInput: "Проверь nginx", ExpectedTools: []string{"check_status"}},
        {UserInput: "Сервис упал, перезапусти", ExpectedTools: []string{"restart_service"}},
    },
}
```

## Acceptance Criteria / Checklist

**Done:**
- [x] Evals integrated into the CI/CD pipeline
- [x] Quality gates block deploys when metrics degrade
- [x] Thresholds are calibrated against a baseline (N≥3 runs), not universal constants
- [x] A cost/latency gate runs alongside the quality gate
- [x] Evaluation at four levels (Task, Tool, Trajectory, Topic)
- [x] Execution trajectory (RunResult) is recorded for analysis
- [x] A smoke set of live evals exists on the real system (ch. 08)
- [x] RAGAS metrics available for RAG agents
- [x] Eval datasets are versioned

**Not done:**
- [ ] Evals not integrated into CI/CD
- [ ] Only the final answer is checked (no Tool/Trajectory level)
- [ ] No trajectory recording (failures cannot be debugged)
- [ ] RAG agents evaluated only on the final answer

## Relation to Other Chapters

- **[Chapter 08: Evals and Reliability](../08-evals-and-reliability/README.md)** — basic eval concepts, live evals in production
- **[Chapter 06: RAG](../06-rag/README.md)** — RAGAS metrics for RAG agents
- **[Chapter 19: Observability and Tracing](../19-observability-and-tracing/README.md)** — RunResult tracing, tied to evals
- **[Chapter 20: Cost & Latency Engineering](../20-cost-latency-engineering/README.md)** — baseline for the cost/latency gate
- **[Chapter 22: Prompt and Program Management](../22-prompt-program-management/README.md)** — prompt testing

## What's Next?

After studying evals in CI/CD, move on to:
- **[24. Data and Privacy](../24-data-and-privacy/README.md)** — data protection and privacy
