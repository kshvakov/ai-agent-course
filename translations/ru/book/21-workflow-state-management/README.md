# 21. Workflow и State Management в продакшене

## Зачем это нужно?

**ВАЖНО:** Базовые концепции state management (идемпотентность, retries, дедлайны, persist) описаны в [Главе 11: State Management](../11-state-management/README.md). Эта глава — про **прод-реальность**: очереди, асинхронность, масштабирование, распределённое состояние.

В продакшене агенты обрабатывают тысячи задач параллельно. Без прод-готовности workflow вы не можете:
- Обрабатывать задачи асинхронно через очереди
- Масштабировать обработку задач горизонтально
- Гарантировать надёжность в распределённой системе
- Управлять приоритетами задач

### Реальный кейс

**Ситуация:** Агент развёртывает приложение. Процесс занимает 10 минут. На 8-й минуте сервер перезагружается.

**Проблема:** Задача теряется. Пользователь не знает, что произошло. При повторном запуске агент начинает с начала, создавая дубликаты.

**Решение:** Сохранение состояния в БД, идемпотентность операций, retry с backoff и дедлайны. Теперь агент продолжит с места остановки, а повторные вызовы не создадут дубликаты.

## Теория простыми словами

### Что такое Workflow?

Workflow — это последовательность шагов для выполнения задачи. Каждый шаг имеет состояние (pending, running, completed, failed) и может быть повторён при ошибке.

### Что такое State Management?

State Management — это сохранение состояния агента между перезапусками. Это позволяет:
- Продолжить выполнение после сбоя
- Отслеживать прогресс задачи
- Гарантировать идемпотентность

### Что такое идемпотентность?

Идемпотентность — это свойство операции: повторный вызов даёт тот же результат, что и первый. Например, "создать файл" не идемпотентно (создаст дубликат), а "создать файл, если его нет" — идемпотентно.

### Прод-паттерн: AgentState + артефакты вместо "толстого" контекста

В продакшене workflow обычно "живет" дольше одного HTTP-запроса и переживает рестарты воркеров. Поэтому полезно хранить **AgentState** как каноническое состояние agent run (цель, бюджеты, план, факты, вопросы, risk flags), а большие результаты инструментов складывать как **артефакты**:

- рантайм/воркер сохраняет сырой результат (логи, JSON, файлы) во внешнем хранилище,
- в состояние добавляет запись `artifact_id + summary + bytes`,
- в контекст LLM возвращает только короткий excerpt (top-k строк) + `artifact_id`.

Так вы снижаете стоимость и не "убиваете" контекстное окно, даже если задача длинная и шагов много.

## Как это работает (пошагово)

### Шаг 1: Структура задачи с состоянием

Создайте структуру для хранения состояния задачи:

```go
type TaskState string

const (
    TaskPending   TaskState = "pending"
    TaskRunning   TaskState = "running"
    TaskCompleted TaskState = "completed"
    TaskFailed    TaskState = "failed"
)

type Task struct {
    ID        string    `json:"id"`
    UserInput string    `json:"user_input"`
    State     TaskState `json:"state"`
    Result    string    `json:"result,omitempty"`
    Error     string    `json:"error,omitempty"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

### Шаг 2: Идемпотентность операций

Проверяйте, не выполнялась ли задача уже:

```go
func executeTask(id string) error {
    // Загружаем задачу из БД
    task, exists := getTask(id)
    if !exists {
        return fmt.Errorf("task not found: %s", id)
    }
    
    // Проверяем идемпотентность
    if task.State == TaskCompleted {
        return nil // Уже выполнено, ничего не делаем
    }
    
    // Устанавливаем состояние "running"
    task.State = TaskRunning
    task.UpdatedAt = time.Now()
    saveTask(task)
    
    // Выполняем задачу...
    result, err := doWork(task.UserInput)
    
    if err != nil {
        task.State = TaskFailed
        task.Error = err.Error()
    } else {
        task.State = TaskCompleted
        task.Result = result
    }
    
    task.UpdatedAt = time.Now()
    saveTask(task)
    
    return err
}
```

### Шаг 3: Retry с экспоненциальным backoff

Повторяйте вызов при ошибке с увеличивающейся задержкой:

```go
func executeWithRetry(fn func() error, maxRetries int) error {
    var lastErr error
    
    for i := 0; i < maxRetries; i++ {
        err := fn()
        if err == nil {
            return nil
        }
        
        lastErr = err
        
        // Не делаем backoff после последней попытки
        if i < maxRetries-1 {
            backoff := time.Duration(1<<i) * time.Second // 1s, 2s, 4s, 8s...
            time.Sleep(backoff)
        }
    }
    
    return fmt.Errorf("failed after %d retries: %v", maxRetries, lastErr)
}
```

### Шаг 4: Дедлайны

Установите timeout для всего agent run и для каждого шага:

```go
func runAgentWithDeadline(ctx context.Context, client *openai.Client, userInput string) (string, error) {
    // Дедлайн для всего agent run (5 минут)
    ctx, cancel := context.WithDeadline(ctx, time.Now().Add(5*time.Minute))
    defer cancel()
    
    // ... agent loop ...
    
    for i := 0; i < maxIterations; i++ {
        // Проверяем дедлайн перед каждой итерацией
        select {
        case <-ctx.Done():
            return "", fmt.Errorf("deadline exceeded")
        default:
        }
        
        // ... выполнение ...
    }
}
```

### Шаг 5: Сохранение состояния

Сохраняйте состояние задачи в БД (или файл для простоты):

```go
// Простая реализация с файлом
var tasks = make(map[string]*Task)
var tasksMutex sync.RWMutex

func saveTask(task *Task) {
    tasksMutex.Lock()
    defer tasksMutex.Unlock()
    
    task.UpdatedAt = time.Now()
    tasks[task.ID] = task
    
    // Сохраняем в файл (для простоты)
    data, _ := json.Marshal(tasks)
    os.WriteFile("tasks.json", data, 0644)
}

func getTask(id string) (*Task, bool) {
    tasksMutex.RLock()
    defer tasksMutex.RUnlock()
    
    task, exists := tasks[id]
    return task, exists
}
```

## Где это встраивать в нашем коде

### Точка интеграции 1: Agent Loop

В `labs/lab04-autonomy/main.go` добавьте сохранение состояния:

```go
// В начале agent run:
taskID := generateTaskID()
task := &Task{
    ID:        taskID,
    UserInput: userInput,
    State:     TaskRunning,
    CreatedAt: time.Now(),
}
saveTask(task)

// В цикле сохраняем прогресс:
task.State = TaskRunning
saveTask(task)

// После завершения:
task.State = TaskCompleted
task.Result = finalAnswer
saveTask(task)
```

### Точка интеграции 2: Tool Execution

В `labs/lab02-tools/main.go` добавьте retry для инструментов:

```go
func executeToolWithRetry(toolCall openai.ToolCall) (string, error) {
    return executeWithRetry(func() error {
        result, err := executeTool(toolCall)
        if err != nil {
            return err
        }
        return nil
    }, 3)
}
```

## Мини-пример кода

Полный пример с workflow и state management на базе `labs/lab04-autonomy/main.go`:

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "sync"
    "time"

    "github.com/sashabaranov/go-openai"
)

type TaskState string

const (
    TaskPending   TaskState = "pending"
    TaskRunning   TaskState = "running"
    TaskCompleted TaskState = "completed"
    TaskFailed    TaskState = "failed"
)

type Task struct {
    ID        string    `json:"id"`
    UserInput string    `json:"user_input"`
    State     TaskState `json:"state"`
    Result    string    `json:"result,omitempty"`
    Error     string    `json:"error,omitempty"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

var tasks = make(map[string]*Task)
var tasksMutex sync.RWMutex

func generateTaskID() string {
    return fmt.Sprintf("task-%d", time.Now().UnixNano())
}

func saveTask(task *Task) {
    tasksMutex.Lock()
    defer tasksMutex.Unlock()
    
    task.UpdatedAt = time.Now()
    tasks[task.ID] = task
    
    data, _ := json.Marshal(tasks)
    os.WriteFile("tasks.json", data, 0644)
}

func getTask(id string) (*Task, bool) {
    tasksMutex.RLock()
    defer tasksMutex.RUnlock()
    
    task, exists := tasks[id]
    return task, exists
}

func executeWithRetry(fn func() error, maxRetries int) error {
    var lastErr error
    
    for i := 0; i < maxRetries; i++ {
        err := fn()
        if err == nil {
            return nil
        }
        
        lastErr = err
        
        if i < maxRetries-1 {
            backoff := time.Duration(1<<i) * time.Second
            fmt.Printf("Retry %d/%d after %v...\n", i+1, maxRetries, backoff)
            time.Sleep(backoff)
        }
    }
    
    return fmt.Errorf("failed after %d retries: %v", maxRetries, lastErr)
}

func checkDisk() string { return "Disk Usage: 95% (CRITICAL). Large folder: /var/log" }
func cleanLogs() string { return "Logs cleaned. Freed 20GB." }

func main() {
    token := os.Getenv("OPENAI_API_KEY")
    baseURL := os.Getenv("OPENAI_BASE_URL")
    if token == "" {
        token = "dummy"
    }

    config := openai.DefaultConfig(token)
    if baseURL != "" {
        config.BaseURL = baseURL
    }
    client := openai.NewClientWithConfig(config)

    ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Minute))
    defer cancel()

    userInput := "У меня кончилось место. Разберись."

    // Создаём задачу
    taskID := generateTaskID()
    task := &Task{
        ID:        taskID,
        UserInput: userInput,
        State:     TaskRunning,
        CreatedAt: time.Now(),
    }
    saveTask(task)

    tools := []openai.Tool{
        {
            Type: openai.ToolTypeFunction,
            Function: &openai.FunctionDefinition{
                Name:        "check_disk",
                Description: "Check current disk usage",
            },
        },
        {
            Type: openai.ToolTypeFunction,
            Function: &openai.FunctionDefinition{
                Name:        "clean_logs",
                Description: "Delete old logs to free space",
            },
        },
    }

    messages := []openai.ChatCompletionMessage{
        {Role: openai.ChatMessageRoleSystem, Content: "You are an autonomous DevOps agent."},
        {Role: openai.ChatMessageRoleUser, Content: userInput},
    }

    fmt.Printf("Starting Agent Loop (task_id: %s)...\n", taskID)

    for i := 0; i < 5; i++ {
        // Проверяем дедлайн
        select {
        case <-ctx.Done():
            task.State = TaskFailed
            task.Error = "deadline exceeded"
            saveTask(task)
            fmt.Println("Deadline exceeded")
            return
        default:
        }

        req := openai.ChatCompletionRequest{
            Model:    openai.GPT3Dot5Turbo,
            Messages: messages,
            Tools:    tools,
        }

        resp, err := client.CreateChatCompletion(ctx, req)
        if err != nil {
            task.State = TaskFailed
            task.Error = err.Error()
            saveTask(task)
            panic(fmt.Sprintf("API Error: %v", err))
        }

        msg := resp.Choices[0].Message
        messages = append(messages, msg)

        if len(msg.ToolCalls) == 0 {
            task.State = TaskCompleted
            task.Result = msg.Content
            saveTask(task)
            fmt.Println("AI:", msg.Content)
            break
        }

        for _, toolCall := range msg.ToolCalls {
            fmt.Printf("Executing tool: %s\n", toolCall.Function.Name)

            var result string
            err := executeWithRetry(func() error {
                if toolCall.Function.Name == "check_disk" {
                    result = checkDisk()
                } else if toolCall.Function.Name == "clean_logs" {
                    result = cleanLogs()
                }
                return nil
            }, 3)

            if err != nil {
                task.State = TaskFailed
                task.Error = err.Error()
                saveTask(task)
                fmt.Printf("Tool execution failed: %v\n", err)
                continue
            }

            fmt.Println("Tool Output:", result)

            messages = append(messages, openai.ChatCompletionMessage{
                Role:       openai.ChatMessageRoleTool,
                Content:    result,
                ToolCallID: toolCall.ID,
            })
        }
    }
}
```

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
            Model:    openai.GPT3Dot5Turbo,
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

### Ошибка 1: Нет идемпотентности

**Симптом:** Повторный вызов создаёт дубликаты (например, создаёт два файла вместо одного).

**Причина:** Операции не проверяют, не выполнялись ли они уже.

**Решение:**
```go
// ПЛОХО
func createFile(filename string) error {
    os.WriteFile(filename, []byte("data"), 0644)
    return nil
}

// ХОРОШО
func createFileIfNotExists(filename string) error {
    if _, err := os.Stat(filename); err == nil {
        return nil // Уже существует
    }
    return os.WriteFile(filename, []byte("data"), 0644)
}
```

### Ошибка 2: Нет retry при ошибках

**Симптом:** Агент падает при первой же временной ошибке (network error, timeout).

**Причина:** Нет повторных попыток при ошибках.

**Решение:**
```go
// ПЛОХО
result, err := executeTool(toolCall)
if err != nil {
    return "", err // Сразу возвращаем ошибку
}

// ХОРОШО
err := executeWithRetry(func() error {
    result, err := executeTool(toolCall)
    return err
}, 3)
```

### Ошибка 3: Нет дедлайнов

**Симптом:** Агент зависает навсегда, пользователь ждёт.

**Причина:** Нет timeout для операций.

**Решение:**
```go
// ПЛОХО
resp, _ := client.CreateChatCompletion(ctx, req)
// Может зависнуть навсегда

// ХОРОШО
ctx, cancel := context.WithDeadline(ctx, time.Now().Add(5*time.Minute))
defer cancel()
resp, err := client.CreateChatCompletion(ctx, req)
```

### Ошибка 4: Состояние не сохраняется

**Симптом:** После перезапуска агент начинает с начала, теряя прогресс.

**Причина:** Состояние хранится только в памяти.

**Решение:**
```go
// ПЛОХО
var taskState = "running" // Только в памяти

// ХОРОШО
task.State = TaskRunning
saveTask(task) // Сохраняем в БД/файл
```

## Мини-упражнения

### Упражнение 1: Реализуйте retry с backoff

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

### Упражнение 2: Реализуйте идемпотентность

Создайте функцию, которая проверяет, не выполнялась ли задача уже:

```go
func executeTaskIfNotDone(taskID string) error {
    // Ваш код здесь
    // Проверяйте состояние задачи перед выполнением
}
```

**Ожидаемый результат:**
- Если задача уже выполнена, функция возвращает nil без выполнения
- Если задача не выполнена, функция выполняет её и сохраняет состояние

## Критерии сдачи / Чек-лист

**Сдано (готовность к прод):**
- [x] Реализована идемпотентность операций (повторный вызов не создаёт дубликатов)
- [x] Реализованы retries с экспоненциальным backoff
- [x] Установлены дедлайны для agent run и отдельных операций
- [x] Состояние задачи сохраняется между перезапусками
- [x] Можно продолжить выполнение задачи после сбоя

**Не сдано:**
- [ ] Нет идемпотентности
- [ ] Нет retry при ошибках
- [ ] Нет дедлайнов
- [ ] Состояние не сохраняется

## Связь с другими главами

- **[Глава 11: State Management](../11-state-management/README.md)** — Базовые концепции (идемпотентность, retries, дедлайны, persist)
- **[Глава 04: Автономность и Циклы](../04-autonomy-and-loops/README.md)** — Базовый цикл агента
- **[Глава 19: Observability и Tracing](../19-observability-and-tracing/README.md)** — Логирование состояния задач
- **[Глава 20: Cost & Latency Engineering](../20-cost-latency-engineering/README.md)** — Контроль стоимости долгих задач


