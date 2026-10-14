# 24. Data and Privacy

## Why does this matter?

An agent works with users' personal data (email, phone, address). This data ends up in logs and gets sent to the LLM API. Without data protection you violate GDPR and risk a data leak.

### A real-world case

**Situation:** The agent processes a user request: "Мой email john@example.com, телефон +7-999-123-4567. Создай тикет".

**Problem:** PII ends up in logs, traces, and tool output without masking. If logs leak, personal data falls into the wrong hands.

**Solution:** Detect and mask PII **at the boundary of logs, traces, and tool output** (leave the model input untouched — the agent needs the data), redaction by default, secret protection, TTL for storage.

> **Junior employee model.** Data privacy for an agent is **the same data policies as for a new employee**. The employee signs an NDA, doesn't forward client emails outside, doesn't drop screenshots with PII into a public channel, deletes personal data after the reporting period (retention). The agent does the same: doesn't write PII into spans/logs as-is (see [Ch. 19](../19-observability-and-tracing/README.md), the rule about redacted attributes), doesn't pass extra data to the LLM provider, deletes data by TTL. Nothing fundamentally new here — it's an adaptation of familiar GDPR practices to code. For the full framework, see [Preface → Mental Model](../00-preface/README.md#mental-model-an-agent-is-a-new-employee) and [Ch. 17: Security and Governance](../17-security-and-governance/README.md).

## The theory in plain words

### What is PII?

PII (Personally Identifiable Information) is data that can identify a person: email, phone, address, passport.

### What is redaction?

Redaction is removing sensitive data from logs before storing them.

## How it works (step by step)

### Step 1: Detect and mask PII — at the boundary, not at the model input

> **Do NOT mask the model input.** If a user writes "Мой email john@example.com, создай тикет" — the agent needs that email to create the ticket. `sanitizePII` at the model input degrades the task: the agent sees `[EMAIL_REDACTED]` and can't fulfill the request. Masking is applied **at the boundary**: logs, traces, tool output — everywhere data leaves the process and lands in storage that someone else can read.

The masking function itself looks like this:

```go
import "regexp"

func sanitizePII(text string) string {
    // Mask emails
    emailRegex := regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`)
    text = emailRegex.ReplaceAllString(text, "[EMAIL_REDACTED]")

    // Mask phone numbers (Russian and international formats)
    phoneRegex := regexp.MustCompile(`[\+]?[78]\s?[\(-]?\d{3}[\)-]?\s?\d{3}[-]?\d{2}[-]?\d{2}`)
    text = phoneRegex.ReplaceAllString(text, "[PHONE_REDACTED]")

    return text
}
```

> **Note:** The regexes above are a simplified example for learning. In production, use specialized libraries: [Microsoft Presidio](https://github.com/microsoft/presidio) for PII detection, [truffleHog](https://github.com/trufflesecurity/trufflehog) or [detect-secrets](https://github.com/Yelp/detect-secrets) for finding secrets in code. They cover dozens of data formats and are regularly updated.

### Step 2: Protect secrets

Never log secrets:

```go
func sanitizeSecrets(text string) string {
    // Remove patterns like "password: ..."
    secretRegex := regexp.MustCompile(`(?i)(password|api_key|token|secret)\s*[:=]\s*[\w-]+`)
    text = secretRegex.ReplaceAllString(text, "[SECRET_REDACTED]")
    
    return text
}
```

### Step 3: Log redaction

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

## Where to plug this into our code

### Integration point: the log boundary, not the model input

In [`labs/lab05-human-interaction/main.go`](https://github.com/kshvakov/ai-agent-course/blob/main/labs/lab05-human-interaction/main.go), user input goes to the model **as is** — the agent needs the data for the task. Masking happens when writing to the log/trace:

```go
// Do NOT mask the model input — otherwise the agent loses the client's email
messages = append(messages, llm.Message{Role: "user", Content: rawInput})

// Mask at the log/trace boundary (redacted attributes — ch. 19)
logWithRedaction(runID, map[string]any{"user_input": rawInput})
```

### Redaction boundaries: summary

| Boundary | What we do |
|---|---|
| **Model input** | Don't mask — masking the input degrades the task |
| **Logs** | Redaction by default: sanitize before writing (Step 3) |
| **Traces** | Redacted attributes + a hash for correlation; span implementation in [ch. 19](../19-observability-and-tracing/README.md) |
| **Tool output** | Masking by default: a tool must not return PII/secrets in plain text ([ch. 16](../16-best-practices/README.md), [ch. 17](../17-security-and-governance/README.md)) |
| **Storage** | TTL/retention for logs **and traces** (Mistake 3 below; ch. 19 writes traces to the same storage) |
| **Build** | Secrets come from vault/CI secrets at build and deploy time, not from code or the image env |
| **GDPR, right to erasure** | Log with a correlation ID (hash of the user identifier) → all of the user's traces/logs are found by ID search and deleted on request |

A couple of clarifications on the table:

- **Vault at build time.** API keys and passwords don't live in `main.go` and aren't baked into the image. CI fetches them from a vault (or GitHub/GitLab secrets) and hands them to the agent at runtime. Then an image leak ≠ a secrets leak.
- **GDPR and the correlation ID.** "Forget me" requires finding *all* records about a user. If every log and trace carries `user_hash = hash(userID)`, deletion is a single storage query by that ID, not log archaeology.

## Common Mistakes

### Mistake 1: PII ends up in logs

**Symptom:** User emails and phone numbers are visible in logs.

**Solution:** Mask PII before logging.

### Mistake 2: Secrets are logged

**Symptom:** API keys and passwords end up in logs.

**Solution:** Remove secrets from logs via redaction.

### Mistake 3: No data retention policy

**Symptom:** Logs grow indefinitely, disk space runs out. Old logs contain PII, but nobody cleans them up.

**Cause:** No data retention policy. Logs and traces are written without TTL and rotation.

**Solution:**
```go
// BAD: logs with no storage limit
func writeLog(entry LogEntry) {
    file.Write(entry) // The file grows forever
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

### Mistake 4: No encryption in transit

**Symptom:** Data between the agent and the LLM API travels over an unsecured channel. A man-in-the-middle attack intercepts requests with PII.

**Cause:** HTTP instead of HTTPS, missing TLS verification, self-signed certificates without checking.

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

### Mistake 5: PII in traces

**Symptom:** OpenTelemetry traces contain user data. Dashboards and alerts show emails and phone numbers.

**Cause:** Span attributes and logs are added without filtering.

**Solution:** This is the course's canonical redaction rule: redaction by default at the trace boundary. The OpenTelemetry span implementation (redacted attributes + a hash for correlation) is in [ch. 19](../19-observability-and-tracing/README.md):
```go
// BAD: PII ends up in span attributes
span.SetAttributes(
    attribute.String("user.input", userMessage), // Contains PII!
)

// GOOD: sanitize before adding to the trace
span.SetAttributes(
    attribute.String("user.input", sanitizePII(userMessage)),
    attribute.String("user.input_hash", hashForCorrelation(userMessage)),
)
```

## Exercises

### Exercise 1: PII detector

Implement a PII detector that finds emails and phone numbers in text and returns a list of matches:

```go
type PIIMatch struct {
    Type  string // "email", "phone"
    Value string // The found value
    Start int    // Start position
    End   int    // End position
}

func detectPII(text string) []PIIMatch {
    // Implement the search for emails and phone numbers
    // Support the formats: user@example.com, +7-999-123-4567, 8 (999) 123-45-67
}
```

**Expected result:**
- Finds email addresses in arbitrary text
- Finds phone numbers in various formats (with +7, 8, parentheses, hyphens)
- Returns match positions for targeted replacement

### Exercise 2: Middleware for log redaction

Create a middleware that automatically masks PII in all agent logs:

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
- Patterns are easy to extend (add INN, passport, card numbers)
- The middleware is transparent to the rest of the code

## Acceptance Criteria / Checklist

**Done:**
- [x] Model input is not masked (the agent gets task data)
- [x] PII is masked at the logs/traces boundary (redaction by default)
- [x] Tool output is masked by default
- [x] Secrets are not logged and are pulled from the vault at build time, not from code
- [x] Logs and traces are stored with TTL/retention
- [x] A correlation ID exists for the GDPR right to erasure

**Not done:**
- [ ] PII ends up in logs/traces in plain text
- [ ] Model input is masked (the agent loses task data)
- [ ] Secrets are logged or baked into the image

## Relation to Other Chapters

- **[Preface → Mental model](../00-preface/README.md#mental-model-an-agent-is-a-new-employee)** — The basic frame: data policies for an agent adapt data policies for humans.
- **[Chapter 17: Security and Governance](../17-security-and-governance/README.md)** — Threat modeling, RBAC, and onboarding discipline for the agent; data privacy is a special case of this frame.
- **[Chapter 19: Observability and Tracing](../19-observability-and-tracing/README.md)** — Safe logging: the rule "PII never reaches a span in plain text", redacted attributes + protected storage keyed by correlation ID.

## What's Next?

After Data and Privacy, move on to:
- **[25. Production Readiness Index](../25-production-readiness-index/README.md)** — Assess how ready your agent is for production
