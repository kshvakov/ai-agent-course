# Проектирование Автономных AI Агентов

Для программистов, желающих строить промышленные AI-агенты

---

## Оглавление

### Часть I: Основы

- **[00. Предисловие](./00-preface/README.md)** — Как читать руководство, требования, что такое агент
- **[01. Физика LLM](./01-llm-fundamentals/README.md)** — Токены, контекст, температура и её ограничения, вероятностная природа
- **[02. Промптинг как Программирование](./02-prompt-engineering/README.md)** — ICL, Few-Shot, CoT, структурирование задач, SOP

### Часть II: Practice-first (собрать агента)

- **[03. Инструменты и Function Calling](./03-tools-and-function-calling/README.md)** — JSON Schema, валидация, обработка ошибок, контракт tool↔runtime
- **[04. Автономность и Циклы](./04-autonomy-and-loops/README.md)** — ReAct loop, остановка, анти-циклы, наблюдаемость
- **[05. Безопасность и Human-in-the-Loop](./05-safety-and-hitl/README.md)** — Confirmation, Clarification, категории инструментов, Prompt Injection
- **[06. RAG и База Знаний](./06-rag/README.md)** — Чанкинг, Retrieval, Grounding, режимы поиска, лимиты
- **[07. Multi-Agent Systems](./07-multi-agent/README.md)** — Supervisor/Worker, изоляция контекста, маршрутизация задач
- **[08. Evals и Надежность](./08-evals-and-reliability/README.md)** — Evals, регрессии промптов, метрики качества, тестовые датасеты

### Часть III: Архитектура и Runtime Core

- **[09. Анатомия Агента](./09-agent-architecture/README.md)** — Memory, Tools, Planning, Runtime
- **[10. Planning и Workflow-паттерны](./10-planning-and-workflows/README.md)** — Plan→Execute, Plan-and-Revise, декомпозиция задач, DAG/workflow, условия остановки
- **[11. State Management](./11-state-management/README.md)** — Идемпотентность инструментов, retries с экспоненциальным backoff, дедлайны, persist state, возобновление задач
- **[12. Системы Памяти Агента](./12-agent-memory/README.md)** — Два горизонта памяти (рабочая в Run vs долгосрочная между сессиями), линейная история как иммутабельный лог, prompt cache и почему system prompt стабилен, compact vs condense vs recall, опциональная блочная память
- **[13. Context Engineering](./13-context-engineering/README.md)** — Стабильный системный префикс, единый порог и одна реакция (condense с лимитом 1/Run), `usage.PromptTokens` как первичный источник, ошибки оверинжиниринга (LayeredContext, скоринг сообщений, динамический system prompt)
- **[14. Экосистема и Фреймворки](./14-ecosystem-and-frameworks/README.md)** — Выбор между собственным runtime и фреймворками, portability, избежание vendor lock-in

### Часть IV: Практика (кейсы/практики)

- **[15. Кейсы из Реальной Практики](./15-case-studies/README.md)** — Примеры агентов в разных доменах (DevOps, Support, Data, Security, Product)
- **[16. Best Practices и Области Применения](./16-best-practices/README.md)** — Лучшие практики создания и поддержки агентов, области применения

### Часть V: Инфраструктура/безопасность платформы

- **[17. Security и Governance](./17-security-and-governance/README.md)** — Threat modeling, категоризация инструментов, защита от prompt injection (канон), sandboxing инструментов, allowlists, policy-as-code, RBAC, dry-run режимы, аудит
- **[18. Протоколы Инструментов и Tool Servers](./18-tool-protocols-and-servers/README.md)** — Контракт tool↔runtime на уровне процесса/сервиса, версионирование схем, authn/authz

### Часть VI: Прод-готовность

- **[19. Observability и Tracing](./19-observability-and-tracing/README.md)** — Структурированное логирование, трейсинг agent runs и tool calls, метрики, кореляция логов
- **[20. Cost & Latency Engineering](./20-cost-latency-engineering/README.md)** — Бюджеты токенов, лимиты итераций, кэширование, fallback-модели, таймауты
- **[21. Workflow и State Management в продакшене](./21-workflow-state-management/README.md)** — Очереди и асинхронность, масштабирование, распределённое состояние
- **[22. Prompt и Program Management](./22-prompt-program-management/README.md)** — Версионирование промптов, промпт-регрессии через evals, конфиги и feature flags, A/B тестинг
- **[23. Evals в CI/CD](./23-evals-in-cicd/README.md)** — Quality gates в CI/CD, версионирование датасетов, обработка flaky-кейсов, тесты на безопасность
- **[24. Data и Privacy](./24-data-and-privacy/README.md)** — Обнаружение и маскирование PII, защита секретов, redaction логов, хранение и TTL логов
- **[25. Индекс Прод-готовности](./25-production-readiness-index/README.md)** — Руководство по приоритизации (1 день / 1–2 недели) и быстрые ссылки на прод-темы

### Приложения

- **[Приложение: Справочники](./appendix/README.md)** — Глоссарий, чек-листы, шаблоны SOP, таблицы решений, Capability Benchmark

---

## Маршрут чтения

### Для начинающих (рекомендуемый путь — practice-first)

1. **Начните с [Предисловия](./00-preface/README.md)** — что такое агент, уравнение `Brain + Tools + Memory + Planning`, и **обязательно** раздел [«Ментальная модель: агент — это новый сотрудник»](./00-preface/README.md#ментальная-модель-агент--это-новый-сотрудник). Без неё security-главы будут читаться как «специальная LLM-машинерия» вместо «здравый смысл, который вы уже знаете».
2. **Изучите [Физику LLM](./01-llm-fundamentals/README.md)** — фундамент для понимания всего остального
3. **Освойте [Промптинг](./02-prompt-engineering/README.md)** — это основа работы с агентами
4. **Соберите работающего агента:**
    - [Инструменты и Function Calling](./03-tools-and-function-calling/README.md) — "руки" агента
    - [Автономность и Циклы](./04-autonomy-and-loops/README.md) — как агент работает в цикле
    - [Безопасность и Human-in-the-Loop](./05-safety-and-hitl/README.md) — защита от опасных действий
5. **Расширьте возможности:**
    - [RAG и База Знаний](./06-rag/README.md) — использование документации
    - [Multi-Agent Systems](./07-multi-agent/README.md) — команда специализированных агентов
    - [Evals и Надежность](./08-evals-and-reliability/README.md) — тестирование агентов
6. **Углубитесь в архитектуру:**
    - [Анатомия Агента](./09-agent-architecture/README.md) — компоненты и их взаимодействие
    - [Planning и Workflow-паттерны](./10-planning-and-workflows/README.md) — планирование сложных задач
    - [State Management](./11-state-management/README.md) — надёжность выполнения
    - [Системы Памяти Агента](./12-agent-memory/README.md) — линейная память в Run, долгосрочная между сессиями, prompt cache
    - [Context Engineering](./13-context-engineering/README.md) — стабильный префикс, condense при переполнении, отказ от оверинжиниринга
7. **Практикуйтесь:** Проходите лабораторные работы параллельно с чтением глав

### Для опытных программистов

Можете пропустить базовые главы и сразу перейти к:
- [Предисловие → Ментальная модель: агент — это новый сотрудник](./00-preface/README.md#ментальная-модель-агент--это-новый-сотрудник) — 5 минут, объясняет всю security-часть курса одной идеей
- [Инструменты и Function Calling](./03-tools-and-function-calling/README.md)
- [Автономность и Циклы](./04-autonomy-and-loops/README.md)
- [Системы Памяти Агента](./12-agent-memory/README.md) и [Context Engineering](./13-context-engineering/README.md) — память и контекст без оверинжиниринга
- [Кейсы](./15-case-studies/README.md) — для понимания реальных применений

### Быстрый трек: Основные концепции за 10 минут

Если вы опытный разработчик и хотите быстро понять суть:

1. **Что такое агент?**
    - Агент = LLM + Tools + Memory + Planning
    - LLM — это "мозг", который принимает решения
    - Tools — это "руки", которые выполняют действия
    - Memory — это история и долговременное хранилище
    - Planning — это способность разбить задачу на шаги

2. **Ментальная модель (важнее, чем вам сейчас кажется):**
    - Агент — это **новый сотрудник на испытательном сроке**, не «новый софт».
    - Доступы по роли, апрув для опасного, audit log, постепенное расширение доверия — то же самое, что для человека.
    - Четыре асимметрии (где модель «как с человеком» ломается): скорость в 1000× быстрее, параллелизм, отсутствие чувства последствий, prompt injection как социальная инженерия.
    - Подробно: [Предисловие → Ментальная модель](./00-preface/README.md#ментальная-модель-агент--это-новый-сотрудник).

3. **Как работает цикл агента?**
    ```
    While (задача не решена):
      1. Отправить историю в LLM
      2. Получить ответ (текст или tool_call)
      3. Если tool_call → выполнить инструмент → добавить результат в историю → повторить
      4. Если текст → показать пользователю и остановиться
    ```

4. **Ключевые моменты:**
    - LLM не выполняет код. Она генерирует JSON с запросом на выполнение.
    - Runtime (ваш код) выполняет реальные функции Go.
    - LLM не "помнит" прошлое. Она видит его в `messages[]`, который собирает Runtime.
    - Для агентных циклов типична низкая `Temperature` (0.0–0.3); воспроизводимость дают schema, валидация и гарды в коде, а не «детерминизм» температуры.
    - История — иммутабельный лог, system prompt стабилен между итерациями (иначе теряется prompt cache, см. [гл. 12](./12-agent-memory/README.md), [гл. 13](./13-context-engineering/README.md)).
    - Число токенов берите из `usage.PromptTokens` ответа провайдера, не из своих счётчиков.

5. **Минимальный пример:**
    ```go
    // 1. Определяем инструменты (схема — JSON Schema параметров)
    tools := []ToolDef{{
        Name:        "check_status",
        Description: "Проверить статус сервера по имени хоста",
        Parameters:  json.RawMessage(`{"type":"object","properties":{"host":{"type":"string"}},"required":["host"]}`),
    }}

    // 2. Запрос к модели (один HTTP-вызов к любому провайдеру с tool calling)
    resp, err := client.Complete(ctx, Request{
        Model: "glm-5.3",
        Messages: []Message{
            {Role: "system", Content: "Ты DevOps инженер"},
            {Role: "user", Content: "Проверь статус сервера web-01"},
        },
        Tools: tools,
    })
    if err != nil {
        return err // ошибки клиента обрабатывает код, не модель (гл. 04)
    }

    // 3. Проверяем tool_call
    if len(resp.Choices[0].Message.ToolCalls) > 0 {
        // 4. Выполняем инструмент (Runtime)
        result := checkStatus(resp.Choices[0].Message.ToolCalls[0].Arguments)
        // 5. Добавляем результат в историю (со ссылкой на ID вызова)
        messages = append(messages,
            resp.Choices[0].Message,
            Message{Role: "tool", ToolCallID: resp.Choices[0].Message.ToolCalls[0].ID, Content: result},
        )
        // 6. Отправляем обновленную историю обратно в модель
    }
    ```

6. **Что читать дальше:**
    - [Глава 03: Инструменты](./03-tools-and-function-calling/README.md) — детальный протокол
    - [Глава 04: Автономность](./04-autonomy-and-loops/README.md) — цикл агента
    - [Глава 09: Анатомия Агента](./09-agent-architecture/README.md) — архитектура
    - [Глава 12: Память Агента](./12-agent-memory/README.md) и [Глава 13: Context Engineering](./13-context-engineering/README.md) — без оверинжиниринга

### После завершения основного курса

После изучения глав 1-16 переходите к:
- **[Часть V: Инфраструктура/безопасность платформы](./17-security-and-governance/README.md)** — security, governance, протоколы инструментов
- **[Часть VI: Прод-готовность](./25-production-readiness-index/README.md)** — практическое руководство по прод-готовности с пошаговыми рецептами внедрения

---

## Связь с лабораторными работами

| Глава руководства | Соответствующие лабораторные работы |
|----------------|-------------------------------------|
| [01. Физика LLM](./01-llm-fundamentals/README.md) | [Lab 00](https://github.com/kshvakov/ai-agent-course/tree/main/translations/ru/labs/lab00-capability-check) (Capability Check) |
| [02. Промптинг](./02-prompt-engineering/README.md) | [Lab 01](https://github.com/kshvakov/ai-agent-course/tree/main/translations/ru/labs/lab01-basics) (Basics) |
| [03. Инструменты](./03-tools-and-function-calling/README.md) | [Lab 02](https://github.com/kshvakov/ai-agent-course/tree/main/translations/ru/labs/lab02-tools) (Tools), [Lab 03](https://github.com/kshvakov/ai-agent-course/tree/main/translations/ru/labs/lab03-real-world) (Architecture) |
| [04. Автономность](./04-autonomy-and-loops/README.md) | [Lab 04](https://github.com/kshvakov/ai-agent-course/tree/main/translations/ru/labs/lab04-autonomy) (Autonomy) |
| [05. Безопасность](./05-safety-and-hitl/README.md) | [Lab 05](https://github.com/kshvakov/ai-agent-course/tree/main/translations/ru/labs/lab05-human-interaction) (Human-in-the-Loop) |
| [02. Промптинг (SOP)](./02-prompt-engineering/README.md) | [Lab 06](https://github.com/kshvakov/ai-agent-course/tree/main/translations/ru/labs/lab06-incident) (Incident) |
| [06. RAG](./06-rag/README.md) | [Lab 07](https://github.com/kshvakov/ai-agent-course/tree/main/translations/ru/labs/lab07-rag) (RAG) |
| [07. Multi-Agent](./07-multi-agent/README.md) | [Lab 08](https://github.com/kshvakov/ai-agent-course/tree/main/translations/ru/labs/lab08-multi-agent) (Multi-Agent) |
| [09. Анатомия Агента](./09-agent-architecture/README.md) | [Lab 01](https://github.com/kshvakov/ai-agent-course/tree/main/translations/ru/labs/lab01-basics) (Basics), [Lab 09](https://github.com/kshvakov/ai-agent-course/tree/main/translations/ru/labs/lab09-context-optimization) (Context Optimization) |
| [10. Planning и Workflow-паттерны](./10-planning-and-workflows/README.md) | [Lab 10](https://github.com/kshvakov/ai-agent-course/tree/main/translations/ru/labs/lab10-planning-workflows) (Planning & Workflow) |
| [11. State Management](./11-state-management/README.md) | [Lab 10](https://github.com/kshvakov/ai-agent-course/tree/main/translations/ru/labs/lab10-planning-workflows) (Planning & Workflow) — частично |
| [12. Системы Памяти Агента](./12-agent-memory/README.md), [13. Context Engineering](./13-context-engineering/README.md) | [Lab 11](https://github.com/kshvakov/ai-agent-course/tree/main/translations/ru/labs/lab11-memory-context) (Memory & Context Engineering) |
| [18. Протоколы Инструментов и Tool Servers](./18-tool-protocols-and-servers/README.md) | [Lab 12](https://github.com/kshvakov/ai-agent-course/tree/main/translations/ru/labs/lab12-tool-server) (Tool Server Protocol), [Lab 13](https://github.com/kshvakov/ai-agent-course/tree/main/translations/ru/labs/lab13-tool-retrieval) (Tool Retrieval & Pipelines) — Опционально |
| [17. Security и Governance](./17-security-and-governance/README.md) | — (читается как теория-капстоун; практика безопасности встроена в Lab 02 / Lab 05 / Lab 12) |
| [19–21, 23–25. Прод-часть](./25-production-readiness-index/README.md) | — (лабы для прод-тем планируются; теория самодостаточна) |
| [22. Prompt и Program Management](./22-prompt-program-management/README.md) | [Lab 01](https://github.com/kshvakov/ai-agent-course/tree/main/translations/ru/labs/lab01-basics) (Basics) — частично |

> **Примечание:** код лабораторных работ пока не ревизован — он остаётся на старом клиенте и старых моделях и будет обновлён отдельным этапом. Примеры в главах ревизованы и надёжнее; лабы выполняйте, сверяясь с каноном глав.

---

## Как пользоваться руководством

1. **Читайте последовательно** — каждая глава опирается на предыдущие
2. **Практикуйтесь параллельно** — после каждой главы выполняйте соответствующую лабораторную работу
3. **Используйте как справочник** — возвращайтесь к нужным разделам при работе над проектами
4. **Изучайте примеры** — в каждой главе есть примеры из разных доменов (DevOps, Support, Data, Security, Product)
5. **Выполняйте упражнения** — мини-упражнения в каждой главе помогают закрепить материал
6. **Проверяйте себя** — используйте чек-листы для самопроверки

---

**Удачного обучения.**

