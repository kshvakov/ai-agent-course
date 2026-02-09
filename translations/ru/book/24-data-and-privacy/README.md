# 24. Data и Privacy

## Зачем это нужно?

Агент работает с персональными данными пользователей (email, телефон, адрес). Эти данные попадают в логи и отправляются в LLM API. Без защиты данных вы нарушаете GDPR и рискуете утечкой данных.

### Реальный кейс

**Ситуация:** Агент обрабатывает запрос пользователя: "Мой email john@example.com, телефон +7-999-123-4567. Создай тикет".

**Проблема:** PII попадает в логи и отправляется в LLM API без маскирования. При утечке логов персональные данные попадают в чужие руки.

**Решение:** Обнаружение и маскирование PII перед логированием и отправкой в LLM, защита секретов, redaction логов, TTL для хранения.


## Теория простыми словами

### Что такое PII?

PII (Personally Identifiable Information) — это данные, которые позволяют идентифицировать человека: email, телефон, адрес, паспорт.

### Что такое redaction?

Redaction — это удаление чувствительных данных из логов перед сохранением.

## Как это работает (пошагово)

### Шаг 1: Обнаружение и маскирование PII

Маскируйте PII перед отправкой в LLM:

```go
import "regexp"

func sanitizePII(text string) string {
    // Маскируем email
    emailRegex := regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`)
    text = emailRegex.ReplaceAllString(text, "[EMAIL_REDACTED]")
    
    // Маскируем телефон
    phoneRegex := regexp.MustCompile(`\b\d{3}-\d{3}-\d{4}\b`)
    text = phoneRegex.ReplaceAllString(text, "[PHONE_REDACTED]")
    
    return text
}
```

### Шаг 2: Защита секретов

Никогда не логируйте секреты:

```go
func sanitizeSecrets(text string) string {
    // Удаляем паттерны типа "password: ..."
    secretRegex := regexp.MustCompile(`(?i)(password|api_key|token|secret)\s*[:=]\s*[\w-]+`)
    text = secretRegex.ReplaceAllString(text, "[SECRET_REDACTED]")
    
    return text
}
```

### Шаг 3: Redaction логов

Удаляйте чувствительные данные из логов:

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

## Где это встраивать в нашем коде

### Точка интеграции: User Input

В `labs/lab05-human-interaction/main.go` санитизируйте входные данные:

```go
userInput := sanitizePII(sanitizeSecrets(rawInput))
messages = append(messages, openai.ChatCompletionMessage{
    Role: "user",
    Content: userInput,
})
```

## Типовые ошибки

### Ошибка 1: PII попадает в логи

**Симптом:** Email и телефоны пользователей видны в логах.

**Решение:** Маскируйте PII перед логированием.

### Ошибка 2: Секреты логируются

**Симптом:** API ключи и пароли попадают в логи.

**Решение:** Удаляйте секреты из логов через redaction.

### Ошибка 3: Нет Data Retention Policy

**Симптом:** Логи растут бесконечно, дисковое пространство заканчивается. Старые логи содержат PII, но никто их не чистит.

**Причина:** Нет политики хранения данных. Логи и трассировки записываются без TTL и ротации.

**Решение:**
```go
// ПЛОХО: логи без ограничения хранения
func writeLog(entry LogEntry) {
    file.Write(entry) // Файл растёт бесконечно
}

// ХОРОШО: TTL и ротация
type RetentionPolicy struct {
    MaxAge    time.Duration // Максимальный срок хранения
    MaxSizeMB int          // Максимальный размер в MB
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

### Ошибка 4: Отсутствие шифрования в Transit

**Симптом:** Данные между агентом и LLM API передаются по незащищённому каналу. Man-in-the-middle атака перехватывает запросы с PII.

**Причина:** HTTP вместо HTTPS, отсутствие TLS-верификации, самоподписанные сертификаты без проверки.

**Решение:**
```go
// ПЛОХО: HTTP без шифрования
client := &http.Client{}
resp, _ := client.Post("http://api.llm.example.com/v1/chat", ...)

// ХОРОШО: HTTPS + проверка сертификатов
client := &http.Client{
    Transport: &http.Transport{
        TLSClientConfig: &tls.Config{
            MinVersion: tls.VersionTLS12,
        },
    },
}
resp, _ := client.Post("https://api.llm.example.com/v1/chat", ...)
```

### Ошибка 5: PII в трассировках

**Симптом:** OpenTelemetry трассировки содержат пользовательские данные. Dashboards и алерты показывают email и телефоны.

**Причина:** Span-атрибуты и логи добавляются без фильтрации.

**Решение:**
```go
// ПЛОХО: PII попадает в span-атрибуты
span.SetAttributes(
    attribute.String("user.input", userMessage), // Содержит PII!
)

// ХОРОШО: санитизация перед добавлением в трассировку
span.SetAttributes(
    attribute.String("user.input", sanitizePII(userMessage)),
    attribute.String("user.input_hash", hashForCorrelation(userMessage)),
)
```

## Мини-упражнения

### Упражнение 1: Детектор PII

Реализуйте детектор PII, который находит email и телефоны в тексте и возвращает список найденных совпадений:

```go
type PIIMatch struct {
    Type  string // "email", "phone"
    Value string // Найденное значение
    Start int    // Позиция начала
    End   int    // Позиция конца
}

func detectPII(text string) []PIIMatch {
    // Реализуйте поиск email и телефонов
    // Поддержите форматы: user@example.com, +7-999-123-4567, 8 (999) 123-45-67
}
```

**Ожидаемый результат:**
- Находит email-адреса в произвольном тексте
- Находит телефоны в разных форматах (с +7, 8, скобками, дефисами)
- Возвращает позиции совпадений для точечной замены

### Упражнение 2: Middleware для Redaction логов

Создайте middleware, который автоматически маскирует PII во всех логах агента:

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
    // Реализуйте фильтрацию всех атрибутов записи
}
```

**Ожидаемый результат:**
- Все строковые атрибуты проходят через redaction
- Паттерны легко расширяются (добавить ИНН, паспорт, карту)
- Middleware прозрачен для остального кода

## Критерии сдачи / Чек-лист

**Сдано:**
- [x] PII маскируется перед отправкой в LLM
- [x] Секреты не логируются
- [x] Логи проходят redaction

**Не сдано:**
- [ ] PII не маскируется
- [ ] Секреты логируются

## Связь с другими главами

- **[Глава 17: Security и Governance](../17-security-and-governance/README.md)** — Защита данных
- **[Глава 19: Observability и Tracing](../19-observability-and-tracing/README.md)** — Безопасное логирование

## Что дальше?

После понимания Data и Privacy переходите к:
- **[25. Production Readiness Index](../25-production-readiness-index/README.md)** — Оцените готовность вашего агента к продакшену


