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

## Что дальше?

После изучения автономности переходите к:
- **[06. Безопасность и Human-in-the-Loop](../06-safety-and-hitl/README.md)** — как защитить агента от опасных действий

---

**Навигация:** [← Инструменты](../04-tools-and-function-calling/README.md) | [Оглавление](../README.md) | [Безопасность →](../06-safety-and-hitl/README.md)

