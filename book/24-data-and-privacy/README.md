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
    emailRegex := regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`)
    text = emailRegex.ReplaceAllString(text, "[EMAIL_REDACTED]")
    
    // Mask phone (Russian and international formats)
    phoneRegex := regexp.MustCompile(`[\+]?[78]\s?[\(-]?\d{3}[\)-]?\s?\d{3}[-]?\d{2}[-]?\d{2}`)
    text = phoneRegex.ReplaceAllString(text, "[PHONE_REDACTED]")
    
    return text
}
```

> **Note:** The regexes above are a simplified example for learning. In production, use specialized libraries: [Microsoft Presidio](https://github.com/microsoft/presidio) for PII detection, [truffleHog](https://github.com/trufflesecurity/trufflehog) or [detect-secrets](https://github.com/Yelp/detect-secrets) for finding secrets in code. They cover dozens of data formats and are regularly updated.

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

In [`labs/lab05-human-interaction/main.go`](https://github.com/kshvakov/ai-agent-course/blob/main/labs/lab05-human-interaction/main.go) sanitize input data:

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

### Error 3: No Data Retention Policy

**Symptom:** Logs grow indefinitely, disk space runs out. Old logs contain PII, but nobody cleans them up.

**Cause:** No data retention policy. Logs and traces are written without TTL or rotation.

**Solution:**
```go
// BAD: logs with no retention limit
func writeLog(entry LogEntry) {
    file.Write(entry) // File grows indefinitely
}

// GOOD: TTL and rotation
type RetentionPolicy struct {
    MaxAge    time.Duration // Maximum retention period
    MaxSizeMB int          // Maximum size in MB
}

func (p *RetentionPolicy) Cleanup(logDir string) error {
    entries, _ := os.ReadDir(logDir)
    for _, entry := range entries {
        info, _ := entry.Info()
        if time.Since(info.ModTime()) > p.MaxAge {
            os.Remove(filepath.Join(logDir, entry.Name()))
        }
    }
    return nil
}
```

### Error 4: No Encryption in Transit

**Symptom:** Data between the agent and LLM API travels over an unprotected channel. A man-in-the-middle attack intercepts requests containing PII.

**Cause:** HTTP instead of HTTPS, missing TLS verification, self-signed certificates without validation.

**Solution:**
```go
// BAD: HTTP without encryption
client := &http.Client{}
resp, _ := client.Post("http://api.llm.example.com/v1/chat", ...)

// GOOD: HTTPS + certificate verification
client := &http.Client{
    Transport: &http.Transport{
        TLSClientConfig: &tls.Config{
            MinVersion: tls.VersionTLS12,
        },
    },
}
resp, _ := client.Post("https://api.llm.example.com/v1/chat", ...)
```

### Error 5: PII in Traces

**Symptom:** OpenTelemetry traces contain user data. Dashboards and alerts expose emails and phones.

**Cause:** Span attributes and logs are added without filtering.

**Solution:**
```go
// BAD: PII leaks into span attributes
span.SetAttributes(
    attribute.String("user.input", userMessage), // Contains PII!
)

// GOOD: sanitize before adding to traces
span.SetAttributes(
    attribute.String("user.input", sanitizePII(userMessage)),
    attribute.String("user.input_hash", hashForCorrelation(userMessage)),
)
```

## Mini-Exercises

### Exercise 1: PII Detector

Implement a PII detector that finds emails and phones in text and returns a list of matches:

```go
type PIIMatch struct {
    Type  string // "email", "phone"
    Value string // Matched value
    Start int    // Start position
    End   int    // End position
}

func detectPII(text string) []PIIMatch {
    // Implement email and phone detection
    // Support formats: user@example.com, +7-999-123-4567, 8 (999) 123-45-67
}
```

**Expected result:**
- Finds email addresses in arbitrary text
- Finds phones in various formats (with +7, 8, parentheses, dashes)
- Returns match positions for targeted replacement

### Exercise 2: Log Redaction Middleware

Create middleware that automatically masks PII in all agent logs:

```go
type RedactionMiddleware struct {
    next     slog.Handler
    patterns []RedactionPattern
}

type RedactionPattern struct {
    Name    string
    Regex   *regexp.Regexp
    Replace string
}

func NewRedactionMiddleware(next slog.Handler) *RedactionMiddleware {
    return &RedactionMiddleware{
        next: next,
        patterns: []RedactionPattern{
            {Name: "email", Regex: regexp.MustCompile(`\b[\w.+-]+@[\w.-]+\.\w{2,}\b`), Replace: "[EMAIL]"},
            {Name: "phone", Regex: regexp.MustCompile(`[\+]?[78]\s?[\(-]?\d{3}[\)-]?\s?\d{3}[-]?\d{2}[-]?\d{2}`), Replace: "[PHONE]"},
        },
    }
}

func (m *RedactionMiddleware) Handle(ctx context.Context, r slog.Record) error {
    // Implement filtering of all record attributes
}
```

**Expected result:**
- All string attributes pass through redaction
- Patterns are easily extensible (add tax ID, passport, credit card)
- Middleware is transparent to the rest of the code

## Completion Criteria / Checklist

**Completed:**
- [x] PII masked before sending to LLM
- [x] Secrets not logged
- [x] Logs go through redaction

**Not completed:**
- [ ] PII not masked
- [ ] Secrets logged

## Connection with Other Chapters

- **[Chapter 17: Security and Governance](../17-security-and-governance/README.md)** — Data protection
- **[Chapter 19: Observability and Tracing](../19-observability-and-tracing/README.md)** — Safe logging

## What's Next?

Once you understand Data and Privacy, move on to:
- **[25. Production Readiness Index](../25-production-readiness-index/README.md)** — Assess your agent's production readiness
