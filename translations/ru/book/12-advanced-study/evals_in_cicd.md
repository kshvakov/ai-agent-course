# Evals в CI/CD

## Зачем это нужно?

Вы изменили промпт или код, и агент стал работать хуже. Но вы узнаёте об этом только после деплоя в прод. Без evals в CI/CD вы не можете автоматически проверять качество перед деплоем.

### Реальный кейс

**Ситуация:** Вы обновили системный промпт и задеплоили изменения. Через день пользователи жалуются, что агент стал хуже работать.

**Проблема:** Нет автоматической проверки качества перед деплоем. Изменения деплоятся без тестирования.

**Решение:** Evals в CI/CD pipeline, quality gates, блокировка деплоя при ухудшении метрик. Теперь плохие изменения не попадают в прод.

## Теория простыми словами

### Что такое Quality Gates?

Quality Gates — это проверки качества, которые блокируют деплой, если метрики ухудшились.

## Как это работает (пошагово)

### Шаг 1: Quality Gates в CI/CD

Интегрируйте evals в CI/CD pipeline:

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

### Шаг 2: Версионирование датасетов

Храните "золотые" сценарии в датасете:

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

## Где это встраивать в нашем коде

### Точка интеграции: CI/CD Pipeline

Создайте отдельный файл `cmd/evals/main.go` для запуска evals:

```go
func main() {
    passRate := runEvals()
    if passRate < 0.95 {
        os.Exit(1)
    }
}
```

## Типовые ошибки

### Ошибка 1: Evals не интегрированы в CI/CD

**Симптом:** Evals запускаются вручную, плохие изменения попадают в прод.

**Решение:** Интегрируйте evals в CI/CD pipeline.

## Критерии сдачи / Чек-лист

✅ **Сдано:**
- Evals интегрированы в CI/CD
- Quality gates блокируют деплой при ухудшении

❌ **Не сдано:**
- Evals не интегрированы в CI/CD

## Связь с другими главами

- **Evals:** Базовые концепции evals — [Глава 09: Evals и Надежность](../09-evals-and-reliability/README.md)
- **Prompt Management:** Проверка промптов через evals — [Prompt и Program Management](prompt_program_mgmt.md)

---

**Навигация:** [← RAG в продакшене](rag_in_prod.md) | [Оглавление главы 12](README.md) | [Multi-Agent в продакшене →](multi_agent_in_prod.md)

