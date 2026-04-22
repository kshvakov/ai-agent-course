# Методическое пособие: Lab 09 — Context Optimization

## Зачем это нужно?

Лаба учит **гигиене контекста**: правильно считать токены, правильно решать «пора сжимать», правильно сжимать. Никаких многоуровневых стратегий — это в [гл. 13](../../book/13-context-engineering/README.md) выкинуто как оверинжиниринг.

### Реальный кейс

DevOps-агент в проде. После часа работы с логами и многих тулзов:

- **Без подсчёта токенов**: рано или поздно провайдер возвращает `context_length_exceeded`, агент валится в середине инцидента.
- **С `len(text)/4` как первоисточником**: ваш счётчик расходится с реальным `usage.PromptTokens` на 30-50% (русский, спецсимволы, JSON tool-выводов). Решения принимаются с погрешностью.
- **С `usage.PromptTokens`**: вы знаете ровно столько, сколько засчитал биллинг. Один порог, одна реакция.

## Теория простыми словами

### Откуда берётся число токенов

```text
provider response
  ├── choices[0].message
  └── usage
        ├── prompt_tokens     ← сколько ушло на вход (включая system + history + tools)
        ├── completion_tokens ← сколько сгенерировалось
        └── total_tokens
```

`prompt_tokens` — это и есть авторитет. Провайдер посчитал ровно столько, сколько списал с вашего баланса. Любой свой счётчик годится только до получения первого ответа (на cold start) или для прикидки «влезет ли вот это сообщение, не отправляя его».

### Один порог, одна реакция

Был соблазн делать «ничего → priorit → summarize → eviction». Не нужно. На практике достаточно:

```text
if lastTokens > contextMax * 0.80 && !condenseDone {
    condense()
}
```

И отдельно — реактивная ветка:

```text
if isContextOverflow(err) {
    condense()
    retry once
}
```

Почему лесенка плоха:

- Каждая «техника» что-то ломает (пары tool-вызовов, prompt cache).
- Сложная логика выбора → сложно отлаживать.
- В реальности после первого `condense` вы уже не упрётесь в окно в этом же Run. Если упёрлись — задача не дробится, и надо вернуть ошибку наверх, а не накручивать ещё технику.

### Защита tool-пар

OpenAI-совместимый протокол: `assistant` сообщение с `tool_calls` обязано иметь следом `tool` сообщения с тем же `tool_call_id`. Если сжатие/обрезка разрывает пару — провайдер ответит 400.

```text
ok:    [system, user, assistant{tc:42}, tool{tc:42}, assistant, ...]
fail:  [system, ......................   tool{tc:42}, assistant, ...]
                                          ^^ нет своего assistant tool_call в хвосте
```

Решение — `safeTail`, который при формировании хвоста сдвигает левую границу влево, пока первый элемент хвоста — `tool`.

### Куда положить summary

Только как обычное `user` сообщение, **не** как `system`:

```text
[system, user("Контекст предыдущей работы:\n\n<summary>"), tail...]
```

Почему не `system`:

- prompt cache рассчитан на стабильный префикс. Каждое изменение system → cache miss → дорого.
- Модель различает «инструкции» (system) и «данные» (user). Смешивая, вы получаете путаницу: модель может начать «исполнять» summary как инструкцию.

## Алгоритм выполнения

### Шаг 1: грубая прикидка токенов (для до-отправки)

```go
func estimateTokens(text string) int {
    if text == "" { return 0 }
    return len(text) / 3 + 1
}

func estimateMessages(msgs []openai.ChatCompletionMessage) int {
    total := 4 // overhead на envelope
    for _, m := range msgs {
        total += estimateTokens(m.Content) + 4
        for _, tc := range m.ToolCalls {
            total += estimateTokens(tc.Function.Name) +
                     estimateTokens(tc.Function.Arguments) + 8
        }
    }
    return total
}
```

Это плохая оценка для биллинга, но достаточная для «не отправлять заведомо переполненный батч». Для production используйте `tiktoken-go` для своей модели — но всё равно сверяйтесь с `usage.PromptTokens`.

### Шаг 2: `Run`-цикл

```go
type Run struct {
    messages     []openai.ChatCompletionMessage
    lastTokens   int
    contextMax   int
    condenseDone bool

    client *openai.Client
    model  string
    tools  []openai.Tool
}

func (r *Run) Step(ctx context.Context, userInput string) (string, error) {
    r.messages = append(r.messages, openai.ChatCompletionMessage{
        Role: openai.ChatMessageRoleUser, Content: userInput,
    })

    for {
        if r.lastTokens > 0 &&
            float64(r.lastTokens) > float64(r.contextMax)*0.80 {
            if err := r.condense(ctx); err != nil {
                return "", err
            }
        }

        resp, err := r.callLLM(ctx)
        if err != nil {
            if !isContextOverflow(err) { return "", err }
            if cerr := r.condense(ctx); cerr != nil { return "", cerr }
            resp, err = r.callLLM(ctx)
            if err != nil {
                return "", fmt.Errorf("overflow even after condense: %w", err)
            }
        }

        r.lastTokens = resp.Usage.PromptTokens
        msg := resp.Choices[0].Message
        r.messages = append(r.messages, msg)

        if len(msg.ToolCalls) == 0 {
            return msg.Content, nil
        }

        for _, tc := range msg.ToolCalls {
            result := r.dispatchTool(ctx, tc)
            r.messages = append(r.messages, openai.ChatCompletionMessage{
                Role:       openai.ChatMessageRoleTool,
                ToolCallID: tc.ID,
                Name:       tc.Function.Name,
                Content:    result,
            })
        }
    }
}
```

### Шаг 3: `safeTail`

```go
func safeTail(msgs []openai.ChatCompletionMessage, n int) []openai.ChatCompletionMessage {
    if n > len(msgs)-1 { n = len(msgs) - 1 }
    start := len(msgs) - n
    for start > 1 && msgs[start].Role == openai.ChatMessageRoleTool {
        start--
    }
    return msgs[start:]
}
```

Для большинства случаев этого достаточно. Если хотите идеально (не разрывать assistant с pending `tool_calls`), сделайте обратный обход и проверяйте, что для каждого `tool` в хвосте есть свой `assistant` с этим `tool_call_id`.

### Шаг 4: `condense`

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

### Шаг 5: лог estimated/actual

После каждого `r.callLLM` выводите:

```go
estimated := estimateMessages(r.messages)
fmt.Printf("Step %d: estimated=%d, actual=%d (Δ=%+d, %+.1f%%), threshold@80%%=%d\n",
    step, estimated, r.lastTokens, r.lastTokens-estimated,
    float64(r.lastTokens-estimated)*100/float64(estimated),
    int(float64(r.contextMax)*0.80))
```

## Типовые ошибки

### Ошибка 1: `len(text)/4` как первоисточник

**Симптом:** `condense` срабатывает или слишком рано (зря дёргаем LLM на summary), или слишком поздно (получаем 400 от провайдера).

**Причина:** свой счётчик не учитывает реальную токенизацию модели + overhead на envelope/tools.

**Решение:** `r.lastTokens = resp.Usage.PromptTokens` после **каждого** ответа. Свой счётчик — только для прикидки «влезет ли вот это» до отправки.

### Ошибка 2: лесенка `prioritize → summarize → truncate`

**Симптом:** в коде три ветки оптимизации; одна из них рано или поздно ломает пары tool-вызовов или меняет порядок сообщений.

**Причина:** скопировано из старых статей. На самом деле достаточно одной операции.

**Решение:** один `condense` по порогу + реактивный на overflow. Если после первого condense снова overflow — fail, дробите задачу.

### Ошибка 3: truncate без проверки tool-пар

**Симптом:** провайдер возвращает 400 «`tool_calls` without matching tool messages».

**Причина:** обрезали ровно между `assistant.tool_call` и `tool` result.

**Решение:** `safeTail` или явный обратный обход с проверкой пар.

### Ошибка 4: summary в `system`

**Симптом:** при каждом `condense` стоимость одного запроса вырастает (cache miss); модель иногда начинает «исполнять» summary как инструкции.

**Причина:** summary положили в `system` сообщение или склеили с исходным system.

**Решение:** summary — обычное `user` сообщение между `system[0]` и хвостом.

### Ошибка 5: condense выполняется на каждом шаге

**Симптом:** агент тормозит, на каждом шаге дополнительный LLM-вызов, история то «ужимается», то снова растёт, контекст путается.

**Причина:** нет лимита.

**Решение:** флаг `condenseDone bool`. Если после первого condense снова overflow — это сигнал, что задача не дробится. Возвращайте ошибку.

### Ошибка 6: `maxContextTokens = 4000` хардкод

**Симптом:** код привязан к одной модели, при смене модели рвутся лимиты.

**Причина:** константа в файле.

**Решение:** окно берётся из конфигурации модели (метаданные провайдера / каталога моделей), а не хардкодится.

## Мини-упражнения

### Упражнение 1: точная оценка через `tiktoken-go`

Подключите `github.com/pkoukk/tiktoken-go`, реализуйте `estimateTokensTiktoken(text, modelName)`, прогоните оба счётчика и сравните с `usage.PromptTokens`. Tiktoken даст точность для OpenAI-моделей, но всё равно не покроет provider-specific токенизаторы (Anthropic, GigaChat и др.).

### Упражнение 2: «идеальный» `safeTail`

Перепишите `safeTail`, чтобы он:

1. Не разрывал `assistant{tool_calls=[a,b,c]}` от `tool{a}, tool{b}, tool{c}`.
2. Возвращал именно `>=N` сообщений, не `=N` (если граница пары не пускает — расширяйте).

### Упражнение 3: артефакты для больших tool-результатов

Если tool возвращает >2000 токенов (логи, JSON-дампы), не кладите весь вывод в `messages`. Сохраните в файл/KV под `artifact_id`, в `tool` сообщение положите короткий excerpt + ID. Это уменьшит расход токенов и сделает `condense` менее частым.

## Критерии сдачи

✅ **Сдано:**

- [x] Решение «пора сжимать» принимается по `resp.Usage.PromptTokens`.
- [x] Один порог `0.80` + реактивный `condense` на `ContextOverflowError`.
- [x] `condense` гарантированно один раз за Run; повтор после реактивного condense ровно один.
- [x] После `condense`: `messages[0]` стабилен; пары tool-вызовов целы; summary в роли `user`.
- [x] Логи показывают estimated и actual токены; видно момент срабатывания condense.
- [x] Окно модели берётся из конфигурации, не хардкодится.

❌ **Не сдано:**

- [ ] `len(text)/4` как первоисточник.
- [ ] Лесенка `prioritize → summarize → truncate`.
- [ ] Truncate без `safeTail`.
- [ ] Summary в роли `system` (или склейка `system+summary+facts+working`).
- [ ] Несколько condense подряд без лимита.
- [ ] Хардкод `4000` как «дефолт окна» — это наследие старых моделей; берите лимит из метаданных модели/конфигурации.

---

**Следующий шаг:** [Lab 10: Planning и Workflows](../lab10-planning-workflows/README.md). Долгосрочная память — в [Lab 11: Memory & Context](../lab11-memory-context/README.md).
