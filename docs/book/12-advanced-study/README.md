# 12. Углублённое изучение

## Зачем это нужно?

Вы прошли базовый курс и создали рабочего агента. Но что дальше? Как перейти от "учебного агента" к "прод-агенту", который работает надёжно, безопасно и эффективно в реальной среде?

Эта глава — роадмап для углублённого изучения. Здесь собраны темы, которые почти всегда появляются при создании прод-агентов, но не всегда очевидны на старте. Мы не будем дублировать уже изученные концепции, а покажем, **что ещё нужно изучить** и **когда это становится критичным**.

### Реальный кейс

**Ситуация:** Вы создали агента для DevOps, он работает локально. Вы запускаете его в продакшен, и через неделю:
- Агент выполнил операцию, которая стоила $500 из-за большого количества токенов
- Вы не можете понять, почему агент принял неправильное решение — нет логов
- Агент завис на задаче и не отвечал 10 минут
- Пользователь пожаловался, что агент не запросил подтверждение перед удалением данных

**Проблема:** Учебный агент работает, но не готов к продакшену. Нет observability, контроля стоимости, обработки ошибок, политик безопасности.

**Решение:** Эта глава показывает, какие темы нужно изучить для создания прод-агента. Каждая тема содержит критерии "когда это нужно" и "что сделать в проде".

## Как выбирать темы для изучения?

Не пытайтесь изучить всё сразу. Используйте этот алгоритм приоритизации:

1. **Начните с обязательных прод-блоков:**
   - Observability (логирование, трейсинг) — нужно сразу, без этого вы слепы
   - Cost & latency engineering — критично, если агент используется активно
   - Evals в CI/CD — уже изучено в [Главе 09](../09-evals-and-reliability/README.md), но нужно интегрировать в CI/CD

2. **Добавьте темы по мере роста:**
   - Workflow/state — когда агенты выполняют долгие задачи или нужна идемпотентность
   - Политики доступа — когда несколько пользователей или разные уровни доступа
   - Prompt/program management — когда промпты меняются часто или есть несколько версий

3. **Специализированные темы:**
   - RAG в проде — если используете RAG (см. [Главу 07](../07-rag/README.md))
   - Data/privacy — если работаете с персональными данными

## Темы для углублённого изучения

### Observability и Tracing

**Когда нужно:** Сразу, как только агент выходит в прод. Без observability вы не можете понять, что происходит с агентом.

**Что сделать в проде:**

1. **Трейсинг agent run:**
   - Каждый запуск агента должен иметь уникальный `run_id`
   - Логируйте все этапы: входной запрос → выбор инструментов → выполнение → результат → финальный ответ
   - Используйте структурированное логирование (JSON)

```go
type AgentRun struct {
    RunID       string    `json:"run_id"`
    UserInput   string    `json:"user_input"`
    ToolCalls   []ToolCall `json:"tool_calls"`
    ToolResults []ToolResult `json:"tool_results"`
    FinalAnswer string    `json:"final_answer"`
    TokensUsed  int       `json:"tokens_used"`
    Latency     time.Duration `json:"latency"`
    Timestamp   time.Time `json:"timestamp"`
}

func logAgentRun(run AgentRun) {
    logJSON, _ := json.Marshal(run)
    log.Info(string(logJSON))
}
```

2. **Span'ы для tool calls:**
   - Каждый вызов инструмента — отдельный span
   - Логируйте: имя инструмента, аргументы, результат, время выполнения, ошибки
   - Используйте OpenTelemetry или аналоги для распределённого трейсинга

3. **Метрики:**
   - Latency (p50, p95, p99) — время выполнения запроса
   - Token usage — количество токенов на запрос
   - Error rate — процент ошибок
   - Iterations per task — сколько итераций нужно для решения задачи
   - Pass rate — процент успешных evals (см. [Главу 09](../09-evals-and-reliability/README.md))

4. **Кореляция с ID запроса:**
   - Все логи должны содержать `run_id` или `request_id`
   - Это позволяет связать логи агента, инструментов и внешних систем

**Связь с другими главами:**
- Базовое логирование уже упоминалось в [Главе 11: Best Practices](../11-best-practices/README.md)
- Здесь мы углубляемся в структурированное логирование и трейсинг

### Cost & Latency Engineering

**Когда нужно:** Когда агент используется активно или работает с большими контекстами. Критично для контроля бюджета и производительности.

**Что сделать в проде:**

1. **Бюджеты токенов:**
   - Установите лимит токенов на запрос (например, 10,000 токенов)
   - Отслеживайте использование токенов и предупреждайте при превышении
   - Используйте более дешёвые модели для простых задач

```go
const MaxTokensPerRequest = 10000

func checkTokenBudget(used int) error {
    if used > MaxTokensPerRequest {
        return fmt.Errorf("token budget exceeded: %d > %d", used, MaxTokensPerRequest)
    }
    return nil
}
```

2. **Кэширование:**
   - Кэшируйте результаты LLM для одинаковых запросов
   - Кэшируйте результаты инструментов, если они не меняются часто
   - Используйте TTL для кэша

3. **Fallback-модели:**
   - Если основная модель недоступна или слишком дорогая, используйте более дешёвую
   - Реализуйте цепочку fallback: GPT-4 → GPT-3.5 → локальная модель

4. **Батчинг:**
   - Группируйте несколько запросов в один batch, если модель поддерживает
   - Это снижает overhead и может снизить стоимость

5. **Ограничения итераций:**
   - Установите максимальное количество итераций ReAct loop (например, 10)
   - Это предотвращает бесконечные циклы и контролирует стоимость

**Связь с другими главами:**
- Контекстные бюджеты упоминались в [Главе 03: Анатомия Агента](../03-agent-architecture/README.md)
- Здесь мы углубляемся в контроль стоимости и оптимизацию

### Workflow и State Management

**Когда нужно:** Когда агенты выполняют долгие задачи (минуты или часы), нужна идемпотентность или обработка ошибок с retry.

**Что сделать в проде:**

1. **Идемпотентность:**
   - Все инструменты должны быть идемпотентными (повторный вызов даёт тот же результат)
   - Используйте уникальные ID для операций, чтобы избежать дублирования

```go
type Task struct {
    ID        string    `json:"id"`
    UserInput string    `json:"user_input"`
    State     TaskState `json:"state"`
    Result    string    `json:"result,omitempty"`
    CreatedAt time.Time `json:"created_at"`
}

func executeTask(id string) error {
    // Проверяем, не выполнялась ли задача уже
    if task, exists := getTask(id); exists && task.State == Completed {
        return nil // Идемпотентность: уже выполнено
    }
    // Выполняем задачу...
}
```

2. **Retries и backoff:**
   - При ошибке инструмента повторяйте вызов с экспоненциальным backoff
   - Ограничьте количество retries (например, 3 попытки)

```go
func executeWithRetry(fn func() error, maxRetries int) error {
    for i := 0; i < maxRetries; i++ {
        err := fn()
        if err == nil {
            return nil
        }
        if i < maxRetries-1 {
            backoff := time.Duration(1<<i) * time.Second
            time.Sleep(backoff)
        }
    }
    return fmt.Errorf("failed after %d retries", maxRetries)
}
```

3. **Дедлайны:**
   - Установите timeout для всего agent run (например, 5 минут)
   - Установите timeout для каждого вызова инструмента (например, 30 секунд)

4. **Очереди и асинхронность:**
   - Для долгих задач используйте очереди (RabbitMQ, Redis Queue)
   - Агент ставит задачу в очередь и возвращает ID, по которому можно проверить статус

5. **Persist state:**
   - Сохраняйте состояние агента между перезапусками (например, в БД)
   - Это позволяет продолжить выполнение задачи после сбоя

**Связь с другими главами:**
- Базовые циклы изучены в [Главе 05: Автономность и Циклы](../05-autonomy-and-loops/README.md)
- Здесь мы углубляемся в обработку ошибок и долгоживущие задачи

### Безопасность и Governance (обязательный прод-блок)

**Когда нужно:** Сразу, как только агент выходит в прод. Безопасность — это не опция, а обязательное требование.

**Что сделать в проде:**

1. **Threat modeling для tool-агентов:**
   - Определите, какие действия считаются критическими (удаление данных, изменение конфигурации, доступ к секретам)
   - Оцените риск каждого инструмента (низкий, средний, высокий)
   - Реализуйте разные уровни защиты для разных уровней риска

```go
type ToolRisk string

const (
    RiskLow    ToolRisk = "low"    // Чтение логов, проверка статуса
    RiskMedium ToolRisk = "medium" // Перезапуск сервисов, изменение настроек
    RiskHigh   ToolRisk = "high"   // Удаление данных, изменение критичных конфигов
)

type ToolDefinition struct {
    Name        string
    Description string
    Risk        ToolRisk
    RequiresConfirmation bool
}

func assessRisk(tool ToolDefinition) ToolRisk {
    // Оцениваем риск на основе имени и описания инструмента
    if strings.Contains(tool.Name, "delete") || strings.Contains(tool.Name, "remove") {
        return RiskHigh
    }
    if strings.Contains(tool.Name, "restart") || strings.Contains(tool.Name, "update") {
        return RiskMedium
    }
    return RiskLow
}
```

2. **Prompt Injection / Data Exfiltration:**
   - Валидируйте входные данные пользователя перед отправкой в LLM
   - Разделяйте пользовательский контекст и системный контекст
   - Используйте специальные маркеры для разделения контекстов

```go
func sanitizeUserInput(input string) string {
    // Удаляем попытки инъекции промпта
    dangerous := []string{
        "Ignore previous instructions",
        "You are now",
        "System:",
        "Assistant:",
    }
    sanitized := input
    for _, pattern := range dangerous {
        sanitized = strings.ReplaceAll(sanitized, pattern, "[REDACTED]")
    }
    return sanitized
}

// Разделяем контексты
func buildMessages(userInput string, systemPrompt string) []openai.ChatCompletionMessage {
    return []openai.ChatCompletionMessage{
        {Role: "system", Content: systemPrompt},
        {Role: "user", Content: sanitizeUserInput(userInput)},
    }
}
```

3. **RBAC к инструментам:**
   - Разные пользователи имеют доступ к разным инструментам
   - Например, junior DevOps может только читать логи, senior — перезапускать сервисы

```go
type UserRole string

const (
    RoleViewer  UserRole = "viewer"  // Только чтение
    RoleOperator UserRole = "operator" // Чтение + безопасные действия
    RoleAdmin   UserRole = "admin"   // Все действия
)

func canUseTool(userRole UserRole, toolName string) bool {
    toolPermissions := map[string][]UserRole{
        "read_logs":      {RoleViewer, RoleOperator, RoleAdmin},
        "restart_service": {RoleOperator, RoleAdmin},
        "delete_database": {RoleAdmin},
    }
    roles, exists := toolPermissions[toolName]
    if !exists {
        return false
    }
    for _, role := range roles {
        if role == userRole {
            return true
        }
    }
    return false
}
```

4. **Sandbox / dry-run режимы:**
   - Реализуйте режим, где инструменты не выполняются реально, а только симулируются
   - Полезно для тестирования и обучения
   - Используйте флаг `dry_run` для переключения режима

```go
type ToolExecutor struct {
    dryRun bool
}

func (e *ToolExecutor) Execute(toolName string, args map[string]interface{}) (string, error) {
    if e.dryRun {
        return fmt.Sprintf("[DRY RUN] Would execute %s with args: %v", toolName, args), nil
    }
    // Реальное выполнение
    return executeTool(toolName, args)
}
```

5. **Аудит:**
   - Логируйте все вызовы инструментов с указанием пользователя, времени, аргументов и результата
   - Храните аудит-логи отдельно и защищённо (не в обычных логах приложения)
   - Это критично для compliance (GDPR, SOC2)

```go
type AuditLog struct {
    Timestamp  time.Time              `json:"timestamp"`
    UserID     string                 `json:"user_id"`
    ToolName   string                 `json:"tool_name"`
    Arguments  map[string]interface{} `json:"arguments"`
    Result     string                 `json:"result"`
    Error      string                 `json:"error,omitempty"`
}

func logAudit(log AuditLog) {
    // Отправляем в отдельную систему аудита (не в обычные логи)
    auditSystem.Send(log)
}
```

**Связь с другими главами:**
- Базовые концепции безопасности изучены в [Главе 06: Безопасность и Human-in-the-Loop](../06-safety-and-hitl/README.md)
- Здесь мы углубляемся в threat modeling, prompt injection и governance

### Prompt и Program Management

**Когда нужно:** Когда промпты меняются часто, есть несколько версий или нужен A/B тестинг.

**Что сделать в проде:**

1. **Версионирование промптов:**
   - Храните промпты в Git или отдельной БД с версиями
   - Каждый промпт имеет версию и метаданные (автор, дата, описание изменений)

```go
type PromptVersion struct {
    ID          string    `json:"id"`
    Version     string    `json:"version"`
    Content     string    `json:"content"`
    Author      string    `json:"author"`
    CreatedAt   time.Time `json:"created_at"`
    Description string    `json:"description"`
}

func getPromptVersion(id string, version string) (*PromptVersion, error) {
    // Загружаем конкретную версию промпта
}
```

2. **Промпт-регрессии:**
   - Используйте evals (см. [Главу 09](../09-evals-and-reliability/README.md)) для проверки каждой новой версии
   - Сравнивайте метрики новой версии с предыдущей
   - Откатывайте версию, если метрики ухудшились

3. **Конфиги и feature flags:**
   - Вынесите параметры промпта (temperature, max_tokens) в конфигурацию
   - Используйте feature flags для включения/выключения функций без деплоя

4. **A/B тестинг:**
   - Запускайте разные версии промпта параллельно
   - Сравнивайте метрики и выбирайте лучшую версию

**Связь с другими главами:**
- Промптинг изучен в [Главе 02: Промптинг как Программирование](../02-prompt-engineering/README.md)
- Evals изучены в [Главе 09: Evals и Надежность](../09-evals-and-reliability/README.md)
- Здесь мы углубляемся в управление версиями и тестирование

### Data и Privacy

**Когда нужно:** Когда агент работает с персональными данными (PII) или секретами.

**Что сделать в проде:**

1. **PII (Personally Identifiable Information):**
   - Обнаруживайте PII в запросах пользователей (email, телефон, адрес)
   - Маскируйте или удаляйте PII перед отправкой в LLM
   - Используйте библиотеки для обнаружения PII (например, `presidio`)

```go
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

2. **Секреты:**
   - Никогда не логируйте секреты (API ключи, пароли, токены)
   - Используйте переменные окружения или secret managers (Vault, AWS Secrets Manager)
   - Проверяйте, что секреты не попадают в промпты

3. **Redaction:**
   - Удаляйте чувствительные данные из логов перед сохранением
   - Используйте паттерны для обнаружения секретов (например, "password: ...")

4. **Хранение логов:**
   - Храните логи в зашифрованном виде
   - Установите TTL для логов (например, удалять через 90 дней)
   - Ограничьте доступ к логам (только для аудита)

**Связь с другими главами:**
- Безопасность изучена в [Главе 06: Безопасность и Human-in-the-Loop](../06-safety-and-hitl/README.md)
- Здесь мы углубляемся в работу с персональными данными

### RAG в продакшене

**Когда нужно:** Если используете RAG (см. [Главу 07: RAG и База Знаний](../07-rag/README.md)) и нужна надёжность в проде.

**Что сделать в проде:**

1. **Версии документов:**
   - Версионируйте документы в базе знаний
   - Отслеживайте, какая версия документа использовалась в конкретном ответе
   - Это позволяет откатить изменения, если документ был обновлён неправильно

2. **Freshness (актуальность):**
   - Отслеживайте дату последнего обновления документа
   - Предупреждайте, если документ устарел (например, старше 30 дней)
   - Автоматически обновляйте документы из источников

3. **Реранкинг:**
   - Используйте второй этап ранжирования для улучшения качества retrieval
   - Реранкер переоценивает найденные документы и выбирает самые релевантные

4. **Grounding:**
   - Требуйте, чтобы агент ссылался на найденные документы в ответе
   - Запрещайте "выдумывать" информацию, которой нет в документах
   - Это снижает галлюцинации

5. **Отказоустойчивость retrieval:**
   - Если поиск не нашёл документы, агент должен сообщить об этом, а не выдумывать
   - Реализуйте fallback: если основной поиск не работает, используйте резервный

**Связь с другими главами:**
- RAG изучен в [Главе 07: RAG и База Знаний](../07-rag/README.md)
- Здесь мы углубляемся в прод-готовность RAG систем

### Evals в CI/CD

**Когда нужно:** Когда промпты или код меняются часто и нужна автоматическая проверка качества.

**Что сделать в проде:**

1. **Quality gates:**
   - Интегрируйте evals в CI/CD pipeline
   - Блокируйте деплой, если evals провалились или метрики ухудшились

```yaml
# Пример .github/workflows/evals.yml
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

2. **Датасеты:**
   - Храните "золотые" сценарии в датасете (JSON, CSV)
   - Расширяйте датасет при обнаружении новых кейсов
   - Версионируйте датасет вместе с кодом

3. **Flaky-кейсы:**
   - Идентифицируйте тесты, которые иногда падают (flaky)
   - Исправляйте flaky-тесты или помечайте их как нестабильные
   - Не блокируйте деплой из-за flaky-тестов

4. **Тесты на безопасность:**
   - Включите в evals тесты на безопасность (см. [Главу 06](../06-safety-and-hitl/README.md))
   - Проверяйте, что агент запрашивает подтверждение для критических действий
   - Проверяйте защиту от Prompt Injection

**Связь с другими главами:**
- Evals изучены в [Главе 09: Evals и Надежность](../09-evals-and-reliability/README.md)
- Здесь мы углубляемся в интеграцию в CI/CD

### Multi-Agent в продакшене

**Когда нужно:** Если инструментов много (20+) или задачи разнородные (DevOps + Security + Data). Multi-Agent системы помогают разделить ответственность и изолировать контексты.

**Что сделать в проде:**

1. **Supervisor/Worker и изоляция контекста:**
   - Supervisor принимает решение, какой Worker должен выполнить задачу
   - Каждый Worker имеет свой изолированный контекст (свой System Prompt, свои инструменты)
   - Это предотвращает "переполнение" контекста и путаницу между задачами

```go
type Supervisor struct {
    workers map[string]*Worker
}

func (s *Supervisor) RouteTask(task string) (*Worker, error) {
    // Supervisor анализирует задачу и выбирает подходящего Worker
    if strings.Contains(task, "database") {
        return s.workers["db_admin"], nil
    }
    if strings.Contains(task, "network") {
        return s.workers["network_admin"], nil
    }
    return s.workers["general"], nil
}

type Worker struct {
    name        string
    systemPrompt string
    tools       []openai.Tool
    // Изолированный контекст для этого Worker
}
```

2. **Маршрутизация задач:**
   - Определите, кто принимает решение (Supervisor или пользователь)
   - Определите, кто исполняет (какой Worker)
   - Реализуйте агрегацию результатов от нескольких Workers

```go
func (s *Supervisor) ExecuteTask(task string) (string, error) {
    // 1. Supervisor решает, какой Worker нужен
    worker, err := s.RouteTask(task)
    if err != nil {
        return "", err
    }
    
    // 2. Worker выполняет задачу в изолированном контексте
    result, err := worker.Execute(task)
    if err != nil {
        return "", err
    }
    
    // 3. Supervisor агрегирует результат
    return s.AggregateResult(result), nil
}
```

3. **Контуры безопасности:**
   - Определите, какие Workers имеют какие инструменты и права
   - DB Admin Worker не должен иметь доступ к network tools
   - Network Admin Worker не должен иметь доступ к database tools

```go
type WorkerConfig struct {
    Name        string
    AllowedTools []string
    MaxRisk     ToolRisk
}

var workerConfigs = map[string]WorkerConfig{
    "db_admin": {
        Name:        "Database Admin",
        AllowedTools: []string{"read_db_logs", "restart_db", "check_db_metrics"},
        MaxRisk:     RiskMedium, // Не может удалять базы
    },
    "network_admin": {
        Name:        "Network Admin",
        AllowedTools: []string{"check_network", "restart_network_service"},
        MaxRisk:     RiskMedium,
    },
}
```

4. **Наблюдаемость цепочки:**
   - Трассируйте цепочку: Supervisor → Worker → Tool
   - Каждый шаг должен иметь свой span в трейсе
   - Это позволяет понять, где произошла ошибка в цепочке

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

**Связь с другими главами:**
- Базовые концепции Multi-Agent изучены в [Главе 08: Multi-Agent Systems](../08-multi-agent/README.md)
- Здесь мы углубляемся в прод-готовность Multi-Agent систем

### Модель и декодинг (то, что ломает агентов чаще всего)

**Когда нужно:** Сразу, на этапе выбора модели и настройки параметров. Неправильный выбор модели или параметров декодинга — частая причина проблем.

**Что сделать в проде:**

1. **Capability benchmark (проверка модели до разработки):**
   - Перед началом разработки проверьте, подходит ли модель для вашей задачи
   - Используйте [Capability Benchmark из Приложения](../appendix/README.md) для проверки:
     - JSON generation
     - Instruction following
     - Function calling
   - Если модель не проходит benchmark — выберите другую модель или адаптируйте задачу

**Связь с другими главами:**
- Capability Benchmark описан в [Приложении](../appendix/README.md)
- Используйте Lab 00 для проверки модели перед началом разработки

2. **Детерминизм:**
   - Для tool calling и JSON generation используйте `Temperature = 0`
   - Это делает выводы детерминированными и предсказуемыми
   - Для творческих задач можно использовать `Temperature > 0`, но не для tool calling

```go
func createToolCallRequest(messages []openai.ChatCompletionMessage, tools []openai.Tool) openai.ChatCompletionRequest {
    return openai.ChatCompletionRequest{
        Model:       "gpt-4",
        Messages:    messages,
        Tools:       tools,
        Temperature: 0, // Детерминизм для tool calling
    }
}
```

3. **Structured outputs / JSON mode:**
   - Если модель поддерживает JSON mode (например, GPT-4 Turbo), используйте его
   - Это снижает вероятность "ломаного" JSON в tool calls
   - JSON mode гарантирует валидный JSON на выходе

```go
func createStructuredRequest(messages []openai.ChatCompletionMessage) openai.ChatCompletionRequest {
    return openai.ChatCompletionRequest{
        Model:       "gpt-4-turbo",
        Messages:    messages,
        ResponseFormat: &openai.ChatCompletionResponseFormat{
            Type: openai.ChatCompletionResponseFormatTypeJSONObject,
        },
        Temperature: 0,
    }
}
```

4. **Выбор модели под задачу:**
   - Оцените требования: стоимость, скорость, размер контекста, качество следования инструкциям
   - Используйте более дешёвые модели для простых задач (GPT-3.5 для простых tool calls)
   - Используйте более мощные модели для сложных задач (GPT-4 для многошагового планирования)

```go
func selectModel(taskComplexity string) string {
    switch taskComplexity {
    case "simple":
        return "gpt-3.5-turbo" // Дешевле и быстрее
    case "complex":
        return "gpt-4" // Лучше качество, но дороже
    default:
        return "gpt-3.5-turbo"
    }
}
```

**Связь с другими главами:**
- Физика LLM изучена в [Главе 01: Физика LLM](../01-llm-fundamentals/README.md)
- Function Calling изучен в [Главе 04: Инструменты и Function Calling](../04-tools-and-function-calling/README.md)
- Здесь мы углубляемся в выбор модели и настройку декодинга

### Справочники и шаблоны

**Когда нужно:** Всегда. Используйте справочники как шпаргалку при работе над проектами.

**Что найти в приложении:**

1. **Глоссарий:**
   - Определения терминов (Agent, ReAct, RAG, SOP и т.д.)
   - Используйте для быстрого поиска определений

2. **Чек-листы:**
   - Чек-лист готовности к продакшену
   - Чек-лист безопасности
   - Чек-лист качества промптов

3. **Шаблоны SOP:**
   - Готовые шаблоны SOP для разных доменов
   - Используйте как основу для своих SOP

4. **Таблицы решений:**
   - Таблицы для принятия решений в разных сценариях
   - Например, таблица решений для инцидентов

5. **Capability Benchmark:**
   - Набор тестов для проверки модели перед разработкой
   - Используйте для выбора подходящей модели

**Связь с другими главами:**
- Все справочники собраны в [Приложении](../appendix/README.md)
- Используйте их как шпаргалку при работе над проектами

## Типовые ошибки

### Ошибка 1: Нет observability

**Симптом:** Агент работает, но вы не можете понять, почему он принял неправильное решение. Нет логов или они неструктурированные.

**Причина:** Не реализовано логирование или оно недостаточно детальное.

**Решение:**
- Реализуйте структурированное логирование (JSON)
- Логируйте все этапы: входной запрос, выбор инструментов, выполнение, результат
- Используйте уникальные ID для кореляции логов

### Ошибка 2: Нет контроля стоимости

**Симптом:** Счёт за LLM API растёт неконтролируемо. Агент использует слишком много токенов.

**Причина:** Нет лимитов на токены, нет мониторинга использования.

**Решение:**
- Установите лимиты токенов на запрос
- Отслеживайте использование токенов и предупреждайте при превышении
- Используйте более дешёвые модели для простых задач
- Реализуйте кэширование

### Ошибка 3: Нет обработки ошибок

**Симптом:** Агент падает при первой же ошибке инструмента. Нет retry или обработки временных сбоев.

**Причина:** Не реализованы retries и обработка ошибок.

**Решение:**
- Реализуйте retry с экспоненциальным backoff
- Установите timeout для операций
- Обрабатывайте временные ошибки (network errors) отдельно от постоянных (validation errors)

### Ошибка 4: Нет политик доступа

**Симптом:** Все пользователи имеют доступ ко всем инструментам. Нет контроля, кто что может делать.

**Причина:** Не реализованы политики доступа или RBAC.

**Решение:**
- Реализуйте RBAC для инструментов
- Разные пользователи имеют доступ к разным инструментам
- Логируйте все действия для аудита

### Ошибка 5: Промпты не версионируются

**Симптом:** После изменения промпта агент стал работать хуже, но вы не можете откатить изменения или понять, что именно изменилось.

**Причина:** Промпты хранятся в коде без версионирования.

**Решение:**
- Версионируйте промпты (Git, БД)
- Используйте evals для проверки каждой версии
- Откатывайте версию, если метрики ухудшились

## Мини-упражнения

### Упражнение 1: Реализуйте структурированное логирование

Реализуйте функцию логирования agent run:

```go
func logAgentRun(runID string, userInput string, toolCalls []ToolCall, result string) {
    // Ваш код здесь
    // Логируйте в формате JSON
}
```

**Ожидаемый результат:**
- Логи в формате JSON
- Содержат все необходимые поля: run_id, user_input, tool_calls, result, timestamp

### Упражнение 2: Реализуйте проверку бюджета токенов

Реализуйте функцию проверки бюджета токенов:

```go
func checkTokenBudget(used int, limit int) error {
    // Ваш код здесь
    // Верните ошибку, если превышен лимит
}
```

**Ожидаемый результат:**
- Функция возвращает ошибку, если использовано токенов больше лимита
- Функция возвращает nil, если лимит не превышен

### Упражнение 3: Реализуйте retry с backoff

Реализуйте функцию выполнения с retry:

```go
func executeWithRetry(fn func() error, maxRetries int) error {
    // Ваш код здесь
    // Повторяйте вызов с экспоненциальным backoff
}
```

**Ожидаемый результат:**
- Функция повторяет вызов при ошибке
- Используется экспоненциальный backoff (1s, 2s, 4s...)
- Функция возвращает ошибку после исчерпания retries

## Критерии сдачи / Чек-лист

✅ **Сдано (готовность к прод-изучению):**
- Понимаете, какие темы нужно изучить для создания прод-агента
- Знаете критерии "когда нужно" для каждой темы
- Реализовано структурированное логирование
- Реализован контроль стоимости (бюджеты токенов)
- Реализована обработка ошибок (retries, timeouts)
- Evals интегрированы в CI/CD (если применимо)
- Политики доступа реализованы (если применимо)

❌ **Не сдано:**
- Не понимаете, какие темы нужно изучить дальше
- Нет observability (логирование, трейсинг)
- Нет контроля стоимости
- Нет обработки ошибок

## Связь с другими главами

- **Безопасность:** Базовые концепции безопасности изучены в [Главе 06: Безопасность и Human-in-the-Loop](../06-safety-and-hitl/README.md)
- **RAG:** Базовые концепции RAG изучены в [Главе 07: RAG и База Знаний](../07-rag/README.md)
- **Multi-Agent:** Базовые концепции Multi-Agent изучены в [Главе 08: Multi-Agent Systems](../08-multi-agent/README.md)
- **Evals:** Базовые концепции evals изучены в [Главе 09: Evals и Надежность](../09-evals-and-reliability/README.md)
- **Best Practices:** Общие практики изучены в [Главе 11: Best Practices](../11-best-practices/README.md)
- **Физика LLM:** Фундаментальные концепции изучены в [Главе 01: Физика LLM](../01-llm-fundamentals/README.md)
- **Инструменты:** Function Calling изучен в [Главе 04: Инструменты и Function Calling](../04-tools-and-function-calling/README.md)
- **Приложение:** Справочники, шаблоны и Capability Benchmark в [Приложении](../appendix/README.md)

---

**Навигация:** [← Best Practices](../11-best-practices/README.md) | [Оглавление](../README.md) | [Приложение →](../appendix/README.md)

