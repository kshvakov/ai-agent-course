# Методическое пособие: Lab 04 — The Agent Loop (Autonomy)

## Зачем это нужно?

В этой лабораторной работе вы реализуете паттерн **ReAct (Reason + Act)** — сердце автономного агента. Агент научится самостоятельно принимать решения, выполнять действия и анализировать результаты в цикле, без вмешательства человека.

### Реальный кейс

**Ситуация:** Пользователь пишет: "У меня кончилось место на сервере. Разберись."

**Без автономного цикла:**
- Агент: "Я проверю использование диска" → вызывает `check_disk` → получает "95%"
- Агент: [Останавливается, ждет следующей команды пользователя]

**С автономным циклом:**
- Агент: "Я проверю использование диска" → вызывает `check_disk` → получает "95%"
- Агент: "Диск переполнен. Очищу логи" → вызывает `clean_logs` → получает "Освобождено 20GB"
- Агент: "Проверю снова" → вызывает `check_disk` → получает "40%"
- Агент: "Готово! Освободил 20GB."

**Разница:** Агент сам решает, что делать дальше, основываясь на результатах предыдущих действий.

## Теория простыми словами

### ReAct Loop (Цикл Автономности)

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

**Ключевой момент:** Пункт 4.c дает "магию" — агент сам смотрит на результат и решает, что делать дальше.

### Замыкание круга

После выполнения инструмента мы **не спрашиваем пользователя**, что делать дальше. Мы отправляем результат обратно в LLM. Модель видит результат своих действий и решает, что делать дальше.

**Пример диалога внутри памяти:**

1. **User:** "Кончилось место."
2. **Assistant (ToolCall):** `check_disk()`
3. **Tool (Result):** "95% usage."
4. **Assistant (ToolCall):** `clean_logs()` ← Агент сам решил это сделать!
5. **Tool (Result):** "Freed 20GB."
6. **Assistant (Text):** "Я почистил логи, теперь места достаточно."

## Алгоритм выполнения

### Шаг 1: Определение инструментов

```go
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
```

### Шаг 2: Начальная история

```go
messages := []openai.ChatCompletionMessage{
    {
        Role:    openai.ChatMessageRoleSystem,
        Content: "You are an autonomous DevOps agent.",
    },
    {
        Role:    openai.ChatMessageRoleUser,
        Content: "У меня кончилось место. Разберись.",
    },
}
```

### Шаг 3: Цикл агента

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
}
```

## Типовые ошибки

### Ошибка 1: Зацикливание

**Симптом:** Агент повторяет одно и то же действие бесконечно.

**Пример:**
```
Action: check_disk()
Result: "95%"
Action: check_disk()  // Снова!
Result: "95%"
Action: check_disk()  // И снова!
```

**Решение:**
1. Добавьте лимит итераций
2. Детектируйте повторяющиеся действия
3. Улучшите промпт: "Если действие не помогло, попробуй другой подход"

### Ошибка 2: Результат инструмента не добавляется в историю

**Симптом:** Агент не видит результат инструмента и продолжает выполнять то же действие.

**Причина:** Вы забыли добавить результат в `messages`.

**Решение:**
```go
// ОБЯЗАТЕЛЬНО добавляйте результат!
messages = append(messages, openai.ChatCompletionMessage{
    Role:       openai.ChatMessageRoleTool,
    Content:    result,
    ToolCallID: toolCall.ID,  // Важно для связи!
})
```

### Ошибка 3: Агент не останавливается

**Симптом:** Агент продолжает вызывать инструменты, даже когда задача решена.

**Решение:**
```go
// Добавьте в System Prompt:
"Если задача решена, ответь текстом пользователю. Не вызывай инструменты без необходимости."
```

## Мини-упражнения

### Упражнение 1: Добавьте детекцию зацикливания

Реализуйте проверку, что последние 3 действия одинаковые:

```go
func isStuck(history []ChatCompletionMessage) bool {
    if len(history) < 3 {
        return false
    }
    lastActions := getLastActions(history, 3)
    return allEqual(lastActions)
}
```

### Упражнение 2: Добавьте логирование

Логируйте каждую итерацию цикла:

```go
fmt.Printf("[Iteration %d] Agent decided: %s\n", i, action)
fmt.Printf("[Iteration %d] Tool result: %s\n", i, result)
```

## Критерии сдачи

✅ **Сдано:**
- Агент выполняет цикл автономно
- Результаты инструментов добавляются в историю
- Агент останавливается, когда задача решена
- Есть защита от зацикливания

❌ **Не сдано:**
- Агент не продолжает цикл после выполнения инструмента
- Результаты инструментов не видны агенту
- Агент зацикливается

---

**Следующий шаг:** После успешного прохождения Lab 04 переходите к [Lab 05: Human-in-the-Loop](../lab05-human-interaction/README.md)

