# 21. Workflow и State Management в продакшене

## Зачем это нужно?

**ВАЖНО:** Базовые концепции state management (идемпотентность, retries, дедлайны, persist) описаны в [Главе 11: State Management](../11-state-management/README.md). Эта глава — про **прод-реальность**: очереди, асинхронность, масштабирование, распределённое состояние.

В продакшене агенты обрабатывают тысячи задач параллельно. Без прод-готовности workflow вы не можете:
- Обрабатывать задачи асинхронно через очереди
- Масштабировать обработку задач горизонтально
- Гарантировать надёжность в распределённой системе
- Управлять приоритетами задач

### Реальный кейс

**Ситуация:** Команда запустила DevOps-агента. Он работает синхронно — один HTTP-запрос, один agent run. В пятницу вечером приходит 200 задач на деплой. Агент обрабатывает их по одной. К понедельнику очередь не разгребена, а 30 задач упали по таймауту.

**Проблема:** Синхронная обработка не масштабируется. Нет приоритизации (hotfix ждёт наравне с рутиной). При падении воркера задачи теряются. Пользователи не видят прогресс и дублируют запросы.

**Решение:** Асинхронная очередь задач с пулом воркеров, SSE для отображения прогресса, Saga для многошаговых деплоев с откатом. Агент масштабируется горизонтально, задачи не теряются, пользователи видят прогресс в реальном времени.

## Теория простыми словами

### Что такое Workflow в продакшене?

Workflow — это последовательность шагов для выполнения задачи. В продакшене workflow "живёт" дольше одного HTTP-запроса и переживает рестарты воркеров. Базовые концепции (состояние задачи, идемпотентность, retry, дедлайны, persist) описаны в [Главе 11](../11-state-management/README.md). Здесь мы рассматриваем, как эти концепции работают в масштабе.

### Прод-паттерн: AgentState + артефакты вместо "толстого" контекста

В продакшене workflow обычно "живет" дольше одного HTTP-запроса и переживает рестарты воркеров. Поэтому полезно хранить **AgentState** как каноническое состояние agent run (цель, бюджеты, план, факты, вопросы, risk flags), а большие результаты инструментов складывать как **артефакты**:

- рантайм/воркер сохраняет сырой результат (логи, JSON, файлы) во внешнем хранилище,
- в состояние добавляет запись `artifact_id + summary + bytes`,
- в контекст LLM возвращает только короткий excerpt (top-k строк) + `artifact_id`.

Так вы снижаете стоимость и не "убиваете" контекстное окно, даже если задача длинная и шагов много.

## Как это работает (пошагово)

Базовые паттерны (структура задачи, идемпотентность, retry с backoff, дедлайны, сохранение состояния) описаны в [Главе 11: State Management](../11-state-management/README.md). Здесь мы строим на этом фундаменте.

### Шаг 1: Очередь задач

Вместо синхронной обработки используйте очередь. Клиент кладёт задачу и сразу получает `task_id`. Воркер забирает задачу, выполняет и сохраняет результат.

```
Клиент → [Очередь задач] → Воркер → [Хранилище результатов] → Клиент
```

### Шаг 2: Пул воркеров

Запустите несколько воркеров. Каждый забирает задачи из общей очереди. Масштабируйте горизонтально: больше задач — больше воркеров.

### Шаг 3: Прогресс в реальном времени

Отправляйте обновления через SSE или WebSocket. Пользователь видит, на каком шаге агент, какой инструмент вызывает.

### Шаг 4: Компенсация при сбоях

Для многошаговых операций используйте Saga. Если шаг N упал — откатите шаги N-1, ..., 1 в обратном порядке.

## Асинхронная коммуникация

### Почему синхронные вызовы не масштабируются

Синхронный вызов агента — это HTTP-запрос, который ждёт ответа. Если задача занимает 10 минут, вызывающая сторона блокируется на 10 минут. При 100 одновременных задачах вам нужно 100 потоков, которые просто ждут. Это не масштабируется.

Проблемы синхронного подхода:
- Вызывающая сторона блокируется на всё время выполнения
- HTTP-таймауты обрывают долгие задачи (обычно 30–60 секунд)
- При перезапуске сервера все текущие запросы теряются
- Нет приоритизации: срочные задачи ждут в одной очереди с обычными

### Паттерн: очередь задач

Асинхронный подход разделяет отправку задачи и получение результата. Вызывающая сторона кладёт задачу в очередь и сразу получает `task_id`. Воркер забирает задачу из очереди, выполняет и кладёт результат обратно.

```
Клиент → [Очередь задач] → Воркер → [Очередь результатов] → Клиент
```

В Go очередь задач можно реализовать через каналы. В продакшене используют RabbitMQ, Kafka или Redis Streams — но принцип тот же.

```go
// TaskQueue — простая очередь задач на каналах
type TaskQueue struct {
    tasks   chan *Task
    results map[string]chan *Task
    mu      sync.RWMutex
}

func NewTaskQueue(bufferSize int) *TaskQueue {
    return &TaskQueue{
        tasks:   make(chan *Task, bufferSize),
        results: make(map[string]chan *Task),
    }
}

// Submit кладёт задачу в очередь и возвращает task_id
func (q *TaskQueue) Submit(userInput string) string {
    task := &Task{
        ID:        generateTaskID(),
        UserInput: userInput,
        State:     TaskPending,
        CreatedAt: time.Now(),
    }

    // Канал для результата этой конкретной задачи
    q.mu.Lock()
    q.results[task.ID] = make(chan *Task, 1)
    q.mu.Unlock()

    q.tasks <- task
    return task.ID
}

// Wait ждёт результат задачи с таймаутом
func (q *TaskQueue) Wait(taskID string, timeout time.Duration) (*Task, error) {
    q.mu.RLock()
    ch, exists := q.results[taskID]
    q.mu.RUnlock()

    if !exists {
        return nil, fmt.Errorf("task not found: %s", taskID)
    }

    select {
    case result := <-ch:
        return result, nil
    case <-time.After(timeout):
        return nil, fmt.Errorf("timeout waiting for task: %s", taskID)
    }
}

// Worker забирает задачи из очереди и выполняет
func (q *TaskQueue) Worker(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case task := <-q.tasks:
            task.State = TaskRunning
            task.UpdatedAt = time.Now()

            // Выполняем задачу (здесь будет agent loop)
            result, err := doWork(task.UserInput)

            if err != nil {
                task.State = TaskFailed
                task.Error = err.Error()
            } else {
                task.State = TaskCompleted
                task.Result = result
            }
            task.UpdatedAt = time.Now()

            // Отправляем результат
            q.mu.RLock()
            if ch, ok := q.results[task.ID]; ok {
                ch <- task
            }
            q.mu.RUnlock()
        }
    }
}
```

Использование:

```go
func main() {
    queue := NewTaskQueue(100)

    // Запускаем 3 воркера
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    for i := 0; i < 3; i++ {
        go queue.Worker(ctx)
    }

    // Клиент отправляет задачу и сразу получает ID
    taskID := queue.Submit("Проверь диски и почисти логи")
    fmt.Printf("Задача принята: %s\n", taskID)

    // Клиент ждёт результат (или проверяет позже)
    result, err := queue.Wait(taskID, 5*time.Minute)
    if err != nil {
        fmt.Printf("Ошибка: %v\n", err)
        return
    }
    fmt.Printf("Результат: %s\n", result.Result)
}
```

### Паттерн: события (Event-Driven)

В event-driven архитектуре агент не вызывает сервисы напрямую. Он отправляет событие ("диск почищен"), а другие сервисы подписаны на это событие и реагируют сами.

```go
// EventBus — простая шина событий
type Event struct {
    Type    string         // "task.completed", "tool.executed", "agent.error"
    TaskID  string
    Payload map[string]any
    Time    time.Time
}

type EventHandler func(Event)

type EventBus struct {
    handlers map[string][]EventHandler
    mu       sync.RWMutex
}

func NewEventBus() *EventBus {
    return &EventBus{handlers: make(map[string][]EventHandler)}
}

func (b *EventBus) Subscribe(eventType string, handler EventHandler) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.handlers[eventType] = append(b.handlers[eventType], handler)
}

func (b *EventBus) Publish(event Event) {
    b.mu.RLock()
    defer b.mu.RUnlock()

    for _, handler := range b.handlers[event.Type] {
        go handler(event) // Обработчики запускаются асинхронно
    }
}
```

Пример подписок:

```go
bus := NewEventBus()

// Сервис мониторинга подписан на ошибки
bus.Subscribe("agent.error", func(e Event) {
    log.Printf("[ALERT] Агент %s: ошибка — %v", e.TaskID, e.Payload["error"])
})

// Сервис нотификаций подписан на завершение
bus.Subscribe("task.completed", func(e Event) {
    notifyUser(e.TaskID, e.Payload["result"].(string))
})

// Агент публикует события по ходу работы
bus.Publish(Event{
    Type:    "task.completed",
    TaskID:  "task-123",
    Payload: map[string]any{"result": "Диск почищен, освобождено 20GB"},
    Time:    time.Now(),
})
```

### Webhooks для уведомлений

Webhook — это HTTP-вызов на URL клиента при завершении задачи. Клиент регистрирует URL при отправке задачи. Агент отправляет POST-запрос с результатом.

```go
// WebhookNotifier отправляет результат на URL клиента
type WebhookNotifier struct {
    client *http.Client
}

func (n *WebhookNotifier) Notify(webhookURL string, task *Task) error {
    payload, err := json.Marshal(task)
    if err != nil {
        return err
    }

    resp, err := n.client.Post(webhookURL, "application/json", bytes.NewReader(payload))
    if err != nil {
        return fmt.Errorf("webhook failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        return fmt.Errorf("webhook returned %d", resp.StatusCode)
    }
    return nil
}
```

### Что использовать в продакшене

| Инструмент     | Когда использовать                     |
|----------------|---------------------------------------|
| Redis Streams  | Простые очереди, уже есть Redis       |
| RabbitMQ       | Классические очереди с подтверждением |
| Kafka          | Высокая пропускная способность, лог событий |
| NATS           | Легковесный pub/sub, микросервисы     |

Каналы Go хороши для прототипа и одного процесса. Для распределённой системы нужна внешняя очередь.

## Agent-to-UI (A2UI)

### Проблема: пользователь в неведении

Пользователь отправляет задачу. Агент работает 30 секунд. Пользователь видит спиннер и не знает, что происходит. Может, агент завис? Может, уже почти готово? Пользователь нажимает "Отмена" на 25-й секунде — за 5 секунд до результата.

Решение — отправлять обновления прогресса в реальном времени.

### Структура обновлений

Прежде чем выбирать транспорт, определите, что отправлять:

```go
// ProgressUpdate — одно обновление прогресса для UI
type ProgressUpdate struct {
    TaskID    string    `json:"task_id"`
    Step      int       `json:"step"`       // Номер текущего шага (итерации agent loop)
    TotalStep int       `json:"total_step"` // Ожидаемое количество шагов (0 — неизвестно)
    Status    string    `json:"status"`     // "thinking", "tool_call", "tool_result", "completed", "error"
    Tool      string    `json:"tool,omitempty"`    // Какой инструмент вызывается
    Message   string    `json:"message"`           // Человекочитаемое описание
    Time      time.Time `json:"time"`
}
```

Примеры обновлений по ходу работы агента:

```go
// Итерация 1: агент думает
update := ProgressUpdate{
    TaskID: "task-123", Step: 1, Status: "thinking",
    Message: "Анализирую задачу...",
}

// Итерация 2: вызов инструмента
update = ProgressUpdate{
    TaskID: "task-123", Step: 2, Status: "tool_call",
    Tool: "check_disk", Message: "Проверяю диски...",
}

// Итерация 2: результат инструмента
update = ProgressUpdate{
    TaskID: "task-123", Step: 2, Status: "tool_result",
    Tool: "check_disk", Message: "Диск /dev/sda1: 95% занято",
}

// Итерация 3: готово
update = ProgressUpdate{
    TaskID: "task-123", Step: 3, Status: "completed",
    Message: "Очистил 20GB логов, свободно 45%",
}
```

### Server-Sent Events (SSE)

SSE — самый простой способ отправить обновления с сервера на клиент. Это обычный HTTP-запрос, который не закрывается. Сервер отправляет строки `data: ...` по мере появления обновлений.

Преимущества SSE:
- Работает через обычный HTTP (проксирование, балансировка)
- Браузер переподключается автоматически
- Достаточно для однонаправленной отправки (сервер → клиент)

```go
// SSEHandler стримит обновления прогресса в браузер
func SSEHandler(queue *TaskQueue) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        taskID := r.URL.Query().Get("task_id")
        if taskID == "" {
            http.Error(w, "task_id required", http.StatusBadRequest)
            return
        }

        // Заголовки для SSE
        w.Header().Set("Content-Type", "text/event-stream")
        w.Header().Set("Cache-Control", "no-cache")
        w.Header().Set("Connection", "keep-alive")

        flusher, ok := w.(http.Flusher)
        if !ok {
            http.Error(w, "streaming not supported", http.StatusInternalServerError)
            return
        }

        // Подписываемся на обновления задачи
        updates := queue.Subscribe(taskID)
        defer queue.Unsubscribe(taskID, updates)

        for {
            select {
            case <-r.Context().Done():
                return // Клиент отключился

            case update, ok := <-updates:
                if !ok {
                    return // Канал закрыт, задача завершена
                }

                data, _ := json.Marshal(update)
                fmt.Fprintf(w, "data: %s\n\n", data)
                flusher.Flush()

                // Если задача завершена, закрываем поток
                if update.Status == "completed" || update.Status == "error" {
                    return
                }
            }
        }
    }
}
```

На стороне клиента (JavaScript в браузере):

```javascript
const source = new EventSource("/api/progress?task_id=task-123");

source.onmessage = (event) => {
    const update = JSON.parse(event.data);
    console.log(`[${update.status}] ${update.message}`);

    // Обновляем UI
    document.getElementById("status").textContent = update.message;
    if (update.total_step > 0) {
        const pct = Math.round((update.step / update.total_step) * 100);
        document.getElementById("progress").style.width = pct + "%";
    }

    if (update.status === "completed" || update.status === "error") {
        source.close();
    }
};
```

### WebSockets

WebSocket нужен, если клиент отправляет данные во время выполнения. Например, пользователь хочет уточнить задачу, отменить или изменить приоритет. SSE — только сервер → клиент. WebSocket — двунаправленный.

Когда использовать WebSocket вместо SSE:
- Пользователь может отменить задачу на лету
- Пользователь отвечает на вопросы агента (human-in-the-loop)
- Агенту нужно подтверждение перед опасной операцией

### Публикация обновлений из agent loop

Чтобы SSE-handler получал обновления, agent loop должен их отправлять. Добавьте callback в цикл агента:

```go
// ProgressCallback — функция для отправки обновлений
type ProgressCallback func(update ProgressUpdate)

func runAgentLoop(
    ctx context.Context,
    client *openai.Client,
    task *Task,
    onProgress ProgressCallback,
) error {
    messages := []openai.ChatCompletionMessage{
        {Role: openai.ChatMessageRoleSystem, Content: "You are a DevOps agent."},
        {Role: openai.ChatMessageRoleUser, Content: task.UserInput},
    }

    for i := 0; i < 10; i++ {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
        }

        // Уведомляем: агент думает
        onProgress(ProgressUpdate{
            TaskID: task.ID, Step: i + 1, Status: "thinking",
            Message: fmt.Sprintf("Итерация %d: анализирую...", i+1),
            Time: time.Now(),
        })

        resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
            Model:    "gpt-4o-mini",
            Messages: messages,
            Tools:    tools,
        })
        if err != nil {
            return err
        }

        msg := resp.Choices[0].Message
        messages = append(messages, msg)

        if len(msg.ToolCalls) == 0 {
            // Финальный ответ
            onProgress(ProgressUpdate{
                TaskID: task.ID, Step: i + 1, Status: "completed",
                Message: msg.Content, Time: time.Now(),
            })
            return nil
        }

        // Уведомляем о каждом вызове инструмента
        for _, tc := range msg.ToolCalls {
            onProgress(ProgressUpdate{
                TaskID: task.ID, Step: i + 1, Status: "tool_call",
                Tool: tc.Function.Name,
                Message: fmt.Sprintf("Вызываю %s...", tc.Function.Name),
                Time: time.Now(),
            })

            result := executeTool(tc)
            messages = append(messages, openai.ChatCompletionMessage{
                Role: openai.ChatMessageRoleTool, Content: result, ToolCallID: tc.ID,
            })

            onProgress(ProgressUpdate{
                TaskID: task.ID, Step: i + 1, Status: "tool_result",
                Tool: tc.Function.Name, Message: result, Time: time.Now(),
            })
        }
    }

    return fmt.Errorf("max iterations reached")
}
```

## Распределённые паттерны

### Saga: многошаговые операции с компенсацией

Агент выполняет цепочку шагов. Каждый шаг меняет состояние системы. Если шаг N упал, нужно откатить шаги N-1, N-2, ..., 1. Это паттерн Saga.

Пример: агент разворачивает приложение.

```
Шаг 1: Создать DNS-запись       → Компенсация: Удалить DNS-запись
Шаг 2: Создать сертификат       → Компенсация: Отозвать сертификат
Шаг 3: Деплой контейнера        → Компенсация: Удалить контейнер
Шаг 4: Настроить мониторинг     → Компенсация: Удалить алерты
```

Если шаг 3 упал, нужно выполнить компенсации шагов 2 и 1 (в обратном порядке).

```go
// SagaStep — один шаг саги
type SagaStep struct {
    Name       string
    Execute    func(ctx context.Context) error
    Compensate func(ctx context.Context) error // Откат
}

// Saga — цепочка шагов с компенсацией
type Saga struct {
    Steps     []SagaStep
    completed []int // Индексы выполненных шагов
}

func NewSaga(steps ...SagaStep) *Saga {
    return &Saga{Steps: steps}
}

// Run выполняет все шаги. При ошибке откатывает выполненные.
func (s *Saga) Run(ctx context.Context) error {
    for i, step := range s.Steps {
        log.Printf("[saga] Выполняю шаг %d: %s", i+1, step.Name)

        if err := step.Execute(ctx); err != nil {
            log.Printf("[saga] Шаг %d (%s) упал: %v", i+1, step.Name, err)
            log.Printf("[saga] Запускаю компенсацию...")

            // Откатываем выполненные шаги в обратном порядке
            s.compensate(ctx)

            return fmt.Errorf("saga failed at step %d (%s): %w", i+1, step.Name, err)
        }

        s.completed = append(s.completed, i)
    }

    log.Printf("[saga] Все %d шагов выполнены", len(s.Steps))
    return nil
}

// compensate откатывает выполненные шаги в обратном порядке
func (s *Saga) compensate(ctx context.Context) {
    for i := len(s.completed) - 1; i >= 0; i-- {
        idx := s.completed[i]
        step := s.Steps[idx]

        log.Printf("[saga] Компенсация шага %d: %s", idx+1, step.Name)

        if err := step.Compensate(ctx); err != nil {
            // Компенсация не должна падать, но логируем
            log.Printf("[saga] ОШИБКА компенсации шага %d: %v", idx+1, err)
        }
    }
}
```

Использование:

```go
func deployApp(ctx context.Context, appName string) error {
    saga := NewSaga(
        SagaStep{
            Name: "create_dns",
            Execute: func(ctx context.Context) error {
                return createDNSRecord(appName + ".example.com")
            },
            Compensate: func(ctx context.Context) error {
                return deleteDNSRecord(appName + ".example.com")
            },
        },
        SagaStep{
            Name: "create_cert",
            Execute: func(ctx context.Context) error {
                return issueCertificate(appName + ".example.com")
            },
            Compensate: func(ctx context.Context) error {
                return revokeCertificate(appName + ".example.com")
            },
        },
        SagaStep{
            Name: "deploy_container",
            Execute: func(ctx context.Context) error {
                return deployContainer(appName, "v1.2.3")
            },
            Compensate: func(ctx context.Context) error {
                return removeContainer(appName)
            },
        },
    )

    return saga.Run(ctx)
}
```

### Распределённое состояние

Когда агенты работают на разных машинах, in-memory хранение не работает. Состояние нужно хранить в общем хранилище:

| Хранилище      | Когда использовать                                |
|----------------|--------------------------------------------------|
| PostgreSQL     | Надёжное хранение, транзакции, сложные запросы   |
| Redis          | Быстрый доступ, TTL, простые структуры           |
| etcd           | Координация, distributed locks, leader election  |

Типичный паттерн: PostgreSQL для персистентного состояния + Redis для блокировок и кэша.

```go
// DistributedTaskStore — хранилище задач в PostgreSQL
type DistributedTaskStore struct {
    db *sql.DB
}

// Claim забирает задачу из очереди атомарно (только один воркер получит задачу)
func (s *DistributedTaskStore) Claim(ctx context.Context, workerID string) (*Task, error) {
    var task Task

    // UPDATE ... RETURNING с блокировкой: только один воркер получит задачу
    err := s.db.QueryRowContext(ctx, `
        UPDATE tasks
        SET state = 'running', worker_id = $1, updated_at = now()
        WHERE id = (
            SELECT id FROM tasks
            WHERE state = 'pending'
            ORDER BY created_at
            FOR UPDATE SKIP LOCKED
            LIMIT 1
        )
        RETURNING id, user_input, state, created_at, updated_at
    `, workerID).Scan(&task.ID, &task.UserInput, &task.State, &task.CreatedAt, &task.UpdatedAt)

    if err == sql.ErrNoRows {
        return nil, nil // Нет задач в очереди
    }

    return &task, err
}
```

`FOR UPDATE SKIP LOCKED` — ключевая конструкция. Она блокирует строку для текущего воркера, а остальные воркеры пропускают заблокированные строки. Это даёт конкурентный доступ без взаимных блокировок.

### Workflow-движки

Для сложных workflow в продакшене используют специализированные движки:

- **Temporal** — самый популярный. Workflow описывается как обычный код (Go, Java, Python). Движок берёт на себя retry, state, таймауты.
- **Cadence** — предшественник Temporal (от Uber).

Temporal полезен, когда у вас десятки шагов, сложная логика ветвления и нужна гарантированная доставка. Для 3–5 шагов достаточно Saga из этой главы.

## Типовые ошибки

### Ошибка 1: Синхронная обработка долгих задач

**Симптом:** HTTP-запрос отваливается по таймауту через 30 секунд, задача не завершена. Пользователь повторяет запрос, получает дубль.

**Причина:** Agent loop выполняется внутри HTTP-хэндлера. Долгая задача не укладывается в таймаут reverse-proxy.

**Решение:**
```go
// ПЛОХО: agent loop внутри HTTP-хэндлера
func handleTask(w http.ResponseWriter, r *http.Request) {
    result := runAgentLoop(r.Context(), task) // Может работать 10 минут
    json.NewEncoder(w).Encode(result)
}

// ХОРОШО: принять задачу, выполнить асинхронно
func handleTask(w http.ResponseWriter, r *http.Request) {
    taskID := queue.Submit(task)
    w.WriteHeader(http.StatusAccepted)
    json.NewEncoder(w).Encode(map[string]string{"task_id": taskID})
}
```

### Ошибка 2: Нет конкурентной защиты при захвате задач

**Симптом:** Два воркера берут одну задачу. Результат: дублирование работы, конфликты при записи результата, испорченное состояние.

**Причина:** `SELECT ... WHERE state = 'pending'` без блокировки. Оба воркера видят одну строку и оба её забирают.

**Решение:**
```sql
-- ПЛОХО: два воркера могут забрать одну задачу
SELECT id FROM tasks WHERE state = 'pending' LIMIT 1;
UPDATE tasks SET state = 'running' WHERE id = $1;

-- ХОРОШО: атомарный захват с блокировкой
UPDATE tasks
SET state = 'running', worker_id = $1
WHERE id = (
    SELECT id FROM tasks
    WHERE state = 'pending'
    ORDER BY created_at
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
RETURNING id;
```

### Ошибка 3: Компенсация без обратного порядка

**Симптом:** Saga откатывает шаги в прямом порядке. DNS-запись удаляется до удаления контейнера — контейнер остаётся с "висящим" сертификатом, пользователи получают ошибки.

**Причина:** Компенсация выполняется в порядке 1, 2, 3 вместо 3, 2, 1.

**Решение:**
```go
// ПЛОХО: откат в прямом порядке
for i := 0; i < len(completed); i++ {
    steps[completed[i]].Compensate(ctx)
}

// ХОРОШО: откат в обратном порядке
for i := len(completed) - 1; i >= 0; i-- {
    steps[completed[i]].Compensate(ctx)
}
```

### Ошибка 4: Нет прогресса для пользователя

**Симптом:** Пользователь отправил задачу. Прошло 2 минуты. Пользователь не знает, работает ли агент. Нажимает "Отмена" или дублирует запрос.

**Причина:** Нет отправки обновлений прогресса. Агент работает "молча".

**Решение:**
```go
// ПЛОХО: agent loop без обратной связи
for i := 0; i < maxIter; i++ {
    resp, _ := client.CreateChatCompletion(ctx, req)
    // ... обработка ...
}

// ХОРОШО: отправляем обновления на каждом шаге
for i := 0; i < maxIter; i++ {
    onProgress(ProgressUpdate{
        TaskID: task.ID, Step: i + 1, Status: "thinking",
        Message: fmt.Sprintf("Шаг %d: анализирую...", i+1),
    })
    resp, _ := client.CreateChatCompletion(ctx, req)
    // ... обработка с onProgress для tool_call/tool_result ...
}
```

### Ошибка 5: Очередь без ограничения размера

**Симптом:** Под нагрузкой сервис съедает всю память и падает с OOM. Или очередь растёт бесконечно, задачи ждут часами.

**Причина:** Канал или очередь создана без буфера или с очень большим буфером. Нет отказа при переполнении.

**Решение:**
```go
// ПЛОХО: безлимитная очередь
tasks := make(chan *Task) // Блокирующий канал, но нет backpressure

// ХОРОШО: ограниченный буфер + отказ при переполнении
tasks := make(chan *Task, 1000)

func (q *TaskQueue) Submit(task *Task) error {
    select {
    case q.tasks <- task:
        return nil
    default:
        return fmt.Errorf("queue full, try later") // HTTP 429
    }
}
```

## Мини-упражнения

### Упражнение 1: Реализуйте SSE-эндпоинт

Создайте HTTP-хэндлер, который стримит обновления прогресса агента через Server-Sent Events:

```go
func SSEProgressHandler(w http.ResponseWriter, r *http.Request) {
    // 1. Получите task_id из query-параметра
    // 2. Установите заголовки Content-Type: text/event-stream
    // 3. Подпишитесь на обновления задачи
    // 4. Стримьте обновления, пока задача не завершится
}
```

**Ожидаемый результат:**
- Клиент получает обновления в реальном времени
- Поток закрывается при завершении задачи
- Корректная обработка отключения клиента (`r.Context().Done()`)

### Упражнение 2: Реализуйте Saga для деплоя

Создайте Saga из 3 шагов с компенсацией:

```go
func deploySaga(appName string) *Saga {
    return NewSaga(
        // Шаг 1: создать namespace
        // Шаг 2: деплой контейнера
        // Шаг 3: настроить маршрутизацию
        // Каждый шаг с компенсацией
    )
}
```

**Ожидаемый результат:**
- При успехе все 3 шага выполняются
- При ошибке на шаге 3 шаги 2 и 1 откатываются в обратном порядке
- Логируется каждый шаг и каждая компенсация

### Упражнение 3: Реализуйте конкурентный захват задач

Напишите функцию `Claim`, которая атомарно забирает задачу из PostgreSQL:

```go
func (s *Store) Claim(ctx context.Context, workerID string) (*Task, error) {
    // Используйте FOR UPDATE SKIP LOCKED
    // Верните nil, nil если нет задач
}
```

**Ожидаемый результат:**
- Два воркера не берут одну задачу
- Задачи забираются в порядке создания
- Корректная обработка пустой очереди

## Критерии сдачи / Чек-лист

**Сдано (готовность к прод):**
- [x] Задачи обрабатываются асинхронно через очередь (не в HTTP-хэндлере)
- [x] Пул воркеров масштабируется горизонтально
- [x] Пользователь видит прогресс в реальном времени (SSE или WebSocket)
- [x] Многошаговые операции используют Saga с компенсацией
- [x] Конкурентный захват задач (`FOR UPDATE SKIP LOCKED`)
- [x] Очередь имеет ограничение размера и backpressure

**Не сдано:**
- [ ] Синхронная обработка в HTTP-хэндлере
- [ ] Нет прогресса для пользователя
- [ ] Два воркера могут взять одну задачу
- [ ] Нет компенсации при сбое многошаговых операций

## Связь с другими главами

- **[Глава 11: State Management](../11-state-management/README.md)** — Базовые концепции: идемпотентность, retries, дедлайны, persist. Эта глава строит на их основе.
- **[Глава 04: Автономность и Циклы](../04-autonomy-and-loops/README.md)** — Agent loop, который выполняется внутри воркера
- **[Глава 07: Multi-Agent Systems](../07-multi-agent/README.md)** — Координация нескольких агентов через очереди и события
- **[Глава 19: Observability и Tracing](../19-observability-and-tracing/README.md)** — Логирование и трейсинг распределённых задач
- **[Глава 20: Cost & Latency Engineering](../20-cost-latency-engineering/README.md)** — Контроль стоимости при масштабировании

## Что дальше?

После понимания прод-паттернов workflow переходите к:
- **[22. Prompt и Program Management](../22-prompt-program-management/README.md)** — Управление промптами и конфигурацией в продакшене

