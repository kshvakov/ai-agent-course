# 22. Prompt and Program Management

## Why does this matter?

You changed a prompt, and the agent started working worse. But you cannot tell what exactly changed, or roll it back. Without prompt management, you lose control over agent behavior.

In production, prompts are **code**. They define agent behavior just like functions and conditionals. They deserve the same treatment: versioning, testing, rollback, monitoring.

### A real-world case

**Situation:** You updated the system prompt to improve answer quality. A day later, users complain the agent got worse at handling incidents.

**Problem:** No prompt versioning. You don't know which version worked yesterday. You can't roll back.

**Solution:** A centralized prompt store with versions. Evals validate each version before deploy. A/B testing shows which version performs better. Rollback is a single command.

## The theory in plain words

### A prompt as an artifact

A prompt is not "text in the code". It is an **artifact** that:
- Changes more often than code
- Affects behavior unpredictably (a tiny change → a big effect)
- Must be tested on every change
- Must be tied to specific runs/traces for debugging

### What are prompt regressions?

A prompt regression is a degradation of agent quality after a prompt change. One word can break behavior. Evals catch regressions before deploy.

### Skills as part of prompt management

Skills ([chapter 09](../09-agent-architecture/README.md)) are the same prompt management, extended to large bodies of knowledge. A `SKILL.md` file is versioned like code: it lives in git and changes go through PRs with a mandatory eval run — the same rules as for the system prompt. The difference is independence: skills are attached and detached without touching the system prompt itself. Progressive disclosure ([chapter 13](../13-context-engineering/README.md)) keeps the prefix cheap: only skill names and short descriptions go into the system prompt, while the agent loads the full content on demand. Large knowledge moves out of the system prompt into skills — a shorter prefix, a more stable cache, more flexible behavior.

> **How the real SDK does it.** The agent's prompt is assembled from sections (PromptSection): first the immutable system prompt, then the skills section — names and descriptions only. The full body of a skill enters the conversation history once the agent picks it. The run metadata records the versions of both the system prompt and every skill used.

## How it works (step by step)

### Step 1: Centralized prompt store

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
    Tags        map[string]string `json:"tags"`        // "model": "glm-5.3", "domain": "devops"
    IsActive    bool              `json:"is_active"`   // Whether it's used in production
}

func (r *PromptRegistry) Get(promptID, version string) (*PromptVersion, error) {
    versions, ok := r.store[promptID]
    if !ok {
        return nil, fmt.Errorf("prompt %s not found", promptID)
    }
    for i := len(versions) - 1; i >= 0; i-- {
        if versions[i].Version == version {
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

Note: `Get` requires an **exact** version. Which version to use is decided by the deploy-time configuration, not the runtime (why — see "Common Mistakes", mistake 1). The "active" version in the registry is a label for humans and for rollback, not a runtime addressing mechanism.

### Step 2: Versioning with semantic versioning

Apply semver to prompts:

- **MAJOR** (1.0 → 2.0): Structural change (new role, new response format)
- **MINOR** (1.0 → 1.1): Adding instructions (a new edge case, a clarification)
- **PATCH** (1.0.0 → 1.0.1): Typo fix, formatting

The diff between versions can be any line-based algorithm; its elegance doesn't matter — what matters is that the diff is visible to a human: in the PR and in the registry UI. A prompt change without a visible diff is the same as deploying code without a diff in the PR.

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

    // Merge defaults with the passed variables
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

### Step 4: Prompt playground (testing prompts)

A Prompt Playground is an environment for testing prompts before deploy. You can try a prompt against several test inputs and see the results.

```go
type PlaygroundRequest struct {
    PromptVersion string   `json:"prompt_version"`
    TestInputs    []string `json:"test_inputs"`   // Test inputs
    Model         string   `json:"model"`
}

type PlaygroundResult struct {
    Input    string  `json:"input"`
    Output   string  `json:"output"`
    Tokens   int     `json:"tokens"`
    Latency  float64 `json:"latency_ms"`
    HasError bool    `json:"has_error"`
}

// LLMClient — a provider-neutral interface (canonical version in chapter 04)
type LLMClient interface {
    Complete(ctx context.Context, req Request) (*Response, error)
}

func runPlayground(ctx context.Context, req PlaygroundRequest, llm LLMClient) []PlaygroundResult {
    prompt, err := registry.Get("system", req.PromptVersion) // exact version
    if err != nil {
        return nil
    }
    var results []PlaygroundResult

    for _, input := range req.TestInputs {
        start := time.Now()
        resp, err := llm.Complete(ctx, Request{
            Model: req.Model, // e.g. "glm-5.3" or "fable-5.1"
            Messages: []Message{
                {Role: "system", Content: prompt.Content},
                {Role: "user", Content: input},
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
            result.Tokens = resp.Usage.PromptTokens + resp.Usage.CompletionTokens
        }
        results = append(results, result)
    }
    return results
}
```

### Step 5: A/B testing prompts

We test two prompt versions side by side on real traffic:

```go
type ABTest struct {
    Name       string  `json:"name"`
    VersionA   string  `json:"version_a"`   // Control group
    VersionB   string  `json:"version_b"`   // Experimental group
    TrafficPct float64 `json:"traffic_pct"` // % of traffic to version B (0.0 - 1.0)
    StartedAt  time.Time
}

func (ab *ABTest) SelectVersion(requestID string) string {
    // Deterministic choice based on requestID (for reproducibility)
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
prompt, err := registry.Get("incident_sop", selectedVersion)
if err != nil {
    return fmt.Errorf("prompt version %s not found: %w", selectedVersion, err)
}
```

Rules of the experiment: both versions are pinned in the A/B test config, the choice is deterministic and happens **once — at the start of the run**. From then on, the prompt version is a constant of the run: the system prompt doesn't change mid-run, the cacheable prefix stays stable ([chapter 02](../02-prompt-engineering/README.md)), and the run's trace is reproducible: `runID → prompt_version` is unambiguous.

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

### Step 6: MCP for prompts

An MCP server can serve prompts to agents. This is useful when several agents share the same prompts:

```go
// The MCP server exposes prompts as resources
type PromptMCPServer struct {
    registry *PromptRegistry
}

// The agent requests a prompt via MCP
func (s *PromptMCPServer) GetResource(uri string) (string, error) {
    // URI: "prompt://incident_sop/1.1.0" — the version comes from the agent's config, not "current"
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

For more on MCP, see [Chapter 18: Tool Protocols](../18-tool-protocols-and-servers/README.md).

### Step 7: Link to traces

Every run writes into the trace metadata: `run_id`, `prompt_id`, `prompt_version`, `model`, timestamp. A single `tracing.LogMetadata(...)` call at the start of the run — and incident questions become a query: "which prompt version was running in run `abc123`?" → `prompt_id=incident_sop, version=1.1.0`. Without this link, a user complaint is unresolvable: you don't even know which prompt produced the bad behavior.

For more on tracing, see [Chapter 19: Observability and Tracing](../19-observability-and-tracing/README.md).

## Mini code example

A minimal example: loading a prompt by version + a feature flag:

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

## Common Mistakes

### Mistake 1: Prompts hardcoded in code

**Symptom:** To change a prompt, you have to change code, go through code review, and deploy.

**Cause:** Prompts are stored as string constants in Go files.

**Solution:** a version registry — but the version is pinned in the deploy-time configuration, not picked by the runtime:

```go
// BAD: prompt in code
const systemPrompt = "You are a DevOps agent..."

// BAD: "latest" at runtime — the runtime decides which version is "current"
prompt, _ := registry.Get("system_devops", "latest")

// GOOD: the version is a configuration constant, pinned at deploy time
prompt, err := registry.Get("system_devops", cfg.PromptVersion) // e.g. "1.4.2"
if err != nil {
    return fmt.Errorf("prompt version %s not found: %w", cfg.PromptVersion, err)
}
```

Why not "latest": the system prompt is a **cacheable prefix** ([chapter 02](../02-prompt-engineering/README.md)). If the runtime pulls the "current" version on every request, any parallel update silently changes the prefix — a cache miss (cost and latency grow) and nondeterminism: two traces of the same run are built from different prompts, making debugging impossible. A version change is a deliberate act: a PR with a diff and an eval run (chapters [08](../08-evals-and-reliability/README.md) and [23](../23-evals-in-cicd/README.md)), then a new version in the config and a deploy.

### Mistake 2: No evals before deploying a prompt

**Symptom:** A new prompt version breaks agent behavior. You find out after the deploy.

**Cause:** Prompts are deployed without testing.

**Solution:**
```go
// BAD: deploy without validation
registry.SetActive("system_devops", "2.0.0")

// GOOD: validate via evals before activation
passRate := runEvalsForPrompt("system_devops", "2.0.0")
if passRate < 0.95 {
    log.Printf("Prompt 2.0.0 failed evals: %.2f < 0.95", passRate)
    return // Don't activate
}
registry.SetActive("system_devops", "2.0.0")
```

A manual run is a half-measure: quality gates for prompts must live in CI/CD and block merging a PR that changes a prompt without a green eval run — how this works is covered in [Chapter 23: Evals in CI/CD](../23-evals-in-cicd/README.md).

### Mistake 3: A/B test without statistics

**Symptom:** You switched 100% of traffic to the new version after "a test on 10 requests showed it's better".

**Cause:** Not enough data for a statistically significant comparison.

**Solution:**
```go
// BAD: 10 requests → decision
if sampleSize < 100 {
    log.Println("Not enough data for A/B decision")
    return
}

// GOOD: a statistically significant sample
// At least 100-500 requests per version
// Compare across multiple metrics (pass rate, latency, tokens)
```

### Mistake 4: No link between prompt and tracing

**Symptom:** A user complains about a bad answer. You don't know which prompt version was used.

**Cause:** Run metadata doesn't record the prompt version.

**Solution:**
```go
// BAD: run the agent without recording the prompt version
runAgent(prompt.Content, ...)

// GOOD: write the version into the trace
logRunMetadata(runID, prompt, model)
runAgent(prompt.Content, ...)
```

### Mistake 5: Variables instead of templates

**Symptom:** The prompt is assembled via `fmt.Sprintf` with 10+ arguments. You can't tell what the resulting prompt looks like.

**Cause:** No templating.

**Solution:**
```go
// BAD:
prompt := fmt.Sprintf("You are a %s. Tools: %s. SOP: %s. Constraints: %s.", role, tools, sop, constraints)

// GOOD:
tmpl := PromptTemplate{
    Template: `You are a {{.Role}}.
Tools: {{.ToolList}}
SOP: {{.SOP}}
Constraints: {{.Constraints}}`,
}
prompt, _ := tmpl.Render(vars)
```

## Exercises

### Exercise 1: Implement PromptRegistry

Implement a prompt store with `Get`, `Add`, `Rollback` methods:

```go
type PromptRegistry struct {
    // Your code
}

func (r *PromptRegistry) Get(id, version string) (*PromptVersion, error) { ... }
func (r *PromptRegistry) Add(pv PromptVersion) error { ... }
func (r *PromptRegistry) Rollback(id, version string) error { ... }
```

**Expected result:**
- You can add several versions of a prompt
- You can fetch a specific version; requesting a nonexistent version returns an error
- Rollback deactivates the current version and activates the specified one

### Exercise 2: Implement an A/B test

Implement prompt version selection by requestID:

```go
func selectVersion(requestID string, trafficPct float64) string {
    // Your code: deterministic selection based on hash(requestID)
}
```

**Expected result:**
- The same requestID always gets the same version
- trafficPct=0.1 routes ~10% of requests to version B

### Exercise 3: Implement a playground

Implement a function for testing a prompt against multiple inputs:

```go
func testPrompt(promptContent string, testInputs []string) []PlaygroundResult {
    // Your code here
}
```

**Expected result:**
- Each input is tested with the given prompt
- The result contains output, tokens, latency

## Acceptance Criteria / Checklist

**Done:**
- [x] Prompts are stored in a centralized registry with versions
- [x] Each version passes evals before activation
- [x] Templating is available for variables in prompts
- [x] The prompt version is recorded in the trace of every run
- [x] There is a rollback mechanism
- [x] Feature flags let you enable/disable versions without a deploy
- [x] The prompt version is pinned in the config at deploy time; the runtime does not pick the "current" one
- [x] Skills (SKILL.md) are versioned like code and pass evals independently of the system prompt

**Not done:**
- [ ] Prompts are hardcoded
- [ ] No evals to verify changes
- [ ] No link between prompt and trace
- [ ] A/B tests run without a statistically significant sample

## For the curious

### Prompt as code vs prompt as config

Two approaches to prompt management:

1. **Prompt as Code**: prompts in Git, changes via PR. Pro — full audit trail. Con — slow change cycle.
2. **Prompt as Config**: prompts in a DB/API, changes via UI. Pro — fast iteration. Con — harder to track.

The optimum: **Prompt as Code** for the system prompt (rarely changes), **Prompt as Config** for few-shot examples and SOPs (change often).

Wherever a prompt lives, one rule is invariant: dynamic insertions (data, task context, fresh facts) go **at the end of the message history**, never into the system prompt (`messages[0]`). The system prompt is a cacheable prefix ([chapter 02](../02-prompt-engineering/README.md)): any dynamism in it breaks the cache and makes every request more expensive, and also makes traces unreproducible.

## Relation to Other Chapters

- **[Chapter 02: Prompting](../02-prompt-engineering/README.md)** — how to write effective prompts; the system prompt as a cacheable prefix
- **[Chapter 08: Evals and Reliability](../08-evals-and-reliability/README.md)** — how to test prompts
- **[Chapter 09: Agent Architecture](../09-agent-architecture/README.md)** — skills (SKILL.md) and progressive disclosure
- **[Chapter 18: Tool Protocols](../18-tool-protocols-and-servers/README.md)** — MCP for serving prompts
- **[Chapter 19: Observability and Tracing](../19-observability-and-tracing/README.md)** — linking prompts to tracing
- **[Chapter 23: Evals in CI/CD](../23-evals-in-cicd/README.md)** — quality gates: automated prompt checks in the pipeline
- **[Agent Skills](https://agentskills.io/)** — the `SKILL.md` format as a standard way to package reusable prompts and instructions

## What's Next?

After mastering prompt management, move on to:
- **[23. Evals in CI/CD](../23-evals-in-cicd/README.md)** — automated quality checks in the pipeline
