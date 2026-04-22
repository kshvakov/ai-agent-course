# Lab 09: Context Optimization (Оптимизация контекста)

## Цель

Научиться корректно управлять контекстным окном LLM:

1. **Считать токены так, как их считает провайдер** — `usage.PromptTokens` из ответа, не свои оценки.
2. **Принимать решение «пора сжимать» по одному порогу** (`0.80` от окна модели), а не лесенкой из 3-4 техник.
3. **Сжимать историю одним `condense`-проходом**, сохраняя пары `tool_call ↔ tool_result`.
4. **Реагировать на `ContextOverflowError`** реактивным `condense` + ровно одним повтором.

Это база. В [Lab 11](../lab11-memory-context/README.md) поверх этого добавляется второй горизонт — долгосрочная память между сессиями (как инструменты).

Чему лаба **не учит** (потому что это вредно — см. [гл. 13: Типовые ошибки](../../book/13-context-engineering/README.md#типовые-ошибки)):

- ❌ `len(text)/4` как первоисточник числа токенов.
- ❌ Адаптивная лесенка `prioritize → summarize → truncate`.
- ❌ Скоринг сообщений и переупорядочивание истории.
- ❌ Truncate без проверки пар `tool_call ↔ tool_result`.
- ❌ Summary в роли `system` (ломает prompt cache и смешивает данные с инструкциями).

## Теория

### Один порог, одна реакция

```text
ответ N+1 ──┐
            ▼
    usage.PromptTokens  ← это первоисточник
            │
            ▼
   > contextMax * 0.80?
            │
       ┌────┴────┐
      Нет        Да
       │          │
  следующий   condense() ── один раз за Run
   запрос         │
            следующий запрос
```

Если после condense провайдер всё-таки вернул `ContextOverflowError` — повторяем `condense + retry` ровно один раз. Снова overflow → возвращаем ошибку наверх. Дробите задачу.

### Где `estimateTokens(text)` всё-таки нужен

Своя оценка `≈ len(content)/3` (или сложнее, через `tiktoken-go`) полезна **до отправки запроса** — чтобы не отправлять заведомо переполненный батч и сэкономить один round-trip. Но первичным источником числа токенов в проде остаётся `resp.Usage.PromptTokens`.

Грубое правило:

| Где | Что используем |
|---|---|
| Решение «надо ли уже сейчас сжать» | `lastTokens = resp.Usage.PromptTokens` (с прошлого ответа) |
| «Влезет ли вот это конкретное сообщение» (до отправки) | `estimateTokens(content)` |
| Метрики / биллинг | только `usage.*` от провайдера |

### Почему condense, а не truncate

Простое `messages = messages[-N:]` или «возьмём только последние 5 + tool-результаты» ломает пары:

```text
[..., assistant{tool_call:tc_42}, tool{tc_42, "ok"}, ...]
                                   ^
                       обрезали тут — провайдер 400
```

`condense` сохраняет:

1. **system** сообщение в `messages[0]` — байт-в-байт, важно для prompt cache.
2. Хвост (последние N сообщений) с расширением границы влево, чтобы tool-пары были целы.
3. Середину истории заменяет **одним user-сообщением** «Контекст предыдущей работы: …».

См. подробнее:

- [Глава 13: Бюджет — один порог, одна реакция](../../book/13-context-engineering/README.md)
- [Глава 12: Системы памяти агента](../../book/12-agent-memory/README.md)

## Задание

В файле `main.go` реализуйте корректный цикл управления контекстом для длинного диалога с тестовым tool-вызовом.

### Часть 1: Прикидка токенов до отправки

```go
func estimateTokens(text string) int { /* len(text)/3 как байзлайн */ }
func estimateMessages(msgs []openai.ChatCompletionMessage) int
```

Эти функции — **только для прикидки до отправки**. Не используйте их как первоисточник.

### Часть 2: `Run` с трекингом `usage.PromptTokens`

```go
type Run struct {
    messages     []openai.ChatCompletionMessage
    lastTokens   int  // resp.Usage.PromptTokens с прошлого ответа
    contextMax   int  // окно модели, например 128_000
    condenseDone bool
}
```

Требования:

- `messages[0]` (system) фиксируется при старте Run и **не меняется**.
- После каждого ответа провайдера: `r.lastTokens = resp.Usage.PromptTokens`.
- Перед каждым следующим запросом: если `lastTokens > contextMax * 0.80` → `condense` (один раз).

### Часть 3: `condense` + `safeTail`

```go
func (r *Run) condense(ctx context.Context) error
func safeTail(msgs []openai.ChatCompletionMessage, n int) []openai.ChatCompletionMessage
```

`safeTail` возвращает **не менее** N последних сообщений, расширяя границу влево, если хвост начинается с `tool` без своего `assistant.tool_call` в хвосте.

`condense`:

1. Срабатывает не более одного раза на Run.
2. `system := messages[0]`, `tail := safeTail(messages, 4)`, `head := messages[1 : len(messages)-len(tail)]`.
3. Получает summary через отдельный LLM-запрос.
4. Собирает: `[system, user("Контекст предыдущей работы:\n\n"+summary), tail...]`.

### Часть 4: реактивный `condense` на overflow

В цикле `Run.Step`:

```go
resp, err := r.callLLM(ctx)
if err != nil {
    if isContextOverflow(err) {
        if cerr := r.condense(ctx); cerr != nil { return "", cerr }
        resp, err = r.callLLM(ctx) // ровно один повтор
        if err != nil { return "", fmt.Errorf("overflow even after condense: %w", err) }
    } else {
        return "", err
    }
}
```

### Часть 5: сравнение «предсказание vs факт»

В тестовом сценарии после каждого ответа выводите:

```text
Step 12: estimated=1840, actual=1812 (Δ=+28, +1.5%), threshold@80%=2560
```

Это упражнение покажет, насколько точна ваша `estimateMessages` относительно реальных `usage.PromptTokens`. Хорошая прикидка — расхождение в пределах 10-20%; точность не главное, главное — не отправить заведомо перепол­ненный запрос.

### Сценарий тестирования

В `main.go` гоним длинный диалог так, чтобы в середине условно занизить `contextMax` (например, до 4000 на лабе) и реально словить:

1. Пробивание порога `0.80` → проактивный condense.
2. (Опционально) симуляцию `ContextOverflowError` → реактивный condense.
3. Рост `usage.PromptTokens` в логе и срабатывание sjатия ровно один раз.

## Что проверить руками

1. После проактивного `condense`: `messages[0]` не изменился (сравните строкой), а длина истории уменьшилась.
2. После `condense` нет «висячих» tool-сообщений: первое сообщение хвоста — либо `user`, либо `assistant` без pending `tool_call`.
3. `condense` сработал ровно один раз — флаг `condenseDone = true` стоит.
4. `estimateMessages` отличается от `resp.Usage.PromptTokens` в разумных пределах (порядок величины тот же).

## Критерии сдачи

**Сдано:**

- [x] Решение «пора сжимать» принимается по `resp.Usage.PromptTokens`, не по своим оценкам.
- [x] Один порог `0.80` от `contextMax` + реактивный `condense` на `ContextOverflowError`.
- [x] `condense` срабатывает не более одного раза на Run.
- [x] После `condense`: `messages[0]` стабилен; пары `tool_call ↔ tool_result` целы; summary лежит как `user`-сообщение, не `system`.
- [x] `estimateTokens` используется только для прикидки до отправки, не как источник истины.
- [x] Логи показывают estimated/actual токены и момент срабатывания condense.

**Не сдано:**

- [ ] Решение принимается по `len(content)/3` без `usage.PromptTokens`.
- [ ] Адаптивная лесенка `prioritize → summarize → truncate` или скоринг сообщений.
- [ ] Truncate без `safeTail` (теряются пары tool-вызовов).
- [ ] Summary в роли `system` или склейка `system + facts + summary + working`.
- [ ] Несколько condense подряд в одном Run без явного лимита.
- [ ] Хардкоженный `maxContextTokens = 4000` (наследие моделей с маленьким окном) — берите окно из метаданных модели или конфигурации, у современных моделей оно 128k–1M.

---

**Следующий шаг:** [Lab 10: Planning и Workflows](../lab10-planning-workflows/README.md) — декомпозиция задач и сохранение состояния. Долгосрочную память между сессиями (как инструменты) добавите в [Lab 11: Memory & Context](../lab11-memory-context/README.md).
