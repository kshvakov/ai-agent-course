# Методическое пособие: Lab 11 — Memory и Context Management

## Зачем это нужно?

В этой лабе вы реализуете **минимально достаточную** систему памяти для агента: один `condense` при заполнении окна модели и три инструмента для долгосрочных заметок. Без `LayeredContext`, без автоматического извлечения фактов, без скоринга «важности».

### Реальный кейс

**Ситуация:** агент-помощник работает с пользователем длинными сессиями.

**Без памяти:**

- Пользователь: «Меня зовут Иван».
- Через 50 сообщений: «Как меня зовут?» → «Я не знаю» (упёрлись в окно, history обрезана).

**С памятью (правильно):**

- Пользователь: «Запомни, что меня зовут Иван и я отвечаю за prod-кластер».
- Агент сам решает: «это устойчивый факт» → вызывает `memory.save("user.name", "Иван, отвечает за prod-кластер")`.
- Через 50 сообщений / в новой сессии: «Кто я по проекту?» → агент вызывает `memory.recall("кто пользователь")` → отвечает.

**Разница с «модной» схемой:** не агент-сервер автоматом гонит весь диалог через extraction-LLM, а сам агент решает, что записать. Меньше шума, понятный лог, дешевле.

## Теория простыми словами

### Два горизонта памяти

```text
                    one Run                       many Runs
              ┌────────────────────────┐    ┌──────────────────┐
   In-Run     │  []Message (linear)    │    │   forgotten      │
              │  + один condense       │    │   при exit       │
              └────────────────────────┘    └──────────────────┘

   Long-term  ┌────────────────────────────────────────────────┐
              │   Store: save / recall / delete (tool calls)   │
              │   живёт в файле / БД, агент сам пишет          │
              └────────────────────────────────────────────────┘
```

| Горизонт | Срок | Кто пишет | Где |
|---|---|---|---|
| In-Run | один запуск | runtime | `[]openai.ChatCompletionMessage` |
| Long-term | переживает рестарт | агент через tool | JSON / SQLite |

### Один порог, одна реакция

Цикл управления окном — простейший:

1. После каждого ответа провайдера запомнили `usage.PromptTokens`.
2. Перед следующим запросом проверили: `lastTokens > contextMax * 0.80`?
3. Да → один `condense` (если ещё не делали в этом Run).
4. Если провайдер всё равно вернул `ContextOverflowError` → реактивный `condense` + повторить запрос ровно один раз. Снова overflow → отдать ошибку наружу, дробить задачу.

Никаких 4-уровневых стратегий, никаких budget-трекеров, никакого скоринга сообщений. См. [гл. 13: Бюджет](../../book/13-context-engineering/README.md).

### Почему не truncate

Простое `messages = messages[len(messages)-N:]` ломает пары `tool_call ↔ tool_result` — провайдер ответит 400. Кроме того, теряется контекст без следа.

Condense:

1. Сохраняет system prompt **байт-в-байт** (важно для prompt cache).
2. Заменяет середину одним `user`-сообщением: «Контекст предыдущей работы: …».
3. Хвост (последние N сообщений) сохраняется целыми, с проверкой пар.

### Long-term memory как инструмент, а не как «слой»

В курсах часто учат «Final context = System + Facts layer + Summary layer + Working memory». Это плохая идея:

- Каждое изменение «Facts layer» инвалидирует prompt cache → вы платите за полный re-encode на каждом шаге.
- Содержимое в `system`-роли смешивается с правилами для модели — модель начинает путаться, что инструкция, а что данные.
- Авто-извлечение фактов из каждого сообщения = шум вроде «пользователь сказал спасибо».

Правильно: long-term память — это просто **инструменты** в каталоге (как `read_file` или `bash`), которые агент вызывает осознанно. В system prompt сообщаем только: «такие инструменты есть, используй для устойчивых фактов». Содержимое памяти попадает в контекст только когда агент сам сделал `memory.recall(...)`.

## Алгоритм выполнения

### Шаг 1: каркас Run + tracking токенов

```go
type Run struct {
    messages     []openai.ChatCompletionMessage
    lastTokens   int
    contextMax   int
    condenseDone bool
    client       *openai.Client
    model        string
    tools        []openai.Tool
}

func (r *Run) Step(ctx context.Context, userInput string) (string, error) {
    r.messages = append(r.messages, openai.ChatCompletionMessage{
        Role: openai.ChatMessageRoleUser, Content: userInput,
    })

    if err := r.beforeRequest(ctx); err != nil {
        return "", err
    }

    resp, err := r.callLLM(ctx)
    if err != nil {
        if isContextOverflow(err) {
            if cerr := r.condense(ctx); cerr != nil {
                return "", cerr
            }
            resp, err = r.callLLM(ctx)
            if err != nil {
                return "", err
            }
        } else {
            return "", err
        }
    }

    r.lastTokens = resp.Usage.PromptTokens
    return r.handleResponse(ctx, resp)
}

func (r *Run) beforeRequest(ctx context.Context) error {
    if r.lastTokens > 0 && float64(r.lastTokens) > float64(r.contextMax)*0.80 {
        return r.condense(ctx)
    }
    return nil
}
```

### Шаг 2: condense

```go
func (r *Run) condense(ctx context.Context) error {
    if r.condenseDone || len(r.messages) < 6 {
        return nil
    }

    system := r.messages[0]
    tail := safeTail(r.messages, 4)
    head := r.messages[1 : len(r.messages)-len(tail)]

    summary, err := r.summarize(ctx, head)
    if err != nil {
        return err
    }

    next := make([]openai.ChatCompletionMessage, 0, 2+len(tail))
    next = append(next, system)
    next = append(next, openai.ChatCompletionMessage{
        Role:    openai.ChatMessageRoleUser,
        Content: "Контекст предыдущей работы:\n\n" + summary,
    })
    next = append(next, tail...)

    r.messages = next
    r.condenseDone = true
    return nil
}

// safeTail возвращает >=N последних сообщений, расширяя границу влево,
// если первый элемент — tool result без своего tool_call в хвосте.
func safeTail(msgs []openai.ChatCompletionMessage, n int) []openai.ChatCompletionMessage {
    if n > len(msgs)-1 {
        n = len(msgs) - 1
    }
    start := len(msgs) - n
    for start > 1 && msgs[start].Role == openai.ChatMessageRoleTool {
        start--
    }
    return msgs[start:]
}
```

Промпт для `summarize`:

```text
Ты сжимаешь рабочую переписку агента в краткую справку для следующего шага.
Сохрани:
1. Исходную задачу пользователя.
2. Решения, которые уже приняты, и обоснования.
3. Какие файлы / ресурсы прочитаны и что в них релевантно.
4. Что ещё осталось сделать.
Опусти вежливости и служебный шум.
```

### Шаг 3: long-term memory как Store + tools

Хранилище — простой JSON-файл:

```go
type Entry struct {
    Key       string    `json:"key"`
    Value     string    `json:"value"`
    CreatedAt time.Time `json:"created_at"`
}

type FileStore struct {
    mu      sync.Mutex
    path    string
    entries []Entry
}

func (s *FileStore) Save(_ context.Context, key, value string) error {
    s.mu.Lock(); defer s.mu.Unlock()
    for i, e := range s.entries {
        if e.Key == key {
            s.entries[i].Value = value
            s.entries[i].CreatedAt = time.Now()
            return s.flush()
        }
    }
    s.entries = append(s.entries, Entry{key, value, time.Now()})
    return s.flush()
}

func (s *FileStore) Recall(_ context.Context, query string) ([]Entry, error) {
    s.mu.Lock(); defer s.mu.Unlock()
    q := strings.ToLower(query)
    var hits []Entry
    for _, e := range s.entries {
        if q == "" || strings.Contains(strings.ToLower(e.Key+" "+e.Value), q) {
            hits = append(hits, e)
        }
    }
    if len(hits) > 5 {
        hits = hits[:5]
    }
    return hits, nil
}

func (s *FileStore) Delete(_ context.Context, key string) error {
    s.mu.Lock(); defer s.mu.Unlock()
    out := s.entries[:0]
    for _, e := range s.entries {
        if e.Key != key {
            out = append(out, e)
        }
    }
    s.entries = out
    return s.flush()
}
```

Регистрация инструментов:

```go
tools := []openai.Tool{
    {Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{
        Name: "memory_save",
        Description: "Save a long-term note. Use for stable facts about the user or project.",
        Parameters: jsonSchema(`{"type":"object","properties":{
            "key":{"type":"string"},"value":{"type":"string"}
        },"required":["key","value"]}`),
    }},
    {Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{
        Name: "memory_recall",
        Description: "Search long-term notes by query (substring).",
        Parameters: jsonSchema(`{"type":"object","properties":{
            "query":{"type":"string"}
        },"required":["query"]}`),
    }},
    {Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{
        Name: "memory_delete",
        Description: "Delete a note by key.",
        Parameters: jsonSchema(`{"type":"object","properties":{
            "key":{"type":"string"}
        },"required":["key"]}`),
    }},
}
```

System prompt — один раз и только про роли/правила:

```text
You are an assistant.
You have memory_save / memory_recall / memory_delete tools that persist between sessions.
Use them for stable facts about the user and project. Don't store transient statuses.
```

### Шаг 4: tool dispatching loop

```go
for {
    resp, err := r.callLLM(ctx)
    if err != nil { return "", err }
    r.lastTokens = resp.Usage.PromptTokens

    msg := resp.Choices[0].Message
    r.messages = append(r.messages, msg)

    if len(msg.ToolCalls) == 0 {
        return msg.Content, nil
    }

    for _, tc := range msg.ToolCalls {
        result := dispatchTool(ctx, store, tc)
        r.messages = append(r.messages, openai.ChatCompletionMessage{
            Role:       openai.ChatMessageRoleTool,
            ToolCallID: tc.ID,
            Content:    result,
        })
    }
}
```

## Типовые ошибки

### Ошибка 1: динамический system prompt со списком памяти

**Симптом:** дорого и медленно — каждое обращение пересчитывает префикс.

**Причина:** содержимое long-term памяти склеивается в `messages[0]` при каждом запросе.

**Решение:** памятью пользуется агент через `memory.recall`, в system prompt — только описание, что инструменты существуют.

### Ошибка 2: truncate без проверки tool-пар

**Симптом:** провайдер возвращает 400 с сообщением про `tool_calls without matching tool messages`.

**Причина:** обрезали `messages` ровно между `assistant`-tool_call и `tool` result.

**Решение:** `condense` или `safeTail`, расширяющий границу влево до целой пары.

### Ошибка 3: токены считаются через `len(content)/3`

**Симптом:** condense срабатывает или слишком рано, или слишком поздно — реальная стоимость не совпадает с предсказанной.

**Причина:** не используется `usage.PromptTokens` от провайдера.

**Решение:** записывайте `r.lastTokens = resp.Usage.PromptTokens` после каждого ответа. Свои оценки годятся только для прикидки до отправки.

### Ошибка 4: condense выполняется на каждом шаге

**Симптом:** агент тормозит, история постоянно «ужимается», теряется свежий контекст.

**Причина:** не выставлен лимит «один condense на Run».

**Решение:** флаг `condenseDone bool`. Если после первого condense снова overflow — это сигнал, что задача не дробится, и надо вернуть ошибку.

### Ошибка 5: автоизвлечение «фактов» из каждого сообщения

**Симптом:** в памяти растёт мусор вроде `user_said_hi=true`.

**Причина:** на каждом шаге запускается отдельный LLM-проход «извлеки факты».

**Решение:** убрать совсем. Агент сам решает, что записать, через `memory_save`. Это и дешевле, и читаемо в логах.

## Критерии сдачи

✅ **Сдано:**

- `messages[0]` (system) стабилен весь Run — проверено сравнением байт-в-байт.
- `condense` срабатывает по `usage.PromptTokens > contextMax * 0.80` или реактивно на overflow.
- В одном Run — не более одного `condense` (плюс ровно один реактивный повтор при overflow).
- После `condense` пары `tool_call ↔ tool_result` целы.
- Long-term память работает через 3 tool'а и переживает рестарт процесса.
- В system prompt нет содержимого long-term памяти.

❌ **Не сдано:**

- `LayeredContext` (Facts/Summary/Working слои в один промпт).
- Авто-извлечение фактов из каждого сообщения.
- Truncate без проверки tool-пар.
- Несколько condense подряд без лимита.
- Подсчёт окна через `len(content)/3` вместо `usage.PromptTokens`.
