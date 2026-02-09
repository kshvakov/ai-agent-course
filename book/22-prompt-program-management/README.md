# 22. Prompt and Program Management

## Why This Chapter?

You changed the prompt, and the agent got worse. But you can't figure out what exactly changed, or roll back. Without prompt management, you lose control over agent behavior.

In production, prompts are **code**. They define agent behavior just like functions and conditionals. They deserve the same treatment: versioning, testing, rollback, monitoring.

### Real-World Case Study

**Situation:** You updated the system prompt to improve response quality. A day later, users complain that the agent handles incidents worse.

**Problem:** No prompt versioning. You don't know which version was running yesterday. You can't roll back.

**Solution:** A centralized prompt registry with versions. Evals validate each version before deployment. A/B testing shows which version performs better. Rollback is one command.

## Theory in Simple Terms

### Prompt as Artifact

A prompt is not "text in code." It's an **artifact** that:
- Changes more often than code
- Affects behavior unpredictably (small change → big effect)
- Must be tested on every change
- Must be linked to specific runs/traces for debugging

### What Are Prompt Regressions?

A prompt regression is agent quality degradation after a prompt change. A single word can break behavior. Evals catch regressions before deployment.

## How It Works (Step by Step)

### Step 1: Centralized Prompt Registry

All prompts live in one place with metadata:

```go
type PromptRegistry struct {
    store map[string][]PromptVersion // promptID → versions
}

type PromptVersion struct {
    ID          string            `json:"id"`
    PromptID    string            `json:"prompt_id"`
    Version     string            `json:"version"`     // "1.0.0", "1.1.0"
    Content     string            `json:"content"`
    Variables   []string          `json:"variables"`   // Variables in the prompt
    Author      string            `json:"author"`
    CreatedAt   time.Time         `json:"created_at"`
    Description string            `json:"description"` // What changed
    Tags        map[string]string `json:"tags"`        // "model": "gpt-4o", "domain": "devops"
    IsActive    bool              `json:"is_active"`   // Currently used in production
}

func (r *PromptRegistry) Get(promptID, version string) (*PromptVersion, error) {
    versions, ok := r.store[promptID]
    if !ok {
        return nil, fmt.Errorf("prompt %s not found", promptID)
    }
    for i := len(versions) - 1; i >= 0; i-- {
        if version == "latest" || versions[i].Version == version {
            return &versions[i], nil
        }
    }
    return nil, fmt.Errorf("version %s not found", version)
}

func (r *PromptRegistry) Rollback(promptID, toVersion string) error {
    version, err := r.Get(promptID, toVersion)
    if err != nil {
        return err
    }
    // Deactivate the current version, activate the rollback target
    r.deactivateAll(promptID)
    version.IsActive = true
    return nil
}
```

### Step 2: Versioning with Semantic Versioning

Apply semver to prompts:

- **MAJOR** (1.0 → 2.0): Structural change (new role, new response format)
- **MINOR** (1.0 → 1.1): Adding instructions (new edge case, clarification)
- **PATCH** (1.0.0 → 1.0.1): Typo fix, formatting

```go
// Diff between versions
func (r *PromptRegistry) Diff(promptID, v1, v2 string) string {
    pv1, _ := r.Get(promptID, v1)
    pv2, _ := r.Get(promptID, v2)

    // Line-by-line comparison
    lines1 := strings.Split(pv1.Content, "\n")
    lines2 := strings.Split(pv2.Content, "\n")

    var diff strings.Builder
    // ... standard diff algorithm ...
    return diff.String()
}
```

### Step 3: Templating

Prompts often contain variables. Separate the template from the data:

```go
type PromptTemplate struct {
    Template  string            // "You are a {{.Role}}. Your tools: {{.ToolList}}"
    Defaults  map[string]string // Default values
}

func (pt *PromptTemplate) Render(vars map[string]string) (string, error) {
    tmpl, err := template.New("prompt").Parse(pt.Template)
    if err != nil {
        return "", err
    }

    // Merge defaults with provided variables
    merged := make(map[string]string)
    for k, v := range pt.Defaults {
        merged[k] = v
    }
    for k, v := range vars {
        merged[k] = v
    }

    var buf bytes.Buffer
    if err := tmpl.Execute(&buf, merged); err != nil {
        return "", err
    }
    return buf.String(), nil
}

// Usage
tmpl := PromptTemplate{
    Template: `You are a {{.Role}} agent.
Available tools: {{.ToolList}}
SOP: {{.SOP}}
Constraints: {{.Constraints}}`,
    Defaults: map[string]string{
        "Constraints": "Always ask for confirmation before destructive actions.",
    },
}

prompt, _ := tmpl.Render(map[string]string{
    "Role":     "DevOps",
    "ToolList": "ping, check_status, restart_service",
    "SOP":      "1. Diagnose 2. Fix 3. Verify",
})
```

### Step 4: Prompt Playground

A Prompt Playground is an environment for testing prompts before deployment. You can run a prompt against several test inputs and see the results.

```go
type PlaygroundRequest struct {
    PromptVersion string   `json:"prompt_version"`
    TestInputs    []string `json:"test_inputs"`   // Test queries
    Model         string   `json:"model"`
}

type PlaygroundResult struct {
    Input    string  `json:"input"`
    Output   string  `json:"output"`
    Tokens   int     `json:"tokens"`
    Latency  float64 `json:"latency_ms"`
    HasError bool    `json:"has_error"`
}

func runPlayground(req PlaygroundRequest, client *openai.Client) []PlaygroundResult {
    prompt, _ := registry.Get("system", req.PromptVersion)
    var results []PlaygroundResult

    for _, input := range req.TestInputs {
        start := time.Now()
        resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
            Model: req.Model,
            Messages: []openai.ChatCompletionMessage{
                {Role: openai.ChatMessageRoleSystem, Content: prompt.Content},
                {Role: openai.ChatMessageRoleUser, Content: input},
            },
        })

        result := PlaygroundResult{
            Input:   input,
            Latency: float64(time.Since(start).Milliseconds()),
        }
        if err != nil {
            result.HasError = true
        } else {
            result.Output = resp.Choices[0].Message.Content
            result.Tokens = resp.Usage.TotalTokens
        }
        results = append(results, result)
    }
    return results
}
```

### Step 5: A/B Testing Prompts

Test two prompt versions in parallel on live traffic:

```go
type ABTest struct {
    Name       string  `json:"name"`
    VersionA   string  `json:"version_a"`   // Control group
    VersionB   string  `json:"version_b"`   // Experimental group
    TrafficPct float64 `json:"traffic_pct"` // % of traffic to version B (0.0 - 1.0)
    StartedAt  time.Time
}

func (ab *ABTest) SelectVersion(requestID string) string {
    // Deterministic selection based on requestID (for reproducibility)
    hash := fnv.New32a()
    hash.Write([]byte(requestID))
    bucket := float64(hash.Sum32()) / float64(math.MaxUint32)

    if bucket < ab.TrafficPct {
        return ab.VersionB
    }
    return ab.VersionA
}

// Usage in the agent loop
abTest := ABTest{
    Name: "improved_sop_prompt",
    VersionA: "1.0.0", // Current version
    VersionB: "1.1.0", // New version
    TrafficPct: 0.1,   // 10% of traffic to the new version
}

selectedVersion := abTest.SelectVersion(runID)
prompt, _ := registry.Get("incident_sop", selectedVersion)
```

**Metrics for comparison:**

```go
type ABMetrics struct {
    Version    string
    PassRate   float64 // % of successful tasks
    AvgLatency float64 // Average latency
    AvgTokens  float64 // Average token consumption
    UserRating float64 // User rating (if available)
}
```

### Step 6: MCP for Prompts

An MCP server can serve prompts to agents. This is useful when multiple agents share common prompts:

```go
// MCP server exposes prompts as resources
type PromptMCPServer struct {
    registry *PromptRegistry
}

// Agent requests a prompt via MCP
func (s *PromptMCPServer) GetResource(uri string) (string, error) {
    // URI: "prompt://incident_sop/latest"
    parts := strings.Split(uri, "/")
    promptID := parts[1]
    version := parts[2]
    pv, err := s.registry.Get(promptID, version)
    if err != nil {
        return "", err
    }
    return pv.Content, nil
}
```

For more on MCP, see [Chapter 18: Tool Protocols and Servers](../18-tool-protocols-and-servers/README.md).

### Step 7: Link to Traces

Every agent run records which prompt version was used. This lets you tie behavior to a specific version:

```go
type RunMetadata struct {
    RunID         string `json:"run_id"`
    PromptID      string `json:"prompt_id"`
    PromptVersion string `json:"prompt_version"`
    Model         string `json:"model"`
    Timestamp     time.Time
}

func logRunMetadata(runID string, prompt *PromptVersion, model string) {
    metadata := RunMetadata{
        RunID:         runID,
        PromptID:      prompt.PromptID,
        PromptVersion: prompt.Version,
        Model:         model,
        Timestamp:     time.Now(),
    }
    // Write to the trace
    tracing.LogMetadata(metadata)
}

// Now when investigating an incident:
// "Which prompt version was used in run_id=abc123?"
// → prompt_id=incident_sop, version=1.1.0
```

For more on tracing, see [Chapter 19: Observability and Tracing](../19-observability-and-tracing/README.md).

## Minimal Code Example

A minimal example: load a prompt by version + feature flag:

```go
func getSystemPrompt(flags FeatureFlags) string {
    version := "1.0.0"
    if flags.UseNewPrompt {
        version = "1.1.0"
    }

    prompt, err := registry.Get("system_devops", version)
    if err != nil {
        log.Printf("Failed to get prompt version %s: %v, using default", version, err)
        return defaultPrompt
    }
    return prompt.Content
}
```

## Common Errors

### Error 1: Prompts Hardcoded in Source

**Symptom:** Changing a prompt requires a code change, code review, and a full deploy.

**Cause:** Prompts are stored as string constants in Go files.

**Solution:**
```go
// BAD: Prompt in code
const systemPrompt = "You are a DevOps agent..."

// GOOD: Prompt from the registry
prompt, _ := registry.Get("system_devops", "latest")
```

### Error 2: No Evals Before Deploying a Prompt

**Symptom:** A new prompt version breaks agent behavior. You find out after deployment.

**Cause:** Prompts are deployed without testing.

**Solution:**
```go
// BAD: Deploy without validation
registry.SetActive("system_devops", "2.0.0")

// GOOD: Run evals before activation
passRate := runEvalsForPrompt("system_devops", "2.0.0")
if passRate < 0.95 {
    log.Printf("Prompt 2.0.0 failed evals: %.2f < 0.95", passRate)
    return // Don't activate
}
registry.SetActive("system_devops", "2.0.0")
```

### Error 3: A/B Test Without Statistical Significance

**Symptom:** You switched 100% of traffic to the new version after "a test on 10 requests showed it's better."

**Cause:** Not enough data for a statistically significant comparison.

**Solution:**
```go
// BAD: 10 requests → decision
if sampleSize < 100 {
    log.Println("Not enough data for A/B decision")
    return
}

// GOOD: Statistically significant sample
// Minimum 100-500 requests per version
// Compare across multiple metrics (pass rate, latency, tokens)
```

### Error 4: No Link Between Prompt and Trace

**Symptom:** A user complains about a bad response. You don't know which prompt version was used.

**Cause:** Run metadata doesn't record the prompt version.

**Solution:**
```go
// BAD: Run the agent without recording the prompt version
runAgent(prompt.Content, ...)

// GOOD: Log the version in the trace
logRunMetadata(runID, prompt, model)
runAgent(prompt.Content, ...)
```

### Error 5: String Formatting Instead of Templates

**Symptom:** The prompt is built with `fmt.Sprintf` and 10+ arguments. You can't predict what the final prompt looks like.

**Cause:** No templating.

**Solution:**
```go
// BAD
prompt := fmt.Sprintf("You are a %s. Tools: %s. SOP: %s. Constraints: %s.", role, tools, sop, constraints)

// GOOD
tmpl := PromptTemplate{
    Template: `You are a {{.Role}}.
Tools: {{.ToolList}}
SOP: {{.SOP}}
Constraints: {{.Constraints}}`,
}
prompt, _ := tmpl.Render(vars)
```

## Exercises

### Exercise 1: Implement a PromptRegistry

Build a prompt store with `Get`, `Add`, and `Rollback` methods:

```go
type PromptRegistry struct {
    // Your code
}

func (r *PromptRegistry) Get(id, version string) (*PromptVersion, error) { ... }
func (r *PromptRegistry) Add(pv PromptVersion) error { ... }
func (r *PromptRegistry) Rollback(id, version string) error { ... }
```

**Expected result:**
- You can add multiple versions of a prompt
- You can retrieve a specific version or "latest"
- Rollback deactivates the current version and activates the target

### Exercise 2: Implement A/B Testing

Implement prompt version selection by requestID:

```go
func selectVersion(requestID string, trafficPct float64) string {
    // Your code: deterministic selection based on hash(requestID)
}
```

**Expected result:**
- The same requestID always gets the same version
- trafficPct=0.1 routes ~10% of requests to version B

### Exercise 3: Implement a Playground

Implement a function that tests a prompt against multiple inputs:

```go
func testPrompt(promptContent string, testInputs []string) []PlaygroundResult {
    // Your code
}
```

**Expected result:**
- Each input is tested with the given prompt
- The result contains output, tokens, latency

## Completion Criteria / Checklist

**Completed:**
- [x] Prompts are stored in a centralized registry with versions
- [x] Each version passes evals before activation
- [x] Templating is used for prompt variables
- [x] The prompt version is recorded in the trace of every run
- [x] A rollback mechanism exists
- [x] Feature flags allow enabling/disabling versions without a deploy

**Not completed:**
- [ ] Prompts are hardcoded in source
- [ ] No evals for validating changes
- [ ] No link between prompt and trace
- [ ] A/B tests run without a statistically significant sample

## Deep Dive

### Prompt as Code vs Prompt as Config

Two approaches to prompt management:

1. **Prompt as Code**: Prompts live in Git, changes go through PRs. Pro — full audit trail. Con — slow iteration cycle.
2. **Prompt as Config**: Prompts live in a DB/API, changes happen through a UI. Pro — fast iteration. Con — harder to track.

The sweet spot: **Prompt as Code** for the system prompt (changes rarely), **Prompt as Config** for few-shot examples and SOPs (changes often).

## Connection with Other Chapters

- **[Chapter 02: Prompt Engineering](../02-prompt-engineering/README.md)** — how to write effective prompts
- **[Chapter 08: Evals and Reliability](../08-evals-and-reliability/README.md)** — how to test prompts
- **[Chapter 18: Tool Protocols and Servers](../18-tool-protocols-and-servers/README.md)** — MCP for serving prompts
- **[Chapter 19: Observability and Tracing](../19-observability-and-tracing/README.md)** — linking prompts to traces
- **[Chapter 23: Evals in CI/CD](../23-evals-in-cicd/README.md)** — automated validation in the pipeline
- **[Agent Skills](https://agentskills.io/)** — `SKILL.md` format as a standard way to package reusable prompts and instructions

## What's Next?

After learning prompt management, move on to:
- **[23. Evals in CI/CD](../23-evals-in-cicd/README.md)** — automated quality checks in the pipeline