# Evals in CI/CD

## Why This Chapter?

You changed prompt or code, and agent works worse. But you only learn about it after deploying to production. Without evals in CI/CD, you cannot automatically check quality before deployment.

### Real-World Case Study

**Situation:** You updated system prompt and deployed changes. After a day, users complain that agent works worse.

**Problem:** No automatic quality check before deployment. Changes are deployed without testing.

**Solution:** Evals in CI/CD pipeline, quality gates, blocking deployment on metric degradation. Now bad changes don't reach production.

## Theory in Simple Terms

### What Are Quality Gates?

Quality Gates are quality checks that block deployment if metrics degraded.

## How It Works (Step-by-Step)

### Step 1: Quality Gates in CI/CD

Integrate evals into CI/CD pipeline:

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

## Where to Integrate in Our Code

### Integration Point: CI/CD Pipeline

Create separate file `cmd/evals/main.go` for running evals:

```go
func main() {
    passRate := runEvals()
    if passRate < 0.95 {
        os.Exit(1)
    }
}
```

## Common Mistakes

### Mistake 1: Evals Not Integrated in CI/CD

**Symptom:** Evals run manually, bad changes reach production.

**Solution:** Integrate evals into CI/CD pipeline.

## Completion Criteria / Checklist

✅ **Completed:**
- Evals integrated in CI/CD
- Quality gates block deployment on degradation

❌ **Not completed:**
- Evals not integrated in CI/CD

## Connection with Other Chapters

- **Evals:** Basic eval concepts — [Chapter 09: Evals and Reliability](../09-evals-and-reliability/README.md)
- **Prompt Management:** Prompt checking via evals — [Prompt and Program Management](prompt_program_mgmt.md)

---

**Navigation:** [← RAG in Production](rag_in_prod.md) | [Chapter 12 Table of Contents](README.md) | [Multi-Agent in Production →](multi_agent_in_prod.md)
