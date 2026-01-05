# Методическое пособие: Lab 08 — Multi-Agent Systems

## Зачем это нужно?

Один агент "мастер на все руки" часто путается в инструментах. Эффективнее разделить ответственность: создать команду узких специалистов, управляемую главным агентом (Supervisor).

### Реальный кейс

**Ситуация:** Пользователь просит: "Проверь, доступен ли сервер БД, и если да — узнай версию базы."

**Без Multi-Agent:**
- Один агент должен знать и `ping`, и SQL
- Контекст переполняется
- Агент может перепутать инструменты

**С Multi-Agent:**
- Supervisor: Раздает задачи специалистам
- Network Specialist: Знает только `ping`
- DB Specialist: Знает только SQL
- Каждый специалист фокусируется на своей задаче

**Разница:** Изоляция контекста и специализация повышают надежность.

## Теория простыми словами

### Паттерн Supervisor (Начальник-Подчиненный)

**Архитектура:**

- **Supervisor:** Главный мозг. Не имеет инструментов, но знает, кто что умеет.
- **Workers:** Специализированные агенты с узким набором инструментов.

**Изоляция контекста:** Worker не видит всей переписки Supervisor-а, только свою задачу. Это экономит токены и фокусирует внимание.

**Пример:**

```
Supervisor получает: "Проверь, доступен ли сервер БД, и если да — узнай версию"

Supervisor думает:
- Сначала нужно проверить сеть → делегирую Network Specialist
- Потом нужно проверить БД → делегирую DB Specialist

Network Specialist получает: "Проверь доступность db-host.example.com"
→ Вызывает ping("db-host.example.com")
→ Возвращает: "Host is reachable"

DB Specialist получает: "Какая версия PostgreSQL на db-host?"
→ Вызывает sql_query("SELECT version()")
→ Возвращает: "PostgreSQL 15.2"

Supervisor собирает результаты и отвечает пользователю
```

### Рекурсия и изоляция

Технически, вызов агента — это просто вызов функции. Внутри функции `runWorkerAgent` мы создаем **новый** контекст диалога (новый массив `messages`). У работника своя короткая память, он не видит переписку супервайзера с пользователем (инкапсуляция контекста).

## Алгоритм выполнения

### Шаг 1: Определение инструментов для Supervisor

```go
supervisorTools := []openai.Tool{
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name: "ask_network_expert",
            Description: "Ask the network specialist about connectivity, pings, ports.",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {"question": {"type": "string"}},
                "required": ["question"]
            }`),
        },
    },
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name: "ask_database_expert",
            Description: "Ask the DB specialist about SQL, schemas, data.",
            // ...
        },
    },
}
```

**Важно:** Инструменты Supervisor-а — это функции вызова других агентов!

### Шаг 2: Определение инструментов для Workers

```go
netTools := []openai.Tool{
    {Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: "ping", Description: "Ping a host"}},
}

dbTools := []openai.Tool{
    {Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: "run_sql", Description: "Run SQL query"}},
}
```

### Шаг 3: Функция запуска Worker-а

```go
func runWorkerAgent(role, prompt, question string, tools []openai.Tool, client *openai.Client) string {
    // Создаем НОВЫЙ контекст для работника
    messages := []openai.ChatCompletionMessage{
        {Role: openai.ChatMessageRoleSystem, Content: prompt},
        {Role: openai.ChatMessageRoleUser, Content: question},
    }
    
    // Простой цикл для работника (1-2 шага обычно)
    for i := 0; i < 5; i++ {
        resp, _ := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
            Model: openai.GPT3Dot5Turbo,
            Messages: messages,
            Tools: tools,
        })
        msg := resp.Choices[0].Message
        messages = append(messages, msg)
        
        if len(msg.ToolCalls) == 0 {
            return msg.Content  // Возвращаем финальный ответ работника
        }
        
        // Выполняем инструменты работника
        for _, tc := range msg.ToolCalls {
            result := executeWorkerTool(tc.Function.Name)
            messages = append(messages, toolResult)
        }
    }
    return "Worker failed."
}
```

### Шаг 4: Цикл Supervisor-а

```go
for i := 0; i < 10; i++ {
    resp := client.CreateChatCompletion(ctx, supervisorRequest)
    msg := resp.Choices[0].Message
    messages = append(messages, msg)
    
    if len(msg.ToolCalls) == 0 {
        fmt.Printf("Supervisor: %s\n", msg.Content)
        break
    }
    
    for _, tc := range msg.ToolCalls {
        var workerResponse string
        if tc.Function.Name == "ask_network_expert" {
            workerResponse = runWorkerAgent("NetworkAdmin", "You are a Network Admin", question, netTools, client)
        } else if tc.Function.Name == "ask_database_expert" {
            workerResponse = runWorkerAgent("DBAdmin", "You are a DBA", question, dbTools, client)
        }
        
        // Возвращаем ответ работника Supervisor-у
        messages = append(messages, ChatCompletionMessage{
            Role: openai.ChatMessageRoleTool,
            Content: workerResponse,
            ToolCallID: tc.ID,
        })
    }
}
```

## Типовые ошибки

### Ошибка 1: Worker видит контекст Supervisor-а

**Симптом:** Worker получает всю историю Supervisor-а, контекст переполняется.

**Причина:** Вы передаете `messages` Supervisor-а в Worker.

**Решение:**
```go
// ПЛОХО
runWorkerAgent(supervisorMessages, ...)  // Worker видит всю историю!

// ХОРОШО
runWorkerAgent([]ChatCompletionMessage{systemMsg, questionMsg}, ...)  // Только своя задача
```

### Ошибка 2: Supervisor не получает ответ Worker-а

**Симптом:** Supervisor вызывает Worker, но не видит результат.

**Причина:** Ответ Worker-а не добавлен в историю Supervisor-а.

**Решение:**
```go
// ОБЯЗАТЕЛЬНО добавляйте ответ Worker-а:
messages = append(messages, ChatCompletionMessage{
    Role: openai.ChatMessageRoleTool,
    Content: workerResponse,  // Supervisor должен это увидеть!
    ToolCallID: tc.ID,
})
```

### Ошибка 3: Worker зацикливается

**Симптом:** Worker выполняет цикл бесконечно.

**Причина:** Нет лимита итераций для Worker-а.

**Решение:**
```go
for i := 0; i < 5; i++ {  // Лимит для Worker-а
    // ...
}
```

## Мини-упражнения

### Упражнение 1: Добавьте третьего специалиста

Создайте Security Specialist с инструментами `query_siem` и `check_ip_reputation`.

### Упражнение 2: Добавьте логирование

Логируйте, кто что делает:

```go
fmt.Printf("[Supervisor] Delegating to: %s\n", workerName)
fmt.Printf("[Worker: %s] Executing: %s\n", workerName, action)
fmt.Printf("[Worker: %s] Result: %s\n", workerName, result)
```

## Критерии сдачи

✅ **Сдано:**
- Supervisor делегирует задачи Workers
- Workers работают изолированно
- Ответы Workers возвращаются Supervisor-у
- Supervisor собирает результаты и отвечает пользователю

❌ **Не сдано:**
- Supervisor не делегирует задачи
- Workers видят контекст Supervisor-а
- Ответы Workers не возвращаются

---

**Поздравляем!** Вы завершили курс. Теперь вы умеете строить промышленных AI-агентов.

