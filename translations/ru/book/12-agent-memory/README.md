# 12. Системы Памяти Агента

## Зачем это нужно?

Агенту нужна память, чтобы удерживать контекст между итерациями цикла, не забывать решения внутри одной задачи и не начинать каждый разговор с нуля. Без памяти агент тратит токены на повторные объяснения, теряет смысл задачи на 5-й итерации и не помнит, что было неделю назад.

Но память — это не только «куда положить данные». Любая работа с памятью затрагивает контекстное окно LLM, бюджет токенов и **prompt cache** провайдера. Неаккуратная архитектура памяти превращает дешёвый агент в дорогой и медленный.

В этой главе разберём, как устроена память в продакшен-агенте: какие горизонты бывают, что хранить иммутабельно, чем `compact` отличается от `condense`, и почему «динамический system prompt» — обычно дорогая ошибка.

### Реальный кейс

**Ситуация:** Пользователь спрашивает агента: «Какая была проблема с базой данных, которую мы исправили на прошлой неделе?» Агент отвечает: «У меня нет информации об этом».

**Проблема:** У агента нет памяти о прошлых разговорах. Каждое взаимодействие начинается с нуля.

**Решение:** Долговременная память сохраняет ключевые факты (решения, артефакты, предпочтения), даёт их извлечь при необходимости и забывает устаревшее, чтобы не вылезать за лимиты контекста.

## Теория простыми словами

### Два горизонта памяти

У агента два разных уровня памяти, и их нельзя путать:

**Внутри Run (между итерациями цикла):**
- История сообщений текущей задачи: `[]Message`.
- Растёт от итерации к итерации.
- Ограничена контекстным окном модели.
- При завершении Run может быть сохранена — или забыта.

**Между Run / сессиями (долговременная):**
- Решения, факты, предпочтения, артефакты.
- Хранится в БД/файлах.
- Не лезет в контекст LLM целиком — извлекается выборочно.

Большинство ошибок в дизайне памяти случается из-за того, что эти два уровня смешивают: пытаются хранить весь Run в долговременном хранилище, или наоборот — превращают долговременное хранилище в часть system prompt и тратят на него токены каждой итерации.

### Понятийная классификация (как принято в литературе)

**Кратковременная (working) память** — состояние текущей задачи: что агент уже сделал, какие файлы прочитал, какой план в работе.

**Долговременная память** — переживает завершение Run: факты, предпочтения, истории решений.

**Episodic** — конкретные события: «Пользователь спрашивал о месте на диске 2026-01-06».

**Semantic** — обобщения: «Пользователь предпочитает JSON ответы». Извлекается из эпизодов.

Эти термины полезны для общения, но в коде вам обычно достаточно различать «история текущего Run» и «персистентное хранилище».

### Операции с памятью

1. **Store** — сохранить информацию.
2. **Retrieve** — найти релевантную.
3. **Forget** — удалить устаревшую.
4. **Update** — изменить существующую.

### Принцип: память — иммутабельная история

Главное правило, на котором держится всё дальнейшее:

> **История уже отправленных сообщений не переписывается. Она только дополняется (append) или полностью заменяется (replace).**

Из этого правила вытекают три практических следствия:

1. **Не переписывай прошлое.** Никаких «удалим лишний tool result» или «уплотним assistant-ответ». Это ломает prompt cache на хвосте и иногда заставляет модель имитировать собственный прежний стиль.
2. **System prompt стабилен.** Динамические данные (что прочитали, какой план) не вписывай в system prompt — это вычислительный налог на каждой итерации.
3. **Сжатие — это полная замена.** Если истории слишком много, мы её осознанно, целиком и редко заменяем на summary + хвост (см. condense ниже). Никаких частичных перезаписей.

Эти правила выглядят строго, но именно они делают разницу между «агент работает быстро и предсказуемо» и «агент стоит дорого, тормозит и теряет контекст».

## Как это работает (пошагово)

### Шаг 1: Базовый интерфейс памяти

```go
type Memory interface {
    Store(key string, value any, metadata map[string]any) error
    Retrieve(query string, limit int) ([]MemoryItem, error)
    Forget(key string) error
    Update(key string, value any) error
}

type MemoryItem struct {
    Key      string
    Value    any
    Metadata map[string]any
    Created  time.Time
    Accessed time.Time
    TTL      time.Duration
}
```

### Шаг 2: Простое хранилище с TTL

```go
type SimpleMemory struct {
    store map[string]MemoryItem
    mu    sync.RWMutex
}

func (m *SimpleMemory) Store(key string, value any, metadata map[string]any) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    m.store[key] = MemoryItem{
        Key:      key,
        Value:    value,
        Metadata: metadata,
        Created:  time.Now(),
        Accessed: time.Now(),
        TTL:      24 * time.Hour,
    }
    return nil
}

func (m *SimpleMemory) Cleanup() error {
    m.mu.Lock()
    defer m.mu.Unlock()

    now := time.Now()
    for key, item := range m.store {
        if item.TTL > 0 && now.Sub(item.Created) > item.TTL {
            delete(m.store, key)
        }
    }
    return nil
}
```

### Шаг 3: Извлечение

```go
func (m *SimpleMemory) Retrieve(query string, limit int) ([]MemoryItem, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    results := make([]MemoryItem, 0, len(m.store))
    queryLower := strings.ToLower(query)

    for _, item := range m.store {
        if item.TTL > 0 && time.Since(item.Created) > item.TTL {
            continue
        }
        valueStr := fmt.Sprintf("%v", item.Value)
        if strings.Contains(strings.ToLower(valueStr), queryLower) {
            item.Accessed = time.Now()
            results = append(results, item)
        }
    }

    sort.Slice(results, func(i, j int) bool {
        return results[i].Accessed.After(results[j].Accessed)
    })

    if len(results) > limit {
        results = results[:limit]
    }
    return results, nil
}
```

В продакшене keyword-поиск заменяют на embeddings: модель кодирует query и каждый item в вектор, и retrieve возвращает top-K по косинусной близости. Это другой раздел и сильно зависит от выбранной vector db; для базового понимания достаточно того, что выше.

### Шаг 4: Интеграция с агентом

```go
func runAgentWithMemory(ctx context.Context, ep llm.Endpoint, mem Memory, userInput string) (string, error) {
    memories, _ := mem.Retrieve(userInput, 5)

    messages := []llm.Message{
        {Role: "system", Content: "Ты полезный ассистент."},
    }

    if len(memories) > 0 {
        var sb strings.Builder
        sb.WriteString("Релевантные факты из долговременной памяти:\n")
        for _, m := range memories {
            fmt.Fprintf(&sb, "- %s: %v\n", m.Key, m.Value)
        }
        messages = append(messages, llm.Message{
            Role:    "user",
            Content: sb.String(),
        })
    }

    messages = append(messages, llm.Message{Role: "user", Content: userInput})

    resp, err := ep.Chat(ctx, llm.Request{Messages: messages})
    if err != nil {
        return "", err
    }
    answer := resp.Content

    if shouldStore(userInput, answer) {
        _ = mem.Store(generateKey(userInput), answer, map[string]any{
            "user_input": userInput,
            "timestamp":  time.Now(),
        })
    }
    return answer, nil
}
```

Обратите внимание: факты из долговременной памяти приходят **в первом user-сообщении**, не в system prompt. Если вы в начале каждого Run меняете system prompt в зависимости от того, что нашлось в памяти, у вас будет cache miss при каждом запросе.

## Линейная память внутри Run

Дефолтная модель памяти на уровне одного Run — **линейная**: плоский `[]Message`, в который каждый шаг цикла дописывается новое.

### Минимальный код

```go
type LinearMemory struct {
    msgs []llm.Message
    mu   sync.Mutex
}

func (m *LinearMemory) Append(msgs ...llm.Message) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.msgs = append(m.msgs, msgs...)
}

func (m *LinearMemory) Snapshot() []llm.Message {
    m.mu.Lock()
    defer m.mu.Unlock()
    out := make([]llm.Message, len(m.msgs))
    copy(out, m.msgs)
    return out
}

// Reset — единственный способ изменить уже добавленное.
// Используется только condense (см. ниже) и тестами.
func (m *LinearMemory) Reset(msgs []llm.Message) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.msgs = append(m.msgs[:0], msgs...)
}
```

### Почему именно так

- **Prompt cache работает.** Современные провайдеры (OpenAI, Anthropic, Z.AI и совместимые) кэшируют префикс запроса. Стабильный префикс — почти бесплатные input-токены на повторных итерациях. Любая мутация хвоста сбрасывает кэш дальше точки изменения, мутация префикса — сбрасывает весь кэш.
- **Меньше движущихся частей.** Отдельная история = отдельная истина. Нет «оригинала» и «compact-копии», которые могут разъехаться.
- **Совместимость с провайдерами.** Все драйверы (OpenAI, Anthropic, Sber, vLLM) принимают один и тот же массив сообщений. Любые надстройки — recall, blocks, summaries — это уже на верхнем уровне.

### Live state без мутации system prompt

Соблазн положить динамическое состояние в system prompt очень большой:

```go
// ПЛОХО: меняется на каждой итерации → cache miss всего префикса
sysPrompt := basePrompt +
    "\n\nFiles read: " + filesRead.Join(", ") +
    "\nLast actions: " + lastActions.Join(", ") +
    "\nPlan: " + plan.Render()
```

Что не так: каждое изменение `filesRead` или `plan` инвалидирует кэш на всех ~N тысячах токенов system prompt. На длинной задаче это превращается в стабильный налог: вы платите за input-токены полную цену каждую итерацию, хотя 95% префикса не менялось.

Куда складывать live state:

1. **В результат последнего tool call.** Если tool вернул `read_file: {content}`, к нему естественно дописать заметки агента.
2. **В отдельные `Notes` структуры**, которые рендерятся в последнее `user`-сообщение или в первое сообщение очередной итерации.
3. **В tool, который агент сам зовёт**, чтобы поставить себе чекпоинт (`update_plan`, `set_goal`).

```go
// ХОРОШО: live state в результате последнего tool call
result := tool.Result{
    Content: actualOutput,
    Notes: []string{
        "plan: 2/5 done",
        "files touched: a.go, b.go",
    },
}
```

Стабильный system prompt + дописываемая история = максимум cache hits и минимум поверхности для багов.

Подробнее про сборку контекста, бюджет и динамические секции (когда они всё-таки нужны и как платить за них минимально) — в [Главе 13: Context Engineering](../13-context-engineering/README.md).

## Блочная память: когда она нужна

Линейная модель решает 80% задач. Оставшиеся 20% — это случаи, когда задачи внутри одного процесса агента естественно делятся на закрытые сюжеты, а пользователю/самому агенту полезно адресовать их по идентификатору.

Типичные кейсы:
- REPL-интерфейс: каждая команда пользователя — отдельный сюжет, и хочется на 10-й команде дать модели возможность сказать «глянь, что я делал в команде #3».
- Длительный коучинг-агент, где сессии явно разделены и между ними есть смысл подтягивать только summary.

Здесь полезна **блочная память**: историю мы группируем в блоки по «один пользовательский запрос → один цикл агента → завершение».

### Структура блока (без compact-через-теги)

```go
type Block struct {
    ID       int
    Query    string         // первые 120 символов первого user-сообщения
    Messages []llm.Message  // оригинал, иммутабельно
    Tokens   int            // Usage.PromptTokens из последнего ответа провайдера
    Summary  string         // 1-2 строки для каталога
}
```

Принцип:
- `Messages` — это правда. Никаких «compact-копий с урезанными tool args».
- `Summary` — короткая строка для каталога блоков, не заменитель содержимого. Делается на закрытии блока (один LLM-вызов на дешёвой модели либо вручную из заголовка задачи).
- `Tokens` берём из ответа провайдера, не считаем «по символам». Это важно — см. ошибку 7 ниже.

### Каталог и recall

Модель не видит содержимое прошлых блоков напрямую. В первое user-сообщение очередного блока подкладывается **каталог** — короткий список блоков с summary:

```
[CONTEXT BLOCKS]
#0: check disk usage — "Disk /data at 92%, cleaned logs" (~5K tokens)
#1: deploy service   — "Deployed v2.3.1 to staging" (~8K tokens)
#2: fix nginx config — "Updated proxy_pass for /api" (~3K tokens)
```

Если для текущей задачи нужны подробности — модель вызывает tool `recall(block_id)`:

```go
type RecallTool struct{ store *BlockStore }

func (t *RecallTool) Execute(ctx context.Context, args RecallArgs) (string, error) {
    msgs, ok := t.store.Block(args.BlockID)
    if !ok {
        return "Block not found", nil
    }
    return formatMessages(msgs), nil
}
```

Recall возвращает **оригинальные** `Messages` блока. Это критично: смысл всего механизма именно в том, что под рукой есть полные данные на случай, если summary недостаточно.

### Что НЕ делать

Исторически в блочную модель часто добавляли «compact-сообщения» — заменяли цепочку `assistant(tool_calls) → tool_result` одной строкой вида:

```
<prior_tool_use>
read_file path=a.go -- ok, 450 lines
exec git diff -- 123 lines, 5 files
</prior_tool_use>
```

На бумаге это даёт 80-90% сжатие. На практике — две устойчивые проблемы:

1. **Cache invalidation.** Compact меняет хвост истории → следующая итерация это full miss prompt cache на всём, что после точки compact.
2. **Имитация.** Если compact-сообщение положить с ролью `assistant`, модель часто начинает воспринимать чужие/свои прежние компакты как «свой стиль» и продолжает писать в этом же формате (включая теги). Если положить как `user` — повышается шум и снижается доверие модели к контексту.

Поэтому compact-через-теги в современных моделях скорее вредит. Если истории много — сжимайте через condense (LLM-summary, см. ниже), а не через структурное переписывание.

## Compact, Condense, Recall: что чем отличается

Терминология часто путается. Зафиксируем:

| Стратегия | Затраты | Cache impact | Когда применять |
|---|---|---|---|
| **Compact** (структурный) | CPU копейки | full miss + риск имитации | Почти никогда. Только если у вас закрыт блок и вы заранее уверены, что cache всё равно сбрасывается. |
| **Condense** (LLM) | один вызов LLM | full miss (полная замена истории) | По threshold (≈75-80% контекстного окна) или при `ContextOverflowError` от провайдера. Максимум 1 раз за Run. |
| **Recall** (model-driven) | один tool call | append, кэш не страдает | Только в блочной модели. Модель сама запрашивает старый блок, когда ей нужны детали. |

Принцип **never destroy originals** одинаково применим ко всем: condense создаёт новую историю, оригиналы (либо в виде блоков, либо в виде snapshot перед condense) остаются для recovery и аудита.

Подробности промпта condense, threshold-логика и инкрементальная суммаризация — в [Главе 13: Context Engineering](../13-context-engineering/README.md#condensation-prompt).

## Долговременная память (между сессиями)

Внутри Run работает линейная (или блочная) история. Между сессиями она теряется — нужно явное хранилище.

### Что класть

- **Решения и их обоснования.** «Выбрали PostgreSQL, потому что нужна транзакционность» — это вечно полезный факт.
- **Стабильные идентификаторы.** Имя пользователя, namespace, окружение, рабочая директория.
- **Установленные предпочтения** — с явным типом `preference` (см. anchoring bias в гл. 13).
- **Артефакты задач.** Ссылки на созданные ресурсы, миграции, ID PR-ов.

### Что НЕ класть

- **Каждый поворот разговора.** Это шум, retrieval начинает возвращать мусор.
- **Гипотезы и временный статус.** «Сервис сейчас падает» — устареет к следующей сессии и собьёт диагностику.
- **Полные транскрипты сессий.** Если очень нужно — храните отдельно, не в той же таблице, где факты для retrieval.

### Связь с лабой

Реализация: запись фактов в файл/БД, retrieval на embeddings, фильтрация по типу — отрабатывается в [Lab 11: Memory & Context Engineering](https://github.com/kshvakov/ai-agent-course/tree/main/translations/ru/labs/lab11-memory-context).

## Checkpoint и Resume

Длинный Run может упасть посередине: процесс убили, кончился rate limit, упала сеть. Чекпоинт сохраняет состояние, чтобы при перезапуске не начинать с нуля.

Базовая реализация (структура, save/load, интеграция с agent loop) описана в [Главе 09: Анатомия Агента](../09-agent-architecture/README.md#checkpoint-и-resume-сохранение-и-восстановление). Продвинутые стратегии (гранулярность, валидация, ротация) — в [Главе 11: State Management](../11-state-management/README.md#продвинутые-стратегии-checkpoint).

В контексте памяти важны три правила:

- **Сохраняй оригинал, не compact.** Если в snapshot записан compact-вариант истории, после resume вы не сможете восстановить детали.
- **Сохраняй после каждого значимого шага** (вызов инструмента, ответ пользователю). Для коротких задач (2-3 итерации) чекпоинт избыточен. Для длинных (10+ итераций) — обязателен.
- **Поставь TTL** (например, 24 часа), чтобы старые snapshot'ы не накапливались.

### Shared Memory между агентами

В мульти-агентных системах агенты обмениваются информацией через общее хранилище, разграничивая данные по namespace:

```go
type SharedMemoryStore struct {
    store CheckpointStore
}

func (s *SharedMemoryStore) Put(ctx context.Context, agentID, key string, value any) error {
    fullKey := fmt.Sprintf("shared:%s:%s", agentID, key)
    data, _ := json.Marshal(value)
    return s.store.Set(ctx, fullKey, data, 0)
}

func (s *SharedMemoryStore) Get(ctx context.Context, agentID, key string) (any, error) {
    fullKey := fmt.Sprintf("shared:%s:%s", agentID, key)
    data, err := s.store.Get(ctx, fullKey)
    if err != nil {
        return nil, err
    }
    var result any
    return result, json.Unmarshal(data, &result)
}

func (s *SharedMemoryStore) ListAll(ctx context.Context) (map[string]any, error) {
    keys, _ := s.store.Keys(ctx, "shared:*")
    result := make(map[string]any)
    for _, key := range keys {
        val, _ := s.store.Get(ctx, key)
        var parsed any
        _ = json.Unmarshal(val, &parsed)
        result[key] = parsed
    }
    return result, nil
}
```

> **Связь:** Подробнее об управлении состоянием агента — в [Главе 11: State Management](../11-state-management/README.md). Чекпоинт — частный случай персистенции состояния.

## Типовые ошибки

### Ошибка 1: Нет TTL

**Симптом:** Память растёт бесконечно, потребляя хранилище и контекст. Retrieval возвращает устаревшие факты.

**Причина:** Не забывается устаревшая информация.

**Решение:** Реализуйте TTL и периодическую очистку. Для разных типов фактов — разный TTL: предпочтения пользователя живут долго, временный статус — часы.

### Ошибка 2: Сохранение всего

**Симптом:** Память заполняется нерелевантной информацией, retrieval становится шумным.

**Причина:** Нет фильтрации того, что вообще стоит сохранять.

**Решение:** Сохраняйте только важные факты, не каждый поворот разговора. Хороший фильтр — отдельный лёгкий LLM-вызов в конце Run: «извлеки из этого диалога факты, которые стоит запомнить надолго».

### Ошибка 3: Только keyword-поиск

**Симптом:** Извлечение возвращает нерелевантные результаты или пропускает важную информацию.

**Причина:** Простое сопоставление подстрок не понимает синонимов и парафразов.

**Решение:** Используйте embeddings для семантического поиска. Hybrid (BM25 + embeddings) обычно лучше любого из двух по отдельности.

### Ошибка 4: Compact уничтожает оригиналы

**Симптом:** После компакции невозможно восстановить детали tool calls. Recall возвращает сжатую версию.

**Причина:** Оригинальные сообщения удалены при создании compact-версии.

**Решение:**

```go
// ПЛОХО: перезаписали оригинал
block.Messages = compactMessages(block.Messages)

// ХОРОШО: оригинал жив, compact — отдельный view
block.Compact = compactMessages(block.Messages)
// block.Messages не трогаем
```

Принцип **never destroy originals**: любая форма сжатия — это view, не мутация. Оригиналы нужны и для recall, и для recovery, и для аудита.

### Ошибка 5: Compact или condense mid-loop

**Симптом:** Агент теряет контекст посередине задачи, начинает заново или повторяет действия. На следующей итерации модель не видит, что только что обсуждалось.

**Причина:** Сжатие вызывается внутри цикла агента, пока задача ещё выполняется.

**Решение:** Compact — только при закрытии блока. Condense — по threshold или `ContextOverflowError`, и не чаще одного раза за Run. Внутри живой задачи историю не трогайте.

### Ошибка 6: Live state в system prompt

**Симптом:** На каждой итерации prompt-cache hit ≈ 0%, latency растёт линейно с длиной истории, цена ×3-5 от ожидаемой. Метрика `cached_tokens` от провайдера колеблется около нуля.

**Причина:** В system prompt вписаны изменяющиеся данные (текущий файл, прочитанные файлы, прогресс плана). Любая мутация делает invalidate всего префикса.

**Решение:** Держи system prompt стабильным. Стабильные включения (дата, рабочая директория) фиксируй один раз в начале Run и больше не меняй. Live state кладёт в tool results, в Notes последнего сообщения или в специальные tool-вызовы (`update_plan`, `set_goal`), которые модель совершает сама.

### Ошибка 7: Оценка токенов через char/3

**Симптом:** Threshold-condense срабатывает не вовремя — то слишком рано (теряем контекст зря), то слишком поздно (получаем `ContextOverflowError`). Поведение между моделями сильно расходится.

**Причина:** «Длина в символах / 3» — приближение, которое систематически промахивается на 30%+. Для русского, кода, и моделей с обновлёнными токенизаторами (например, после смены словаря) промах ещё больше.

**Решение:** Берите `Usage.PromptTokens` из ответа провайдера на предыдущую итерацию — это бесплатно и точно. Char-based оценка нужна только для нового пользовательского сообщения, которое ещё не уехало в LLM.

```go
// ПЛОХО
estimate := totalChars / 3
if estimate > threshold { condense() }

// ХОРОШО
budget := lastUsage.PromptTokens + roughEstimate(newUserMessage)
if budget > threshold { condense() }
```

## Мини-упражнения

### Упражнение 1: File-backed Memory

Реализуйте хранилище памяти, переживающее перезапуск процесса:

```go
type FileMemory struct {
    filepath string
    // ...
}

func (m *FileMemory) Store(key string, value any, metadata map[string]any) error {
    // Сохраните в JSON-файл (atomic write через temp + rename)
}
```

**Ожидаемый результат:**
- Память переживает перезапуск.
- Запись атомарна (нет частично записанных файлов при крахе).
- Есть отдельная команда `Cleanup()` для пробежки по TTL.

### Упражнение 2: Семантический поиск

Реализуйте retrieval через embeddings:

```go
func (m *Memory) RetrieveSemantic(query string, limit int) ([]MemoryItem, error) {
    // Закодируйте query, посчитайте cosine similarity к items, верните top-K
}
```

**Ожидаемый результат:**
- Находит релевантные элементы без точного совпадения слов.
- Возвращает наиболее похожие первыми.
- Падение модели embeddings не должно крашить Retrieve (ошибки логируются, метод возвращает то, что смог).

### Упражнение 3: Линейная память + threshold-condense

Реализуйте `LinearMemory` (Append / Snapshot / Reset) и функцию-сторож:

```go
func shouldCondense(usage llm.Usage, ctxWindow int, threshold float64) bool {
    // true, если usage.PromptTokens / ctxWindow >= threshold
}

func condense(ctx context.Context, ep llm.Endpoint, msgs []llm.Message) ([]llm.Message, error) {
    // 1. Разделить на head (старое) и tail (последние 2 user-шага)
    // 2. Попросить ep собрать summary head по промпту из гл. 13
    // 3. Вернуть [summary as user message] + tail
}
```

**Ожидаемый результат:**
- Память append-only, никаких mutations кроме `Reset`.
- Threshold срабатывает по реальным `PromptTokens`, не по char/3.
- Original-snapshot сохраняется до успешного condense (если LLM-вызов упал, история не теряется).
- Condense не вызывается чаще одного раза за Run.

## Критерии сдачи / Чек-лист

✅ **Сдано:**
- Понимаете разделение «внутри Run» vs «между сессиями».
- Используете линейную память по умолчанию; переходите на блочную осознанно.
- Не мутируете system prompt live-состоянием.
- Различаете compact, condense и recall по применимости.
- Считаете токены через `Usage.PromptTokens`, не через char/3.
- Реализуете TTL и фильтрацию для долговременной памяти.
- Соблюдаете «never destroy originals».

❌ **Не сдано:**
- Compact или condense вызываются mid-loop.
- Live state живёт в system prompt.
- Char/3 — единственный счётчик токенов.
- Compact уничтожает оригиналы.
- Долговременная память без TTL и без фильтра, что туда класть.
- Используется блочная память без явной причины (вроде REPL или модельного recall).

## Связь с другими главами

- **[Глава 09: Анатомия Агента](../09-agent-architecture/README.md)** — память как один из ключевых компонентов агента, связь с runtime.
- **[Глава 11: State Management](../11-state-management/README.md)** — чекпоинты, идемпотентность, persist state.
- **[Глава 13: Context Engineering](../13-context-engineering/README.md)** — сборка контекста из памяти, бюджеты, condense, динамические секции system prompt.
- **[Глава 20: Cost & Latency Engineering](../20-cost-latency-engineering/README.md)** — почему prompt cache настолько важен и как его не сломать.

> **Граница:** эта глава отвечает за **хранение и извлечение** информации. Управление тем, как эта информация складывается в контекст LLM, — в [Context Engineering](../13-context-engineering/README.md).

## Что дальше?

После понимания систем памяти переходите к:
- **[13. Context Engineering](../13-context-engineering/README.md)** — научитесь собирать контекст из памяти, состояния и retrieval, не ломая prompt cache.
