# 04. Инструменты и Function Calling — "руки" агента

## Зачем это нужно?

Инструменты превращают LLM из болтуна в работника. Без инструментов агент может только отвечать текстом, но не может взаимодействовать с реальным миром.

Function Calling — это механизм, который позволяет модели вызывать реальные функции Go, выполнять команды, читать данные и выполнять действия.

### Реальный кейс

**Ситуация:** Вы создали чат-бота для DevOps. Пользователь пишет: "Проверь статус сервера web-01"

**Проблема:** Бот не может реально проверить сервер. Он только говорит: "Я проверю статус сервера web-01 для вас..." (текст)

**Решение:** Function Calling позволяет модели вызывать реальные функции Go. Модель генерирует структурированный JSON с именем функции и аргументами, ваш код выполняет функцию и возвращает результат обратно в модель.

## Теория простыми словами

### Как работает Function Calling?

1. **Вы описываете функцию** в формате JSON Schema
2. **LLM видит описание** и решает: "Мне нужно вызвать эту функцию"
3. **LLM генерирует JSON** с именем функции и аргументами
4. **Ваш код парсит JSON** и выполняет реальную функцию
5. **Результат возвращается** в LLM для дальнейшей обработки

## Function Calling — механизм работы

**Function Calling** — это механизм, при котором LLM возвращает не текст, а структурированный JSON с именем функции и аргументами.

### Полный цикл: от определения до выполнения

Давайте разберем **полный цикл** на примере инструмента `ping`:

#### Шаг 1: Определение инструмента (Tool Schema)

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

**Что происходит:** Мы описываем инструмент в формате JSON Schema. Это описание отправляется в модель вместе с запросом.

#### Шаг 2: Запрос к модели

```go
messages := []openai.ChatCompletionMessage{
    {Role: "system", Content: "You are a network admin. Use tools to check connectivity."},
    {Role: "user", Content: "Проверь доступность google.com"},
}

req := openai.ChatCompletionRequest{
    Model:    openai.GPT3Dot5Turbo,
    Messages: messages,
    Tools:    tools,  // Модель видит описание инструментов!
    Temperature: 0,
}

resp, _ := client.CreateChatCompletion(ctx, req)
msg := resp.Choices[0].Message
```

**Что происходит:** Модель видит:
- System prompt (роль и инструкции)
- User input (запрос пользователя)
- **Tools schema** (описание доступных инструментов)

#### Шаг 3: Ответ модели (Tool Call)

Модель **не возвращает текст** "Я проверю ping". Она возвращает **структурированный tool call**:

```go
// msg.ToolCalls содержит:
[]openai.ToolCall{
    {
        ID: "call_abc123",
        Type: "function",
        Function: openai.FunctionCall{
            Name:      "ping",
            Arguments: `{"host": "google.com"}`,
        },
    },
}
```

**Что происходит:** Модель **сгенерировала tool_call** для инструмента `ping` и JSON с аргументами. Это **не магия** — модель видела `Description: "Ping a host to check connectivity"` и связала это с запросом пользователя.

**Как модель выбирает между несколькими инструментами?**

Давайте расширим пример, добавив несколько инструментов:

```go
tools := []openai.Tool{
    {
        Function: &openai.FunctionDefinition{
            Name:        "ping",
            Description: "Ping a host to check network connectivity. Use this when user asks about network reachability or connectivity.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "check_http",
            Description: "Check HTTP status code of a website. Use this when user asks about website availability or HTTP errors.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "traceroute",
            Description: "Trace the network path to a host. Use this when user asks about network routing or path analysis.",
        },
    },
}

userInput := "Проверь доступность google.com"
```

**Процесс выбора:**

1. Модель видит **все три инструмента** и их `Description`:
   - `ping`: "check network connectivity... Use this when user asks about network reachability"
   - `check_http`: "Check HTTP status... Use this when user asks about website availability"
   - `traceroute`: "Trace network path... Use this when user asks about routing"

2. Модель сопоставляет запрос "Проверь доступность google.com" с описаниями:
   - ✅ `ping` — описание содержит "connectivity" и "reachability" → **выбирает этот**
   - ❌ `check_http` — про HTTP статус, не про сетевую доступность
   - ❌ `traceroute` — про маршрутизацию, не про проверку доступности

3. Модель возвращает tool call для `ping`:
   ```json
   {"name": "ping", "arguments": "{\"host\": \"google.com\"}"}
   ```

**Пример с другим запросом:**

```go
userInput := "Проверь, отвечает ли сайт google.com"

// Модель видит те же 3 инструмента
// Сопоставляет:
// - ping: про сетевую доступность → не совсем то
// - check_http: "Use this when user asks about website availability" → ✅ ВЫБИРАЕТ ЭТОТ
// - traceroute: про маршрутизацию → не подходит

// Модель возвращает:
// {"name": "check_http", "arguments": "{\"url\": \"https://google.com\"}"}
```

**Ключевой момент:** Модель выбирает инструмент на основе **семантического соответствия** между запросом пользователя и `Description`. Чем точнее и конкретнее `Description`, тем лучше модель выбирает нужный инструмент.

#### Шаг 4: Валидация (Runtime)

```go
// Проверяем, что инструмент существует
if msg.ToolCalls[0].Function.Name != "ping" {
    return fmt.Errorf("unknown tool: %s", msg.ToolCalls[0].Function.Name)
}

// Валидируем JSON аргументов
var args struct {
    Host string `json:"host"`
}
if err := json.Unmarshal([]byte(msg.ToolCalls[0].Function.Arguments), &args); err != nil {
    return fmt.Errorf("invalid JSON: %v", err)
}

// Проверяем обязательные поля
if args.Host == "" {
    return fmt.Errorf("host is required")
}
```

**Что происходит:** Runtime валидирует вызов перед выполнением. Это **критично** для безопасности.

#### Шаг 5: Выполнение инструмента

```go
func executePing(host string) string {
    cmd := exec.Command("ping", "-c", "1", host)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Sprintf("Error: %s", err)
    }
    return string(output)
}

result := executePing(args.Host)  // "PING google.com: 64 bytes from ..."
```

**Что происходит:** Runtime выполняет **реальную функцию** (в данном случае системную команду `ping`).

#### Шаг 6: Возврат результата в модель

```go
// Добавляем результат в историю как сообщение с ролью "tool"
messages = append(messages, openai.ChatCompletionMessage{
    Role:       openai.ChatMessageRoleTool,
    Content:    result,  // "PING google.com: 64 bytes from ..."
    ToolCallID: msg.ToolCalls[0].ID,  // Связываем с вызовом
})

// Отправляем обновленную историю в модель снова
resp2, _ := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
    Model:    openai.GPT3Dot5Turbo,
    Messages: messages,  // Теперь включает результат инструмента!
    Tools:    tools,
})
```

**Что происходит:** Модель видит результат выполнения инструмента и может:
- Сформулировать финальный ответ пользователю
- Вызвать другой инструмент, если нужно
- Задать уточняющий вопрос

#### Шаг 7: Финальный ответ

```go
finalMsg := resp2.Choices[0].Message
if len(finalMsg.ToolCalls) == 0 {
    // Это финальный текстовый ответ
    fmt.Println(finalMsg.Content)  // "google.com доступен, время отклика 10ms"
}
```

**Что происходит:** Модель видела результат `ping` и сформулировала понятный ответ для пользователя.

## Сквозной протокол: полный запрос и два хода

Теперь разберем **полный протокол** с точки зрения разработчика агента: где что хранится, как собирается запрос, и как runtime обрабатывает ответы.

### Где что хранится в коде агента?

```mermaid
graph TB
    A["System Prompt (в коде/конфиге)"] --> D[ChatCompletionRequest]
    B["Tools Registry (в коде runtime)"] --> D
    C["User Input (от пользователя)"] --> D
    D --> E[LLM API]
    E --> F{Ответ}
    F -->|tool_calls| G[Runtime валидирует]
    G --> H[Runtime выполняет]
    H --> I[Добавляет tool result]
    I --> D
    F -->|content| J[Финальный ответ]
```

**Схема хранения:**

1. **System Prompt** — хранится в коде агента (константа или конфиг):
   - Инструкции (Role, Goal, Constraints)
   - Few-shot примеры (если используются)
   - SOP (алгоритм действий)

2. **Tools Schema** — хранится в **registry runtime** (не в промпте!):
   - Определения инструментов (JSON Schema)
   - Функции-обработчики инструментов
   - Валидация и выполнение

3. **User Input** — приходит от пользователя:
   - Текущий запрос
   - История диалога (хранится в `messages[]`)

4. **Tool Results** — генерируются runtime'ом:
   - После выполнения инструмента
   - Добавляются в `messages[]` с `Role = "tool"`

### Полный протокол: JSON запросы и ответы (2 хода)

**Ход 1: Request с несколькими инструментами**

```json
{
  "model": "gpt-3.5-turbo",
  "messages": [
    {
      "role": "system",
      "content": "Ты DevOps инженер. Используй инструменты для проверки сервисов.\n\nПримеры использования:\nUser: \"Проверь статус nginx\"\nAssistant: возвращает tool_call check_status(\"nginx\")\n\nUser: \"Перезапусти сервер\"\nAssistant: возвращает tool_call restart_service(\"web-01\")"
    },
    {
      "role": "user",
      "content": "Проверь статус nginx"
    }
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "check_status",
        "description": "Check if a service is running. Use this when user asks about service status.",
        "parameters": {
          "type": "object",
          "properties": {
            "service": {
              "type": "string",
              "description": "Service name"
            }
          },
          "required": ["service"]
        }
      }
    },
    {
      "type": "function",
      "function": {
        "name": "restart_service",
        "description": "Restart a systemd service. Use this when user explicitly asks to restart a service.",
        "parameters": {
          "type": "object",
          "properties": {
            "service_name": {
              "type": "string",
              "description": "Service name to restart"
            }
          },
          "required": ["service_name"]
        }
      }
    }
  ],
  "tool_choice": "auto"
}
```

**Где что находится:**
- **System Prompt** (инструкции + few-shot примеры) → `messages[0].content`
- **User Input** → `messages[1].content`
- **Tools Schema** (2 инструмента с полными JSON Schema) → отдельное поле `tools[]`

**Response #1 (tool call):**

```json
{
  "id": "chatcmpl-abc123",
  "choices": [{
    "message": {
      "role": "assistant",
      "content": null,
      "tool_calls": [
        {
          "id": "call_xyz789",
          "type": "function",
          "function": {
            "name": "check_status",
            "arguments": "{\"service\": \"nginx\"}"
          }
        }
      ]
    }
  }]
}
```

**Runtime обрабатывает tool call:**
1. Валидирует: `tool_calls[0].function.name` существует в registry
2. Парсит: `json.Unmarshal(tool_calls[0].function.arguments)` → `{"service": "nginx"}`
3. Выполняет: `check_status("nginx")` → результат: `"Service nginx is ONLINE"`
4. Добавляет tool result в `messages[]`

**Ход 2: Request с tool result**

```json
{
  "model": "gpt-3.5-turbo",
  "messages": [
    {
      "role": "system",
      "content": "Ты DevOps инженер. Используй инструменты для проверки сервисов.\n\nПримеры использования:\nUser: \"Проверь статус nginx\"\nAssistant: возвращает tool_call check_status(\"nginx\")\n\nUser: \"Перезапусти сервер\"\nAssistant: возвращает tool_call restart_service(\"web-01\")"
    },
    {
      "role": "user",
      "content": "Проверь статус nginx"
    },
    {
      "role": "assistant",
      "content": null,
      "tool_calls": [
        {
          "id": "call_xyz789",
          "type": "function",
          "function": {
            "name": "check_status",
            "arguments": "{\"service\": \"nginx\"}"
          }
        }
      ]
    },
    {
      "role": "tool",
      "content": "Service nginx is ONLINE",
      "tool_call_id": "call_xyz789"
    }
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "check_status",
        "description": "Check if a service is running. Use this when user asks about service status.",
        "parameters": {
          "type": "object",
          "properties": {
            "service": {
              "type": "string",
              "description": "Service name"
            }
          },
          "required": ["service"]
        }
      }
    },
    {
      "type": "function",
      "function": {
        "name": "restart_service",
        "description": "Restart a systemd service. Use this when user explicitly asks to restart a service.",
        "parameters": {
          "type": "object",
          "properties": {
            "service_name": {
              "type": "string",
              "description": "Service name to restart"
            }
          },
          "required": ["service_name"]
        }
      }
    }
  ]
}
```

**Где что находится:**
- **System Prompt** → `messages[0].content` (тот же)
- **User Input** → `messages[1].content` (тот же)
- **Tool Call** → `messages[2]` (добавлен runtime'ом после первого ответа)
- **Tool Result** → `messages[3].content` (добавлен runtime'ом после выполнения)
- **Tools Schema** → поле `tools[]` (тот же)

**Response #2 (финальный ответ):**

```json
{
  "id": "chatcmpl-def456",
  "choices": [{
    "message": {
      "role": "assistant",
      "content": "nginx работает нормально, сервис ONLINE",
      "tool_calls": null
    }
  }]
}
```

### Эволюция messages[] массива (JSON)

**До первого запроса:**
```json
[
  {"role": "system", "content": "Ты DevOps инженер..."},
  {"role": "user", "content": "Проверь статус nginx"}
]
```

**После первого запроса (runtime добавляет tool call):**
```json
[
  {"role": "system", "content": "Ты DevOps инженер..."},
  {"role": "user", "content": "Проверь статус nginx"},
  {
    "role": "assistant",
    "content": null,
    "tool_calls": [{
      "id": "call_xyz789",
      "type": "function",
      "function": {
        "name": "check_status",
        "arguments": "{\"service\": \"nginx\"}"
      }
    }]
  }
]
```

**После выполнения инструмента (runtime добавляет tool result):**
```json
[
  {"role": "system", "content": "Ты DevOps инженер..."},
  {"role": "user", "content": "Проверь статус nginx"},
  {
    "role": "assistant",
    "content": null,
    "tool_calls": [{
      "id": "call_xyz789",
      "type": "function",
      "function": {
        "name": "check_status",
        "arguments": "{\"service\": \"nginx\"}"
      }
    }]
  },
  {
    "role": "tool",
    "content": "Service nginx is ONLINE",
    "tool_call_id": "call_xyz789"
  }
]
```

**После второго запроса (модель видит tool result и формулирует ответ):**
- Модель видит весь контекст (system + user + tool call + tool result)
- Генерирует финальный ответ: `"nginx работает нормально, сервис ONLINE"`

**Примечание:** Для реализации на Go см. примеры в [Lab 02: Tools](../../../labs/lab02-tools/README.md) и [Lab 04: Autonomy](../../../labs/lab04-autonomy/README.md)

### Ключевые моменты для разработчика

1. **System Prompt и Tools Schema — разные вещи:**
   - System Prompt — текст в `Messages[0].Content` (может содержать few-shot примеры)
   - Tools Schema — отдельное поле `Tools` в запросе (JSON Schema)

2. **Few-shot примеры — внутри System Prompt:**
   - Это текст, показывающий модели формат ответа или выбор инструментов
   - Отличается от Tools Schema (которая описывает структуру инструментов)

3. **Runtime управляет циклом:**
   - Валидирует `tool_calls`
   - Выполняет инструменты
   - Добавляет результаты в `messages`
   - Отправляет следующий запрос

4. **Tools не "внутри промпта":**
   - В API они передаются отдельным полем `Tools`
   - Модель видит их вместе с промптом, но это разные части запроса

См. как писать инструкции и примеры: **[Глава 02: Промптинг](../02-prompt-engineering/README.md)**

Практика: **[Lab 02: Tools](../../../labs/lab02-tools/README.md)**, **[Lab 04: Autonomy](../../../labs/lab04-autonomy/README.md)**

### Почему это не магия?

**Ключевые моменты:**

1. **Модель видит описание ВСЕХ инструментов** — она не "знает" про инструменты из коробки, она видит их `Description` в JSON Schema. Модель выбирает нужный инструмент, сопоставляя запрос пользователя с описаниями.

2. **Механизм выбора основан на семантике** — модель ищет соответствие между:
   - Запросом пользователя ("Проверь доступность")
   - Описанием инструмента ("Use this when user asks about network reachability")
   - Контекстом предыдущих результатов (если есть)

3. **Модель возвращает структурированный JSON** — это не текст "я вызову ping", а конкретный tool call с именем инструмента и аргументами

4. **Runtime делает всю работу** — парсинг, валидация, выполнение, возврат результата

5. **Модель видит результат** — она получает результат как новое сообщение в истории и продолжает работу

**Пример выбора между похожими инструментами:**

```go
tools := []openai.Tool{
    {
        Function: &openai.FunctionDefinition{
            Name:        "check_service_status",
            Description: "Check if a systemd service is running. Use this for Linux services like nginx, mysql, etc.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "check_http_status",
            Description: "Check HTTP response code of a web service. Use this for checking if a website or API is responding.",
        },
    },
}

// Запрос 1: "Проверь статус nginx"
// Модель выбирает: check_service_status (nginx - это systemd service)

// Запрос 2: "Проверь, отвечает ли сайт example.com"
// Модель выбирает: check_http_status (сайт - это HTTP сервис)
```

**Важно:** Если описания инструментов слишком похожи или неясны, модель может выбрать неправильный инструмент. Поэтому `Description` должен быть **конкретным и различимым**.

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

## Типовые ошибки

### Ошибка 1: Модель не генерирует tool_call

**Симптом:** Агент получает от модели текстовый ответ вместо `tool_calls`. Модель отвечает текстом вместо вызова инструмента.

**Причина:** 
- Модель не обучена на function calling
- Плохое описание инструмента (`Description` неясное)
- `Temperature > 0` (слишком случайно)

**Решение:**
```go
// ХОРОШО: Используйте модель с поддержкой tools
// Проверьте через Lab 00: Capability Check

// ХОРОШО: Улучшите Description
Description: "Check the status of a server by hostname. Use this when user asks about server status or availability."

// ХОРОШО: Temperature = 0
Temperature: 0,  // Детерминированное поведение
```

### Ошибка 2: Сломанный JSON в аргументах

**Симптом:** `json.Unmarshal` возвращает ошибку. JSON в аргументах некорректен.

**Причина:** Модель генерирует некорректный JSON (пропущены скобки, неправильный формат).

**Решение:**
```go
// ХОРОШО: Валидация JSON перед парсингом
if !json.Valid([]byte(call.Function.Arguments)) {
    return "Error: Invalid JSON", nil
}

// ХОРОШО: Temperature = 0 для детерминированного JSON
Temperature: 0,
```

### Ошибка 3: Галлюцинации инструментов

**Симптом:** Агент вызывает несуществующий инструмент. Модель генерирует tool_call с именем, которого нет в списке.

**Причина:** Модель не видит четкий список доступных инструментов или список слишком большой.

**Решение:**
```go
// ХОРОШО: Валидация имени инструмента перед выполнением
allowedFunctions := map[string]bool{
    "get_server_status": true,
    "ping":              true,
}
if !allowedFunctions[call.Function.Name] {
    return "Error: Unknown function", nil
}
```

### Ошибка 4: Плохое описание инструмента

**Симптом:** Модель выбирает неправильный инструмент или не выбирает его вообще.

**Причина:** `Description` слишком общее или не содержит ключевых слов из запроса пользователя.

**Решение:**
```text
// ПЛОХО
Description: "Ping a host"

// ХОРОШО
Description: "Ping a host to check network connectivity. Use this when user asks about network reachability or connectivity."
```

## Мини-упражнения

### Упражнение 1: Создайте инструмент

Создайте инструмент `check_disk_usage`, который проверяет использование диска:

```go
tools := []openai.Tool{
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name:        "check_disk_usage",
            Description: "...",  // Ваш код здесь
            Parameters:  json.RawMessage(`{
                // Ваш код здесь
            }`),
        },
    },
}
```

**Ожидаемый результат:**
- `Description` содержит ключевые слова: "disk", "usage", "space"
- JSON Schema корректна
- Обязательные поля указаны в `required`

### Упражнение 2: Валидация аргументов

Реализуйте функцию валидации аргументов для инструмента:

```go
func validateToolCall(call openai.ToolCall) error {
    // Проверьте имя инструмента
    // Проверьте валидность JSON
    // Проверьте обязательные поля
}
```

**Ожидаемый результат:**
- Функция возвращает ошибку, если имя инструмента неизвестно
- Функция возвращает ошибку, если JSON некорректен
- Функция возвращает ошибку, если отсутствуют обязательные поля

## Критерии сдачи / Чек-лист

✅ **Сдано:**
- `Description` конкретное и понятное (содержит ключевые слова)
- JSON Schema корректна
- Обязательные поля указаны в `required`
- Валидация аргументов реализована
- Ошибки обрабатываются и возвращаются агенту
- Критические инструменты требуют подтверждения
- Модель успешно генерирует tool_call

❌ **Не сдано:**
- Модель не генерирует tool_call (плохое описание или неподходящая модель)
- JSON в аргументах сломан (нет валидации)
- Модель вызывает несуществующий инструмент (нет валидации имени)
- `Description` слишком общее (модель не может выбрать правильный инструмент)

## Связь с другими главами

- **Физика LLM:** Почему модель выбирает tool call вместо текста, см. [Главу 01: Физика LLM](../01-llm-fundamentals/README.md)
- **Промптинг:** Как описать инструменты так, чтобы модель их правильно использовала, см. [Главу 02: Промптинг](../02-prompt-engineering/README.md)
- **Цикл:** Как результаты инструментов возвращаются в модель, см. [Главу 05: Автономность](../05-autonomy-and-loops/README.md)

## Что дальше?

После изучения инструментов переходите к:
- **[05. Автономность и Циклы](../05-autonomy-and-loops/README.md)** — как агент работает в цикле

---

**Навигация:** [← Анатомия Агента](../03-agent-architecture/README.md) | [Оглавление](../README.md) | [Автономность →](../05-autonomy-and-loops/README.md)

