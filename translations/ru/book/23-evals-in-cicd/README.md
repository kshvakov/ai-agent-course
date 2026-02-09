# 23. Evals в CI/CD

## Зачем это нужно?

Вы изменили промпт или код, и агент стал работать хуже. Но узнаёте об этом только после деплоя. Без evals в CI/CD плохие изменения попадают в прод.

В [Главе 08](../08-evals-and-reliability/README.md) мы написали тесты для агента. Теперь встроим их в CI/CD pipeline и добавим четырёхуровневую систему оценки.

### Реальный кейс

**Ситуация:** Вы обновили системный промпт и задеплоили. Через день пользователи жалуются, что агент выбирает неправильные инструменты.

**Проблема:** Evals проверяли только "задача выполнена" (Task Level). Но не проверяли "правильный ли инструмент выбран" (Tool Level).

**Решение:** Четырёхуровневая система evals в CI/CD: Task → Tool → Trajectory → Topic. Quality gates блокируют деплой при ухудшении на любом уровне.

## Теория простыми словами

### Четырёхуровневая система оценки

Одна метрика "pass/fail" недостаточна. Агент может выполнить задачу, но неэффективно (лишние tool calls), небезопасно (обошёл проверки) или некорректно (правильный ответ случайно).

| Уровень | Что оценивает | Пример метрики |
|---------|---------------|----------------|
| **Task Level** | Задача выполнена корректно? | Pass rate, answer correctness |
| **Tool Level** | Правильный инструмент выбран? Аргументы верны? | Tool selection accuracy, argument validity |
| **Trajectory Level** | Путь выполнения оптимален? | Step count, unnecessary tool calls, loops |
| **Topic Level** | Качество в конкретном домене | Domain-specific metrics (e.g., SQL validity) |

### Quality Gates

Quality Gate — проверка, которая блокирует деплой при ухудшении метрик. Каждый уровень имеет свой порог.

## Как это работает (пошагово)

### Шаг 1: Структура eval-кейса с уровнями

```go
type EvalCase struct {
    ID       string `json:"id"`
    Input    string `json:"input"`   // Запрос пользователя
    Topic    string `json:"topic"`   // Домен: "devops", "database", "security"

    // Task Level
    ExpectedOutput   string   `json:"expected_output"`    // Ожидаемый финальный ответ (или паттерн)
    MustContain      []string `json:"must_contain"`       // Строки, которые должны быть в ответе

    // Tool Level
    ExpectedTools    []string `json:"expected_tools"`     // Какие инструменты должны быть вызваны
    ForbiddenTools   []string `json:"forbidden_tools"`    // Какие инструменты НЕ должны быть вызваны
    ExpectedArgs     map[string]json.RawMessage `json:"expected_args"` // Ожидаемые аргументы

    // Trajectory Level
    MaxSteps         int      `json:"max_steps"`          // Максимальное число шагов
    MustNotLoop      bool     `json:"must_not_loop"`      // Не должен зацикливаться
}
```

### Шаг 2: Запись траектории выполнения

Для оценки на всех уровнях нужно записывать полный путь агента:

```go
type AgentTrajectory struct {
    RunID    string          `json:"run_id"`
    Steps    []TrajectoryStep `json:"steps"`
    Duration time.Duration   `json:"duration"`
    Tokens   int             `json:"tokens"`
}

type TrajectoryStep struct {
    Iteration int    `json:"iteration"`
    Type      string `json:"type"` // "tool_call", "tool_result", "final_answer"
    ToolName  string `json:"tool_name,omitempty"`
    ToolArgs  string `json:"tool_args,omitempty"`
    Result    string `json:"result,omitempty"`
}

// Запись траектории в agent loop
func runAgentWithTracing(input string, tools []openai.Tool) (string, AgentTrajectory) {
    var trajectory AgentTrajectory
    trajectory.RunID = generateRunID()

    for i := 0; i < maxIterations; i++ {
        resp, _ := client.CreateChatCompletion(ctx, req)
        msg := resp.Choices[0].Message

        if len(msg.ToolCalls) == 0 {
            trajectory.Steps = append(trajectory.Steps, TrajectoryStep{
                Iteration: i, Type: "final_answer", Result: msg.Content,
            })
            return msg.Content, trajectory
        }

        for _, tc := range msg.ToolCalls {
            trajectory.Steps = append(trajectory.Steps, TrajectoryStep{
                Iteration: i, Type: "tool_call",
                ToolName: tc.Function.Name, ToolArgs: tc.Function.Arguments,
            })
            result := executeTool(tc)
            trajectory.Steps = append(trajectory.Steps, TrajectoryStep{
                Iteration: i, Type: "tool_result",
                ToolName: tc.Function.Name, Result: result,
            })
        }
    }
    return "", trajectory
}
```

### Шаг 3: Оценка на четырёх уровнях

```go
type EvalResult struct {
    CaseID string `json:"case_id"`

    // Task Level
    TaskPass     bool    `json:"task_pass"`
    TaskScore    float64 `json:"task_score"`    // 0.0 - 1.0

    // Tool Level
    ToolPass     bool    `json:"tool_pass"`
    ToolAccuracy float64 `json:"tool_accuracy"` // % правильных tool calls

    // Trajectory Level
    TrajectoryPass bool  `json:"trajectory_pass"`
    StepCount      int   `json:"step_count"`
    HasLoops       bool  `json:"has_loops"`

    // Topic Level
    TopicPass    bool    `json:"topic_pass"`
    TopicScore   float64 `json:"topic_score"`
}

func evaluateCase(c EvalCase, answer string, traj AgentTrajectory) EvalResult {
    result := EvalResult{CaseID: c.ID}

    // --- Task Level ---
    result.TaskPass = checkTaskCompletion(c, answer)
    result.TaskScore = scoreAnswer(c.ExpectedOutput, answer)

    // --- Tool Level ---
    usedTools := extractToolNames(traj)
    result.ToolAccuracy = toolSelectionAccuracy(c.ExpectedTools, usedTools)
    result.ToolPass = result.ToolAccuracy >= 0.8 && !containsForbidden(usedTools, c.ForbiddenTools)

    // --- Trajectory Level ---
    result.StepCount = len(traj.Steps)
    result.HasLoops = detectLoops(traj)
    result.TrajectoryPass = result.StepCount <= c.MaxSteps && !result.HasLoops

    // --- Topic Level ---
    result.TopicPass, result.TopicScore = evaluateTopic(c.Topic, answer, traj)

    return result
}
```

**Tool Level — проверка выбора инструментов:**

```go
func toolSelectionAccuracy(expected, actual []string) float64 {
    if len(expected) == 0 {
        return 1.0
    }
    matches := 0
    for _, exp := range expected {
        for _, act := range actual {
            if exp == act {
                matches++
                break
            }
        }
    }
    return float64(matches) / float64(len(expected))
}

func containsForbidden(used, forbidden []string) bool {
    for _, f := range forbidden {
        for _, u := range used {
            if f == u {
                return true // Использован запрещённый инструмент
            }
        }
    }
    return false
}
```

**Trajectory Level — детекция циклов:**

```go
func detectLoops(traj AgentTrajectory) bool {
    // Если одна и та же последовательность tool calls повторяется 3+ раз — это цикл
    var calls []string
    for _, step := range traj.Steps {
        if step.Type == "tool_call" {
            calls = append(calls, step.ToolName+":"+step.ToolArgs)
        }
    }

    windowSize := 3
    for i := 0; i <= len(calls)-windowSize*2; i++ {
        pattern := strings.Join(calls[i:i+windowSize], "|")
        next := strings.Join(calls[i+windowSize:min(i+windowSize*2, len(calls))], "|")
        if pattern == next {
            return true
        }
    }
    return false
}
```

### Шаг 4: Multi-turn Evaluation

Оценка многошаговых диалогов, где агент ведёт несколько раундов общения:

```go
type MultiTurnCase struct {
    ID    string      `json:"id"`
    Turns []TurnCase  `json:"turns"`
}

type TurnCase struct {
    UserInput      string   `json:"user_input"`
    ExpectedAction string   `json:"expected_action"` // "tool_call" или "text_response"
    ExpectedTools  []string `json:"expected_tools,omitempty"`
    MustContain    []string `json:"must_contain,omitempty"`
}

func evaluateMultiTurn(mtc MultiTurnCase, client *openai.Client) (float64, error) {
    var messages []openai.ChatCompletionMessage
    passedTurns := 0

    for _, turn := range mtc.Turns {
        messages = append(messages, openai.ChatCompletionMessage{
            Role: openai.ChatMessageRoleUser, Content: turn.UserInput,
        })

        resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
            Model:    model,
            Messages: messages,
            Tools:    tools,
        })
        if err != nil {
            return 0, err
        }

        msg := resp.Choices[0].Message
        messages = append(messages, msg)

        // Проверяем ожидания для этого turn
        if turn.ExpectedAction == "tool_call" && len(msg.ToolCalls) > 0 {
            passedTurns++
        } else if turn.ExpectedAction == "text_response" && len(msg.ToolCalls) == 0 {
            passedTurns++
        }

        // Выполняем tool calls если есть
        for _, tc := range msg.ToolCalls {
            result := executeTool(tc)
            messages = append(messages, openai.ChatCompletionMessage{
                Role: openai.ChatMessageRoleTool, Content: result, ToolCallID: tc.ID,
            })
        }
    }

    return float64(passedTurns) / float64(len(mtc.Turns)), nil
}
```

### Шаг 5: RAGAS-метрики для RAG

Если агент использует RAG, нужны специализированные метрики:

```go
// RAGAS (Retrieval Augmented Generation Assessment)
type RAGASMetrics struct {
    ContextPrecision float64 `json:"context_precision"` // Какая доля retrieved docs релевантна
    ContextRecall    float64 `json:"context_recall"`    // Какая доля нужных docs найдена
    Faithfulness     float64 `json:"faithfulness"`      // Ответ основан на retrieved docs (не галлюцинация)
    AnswerRelevance  float64 `json:"answer_relevance"`  // Ответ релевантен вопросу
}

func evaluateRAGAS(query, answer string, retrievedDocs, groundTruthDocs []string,
    client *openai.Client) RAGASMetrics {

    metrics := RAGASMetrics{}

    // Context Precision: какая доля найденных документов релевантна?
    relevantCount := 0
    for _, doc := range retrievedDocs {
        if isRelevant(query, doc, client) {
            relevantCount++
        }
    }
    if len(retrievedDocs) > 0 {
        metrics.ContextPrecision = float64(relevantCount) / float64(len(retrievedDocs))
    }

    // Context Recall: какая доля нужных документов найдена?
    foundCount := 0
    for _, gtDoc := range groundTruthDocs {
        for _, retDoc := range retrievedDocs {
            if isSameContent(gtDoc, retDoc) {
                foundCount++
                break
            }
        }
    }
    if len(groundTruthDocs) > 0 {
        metrics.ContextRecall = float64(foundCount) / float64(len(groundTruthDocs))
    }

    // Faithfulness: ответ основан на документах, а не на галлюцинациях?
    metrics.Faithfulness = scoreFaithfulness(answer, retrievedDocs, client)

    // Answer Relevance: ответ релевантен вопросу?
    metrics.AnswerRelevance = scoreRelevance(query, answer, client)

    return metrics
}
```

### Шаг 6: Quality Gates в CI/CD

```yaml
# .github/workflows/evals.yml
name: Agent Evals
on: [pull_request]
jobs:
  evals:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Run four-level evals
        run: go run cmd/evals/main.go --output=results.json

      - name: Check quality gates
        run: |
          # Парсим результаты
          TASK_PASS=$(jq '.task_pass_rate' results.json)
          TOOL_ACCURACY=$(jq '.tool_accuracy' results.json)
          TRAJECTORY_PASS=$(jq '.trajectory_pass_rate' results.json)
          TOPIC_SCORE=$(jq '.topic_avg_score' results.json)

          echo "Task Pass Rate: $TASK_PASS"
          echo "Tool Accuracy: $TOOL_ACCURACY"
          echo "Trajectory Pass Rate: $TRAJECTORY_PASS"
          echo "Topic Score: $TOPIC_SCORE"

          # Quality gates по каждому уровню
          FAILED=0
          if (( $(echo "$TASK_PASS < 0.95" | bc -l) )); then
            echo "FAIL: Task pass rate $TASK_PASS < 0.95"
            FAILED=1
          fi
          if (( $(echo "$TOOL_ACCURACY < 0.90" | bc -l) )); then
            echo "FAIL: Tool accuracy $TOOL_ACCURACY < 0.90"
            FAILED=1
          fi
          if (( $(echo "$TRAJECTORY_PASS < 0.85" | bc -l) )); then
            echo "FAIL: Trajectory pass rate $TRAJECTORY_PASS < 0.85"
            FAILED=1
          fi

          if [ "$FAILED" -eq 1 ]; then
            echo "Quality gates FAILED"
            exit 1
          fi
          echo "All quality gates PASSED"
```

```yaml
# .gitlab-ci.yml
stages:
  - evals

agent-evals:
  stage: evals
  image: golang:1.22
  script:
    - go run cmd/evals/main.go --output=results.json
    - |
      TASK_PASS=$(jq '.task_pass_rate' results.json)
      TOOL_ACCURACY=$(jq '.tool_accuracy' results.json)
      echo "Task: $TASK_PASS, Tool: $TOOL_ACCURACY"
      if (( $(echo "$TASK_PASS < 0.95" | bc -l) )); then
        echo "Quality gate failed"
        exit 1
      fi
  only:
    - merge_requests
  artifacts:
    paths:
      - results.json
```

### Шаг 7: Continuous Evaluation (в проде)

Evals в CI/CD ловят проблемы до деплоя. Но модели обновляются, данные меняются. Нужна оценка и в проде:

```go
// Фоновый процесс: запускает evals на реальных данных периодически
func continuousEval(interval time.Duration) {
    ticker := time.NewTicker(interval)
    for range ticker.C {
        // Берём случайную выборку из последних runs
        recentRuns := getRecentRuns(100)
        results := evaluateRuns(recentRuns)

        // Проверяем пороги
        if results.TaskPassRate < 0.90 {
            alert("Task pass rate dropped to %.2f", results.TaskPassRate)
        }
        if results.ToolAccuracy < 0.85 {
            alert("Tool accuracy dropped to %.2f", results.ToolAccuracy)
        }

        // Записываем метрики для дашборда
        metrics.Record("eval.task_pass_rate", results.TaskPassRate)
        metrics.Record("eval.tool_accuracy", results.ToolAccuracy)
    }
}
```

### Шаг 8: Версионирование датасетов

Eval-датасеты тоже версионируются:

```go
type EvalDataset struct {
    Version   string     `json:"version"`
    CreatedAt time.Time  `json:"created_at"`
    Cases     []EvalCase `json:"cases"`
}

// Датасет хранится в Git рядом с кодом
// testdata/evals/v1.0.json
// testdata/evals/v1.1.json (добавлены новые edge cases)
```

## Мини-пример кода

Минимальный eval runner для CI/CD:

```go
package main

import (
    "encoding/json"
    "fmt"
    "os"
)

type EvalSummary struct {
    TaskPassRate      float64 `json:"task_pass_rate"`
    ToolAccuracy      float64 `json:"tool_accuracy"`
    TrajectoryPassRate float64 `json:"trajectory_pass_rate"`
    TopicAvgScore     float64 `json:"topic_avg_score"`
}

func main() {
    dataset := loadDataset("testdata/evals/latest.json")
    var results []EvalResult

    for _, c := range dataset.Cases {
        answer, traj := runAgentWithTracing(c.Input, tools)
        result := evaluateCase(c, answer, traj)
        results = append(results, result)
    }

    summary := summarize(results)

    // Вывод для CI/CD
    out, _ := json.MarshalIndent(summary, "", "  ")
    os.WriteFile("results.json", out, 0644)
    fmt.Printf("Task: %.2f, Tool: %.2f, Trajectory: %.2f\n",
        summary.TaskPassRate, summary.ToolAccuracy, summary.TrajectoryPassRate)

    // Quality gate
    if summary.TaskPassRate < 0.95 || summary.ToolAccuracy < 0.90 {
        fmt.Println("FAILED: Quality gates not met")
        os.Exit(1)
    }
    fmt.Println("PASSED: All quality gates met")
}
```

## Типовые ошибки

### Ошибка 1: Только Task Level evals

**Симптом:** Агент проходит тесты, но в проде выбирает неправильные инструменты или делает лишние шаги.

**Причина:** Проверяется только финальный ответ, а не путь к нему.

**Решение:**
```go
// ПЛОХО: Только "ответ правильный?"
if answer == expected { pass++ }

// ХОРОШО: Четыре уровня оценки
result := evaluateCase(c, answer, trajectory)
// Проверяем task + tool + trajectory + topic
```

### Ошибка 2: Evals без записи траектории

**Симптом:** Тест провалился, но непонятно, на каком шаге что пошло не так.

**Причина:** Нет записи траектории выполнения.

**Решение:**
```go
// ПЛОХО: Запускаем агента, проверяем только ответ
answer := runAgent(input)

// ХОРОШО: Записываем траекторию
answer, trajectory := runAgentWithTracing(input, tools)
// Теперь видим каждый шаг: какой tool, какие args, какой результат
```

### Ошибка 3: Жёсткие пороги для всех уровней

**Симптом:** CI/CD постоянно падает из-за flaky evals на Trajectory Level.

**Причина:** Одинаковые строгие пороги для всех уровней. Trajectory Level нестабилен — модель может выбрать разные пути к одному результату.

**Решение:**
```go
// ПЛОХО: Одинаковый порог 0.95 для всех
taskThreshold := 0.95
toolThreshold := 0.95
trajectoryThreshold := 0.95 // Слишком строго для trajectory!

// ХОРОШО: Разные пороги для разных уровней
taskThreshold := 0.95       // Задача должна выполняться
toolThreshold := 0.90       // Правильный выбор инструментов
trajectoryThreshold := 0.80 // Путь может варьироваться
```

### Ошибка 4: Нет RAGAS-метрик для RAG-агентов

**Симптом:** RAG-агент находит нерелевантные документы, но evals этого не видят (проверяется только ответ).

**Причина:** Нет оценки качества retrieval.

**Решение:**
```go
// ПЛОХО: Проверяем только финальный ответ RAG-агента
if answerCorrect { pass++ }

// ХОРОШО: Проверяем и retrieval, и ответ
ragasMetrics := evaluateRAGAS(query, answer, retrievedDocs, groundTruthDocs, client)
if ragasMetrics.Faithfulness < 0.8 {
    log.Printf("Low faithfulness: agent may be hallucinating")
}
```

### Ошибка 5: Evals только в CI/CD, не в проде

**Симптом:** Evals проходят в CI/CD, но в проде качество деградирует (модель обновилась, данные изменились).

**Причина:** Нет continuous evaluation.

**Решение:** Запускайте evals и в проде (на выборке из реальных запросов).

## Мини-упражнения

### Упражнение 1: Напишите Tool Level eval

Напишите eval-кейс, который проверяет, что агент вызывает `check_status` (а не `restart_service`) для запроса "Какой статус сервера?":

```go
testCase := EvalCase{
    Input:          "Какой статус сервера web-01?",
    ExpectedTools:  []string{"check_status"},
    ForbiddenTools: []string{"restart_service"},
    // ...
}
```

### Упражнение 2: Реализуйте детекцию циклов

Реализуйте функцию `detectLoops` для Trajectory Level:

```go
func detectLoops(trajectory AgentTrajectory) bool {
    // Ваш код: проверьте, повторяются ли tool calls
}
```

### Упражнение 3: Реализуйте multi-turn eval

Напишите тест, где агент должен сначала проверить статус, а потом — если сервис упал — перезапустить:

```go
multiTurnCase := MultiTurnCase{
    Turns: []TurnCase{
        {UserInput: "Проверь nginx", ExpectedTools: []string{"check_status"}},
        {UserInput: "Сервис упал, перезапусти", ExpectedTools: []string{"restart_service"}},
    },
}
```

## Критерии сдачи / Чек-лист

**Сдано:**
- [x] Evals интегрированы в CI/CD pipeline
- [x] Quality gates блокируют деплой при ухудшении метрик
- [x] Есть оценка на четырёх уровнях (Task, Tool, Trajectory, Topic)
- [x] Записывается траектория выполнения для анализа
- [x] Для RAG-агентов есть RAGAS-метрики
- [x] Eval-датасеты версионируются

**Не сдано:**
- [ ] Evals не интегрированы в CI/CD
- [ ] Проверяется только финальный ответ (нет Tool/Trajectory Level)
- [ ] Нет записи траектории (невозможно отлаживать провалы)
- [ ] RAG-агенты оцениваются только по финальному ответу

## Связь с другими главами

- **[Глава 08: Evals и Надежность](../08-evals-and-reliability/README.md)** — базовые концепции evals
- **[Глава 06: RAG](../06-rag/README.md)** — RAGAS-метрики для RAG-агентов
- **[Глава 19: Observability и Tracing](../19-observability-and-tracing/README.md)** — трассировка для связи с evals
- **[Глава 22: Prompt и Program Management](../22-prompt-program-management/README.md)** — тестирование промптов

## Что дальше?

После изучения evals в CI/CD переходите к:
- **[24. Data и Privacy](../24-data-and-privacy/README.md)** — защита данных и приватность
