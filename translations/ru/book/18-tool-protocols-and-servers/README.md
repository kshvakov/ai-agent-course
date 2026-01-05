# 18. Протоколы Инструментов и Tool Servers

## Зачем это нужно?

По мере роста агентов инструменты становятся сложными сервисами. Вместо встраивания кода инструментов напрямую, можно запускать инструменты как отдельные процессы или сервисы. Это требует протоколов для коммуникации, версионирования и безопасности.

Эта глава покрывает паттерны tool servers: stdio протоколы, HTTP API, gRPC, версионирование схем и аутентификацию/авторизацию.

### Реальный кейс

**Ситуация:** У вас 50+ инструментов. Некоторые написаны на Go, некоторые на Python, некоторые — внешние сервисы. Встраивание всех в один бинарник агента непрактично.

**Проблема:**
- Разные языки требуют разных подходов интеграции
- Обновления инструментов требуют переразвёртывания агента
- Нет изоляции между инструментами
- Сложно масштабировать отдельные инструменты

**Решение:** Tool servers: Каждый инструмент запускается как отдельный процесс/сервис. Агент общается через стандартный протокол (stdio, HTTP или gRPC). Инструменты могут обновляться независимо, масштабироваться отдельно и изолироваться для безопасности.

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

✅ **Сдано:**
- Понимаете архитектуру tool server
- Можете реализовать stdio протокол
- Можете реализовать HTTP протокол
- Понимаете преимущества gRPC для production tool servers
- Понимаете версионирование схем

❌ **Не сдано:**
- Нет версионирования, обновления ломают совместимость
- Нет аутентификации, риск безопасности

## Связь с другими главами

- **[Глава 03: Инструменты и Function Calling](../03-tools-and-function-calling/README.md)** — Основы выполнения инструментов
- **[Глава 17: Security и Governance](../17-security-and-governance/README.md)** — Безопасность для tool servers

## Что дальше?

После понимания протоколов инструментов переходите к:
- **[19. Observability и Tracing](../19-observability-and-tracing/README.md)** — Узнайте прод-observability

---

**Навигация:** [← Security и Governance](../17-security-and-governance/README.md) | [Оглавление](../README.md) | [Observability и Tracing →](../19-observability-and-tracing/README.md)

