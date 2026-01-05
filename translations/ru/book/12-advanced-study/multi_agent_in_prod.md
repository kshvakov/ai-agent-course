# Multi-Agent в продакшене

## Зачем это нужно?

Multi-Agent система работает, но вы не можете понять, где произошла ошибка в цепочке Supervisor → Worker → Tool. Без прод-готовности Multi-Agent вы не можете отлаживать сложные системы.

### Реальный кейс

**Ситуация:** Supervisor направил задачу Network Admin Worker, но задача не выполнилась. Вы не можете понять, где произошла ошибка.

**Проблема:** Нет трейсинга через цепочку агентов, нет изоляции контекста, нет кореляции логов.

**Решение:** Трейсинг через цепочку агентов, изоляция контекста, кореляция логов через `run_id`, контуры безопасности для каждого Worker.

## Теория простыми словами

### Что такое изоляция контекста?

Изоляция контекста — это отдельный контекст для каждого Worker. Supervisor имеет свой контекст, каждый Worker — свой. Это предотвращает "переполнение" контекста.

### Что такое трейсинг цепочки?

Трейсинг цепочки — это отслеживание полного пути запроса через Supervisor → Worker → Tool. Каждый шаг имеет свой span в трейсе.

## Как это работает (пошагово)

### Шаг 1: Трейсинг через цепочку

Трассируйте цепочку Supervisor → Worker → Tool:

```go
func (s *Supervisor) ExecuteTaskWithTracing(ctx context.Context, task string) (string, error) {
    span := trace.StartSpan("supervisor.route_task")
    defer span.End()
    
    worker, err := s.RouteTask(task)
    if err != nil {
        span.RecordError(err)
        return "", err
    }
    
    span.SetAttributes(
        attribute.String("worker.name", worker.name),
    )
    
    // Worker выполняет с трейсингом
    result, err := worker.ExecuteWithTracing(ctx, task)
    if err != nil {
        span.RecordError(err)
        return "", err
    }
    
    return result, nil
}
```

### Шаг 2: Изоляция контекста

Каждый Worker имеет свой изолированный контекст:

```go
type Worker struct {
    name         string
    systemPrompt string
    tools        []openai.Tool
    // Изолированный контекст для этого Worker
}

func (w *Worker) Execute(task string) (string, error) {
    // Worker использует свой systemPrompt и tools
    messages := []openai.ChatCompletionMessage{
        {Role: "system", Content: w.systemPrompt},
        {Role: "user", Content: task},
    }
    // ... выполнение ...
}
```

## Где это встраивать в нашем коде

### Точка интеграции: Multi-Agent System

В `labs/lab08-multi-agent/main.go` добавьте трейсинг и изоляцию контекста:

```go
func (s *Supervisor) ExecuteTask(ctx context.Context, task string) (string, error) {
    runID := generateRunID()
    
    // Трейсинг Supervisor
    log.Printf("SUPERVISOR_START: run_id=%s task=%s", runID, task)
    
    worker := s.RouteTask(task)
    
    // Трейсинг Worker
    log.Printf("WORKER_START: run_id=%s worker=%s", runID, worker.name)
    
    result, err := worker.Execute(task)
    
    log.Printf("WORKER_END: run_id=%s worker=%s result=%s", runID, worker.name, result)
    log.Printf("SUPERVISOR_END: run_id=%s", runID)
    
    return result, err
}
```

## Типовые ошибки

### Ошибка 1: Нет трейсинга цепочки

**Симптом:** Невозможно понять, где произошла ошибка в цепочке Supervisor → Worker → Tool.

**Решение:** Трассируйте каждый шаг цепочки с `run_id`.

### Ошибка 2: Нет изоляции контекста

**Симптом:** Контекст переполняется, агенты путаются между задачами.

**Решение:** Каждый Worker имеет свой изолированный контекст.

## Критерии сдачи / Чек-лист

✅ **Сдано:**
- Трейсинг через цепочку агентов
- Изоляция контекста для каждого Worker
- Кореляция логов через `run_id`

❌ **Не сдано:**
- Нет трейсинга цепочки
- Нет изоляции контекста

## Связь с другими главами

- **Multi-Agent:** Базовые концепции — [Глава 08: Multi-Agent Systems](../08-multi-agent/README.md)
- **Observability:** Трейсинг как часть observability — [Observability и Tracing](observability.md)

---

**Навигация:** [← Evals в CI/CD](evals_in_cicd.md) | [Оглавление главы 12](README.md) | [Модель и декодинг →](model_and_decoding.md)

