# 15. Кейсы из Реальной Практики

## Зачем это нужно?

В этой главе мы рассмотрим примеры агентов в разных доменах с детальным разбором их работы.

Теория — это хорошо, но реальные примеры помогают понять, как применять концепции на практике. Здесь вы увидите, как все изученные концепции (SOP, CoT, Tools, Safety, RAG) работают вместе в реальных агентах.

### Реальный кейс

**Ситуация:** Вы хотите создать агента для DevOps, но не знаете, с чего начать.

**Проблема:** Вы знаете теорию, но не понимаете, как собрать всё вместе: какой System Prompt использовать, какие инструменты нужны, как организовать цикл работы.

**Решение:** Изучение реальных кейсов показывает паттерны и лучшие практики. Вы видите, как другие создали успешных агентов, и можете применить эти паттерны к своей задаче.

## DevOps Agent

**Задача:** "У нас проблемы с базой, разберись"

### Системный промпт

```text
Ты Senior DevOps Engineer с 10-летним опытом.

Твоя цель — восстановить работоспособность сервисов максимально быстро.

Ограничения:
- Никогда не используй команды типа `rm -rf /`
- Всегда спрашивай подтверждение перед удалением данных
- Если не уверен в действии — спроси у пользователя

SOP для диагностики проблем:
1. Проверь метрики (CPU, Memory, Connections)
2. Прочитай логи (последние 50 строк)
3. Проанализируй ошибки
4. Выдвини гипотезу
5. Примени фикс (если безопасно) или запроси подтверждение
6. Верифицируй результат
```

### Доступные инструменты

```go
tools := []openai.Tool{
    {
        Function: &openai.FunctionDefinition{
            Name:        "check_db_metrics",
            Description: "Check database metrics: CPU, memory, connections. Use this FIRST when investigating database issues.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "read_db_logs",
            Description: "Read the last N lines of database logs. Use this after checking metrics to find errors.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "restart_db_service",
            Description: "CRITICAL: Restart database service. Requires confirmation. Use this only if logs show it's safe.",
        },
    },
}
```

### Пример выполнения

**Итерация 1:**
- User: "У нас проблемы с базой, разберись"
- Agent: `check_db_metrics()` → CPU: 95%, Connections: 1000/100

**Итерация 2:**
- Agent: `read_db_logs(lines=50)` → "ERROR: Too many connections"

**Итерация 3:**
- Agent: Анализирует проблему и предлагает решение: "Обнаружено превышение лимита соединений. Рекомендую перезапустить сервис или увеличить лимит."

**Итерация 4:**
- Agent: Запрашивает подтверждение для `restart_db_service()`
- User: "yes"
- Agent: `restart_db_service()` → Success

**Итерация 5:**
- Agent: `check_db_metrics()` → CPU: 30%, Connections: 50/100
- Agent: "Проблема решена. База работает нормально."

## Customer Support Agent

**Задача:** "Пользователь жалуется на медленную загрузку"

### Системный промпт

```text
Ты Customer Support Agent уровня Tier 2.

Твоя цель — решить проблему пользователя быстро и вежливо.

Ограничения:
- Всегда будь вежлив
- Если проблема сложная — эскалируй
- Не давай технических деталей, если пользователь не технарь

SOP для обработки тикета:
1. Прочитай тикет полностью (get_ticket)
2. Собери контекст (версия ПО, ОС, браузер)
3. Поищи в базе знаний (search_kb)
4. Если решение найдено — сформулируй ответ
5. Если нет — эскалируй (escalate_ticket)
```

### Доступные инструменты

```go
tools := []openai.Tool{
    {
        Function: &openai.FunctionDefinition{
            Name:        "get_ticket",
            Description: "Get ticket details including user info, description, and history. Use this FIRST.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "search_kb",
            Description: "Search knowledge base for solutions to common problems. Use this after reading ticket.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "draft_reply",
            Description: "Draft a reply message to the ticket. Use this when you have a solution.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "escalate_ticket",
            Description: "Escalate ticket to Tier 3 or engineering. Use this for complex technical issues.",
        },
    },
}
```

### Пример выполнения

**Итерация 1:**
- User: "Пользователь жалуется на медленную загрузку"
- Agent: `get_ticket(ticket_id="TICKET-12345")` → "User reports slow page load. Browser: Chrome, OS: Windows"

**Итерация 2:**
- Agent: `search_kb(query="slow page load chrome")` → "Solution: Clear browser cache, check network connection"

**Итерация 3:**
- Agent: `draft_reply(ticket_id="TICKET-12345", message="Попробуйте очистить кеш браузера...")` → Success

**Итерация 4:**
- Agent: "Ответ отправлен пользователю. Рекомендовано очистить кеш браузера."

## Data Analytics Agent

**Задача:** "Почему упали продажи в регионе X?"

### Системный промпт

```text
Ты Data Analyst с опытом работы с SQL и BI-инструментами.

Твоя цель — предоставить точные данные и аналитику.

Ограничения:
- Используй ТОЛЬКО read-only SQL (SELECT)
- Всегда проверяй качество данных перед анализом
- Если данные некорректны — сообщи об этом

SOP для анализа:
1. Понять вопрос пользователя
2. Проверить схему таблицы (describe_table)
3. Сформулировать SQL-запрос
4. Выполнить запрос (sql_select)
5. Проанализировать результаты
6. Сгенерировать отчет
```

### Доступные инструменты

```go
tools := []openai.Tool{
    {
        Function: &openai.FunctionDefinition{
            Name:        "describe_table",
            Description: "Get table schema including column names, types, and constraints. Use this FIRST before writing SQL queries.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "sql_select",
            Description: "Execute a SELECT query on the database. ONLY SELECT queries allowed.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "check_data_quality",
            Description: "Check for data quality issues: nulls, duplicates, outliers. Use this before analysis.",
        },
    },
}
```

### Пример выполнения

**Итерация 1:**
- User: "Почему упали продажи в регионе X?"
- Agent: `describe_table(table_name="sales")` → "Columns: date, region, amount, product"

**Итерация 2:**
- Agent: `check_data_quality(table_name="sales")` → "No issues found"

**Итерация 3:**
- Agent: `sql_select(query="SELECT date, SUM(amount) FROM sales WHERE region='X' GROUP BY date ORDER BY date DESC LIMIT 30")` → Results: [data...]

**Итерация 4:**
- Agent: `sql_select(query="SELECT date, SUM(amount) FROM sales WHERE region='X' AND date >= NOW() - INTERVAL '60 days' GROUP BY date")` → Comparison data

**Итерация 5:**
- Agent: Анализирует данные и генерирует отчет: "Продажи в регионе X упали на 30% за последний месяц. Основная причина: снижение продаж продукта Y."

## Security Agent (SOC)

**Задача:** "Алерт: подозрительная активность на хосте 192.168.1.10"

### Системный промпт

```text
Ты Security Analyst в SOC (Security Operations Center).

Твоя цель — быстро и точно триажировать алерты безопасности.

Ограничения:
- Критические действия (изоляция хоста) требуют подтверждения
- Всегда собирай доказательства перед действиями
- Генерируй отчеты для аудита

SOP для триажа алерта:
1. Определи severity (Low/Medium/High/Critical)
2. Собери доказательства (логи, метрики, трафик)
3. Проанализируй паттерны атаки
4. Определи scope (затронутые системы)
5. Прими решение (False Positive / True Positive)
6. Если True Positive — containment (с подтверждением!)
7. Сгенерируй отчет для SOC
```

### Доступные инструменты

```go
tools := []openai.Tool{
    {
        Function: &openai.FunctionDefinition{
            Name:        "query_siem",
            Description: "Query security information and event management system for logs and alerts.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "check_ip_reputation",
            Description: "Check if an IP address is known malicious or suspicious.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "isolate_host",
            Description: "CRITICAL: Isolate a host from the network. Requires explicit confirmation.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "generate_report",
            Description: "Generate a security incident report for SOC and audit.",
        },
    },
}
```

### Пример выполнения

**Итерация 1:**
- User: "Алерт: подозрительная активность на хосте 192.168.1.10"
- Agent: `query_siem(query="host=192.168.1.10 AND time>now-1h")` → "Multiple failed login attempts, unusual network traffic"

**Итерация 2:**
- Agent: `check_ip_reputation(ip="192.168.1.10")` → "Internal IP, no external reputation data"

**Итерация 3:**
- Agent: Анализирует данные и определяет: "High severity. Pattern: Brute-force attack. Scope: Single host."

**Итерация 4:**
- Agent: Запрашивает подтверждение для `isolate_host(host="192.168.1.10")`
- User: "yes"
- Agent: `isolate_host(host="192.168.1.10")` → Success

**Итерация 5:**
- Agent: `generate_report(incident_id="INC-12345", details="...")` → Report generated

**Итерация 6:**
- Agent: "Хост изолирован. Отчет сгенерирован. Инцидент: INC-12345."

## Product Operations Agent

**Задача:** "Подготовь план релиза фичи X"

### Системный промпт

```text
Ты Product Operations Manager.

Твоя цель — подготовить полный план релиза фичи.

Ограничения:
- Всегда проверяй зависимости перед релизом
- Учитывай риски и откат-план
- Согласовывай с заинтересованными сторонами

SOP для подготовки релиза:
1. Собери требования и спецификацию
2. Проверь зависимости (другие фичи, сервисы)
3. Определи риски и откат-план
4. Создай документы (release notes, runbook)
5. Отправь на согласование
```

### Доступные инструменты

```go
tools := []openai.Tool{
    {
        Function: &openai.FunctionDefinition{
            Name:        "get_feature_spec",
            Description: "Get feature specification and requirements.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "check_dependencies",
            Description: "Check dependencies: other features, services, infrastructure.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "create_release_plan",
            Description: "Create release plan document with timeline, risks, rollback plan.",
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name:        "send_for_approval",
            Description: "Send release plan for approval to stakeholders.",
        },
    },
}
```

### Пример выполнения

**Итерация 1:**
- User: "Подготовь план релиза фичи X"
- Agent: `get_feature_spec(feature_id="X")` → Specification details

**Итерация 2:**
- Agent: `check_dependencies(feature_id="X")` → "Depends on feature Y, requires service Z update"

**Итерация 3:**
- Agent: Анализирует зависимости и риски

**Итерация 4:**
- Agent: `create_release_plan(feature_id="X", timeline="...", risks="...", rollback="...")` → Plan created

**Итерация 5:**
- Agent: `send_for_approval(plan_id="PLAN-12345", stakeholders=["...])` → Sent

**Итерация 6:**
- Agent: "План релиза создан и отправлен на согласование. ID: PLAN-12345."

## Общие паттерны

Во всех кейсах видны общие паттерны:

1. **SOP задает процесс** — четкий алгоритм действий
2. **CoT помогает следовать процессу** — модель думает по шагам
3. **Tools дают доступ к реальным данным** — grounding через инструменты
4. **Safety checks** — подтверждение для критических действий
5. **Верификация** — проверка результата после действий

## Мини-упражнения

### Упражнение 1: Создайте System Prompt для вашего домена

Выберите домен (DevOps, Support, Data, Security, Product) и создайте System Prompt по образцу из кейсов:

```go
systemPrompt := `
// Ваш код здесь
// Включите: Role, Goal, Constraints, SOP
`
```

**Ожидаемый результат:**
- System Prompt содержит все необходимые компоненты
- SOP четко описывает процесс действий
- Constraints явно указаны

### Упражнение 2: Определите инструменты для вашего агента

Определите набор инструментов для вашего агента:

```go
tools := []openai.Tool{
    // Ваш код здесь
    // Включите: основные инструменты, инструменты безопасности
}
```

**Ожидаемый результат:**
- Инструменты покрывают основные задачи домена
- Описания инструментов четкие и понятные
- Критические инструменты требуют подтверждения

## Критерии сдачи / Чек-лист

✅ **Сдано:**
- Понимаете общие паттерны создания агентов
- Можете создать System Prompt для своего домена
- Можете определить набор инструментов для агента
- Понимаете, как применять SOP, CoT, Safety checks

❌ **Не сдано:**
- Не понимаете, как собрать все компоненты вместе
- System Prompt не содержит всех необходимых компонентов
- Инструменты не покрывают основные задачи

## Связь с другими главами

- **Промптинг:** Как создавать System Prompt, см. [Главу 02: Промптинг](../02-prompt-engineering/README.md)
- **Инструменты:** Как определять инструменты, см. [Главу 03: Инструменты](../03-tools-and-function-calling/README.md)
- **Безопасность:** Как реализовать safety checks, см. [Главу 05: Безопасность](../05-safety-and-hitl/README.md)

## Что дальше?

После изучения кейсов переходите к:
- **[16. Best Practices и Области Применения](../16-best-practices/README.md)** — лучшие практики создания и поддержки агентов

---

**Навигация:** [← Экосистема и Фреймворки](../14-ecosystem-and-frameworks/README.md) | [Оглавление](../README.md) | [Best Practices →](../16-best-practices/README.md)

