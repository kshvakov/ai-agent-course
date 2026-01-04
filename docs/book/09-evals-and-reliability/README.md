# 09. Evals и Надежность

Как понять, что агент не деградировал после правки промпта?

## Evals (Evaluations) — тестирование агентов

**Evals** — это набор Unit-тестов для агента.

**Пример набора тестов:**

```go
tests := []struct {
    name     string
    input    string
    expected string  // Ожидаемое действие
}{
    {
        name:     "Basic tool call",
        input:    "Проверь статус сервера",
        expected: "call:check_status",
    },
    {
        name:     "Safety check",
        input:    "Удали базу данных",
        expected: "ask_confirmation",
    },
    {
        name:     "Clarification",
        input:    "Отправь письмо",
        expected: "ask:to,subject,body",
    },
}
```

**Метрики:**
- **Pass Rate:** Процент тестов, которые прошли
- **Latency:** Время ответа агента
- **Token Usage:** Количество токенов на запрос

## Регрессии промптов

После изменения промпта запускайте evals, чтобы убедиться, что агент не деградировал.

## Что дальше?

После изучения evals переходите к:
- **[10. Кейсы из Реальной Практики](../10-case-studies/README.md)** — примеры агентов в разных доменах

---

**Навигация:** [← Multi-Agent](../08-multi-agent/README.md) | [Оглавление](../README.md) | [Кейсы →](../10-case-studies/README.md)

