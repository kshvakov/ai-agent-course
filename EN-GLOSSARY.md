# EN Translation Glossary — канон терминов

Единый источник терминов для перевода `translations/ru/book` → `book/` (EN).
Все переводчики обязаны следовать этому файлу. При конфликте — глоссарий выигрывает.

## Ядро (употреблять всегда одинаково)

| RU | EN | Примечание |
|---|---|---|
| ход (агента) | run | сущ.; «за ход» = per run |
| итерация цикла | iteration | |
| автономный агент | autonomous agent | |
| инструмент | tool | |
| вызов инструмента | tool call | |
| вызов (как акт) | call | `call.Name`, «три вызова» = three calls |
| разрешение / allowlist | allowlist | не «white list» |
| подтверждение (человеком) | confirmation | |
| человек в контуре | human-in-the-loop (HITL) | |
| подтвердил бот | confirmed by a bot | в коде — `confirmed_by` (не переводить) |
| категория инструмента | tool category | |
| читающий / пишущий / деструктивный | read-only / mutating / destructive | три категории, всегда в этом порядке |
| dry-run | dry run (текст) / dry-run (прилагательное) | |
| цикл агента | agent loop | |
| цикл ReAct | ReAct loop | |
| прогретый кэш | warm cache | |
| кэш провайдера / промпт-кэш | provider cache / prompt cache | |
| кэш-чтение | cache read | в коде — `CacheReadTokens` |
| окно (контекста) | context window | |
| переполнение окна | context overflow | |
| сжатие истории / конденсация | condense | глагол: to condense; не переводить как compression (compression — про gzip) |
| порог | threshold | «~0.85 окна» = ~0.85 of the window |
| суммаризация | summarization | |
| tail / голова истории | tail / head | head = старые сообщения, tail = последние |
| остаток истории | tail | |
| досрочно завершить ход | wrap up | сущ. — `wrapUp` в коде |
| терминальная ошибка | terminal error | не fatal |
| граница хода | run boundary | «лимиты/дедлайн/бюджет — границы хода» = run boundaries |
| бюджет токенов | token budget | |
| лимит итераций | iteration limit | |
| дедлайн хода | run deadline | |
| субагент | subagent | одно слово |
| пишущий инструмент | mutating tool | |
| защита от инъекций | prompt injection defense | сам термин: prompt injection |
| утечка промпта | prompt leaking / leakage | |
| маскирование | redaction | |
| токены | tokens | |
| оценка (предварительная) | estimate | |
| детерминизм | determinism | только в опровержениях мифа |
| эталон / образец | ground truth | |
| врать / выдумывать | hallucinate / hallucination | |
| зацикливание | loop / stuck loop | детектор — loop detector |
| признак зацикливания | loop signature | |
| очередь | queue | |
| состояние хода | run state | FSM-состояния: queued / working / waiting / paused / asleep / done / failed / cancelled (уже EN, не менять) |
| чекпоинт | checkpoint | |
| трейс | trace | |
| дашборд | dashboard | |
| качественный порог (CI) | quality gate | |
| прогон (eval) | eval run | |

## Код-артефакты (не переводить никогда)

Все идентификаторы, имена файлов, JSON-ключи, имена событий, типы, поля:
`RunResult`, `AgentConfig`, `ToolDef`, `ToolCall`, `Message`, `Client`, `Complete`,
`Stream`, `Decision`, `confirm_action`, `ask_user`, `check_status`, `restart_service`,
`search_memory`, `read_note`, `CondenseErrorEvent`, `ToolEndEvent`, `ErrLoopTerminal`,
`setConfirmedBy`, `executeBatch`, `collectMetrics`, `ModelConfig`, и т.п.

Комментарии внутри код-блоков **переводить** (на EN), если они несут смысл для читателя.
Префиксы «ПЛОХО/ХОРОШО» в комментариях → `// BAD:` / `// GOOD:`.

## Устойчивые формулы

| RU | EN |
|---|---|
| «свобода внутри хода, границы снаружи» | "freedom inside the run, boundaries on the outside" |
| «ошибка — тоже результат» | "an error is a result too" |
| «модель видит только то, что в контексте» | "the model sees only what is in its context" |
| «никакой магии в промпте — механика в коде» | "no magic in the prompt — the mechanics live in code" |
| «never destroy originals» | never destroy originals |
| «лаба пока не ревизована…» | > **Note:** this lab is not yet revised (still on the old stack) — use it as a pointer; the canonical material is in this chapter. |
| «Как это устроено в реальном SDK» | **How the real SDK does it.** |
| «Мини-упражнения» | Exercises |
| «Зачем это нужно?» | Why does this matter? |
| «Ошибка N: …» (нумерованный разбор) | Mistake N: … |
| «Теория простыми словами» | The theory in plain words |
| «Как это работает (пошагово)» | How it works (step by step) |
| «Типовые ошибки» | Common Mistakes |
| «Что дальше?» | What's Next? |
| «Критерии сдачи / Чек-лист» | Acceptance Criteria / Checklist |
| «Связь с другими главами» | Relation to Other Chapters |
| «Для любопытных» | For the curious |
| «Мини-пример кода» | Mini code example |
| «Где это встраивать в нашем коде» | Where to plug this into our code |
| «Прод-заметки» | Production Notes |
| «Ключевые моменты» | Key takeaways |
| «Анти-паттерн» | Anti-pattern |

## Стиль

- Техническая книга, обращение на «вы» → "you", без канцелярита.
- Не калькировать русский синтаксис; перестраивать предложения по-английски.
- Заголовки: H1 — Title Case; содержательные H2 — Title Case; повторяющиеся
  шаблонные H2 (см. таблицу выше: «Теория простыми словами», «Типовые ошибки»,
  «Что дальше?», «Связь с другими главами» и т.д.) — строго в фиксированной
  форме из таблицы, единообразно во всех главах; H3+ — Sentence case.
- Zero-shot / Few-shot / Post-query / Vendor lock-in / Over-engineering — не
  акронимы: пишутся в sentence case (Zero-shot, Few-shot) во всех контекстах —
  заголовках, прозе, комментариях в коде — а не Zero-Shot/Few-Shot.
  Акронимы (ReAct, CoT, LM Studio, gRPC, SSE, SHA256, LivePlan) сохраняют
  канонический регистр.
- H1 каждой главы совпадает с меткой навигации в mkdocs.yml (EN-ветка).
- Списки и таблицы сохранять структуру 1:1.
- Ссылки между главами — относительные те же пути (`../04-autonomy-and-loops/README.md#anchor`),
  но **якорь пересчитывать** под английский заголовок того же раздела.
- Числа, величины, имена моделей, версии — без изменений.
