# 23. Evals in CI/CD

## Why This Chapter?

You changed the prompt or code, and the agent works worse. But you only learn about it after deploying to production. Without evals in CI/CD, you cannot automatically check quality before deployment.

### Real-World Case Study

**Situation:** You updated the system prompt and deployed changes. After a day, users complain that the agent works worse.

**Problem:** No automatic quality check before deployment. Changes are deployed without testing.

**Solution:** Evals in CI/CD pipeline, quality gates, blocking deployment on metric degradation. Now bad changes don't reach production.

## Theory in Simple Terms

### What Are Quality Gates?

Quality Gates are quality checks that block deployment if metrics have degraded.

## How It Works (Step by Step)

### Step 1: Quality Gates in CI/CD

Integrate evals into CI/CD pipeline:

#### GitHub Actions

```yaml
# .github/workflows/evals.yml
name: Run Evals
on: [pull_request]
jobs:
  evals:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - name: Run evals
        run: go run cmd/evals/main.go
      - name: Check quality gate
        run: |
          if [ "$PASS_RATE" -lt "0.95" ]; then
            echo "Quality gate failed: Pass rate $PASS_RATE < 0.95"
            exit 1
          fi
```

#### GitLab CI/CD

```yaml
# .gitlab-ci.yml
stages:
  - evals

evals:
  stage: evals
  image: golang:1.21
  before_script:
    - go version
  script:
    # Run evals and save result
    - |
      PASS_RATE=$(go run cmd/evals/main.go 2>&1 | grep -oP 'Pass rate: \K[0-9.]+' || echo "0")
      echo "Pass rate: $PASS_RATE"
      # Check quality gate
      if (( $(echo "$PASS_RATE < 0.95" | bc -l) )); then
        echo "Quality gate failed: Pass rate $PASS_RATE < 0.95"
        exit 1
      fi
  only:
    - merge_requests
  tags:
    - docker
```

**Alternative** (if `cmd/evals/main.go` exports environment variable):

```yaml
evals:
  stage: evals
  image: golang:1.21
  script:
    - go run cmd/evals/main.go
    - |
      if [ "$PASS_RATE" -lt "0.95" ]; then
        echo "Quality gate failed: Pass rate $PASS_RATE < 0.95"
        exit 1
      fi
  only:
    - merge_requests
```

**Note:** Ensure `cmd/evals/main.go` outputs result in parseable format (e.g., `Pass rate: 0.95`), or exports environment variable `PASS_RATE`.

### Step 2: Dataset Versioning

Store "golden" scenarios in dataset:

```go
type EvalDataset struct {
    Version string
    Cases   []EvalCase
}

type EvalCase struct {
    Input    string
    Expected string
}
```

## Where to Integrate This in Our Code

### Integration Point: CI/CD Pipeline

Create separate file `cmd/evals/main.go` to run evals:

```go
package main

import (
    "fmt"
    "os"
)

func main() {
    passRate := runEvals()
    
    // Output result in format parseable in CI/CD
    fmt.Printf("Pass rate: %.2f\n", passRate)
    
    // Export environment variable for convenience (works in both systems)
    os.Setenv("PASS_RATE", fmt.Sprintf("%.2f", passRate))
    
    // Quality gate: if pass rate below threshold, exit with error
    if passRate < 0.95 {
        fmt.Printf("Quality gate failed: Pass rate %.2f < 0.95\n", passRate)
        os.Exit(1)
    }
    
    fmt.Println("Quality gate passed!")
}
```

**Note:** This example works with both GitHub Actions and GitLab CI/CD. Environment variable `PASS_RATE` is available in both cases, and output in format `Pass rate: 0.95` allows parsing result via `grep` or other tools.

## Common Errors

### Error 1: Evals Not Integrated in CI/CD

**Symptom:** Evals run manually, bad changes reach production.

**Solution:** Integrate evals into CI/CD pipeline.

## Completion Criteria / Checklist

✅ **Completed:**
- Evals integrated in CI/CD
- Quality gates block deployment on degradation

❌ **Not completed:**
- Evals not integrated in CI/CD

## Connection with Other Chapters

- **[Chapter 08: Evals and Reliability](../08-evals-and-reliability/README.md)** — Basic eval concepts
- **[Chapter 22: Prompt and Program Management](../22-prompt-program-management/README.md)** — Checking prompts via evals

---

**Navigation:** [← Prompt and Program Management](../22-prompt-program-management/README.md) | [Table of Contents](../README.md) | [Data and Privacy →](../24-data-and-privacy/README.md)
