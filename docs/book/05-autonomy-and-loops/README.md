# 05. Автономность и Циклы — ReAct Loop

В этой главе мы реализуем паттерн **ReAct (Reason + Act)** — сердце автономного агента.

## ReAct Loop (Цикл Автономности)

Автономный агент работает в цикле:

```
While (Задача не решена):
  1. Отправить историю в LLM
  2. Получить ответ
  3. ЕСЛИ это текст → Показать пользователю и ждать нового ввода
  4. ЕСЛИ это вызов инструмента →
       a. Выполнить инструмент
       b. Добавить результат в историю
       c. GOTO 1 (ничего не спрашивая у пользователя!)
```

**Ключевой момент:** Пункт 4.c дает "магию" — агент сам смотрит на результат и решает, что делать дальше. Но это не настоящая магия: модель видит результат инструмента в контексте (`messages[]`) и генерирует следующий шаг на основе этого контекста.

### Замыкание круга

После выполнения инструмента мы **не спрашиваем пользователя**, что делать дальше. Мы отправляем результат обратно в LLM. Модель видит результат своих действий и решает, что делать дальше.

**Пример диалога внутри памяти:**

### Магия vs Реальность: Как работает цикл

**❌ Магия (как обычно объясняют):**
> Агент сам решил вызвать `clean_logs()` после проверки диска

**✅ Реальность (как на самом деле):**

**Итерация 1: Первый запрос**

```go
// messages перед первой итерацией
messages := []openai.ChatCompletionMessage{
    {Role: "system", Content: "You are an autonomous DevOps agent."},
    {Role: "user", Content: "Кончилось место."},
}

// Отправляем в модель
resp1, _ := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
    Model:    openai.GPT3Dot5Turbo,
    Messages: messages,
    Tools:    tools,
})

msg1 := resp1.Choices[0].Message
// msg1.ToolCalls = [{ID: "call_1", Function: {Name: "check_disk_usage", Arguments: "{}"}}]

// Добавляем ответ ассистента в историю
messages = append(messages, msg1)
// Теперь messages содержит:
// [system, user, assistant(tool_call: check_disk_usage)]
```

**Итерация 2: Выполнение инструмента и возврат результата**

```go
// Выполняем инструмент
result1 := checkDiskUsage()  // "95% usage"

// Добавляем результат как сообщение с ролью "tool"
messages = append(messages, openai.ChatCompletionMessage{
    Role:       "tool",
    Content:    result1,  // "95% usage"
    ToolCallID: "call_1",
})
// Теперь messages содержит:
// [system, user, assistant(tool_call), tool("95% usage")]

// Отправляем ОБНОВЛЕННУЮ историю в модель снова
resp2, _ := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
    Model:    openai.GPT3Dot5Turbo,
    Messages: messages,  // Модель видит результат check_disk_usage!
    Tools:    tools,
})

msg2 := resp2.Choices[0].Message
// msg2.ToolCalls = [{ID: "call_2", Function: {Name: "clean_logs", Arguments: "{}"}}]

messages = append(messages, msg2)
// Теперь messages содержит:
// [system, user, assistant(tool_call_1), tool("95%"), assistant(tool_call_2)]
```

**Итерация 3: Второй инструмент**

```go
// Выполняем второй инструмент
result2 := cleanLogs()  // "Freed 20GB"

messages = append(messages, openai.ChatCompletionMessage{
    Role:       "tool",
    Content:    result2,  // "Freed 20GB"
    ToolCallID: "call_2",
})
// Теперь messages содержит:
// [system, user, assistant(tool_call_1), tool("95%"), assistant(tool_call_2), tool("Freed 20GB")]

// Отправляем снова
resp3, _ := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
    Model:    openai.GPT3Dot5Turbo,
    Messages: messages,  // Модель видит оба результата!
    Tools:    tools,
})

msg3 := resp3.Choices[0].Message
// msg3.ToolCalls = []  // Пусто! Модель решила ответить текстом
// msg3.Content = "Я почистил логи, теперь места достаточно."

// Это финальный ответ - выходим из цикла
```

**Почему это не магия:**

1. **Модель видит всю историю** — она не "помнит" прошлое, она видит его в `messages[]`
2. **Модель видит результат инструмента** — результат добавляется как новое сообщение с ролью `tool`
3. **Модель решает на основе контекста** — видя "95% usage", модель понимает, что нужно освободить место
4. **Runtime управляет циклом** — код проверяет `len(msg.ToolCalls)` и решает, продолжать ли цикл

**Ключевой момент:** Модель не "сама решила" — она увидела результат `check_disk_usage` в контексте и сгенерировала следующий tool call на основе этого контекста.

## Реализация цикла

```go
for i := 0; i < maxIterations; i++ {
    // 1. Отправляем запрос
    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model:    openai.GPT3Dot5Turbo,
        Messages: messages,
        Tools:    tools,
    })
    
    msg := resp.Choices[0].Message
    messages = append(messages, msg)  // Сохраняем ответ
    
    // 2. Проверяем тип ответа
    if len(msg.ToolCalls) == 0 {
        // Это финальный текстовый ответ
        fmt.Println("Agent:", msg.Content)
        break
    }
    
    // 3. Выполняем инструменты
    for _, toolCall := range msg.ToolCalls {
        result := executeTool(toolCall.Function.Name, toolCall.Function.Arguments)
        
        // 4. Добавляем результат в историю
        messages = append(messages, openai.ChatCompletionMessage{
            Role:       openai.ChatMessageRoleTool,
            Content:    result,
            ToolCallID: toolCall.ID,
        })
    }
    // Цикл продолжается автоматически!
    // Но это не магия: мы отправляем обновленную историю (с результатом инструмента)
    // в модель снова, и модель видит результат и решает, что делать дальше
}
```

## Типовые проблемы

### Проблема 1: Зацикливание

**Симптом:** Агент повторяет одно и то же действие бесконечно.

**Решение:**
1. Добавьте лимит итераций
2. Детектируйте повторяющиеся действия
3. Улучшите промпт: "Если действие не помогло, попробуй другой подход"

### Проблема 2: Результат инструмента не добавляется в историю

**Симптом:** Агент не видит результат инструмента и продолжает выполнять то же действие.

**Решение:**
```go
// ОБЯЗАТЕЛЬНО добавляйте результат!
messages = append(messages, openai.ChatCompletionMessage{
    Role:       openai.ChatMessageRoleTool,
    Content:    result,
    ToolCallID: toolCall.ID,  // Важно для связи!
})
```

## Связь с другими главами

- **Инструменты:** Как инструменты вызываются и возвращают результаты, см. [Главу 04: Инструменты](../04-tools-and-function-calling/README.md)
- **Память:** Как история сообщений (`messages[]`) растет и управляется, см. [Главу 03: Анатомия Агента](../03-agent-architecture/README.md)
- **Безопасность:** Как остановить цикл для подтверждения, см. [Главу 06: Безопасность](../06-safety-and-hitl/README.md)

## Что дальше?

После изучения автономности переходите к:
- **[06. Безопасность и Human-in-the-Loop](../06-safety-and-hitl/README.md)** — как защитить агента от опасных действий

---

**Навигация:** [← Инструменты](../04-tools-and-function-calling/README.md) | [Оглавление](../README.md) | [Безопасность →](../06-safety-and-hitl/README.md)

