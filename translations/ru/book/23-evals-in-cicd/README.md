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

### Шаг 2: Траектория выполнения — это RunResult

Вторую структуру траектории придумывать не нужно. Агентный цикл уже возвращает **RunResult** ([гл. 04](../04-autonomy-and-loops/README.md)): итерации, tool-вызовы с аргументами и результатами, usage. [Глава 08](../08-evals-and-reliability/README.md) строит на нём базовые evals, [глава 19](../19-observability-and-tracing/README.md) трассирует те же события. Eval-раннер просто читает готовую траекторию:

```go
// Eval-обёртка над каноническим RunResult (гл. 04): прод-поля
// (Iterations, Usage, Output) плюс eval-специфика — RunID для корреляции
// с трейсом (гл. 19), латентность прогона и запись tool-вызовов
type RunResult struct {
    RunID      string        `json:"run_id"`
    Iterations int           `json:"iterations"`
    ToolCalls  []ToolCallRec `json:"tool_calls"`
    Usage      Usage         `json:"usage"` // prompt/completion токены
    LatencyMs  float64       `json:"latency_ms"`
    Output     string        `json:"output"`
    Err        error         `json:"-"` // заполняется из RunResult.Err (гл. 04)
}

type ToolCallRec struct {
    Iteration int    `json:"iteration"`
    ToolName  string `json:"tool_name"`
    ToolArgs  string `json:"tool_args"`
    Result    string `json:"result"`
}

// runEvalCase запускает агента тем же Run'ом, что и прод, и заполняет
// eval-обёртку из канонного RunResult; запись вызовов и латентность —
// из событий Config.On (ToolEndEvent — гл. 09) и замера времени
func runEvalCase(ctx context.Context, input string) (string, RunResult) {
    start := time.Now()

    cfg := evalConfig(input)
    var rec RunResult
    rec.RunID = "eval-" + time.Now().UTC().Format("20060102T150405.000000000") // корреляция с трейсом (гл. 19)
    cfg.On = func(ev Event) {
        if end, ok := ev.(ToolEndEvent); ok {
            recTool := ToolCallRec{
                ToolName: end.Call.Name, ToolArgs: end.Call.Arguments,
                Iteration: len(rec.ToolCalls) + 1,
            }
            // Упавший инструмент — не паника: ошибка попадает в запись вызова
            if end.Err != nil || end.Res == nil {
                recTool.Result = "tool error"
            } else {
                recTool.Result = end.Res.Content
            }
            rec.ToolCalls = append(rec.ToolCalls, recTool)
        }
    }

    res := Run(ctx, cfg)
    rec.Output, rec.Iterations = res.Output, res.Iterations
    rec.Usage = res.Usage
    rec.Err = res.Err // терминальная ошибка хода: упавший прогон не выглядит успешным
    rec.LatencyMs = float64(time.Since(start).Milliseconds())

    return rec.Output, rec
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

func evaluateCase(c EvalCase, answer string, res RunResult) EvalResult {
    result := EvalResult{CaseID: c.ID}

    // Терминальная ошибка хода (гл. 04) — случай упал, любые оценки не считаются
    if res.Err != nil {
        result.TaskPass, result.ToolPass, result.TrajectoryPass = false, false, false
        result.TaskScore = 0
        return result
    }

    // --- Task Level ---
    result.TaskPass = checkTaskCompletion(c, answer)
    result.TaskScore = scoreAnswer(c.ExpectedOutput, answer)

    // --- Tool Level ---
    usedTools := extractToolNames(res)
    result.ToolAccuracy = toolSelectionAccuracy(c.ExpectedTools, usedTools)
    result.ToolPass = result.ToolAccuracy >= 0.8 && !containsForbidden(usedTools, c.ForbiddenTools)

    // --- Trajectory Level ---
    result.StepCount = res.Iterations
    result.HasLoops = detectLoops(res)
    result.TrajectoryPass = result.StepCount <= c.MaxSteps && !result.HasLoops

    // --- Topic Level ---
    result.TopicPass, result.TopicScore = evaluateTopic(c.Topic, answer, res)

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
func detectLoops(res RunResult) bool {
    // Если одна и та же последовательность tool calls повторяется 3+ раз — это цикл
    var calls []string
    for _, tc := range res.ToolCalls {
        calls = append(calls, tc.ToolName+":"+tc.ToolArgs)
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

func evaluateMultiTurn(mtc MultiTurnCase, client *llm.Client) (float64, error) {
    var messages []llm.Message
    passedTurns := 0

    for _, turn := range mtc.Turns {
        messages = append(messages, llm.Message{
            Role: "user", Content: turn.UserInput,
        })

        resp, err := client.Complete(ctx, llm.Request{
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
            messages = append(messages, llm.Message{
                Role: "tool", Content: result, ToolCallID: tc.ID,
            })
        }
    }

    return float64(passedTurns) / float64(len(mtc.Turns)), nil
}
```

### Шаг 5: RAGAS-метрики для RAG

Если агент использует RAG, нужны специализированные метрики.

> **Примечание:** Ниже показана упрощённая реализация RAGAS-метрик на Go. Реальный [RAGAS](https://docs.ragas.io/) — это Python-библиотека, в которой `isRelevant`, `scoreFaithfulness` и `scoreRelevance` реализованы через LLM-based evaluation. Пример показывает структуру метрик, а не production-реализацию.

```go
// RAGAS (Retrieval Augmented Generation Assessment)
type RAGASMetrics struct {
    ContextPrecision float64 `json:"context_precision"` // Какая доля retrieved docs релевантна
    ContextRecall    float64 `json:"context_recall"`    // Какая доля нужных docs найдена
    Faithfulness     float64 `json:"faithfulness"`      // Ответ основан на retrieved docs (не галлюцинация)
    AnswerRelevance  float64 `json:"answer_relevance"`  // Ответ релевантен вопросу
}

func evaluateRAGAS(query, answer string, retrievedDocs, groundTruthDocs []string,
    client *llm.Client) RAGASMetrics {

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
      - uses: actions/setup-go@v6
        with:
          go-version-file: go.mod

      - name: Run four-level evals
        run: go run cmd/evals/main.go --output=results.json

      - name: Check quality gates
        run: |
          # Пороги НЕ универсальные константы: каждый порог калибруется
          # на baseline — прогоните текущую (эталонную) версию агента
          # N>=3-5 раз и поставьте порог чуть ниже медианы.
          # Значения ниже — пример калибровки, не «production-стандарт».
          TASK_THRESHOLD=0.90        # калиброван на своём baseline task-эвалов
          TOOL_THRESHOLD=0.85        # свой baseline для tool-эвалов
          TRAJECTORY_THRESHOLD=0.75  # траектория варьируется сильнее всего

          # Парсим результаты
          TASK_PASS=$(jq '.task_pass_rate' results.json)
          TOOL_ACCURACY=$(jq '.tool_accuracy' results.json)
          TRAJECTORY_PASS=$(jq '.trajectory_pass_rate' results.json)
          TOPIC_SCORE=$(jq '.topic_avg_score' results.json)

          echo "Task Pass Rate: $TASK_PASS"
          echo "Tool Accuracy: $TOOL_ACCURACY"
          echo "Trajectory Pass Rate: $TRAJECTORY_PASS"
          echo "Topic Score: $TOPIC_SCORE"

          # Quality gates по каждому уровню — каждый со своим порогом
          FAILED=0
          if (( $(echo "$TASK_PASS < $TASK_THRESHOLD" | bc -l) )); then
            echo "FAIL: Task pass rate $TASK_PASS < $TASK_THRESHOLD"
            FAILED=1
          fi
          if (( $(echo "$TOOL_ACCURACY < $TOOL_THRESHOLD" | bc -l) )); then
            echo "FAIL: Tool accuracy $TOOL_ACCURACY < $TOOL_THRESHOLD"
            FAILED=1
          fi
          if (( $(echo "$TRAJECTORY_PASS < $TRAJECTORY_THRESHOLD" | bc -l) )); then
            echo "FAIL: Trajectory pass rate $TRAJECTORY_PASS < $TRAJECTORY_THRESHOLD"
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
  image: golang:1
  script:
    - go run cmd/evals/main.go --output=results.json
    - |
      TASK_PASS=$(jq '.task_pass_rate' results.json)
      TOOL_ACCURACY=$(jq '.tool_accuracy' results.json)
      echo "Task: $TASK_PASS, Tool: $TOOL_ACCURACY"
      # Порог калибруется на baseline (N>=3 прогонов эталонной версии)
      if (( $(echo "$TASK_PASS < $TASK_THRESHOLD" | bc -l) )); then
        echo "Quality gate failed"
        exit 1
      fi
  only:
    - merge_requests
  artifacts:
    paths:
      - results.json
```

#### Cost/latency-гейт — рядом с quality-гейтом

Качество — не единственная ось регрессии. Правка промпта может не уронить pass rate, но удвоить токены на прогон или латентность. Поэтому рядом с quality-гейтом стоит **cost/latency-гейт**: те же прогоны, но вместо pass rate проверяем токены, цену и латентность против baseline ([гл. 20](../20-cost-latency-engineering/README.md)):

```go
// Cost/latency-гейт: усредняем по N>=3 прогонам (один прогон недостоверен)
// и сравниваем с baseline, снятым на эталонной версии агента
type CostBaseline struct {
    AvgTokensPerRun float64       // из RunResult.Usage, не ручной подсчёт
    AvgLatencyMs    float64
}

func checkCostGate(runs []RunResult, base CostBaseline) error {
    var tokens, latency float64
    for _, r := range runs {
        tokens += float64(r.Usage.PromptTokens + r.Usage.CompletionTokens)
        latency += r.LatencyMs
    }
    if tokens/float64(len(runs)) > base.AvgTokensPerRun*1.2 {
        return fmt.Errorf("token regression: %.0f vs baseline %.0f", tokens/float64(len(runs)), base.AvgTokensPerRun)
    }
    if latency/float64(len(runs)) > base.AvgLatencyMs*1.3 {
        return fmt.Errorf("latency regression: %.0fms vs baseline %.0fms", latency/float64(len(runs)), base.AvgLatencyMs)
    }
    return nil
}
```

Baseline переснимается при сознательном изменении тарифа/модели, а не подгоняется под каждый PR.

#### Живые evals: smoke-набор на реальной системе

CI гоняет evals на заглушках — этого мало. При правке промптов или скилов прогоните **smoke-набор ключевых сценариев на реальной прод-системе** — вручную или «по кнопке» (отдельный режим с подтверждением опасных действий, [гл. 05](../05-safety-and-hitl/README.md)). Это обязательная часть процесса, а не «когда-нибудь потом»: CI-прогон может быть зелёным, а на проде — другой раннинг модели, другие данные, другая латентность. Как строить живые evals — [глава 08](../08-evals-and-reliability/README.md); метрики живых прогонов идут в тот же дашборд, что и CI ([гл. 19](../19-observability-and-tracing/README.md)).

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

        // Пороги — отклонение от baseline той же модели,
        // не универсальные константы
        if results.TaskPassRate < baseline.TaskPassRate-0.05 {
            alert("Task pass rate dropped to %.2f (baseline %.2f)",
                results.TaskPassRate, baseline.TaskPassRate)
        }
        if results.ToolAccuracy < baseline.ToolAccuracy-0.05 {
            alert("Tool accuracy dropped to %.2f (baseline %.2f)",
                results.ToolAccuracy, baseline.ToolAccuracy)
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
    "context"
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
    ctx := context.Background()
    dataset := loadDataset("testdata/evals/latest.json")
    var results []EvalResult

    for _, c := range dataset.Cases {
        answer, res := runEvalCase(ctx, c.Input)
        result := evaluateCase(c, answer, res)
        results = append(results, result)
    }

    summary := summarize(results)

    // Вывод для CI/CD
    out, _ := json.MarshalIndent(summary, "", "  ")
    os.WriteFile("results.json", out, 0644)
    fmt.Printf("Task: %.2f, Tool: %.2f, Trajectory: %.2f\n",
        summary.TaskPassRate, summary.ToolAccuracy, summary.TrajectoryPassRate)

    // Quality gates: пороги читаем из конфига, калибруются на baseline
    // текущей версии (N>=3-5 прогонов), это не универсальные константы
    gates := loadGates("gates.yaml")
    if summary.TaskPassRate < gates.TaskThreshold || summary.ToolAccuracy < gates.ToolThreshold {
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

// ХОРОШО: Запускаем через Run — RunResult уже несёт траекторию
answer, res := runEvalCase(ctx, input)
// Теперь видим каждый шаг: какой tool, какие args, какой результат
```

### Ошибка 3: Универсальные пороги «как production-стандарт»

**Симптом:** CI/CD постоянно падает из-за flaky evals на Trajectory Level. Или наоборот — гейт зелёный, а в проде качество хуже, чем было.

**Причина:** Пороги взяты «из головы» (0.95 для всего) или скопированы из чужого проекта. У вас другая модель, другой домен, другая вариативность. Trajectory Level нестабилен по своей природе — модель может выбрать разные пути к одному результату.

**Решение:** Каждый уровень eval калибруется на своём baseline: прогоните текущую (эталонную) версию агента N≥3–5 раз, возьмите медиану каждого показателя и поставьте порог чуть ниже неё. Один прогон для baseline недостоверен — nondeterminism модели даст случайный сдвиг.
```go
// ПЛОХО: один порог «0.95 как стандарт» для всех уровней
threshold := 0.95 // откуда это число? ниоткуда

// ХОРОШО: порог каждого уровня — от своего baseline
// (медиана N>=3 прогонов эталонной версии минус допуск)
taskThreshold       := baseline.TaskPassRate - 0.05
toolThreshold       := baseline.ToolAccuracy - 0.05
trajectoryThreshold := baseline.TrajectoryPassRate - 0.10 // путь варьируется сильнее
```

Baseline переснимается при осознанном улучшении агента — и тогда порог растёт вместе с качеством, а не остаётся константой.

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
func detectLoops(res RunResult) bool {
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
- [x] Пороги калибруются на baseline (N≥3 прогонов), не универсальные константы
- [x] Cost/latency-гейт работает рядом с quality-гейтом
- [x] Есть оценка на четырёх уровнях (Task, Tool, Trajectory, Topic)
- [x] Записывается траектория выполнения (RunResult) для анализа
- [x] Есть smoke-набор живых evals на реальной системе (гл. 08)
- [x] Для RAG-агентов есть RAGAS-метрики
- [x] Eval-датасеты версионируются

**Не сдано:**
- [ ] Evals не интегрированы в CI/CD
- [ ] Проверяется только финальный ответ (нет Tool/Trajectory Level)
- [ ] Нет записи траектории (невозможно отлаживать провалы)
- [ ] RAG-агенты оцениваются только по финальному ответу

## Связь с другими главами

- **[Глава 08: Evals и Надежность](../08-evals-and-reliability/README.md)** — базовые концепции evals, живые evals на проде
- **[Глава 06: RAG](../06-rag/README.md)** — RAGAS-метрики для RAG-агентов
- **[Глава 19: Observability и Tracing](../19-observability-and-tracing/README.md)** — трассировка RunResult для связи с evals
- **[Глава 20: Cost & Latency Engineering](../20-cost-latency-engineering/README.md)** — baseline для cost/latency-гейта
- **[Глава 22: Prompt и Program Management](../22-prompt-program-management/README.md)** — тестирование промптов

## Что дальше?

После изучения evals в CI/CD переходите к:
- **[24. Data и Privacy](../24-data-and-privacy/README.md)** — защита данных и приватность
