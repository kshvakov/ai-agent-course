# Методическое пособие: Lab 12 — Tool Server Protocol

## Зачем это нужно?

В этой лабе вы реализуете tool server — архитектуру, где инструменты работают в отдельных процессах и общаются с агентом через протоколы (stdio или HTTP).

### Реальный кейс

**Ситуация:** Агент использует множество инструментов, написанных на разных языках.

**Без tool server:**
- Все инструменты скомпилированы с агентом
- Обновление инструмента требует пересборки агента
- Инструменты на разных языках сложно интегрировать

**С tool server:**
- Инструменты в отдельных процессах
- Обновление инструмента не требует пересборки агента
- Инструменты могут быть написаны на любом языке
- Лучшая изоляция и безопасность

**Разница:** Tool server позволяет независимо разрабатывать и обновлять инструменты.

## Теория простыми словами

### Архитектура Tool Server

**Монолитный агент (Lab 01-09):**
```
[Agent Process]
  ├── Tool 1 (Go)
  ├── Tool 2 (Go)
  └── Tool 3 (Go)
```
Все в одном процессе, все на одном языке.

**Tool Server архитектура (Lab 12):**
```
[Agent Process]  ←→  [Tool Server Process]
                        ├── Tool 1 (Go)
                        ├── Tool 2 (Python)
                        └── Tool 3 (Bash)
```
Инструменты в отдельном процессе, могут быть на разных языках.

### Протоколы коммуникации

**stdio Protocol:**
- Простой: читать из stdin, писать в stdout
- Формат JSON request/response
- Хорошо для локальных инструментов
- Пример: `echo '{"tool":"check_status"}' | tool-server`

**HTTP Protocol:**
- REST API для выполнения инструментов
- Лучше для распределенных систем
- Можно вызывать из любого места
- Пример: `POST http://localhost:8080/execute`

### Версионирование схем

Инструменты развиваются со временем:
- Версия 1.0: `check_status(hostname)`
- Версия 2.0: `check_status(hostname, timeout)` — добавлен параметр

**Стратегия версионирования:**
- Каждый инструмент имеет версию
- Агент указывает требуемую версию
- Tool server проверяет совместимость
- Возвращает ошибку, если версии не совпадают

**Пример:**
```go
ToolDefinition{
    Name:           "check_status",
    Version:        "2.0",
    CompatibleWith: []string{"1.0", "1.1", "2.0"},
}
```

## Алгоритм выполнения

### Шаг 1: stdio Protocol

```go
func (s *StdioToolServer) Start() error {
    scanner := bufio.NewScanner(os.Stdin)
    encoder := json.NewEncoder(os.Stdout)
    
    for scanner.Scan() {
        var req ToolRequest
        if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
            continue
        }
        
        // Проверяем версию
        tool := s.tools[req.Tool]
        if !checkVersionCompatibility(tool, req.Version) {
            encoder.Encode(ToolResponse{
                Success: false,
                Error:  "Version mismatch",
            })
            continue
        }
        
        // Выполняем инструмент
        result, err := executeTool(req.Tool, req.Arguments)
        if err != nil {
            encoder.Encode(ToolResponse{
                Success: false,
                Error:  err.Error(),
            })
            continue
        }
        
        // Возвращаем результат
        encoder.Encode(ToolResponse{
            Success: true,
            Result:  result,
        })
    }
    
    return scanner.Err()
}
```

### Шаг 2: HTTP Protocol

```go
func (s *HTTPToolServer) Start(port string) error {
    http.HandleFunc("/execute", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != "POST" {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }
        
        var req ToolRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }
        
        // Проверяем версию
        tool := s.tools[req.Tool]
        if !checkVersionCompatibility(tool, req.Version) {
            json.NewEncoder(w).Encode(ToolResponse{
                Success: false,
                Error:  "Version mismatch",
            })
            return
        }
        
        // Выполняем инструмент
        result, err := executeTool(req.Tool, req.Arguments)
        if err != nil {
            json.NewEncoder(w).Encode(ToolResponse{
                Success: false,
                Error:  err.Error(),
            })
            return
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(ToolResponse{
            Success: true,
            Result:  result,
        })
    })
    
    return http.ListenAndServe(":"+port, nil)
}
```

### Шаг 3: Проверка совместимости версий

```go
func checkVersionCompatibility(tool *ToolDefinition, requestedVersion string) bool {
    // Точное совпадение
    if tool.Version == requestedVersion {
        return true
    }
    
    // Проверяем совместимые версии
    for _, compatible := range tool.CompatibleWith {
        if compatible == requestedVersion {
            return true
        }
    }
    
    return false
}
```

### Шаг 4: Tool Client для агента

```go
type HTTPToolClient struct {
    baseURL string
    client  *http.Client
}

func (c *HTTPToolClient) CallTool(tool string, version string, arguments json.RawMessage) (string, error) {
    req := ToolRequest{
        Tool:      tool,
        Version:   version,
        Arguments: arguments,
    }
    
    data, _ := json.Marshal(req)
    resp, err := c.client.Post(c.baseURL+"/execute", "application/json", bytes.NewBuffer(data))
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    
    var toolResp ToolResponse
    if err := json.NewDecoder(resp.Body).Decode(&toolResp); err != nil {
        return "", err
    }
    
    if !toolResp.Success {
        return "", fmt.Errorf("tool error: %s", toolResp.Error)
    }
    
    return toolResp.Result, nil
}
```

## Типовые ошибки

### Ошибка 1: Версия не проверяется

**Симптом:** Инструмент вызывается с несовместимой версией.

**Причина:** Не проверяется совместимость версий.

**Решение:** Всегда вызывайте `checkVersionCompatibility()` перед выполнением.

### Ошибка 2: Протокол не реализован

**Симптом:** Агент не может вызвать tool server.

**Причина:** Протокол не реализован или реализован неправильно.

**Решение:** Следуйте формату JSON request/response строго.

### Ошибка 3: Инструменты не изолированы

**Симптом:** Ошибка в одном инструменте крашит весь сервер.

**Причина:** Инструменты выполняются в том же процессе.

**Решение:** Используйте отдельные процессы для каждого инструмента или обрабатывайте панику.

## Критерии сдачи

✅ **Сдано:**
- stdio протокол реализован
- HTTP протокол реализован
- Версионирование работает
- Агент может вызывать tool server
- Совместимость версий проверяется

❌ **Не сдано:**
- Протокол не реализован
- Версия не проверяется
- Агент не может вызвать tool server

