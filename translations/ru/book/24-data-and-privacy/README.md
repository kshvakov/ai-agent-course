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

## Критерии сдачи / Чек-лист

✅ **Сдано:**
- PII маскируется перед отправкой в LLM
- Секреты не логируются
- Логи проходят redaction

❌ **Не сдано:**
- PII не маскируется
- Секреты логируются

## Связь с другими главами

- **[Глава 17: Security и Governance](../17-security-and-governance/README.md)** — Защита данных
- **[Глава 19: Observability и Tracing](../19-observability-and-tracing/README.md)** — Безопасное логирование

---

**Навигация:** [← Evals в CI/CD](../23-evals-in-cicd/README.md) | [Оглавление](../README.md) | [Индекс Прод-готовности →](../25-production-readiness-index/README.md)

