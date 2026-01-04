# 04. Инструменты и Function Calling — "руки" агента

Инструменты превращают LLM из болтуна в работника.

## Function Calling — механизм работы

**Function Calling** — это механизм, при котором LLM возвращает не текст, а структурированный JSON с именем функции и аргументами.

**Процесс:**

1. Вы описываете функцию в формате JSON Schema
2. LLM генерирует вызов: `{"name": "ping", "arguments": "{\"host\": \"google.com\"}"}`
3. Ваш код (Runtime) парсит JSON, выполняет функцию и возвращает результат в LLM

**Пример определения инструмента:**

```go
tools := []openai.Tool{
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name:        "ping",
            Description: "Ping a host to check connectivity",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "host": {
                        "type": "string",
                        "description": "Hostname or IP address to ping"
                    }
                },
                "required": ["host"]
            }`),
        },
    },
}
```

**Важно:** `Description` — это самое важное поле! LLM ориентируется именно по нему, решая, какой инструмент вызвать.

## Примеры инструментов в разных доменах

### DevOps

```go
// Проверка статуса сервиса
{
    Name: "check_service_status",
    Description: "Check if a systemd service is running",
    Parameters: {"service_name": "string"}
}

// Перезапуск сервиса
{
    Name: "restart_service",
    Description: "Restart a systemd service. WARNING: This will cause downtime.",
    Parameters: {"service_name": "string"}
}

// Чтение логов
{
    Name: "read_logs",
    Description: "Read the last N lines of service logs",
    Parameters: {"service": "string", "lines": "number"}
}
```

### Support

```go
// Получение тикета
{
    Name: "get_ticket",
    Description: "Get ticket details by ID",
    Parameters: {"ticket_id": "string"}
}

// Поиск в базе знаний
{
    Name: "search_kb",
    Description: "Search knowledge base for solutions",
    Parameters: {"query": "string"}
}

// Черновик ответа
{
    Name: "draft_reply",
    Description: "Draft a reply to the ticket",
    Parameters: {"ticket_id": "string", "message": "string"}
}
```

### Data Analytics

```go
// SQL запрос (read-only!)
{
    Name: "sql_select",
    Description: "Execute a SELECT query on the database. ONLY SELECT queries allowed.",
    Parameters: {"query": "string"}
}

// Описание таблицы
{
    Name: "describe_table",
    Description: "Get table schema and column information",
    Parameters: {"table_name": "string"}
}

// Проверка качества данных
{
    Name: "check_data_quality",
    Description: "Check for nulls, duplicates, outliers in a table",
    Parameters: {"table_name": "string"}
}
```

### Security

```go
// Запрос к SIEM
{
    Name: "query_siem",
    Description: "Query security information and event management system",
    Parameters: {"query": "string", "time_range": "string"}
}

// Изоляция хоста (требует подтверждения!)
{
    Name: "isolate_host",
    Description: "CRITICAL: Isolate a host from the network. Requires confirmation.",
    Parameters: {"host": "string"}
}

// Проверка IP репутации
{
    Name: "check_ip_reputation",
    Description: "Check if an IP address is known malicious",
    Parameters: {"ip": "string"}
}
```

## Обработка ошибок инструментов

Если инструмент вернул ошибку, агент должен это увидеть и обработать.

**Пример:**

```go
// Агент вызывает ping("nonexistent-host")
result := ping("nonexistent-host")
// result = "Error: Name or service not known"

// Добавляем ошибку в историю
messages = append(messages, ChatCompletionMessage{
    Role:    "tool",
    Content: result,  // Модель увидит ошибку!
    ToolCallID: call.ID,
})

// Модель получит ошибку и может:
// 1. Попробовать другой хост
// 2. Сообщить пользователю о проблеме
// 3. Эскалировать проблему
```

**Важно:** Ошибка — это тоже результат! Не скрывайте ошибки от модели.

## Валидация вызова инструментов

Перед выполнением инструмента нужно валидировать аргументы.

**Пример валидации:**

```go
func executeTool(name string, args json.RawMessage) (string, error) {
    switch name {
    case "restart_service":
        var params struct {
            ServiceName string `json:"service_name"`
        }
        if err := json.Unmarshal(args, &params); err != nil {
            return "", fmt.Errorf("invalid args: %v", err)
        }
        
        // Валидация
        if params.ServiceName == "" {
            return "", fmt.Errorf("service_name is required")
        }
        
        // Проверка безопасности
        if params.ServiceName == "critical-db" {
            return "", fmt.Errorf("Cannot restart critical service without confirmation")
        }
        
        return restartService(params.ServiceName), nil
    }
    return "", fmt.Errorf("unknown tool: %s", name)
}
```

## Типовые проблемы

### Проблема 1: Модель не вызывает инструменты

**Симптом:** Агент отвечает текстом вместо вызова инструмента.

**Решение:**
- Используйте модель с поддержкой tools (Hermes-2-Pro, Llama-3-Instruct)
- Улучшите `Description` инструмента
- Добавьте Few-Shot примеры в промпт

### Проблема 2: Сломанный JSON в аргументах

**Симптом:** `json.Unmarshal` возвращает ошибку.

**Решение:**
- `Temperature = 0`
- Валидация JSON перед парсингом
- Использование structured output (если модель поддерживает)

### Проблема 3: Галлюцинации инструментов

**Симптом:** Агент вызывает несуществующий инструмент.

**Решение:**
- Явно указывайте список доступных инструментов в промпте
- Валидируйте имя инструмента перед выполнением

## Чек-лист: Создание инструментов

- [ ] `Description` конкретное и понятное
- [ ] JSON Schema корректна
- [ ] Обязательные поля указаны в `required`
- [ ] Валидация аргументов реализована
- [ ] Ошибки обрабатываются и возвращаются агенту
- [ ] Критические инструменты требуют подтверждения

## Что дальше?

После изучения инструментов переходите к:
- **[05. Автономность и Циклы](../05-autonomy-and-loops/README.md)** — как агент работает в цикле

---

**Навигация:** [← Анатомия Агента](../03-agent-architecture/README.md) | [Оглавление](../README.md) | [Автономность →](../05-autonomy-and-loops/README.md)

