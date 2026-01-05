# 01. Физика LLM — как работает "мозг" агента

## Зачем это нужно?

Чтобы управлять агентом, нужно понимать, как работает его "мозг". Без понимания физики LLM вы не сможете:
- Правильно настроить модель для агента
- Понять, почему агент ведет себя недетерминированно
- Управлять контекстом и историей диалога
- Избежать галлюцинаций и ошибок

Эта глава объясняет основы работы LLM простым языком, без лишней математики.

### Реальный кейс

**Ситуация:** Вы создали агента для DevOps. Пользователь пишет: "Проверь статус сервера web-01"

**Проблема:** Агент иногда отвечает текстом "Сервер работает", а иногда вызывает инструмент `check_status`. Поведение непредсказуемо.

**Решение:** Понимание вероятностной природы LLM и настройка `Temperature = 0` делает поведение детерминированным. Понимание контекстного окна помогает управлять историей диалога.

## Теория простыми словами

### Вероятностная природа

**Ключевой факт:** LLM не думает, она предсказывает.

LLM — это функция `NextToken(Context) -> Distribution`.  
На вход подается последовательность токенов $x_1, ..., x_t$. Модель вычисляет распределение вероятностей для следующего токена:

$$P(x_{t+1} | x_1, ..., x_t)$$

**Что это значит на практике?**

#### Пример 1: DevOps — Магия vs Реальность

**❌ Магия (как обычно объясняют):**
> Промпт: `"Проверь статус сервера"`  
> Модель видит контекст и предсказывает: "Я вызову инструмент `check_status`" (вероятность 0.85)

**✅ Реальность (как на самом деле):**

**1. Что отправляется в модель:**

```go
// System Prompt (задает роль и поведение)
systemPrompt := `You are a DevOps assistant. 
When user asks about server status, use the check_status tool.
When user asks about logs, use the read_logs tool.
When user asks to restart, use the restart_service tool.`

// User Input
userInput := "Проверь статус сервера"

// Описание доступных инструментов (tools schema)
// ВАЖНО: Модель видит ВСЕ инструменты и выбирает нужный!
tools := []openai.Tool{
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name:        "check_status",
            Description: "Check the status of a server by hostname. Use this when user asks about server status or availability.",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "hostname": {"type": "string", "description": "Server hostname"}
                },
                "required": ["hostname"]
            }`),
        },
    },
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name:        "read_logs",
            Description: "Read the last N lines of service logs. Use this when user asks about logs, errors, or troubleshooting.",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "service": {"type": "string", "description": "Service name"},
                    "lines": {"type": "number", "description": "Number of lines to read"}
                },
                "required": ["service"]
            }`),
        },
    },
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name:        "restart_service",
            Description: "Restart a systemd service. Use this when user explicitly asks to restart a service. WARNING: This causes downtime.",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "service_name": {"type": "string", "description": "Service name to restart"}
                },
                "required": ["service_name"]
            }`),
        },
    },
}

// Полный запрос к API
messages := []openai.ChatCompletionMessage{
    {Role: "system", Content: systemPrompt},
    {Role: "user", Content: userInput},
}

req := openai.ChatCompletionRequest{
    Model:    openai.GPT3Dot5Turbo,
    Messages: messages,
    Tools:    tools,  // Ключевой момент: модель видит описание инструментов!
}
```

**2. Что возвращает модель:**

Модель **не возвращает текст** "Я вызову инструмент". Она возвращает **структурированный tool call**:

```json
{
  "role": "assistant",
  "content": null,
  "tool_calls": [
    {
      "id": "call_abc123",
      "type": "function",
      "function": {
        "name": "check_status",
        "arguments": "{\"hostname\": \"web-01\"}"
      }
    }
  ]
}
```

**Как модель выбирает инструмент?**

Модель видит **все три инструмента** и их `Description`:
- `check_status`: "Check the status... Use this when user asks about server status"
- `read_logs`: "Read logs... Use this when user asks about logs"
- `restart_service`: "Restart service... Use this when user explicitly asks to restart"

Запрос пользователя: "Проверь статус сервера"

Модель сопоставляет запрос с описаниями:
- ✅ `check_status` — описание содержит "server status" → **выбирает этот**
- ❌ `read_logs` — описание про логи, не про статус
- ❌ `restart_service` — описание про рестарт, не про проверку

**Пример с другим запросом:**

```go
userInput := "Покажи последние ошибки в логах nginx"

// Модель видит те же 3 инструмента
// Сопоставляет:
// - check_status: про статус, не про логи → не подходит
// - read_logs: "Use this when user asks about logs" → ✅ ВЫБИРАЕТ ЭТОТ
// - restart_service: про рестарт → не подходит

// Модель возвращает:
// tool_calls: [{function: {name: "read_logs", arguments: "{\"service\": \"nginx\", \"lines\": 50}"}}]
```

**Ключевой момент:** Модель выбирает инструмент на основе **семантического соответствия** между запросом пользователя и `Description` инструмента. Чем точнее `Description`, тем лучше выбор.

**3. Что делает Runtime:**

```go
resp, _ := client.CreateChatCompletion(ctx, req)
msg := resp.Choices[0].Message

// Runtime проверяет: есть ли tool_calls?
if len(msg.ToolCalls) > 0 {
    // Парсим аргументы
    var args struct {
        Hostname string `json:"hostname"`
    }
    json.Unmarshal([]byte(msg.ToolCalls[0].Function.Arguments), &args)
    
    // Выполняем реальную функцию
    result := checkStatus(args.Hostname)  // "Server is ONLINE"
    
    // Возвращаем результат обратно в модель
    messages = append(messages, openai.ChatCompletionMessage{
        Role:       "tool",
        Content:    result,
        ToolCallID: msg.ToolCalls[0].ID,
    })
    
    // Отправляем обновленную историю в модель снова
    // Модель видит результат и решает, что делать дальше
}
```

**Примечание о "вероятностях":**

Цифры типа "вероятность 0.85" — это **иллюстрация** для понимания. API OpenAI/локальных моделей обычно **не возвращает** эти вероятности напрямую (если не использовать `logprobs`). Важно понимать принцип: когда в контексте есть `tools` с хорошим `Description`, модель с высокой вероятностью выберет tool call вместо текста. Но это происходит **внутри модели**, мы видим только финальный выбор.

#### Пример 2: Support — Магия vs Реальность

**❌ Магия:**
> Промпт: `"Пользователь жалуется на ошибку 500"`  
> Модель предсказывает: "Сначала соберу контекст через `get_ticket_details`" (вероятность 0.9)

**✅ Реальность:**

**Что отправляется:**

```go
systemPrompt := `You are a Customer Support agent.
When user reports an error, first gather context using get_ticket_details.
When user asks about account, use check_account_status.
When you find a solution, use draft_reply to create a response.`

tools := []openai.Tool{
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name:        "get_ticket_details",
            Description: "Get ticket details including user info, error logs, and history. Use this FIRST when user reports an error or problem.",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "ticket_id": {"type": "string"}
                },
                "required": ["ticket_id"]
            }`),
        },
    },
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name:        "check_account_status",
            Description: "Check if a user account is active, locked, or suspended. Use this when user asks about account status or login issues.",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "user_id": {"type": "string"}
                },
                "required": ["user_id"]
            }`),
        },
    },
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name:        "search_kb",
            Description: "Search knowledge base for solutions to common problems. Use this after gathering ticket details to find similar cases.",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "query": {"type": "string"}
                },
                "required": ["query"]
            }`),
        },
    },
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name:        "draft_reply",
            Description: "Draft a reply message to the ticket. Use this when you have a solution or need to ask user for more information.",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "ticket_id": {"type": "string"},
                    "message": {"type": "string"}
                },
                "required": ["ticket_id", "message"]
            }`),
        },
    },
}

messages := []openai.ChatCompletionMessage{
    {Role: "system", Content: systemPrompt},
    {Role: "user", Content: "Пользователь жалуется на ошибку 500"},
}
```

**Что возвращает модель:**

```json
{
  "role": "assistant",
  "tool_calls": [
    {
      "id": "call_xyz789",
      "function": {
        "name": "get_ticket_details",
        "arguments": "{\"ticket_id\": \"TICKET-12345\"}"
      }
    }
  ]
}
```

**Как модель выбрала именно `get_ticket_details`?**

Модель видела **4 инструмента**:
- `get_ticket_details`: "Use this FIRST when user reports an error" ✅
- `check_account_status`: "Use this when user asks about account status" ❌
- `search_kb`: "Use this after gathering ticket details" ❌ (слишком рано)
- `draft_reply`: "Use this when you have a solution" ❌ (еще нет решения)

Запрос: "Пользователь жалуется на ошибку 500"

Модель сопоставила:
- ✅ `get_ticket_details` — описание говорит "FIRST when user reports an error" → **выбирает этот**
- Остальные не подходят по контексту

**Пример последовательного выбора инструментов:**

```go
// Итерация 1: Пользователь жалуется на ошибку
userInput := "Пользователь жалуется на ошибку 500"
// Модель выбирает: get_ticket_details (собирает контекст)

// Итерация 2: После получения деталей тикета
// Модель видит в контексте: "Error 500, user_id: 12345"
// Модель выбирает: search_kb("error 500") (ищет решение)

// Итерация 3: После поиска в KB
// Модель видит решение в контексте
// Модель выбирает: draft_reply(ticket_id, solution) (создает ответ)
```

**Ключевой момент:** Модель выбирает инструменты последовательно, основываясь на:
1. **Текущем запросе пользователя**
2. **Результатах предыдущих инструментов** (в контексте)
3. **Описаниях инструментов** (`Description`)

**Runtime:**
- Парсит `ticket_id` из JSON
- Вызывает реальную функцию `getTicketDetails("TICKET-12345")`
- Возвращает результат в модель как сообщение с ролью `tool`
- Модель видит результат и продолжает работу

#### Пример 3: Data Analytics — Магия vs Реальность

**❌ Магия:**
> Промпт: `"Покажи продажи за последний месяц"`  
> Модель предсказывает: "Сформулирую SQL-запрос через `sql_select`" (вероятность 0.95)

**✅ Реальность:**

**Что отправляется:**

```go
systemPrompt := `You are a Data Analyst.
When user asks for data, first check table schema using describe_table.
Then formulate SQL query and use sql_select tool.
If data quality is questionable, use check_data_quality.`

tools := []openai.Tool{
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name:        "describe_table",
            Description: "Get table schema including column names, types, and constraints. Use this FIRST when user asks about data structure or before writing SQL queries.",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "table_name": {"type": "string", "description": "Name of the table"}
                },
                "required": ["table_name"]
            }`),
        },
    },
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name:        "sql_select",
            Description: "Execute a SELECT query on the database. ONLY SELECT queries allowed. Use this when user asks for specific data or reports.",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "query": {"type": "string", "description": "SQL SELECT query"}
                },
                "required": ["query"]
            }`),
        },
    },
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name:        "check_data_quality",
            Description: "Check for data quality issues: nulls, duplicates, outliers. Use this when user asks about data quality or before analysis.",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "table_name": {"type": "string"}
                },
                "required": ["table_name"]
            }`),
        },
    },
}
```

**Что возвращает модель:**

```json
{
  "role": "assistant",
  "tool_calls": [
    {
      "function": {
        "name": "sql_select",
        "arguments": "{\"query\": \"SELECT region, SUM(amount) FROM sales WHERE date >= NOW() - INTERVAL '1 month' GROUP BY region\"}"
      }
    }
  ]
}
```

**Как модель выбрала `sql_select`?**

Модель видела **3 инструмента**:
- `describe_table`: "Use this FIRST when user asks about data structure" ❌ (пользователь не спрашивает про структуру)
- `sql_select`: "Use this when user asks for specific data or reports" ✅
- `check_data_quality`: "Use this when user asks about data quality" ❌ (не про качество)

Запрос: "Покажи продажи за последний месяц"

Модель сопоставила:
- ✅ `sql_select` — описание говорит "when user asks for specific data" → **выбирает этот**
- Остальные не подходят

**Пример с другим запросом:**

```go
userInput := "Какие поля есть в таблице sales?"

// Модель видит те же 3 инструмента
// Сопоставляет:
// - describe_table: "Use this FIRST when user asks about data structure" → ✅ ВЫБИРАЕТ ЭТОТ
// - sql_select: про выполнение запросов → не подходит
// - check_data_quality: про качество данных → не подходит

// Модель возвращает:
// tool_calls: [{function: {name: "describe_table", arguments: "{\"table_name\": \"sales\"}"}}]
```

**Пример последовательного выбора:**

```go
// Итерация 1: Пользователь спрашивает про продажи
userInput := "Почему упали продажи в регионе X?"
// Модель выбирает: describe_table("sales") (сначала нужно понять структуру)

// Итерация 2: После получения схемы таблицы
// Модель видит в контексте: "columns: date, region, amount"
// Модель выбирает: sql_select("SELECT region, SUM(amount) FROM sales WHERE region='X' GROUP BY date")

// Итерация 3: После получения данных
// Модель анализирует результаты и может выбрать: check_data_quality("sales")
// если нужно проверить качество данных перед выводом
```

**Ключевой момент:** Модель выбирает инструменты на основе:
1. **Семантического соответствия** запроса и `Description`
2. **Последовательности** (сначала schema, потом query)
3. **Контекста** предыдущих результатов

**Runtime:**
- Валидирует, что это SELECT (не DELETE/DROP!)
- Выполняет SQL через безопасное соединение (read-only)
- Возвращает результаты в модель
- Модель форматирует результаты для пользователя

### Почему это важно для инженера?

#### 1. Недетерминированность

Запустив агента дважды с одним промптом, вы можете получить разные действия.

**Пример:**
```
Запрос 1: "Проверь сервер"
Ответ 1: [Вызывает check_status]

Запрос 2: "Проверь сервер" (тот же промпт)
Ответ 2: [Отвечает текстом "Сервер работает"]
```

**Решение:** `Temperature = 0` (Greedy decoding) сжимает распределение, заставляя модель всегда выбирать наиболее вероятный путь.

```go
req := openai.ChatCompletionRequest{
    Temperature: 0,  // Детерминированное поведение
    // ...
}
```

#### 2. Галлюцинации

Модель стремится сгенерировать *правдоподобный*, а не *истинный* текст.

**DevOps пример:** Модель может написать "используй флаг `--force`" для команды, которая его не поддерживает.

**Data пример:** Модель может сгенерировать SQL с несуществующим полем `user.email` вместо `users.email`.

**Support пример:** Модель может "выдумать" решение проблемы, которого нет в базе знаний.

**Решение:** **Grounding** (Заземление). Мы даем агенту доступ к реальным данным (Tools/RAG) и запрещаем выдумывать факты.

```go
systemPrompt := `You are a DevOps assistant.
CRITICAL: Never invent facts. Always use tools to get real data.
If you don't know something, say "I don't know" or use a tool.`
```

## Токены и контекстное окно

### Что такое токен?

**Токен** — это единица текста, которую обрабатывает модель.
- Один токен ≈ 0.75 слова (в английском)
- В русском: одно слово ≈ 1.5 токена

**Пример:**
```
Текст: "Проверь статус сервера"
Токены: ["Проверь", " статус", " сервера"]  // ~3 токена
```

### Контекстное окно (Context Window)

**Контекстное окно** — это "оперативная память" модели.

**Примеры размеров контекстного окна (на момент написания):**
- GPT-3.5: 4k токенов (~3000 слов)
- GPT-4 Turbo: 128k токенов (~96000 слов)
- Llama 3 70B: 8k токенов

> **Примечание:** Конкретные модели и размеры контекста могут меняться со временем. Важно понимать принцип: чем больше контекстное окно, тем больше информации агент может "помнить" в рамках одного запроса.

**Что это значит для агента?**

Все, что агент "знает" о текущей задаче — это то, что влезает в контекстное окно (Prompt + History).

**Пример расчета (приблизительно):**
```
Контекстное окно: 4k токенов
System Prompt: 200 токенов
История диалога: 3000 токенов
Результаты инструментов: 500 токенов
Осталось места: 300 токенов
```

> **Примечание:** Это приблизительная оценка. Точный подсчет токенов зависит от модели и используемой библиотеки (например, `tiktoken` для OpenAI моделей).

Если история переполняется, агент "забывает" начало разговора.

**Модель Stateless:** Она не помнит ваш прошлый запрос, если вы не передали его снова в `messages`.

```go
// Каждый запрос должен включать всю историю
messages := []openai.ChatCompletionMessage{
    {Role: "system", Content: systemPrompt},
    {Role: "user", Content: "Проверь сервер"},
    {Role: "assistant", Content: "Проверяю..."},
    {Role: "tool", Content: "Server is ONLINE"},
    {Role: "user", Content: "А что с базой?"},  // Агент видит всю историю!
}
```

## Температура (Temperature)

**Температура** — это параметр энтропии распределения вероятностей.

```go
Temperature = 0  // Детерминировано (для агентов!)
Temperature = 0.7  // Баланс креативности и стабильности
Temperature = 1.0+  // Креативно, но нестабильно
```

### Когда использовать какое значение?

| Temperature | Использование | Пример |
|-------------|---------------|--------|
| 0.0 | Агенты, JSON-генерация, Tool Calling | DevOps-агент должен стабильно вызывать `restart_service`, а не "творить" |
| 0.1-0.3 | Структурированные ответы | Support-агент генерирует шаблоны ответов |
| 0.7-1.0 | Креативные задачи | Product-агент пишет маркетинговые тексты |

**Практический пример:**

```go
// ПЛОХО: Для агента
req := openai.ChatCompletionRequest{
    Temperature: 0.9,  // Слишком случайно!
    // ...
}

// ХОРОШО: Для агента
req := openai.ChatCompletionRequest{
    Temperature: 0,  // Максимальная детерминированность
    // ...
}
```

## Выбор модели для локального запуска

Не все модели одинаково хороши для агентов.

### Критерии выбора

1. **Поддержка Function Calling:** Модель должна уметь генерировать структурированные вызовы инструментов.
   - ✅ Хорошо: Модели с fine-tuning на function calling (например, `Hermes-2-Pro`, `Llama-3-Instruct`, `Mistral-7B-Instruct` на момент написания)
   - ❌ Плохо: Базовые модели без fine-tuning на tools
   
   > **Примечание:** Конкретные модели могут меняться. Важно проверить поддержку function calling через capability benchmark (см. [Приложение: Capability Benchmark](../appendix/README.md#capability-benchmark-characterization)).

2. **Размер контекста:** Для сложных задач нужен большой контекст.
   - Минимум: 4k токенов
   - Рекомендуется: 8k+

3. **Качество следования инструкциям:** Модель должна строго следовать System Prompt.
   - Проверяется через capability benchmark (см. [Приложение: Capability Benchmark](../appendix/README.md#capability-benchmark-characterization))

### Как проверить модель?

**Теория:** См. [Приложение: Capability Benchmark](../appendix/README.md#capability-benchmark-characterization) — подробное описание того, что проверяем и почему это важно.

**Практика:** См. [Lab 00: Model Capability Benchmark](../../labs/lab00-capability-check/README.md) — готовый инструмент для проверки модели.

## Типовые ошибки

### Ошибка 1: Модель недетерминированна

**Симптом:** Один и тот же промпт дает разные результаты. Агент иногда вызывает инструмент, иногда отвечает текстом.

**Причина:** `Temperature > 0` делает модель случайной. Она выбирает не самый вероятный токен, а случайный из распределения.

**Решение:**
```go
// ПЛОХО
req := openai.ChatCompletionRequest{
    Temperature: 0.7,  // Случайное поведение!
    // ...
}

// ХОРОШО
req := openai.ChatCompletionRequest{
    Temperature: 0,  // Всегда используйте для агентов
    // ...
}
```

### Ошибка 2: Контекст переполняется

**Симптом:** Агент "забывает" начало разговора. После N сообщений перестает помнить, что обсуждалось в начале.

**Причина:** История диалога превышает размер контекстного окна модели. Старые сообщения "выталкиваются" из контекста.

**Решение:**

Есть два подхода:

**Вариант 1: Обрезка истории (простое, но теряем информацию)**
```go
// ПЛОХО: Теряем важную информацию из начала разговора!
if len(messages) > maxHistoryLength {
    messages = append(
        []openai.ChatCompletionMessage{messages[0]},  // System
        messages[len(messages)-maxHistoryLength+1:]...,  // Последние
    )
}
```

**Вариант 2: Сжатие контекста через саммаризацию (лучшее решение)**

Вместо обрезки лучше **сжать** старые сообщения через LLM, сохранив важную информацию:

```go
// 1. Разделяем на "старые" и "новые" сообщения
systemMsg := messages[0]
oldMessages := messages[1 : len(messages)-10]  // Все кроме последних 10
recentMessages := messages[len(messages)-10:]  // Последние 10

// 2. Сжимаем старые сообщения через LLM
summary := summarizeMessages(ctx, client, oldMessages)

// 3. Собираем новый контекст: System + Summary + Recent
compressed := []openai.ChatCompletionMessage{
    systemMsg,
    {
        Role:    "system",
        Content: fmt.Sprintf("Summary of previous conversation:\n%s", summary),
    },
}
compressed = append(compressed, recentMessages...)
```

**Почему саммаризация лучше обрезки?**

- ✅ **Сохраняет важную информацию:** Имя пользователя, контекст задачи, принятые решения
- ✅ **Экономит токены:** Сжимает 2000 токенов до 200, сохраняя суть
- ✅ **Агент помнит начало:** Может отвечать на вопросы о ранних сообщениях

**Пример:**
```
Исходная история (2000 токенов):
- User: "Меня зовут Иван, я DevOps инженер"
- Assistant: "Привет, Иван!"
- User: "У нас проблема с сервером"
- Assistant: "Опишите проблему"
... (еще 50 сообщений)

После обрезки: Теряем имя и контекст ❌
После саммаризации: "Пользователь Иван, DevOps инженер. Обсуждали проблему с сервером. Текущая задача: диагностика." ✅
```

**Когда использовать:**
- **Обрезка:** Быстрые одноразовые задачи, неважна история
- **Саммаризация:** Долгие сессии, важна контекстная информация, автономные агенты

См. подробнее: раздел "Оптимизация контекста" в [Главе 09: Анатомия Агента](../09-agent-architecture/README.md#оптимизация-контекста-context-optimization) и [Lab 09: Context Optimization](../../labs/lab09-context-optimization/README.md)

### Ошибка 3: Галлюцинации

**Симптом:** Модель выдумывает факты. Например, говорит "используй флаг `--force`" для команды, которая его не поддерживает.

**Причина:** Модель стремится сгенерировать *правдоподобный* текст, а не *истинный*. Она не знает реальных фактов о вашей системе.

**Решение:**
```go
// ХОРОШО: Запрещаем выдумывать факты
systemPrompt := `You are a DevOps assistant.
CRITICAL: Never invent facts. Always use tools to get real data.
If you don't know something, say "I don't know" or use a tool.`

// Также используйте:
// 1. Tools для получения реальных данных
// 2. RAG для доступа к документации
```

## Критерии сдачи / Чек-лист

✅ **Сдано:**
- Понимаете, что LLM предсказывает токены, а не "думает"
- Знаете, как настроить `Temperature = 0` для детерминированного поведения
- Понимаете ограничения контекстного окна
- Знаете, как управлять историей диалога (саммаризация или обрезка)
- Модель поддерживает Function Calling (проверено через Lab 00)
- System Prompt запрещает галлюцинации

❌ **Не сдано:**
- Модель ведет себя недетерминированно (`Temperature > 0`)
- Агент "забывает" начало разговора (контекст переполняется)
- Модель выдумывает факты (нет grounding через Tools/RAG)

## Мини-упражнения

### Упражнение 1: Подсчет токенов

Напишите функцию, которая приблизительно подсчитывает количество токенов в тексте:

```go
func estimateTokens(text string) int {
    // Примерная оценка: 1 токен ≈ 4 символа (для английского)
    // Для русского: 1 токен ≈ 3 символа
    return len(text) / 4
}
```

**Ожидаемый результат:**
- Функция возвращает приблизительное количество токенов
- Учитывает разницу между английским и русским текстом

### Упражнение 2: Обрезка истории

Реализуйте функцию обрезки истории сообщений:

```go
func trimHistory(messages []ChatCompletionMessage, maxTokens int) []ChatCompletionMessage {
    // Оставляем System Prompt + последние сообщения, которые влезают в maxTokens
    // ...
}
```

**Ожидаемый результат:**
- System Prompt всегда остается первым
- Последние сообщения добавляются, пока не превысят maxTokens
- Функция возвращает обрезанную историю

## Для любопытных

> Этот раздел объясняет формализацию работы LLM на более глубоком уровне. Можно пропустить, если вас интересует только практика.

### Формальное определение LLM

LLM — это функция `NextToken(Context) -> Distribution`:

$$P(x_{t+1} | x_1, ..., x_t)$$

Где:
- $x_1, ..., x_t$ — последовательность токенов (контекст)
- $P(x_{t+1})$ — распределение вероятностей для следующего токена
- Модель выбирает токен на основе этого распределения

**Temperature** изменяет энтропию распределения:
- `Temperature = 0`: выбирается наиболее вероятный токен (greedy decoding)
- `Temperature > 0`: выбирается случайный токен из распределения (sampling)

### Почему модель "выбирает" инструмент?

Когда модель видит в контексте:
- System Prompt: "Use tools when needed"
- Tools Schema: `[{name: "check_status", description: "..."}]`
- User Input: "Проверь статус сервера"

Модель генерирует последовательность токенов, которая соответствует формату tool call. Это не "магия" — это результат обучения на примерах вызовов функций.

## Связь с другими главами

- **Function Calling:** Подробнее о том, как модель генерирует tool calls, см. [Главу 03: Инструменты](../03-tools-and-function-calling/README.md)
- **Контекстное окно:** Как управлять историей сообщений, см. [Главу 09: Анатомия Агента](../09-agent-architecture/README.md#оптимизация-контекста-context-optimization)
- **Temperature:** Почему для агентов используется `Temperature = 0`, см. [Главу 03: Инструменты](../03-tools-and-function-calling/README.md)

## Что дальше?

После изучения физики LLM переходите к:
- **[02. Промптинг как Программирование](../02-prompt-engineering/README.md)** — как управлять поведением модели через промпты

---

**Навигация:** [← Предисловие](../00-preface/README.md) | [Оглавление](../README.md) | [Промптинг →](../02-prompt-engineering/README.md)

