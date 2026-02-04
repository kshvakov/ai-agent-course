# 10. Planning и Workflow-паттерны

## Зачем это нужно?

Простые ReAct циклы хорошо работают для прямолинейных задач. Но как только задача становится многошаговой, обычно нужно планирование: разбить работу на шаги, учесть зависимости, пережить сбои и не потерять прогресс.

В этой главе разберём паттерны планирования, которые помогают агентам справляться со сложными и долго выполняющимися задачами.

### Реальный кейс

**Ситуация:** Пользователь просит: "Разверни новый микросервис: создай VM, установи зависимости, настрой сеть, разверни приложение, настрой мониторинг."

**Проблема:** Простой ReAct цикл может:
- Прыгать между шагами случайно
- Пропускать зависимости (пытаться развернуть до создания VM)
- Не отслеживать, какие шаги завершены
- Падать и начинать с нуля

**Решение:** Паттерн планирования: агент сначала строит план (шаги + зависимости), потом выполняет его по порядку, отслеживая состояние и корректно обрабатывая сбои.

## Теория простыми словами

### Что такое Planning?

Planning — это процесс разбиения сложной задачи на меньшие, управляемые шаги с чёткими зависимостями и порядком выполнения.

**Ключевые компоненты:**
1. **Декомпозиция задачи** — Разбить большую задачу на шаги
2. **Граф зависимостей** — Понять, какие шаги зависят от других
3. **Порядок выполнения** — Определить последовательность (или параллельное выполнение)
4. **Отслеживание состояния** — Знать, что сделано, что в процессе, что упало
5. **Обработка сбоев** — Повтор, пропуск или прерывание при ошибках

### Паттерны планирования

**Паттерн 1: Plan→Execute**
- Агент создаёт полный план заранее
- Выполняет шаги последовательно
- Просто, но негибко

**Паттерн 2: Plan-and-Revise**
- Агент создаёт начальный план
- Пересматривает план по мере обучения (например, шаг упал, обнаружена новая информация)
- Более адаптивно, но сложнее

**Паттерн 3: DAG/Workflow**
- Шаги образуют направленный ациклический граф
- Некоторые шаги могут выполняться параллельно
- Обрабатывает сложные зависимости

## Как это работает (пошагово)

### Шаг 1: Декомпозиция задачи

Агент получает задачу высокого уровня и разбивает её на шаги:

```go
type Plan struct {
    Steps []Step
}

type Step struct {
    ID          string
    Description string
    Dependencies []string  // ID шагов, которые должны завершиться первыми
    Status      StepStatus
    Result      any
    Error       error
}

type StepStatus string

const (
    StepStatusPending   StepStatus = "pending"
    StepStatusRunning   StepStatus = "running"
    StepStatusCompleted StepStatus = "completed"
    StepStatusFailed    StepStatus = "failed"
    StepStatusSkipped   StepStatus = "skipped"
)
```

**Пример:** "Развернуть микросервис" разбивается на:
1. Создать VM (нет зависимостей)
2. Установить зависимости (зависит от: Создать VM)
3. Настроить сеть (зависит от: Создать VM)
4. Развернуть приложение (зависит от: Установить зависимости, Настроить сеть)
5. Настроить мониторинг (зависит от: Развернуть приложение)

### Шаг 2: Создать план

Агент использует LLM для декомпозиции задачи:

```go
func createPlan(ctx context.Context, client *openai.Client, task string) (*Plan, error) {
    prompt := fmt.Sprintf(`Разбей эту задачу на шаги с зависимостями:
Задача: %s

Верни JSON с массивом steps. Каждый шаг имеет: id, description, dependencies (массив ID шагов).

Пример:
{
  "steps": [
    {"id": "step1", "description": "Создать VM", "dependencies": []},
    {"id": "step2", "description": "Установить зависимости", "dependencies": ["step1"]}
  ]
}`, task)

    messages := []openai.ChatCompletionMessage{
        {Role: "system", Content: "Ты агент планирования. Разбивай задачи на шаги."},
        {Role: "user", Content: prompt},
    }

    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model:    openai.GPT3Dot5Turbo,
        Messages: messages,
        Temperature: 0, // Детерминированное планирование
    })
    if err != nil {
        return nil, err
    }

    // Парсим JSON ответ в Plan
    var plan Plan
    json.Unmarshal([]byte(resp.Choices[0].Message.Content), &plan)
    return &plan, nil
}
```

### Шаг 3: Выполнить план

Выполняем шаги с учётом зависимостей:

```go
func executePlan(ctx context.Context, plan *Plan, executor StepExecutor) error {
    for {
        // Находим шаги, готовые к выполнению (все зависимости завершены)
        readySteps := findReadySteps(plan)
        
        if len(readySteps) == 0 {
            // Проверяем, все ли завершены или застряли
            if allStepsCompleted(plan) {
                return nil
            }
            if allRemainingStepsBlocked(plan) {
                return fmt.Errorf("план заблокирован: некоторые шаги упали")
            }
            // Ждём асинхронные шаги или повторяем упавшие шаги
            continue
        }

        // Выполняем готовые шаги (могут быть параллельными)
        for _, step := range readySteps {
            step.Status = StepStatusRunning
            result, err := executor.Execute(ctx, step)
            
            if err != nil {
                step.Status = StepStatusFailed
                step.Error = err
                // Решаем: повторить, пропустить или прервать
                if shouldRetry(step) {
                    step.Status = StepStatusPending
                    continue
                }
            } else {
                step.Status = StepStatusCompleted
                step.Result = result
            }
        }
    }
}

func findReadySteps(plan *Plan) []*Step {
    ready := make([]*Step, 0, len(plan.Steps))
    for i := range plan.Steps {
        step := &plan.Steps[i]
        if step.Status != StepStatusPending {
            continue
        }
        
        // Проверяем, все ли зависимости завершены
        allDepsDone := true
        for _, depID := range step.Dependencies {
            dep := findStep(plan, depID)
            if dep == nil || dep.Status != StepStatusCompleted {
                allDepsDone = false
                break
            }
        }
        
        if allDepsDone {
            ready = append(ready, step)
        }
    }
    return ready
}
```

### Шаг 4: Обработка сбоев

Реализуем логику повторных попыток с экспоненциальным backoff:

```go
type StepExecutor interface {
    Execute(ctx context.Context, step *Step) (any, error)
}

func executeWithRetry(ctx context.Context, executor StepExecutor, step *Step, maxRetries int) (any, error) {
    var lastErr error
    backoff := time.Second
    
    for attempt := 0; attempt <= maxRetries; attempt++ {
        if attempt > 0 {
            // Экспоненциальный backoff
            time.Sleep(backoff)
            backoff *= 2
        }
        
        result, err := executor.Execute(ctx, step)
        if err == nil {
            return result, nil
        }
        
        lastErr = err
        // Проверяем, можно ли повторить ошибку
        if !isRetryableError(err) {
            return nil, err
        }
    }
    
    return nil, fmt.Errorf("не удалось после %d попыток: %w", maxRetries, lastErr)
}
```

### Шаг 5: Сохранение состояния плана

**ВАЖНО:** Сохранение состояния для возобновления выполнения описано в [State Management](../11-state-management/README.md). Здесь описывается только структура состояния плана.

```go
// Состояние плана используется для отслеживания прогресса
// Сохранение и возобновление описано в State Management
type PlanState struct {
    PlanID    string
    Steps     []Step
    UpdatedAt time.Time
}
```

## Типовые ошибки

### Ошибка 1: Нет отслеживания зависимостей

**Симптом:** Агент пытается выполнить шаги не по порядку, вызывая сбои.

**Причина:** Не отслеживаются зависимости между шагами.

**Решение:**
```go
// ПЛОХО: Выполняем шаги по порядку без проверки зависимостей
for _, step := range plan.Steps {
    executor.Execute(ctx, step)
}

// ХОРОШО: Сначала проверяем зависимости
readySteps := findReadySteps(plan)
for _, step := range readySteps {
    executor.Execute(ctx, step)
}
```

### Ошибка 2: Нет сохранения состояния

**Симптом:** Агент начинает с нуля после сбоя, теряя прогресс.

**Причина:** Состояние плана не сохраняется.

**Решение:** Используйте техники из [State Management](../11-state-management/README.md) для сохранения и возобновления выполнения плана.

### Ошибка 3: Бесконечные повторы

**Симптом:** Агент повторяет упавший шаг вечно, тратя ресурсы.

**Причина:** Нет лимитов повторов или backoff.

**Решение:** Реализуйте максимальное количество повторов и экспоненциальный backoff.

### Ошибка 4: Нет параллельного выполнения

**Симптом:** Агент выполняет независимые шаги последовательно, тратя время.

**Причина:** Не определяются шаги, которые могут выполняться параллельно.

**Решение:** Используйте `findReadySteps` для получения всех готовых шагов, выполняйте их конкурентно:

```go
// Выполняем готовые шаги параллельно
var wg sync.WaitGroup
for _, step := range readySteps {
    wg.Add(1)
    go func(s *Step) {
        defer wg.Done()
        executor.Execute(ctx, s)
    }(step)
}
wg.Wait()
```

## Паттерн: Controller + Processor (оркестратор + нормализатор)

Когда workflow сложный и инструментов много, полезно разделить две разные задачи:

- **Controller (оркестратор)** выбирает следующий шаг: вызвать инструмент или ответить пользователю.
- **Processor (аналитик/нормализатор)** преобразует результаты инструментов и ответы пользователя в структурное обновление состояния (например: "добавь факты", "обнови план", "добавь открытые вопросы").

Так вы снижаете "хаос" в agent loop: controller не тонет в больших данных, а processor не принимает решений о сайд-эффектах.

Мини-трасса (read-only поиск + чтение файла):

1) Controller вызывает поиск.

```json
{
  "tool_call": {
    "name": "search_code",
    "arguments": { "query": "type ClientError struct" }
  }
}
```

2) ToolRunner сохраняет сырой результат как артефакт и возвращает короткий payload (top-k совпадений).

3) Processor возвращает `state_patch`:

```json
{
  "replace_plan": [
    "Прочитать файл с лучшим совпадением",
    "Сформировать краткое объяснение пользователю"
  ],
  "append_known_facts": [
    {
      "key": "client_error_candidate",
      "value": "pkg/errors/client_error.go:12",
      "source": "tool",
      "artifact_id": "srch_123",
      "confidence": 0.9
    }
  ]
}
```

4) Controller читает файл по плану и формирует финальный ответ.

## Мини-упражнения

### Упражнение 1: Декомпозиция задачи

Реализуйте функцию, которая разбивает задачу на шаги:

```go
func decomposeTask(task string) (*Plan, error) {
    // Используйте LLM для создания плана
    // Верните Plan с шагами и зависимостями
}
```

**Ожидаемый результат:**
- План содержит логические шаги
- Зависимости правильно определены
- Шаги могут выполняться в валидном порядке

### Упражнение 2: Разрешение зависимостей

Реализуйте `findReadySteps`, который возвращает шаги, все зависимости которых завершены:

```go
func findReadySteps(plan *Plan) []*Step {
    // Ваша реализация
}
```

**Ожидаемый результат:**
- Возвращает только шаги со всеми удовлетворёнными зависимостями
- Обрабатывает циклические зависимости (обнаруживает и ошибки)

### Упражнение 3: Выполнение плана с повторами

Реализуйте выполнение плана с логикой повторов:

```go
func executePlanWithRetries(ctx context.Context, plan *Plan, executor StepExecutor, maxRetries int) error {
    // Выполните план с логикой повторов
    // Обработайте сбои корректно
}
```

**Ожидаемый результат:**
- Шаги выполняются с учётом зависимостей
- Упавшие шаги повторяются до maxRetries
- План завершается или корректно падает

## Критерии сдачи / Чек-лист

✅ **Сдано:**
- Можете разбить сложные задачи на шаги
- Понимаете графы зависимостей
- Можете выполнять планы с учётом зависимостей
- Обрабатываете сбои с повторами
- Сохраняете состояние плана для возобновления

❌ **Не сдано:**
- Выполнение шагов без проверки зависимостей
- Нет сохранения состояния
- Бесконечные повторы без лимитов
- Последовательное выполнение, когда возможно параллельное

## Связь с другими главами

- **[Глава 04: Автономность и Циклы](../04-autonomy-and-loops/README.md)** — Планирование расширяет ReAct цикл для сложных задач
- **[Глава 07: Multi-Agent Systems](../07-multi-agent/README.md)** — Планирование может координировать несколько агентов
- **[Глава 11: State Management](../11-state-management/README.md)** — Надёжное выполнение планов (идемпотентность, retries, persist)
- **[Глава 21: Workflow и State Management в продакшене](../21-workflow-state-management/README.md)** — Прод-паттерны workflow

**ВАЖНО:** Planning фокусируется на **декомпозиции задач** и **графах зависимостей**. Надёжность выполнения (persist, retries, дедлайны) описана в [State Management](../11-state-management/README.md).

## Что дальше?

После освоения паттернов планирования переходите к:
- **[11. State Management](../11-state-management/README.md)** — Узнайте, как гарантировать надёжное выполнение планов


