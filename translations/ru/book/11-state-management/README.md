# 11. State Management

## Зачем это нужно?

Агент выполняет долгую задачу (например, развёртывание приложения), и сервер перезагружается. Задача теряется, пользователь ждёт, но ничего не происходит. Без управления состоянием вы не можете:
- Продолжить выполнение после сбоя
- Гарантировать идемпотентность (повторный вызов не создаёт дубликатов)
- Обрабатывать ошибки с retry
- Устанавливать дедлайны для долгих задач

State Management — это надёжность для долгоживущих агентов. Без него агент не может работать с задачами, которые занимают минуты или часы.

### Реальный кейс

**Ситуация:** Агент развёртывает приложение. Процесс занимает 10 минут. На 8-й минуте сервер перезагружается.

**Проблема:** Задача теряется. Пользователь не знает, что произошло. При повторном запуске агент начинает с начала, создавая дубликаты.

**Решение:** Сохранение состояния в БД, идемпотентность операций, retry с backoff, дедлайны. Теперь агент может продолжить выполнение с места остановки, а повторные вызовы не создают дубликатов.

## Теория простыми словами

### Что такое State Management?

State Management — это сохранение состояния агента между перезапусками. Это позволяет:
- Продолжить выполнение после сбоя
- Отслеживать прогресс задачи
- Гарантировать идемпотентность

### Что такое идемпотентность?

Идемпотентность — это свойство операции: повторный вызов даёт тот же результат, что и первый. Например, "создать файл" не идемпотентно (создаст дубликат), а "создать файл, если его нет" — идемпотентно.

### Связь с Planning

State Management тесно связан с [Planning](../10-planning-and-workflows/README.md), но фокусируется на надёжности выполнения, а не на декомпозиции задач. Planning создаёт план, State Management гарантирует его надёжное выполнение.

## Как это работает (пошагово)

### Шаг 0: Состояние агента как контракт (AgentState)

В примерах ниже мы храним состояние "задачи" (`Task`). В прод-агенте удобно ввести отдельный контракт состояния **agent run**. Тогда агент может:

- продолжать выполнение после рестартов,
- пересобирать план по мере новых данных,
- требовать подтверждение (HITL) по политике,
- экономить контекст через артефакты.

Минимальная каноническая форма (упрощённо):

```json
{
  "goal": "Развернуть сервис X в staging",
  "constraints": {
    "human_in_the_loop": { "required_for_risk_levels": ["write_local", "external_action"] }
  },
  "budget": {
    "max_steps": 20,
    "max_wall_time_ms": 300000,
    "max_llm_tokens": 200000,
    "max_artifact_bytes_in_context": 8000
  },
  "plan": ["Проверить текущий статус", "Собрать конфиг", "Применить изменения", "Проверить снова"],
  "known_facts": [{ "key": "service", "value": "X", "source": "user" }],
  "open_questions": ["В какой namespace деплоить?"],
  "artifacts": [{ "artifact_id": "log_123", "type": "tool_result.logs", "summary": "nginx error log", "bytes": 48231 }],
  "risk_flags": ["budget_pressure"]
}
```

А для обновления такого состояния между шагами удобно использовать **StatePatch**: "добавь факты", "замени план", "добавь вопросы". Это помогает разделить роли: один компонент нормализует данные, другой выбирает следующий шаг.

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

### Шаг 6: Возобновление выполнения

Продолжайте выполнение задачи после сбоя:

```go
func resumeTask(taskID string) error {
    task, exists := getTask(taskID)
    if !exists {
        return fmt.Errorf("task not found: %s", taskID)
    }
    
    // Если задача уже завершена, ничего не делаем
    if task.State == TaskCompleted {
        return nil
    }
    
    // Если задача упала, можно повторить
    if task.State == TaskFailed {
        task.State = TaskPending
        saveTask(task)
    }
    
    // Продолжаем выполнение
    return executeTask(taskID)
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

✅ **Сдано (готовность к прод):**
- Реализована идемпотентность операций (повторный вызов не создаёт дубликатов)
- Реализованы retries с экспоненциальным backoff
- Установлены дедлайны для agent run и отдельных операций
- Состояние задачи сохраняется между перезапусками
- Можно продолжить выполнение задачи после сбоя

❌ **Не сдано:**
- Нет идемпотентности
- Нет retry при ошибках
- Нет дедлайнов
- Состояние не сохраняется

## Связь с другими главами

- **[Глава 04: Автономность и Циклы](../04-autonomy-and-loops/README.md)** — Базовый цикл агента
- **[Глава 10: Planning и Workflow-паттерны](../10-planning-and-workflows/README.md)** — State Management гарантирует надёжное выполнение планов
- **[Глава 19: Observability и Tracing](../19-observability-and-tracing/README.md)** — Логирование состояния задач
- **[Глава 20: Cost & Latency Engineering](../20-cost-latency-engineering/README.md)** — Контроль стоимости долгих задач

## Что дальше?

После понимания state management переходите к:
- **[12. Системы Памяти Агента](../12-agent-memory/README.md)** — Узнайте, как агенты запоминают и извлекают информацию

---

**Навигация:** [← Planning и Workflow-паттерны](../10-planning-and-workflows/README.md) | [Оглавление](../README.md) | [Системы Памяти Агента →](../12-agent-memory/README.md)

