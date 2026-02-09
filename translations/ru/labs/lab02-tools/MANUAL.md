# Методическое пособие: Lab 02 — Function Calling (Tools)

## Зачем это нужно?

Обычная LLM возвращает текст. Но для создания агента нам нужно, чтобы модель могла вызывать функции (инструменты). Это превращает LLM из "болтуна" в "работника".

### Реальный кейс

**Ситуация:** Вы создали чат-бота для DevOps. Пользователь пишет:
- "Проверь статус сервера web-01"
- Бот отвечает: "Я проверю статус сервера web-01 для вас..." (текст)

**Проблема:** Бот не может реально проверить сервер. Он только говорит.

**Решение:** Function Calling позволяет модели вызывать реальные функции Go.

## Теория простыми словами

### Как работает Function Calling?

1. **Вы описываете функцию** в формате JSON Schema
2. **LLM видит описание** и решает: "Мне нужно вызвать эту функцию"
3. **LLM генерирует JSON** с именем функции и аргументами
4. **Ваш код парсит JSON** и выполняет реальную функцию
5. **Результат возвращается** в LLM для дальнейшей обработки

### Почему не все модели умеют Tools?

Function Calling — это результат специальной тренировки. Если модель не видела примеров вызовов функций, она будет просто продолжать диалог текстом.

**Как проверить:** Запустите Lab 00 перед этой лабой!

## Алгоритм выполнения

### Шаг 1: Определение инструмента

```go
tools := []openai.Tool{
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name:        "get_server_status",
            Description: "Get the status of a server by IP address",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "ip": {
                        "type": "string",
                        "description": "IP address of the server"
                    }
                },
                "required": ["ip"]
            }`),
        },
    },
}
```

**Важно:** `Description` — это самое важное поле! LLM ориентируется именно по нему.

### Шаг 2: Отправка запроса с инструментами

```go
req := openai.ChatCompletionRequest{
    Model:    "gpt-4o-mini",
    Messages: messages,
    Tools:    tools,  // Передаем список инструментов
}
```

### Шаг 3: Обработка ответа

```go
resp, err := client.CreateChatCompletion(ctx, req)
msg := resp.Choices[0].Message

// Проверяем, хочет ли модель вызвать функцию
if len(msg.ToolCalls) > 0 {
    // Модель хочет вызвать инструмент!
    call := msg.ToolCalls[0]
    fmt.Printf("Function: %s\n", call.Function.Name)
    fmt.Printf("Arguments: %s\n", call.Function.Arguments)
    
    // Парсим аргументы
    var args struct {
        IP string `json:"ip"`
    }
    json.Unmarshal([]byte(call.Function.Arguments), &args)
    
    // Вызываем реальную функцию
    result := runGetServerStatus(args.IP)
    fmt.Printf("Result: %s\n", result)
} else {
    // Модель ответила текстом
    fmt.Println("Text response:", msg.Content)
}
```

## Типовые ошибки

### Ошибка 1: Модель не вызывает функцию

**Симптом:** `len(msg.ToolCalls) == 0`, модель отвечает текстом.

**Причины:**
1. Модель не обучена на Function Calling
2. Плохое описание инструмента (`Description` неясное)
3. Temperature > 0 (слишком случайно)

**Решение:**
1. Проверьте модель через Lab 00
2. Улучшите `Description`: сделайте его конкретным и понятным
3. Установите `Temperature = 0`

### Ошибка 2: Сломанный JSON в аргументах

**Симптом:** `json.Unmarshal` возвращает ошибку.

**Пример:**
```json
{"ip": "192.168.1.10"  // Пропущена закрывающая скобка
```

**Решение:**
```go
// Валидация JSON перед парсингом
if !json.Valid([]byte(call.Function.Arguments)) {
    return "Error: Invalid JSON", nil
}
```

### Ошибка 3: Неправильное имя функции

**Симптом:** Модель вызывает функцию с другим именем.

**Пример:**
```json
{"name": "check_server"}  // Но функция называется "get_server_status"
```

**Решение:**
```go
// Валидация имени функции
allowedFunctions := map[string]bool{
    "get_server_status": true,
}
if !allowedFunctions[call.Function.Name] {
    return "Error: Unknown function", nil
}
```

## Мини-упражнения

### Упражнение 1: Добавьте второй инструмент

Создайте инструмент `ping_host(host string)` и проверьте, что модель правильно выбирает между двумя инструментами.

### Упражнение 2: Улучшите Description

Попробуйте разные описания и посмотрите, как это влияет на выбор модели:

```go
// Вариант 1: Короткое
Description: "Ping a host"

// Вариант 2: Детальное
Description: "Ping a host to check network connectivity. Returns latency in milliseconds."
```

## Критерии сдачи

✅ **Сдано:**
- Модель успешно вызывает функцию
- Аргументы парсятся корректно
- Результат функции обрабатывается

❌ **Не сдано:**
- Модель не вызывает функцию (только текст)
- JSON аргументов сломан
- Код не компилируется

---

**Следующий шаг:** После успешного прохождения Lab 02 переходите к [Lab 03: Architecture](../lab03-real-world/README.md)

