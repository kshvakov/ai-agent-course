# Методическое пособие: Lab 01 — Hello, LLM!

## Зачем это нужно?

В этой лабораторной работе вы научитесь основам взаимодействия с LLM: отправке запросов, получению ответов и, самое главное, **управлению контекстом**. Без сохранения контекста (истории сообщений) невозможно построить диалог.

### Реальный кейс

**Ситуация:** Вы создали чат-бота для поддержки клиентов. Пользователь пишет:
- "У меня проблема с входом"
- Бот отвечает: "Опишите проблему подробнее"
- Пользователь: "Я забыл пароль"
- Бот: "Опишите проблему подробнее" (снова!)

**Проблема:** Бот не помнит предыдущие сообщения.

**Решение:** Передавать всю историю диалога в каждый запрос.

## Теория простыми словами

### LLM — это Stateless система

**Stateless** означает "без состояния". Каждый запрос для модели — это новый запрос. Она не помнит, что вы писали секунду назад.

Чтобы создать иллюзию диалога, мы каждый раз отправляем **весь** список предыдущих сообщений (историю).

### Структура сообщения

Сообщение состоит из:
- **Role (Роль):** `system`, `user`, `assistant`
- **Content (Содержимое):** Текст сообщения

**Пример:**

```go
messages := []ChatCompletionMessage{
    {Role: "system", Content: "Ты опытный Linux администратор"},
    {Role: "user", Content: "Как проверить статус сервиса?"},
    {Role: "assistant", Content: "Используйте команду systemctl status nginx"},
    {Role: "user", Content: "А как его перезапустить?"},
}
```

Модель видит всю историю и понимает контекст ("его" = nginx).

## Алгоритм выполнения

### Шаг 1: Инициализация клиента

```go
config := openai.DefaultConfig(token)
if baseURL != "" {
    config.BaseURL = baseURL  // Для локальных моделей
}
client := openai.NewClientWithConfig(config)
```

### Шаг 2: Создание истории

```go
messages := []openai.ChatCompletionMessage{
    {
        Role:    openai.ChatMessageRoleSystem,
        Content: "Ты опытный Linux администратор. Отвечай кратко и по делу.",
    },
}
```

**Важно:** System Prompt задает роль агента. Это влияет на стиль ответов.

### Шаг 3: Цикл чата

```go
for {
    // 1. Читаем ввод пользователя
    input := readUserInput()
    
    // 2. Добавляем в историю
    messages = append(messages, openai.ChatCompletionMessage{
        Role:    openai.ChatMessageRoleUser,
        Content: input,
    })
    
    // 3. Отправляем ВСЮ историю в API
    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model:    openai.GPT3Dot5Turbo,
        Messages: messages,  // Вся история!
    })
    
    // 4. Получаем ответ
    answer := resp.Choices[0].Message.Content
    
    // 5. Сохраняем ответ в историю
    messages = append(messages, openai.ChatCompletionMessage{
        Role:    openai.ChatMessageRoleAssistant,
        Content: answer,
    })
}
```

## Типовые ошибки

### Ошибка 1: История не сохраняется

**Симптом:** Агент не помнит предыдущие сообщения.

**Причина:** Вы не добавляете ответ ассистента в историю.

**Решение:**
```go
// ПЛОХО
messages = append(messages, userMessage)
resp := client.CreateChatCompletion(...)
answer := resp.Choices[0].Message.Content
// История не обновлена!

// ХОРОШО
messages = append(messages, userMessage)
resp := client.CreateChatCompletion(...)
messages = append(messages, resp.Choices[0].Message)  // Сохраняем ответ!
```

### Ошибка 2: System Prompt не работает

**Симптом:** Агент отвечает не в нужном стиле.

**Причина:** System Prompt не добавлен или добавлен не в начало.

**Решение:**
```go
// System Prompt должен быть ПЕРВЫМ сообщением
messages := []openai.ChatCompletionMessage{
    {Role: "system", Content: "Ты DevOps инженер"},  // Первое!
    {Role: "user", Content: "..."},
}
```

### Ошибка 3: Контекст переполняется

**Симптом:** После N сообщений агент "забывает" начало разговора.

**Причина:** История слишком длинная, не влезает в контекстное окно.

**Решение:**
```go
// Обрезка истории (оставляем только последние N сообщений)
if len(messages) > maxHistoryLength {
    // Оставляем System Prompt + последние N-1 сообщений
    messages = append(
        []openai.ChatCompletionMessage{messages[0]},  // System
        messages[len(messages)-maxHistoryLength+1:]...,  // Последние
    )
}
```

## Мини-упражнения

### Упражнение 1: Измените роль

Попробуйте разные System Prompts:
- "Ты вежливый помощник"
- "Ты строгий учитель"
- "Ты дружелюбный коллега"

Наблюдайте, как меняется стиль ответов.

### Упражнение 2: Добавьте счетчик токенов

Подсчитайте, сколько токенов используется в истории:

```go
import "github.com/sashabaranov/go-openai"

// Примерная оценка (1 токен ≈ 4 символа)
tokenCount := 0
for _, msg := range messages {
    tokenCount += len(msg.Content) / 4
}
fmt.Printf("Tokens used: %d\n", tokenCount)
```

## Критерии сдачи

✅ **Сдано:**
- Агент помнит предыдущие сообщения
- System Prompt влияет на стиль ответов
- Код компилируется и работает

❌ **Не сдано:**
- Агент не помнит контекст
- System Prompt игнорируется
- Код не компилируется

---

**Следующий шаг:** После успешного прохождения Lab 01 переходите к [Lab 02: Tools](../lab02-tools/README.md)

