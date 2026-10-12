# 22. Prompt и Program Management

## Зачем это нужно?

Вы изменили промпт, и агент стал работать хуже. Но вы не можете понять, что именно изменилось, или откатить изменения. Без управления промптами вы теряете контроль над поведением агента.

В продакшене промпты — это **код**. Они определяют поведение агента так же, как функции и условия. И к ним нужен такой же подход: версионирование, тестирование, откат, мониторинг.

### Реальный кейс

**Ситуация:** Вы обновили системный промпт, чтобы улучшить качество ответов. Через день пользователи жалуются, что агент стал хуже обрабатывать инциденты.

**Проблема:** Нет версионирования промптов. Вы не знаете, какая версия работала вчера. Откатить не можете.

**Решение:** Централизованное хранилище промптов с версиями. Evals проверяют каждую версию перед деплоем. A/B тестирование показывает, какая версия лучше. Откат — одна команда.

## Теория простыми словами

### Промпт как артефакт

Промпт — это не "текст в коде". Это **артефакт**, который:
- Меняется чаще, чем код
- Влияет на поведение непредсказуемо (маленькое изменение → большой эффект)
- Должен тестироваться на каждом изменении
- Должен быть привязан к конкретным runs/traces для отладки

### Что такое промпт-регрессии?

Промпт-регрессия — ухудшение качества агента после изменения промпта. Одно слово может сломать поведение. Evals обнаруживают регрессии до деплоя.

### Скилы как часть промпт-менеджмента

Скилы ([глава 09](../09-agent-architecture/README.md)) — это тот же промпт-менеджмент, распространённый на большие знания. Файл `SKILL.md` версионируется как код: живёт в git, меняется через PR с обязательным eval-прогоном — те же правила, что и для system prompt. Разница — в независимости: скилы подключаются и отключаются, не трогая сам system prompt. Progressive disclosure ([глава 13](../13-context-engineering/README.md)) держит префикс дешёвым: в system prompt попадают только названия и краткие описания скилов, а полное содержимое агент подгружает при необходимости. Большие знания уезжают из system prompt в скилы — префикс короче, кэш стабильнее, поведение гибче.

> **Как это устроено в реальном SDK.** Промпт агента собирается из секций (PromptSection): сначала неизменяемый system prompt, затем секция скилов — только названия и описания. Полное тело скила попадает в историю диалога, когда агент его выбрал. В метаданных хода фиксируются версии и system prompt, и каждого использованного скила.

## Как это работает (пошагово)

### Шаг 1: Централизованное хранилище промптов

Все промпты хранятся в одном месте с метаданными:

```go
type PromptRegistry struct {
    store map[string][]PromptVersion // promptID → versions
}

type PromptVersion struct {
    ID          string            `json:"id"`
    PromptID    string            `json:"prompt_id"`
    Version     string            `json:"version"`     // "1.0.0", "1.1.0"
    Content     string            `json:"content"`
    Variables   []string          `json:"variables"`   // Переменные в промпте
    Author      string            `json:"author"`
    CreatedAt   time.Time         `json:"created_at"`
    Description string            `json:"description"` // Что изменилось
    Tags        map[string]string `json:"tags"`        // "model": "glm-5.3", "domain": "devops"
    IsActive    bool              `json:"is_active"`   // Используется ли в проде
}

func (r *PromptRegistry) Get(promptID, version string) (*PromptVersion, error) {
    versions, ok := r.store[promptID]
    if !ok {
        return nil, fmt.Errorf("prompt %s not found", promptID)
    }
    for i := len(versions) - 1; i >= 0; i-- {
        if versions[i].Version == version {
            return &versions[i], nil
        }
    }
    return nil, fmt.Errorf("version %s not found", version)
}

func (r *PromptRegistry) Rollback(promptID, toVersion string) error {
    version, err := r.Get(promptID, toVersion)
    if err != nil {
        return err
    }
    // Деактивируем текущую версию, активируем откатную
    r.deactivateAll(promptID)
    version.IsActive = true
    return nil
}
```

Обратите внимание: `Get` требует **точную** версию. Какую версию использовать — решает конфигурация при деплое, а не рантайм (почему — см. «Типовые ошибки», ошибка 1). «Активная» версия в реестре — это метка для людей и отката, а не способ адресации в рантайме.

### Шаг 2: Версионирование с Semantic Versioning

Применяем semver к промптам:

- **MAJOR** (1.0 → 2.0): Структурное изменение (новая роль, новый формат ответа)
- **MINOR** (1.0 → 1.1): Добавление инструкций (новый edge case, уточнение)
- **PATCH** (1.0.0 → 1.0.1): Исправление опечатки, форматирование

Diff между версиями — любой построчный алгоритм; важно не его изящество, а то, что diff виден человеку: в PR и в интерфейсе реестра. Изменение промпта без видимого diff — то же, что деплой кода без diff в PR.

### Шаг 3: Templating (Шаблонизация промптов)

Промпты часто содержат переменные. Разделяем шаблон и данные:

```go
type PromptTemplate struct {
    Template  string            // "You are a {{.Role}}. Your tools: {{.ToolList}}"
    Defaults  map[string]string // Значения по умолчанию
}

func (pt *PromptTemplate) Render(vars map[string]string) (string, error) {
    tmpl, err := template.New("prompt").Parse(pt.Template)
    if err != nil {
        return "", err
    }

    // Объединяем defaults с переданными переменными
    merged := make(map[string]string)
    for k, v := range pt.Defaults {
        merged[k] = v
    }
    for k, v := range vars {
        merged[k] = v
    }

    var buf bytes.Buffer
    if err := tmpl.Execute(&buf, merged); err != nil {
        return "", err
    }
    return buf.String(), nil
}

// Использование
tmpl := PromptTemplate{
    Template: `You are a {{.Role}} agent.
Available tools: {{.ToolList}}
SOP: {{.SOP}}
Constraints: {{.Constraints}}`,
    Defaults: map[string]string{
        "Constraints": "Always ask for confirmation before destructive actions.",
    },
}

prompt, _ := tmpl.Render(map[string]string{
    "Role":     "DevOps",
    "ToolList": "ping, check_status, restart_service",
    "SOP":      "1. Diagnose 2. Fix 3. Verify",
})
```

### Шаг 4: Prompt Playground (Тестирование промптов)

Prompt Playground — среда для тестирования промптов до деплоя. Вы можете проверить промпт на нескольких тестовых запросах и увидеть результаты.

```go
type PlaygroundRequest struct {
    PromptVersion string   `json:"prompt_version"`
    TestInputs    []string `json:"test_inputs"`   // Тестовые запросы
    Model         string   `json:"model"`
}

type PlaygroundResult struct {
    Input    string  `json:"input"`
    Output   string  `json:"output"`
    Tokens   int     `json:"tokens"`
    Latency  float64 `json:"latency_ms"`
    HasError bool    `json:"has_error"`
}

// LLMClient — нейтральный интерфейс к провайдеру (канон — глава 04)
type LLMClient interface {
    Complete(ctx context.Context, req Request) (*Response, error)
}

func runPlayground(ctx context.Context, req PlaygroundRequest, llm LLMClient) []PlaygroundResult {
    prompt, err := registry.Get("system", req.PromptVersion) // точная версия
    if err != nil {
        return nil
    }
    var results []PlaygroundResult

    for _, input := range req.TestInputs {
        start := time.Now()
        resp, err := llm.Complete(ctx, Request{
            Model: req.Model, // например, "glm-5.3" или "fable-5.1"
            Messages: []Message{
                {Role: "system", Content: prompt.Content},
                {Role: "user", Content: input},
            },
        })

        result := PlaygroundResult{
            Input:   input,
            Latency: float64(time.Since(start).Milliseconds()),
        }
        if err != nil {
            result.HasError = true
        } else {
            result.Output = resp.Choices[0].Message.Content
            result.Tokens = resp.Usage.PromptTokens + resp.Usage.CompletionTokens
        }
        results = append(results, result)
    }
    return results
}
```

### Шаг 5: A/B Testing промптов

Параллельно тестируем две версии промпта на реальном трафике:

```go
type ABTest struct {
    Name       string  `json:"name"`
    VersionA   string  `json:"version_a"`   // Контрольная группа
    VersionB   string  `json:"version_b"`   // Экспериментальная группа
    TrafficPct float64 `json:"traffic_pct"` // % трафика на версию B (0.0 - 1.0)
    StartedAt  time.Time
}

func (ab *ABTest) SelectVersion(requestID string) string {
    // Детерминированный выбор на основе requestID (для воспроизводимости)
    hash := fnv.New32a()
    hash.Write([]byte(requestID))
    bucket := float64(hash.Sum32()) / float64(math.MaxUint32)

    if bucket < ab.TrafficPct {
        return ab.VersionB
    }
    return ab.VersionA
}

// Использование в agent loop
abTest := ABTest{
    Name: "improved_sop_prompt",
    VersionA: "1.0.0", // Текущая версия
    VersionB: "1.1.0", // Новая версия
    TrafficPct: 0.1,   // 10% трафика на новую версию
}

selectedVersion := abTest.SelectVersion(runID)
prompt, err := registry.Get("incident_sop", selectedVersion)
if err != nil {
    return fmt.Errorf("prompt version %s not found: %w", selectedVersion, err)
}
```

Правила эксперимента: обе версии зафиксированы в конфиге A/B-теста, выбор детерминирован и происходит **один раз — в начале хода**. Дальше версия промпта — константа хода: system prompt не меняется посреди выполнения, кэшируемый префикс стабилен ([глава 02](../02-prompt-engineering/README.md)), а трасса хода воспроизводима: `runID → prompt_version` однозначно.

**Метрики для сравнения:**

```go
type ABMetrics struct {
    Version    string
    PassRate   float64 // % успешных задач
    AvgLatency float64 // Средняя задержка
    AvgTokens  float64 // Среднее потребление токенов
    UserRating float64 // Пользовательская оценка (если есть)
}
```

### Шаг 6: MCP для промптов

MCP-сервер может раздавать промпты агентам. Это полезно, когда несколько агентов используют общие промпты:

```go
// MCP-сервер предоставляет промпты как ресурсы
type PromptMCPServer struct {
    registry *PromptRegistry
}

// Агент запрашивает промпт через MCP
func (s *PromptMCPServer) GetResource(uri string) (string, error) {
    // URI: "prompt://incident_sop/1.1.0" — версия задана конфигом агента, не «текущая»
    parts := strings.Split(uri, "/")
    promptID := parts[1]
    version := parts[2]
    pv, err := s.registry.Get(promptID, version)
    if err != nil {
        return "", err
    }
    return pv.Content, nil
}
```

Подробнее о MCP см. [Главу 18: Протоколы Инструментов](../18-tool-protocols-and-servers/README.md).

### Шаг 7: Link to Traces (Связь промптов с трассировками)

Каждый run записывает в метаданные трейса: `run_id`, `prompt_id`, `prompt_version`, `model`, timestamp. Один вызов `tracing.LogMetadata(...)` при старте хода — и вопросы инцидентов решаются запросом: «какая версия промпта работала в run `abc123`?» → `prompt_id=incident_sop, version=1.1.0`. Без этой связи жалоба пользователя неразрешима: вы даже не знаете, какой промпт показал плохое поведение.

Подробнее о трассировке см. [Главу 19: Observability и Tracing](../19-observability-and-tracing/README.md).

## Мини-пример кода

Минимальный пример: загрузка промпта по версии + feature flag:

```go
func getSystemPrompt(flags FeatureFlags) string {
    version := "1.0.0"
    if flags.UseNewPrompt {
        version = "1.1.0"
    }

    prompt, err := registry.Get("system_devops", version)
    if err != nil {
        log.Printf("Failed to get prompt version %s: %v, using default", version, err)
        return defaultPrompt
    }
    return prompt.Content
}
```

## Типовые ошибки

### Ошибка 1: Промпты захардкожены в коде

**Симптом:** Чтобы изменить промпт, нужно менять код, проходить code review и деплоить.

**Причина:** Промпты хранятся как строковые константы в Go-файлах.

**Решение:** реестр версий — но версия фиксируется в конфигурации при деплое, а не выбирается рантаймом:

```go
// ПЛОХО: промпт в коде
const systemPrompt = "You are a DevOps agent..."

// ПЛОХО: «latest» в рантайме — рантайм сам решает, какая версия «текущая»
prompt, _ := registry.Get("system_devops", "latest")

// ХОРОШО: версия — константа конфигурации, зафиксирована при деплое
prompt, err := registry.Get("system_devops", cfg.PromptVersion) // например, "1.4.2"
if err != nil {
    return fmt.Errorf("prompt version %s not found: %w", cfg.PromptVersion, err)
}
```

Почему не «latest»: system prompt — это **кэшируемый префикс** ([глава 02](../02-prompt-engineering/README.md)). Если рантайм при каждом запросе вытягивает «текущую» версию, любое параллельное обновление молча меняет префикс — промах кэша (растут стоимость и латентность) и недетерминизм: две трассы одного хода собраны из разных промптов, отладка невозможна. Смена версии — осознанный акт: PR с diff и eval-прогоном (главы [08](../08-evals-and-reliability/README.md) и [23](../23-evals-in-cicd/README.md)), затем новая версия в конфиге и деплой.

### Ошибка 2: Нет evals перед деплоем промпта

**Симптом:** Новая версия промпта ломает поведение агента. Узнаёте после деплоя.

**Причина:** Промпты деплоятся без тестирования.

**Решение:**
```go
// ПЛОХО: Деплой без проверки
registry.SetActive("system_devops", "2.0.0")

// ХОРОШО: Проверка через evals перед активацией
passRate := runEvalsForPrompt("system_devops", "2.0.0")
if passRate < 0.95 {
    log.Printf("Prompt 2.0.0 failed evals: %.2f < 0.95", passRate)
    return // Не активируем
}
registry.SetActive("system_devops", "2.0.0")
```

Ручной прогон — полумера: quality gates для промптов должны стоять в CI/CD и блокировать слияние PR с изменением промпта без зелёного eval-прогона — как это устроено, разбираем в [Главе 23: Evals в CI/CD](../23-evals-in-cicd/README.md).

### Ошибка 3: A/B тест без статистики

**Симптом:** Переключили 100% трафика на новую версию после "тест на 10 запросах показал, что лучше".

**Причина:** Недостаточно данных для статистически значимого сравнения.

**Решение:**
```go
// ПЛОХО: 10 запросов → решение
if sampleSize < 100 {
    log.Println("Not enough data for A/B decision")
    return
}

// ХОРОШО: Статистически значимая выборка
// Минимум 100-500 запросов на каждую версию
// Сравнение по нескольким метрикам (pass rate, latency, tokens)
```

### Ошибка 4: Нет связи промпта с трассировкой

**Симптом:** Пользователь жалуется на плохой ответ. Вы не знаете, какая версия промпта использовалась.

**Причина:** Run metadata не записывает версию промпта.

**Решение:**
```go
// ПЛОХО: Запускаем агента без записи версии промпта
runAgent(prompt.Content, ...)

// ХОРОШО: Записываем версию в трассировку
logRunMetadata(runID, prompt, model)
runAgent(prompt.Content, ...)
```

### Ошибка 5: Переменные вместо шаблонов

**Симптом:** Промпт формируется через `fmt.Sprintf` с 10+ аргументами. Нельзя понять, что за промпт получится.

**Причина:** Нет шаблонизации.

**Решение:**
```go
// ПЛОХО
prompt := fmt.Sprintf("You are a %s. Tools: %s. SOP: %s. Constraints: %s.", role, tools, sop, constraints)

// ХОРОШО
tmpl := PromptTemplate{
    Template: `You are a {{.Role}}.
Tools: {{.ToolList}}
SOP: {{.SOP}}
Constraints: {{.Constraints}}`,
}
prompt, _ := tmpl.Render(vars)
```

## Мини-упражнения

### Упражнение 1: Реализуйте PromptRegistry

Реализуйте хранилище промптов с методами `Get`, `Add`, `Rollback`:

```go
type PromptRegistry struct {
    // Ваш код
}

func (r *PromptRegistry) Get(id, version string) (*PromptVersion, error) { ... }
func (r *PromptRegistry) Add(pv PromptVersion) error { ... }
func (r *PromptRegistry) Rollback(id, version string) error { ... }
```

**Ожидаемый результат:**
- Можно добавить несколько версий промпта
- Можно получить конкретную версию; запрос несуществующей версии возвращает ошибку
- Rollback деактивирует текущую и активирует указанную версию

### Упражнение 2: Реализуйте A/B тест

Реализуйте выбор версии промпта по requestID:

```go
func selectVersion(requestID string, trafficPct float64) string {
    // Ваш код: детерминированный выбор на основе hash(requestID)
}
```

**Ожидаемый результат:**
- Один и тот же requestID всегда получает одну версию
- trafficPct=0.1 направляет ~10% запросов на версию B

### Упражнение 3: Реализуйте Playground

Реализуйте функцию тестирования промпта на нескольких входах:

```go
func testPrompt(promptContent string, testInputs []string) []PlaygroundResult {
    // Ваш код
}
```

**Ожидаемый результат:**
- Каждый вход тестируется с указанным промптом
- Результат содержит output, tokens, latency

## Критерии сдачи / Чек-лист

**Сдано:**
- [x] Промпты хранятся в централизованном реестре с версиями
- [x] Каждая версия проходит evals перед активацией
- [x] Есть шаблонизация (templating) для переменных в промптах
- [x] Версия промпта записывается в трассировку каждого run
- [x] Есть механизм отката (rollback)
- [x] Feature flags позволяют включать/выключать версии без деплоя
- [x] Версия промпта фиксируется в конфиге при деплое, рантайм не выбирает «текущую»
- [x] Скилы (SKILL.md) версионируются как код и проходят evals независимо от system prompt

**Не сдано:**
- [ ] Промпты захардкожены в коде
- [ ] Нет evals для проверки изменений
- [ ] Нет связи между промптом и трассировкой
- [ ] A/B тесты проводятся без статистически значимой выборки

## Для любопытных

### Prompt as Code vs Prompt as Config

Два подхода к управлению промптами:

1. **Prompt as Code**: Промпты в Git, изменения через PR. Плюс — полный audit trail. Минус — медленный цикл изменений.
2. **Prompt as Config**: Промпты в БД/API, изменения через UI. Плюс — быстрые итерации. Минус — сложнее отслеживать.

Оптимум: **Prompt as Code** для system prompt (редко меняется), **Prompt as Config** для few-shot примеров и SOP (меняется часто).

Куда бы ни жил промпт, одно правило неизменно: динамические вставки (данные, контекст задачи, свежие факты) идут **в конец истории сообщений**, никогда в system prompt (`messages[0]`). System prompt — кэшируемый префикс ([глава 02](../02-prompt-engineering/README.md)): любая динамика в нём ломает кэш и удорожает каждый запрос, а заодно делает трассы невоспроизводимыми.

## Связь с другими главами

- **[Глава 02: Промптинг](../02-prompt-engineering/README.md)** — как писать эффективные промпты; системный промпт как кэшируемый префикс
- **[Глава 08: Evals и Надежность](../08-evals-and-reliability/README.md)** — как тестировать промпты
- **[Глава 09: Архитектура агента](../09-agent-architecture/README.md)** — скилы (SKILL.md) и progressive disclosure
- **[Глава 18: Протоколы Инструментов](../18-tool-protocols-and-servers/README.md)** — MCP для раздачи промптов
- **[Глава 19: Observability и Tracing](../19-observability-and-tracing/README.md)** — связь промптов с трассировкой
- **[Глава 23: Evals в CI/CD](../23-evals-in-cicd/README.md)** — quality gates: автоматическая проверка промптов в pipeline
- **[Agent Skills](https://agentskills.io/)** — формат `SKILL.md` как стандартный способ упаковки переиспользуемых промптов и инструкций

## Что дальше?

После изучения управления промптами переходите к:
- **[23. Evals в CI/CD](../23-evals-in-cicd/README.md)** — автоматическая проверка качества в pipeline
