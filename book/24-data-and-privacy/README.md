# 24. Data and Privacy

## Why This Chapter?

An agent processes user personal data (email, phone, address). This data gets into logs and is sent to the LLM API. Without data protection, you violate GDPR and risk data leakage.

### Real-World Case Study

**Situation:** Agent processes user request: "My email john@example.com, phone +7-999-123-4567. Create ticket".

**Problem:** PII gets into logs and sent to LLM API without masking. On log leakage, personal data falls into wrong hands.

**Solution:** Detect and mask PII before logging and sending to the LLM, protect secrets, redact logs, and enforce TTL for stored data.

## Theory in Simple Terms

### What Is PII?

PII (Personally Identifiable Information) is data that allows identifying a person: email, phone, address, passport.

### What Is Redaction?

Redaction is the process of removing sensitive data from logs before saving.

## How It Works (Step by Step)

### Step 1: PII Detection and Masking

Mask PII before sending to LLM:

```go
import "regexp"

func sanitizePII(text string) string {
    // Mask email
    emailRegex := regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`)
    text = emailRegex.ReplaceAllString(text, "[EMAIL_REDACTED]")
    
    // Mask phone
    phoneRegex := regexp.MustCompile(`\b\d{3}-\d{3}-\d{4}\b`)
    text = phoneRegex.ReplaceAllString(text, "[PHONE_REDACTED]")
    
    return text
}
```

### Step 2: Secret Protection

Never log secrets:

```go
func sanitizeSecrets(text string) string {
    // Remove patterns like "password: ..."
    secretRegex := regexp.MustCompile(`(?i)(password|api_key|token|secret)\s*[:=]\s*[\w-]+`)
    text = secretRegex.ReplaceAllString(text, "[SECRET_REDACTED]")
    
    return text
}
```

### Step 3: Log Redaction

Remove sensitive data from logs:

```go
func logWithRedaction(runID string, data map[string]any) {
    sanitized := make(map[string]any)
    for k, v := range data {
        if str, ok := v.(string); ok {
            sanitized[k] = sanitizePII(sanitizeSecrets(str))
        } else {
            sanitized[k] = v
        }
    }
    
    logJSON, _ := json.Marshal(sanitized)
    log.Printf("AGENT_RUN: %s", string(logJSON))
}
```

## Where to Integrate This in Our Code

### Integration Point: User Input

In `labs/lab05-human-interaction/main.go` sanitize input data:

```go
userInput := sanitizePII(sanitizeSecrets(rawInput))
messages = append(messages, openai.ChatCompletionMessage{
    Role: "user",
    Content: userInput,
})
```

## Common Errors

### Error 1: PII Gets into Logs

**Symptom:** User emails and phones visible in logs.

**Solution:** Mask PII before logging.

### Error 2: Secrets Logged

**Symptom:** API keys and passwords get into logs.

**Solution:** Remove secrets from logs via redaction.

## Completion Criteria / Checklist

✅ **Completed:**
- PII masked before sending to LLM
- Secrets not logged
- Logs go through redaction

❌ **Not completed:**
- PII not masked
- Secrets logged

## Connection with Other Chapters

- **[Chapter 17: Security and Governance](../17-security-and-governance/README.md)** — Data protection
- **[Chapter 19: Observability and Tracing](../19-observability-and-tracing/README.md)** — Safe logging

