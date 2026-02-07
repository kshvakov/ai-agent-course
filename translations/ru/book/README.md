# Проектирование Автономных AI Агентов

Для программистов, желающих строить промышленные AI-агенты

## Переводы

- **Русский (RU)** — `ru` (эта ветка)
- **English (EN)** — [English version](../../book/README.md)

---

## Оглавление

### Часть I: Основы

- **[00. Предисловие](./00-preface/README.md)** — Как читать руководство, требования, что такое агент
- **[01. Физика LLM](./01-llm-fundamentals/README.md)** — Токены, контекст, температура, детерминизм, вероятностная природа
- **[02. Промптинг как Программирование](./02-prompt-engineering/README.md)** — ICL, Few-Shot, CoT, структурирование задач, SOP

### Часть II: Practice-first (собрать агента)

- **[03. Инструменты и Function Calling](./03-tools-and-function-calling/README.md)** — JSON Schema, валидация, обработка ошибок, контракт tool↔runtime
- **[04. Автономность и Циклы](./04-autonomy-and-loops/README.md)** — ReAct loop, остановка, анти-циклы, наблюдаемость
- **[05. Безопасность и Human-in-the-Loop](./05-safety-and-hitl/README.md)** — Confirmation, Clarification, Risk Scoring, Prompt Injection
- **[06. RAG и База Знаний](./06-rag/README.md)** — Чанкинг, Retrieval, Grounding, режимы поиска, лимиты
- **[07. Multi-Agent Systems](./07-multi-agent/README.md)** — Supervisor/Worker, изоляция контекста, маршрутизация задач
- **[08. Evals и Надежность](./08-evals-and-reliability/README.md)** — Evals, регрессии промптов, метрики качества, тестовые датасеты

### Часть III: Архитектура и Runtime Core

- **[09. Анатомия Агента](./09-agent-architecture/README.md)** — Memory, Tools, Planning, Runtime
- **[10. Planning и Workflow-паттерны](./10-planning-and-workflows/README.md)** — Plan→Execute, Plan-and-Revise, декомпозиция задач, DAG/workflow, условия остановки
- **[11. State Management](./11-state-management/README.md)** — Идемпотентность инструментов, retries с экспоненциальным backoff, дедлайны, persist state, возобновление задач
- **[12. Системы Памяти Агента](./12-agent-memory/README.md)** — Кратковременная/долговременная память, episodic/semantic память, забывание/TTL, верификация памяти, storage/retrieval
- **[13. Context Engineering](./13-context-engineering/README.md)** — Слои контекста, политики отбора фактов, саммаризация, бюджеты токенов, сборка контекста из state+memory+retrieval
- **[14. Экосистема и Фреймворки](./14-ecosystem-and-frameworks/README.md)** — Выбор между собственным runtime и фреймворками, portability, избежание vendor lock-in

### Часть IV: Практика (кейсы/практики)

- **[15. Кейсы из Реальной Практики](./15-case-studies/README.md)** — Примеры агентов в разных доменах (DevOps, Support, Data, Security, Product)
- **[16. Best Practices и Области Применения](./16-best-practices/README.md)** — Лучшие практики создания и поддержки агентов, области применения

### Часть V: Инфраструктура/безопасность платформы

- **[17. Security и Governance](./17-security-and-governance/README.md)** — Threat modeling, risk scoring, защита от prompt injection (канон), sandboxing инструментов, allowlists, policy-as-code, RBAC, dry-run режимы, аудит
- **[18. Протоколы Инструментов и Tool Servers](./18-tool-protocols-and-servers/README.md)** — Контракт tool↔runtime на уровне процесса/сервиса, версионирование схем, authn/authz

### Часть VI: Прод-готовность

- **[19. Observability и Tracing](./19-observability-and-tracing/README.md)** — Структурированное логирование, трейсинг agent runs и tool calls, метрики, кореляция логов
- **[20. Cost & Latency Engineering](./20-cost-latency-engineering/README.md)** — Бюджеты токенов, лимиты итераций, кэширование, fallback-модели, батчинг, таймауты
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

1. **Начните с [Предисловия](./00-preface/README.md)** — узнайте, что такое агент и как работать с руководством
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
   - [Системы Памяти Агента](./12-agent-memory/README.md) — долговременная память
   - [Context Engineering](./13-context-engineering/README.md) — управление контекстом
7. **Практикуйтесь:** Проходите лабораторные работы параллельно с чтением глав

### Для опытных программистов

Можете пропустить базовые главы и сразу перейти к:
- [Инструменты и Function Calling](./03-tools-and-function-calling/README.md)
- [Автономность и Циклы](./04-autonomy-and-loops/README.md)
- [Кейсы](./15-case-studies/README.md) — для понимания реальных применений

### Быстрый трек: Основные концепции за 10 минут

Если вы опытный разработчик и хотите быстро понять суть:

1. **Что такое агент?**
   - Агент = LLM + Tools + Memory + Planning
   - LLM — это "мозг", который принимает решения
   - Tools — это "руки", которые выполняют действия
   - Memory — это история и долговременное хранилище
   - Planning — это способность разбить задачу на шаги

2. **Как работает цикл агента?**
   ```
   While (задача не решена):
     1. Отправить историю в LLM
     2. Получить ответ (текст или tool_call)
     3. Если tool_call → выполнить инструмент → добавить результат в историю → повторить
     4. Если текст → показать пользователю и остановиться
   ```

3. **Ключевые моменты:**
   - LLM не выполняет код. Она генерирует JSON с запросом на выполнение.
   - Runtime (ваш код) выполняет реальные функции Go.
   - LLM не "помнит" прошлое. Она видит его в `messages[]`, который собирает Runtime.
   - Temperature = 0 для детерминированного поведения агентов.

4. **Минимальный пример:**
   ```go
   // 1. Определяем инструмент
   tools := []openai.Tool{{
       Function: &openai.FunctionDefinition{
           Name: "check_status",
           Description: "Check server status",
       },
   }}
   
   // 2. Запрос к модели
   resp, _ := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
       Model: openai.GPT3Dot5Turbo,
       Messages: []openai.ChatCompletionMessage{
           {Role: "system", Content: "Ты DevOps инженер"},
           {Role: "user", Content: "Проверь статус сервера"},
       },
       Tools: tools,
   })
   
   // 3. Проверяем tool_call
   if len(resp.Choices[0].Message.ToolCalls) > 0 {
       // 4. Выполняем инструмент (Runtime)
       result := checkStatus()
       // 5. Добавляем результат в историю
       messages = append(messages, openai.ChatCompletionMessage{
           Role: "tool",
           Content: result,
       })
       // 6. Отправляем обновленную историю обратно в модель
   }
   ```

5. **Что читать дальше:**
   - [Глава 03: Инструменты](./03-tools-and-function-calling/README.md) — детальный протокол
   - [Глава 04: Автономность](./04-autonomy-and-loops/README.md) — цикл агента
   - [Глава 09: Анатомия Агента](./09-agent-architecture/README.md) — архитектура

### После завершения основного курса

После изучения глав 1-16 переходите к:
- **[Часть V: Инфраструктура/безопасность платформы](./17-security-and-governance/README.md)** — security, governance, протоколы инструментов
- **[Часть VI: Прод-готовность](./25-production-readiness-index/README.md)** — практическое руководство по прод-готовности с пошаговыми рецептами внедрения

---

## Связь с лабораторными работами

| Глава руководства | Соответствующие лабораторные работы |
|----------------|-------------------------------------|
| [01. Физика LLM](./01-llm-fundamentals/README.md) | Lab 00 (Capability Check) |
| [02. Промптинг](./02-prompt-engineering/README.md) | Lab 01 (Basics) |
| [03. Инструменты](./03-tools-and-function-calling/README.md) | Lab 02 (Tools), Lab 03 (Architecture) |
| [04. Автономность](./04-autonomy-and-loops/README.md) | Lab 04 (Autonomy) |
| [05. Безопасность](./05-safety-and-hitl/README.md) | Lab 05 (Human-in-the-Loop) |
| [02. Промптинг (SOP)](./02-prompt-engineering/README.md) | Lab 06 (Incident) |
| [06. RAG](./06-rag/README.md) | Lab 07 (RAG) |
| [07. Multi-Agent](./07-multi-agent/README.md) | Lab 08 (Multi-Agent) |
| [09. Анатомия Агента](./09-agent-architecture/README.md) | Lab 01 (Basics), Lab 09 (Context Optimization) |
| [10. Planning и Workflow-паттерны](./10-planning-and-workflows/README.md) | Lab 10 (Planning & Workflow) |
| [11. State Management](./11-state-management/README.md) | Lab 10 (Planning & Workflow) — частично |
| [12. Системы Памяти Агента](./12-agent-memory/README.md), [13. Context Engineering](./13-context-engineering/README.md) | Lab 11 (Memory & Context Engineering) |
| [18. Протоколы Инструментов и Tool Servers](./18-tool-protocols-and-servers/README.md) | Lab 12 (Tool Server Protocol) |
| [17. Security и Governance](./17-security-and-governance/README.md) | Lab 13 (Agent Security Hardening) — Опционально |
| [22. Prompt и Program Management](./22-prompt-program-management/README.md) | Lab 01 (Basics) — частично |
| [23. Evals в CI/CD](./23-evals-in-cicd/README.md) | Lab 14 (Evals in CI) — Опционально |

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

