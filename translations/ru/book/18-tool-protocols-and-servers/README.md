# 18. Протоколы Инструментов и Tool Servers

## Зачем это нужно?

По мере роста агентов инструменты часто превращаются в сервисы. Вместо того чтобы встраивать их код прямо в агент, их можно запускать отдельными процессами или сервисами. Тогда нужны протоколы для коммуникации, версионирования и безопасности.

В этой главе разберём паттерны tool servers: stdio, HTTP API, gRPC, версионирование схем и аутентификацию/авторизацию. Также рассмотрим два стандартных протокола: MCP (Model Context Protocol) для подключения агентов к инструментам и A2A (Agent-to-Agent) для взаимодействия агентов между собой.

### Реальный кейс

**Ситуация:** У вас 50+ инструментов. Некоторые написаны на Go, некоторые на Python, некоторые — внешние сервисы. Встраивание всех в один бинарник агента непрактично.

**Проблема:**
- Разные языки требуют разных подходов интеграции
- Обновления инструментов требуют переразвёртывания агента
- Нет изоляции между инструментами
- Сложно масштабировать отдельные инструменты

**Решение:** Tool servers: каждый инструмент запускается как отдельный процесс/сервис. Агент общается через стандартный протокол (stdio, HTTP или gRPC). Инструменты можно обновлять и масштабировать независимо, и изолировать для безопасности.

## Теория простыми словами

### Архитектура Tool Server

**Agent Runtime:**
- Управляет потоком разговора
- Вызывает инструменты через протокол
- Обрабатывает ответы инструментов

**Tool Server:**
- Реализует логику инструмента
- Предоставляет интерфейс протокола
- Может быть отдельным процессом/сервисом

**Протокол:**
- Контракт коммуникации
- Формат запроса/ответа
- Обработка ошибок

### Типы протоколов

**1. stdio Протокол:**
- Инструмент запускается как подпроцесс
- Коммуникация через stdin/stdout
- Просто, хорошо для локальных инструментов

**2. HTTP Протокол:**
- Инструмент запускается как HTTP сервис
- REST API интерфейс
- Хорошо для распределённых систем

**3. gRPC Протокол:**
- Инструмент запускается как gRPC сервис
- Строгий контракт через Protobuf (IDL)
- Типобезопасность и обратная совместимость схем
- Богатая экосистема в Go: кодогенерация клиентов/серверов, интерсепторы, reflection
- Встроенные механизмы: TLS/mTLS, аутентификация через metadata, дедлайны, ретраи, балансировка
- Наблюдаемость: интеграция с tracing/metrics/logging
- Практичный выбор для production tool servers

## Как это работает (пошагово)

### Шаг 1: Интерфейс протокола инструмента

```go
type ToolServer interface {
    ListTools() ([]ToolDefinition, error)
    ExecuteTool(name string, args map[string]any) (any, error)
}

type ToolDefinition struct {
    Name        string
    Description string
    Schema      json.RawMessage
    Version     string
}
```

### Шаг 2: stdio Протокол

```go
// Tool server читает из stdin, пишет в stdout
type StdioToolServer struct {
    tools map[string]Tool
}

func (s *StdioToolServer) Run() {
    scanner := bufio.NewScanner(os.Stdin)
    for scanner.Scan() {
        var req ToolRequest
        json.Unmarshal(scanner.Bytes(), &req)
        
        result, err := s.ExecuteTool(req.Name, req.Args)
        
        resp := ToolResponse{
            Result: result,
            Error:  errString(err),
        }
        
        json.NewEncoder(os.Stdout).Encode(resp)
    }
}

type ToolRequest struct {
    Name string
    Args map[string]any
}

type ToolResponse struct {
    Result any
    Error  string
}
```

### Шаг 3: Агент вызывает Tool Server

```go
func executeToolViaStdio(toolName string, args map[string]any) (any, error) {
    cmd := exec.Command("tool-server")
    stdin, _ := cmd.StdinPipe()
    stdout, _ := cmd.StdoutPipe()
    
    cmd.Start()
    
    req := ToolRequest{Name: toolName, Args: args}
    json.NewEncoder(stdin).Encode(req)
    stdin.Close()
    
    var resp ToolResponse
    json.NewDecoder(stdout).Decode(&resp)
    
    cmd.Wait()
    
    if resp.Error != "" {
        return nil, fmt.Errorf(resp.Error)
    }
    return resp.Result, nil
}
```

### Шаг 4: HTTP Протокол

```go
// Tool server как HTTP сервис
func (s *HTTPToolServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    var req ToolRequest
    json.NewDecoder(r.Body).Decode(&req)
    
    result, err := s.ExecuteTool(req.Name, req.Args)
    
    resp := ToolResponse{
        Result: result,
        Error:  errString(err),
    }
    
    json.NewEncoder(w).Encode(resp)
}
```

### Шаг 5: gRPC Протокол

gRPC предоставляет строгий контракт через Protocol Buffers. Определение сервиса:

```protobuf
syntax = "proto3";

package tools.v1;

service ToolServer {
  rpc ListTools(ListToolsRequest) returns (ListToolsResponse);
  rpc ExecuteTool(ExecuteToolRequest) returns (ExecuteToolResponse);
}

message ListToolsRequest {
  string version = 1; // Версия протокола
}

message ListToolsResponse {
  repeated ToolDefinition tools = 1;
}

message ExecuteToolRequest {
  string tool_name = 1;
  string version = 2;
  bytes arguments = 3; // JSON-сериализованные аргументы
}

message ExecuteToolResponse {
  bytes result = 1;
  string error = 2;
}

message ToolDefinition {
  string name = 1;
  string description = 2;
  string schema = 3; // JSON Schema
  string version = 4;
  repeated string compatible_versions = 5;
}
```

**Преимущества gRPC для tool servers:**
- **Строгий контракт**: Protobuf гарантирует типобезопасность и эволюцию схем без breaking changes
- **Экосистема Go**: Автоматическая генерация клиентов/серверов, интерсепторы для authn/authz, health checks
- **Безопасность**: Встроенная поддержка TLS/mTLS, аутентификация через metadata (токены, API ключи)
- **Надёжность**: Дедлайны, ретраи, балансировка на уровне клиента или через service mesh
- **Наблюдаемость**: Интеграция с OpenTelemetry, метрики gRPC, structured logging

### Шаг 6: Версионирование схем

```go
type ToolDefinition struct {
    Name        string
    Version     string
    Schema      json.RawMessage
    CompatibleVersions []string
}

func (s *ToolServer) GetToolDefinition(name string, version string) (*ToolDefinition, error) {
    tool := s.tools[name]
    if tool.Version == version {
        return &tool, nil
    }
    
    // Проверяем совместимость
    for _, v := range tool.CompatibleVersions {
        if v == version {
            return &tool, nil
        }
    }
    
    return nil, fmt.Errorf("несовместимая версия")
}
```

## MCP (Model Context Protocol)

### Что такое MCP

MCP (Model Context Protocol) — открытый стандарт от Anthropic для подключения LLM-агентов к инструментам и данным. Вместо того чтобы каждый агент писал свою интеграцию с каждым сервисом, MCP задаёт единый протокол. Инструмент, написанный как MCP-сервер, работает с любым агентом, который поддерживает MCP.

Аналогия: USB для AI-инструментов. Один разъём — любое устройство.

### Архитектура

```
Host (IDE / чатбот / CI pipeline)
  └── MCP Client ──── JSON-RPC 2.0 ──── MCP Server
                                           ├── Resources  (данные для чтения)
                                           ├── Tools      (действия)
                                           └── Prompts    (шаблоны промптов)
```

**Host** — приложение, в котором работает агент (IDE, чатбот, CI/CD pipeline). Один хост может подключать несколько MCP-серверов одновременно.

**MCP Client** — компонент хоста. Устанавливает соединение с MCP-сервером по протоколу JSON-RPC 2.0.

**MCP Server** — отдельный процесс или сервис. Предоставляет инструменты, данные и шаблоны через стандартный протокол.

### Три примитива

| Примитив | Аналог | Что делает | Пример |
|----------|--------|------------|--------|
| **Resources** | GET | Данные для чтения. Агент запрашивает — сервер отдаёт | Содержимое файла, результат SQL-запроса, метрики |
| **Tools** | POST | Действия с побочными эффектами. Требуют подтверждения | Создать тикет, запустить деплой, отправить алерт |
| **Prompts** | Шаблон | Готовые промпты для типовых задач | Шаблон code review, анализ инцидента |

### Транспорт

MCP поддерживает два типа транспорта:

**stdio** — хост запускает MCP-сервер как подпроцесс. Общение через stdin/stdout. Просто, не требует сети. Подходит для локальных инструментов: CLI-утилиты, работа с файлами, IDE-плагины.

**Streamable HTTP** — клиент отправляет JSON-RPC запросы по HTTP POST. Сервер может отвечать через SSE (Server-Sent Events) для стриминга результатов. Подходит для удалённых серверов и production-окружений.

### Пример: MCP-сервер на Go

Минимальный MCP-сервер, который предоставляет инструмент для деплоя сервисов.

Типы JSON-RPC 2.0 — основа протокола MCP:

```go
type JSONRPCRequest struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      any             `json:"id,omitempty"`
    Method  string          `json:"method"`
    Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
    JSONRPC string    `json:"jsonrpc"`
    ID      any       `json:"id"`
    Result  any       `json:"result,omitempty"`
    Error   *RPCError `json:"error,omitempty"`
}

type RPCError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}
```

Определяем инструмент через JSON Schema. LLM использует описание и схему для генерации аргументов:

```go
type MCPTool struct {
    Name        string          `json:"name"`
    Description string          `json:"description"`
    InputSchema json.RawMessage `json:"inputSchema"`
}

var deployTool = MCPTool{
    Name:        "deploy_service",
    Description: "Деплой сервиса в указанное окружение",
    InputSchema: json.RawMessage(`{
        "type": "object",
        "properties": {
            "service": {"type": "string", "description": "Имя сервиса"},
            "env":     {"type": "string", "enum": ["staging", "production"]}
        },
        "required": ["service", "env"]
    }`),
}
```

Обработка запросов. MCP-сервер отвечает на три ключевых метода — `initialize`, `tools/list` и `tools/call`:

```go
func handleRequest(req JSONRPCRequest) JSONRPCResponse {
    switch req.Method {
    case "initialize":
        // Рукопожатие: сервер сообщает свои возможности
        return JSONRPCResponse{
            JSONRPC: "2.0", ID: req.ID,
            Result: map[string]any{
                "protocolVersion": "2025-03-26",
                "capabilities":    map[string]any{"tools": map[string]any{}},
                "serverInfo": map[string]any{
                    "name": "deploy-server", "version": "1.0.0",
                },
            },
        }

    case "tools/list":
        // Агент вызывает при подключении, чтобы узнать доступные инструменты
        return JSONRPCResponse{
            JSONRPC: "2.0", ID: req.ID,
            Result:  map[string]any{"tools": []MCPTool{deployTool}},
        }

    case "tools/call":
        // Вызов инструмента: агент передаёт имя и аргументы
        var params struct {
            Name      string         `json:"name"`
            Arguments map[string]any `json:"arguments"`
        }
        json.Unmarshal(req.Params, &params)

        result, err := executeDeploy(params.Arguments)
        if err != nil {
            return JSONRPCResponse{
                JSONRPC: "2.0", ID: req.ID,
                Result: map[string]any{
                    "content": []map[string]any{{"type": "text", "text": err.Error()}},
                    "isError": true,
                },
            }
        }
        return JSONRPCResponse{
            JSONRPC: "2.0", ID: req.ID,
            Result: map[string]any{
                "content": []map[string]any{{"type": "text", "text": result}},
            },
        }

    default:
        return JSONRPCResponse{
            JSONRPC: "2.0", ID: req.ID,
            Error:   &RPCError{Code: -32601, Message: "method not found"},
        }
    }
}
```

Запуск через stdio. Сервер читает JSON-RPC сообщения из stdin, пишет ответы в stdout:

```go
func main() {
    scanner := bufio.NewScanner(os.Stdin)
    scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

    for scanner.Scan() {
        var req JSONRPCRequest
        if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
            continue
        }
        resp := handleRequest(req)
        json.NewEncoder(os.Stdout).Encode(resp)
    }
}
```

### Пример: MCP-клиент

Клиент запускает MCP-сервер как подпроцесс и вызывает инструменты:

```go
type MCPClient struct {
    cmd    *exec.Cmd
    stdin  io.WriteCloser
    stdout *bufio.Scanner
    nextID int
}

func NewMCPClient(serverPath string) (*MCPClient, error) {
    cmd := exec.Command(serverPath)
    stdin, _ := cmd.StdinPipe()
    stdoutPipe, _ := cmd.StdoutPipe()

    if err := cmd.Start(); err != nil {
        return nil, fmt.Errorf("не удалось запустить MCP-сервер: %w", err)
    }

    client := &MCPClient{
        cmd:    cmd,
        stdin:  stdin,
        stdout: bufio.NewScanner(stdoutPipe),
    }

    // Инициализация — обязательный первый шаг
    _, err := client.call("initialize", map[string]any{
        "protocolVersion": "2025-03-26",
        "clientInfo":      map[string]any{"name": "my-agent", "version": "1.0.0"},
    })
    return client, err
}

func (c *MCPClient) call(method string, params any) (json.RawMessage, error) {
    c.nextID++
    req := JSONRPCRequest{
        JSONRPC: "2.0",
        ID:      c.nextID,
        Method:  method,
    }
    if params != nil {
        req.Params, _ = json.Marshal(params)
    }

    // Отправляем запрос в stdin сервера
    data, _ := json.Marshal(req)
    fmt.Fprintf(c.stdin, "%s\n", data)

    // Читаем ответ из stdout сервера
    if !c.stdout.Scan() {
        return nil, fmt.Errorf("сервер не ответил")
    }
    var resp JSONRPCResponse
    json.Unmarshal(c.stdout.Bytes(), &resp)

    if resp.Error != nil {
        return nil, fmt.Errorf("RPC error %d: %s", resp.Error.Code, resp.Error.Message)
    }
    result, _ := json.Marshal(resp.Result)
    return result, nil
}
```

Использование клиента:

```go
func main() {
    client, err := NewMCPClient("./deploy-server")
    if err != nil {
        log.Fatal(err)
    }
    defer client.cmd.Process.Kill()

    // Получаем список инструментов
    tools, _ := client.call("tools/list", nil)
    fmt.Println("Инструменты:", string(tools))

    // Вызываем инструмент
    result, _ := client.call("tools/call", map[string]any{
        "name":      "deploy_service",
        "arguments": map[string]any{"service": "api-gateway", "env": "staging"},
    })
    fmt.Println("Результат:", string(result))
}
```

### Когда использовать MCP vs прямой HTTP/gRPC

| Критерий | MCP | Прямой HTTP/gRPC |
|----------|-----|-------------------|
| **Агент использует инструмент** | Да — MCP создан для этого | Требует ручной интеграции |
| **Инструменты для нескольких агентов** | Один сервер — любой MCP-клиент | Каждый агент пишет свой клиент |
| **Высоконагруженный API** | Нет — не оптимизирован для RPS | Да — gRPC/HTTP лучший выбор |
| **Сложная бизнес-логика** | Нет — лучше отдельный сервис | Да |
| **IDE и инструменты разработчика** | Да — большинство IDE поддерживают MCP | Нет |

**Правило:** Если инструмент нужен LLM-агенту — используйте MCP. Если это API для других сервисов — используйте HTTP/gRPC.

## A2A (Agent-to-Agent Protocol)

### Что такое A2A

A2A (Agent-to-Agent) — открытый протокол от Google для взаимодействия агентов друг с другом. MCP решает задачу "агент ↔ инструмент". A2A решает другую задачу — "агент ↔ агент".

Зачем это нужно? Когда агентов много и их создают разные команды, нужен стандартный способ обнаруживать друг друга и делегировать задачи. A2A даёт это из коробки.

### Ключевые концепции

**Agent Card** — JSON-документ, описывающий возможности агента. Публикуется по адресу `/.well-known/agent.json`. Любой агент может получить карточку и понять, какие задачи умеет решать другой агент.

**Task (задача)** — единица работы. Один агент создаёт задачу, другой выполняет. У задачи есть жизненный цикл с чёткими статусами.

**Message (сообщение)** — коммуникация в рамках задачи. Содержит части (Part): текст, файлы, структурированные данные.

**Artifact (артефакт)** — результат выполнения задачи. Агент-исполнитель возвращает артефакты по мере готовности.

### Жизненный цикл задачи

```
submitted ──→ working ──→ completed
                │      ──→ failed
                │      ──→ canceled
                ▼
          input-required ──→ working ──→ ...
```

- **submitted** — задача создана, ждёт обработки
- **working** — агент работает над задачей
- **input-required** — агенту нужна дополнительная информация от вызывающей стороны
- **completed** — задача выполнена, артефакты готовы
- **failed** — задача завершилась ошибкой
- **canceled** — задача отменена вызывающей стороной

### Пример: Agent Card и A2A-сервер

Agent Card — описание возможностей агента:

```go
// Agent Card — публикуется по /.well-known/agent.json
type AgentCard struct {
    Name         string       `json:"name"`
    Description  string       `json:"description"`
    URL          string       `json:"url"`
    Version      string       `json:"version"`
    Capabilities Capabilities `json:"capabilities"`
    Skills       []Skill      `json:"skills"`
}

type Capabilities struct {
    Streaming         bool `json:"streaming"`
    PushNotifications bool `json:"pushNotifications"`
}

// Skill — конкретная задача, которую агент умеет решать
type Skill struct {
    ID          string   `json:"id"`
    Name        string   `json:"name"`
    Description string   `json:"description"`
    InputModes  []string `json:"inputModes"`  // "text", "file", "data"
    OutputModes []string `json:"outputModes"`
}
```

Типы для работы с задачами:

```go
type Task struct {
    ID        string     `json:"id"`
    Status    TaskStatus `json:"status"`
    Messages  []Message  `json:"messages,omitempty"`
    Artifacts []Artifact `json:"artifacts,omitempty"`
}

type TaskStatus struct {
    State   string `json:"state"` // submitted, working, input-required, completed, failed
    Message string `json:"message,omitempty"`
}

type Message struct {
    Role  string `json:"role"` // "user" или "agent"
    Parts []Part `json:"parts"`
}

type Part struct {
    Type string `json:"type"` // "text", "file", "data"
    Text string `json:"text,omitempty"`
    Data any    `json:"data,omitempty"`
}

type Artifact struct {
    Name  string `json:"name"`
    Parts []Part `json:"parts"`
}
```

A2A-сервер обрабатывает задачи от других агентов:

```go
type A2AServer struct {
    card  AgentCard
    tasks sync.Map // taskID → *Task
}

func (s *A2AServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    switch r.URL.Path {
    case "/.well-known/agent.json":
        // Обнаружение: любой агент получает карточку
        json.NewEncoder(w).Encode(s.card)

    case "/tasks/send":
        // Получение задачи от другого агента
        var req struct {
            ID      string  `json:"id"`
            Message Message `json:"message"`
        }
        json.NewDecoder(r.Body).Decode(&req)

        task := &Task{
            ID:       req.ID,
            Status:   TaskStatus{State: "working"},
            Messages: []Message{req.Message},
        }
        s.tasks.Store(req.ID, task)

        // Выполняем задачу асинхронно
        go s.processTask(task)

        json.NewEncoder(w).Encode(task)

    case "/tasks/get":
        // Проверка статуса задачи
        taskID := r.URL.Query().Get("id")
        if task, ok := s.tasks.Load(taskID); ok {
            json.NewEncoder(w).Encode(task)
        } else {
            http.Error(w, "task not found", http.StatusNotFound)
        }
    }
}

func (s *A2AServer) processTask(task *Task) {
    // Извлекаем текст задачи
    input := task.Messages[0].Parts[0].Text

    // Агент выполняет свою работу...
    result := fmt.Sprintf("Проанализировано: %s", input)

    // Обновляем задачу с результатом
    task.Artifacts = []Artifact{
        {Name: "result", Parts: []Part{{Type: "text", Text: result}}},
    }
    task.Status = TaskStatus{State: "completed"}
}
```

Использование — клиент обнаруживает агента и отправляет задачу:

```go
func discoverAndDelegate(agentURL string, taskText string) (*Task, error) {
    // 1. Получаем Agent Card
    resp, _ := http.Get(agentURL + "/.well-known/agent.json")
    var card AgentCard
    json.NewDecoder(resp.Body).Decode(&card)
    resp.Body.Close()

    fmt.Printf("Агент: %s — %s\n", card.Name, card.Description)

    // 2. Отправляем задачу
    taskReq := map[string]any{
        "id": "task-001",
        "message": Message{
            Role:  "user",
            Parts: []Part{{Type: "text", Text: taskText}},
        },
    }
    body, _ := json.Marshal(taskReq)
    resp, _ = http.Post(agentURL+"/tasks/send", "application/json", bytes.NewReader(body))

    var task Task
    json.NewDecoder(resp.Body).Decode(&task)
    resp.Body.Close()

    // 3. Поллим статус до завершения
    for task.Status.State == "working" {
        time.Sleep(time.Second)
        resp, _ = http.Get(agentURL + "/tasks/get?id=" + task.ID)
        json.NewDecoder(resp.Body).Decode(&task)
        resp.Body.Close()
    }

    return &task, nil
}
```

### Когда использовать A2A vs паттерн Supervisor/Worker

| Критерий | A2A | Supervisor/Worker |
|----------|-----|-------------------|
| **Агенты от разных команд** | Да — стандартный протокол обнаружения | Нет — требует общий код |
| **Разные фреймворки** | Да — протокол не зависит от реализации | Нет — общий фреймворк |
| **Простая оркестрация** | Избыточно | Да — проще реализовать |
| **Динамическое обнаружение** | Да — через Agent Card | Нет — агенты заданы статически |
| **Одна команда, один репозиторий** | Избыточно | Да — прямые вызовы проще |

**Правило:** A2A нужен, когда агенты создаются независимыми командами и должны обнаруживать друг друга динамически. Для агентов внутри одной системы достаточно прямой оркестрации (см. [Глава 07: Мультиагентные системы](../07-multi-agent/README.md)).

## Сравнительная таблица протоколов

| | stdio | HTTP | gRPC | MCP | A2A |
|---|---|---|---|---|---|
| **Применение** | Локальные инструменты | Распределённые сервисы | Production API | Инструменты для LLM-агентов | Взаимодействие агентов |
| **Латентность** | Минимальная (IPC) | Средняя (сеть + HTTP/1.1) | Низкая (HTTP/2 + бинарный формат) | Зависит от транспорта (stdio или HTTP) | Средняя (HTTP) |
| **Сложность реализации** | Низкая | Средняя | Высокая (protobuf, codegen) | Средняя (JSON-RPC 2.0) | Средняя-высокая |
| **Обнаружение инструментов** | Нет | Swagger / OpenAPI | Reflection, service mesh | `tools/list`, `resources/list` | Agent Card (`/.well-known/agent.json`) |
| **Стриминг** | Построчный через stdout | SSE, WebSocket | Bidirectional streams | SSE (Streamable HTTP) | SSE |
| **Типизация контракта** | Нет (свободный JSON) | JSON Schema / OpenAPI | Строгая (Protobuf IDL) | JSON Schema (для инструментов) | JSON Schema |
| **Когда выбирать** | CLI-утилиты, IDE-плагины | Внешние API, вебхуки | Микросервисы, высокие нагрузки | Инструменты для LLM-агентов | Мультиагентные системы |

### Аутентификация: примеры

**HTTP — Bearer Token:**

```go
func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        if !strings.HasPrefix(token, "Bearer ") {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }

        apiKey := strings.TrimPrefix(token, "Bearer ")
        if !isValidKey(apiKey) {
            http.Error(w, "invalid token", http.StatusForbidden)
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

**MCP — аутентификация через OAuth 2.1:**

MCP использует [OAuth 2.1](https://modelcontextprotocol.io/specification/2025-03-26/basic/authorization) (IETF draft-ietf-oauth-v2-1) для удалённых серверов (Streamable HTTP транспорт). Клиент получает токен через стандартный OAuth-флоу и передаёт его в заголовке `Authorization`:

```go
// MCP-клиент с аутентификацией для HTTP-транспорта
type AuthenticatedMCPClient struct {
    serverURL string
    token     string
    client    *http.Client
}

func (c *AuthenticatedMCPClient) call(method string, params any) (json.RawMessage, error) {
    body, _ := json.Marshal(JSONRPCRequest{
        JSONRPC: "2.0",
        ID:      1,
        Method:  method,
        Params:  mustMarshal(params),
    })

    req, _ := http.NewRequest("POST", c.serverURL, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+c.token)

    resp, err := c.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusUnauthorized {
        return nil, fmt.Errorf("токен истёк, требуется повторная аутентификация")
    }

    var rpcResp JSONRPCResponse
    json.NewDecoder(resp.Body).Decode(&rpcResp)

    if rpcResp.Error != nil {
        return nil, fmt.Errorf("RPC error: %s", rpcResp.Error.Message)
    }
    result, _ := json.Marshal(rpcResp.Result)
    return result, nil
}
```

Для stdio-транспорта аутентификация обычно не нужна — сервер запускается локально как подпроцесс хоста.

## Типовые ошибки

### Ошибка 1: Нет версионирования

**Симптом:** Обновления инструментов ломают агента.

**Причина:** Нет версионирования, агент ожидает старый интерфейс.

**Решение:** Версионируйте схемы инструментов, проверяйте совместимость.

### Ошибка 2: Нет аутентификации

**Симптом:** Неавторизованный доступ к инструментам.

**Причина:** Нет authn/authz для tool servers.

**Решение:** Реализуйте аутентификацию (API ключи, токены).

## Мини-упражнения

### Упражнение 1: Реализуйте stdio Tool Server

Создайте tool server, который общается через stdio:

```go
func main() {
    server := NewStdioToolServer()
    server.Run()
}
```

**Ожидаемый результат:**
- Читает запросы из stdin
- Выполняет инструменты
- Пишет ответы в stdout

## Критерии сдачи / Чек-лист

**Сдано:**
- [x] Понимаете архитектуру tool server
- [x] Можете реализовать stdio протокол
- [x] Можете реализовать HTTP протокол
- [x] Понимаете преимущества gRPC для production tool servers
- [x] Понимаете версионирование схем
- [x] Понимаете MCP: архитектуру, три примитива, транспорт
- [x] Можете написать MCP-сервер и MCP-клиент на Go
- [x] Понимаете A2A: Agent Card, жизненный цикл задачи
- [x] Знаете, когда выбирать MCP, A2A, HTTP или gRPC

**Не сдано:**
- [ ] Нет версионирования, обновления ломают совместимость
- [ ] Нет аутентификации, риск безопасности
- [ ] Используете MCP для высоконагруженных API вместо gRPC
- [ ] Используете A2A там, где достаточно прямой оркестрации

## Связь с другими главами

- **[Глава 03: Инструменты и Function Calling](../03-tools-and-function-calling/README.md)** — Основы выполнения инструментов
- **[Глава 07: Мультиагентные системы](../07-multi-agent/README.md)** — Паттерны оркестрации агентов (Supervisor/Worker и другие)
- **[Глава 17: Security и Governance](../17-security-and-governance/README.md)** — Безопасность для tool servers

## Что дальше?

После понимания протоколов инструментов переходите к:
- **[19. Observability и Tracing](../19-observability-and-tracing/README.md)** — Узнайте прод-observability


