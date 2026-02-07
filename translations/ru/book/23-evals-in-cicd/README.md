# Evals в CI/CD

## Зачем это нужно?

Вы изменили промпт или код, и агент стал работать хуже. Но вы узнаёте об этом только после деплоя в прод. Без evals в CI/CD вы не сможете автоматически проверять качество перед деплоем.

### Реальный кейс

**Ситуация:** Вы обновили системный промпт и задеплоили изменения. Через день пользователи жалуются, что агент стал хуже работать.

**Проблема:** Нет автоматической проверки качества перед деплоем. Изменения деплоятся без тестирования.

**Решение:** Evals в CI/CD pipeline, quality gates и блокировка деплоя при ухудшении метрик. Теперь плохие изменения не попадают в прод.

## Теория простыми словами

### Что такое Quality Gates?

Quality Gates — это проверки качества, которые блокируют деплой, если метрики ухудшились.

## Как это работает (пошагово)

### Шаг 1: Quality Gates в CI/CD

Интегрируйте evals в CI/CD pipeline:

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
    # Запускаем evals и сохраняем результат
    - |
      PASS_RATE=$(go run cmd/evals/main.go 2>&1 | grep -oP 'Pass rate: \K[0-9.]+' || echo "0")
      echo "Pass rate: $PASS_RATE"
      # Проверяем quality gate
      if (( $(echo "$PASS_RATE < 0.95" | bc -l) )); then
        echo "Quality gate failed: Pass rate $PASS_RATE < 0.95"
        exit 1
      fi
  only:
    - merge_requests
  tags:
    - docker
```

**Альтернативный вариант** (если `cmd/evals/main.go` экспортирует переменную окружения):

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

**Примечание:** Убедитесь, что `cmd/evals/main.go` выводит результат в формате, который можно распарсить (например, `Pass rate: 0.95`), или экспортирует переменную окружения `PASS_RATE`.

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
package main

import (
    "fmt"
    "os"
)

func main() {
    passRate := runEvals()
    
    // Выводим результат в формате, который можно распарсить в CI/CD
    fmt.Printf("Pass rate: %.2f\n", passRate)
    
    // Экспортируем переменную окружения для удобства (работает в обеих системах)
    os.Setenv("PASS_RATE", fmt.Sprintf("%.2f", passRate))
    
    // Quality gate: если pass rate ниже порога, завершаем с ошибкой
    if passRate < 0.95 {
        fmt.Printf("Quality gate failed: Pass rate %.2f < 0.95\n", passRate)
        os.Exit(1)
    }
    
    fmt.Println("Quality gate passed!")
}
```

**Примечание:** Этот пример работает как с GitHub Actions, так и с GitLab CI/CD. Переменная окружения `PASS_RATE` доступна в обоих случаях, а вывод в формате `Pass rate: 0.95` позволяет парсить результат через `grep` или другие инструменты.

## Типовые ошибки

### Ошибка 1: Evals не интегрированы в CI/CD

**Симптом:** Evals запускаются вручную, плохие изменения попадают в прод.

**Решение:** Интегрируйте evals в CI/CD pipeline.

## Критерии сдачи / Чек-лист

**Сдано:**
- [x] Evals интегрированы в CI/CD
- [x] Quality gates блокируют деплой при ухудшении

**Не сдано:**
- [ ] Evals не интегрированы в CI/CD

## Связь с другими главами

- **[Глава 08: Evals и Надежность](../08-evals-and-reliability/README.md)** — Базовые концепции evals
- **[Глава 22: Prompt и Program Management](../22-prompt-program-management/README.md)** — Проверка промптов через evals


