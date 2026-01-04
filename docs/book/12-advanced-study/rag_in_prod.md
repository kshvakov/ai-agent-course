# RAG in Production

## Why This Chapter?

RAG system works, but documents are outdated, search returns irrelevant results, agent hallucinates. Without production-ready RAG, you cannot guarantee answer quality.

### Real-World Case Study

**Situation:** RAG system uses API documentation. Documentation updates daily, but RAG uses old version.

**Problem:** Agent gives outdated answers, referencing old documentation. No document versioning, no currency check.

**Solution:** Document versioning, freshness tracking, grounding (requiring document references), fallback on retrieval errors.

## Theory in Simple Terms

### What Is Freshness?

Freshness is document currency. Document is considered outdated if older than certain age (e.g., 30 days).

### What Is Grounding?

Grounding is requirement that agent references found documents in answer. This reduces hallucinations.

## How It Works (Step-by-Step)

### Step 1: Document Versioning

Version documents in knowledge base:

```go
type DocumentVersion struct {
    ID        string    `json:"id"`
    Version   string    `json:"version"`
    Content   string    `json:"content"`
    UpdatedAt time.Time `json:"updated_at"`
}

func getDocumentVersion(id string, version string) (*DocumentVersion, error) {
    // Load specific document version
    return nil, nil
}
```

### Step 2: Freshness Check

Check document currency:

```go
func checkFreshness(doc DocumentVersion, maxAge time.Duration) bool {
    age := time.Since(doc.UpdatedAt)
    return age < maxAge
}
```

### Step 3: Grounding

Require document references in answer:

```go
func validateGrounding(answer string, documents []DocumentVersion) bool {
    // Check that answer contains document references
    for _, doc := range documents {
        if strings.Contains(answer, doc.ID) {
            return true
        }
    }
    return false
}
```

## Where to Integrate in Our Code

### Integration Point: RAG Retrieval

In `labs/lab07-rag/main.go`, add freshness and grounding checks:

```go
func retrieveDocuments(query string) ([]DocumentVersion, error) {
    docs := searchDocuments(query)
    
    // Filter outdated documents
    freshDocs := []DocumentVersion{}
    for _, doc := range docs {
        if checkFreshness(doc, 30*24*time.Hour) {
            freshDocs = append(freshDocs, doc)
        }
    }
    
    return freshDocs, nil
}
```

## Common Mistakes

### Mistake 1: Documents Not Versioned

**Symptom:** Cannot understand which document version was used in answer.

**Solution:** Version documents and track version in answers.

### Mistake 2: No Currency Check

**Symptom:** Agent uses outdated documents.

**Solution:** Check document freshness before use.

## Completion Criteria / Checklist

✅ **Completed:**
- Documents versioned
- Document currency checked
- Grounding required in answers

❌ **Not completed:**
- Documents not versioned
- No currency check

## Connection with Other Chapters

- **RAG:** Basic RAG concepts — [Chapter 07: RAG and Knowledge Base](../07-rag/README.md)

---

**Navigation:** [← Data and Privacy](data_privacy.md) | [Chapter 12 Table of Contents](README.md) | [Evals in CI/CD →](evals_in_cicd.md)
