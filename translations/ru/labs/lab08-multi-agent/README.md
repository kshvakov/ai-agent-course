# Lab 08: Команда Агентов (Multi-Agent)

## Цель
Создать систему из нескольких специализированных агентов, управляемую главным агентом (Supervisor). Реализовать паттерн Supervisor/Worker с изоляцией контекста.

## Теория

### Проблема: Один агент "мастер на все руки"

Один агент с множеством инструментов часто путается. Контекст переполняется, агент может перепутать инструменты или принять неправильное решение.

**Решение:** Разделить ответственность между специализированными агентами.

### Паттерн Supervisor (Начальник-Подчиненный)

**Архитектура:**

- **Supervisor:** Главный мозг. Не имеет инструментов для работы с инфраструктурой, но знает, кто что умеет.
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

## Задание

В `main.go` реализуйте Multi-Agent систему.

### Часть 1: Определение инструментов для Supervisor

Supervisor имеет инструменты для вызова специалистов:

```go
supervisorTools := []openai.Tool{
    {
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name: "ask_network_expert",
            Description: "Ask the network specialist about connectivity, pings, ports.",
            // ...
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

### Часть 2: Определение инструментов для Workers

```go
netTools := []openai.Tool{
    {Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: "ping", Description: "Ping a host"}},
}

dbTools := []openai.Tool{
    {Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{Name: "run_sql", Description: "Run SQL query"}},
}
```

### Часть 3: Функция запуска Worker-а

Реализуйте функцию `runWorkerAgent`, которая:
- Создает **новый** контекст диалога для работника (изоляция!)
- Запускает простой цикл агента (1-2 шага обычно)
- Возвращает финальный ответ работника

### Часть 4: Цикл Supervisor-а

Реализуйте цикл Supervisor-а, который:
- Получает задачу от пользователя
- Решает, какому специалисту делегировать
- Вызывает соответствующего Worker-а
- Получает ответ и добавляет его в свою историю
- Собирает результаты и отвечает пользователю

### Сценарий тестирования

Запустите систему с промптом: *"Проверь, доступен ли сервер БД db-host.example.com, и если да — узнай версию PostgreSQL"*

**Ожидание:**
- Supervisor делегирует Network Specialist → проверяет доступность
- Supervisor делегирует DB Specialist → узнает версию
- Supervisor собирает результаты и отвечает пользователю

## Важно

- **Изоляция контекста:** Worker не должен видеть контекст Supervisor-а
- **Возврат результатов:** Ответы Workers должны быть добавлены в историю Supervisor-а (role: "tool")
- **Лимиты:** Установите лимит итераций для Workers (обычно 3-5)

## Критерии сдачи

✅ **Сдано:**
- Supervisor делегирует задачи Workers
- Workers работают изолированно (не видят контекст Supervisor-а)
- Ответы Workers возвращаются Supervisor-у
- Supervisor собирает результаты и отвечает пользователю
- Код компилируется и работает

❌ **Не сдано:**
- Supervisor не делегирует задачи
- Workers видят контекст Supervisor-а
- Ответы Workers не возвращаются
- Система зацикливается

---

**Следующий шаг:** После завершения Lab 08 переходите к [Lab 09: Context Optimization](../lab09-context-optimization/README.md) — оптимизация контекстного окна LLM.
