# Prompt and Program Management

## Why This Chapter?

You changed prompt, and agent works worse. But you cannot understand what exactly changed or rollback changes. Without prompt management, you cannot:
- Version prompts
- Track changes and their impact
- Test new versions before deployment
- Rollback bad changes

Prompt and Program Management is control over agent behavior. Without it, you cannot safely change prompts in production.

### Real-World Case Study

**Situation:** You updated system prompt to improve answer quality. After a day, users complain that agent works worse.

**Problem:** No prompt versioning, no evals for checking changes. Cannot understand what exactly changed or rollback changes.

**Solution:** Prompt versioning in Git, evals for checking each version, rollback on metric degradation. Now you can safely experiment with prompts and rollback bad changes.

## Theory in Simple Terms

### What Is Prompt Versioning?

Prompt versioning is storing all prompt versions with metadata (author, date, change description). This allows rolling back changes or comparing versions.

### What Are Prompt Regressions?

Prompt regressions are agent quality degradation after prompt change. Evals help detect regressions before deployment.

## How It Works (Step-by-Step)

### Step 1: Prompt Versioning

Store prompts in Git or DB with versions:

```go
type PromptVersion struct {
    ID          string    `json:"id"`
    Version     string    `json:"version"`
    Content     string    `json:"content"`
    Author      string    `json:"author"`
    CreatedAt   time.Time `json:"created_at"`
    Description string    `json:"description"`
}

func getPromptVersion(id string, version string) (*PromptVersion, error) {
    // Load specific prompt version
    // Can store in Git or DB
    return nil, nil
}
```

### Step 2: Prompt Regressions via Evals

Use evals to check each version (see [Chapter 09](../09-evals-and-reliability/README.md)):

```go
func testPromptVersion(prompt PromptVersion) (float64, error) {
    // Run evals for this prompt version
    passRate := runEvals(prompt.Content)
    
    // Compare with previous version
    prevVersion := getPreviousVersion(prompt.ID)
    if prevVersion != nil {
        prevPassRate := runEvals(prevVersion.Content)
        if passRate < prevPassRate {
            return passRate, fmt.Errorf("regression detected: %.2f < %.2f", passRate, prevPassRate)
        }
    }
    
    return passRate, nil
}
```

### Step 3: Feature Flags

Use feature flags to enable/disable features without deployment:

```go
type FeatureFlags struct {
    UseNewPrompt bool
    UseNewModel  bool
}

func getSystemPrompt(flags FeatureFlags) string {
    if flags.UseNewPrompt {
        return getPromptVersion("system", "v2.0").Content
    }
    return getPromptVersion("system", "v1.0").Content
}
```

## Where to Integrate in Our Code

### Integration Point: System Prompt

In `labs/lab06-incident/SOLUTION.md`, SOP already exists in prompt. Version it:

```go
func getSystemPrompt() string {
    version := os.Getenv("PROMPT_VERSION")
    if version == "" {
        version = "latest"
    }
    
    prompt := getPromptVersion("incident_sop", version)
    return prompt.Content
}
```

## Common Mistakes

### Mistake 1: Prompts Not Versioned

**Symptom:** After prompt change, agent works worse, but you cannot rollback changes.

**Solution:** Version prompts in Git or DB.

### Mistake 2: No Evals for Checking Changes

**Symptom:** Prompt changes deployed without checking, regressions discovered only in production.

**Solution:** Use evals to check each version before deployment.

## Completion Criteria / Checklist

✅ **Completed:**
- Prompts versioned
- Evals check each version
- Rollback on metric degradation

❌ **Not completed:**
- Prompts not versioned
- No evals for checking

## Connection with Other Chapters

- **Evals:** Prompt quality checking — [Chapter 09: Evals and Reliability](../09-evals-and-reliability/README.md)
- **Evals in CI/CD:** Automatic checking — [Evals in CI/CD](evals_in_cicd.md)

---

**Navigation:** [← Security and Governance](security_governance.md) | [Chapter 12 Table of Contents](README.md) | [Data and Privacy →](data_privacy.md)
