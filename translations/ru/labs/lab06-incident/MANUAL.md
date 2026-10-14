# Методическое пособие: Lab 06 — Incident Management (SOP)

## Зачем это нужно?

В этой лабораторной работе вы создадите агента уровня **SRE (Site Reliability Engineer)**. В отличие от простых задач, инциденты требуют **стратегического мышления** и следования строгому алгоритму (SOP).

### Реальный кейс

**Ситуация:** Сервис оплаты недоступен (502 Bad Gateway).

**Без SOP:**
- Агент: [Сразу рестартит сервис]
- Результат: Сервис не запускается (проблема в конфиге)
- Агент: [Рестартит снова]
- Результат: Та же ошибка
- Агент: [Зацикливается]

**С SOP:**
- Агент: Проверяет HTTP статус → 502
- Агент: Читает логи → Видит "Config syntax error"
- Агент: Понимает, что рестарт не поможет
- Агент: Делает rollback → Сервис восстанавливается

**Разница:** SOP заставляет агента следовать алгоритму, а не гадать.

## Теория простыми словами

### Planning (Планирование) — разбиение задачи на шаги

В этой лабе мы используем **явное планирование** (Plan-and-Solve), в отличие от имплицитного планирования (ReAct) из Lab 04.

**Разница:**

| Имплицитное (ReAct) | Явное (Plan-and-Solve) |
|---------------------|------------------------|
| Планирует "на лету" | Сначала создает план |
| Подходит для простых задач (2-4 шага) | Подходит для сложных задач (5+ шагов) |
| Гибкий, адаптируется к результатам | Структурированный, гарантирует выполнение всех шагов |

**Как работает явное планирование:**

1. **Генерация плана:** Агент получает задачу и создает полный план
   ```
   План:
   1. Проверить HTTP статус
   2. Прочитать логи
   3. Проанализировать ошибки
   4. Применить фикс
   5. Верифицировать
   ```

2. **Выполнение плана:** Агент выполняет шаги по очереди, отмечая прогресс

3. **Адаптация:** Если шаг не помог, агент может перепланировать

### Декомпозиция задачи

Задача "Разберись с инцидентом" разбивается на подзадачи:

**Принципы декомпозиции:**
- **Атомарность:** Каждый шаг выполним одним действием
  - ❌ Плохо: "Проверить и починить сервер"
  - ✅ Хорошо: "Проверить статус" → "Прочитать логи" → "Применить фикс"

- **Зависимости:** Шаги выполняются в правильном порядке
  - ❌ Плохо: "Применить фикс" → "Прочитать логи"
  - ✅ Хорошо: "Прочитать логи" → "Проанализировать" → "Применить фикс"

- **Проверяемость:** Каждый шаг имеет четкий критерий успеха
  - ❌ Плохо: "Улучшить производительность"
  - ✅ Хорошо: "Снизить CPU с 95% до 50%"

**Пример декомпозиции для инцидента:**
```
Исходная задача: "Сервис недоступен (502). Разберись."

Декомпозиция:
1. Проверить HTTP статус сервиса
   - Критерий успеха: Получен HTTP код (200/502/500)
   
2. Прочитать логи сервиса
   - Критерий успеха: Получены последние 20 строк логов
   
3. Проанализировать ошибки в логах
   - Критерий успеха: Определена причина (Syntax error / Connection error / Memory)
   
4. Применить фикс согласно анализу
   - Критерий успеха: Фикс применен (rollback/restart выполнен)
   
5. Верифицировать восстановление
   - Критерий успеха: HTTP статус = 200 OK
```

### SOP (Standard Operating Procedure)

**SOP** — это алгоритм действий, закодированный в промпте. Это как устав для солдата: четкие инструкции, что делать в каждой ситуации.

**Пример SOP для инцидента:**

```
SOP для падения сервиса:
1. Check Status: Проверь HTTP код ответа
2. Check Logs: Если 500/502 — читай последние 20 строк логов
3. Analyze: Найди ключевые слова:
   - "Syntax error" → Rollback
   - "Connection refused" → Check Database
   - "Out of memory" → Restart
4. Action: Примени фикс согласно анализу
5. Verify: Проверь HTTP статус снова
```

**Почему это работает?**

Без SOP модель видит: `User: Fix it`. Ее вероятностный механизм может выдать: `Call: restart_service`. Это самое "популярное" действие.

С SOP модель вынуждена сгенерировать текст:
- "Step 1: I need to check HTTP status." → Это повышает вероятность вызова `check_http`
- "HTTP is 502. Step 2: I need to check logs." → Это повышает вероятность вызова `read_logs`

Мы **направляем внимание** модели по нужному руслу.

### Chain-of-Thought в действии

Обратите внимание на System Prompt в решении:
`"Think step by step following this SOP: 1. Check HTTP... 2. Check Logs..."`

Зачем это нужно?

Без этого промпта модель видит: `User: Fix it`.  
Ее вероятностный механизм может выдать: `Call: restart_service`. Это самое "популярное" действие.

С этим промптом модель вынуждена сгенерировать текст:
- "Step 1: I need to check HTTP status." → Это повышает вероятность вызова `check_http`
- "HTTP is 502. Step 2: I need to check logs." → Это повышает вероятность вызова `read_logs`

Мы **направляем внимание** модели по нужному руслу.

## Алгоритм выполнения

### Шаг 1: Определение инструментов

```go
tools := []llm.ToolDef{
    {Name: "check_http", Description: "Check service HTTP status"},
    {Name: "read_logs", Description: "Read service logs. Do this if HTTP is 500/502."},
    {Name: "restart_service", Description: "Restart the service. Use ONLY if logs show transient error."},
    {Name: "rollback_deploy", Description: "Rollback to previous version. Use if logs show Config/Syntax error."},
}
```

**Важно:** В `Description` указывайте, когда использовать инструмент. Это помогает модели выбрать правильный.

### Шаг 2: SOP в System Prompt

```go
sopPrompt := `You are a Site Reliability Engineer (SRE).
Your goal is to fix the Payment Service.
Follow this Standard Operating Procedure (SOP) strictly:
1. Check HTTP status first.
2. If status is not 200, READ LOGS immediately. Do not guess.
3. Analyze logs:
   - If "Syntax Error" or "Config Error" -> ROLLBACK.
   - If "Connection Error" -> RESTART.
4. Verify fix by checking HTTP status again.

ALWAYS Think step by step. Output your thought process before calling a tool.`
```

### Шаг 3: Цикл агента с логированием

```go
for i := 0; i < 15; i++ {
    resp, err := client.Complete(ctx, req)
    msg := resp.Choices[0].Message
    messages = append(messages, msg)
    
    // Логируем мысли агента
    if msg.Content != "" {
        fmt.Printf("🧠 Thought: %s\n", msg.Content)
    }
    
    if len(msg.ToolCalls) == 0 {
        fmt.Printf("🤖 Agent: %s\n", msg.Content)
        break
    }
    
    for _, toolCall := range msg.ToolCalls {
        fmt.Printf("🔧 Call: %s\n", toolCall.Name)
        result := executeTool(toolCall)
        fmt.Printf("📦 Result: %s\n", result)
        
        messages = append(messages, toolResult)
    }
}
```

## Типовые ошибки

### Ошибка 1: Агент не следует SOP

**Симптом:** Агент сразу рестартит без чтения логов.

**Причина:** SOP недостаточно строгий или модель его игнорирует.

**Решение:**
1. Усильте SOP: "CRITICAL: Never restart without reading logs first"
2. Добавьте Few-Shot примеры в промпт
3. Используйте модель помощнее (задайте более мощную модель через `LLM_MODEL`)

### Ошибка 2: Агент зацикливается на одном шаге

**Симптом:** Агент повторяет `check_http` несколько раз подряд.

**Причина:** Нет явного указания переходить к следующему шагу.

**Решение:**
```go
// В SOP:
"After checking HTTP status, move to step 2. Do not repeat step 1."
```

### Ошибка 3: Неправильный выбор действия

**Симптом:** Агент делает rollback вместо restart (или наоборот).

**Причина:** Описания инструментов неясные.

**Решение:**
```go
// Улучшите Description:
Description: "Rollback to previous version. Use ONLY if logs show 'Syntax error' or 'Config error'."
```

## Мини-упражнения

### Упражнение 1: Добавьте таблицу решений

Создайте таблицу "симптом → гипотеза → проверка → действие":

```go
decisionTable := map[string]string{
    "502 + Syntax error": "rollback",
    "502 + Connection refused": "check_database",
    "502 + Out of memory": "restart",
}
```

### Упражнение 2: Добавьте верификацию

После каждого действия проверяйте, помогло ли оно:

```go
// После rollback:
verifyResult := checkHttp()
if verifyResult == "200 OK" {
    fmt.Println("✅ Incident resolved!")
} else {
    fmt.Println("⚠️ Rollback didn't help. Trying next step...")
}
```

## Критерии сдачи

✅ **Сдано:**
- Агент следует SOP строго
- Агент читает логи перед действием
- Агент выбирает правильное действие (rollback vs restart)
- Агент верифицирует результат

❌ **Не сдано:**
- Агент не следует SOP
- Агент не читает логи
- Агент выбирает неправильное действие
- Агент не верифицирует результат

---

**Следующий шаг:** После успешного прохождения Lab 06 переходите к [Lab 07: RAG](../lab07-rag/README.md)

