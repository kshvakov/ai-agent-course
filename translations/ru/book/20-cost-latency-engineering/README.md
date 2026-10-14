# 20. Cost & Latency Engineering

## Зачем это нужно?

Агент работает, но счёт за LLM API растёт неконтролируемо. Один запрос стоит $5, а другой — $0.10. Почему? Без контроля стоимости и оптимизации latency вы не можете:
- Предсказать бюджет на месяц
- Оптимизировать дорогие запросы
- Гарантировать время ответа пользователю

Cost & Latency Engineering — это контроль бюджета и производительности. Без него легко потратить тысячи долларов на простые запросы или получить медленного агента, которым никто не будет пользоваться.

### Реальный кейс

**Ситуация:** Агент для DevOps работает в проде неделю. Счёт за LLM API — $5000 за месяц вместо ожидаемых $500.

**Проблема:** Агент использует самую дорогую модель для всех запросов, даже для простых проверок статуса. Нет лимитов на токены, system-префикс меняется между итерациями (кэш провайдера не работает), нет лимита итераций. Один запрос "проверь статус сервера" использует 50,000 токенов из-за большого контекста.

**Решение:** Бюджеты токенов, лимиты итераций, неизменяемый system-префикс под prompt cache провайдера, дешёвая модель на служебных узлах вне цикла. Теперь служебные запросы идут на лёгкую модель (GLM-5.3 Flash), основной цикл — на одну модель (GLM-5.3 или Fable 5.1). Счёт снизился до $600 в месяц.

> **Примечание:** Цены на LLM API меняются часто. Актуальные тарифы проверяйте на сайтах провайдеров и держите в конфиге. В этой главе конкретных цен нет — только формулы, в которые подставляются ваши актуальные тарифы.

## Теория простыми словами

### Что такое Cost Engineering?

Cost Engineering — это контроль и оптимизация стоимости использования LLM API. Основные рычаги:
1. **Выбор модели** — мощные модели (Fable 5.1, Opus 5.5) дороже лёгких (GLM-5.3 Flash) в разы; лёгкая — для служебных узлов вне цикла
2. **Количество токенов** — чем больше контекст, тем дороже; блок `tools[]` оплачивается в каждом запросе
3. **Prompt cache провайдера** — неизменяемый system-префикс (system + блок tools) кэшируется провайдером и тарифицируется в разы дешевле
4. **Количество запросов** — каждый вызов LLM стоит денег

### Что такое Latency Engineering?

Latency Engineering — это контроль времени ответа. Основные факторы:
1. **Модель** — мощные модели медленнее лёгких
2. **Размер контекста** — больше токенов = больше времени обработки
3. **Количество итераций** — больше циклов ReAct = больше времени
4. **Таймауты** — защита от зависаний

## Как это работает (пошагово)

### Шаг 1: Бюджеты токенов

Установите лимит платных токенов на ход (накопительно, с учётом кэш-скидки — см. ниже):

```go
const (
    MaxTokensPerTurn = 10000
    MaxIterations    = 10
)

type TokenBudget struct {
    Used  int
    Limit int
}

func checkTokenBudget(budget TokenBudget) error {
    if budget.Used > budget.Limit {
        return fmt.Errorf("token budget exceeded: %d > %d", budget.Used, budget.Limit)
    }
    return nil
}
```

### Шаг 2: Лимиты итераций

Ограничьте количество итераций ReAct loop — канон цикла и лимитов в [главе 04](../04-autonomy-and-loops/README.md); лаба [`lab04-autonomy`](https://github.com/kshvakov/ai-agent-course/tree/main/translations/ru/labs/lab04-autonomy) переведена на тот же нейтральный клиент. Примеры используют нейтральный клиент из главы 04: `client.Complete(ctx, Request{...})`:

```go
func runAgent(ctx context.Context, client Client, userInput string) (string, error) {
    maxIterations := 10
    tokenBudget := TokenBudget{Limit: MaxTokensPerTurn}

    messages := []Message{
        {Role: "system", Content: systemPrompt}, // неизменяемый префикс — под кэш
        {Role: "user", Content: userInput},
    }
    toolDefs := []ToolDef{/* инструменты хода (гл. 03) */}

    for i := 0; i < maxIterations; i++ {
        resp, err := client.Complete(ctx, Request{Messages: messages, Tools: toolDefs})
        if err != nil {
            return "", err // транзиентные ошибки ретраит сам клиент (гл. 04)
        }
        
        // Бюджет — из Usage провайдера, по платным токенам, а не по TotalTokens:
        // кэш-префикс дешевле генерации и вычитается из prompt (гл. 04, 13)
        tokenBudget.Used += resp.Usage.PromptTokens - resp.Usage.CacheReadTokens + resp.Usage.CompletionTokens
        
        // Проверяем бюджет
        if err := checkTokenBudget(tokenBudget); err != nil {
            return "", fmt.Errorf("stopping: %v", err)
        }
        
        // ... остальной код ...
    }
    
    return "", fmt.Errorf("max iterations (%d) exceeded", maxIterations)
}
```

### Шаг 2.5: Артефакты для больших результатов инструментов

Один из самых быстрых способов уменьшить cost/latency — перестать складывать "толстые" результаты инструментов прямо в `messages[]`.

Паттерн:

- сохраняем сырой результат инструмента во внешнем хранилище как **артефакт**,
- в диалог добавляем только короткий excerpt (например, top-20 строк) + `artifact_id`,
- если нужно — по запросу достаём артефакт кусками (`range`/`offset`/`limit`).

Это часто дешевле и надёжнее, чем саммаризация: вы не теряете данные, просто переносите их из контекста в хранилище.

### Шаг 3: Prompt cache — главный рычаг экономии

Классическая идея «закэшировать ответы LLM» — `map[hash(messages)] → answer` — для агентов **не работает**. Ключ почти всегда уникален: на каждой итерации история растёт (добавился ответ модели, добавились результаты инструментов), значит хэш меняется, значит попаданий в кэш нет. Ровно тот же вывод мы сделали для кэша контекста в [главе 13](../13-context-engineering/README.md).

Реальный рычаг — **prompt cache провайдера**. Провайдер кэширует **неизменяемый префикс** запроса: system-сообщение + блок `tools[]`. Правила:

- **Префикс не меняется** между итерациями и запросами: один и тот же system-промпт, один и тот же список tools. Всё динамическое — в конце истории (`user`, `tool`-сообщения).
- **История только дополняется**: запрос N+1 начинается с того же префикса + вся история запроса N. Провайдер не пересчитывает совпавший префикс.
- **Попадание видно по метрикам**: `cache-read > 0` в `Usage` ответа (названия полей у провайдеров отличаются — `cache_read` / `cached_tokens` / `CacheRead`, — суть одна).
- **Кэш привязан к модели**: смена модели между запросами ломает кэш — у каждой модели он свой.

```go
// Попадание в prompt cache видно прямо в Usage ответа:
resp, err := client.Complete(ctx, Request{Messages: messages, Tools: toolDefs})
if err != nil {
    return "", err
}
// resp.Usage.PromptTokens      — все prompt-токены (включая кэш)
// resp.Usage.CacheReadTokens   — сколько из них пришло из кэша провайдера
hitRate := float64(resp.Usage.CacheReadTokens) / float64(resp.Usage.PromptTokens)
```

Экономия двойная:

1. **Стоимость**: кэш-чтение тарифицируется в разы дешевле полных prompt-токенов (коэффициенты — в конфиге, см. [главу 19](../19-observability-and-tracing/README.md): таблица цен per-модель).
2. **Латентность**: кэшированный префикс не пересчитывается — время до первого токена заметно падает. Prompt cache — это одновременно и оптимизация **latency**, а не только денег.

Что убивает кэш на корню: «умный» system-промпт с датой/именем пользователя внутри, переупорядочивание tools между итерациями, ротация моделей внутри одного цикла.

### Шаг 4: Выбор модели — один раз, на границе

Модель выбирается **до старта цикла** и не меняется внутри него. Критерии — класс задачи и требования к качеству, а решение принимает код на границе системы: явные сценарии в конфиге, отдельный классификатор (см. «Дешёвая модель на служебных узлах» ниже) или человек.

```go
// selectModel вызывается один раз — до agent loop, не внутри него.
// Критерии приходят снаружи (конфиг сценария / классификатор), а не из
// эвристик по подстрокам пользовательского ввода: substring-магия хрупка
// и не переживает смену формулировок.
func selectModel(profile TaskProfile) string {
    switch profile {
    case ProfileAuxiliary: // служебные узлы вне цикла
        return "glm-5.3-flash"
    default: // основной цикл — одна модель на весь ход
        return "glm-5.3"
    }
}
```

Внутри цикла модель фиксирована: смена модели между итерациями ломает prompt cache (у каждой модели свой кэш) и обнуляет всю экономию из Шага 3.

### Шаг 5: Fallback-модели

Fallback-цепочка нужна для **отказоустойчивости**, а не для экономии: «если основная модель недоступна — ход продолжается на резервной, а не падает». Переключение — только при transient-ошибках и не по соображениям цены. Механизм — [гл. 03 → Fallbacks вместо маршрутизации](../03-tools-and-function-calling/README.md#fallbacks-вместо-маршрутизации) и [гл. 04](../04-autonomy-and-loops/README.md):

```go
// Fallback-и живут в конфигурации клиента (Resolve), цикл о них не знает.
cfg := ModelConfig{ // форма гл. 03: primary + явные fallback'и
    Primary:   "glm-5.3",
    Fallbacks: []string{"fable-5.1"}, // только доступность, не «экономия»
}
// Клиент сам повторяет запрос к резервной модели при transient-ошибке;
// счётчик переключений попадает в RunResult (гл. 04) и в метрики failovers (гл. 19).
```

### Шаг 6: Таймауты

Установите timeout для всего agent run и для каждого вызова инструмента:

```go
import "context"
import "time"

func runAgentWithTimeout(ctx context.Context, client Client, userInput string) (string, error) {
    // Timeout для всего agent run (5 минут)
    ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
    defer cancel()

    maxIterations := 10 // терминальная граница хода (гл. 04)

    messages := []Message{
        {Role: "system", Content: systemPrompt},
        {Role: "user", Content: userInput},
    }
    toolDefs := []ToolDef{/* инструменты хода (гл. 03) */}

    // ... agent loop ...
    
    for i := 0; i < maxIterations; i++ {
        // Timeout для каждого вызова LLM (30 секунд)
        callCtx, callCancel := context.WithTimeout(ctx, 30*time.Second)
        resp, err := client.Complete(callCtx, Request{Messages: messages, Tools: toolDefs})
        callCancel()

        if err != nil {
            if errors.Is(err, context.DeadlineExceeded) {
                return "", fmt.Errorf("LLM call timeout")
            }
            return "", err
        }

        // ... остальной код ...
    }

    // Исчерпание лимита итераций — терминальная граница (гл. 04)
    return "", fmt.Errorf("agent exceeded max iterations (%d)", maxIterations)
}
```

## Где это встраивать в нашем коде

### Точка интеграции 1: Agent Loop

В своём рантайме из главы 04 добавьте проверку бюджета и лимит итераций (ревизованная lab04-autonomy использует ту же форму цикла):

```go
const MaxIterations = 10
const MaxTokensPerTurn = 10000

// В цикле: бюджет — накопительный, на весь ход (гл. 04), не на один запрос
var usedPaidTokens int
for i := 0; i < MaxIterations; i++ {
    resp, err := client.Complete(ctx, Request{Messages: messages, Tools: toolDefs})
    if err != nil {
        return "", err // до чтения Usage: resp == nil при ошибке
    }
    // Копим платные токены (кэш-префикс дешевле, гл. 04/13) и сверяем с лимитом хода
    usedPaidTokens += resp.Usage.PromptTokens - resp.Usage.CacheReadTokens + resp.Usage.CompletionTokens
    if usedPaidTokens > MaxTokensPerTurn {
        return "", fmt.Errorf("token limit exceeded")
    }
    // ... остальной код ...
}
```

### Точка интеграции 2: Context Optimization

В своём рантайме добавьте condense по порогу окна (гл. 12/13), не по бюджету хода (ревизованная lab09-context-optimization использует тот же клиент):

```go
func adaptiveContextManagement(ctx context.Context, client Client, lastResp *Response, messages []Message, condenseAt float64) []Message {
    // Триггер сжатия — порог контекстного окна (гл. 12/13), не бюджет хода:
    // budget (MaxTokensPerTurn) — про деньги, condenseAt — про место в окне.
    // Порог считается по usage.PromptTokens последнего ответа провайдера (гл. 12);
    // пред-оценка «~3 символа на токен» — только fallback, пока ответа нет
    var usedTokens int
    if lastResp != nil {
        usedTokens = lastResp.Usage.PromptTokens // первичный источник — провайдер
    } else {
        usedTokens = estimateTokens(messages) // fallback до первого ответа (гл. 12)
    }
    window := contextWindowTokens() // размер окна модели (гл. 01)

    if float64(usedTokens)/float64(window) >= condenseAt { // типовое ~0.85
        return compressOldMessages(ctx, client, messages)
    }

    // ... остальная логика ...
}
```

## Мини-пример кода

Компактный пример с контролем стоимости на базе цикла из главы 04 — бюджеты, лимит итераций, таймауты, одна модель в цикле (клиент — нейтральный, из [главы 04](../04-autonomy-and-loops/README.md); шаги 1–2 и 6 выше, собранные в один листинг):

```go
const (
    MaxTokensPerTurn    = 10000
    MaxIterations       = 10
    LLMTimeout          = 30 * time.Second
    AgentTimeout        = 5 * time.Minute
)

type TokenBudget struct {
    Used  int
    Limit int
}

func checkTokenBudget(budget TokenBudget) error {
    if budget.Used > budget.Limit {
        return fmt.Errorf("token budget exceeded: %d > %d", budget.Used, budget.Limit)
    }
    return nil
}

// runAgentWithBudgets — цикл из главы 04 с бюджетом, лимитом итераций и таймаутами.
func runAgentWithBudgets(ctx context.Context, client Client, userInput string) (string, error) {
    ctx, cancel := context.WithTimeout(ctx, AgentTimeout) // дедлайн всего хода
    defer cancel()

    // Одна primary-модель на весь цикл (задана на клиенте, гл. 04) —
    // prompt cache работает, переключение только через fallback-цепочку.

    tokenBudget := TokenBudget{Limit: MaxTokensPerTurn}

    messages := []Message{
        {Role: "system", Content: systemPrompt}, // неизменяемый префикс — под кэш провайдера
        {Role: "user", Content: userInput},
    }
    toolDefs := []ToolDef{/* инструменты хода (гл. 03) */}

    for i := 0; i < MaxIterations; i++ {
        // Проверяем бюджет перед запросом
        if err := checkTokenBudget(tokenBudget); err != nil {
            log.Printf("budget exceeded: %v", err)
            return "", fmt.Errorf("token budget exceeded: %w", err) // терминально (гл. 04)
        }

        // Timeout для каждого вызова LLM
        callCtx, callCancel := context.WithTimeout(ctx, LLMTimeout)
        resp, err := client.Complete(callCtx, Request{Messages: messages, Tools: toolDefs})
        callCancel()

        if err != nil {
            if errors.Is(err, context.DeadlineExceeded) {
                log.Printf("LLM call timeout")
                return "", err
            }
            log.Printf("API error: %v", err)
            return "", err
        }

        msg := resp.Choices[0].Message
        messages = append(messages, msg)

        // Обновляем бюджет — по платным токенам, не по TotalTokens (гл. 04, 13).
        // Упрощение-нижняя оценка: кэш-чтение не бесплатное, а тарифицируется
        // со скидкой — точный учёт коэффициентом из конфига цен (гл. 19).
        tokenBudget.Used += resp.Usage.PromptTokens - resp.Usage.CacheReadTokens + resp.Usage.CompletionTokens
        log.Printf("tokens: %d/%d (cache_read=%d)",
            tokenBudget.Used, tokenBudget.Limit, resp.Usage.CacheReadTokens)

        if len(msg.ToolCalls) == 0 {
            return msg.Content, nil
        }

        for _, toolCall := range msg.ToolCalls {
            result, err := executeTool(ctx, toolCall)
            if err != nil {
                result = "error: " + err.Error()
            }
            messages = append(messages, Message{
                Role: "tool", Content: result, ToolCallID: toolCall.ID,
            })
        }
    }
    // Исчерпание лимита итераций — терминальная граница (гл. 04), не тихий возврат
    log.Printf("agent exceeded max iterations (%d), tokens: %d/%d", MaxIterations, tokenBudget.Used, tokenBudget.Limit)
    return "", fmt.Errorf("agent exceeded max iterations (%d)", MaxIterations)
}
```

## Дешёвая модель на служебных узлах

Идея «выбирать модель по сложности на каждой итерации» выглядит заманчиво, но внутри основного цикла она не работает: **смена модели между запросами ломает prompt cache** (у каждой модели свой кэш, Шаг 3), и «экономия 50%» из маркетинговых примеров не учитывает ни потерю кэша, ни рост latency на пересчёт префикса. Кроме того, дешёвая модель, выбирающая инструменты в середине хода, может увести рассуждение не туда — и тогда «сэкономленный» вызов оборачивается лишними итерациями.

Правильное место для дешёвой модели — **служебные узлы вне основного цикла**:

| Узел | Что делает | Модель |
|------|-----------|--------|
| Классификация входа | простой/сложный запрос, роутинг сценария | GLM-5.3 Flash |
| Structured output | извлечение полей, парсинг ответа в структуру | GLM-5.3 Flash |
| **Основной цикл** | ReAct-ход: выбор инструментов, анализ, ответ | **одна модель**: GLM-5.3 или Fable 5.1 |

**Condense — исключение, не служебный узел.** Запрос суммаризации несёт **тот же кэшируемый префикс** (system + tools) и идёт на ту же модель, что и основной цикл: head читается из прогретого кэша, а не платится по полной ([гл. 13](../13-context-engineering/README.md)). Дешёвая модель здесь — ложная экономия: свой промпт и «урезанная» история обнуляют cache-hit по всей старой истории, и condense становится дороже, чем вызов основной модели.

```go
// condense — по канону гл. 12/13: та же модель, тот же кэшируемый префикс
// (system + tools), вход — вся старая история (head), максимум 1 раз за Run.
func condense(ctx context.Context, ep Client, msgs []Message) ([]Message, error) {
    // 1. Разделить на head (старое) и tail (последние 2 user-шага)
    // 2. Собрать summary head: запрос несёт тот же префикс system + tools
    //    — head читается из прогретого кэша (гл. 13)
    // 3. Вернуть [summary as user message] + tail
}
```

Ключевые свойства служебных узлов: фиксированный формат выхода, короткий вход (нет растущей истории), ошибка обрабатывается fallback-ом на основную модель — качество деградирует мягко, а не падает.

**Fallback-цепочки — отдельная история:** они нужны для отказоустойчивости (основная модель недоступна — ход продолжается на запасной), а не для экономии. Механизм — [гл. 03](../03-tools-and-function-calling/README.md) и [гл. 04](../04-autonomy-and-loops/README.md).

> **Связь с другими главами:** Function Calling и устройство tools — [Глава 03: Инструменты и Function Calling](../03-tools-and-function-calling/README.md). Сам цикл агента — [Глава 04: Автономность и Циклы](../04-autonomy-and-loops/README.md).

## Бюджет контекста = деньги

Каждый запрос агента оплачивает не только историю, но и **блок `tools[]` целиком** — он уходит в каждом вызове. Четыре инструмента с развёрнутыми описаниями — это порядка ~700+ токенов в каждом запросе; на 10 итерациях и тысячах ходов в день счёт растёт быстрее, чем кажется.

Что с этим делать:

- **Пишите короткие описания инструментов**: имя, что делает, когда звать, ограничения. «Документация» на 500 токенов в description — это постоянный налог.
- **Не таскайте весь каталог**: давайте агенту список tools под задачу (узкий ToolRegistry), а не «всё, что есть». Связка — [гл. 03: Инструменты и Function Calling](../03-tools-and-function-calling/README.md).
- **Скилы вместо каталогов**: вместо 50 инструментов с длинными описаниями — несколько мета-инструментов («найди подходящий скил», «выполни скил»), которые подгружают инструкции по требованию. Тот же префикс, но оплата — по факту использования.
- **Следите за метрикой**: доля `cache_read / prompt` в Usage (гл. 19) показывает, что префикс действительно кэшируется, а не оплачивается заново.

Подробный разбор управления контекстом — [Глава 13: Context Engineering](../13-context-engineering/README.md).

## Streaming для снижения Perceived Latency

Latency бывает двух видов:
- **Фактическая** — время от запроса до последнего токена ответа
- **Воспринимаемая (perceived)** — время, которое пользователь *ощущает* как ожидание

Streaming не ускоряет генерацию. Модель тратит столько же времени на весь ответ. Но пользователь видит первые слова через 200-500 мс вместо того, чтобы ждать 5-10 секунд до появления полного ответа. Это принципиально меняет UX.

### Как работает streaming

Без streaming: запрос → ожидание 5 секунд → весь ответ целиком.

Со streaming: запрос → 200 мс → первый токен → токены появляются по одному → ответ готов.

Технически это Server-Sent Events (SSE) — HTTP-соединение остаётся открытым, сервер отправляет данные по мере готовности.

### Streaming в agent loop

```go
func streamAgentResponse(ctx context.Context, client Client, messages []Message) error {
    stream, err := client.Stream(ctx, Request{Messages: messages}) // нейтральный стриминг-вызов
    if err != nil {
        return fmt.Errorf("stream error: %w", err)
    }
    defer stream.Close()

    for {
        delta, err := stream.Recv()
        if errors.Is(err, io.EOF) {
            fmt.Println()
            return nil
        }
        if err != nil {
            return fmt.Errorf("stream recv error: %w", err)
        }
        fmt.Print(delta.Text) // Выводим фрагмент сразу, без буферизации
    }
}
```

### Streaming через SSE для веб-клиентов

Если агент работает как HTTP-сервер, стриминг передаётся клиенту через Server-Sent Events: соединение не закрывается, сервер пишет строки `data: ...` и вызывает `flusher.Flush()` после каждого фрагмента. Полный разбор SSE-транспорта (заголовки, `[DONE]`, реконнекты, WebSocket против SSE) — в [Главе 21: Workflow и State Management](../21-workflow-state-management/README.md).

### Когда streaming не помогает

Streaming снижает perceived latency только для финального ответа пользователю. Промежуточные вызовы LLM (выбор инструмента, анализ результатов) не нужно стримить — пользователь их не видит. Для промежуточных вызовов важна фактическая latency. Её снижают выбором быстрой модели и оптимизацией контекста.

## Параллельное выполнение Tool Calls

LLM может вернуть несколько tool calls за одну итерацию (см. [Глава 03: Инструменты и Function Calling](../03-tools-and-function-calling/README.md)). Если инструменты независимы друг от друга, их можно выполнить параллельно.

Параллельное выполнение **не экономит токены** — LLM всё равно сгенерировал все tool calls за один запрос. Но оно **сокращает wall-clock time** — время от начала до конца итерации.

### Когда безопасно параллелить

| Безопасно | Небезопасно |
|-----------|-------------|
| `check_disk` + `check_memory` — читают разные ресурсы | `create_file` → `write_to_file` — второй зависит от первого |
| `get_logs("app-1")` + `get_logs("app-2")` — разные серверы | `stop_service` → `deploy` → `start_service` — строгий порядок |
| `dns_lookup` + `ping` — независимые проверки | `read_config` → `apply_config` — сначала прочитать, потом применить |

Правило: если результат одного инструмента нужен как вход для другого — выполнять последовательно. Если нет — параллельно.

### Реализация: executeToolsParallel

```go
// ToolResult хранит результат выполнения одного инструмента
type ToolResult struct {
    ToolCallID string
    Content    string
    Err        error
}

// executeToolsParallel запускает все tool calls одновременно.
// Каждый инструмент выполняется в отдельной горутине.
func executeToolsParallel(ctx context.Context, toolCalls []ToolCall) []Message {
    results := make([]ToolResult, len(toolCalls))

    var wg sync.WaitGroup
    for i, tc := range toolCalls {
        wg.Add(1)
        go func(idx int, call ToolCall) {
            defer wg.Done()

            // Таймаут на каждый инструмент отдельно
            toolCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
            defer cancel()

            content, err := executeTool(toolCtx, call)
            results[idx] = ToolResult{
                ToolCallID: call.ID,
                Content:    content,
                Err:        err,
            }
        }(i, tc)
    }
    wg.Wait()

    // Собираем результаты в формате messages
    var messages []Message
    for _, r := range results {
        content := r.Content
        if r.Err != nil {
            content = fmt.Sprintf("error: %v", r.Err)
        }
        messages = append(messages, Message{
            Role:       "tool",
            Content:    content,
            ToolCallID: r.ToolCallID,
        })
    }
    return messages
}
```

Интеграция в agent loop — заменяем последовательное выполнение на параллельное:

```go
for i := 0; i < MaxIterations; i++ {
    resp, err := client.Complete(ctx, Request{Messages: messages, Tools: toolDefs})
    if err != nil {
        return "", err
    }

    msg := resp.Choices[0].Message
    messages = append(messages, msg)

    if len(msg.ToolCalls) == 0 {
        return msg.Content, nil
    }

    // Было: последовательное выполнение
    // for _, tc := range msg.ToolCalls { ... }

    // Стало: параллельное выполнение
    toolMessages := executeToolsParallel(ctx, msg.ToolCalls)
    messages = append(messages, toolMessages...)
}
```

### Сколько экономим по времени

Допустим, агент вызвал 3 инструмента за одну итерацию. Каждый инструмент выполняется ~2 секунды.

- **Последовательно:** 2 + 2 + 2 = 6 секунд
- **Параллельно:** max(2, 2, 2) = 2 секунды

На 8 итерациях с 2-3 tool calls каждая — это разница между 48 секундами и 16 секундами. Пользователь ждёт в 3 раза меньше.

> **Связь с Главой 04:** В [Главе 04: Автономность и Циклы](../04-autonomy-and-loops/README.md) tool calls выполняются последовательно. Параллельное выполнение — естественная оптимизация, когда агент стабильно работает и вы начинаете оптимизировать latency.

## Типовые ошибки

Короткая сводка (подробные разборы — ниже):

| # | Ошибка | Симптом | Решение |
|---|--------|---------|---------|
| 1 | Нет лимитов на токены | Счёт растёт неконтролируемо, запрос по 100k токенов | TokenBudget по Usage провайдера |
| 2 | Нет лимита итераций | Агент зацикливается, 50+ итераций | `MaxIterations` — терминальная граница (гл. 04) |
| 3 | Одна дорогая модель на всё | Служебные вызовы стоят дороже, чем нужно | GLM-5.3 Flash на служебных узлах вне цикла |
| 4 | Смена модели внутри цикла | Prompt cache ломается, префикс оплачивается заново | Одна модель в цикле (разбор ниже) |
| 5 | Свой result-cache вместо prompt cache | Попаданий нет, экономии нет | Неизменяемый system-префикс (Шаг 3, разбор ниже) |
| 6 | Нет таймаутов | Агент зависает на 10+ минут | Timeout на ход и на каждый вызов LLM |

### Разбор 1: смена модели внутри цикла

**Симптом:** доля `cache_read / prompt` в Usage почти нулевая, хотя system-промпт не меняется; latency каждого вызова заметно выше первого.

**Причина:** модель чередуется между итерациями («дешёвая выбирает инструмент, дорогая отвечает»). У каждой модели свой prompt cache — при переключении префикс не переиспользуется и оплачивается по полной ставке. «Экономия 50%» из примеров про stage-роутинг не учитывает ни это, ни потерю качества рассуждения при смене модели посреди хода.

**Решение:** одна модель на весь цикл; лёгкая модель (GLM-5.3 Flash) — только на служебных узлах вне цикла, где истории почти нет и кэш основного хода не задействован.

### Разбор 2: свой result-cache вместо prompt cache

**Симптом:** реализован `map[hash(messages)] → answer`, но hit rate около нуля, экономии нет.

**Причина:** ключ кэша уникален почти всегда — история диалога растёт на каждой итерации (ответ модели, результаты инструментов), хэш меняется, попаданий не бывает.

**Решение:** кэшируйте не ответы, а **префикс** — system-сообщение + блок `tools[]` держите неизменным, историю только дополняйте. Тогда работает кэш провайдера: попадание видно по `CacheReadTokens > 0` в Usage (Шаг 3). Свой map для кэша ответов имеет смысл только для действительно идентичных запросов без истории (например, кэш эмбеддингов в RAG, гл. 06) — для агентского цикла он мёртв.

## Мини-упражнения

### Упражнение 1: Реализуйте проверку бюджета токенов

Добавьте проверку бюджета в свой рантайм из главы 04 (ревизованная lab04-autonomy использует тот же клиент):

```go
func checkTokenBudget(used int, limit int) error {
    // Ваш код здесь
    // Верните ошибку, если превышен лимит
}
```

**Ожидаемый результат:**
- Функция возвращает ошибку, если использовано токенов больше лимита
- Функция возвращает nil, если лимит не превышен

### Упражнение 2: Condense по канону гл. 12/13

Реализуйте condense-суммаризацию по канону: та же модель, тот же кэшируемый префикс, максимум 1 раз за Run:

```go
func condense(ctx context.Context, ep Client, msgs []Message) ([]Message, error) {
    // Ваш код здесь
    // 1. Разделить на head/tail  2. Summary: запрос с тем же префиксом
    // system + tools (гл. 13)  3. [summary as user message] + tail
}
```

**Ожидаемый результат:**
- Запрос суммаризации несёт тот же кэшируемый префикс — head читается из прогретого кэша
- Максимум 1 сжатие за Run; ошибка суммаризации — ход продолжается с несжатой историей (гл. 13)

### Упражнение 3: Метрики prompt cache

Считайте hit rate кэша провайдера по Usage из ответов:

```go
func cacheHitRate(usages []Usage) float64 {
    // Ваш код здесь
    // Сумма CacheReadTokens / сумма PromptTokens по всем вызовам
}
```

**Ожидаемый результат:**
- По неизменяемому system-префиксу hit rate заметно выше нуля
- При смене модели между запросами hit rate падает к нулю

## Критерии сдачи / Чек-лист

**Сдано (готовность к прод):**
- [x] Реализованы бюджеты токенов по Usage провайдера (prompt/completion) с проверкой перед каждым запросом
- [x] Установлен лимит итераций ReAct loop
- [x] Неизменяемый system-префикс (system + tools) — работает prompt cache провайдера, метрики cache_read отслеживаются
- [x] Дешёвая модель (GLM-5.3 Flash) на служебных узлах вне цикла; в цикле — одна модель
- [x] Fallback-цепочка настроена для отказоустойчивости, а не для экономии
- [x] Установлены таймауты для вызовов LLM и agent run

**Не сдано:**
- [ ] Нет лимитов на токены
- [ ] Нет лимита итераций
- [ ] System-префикс меняется между итерациями / модель меняется внутри цикла
- [ ] Бюджет считается по TotalTokens или собственным оценкам вместо Usage провайдера
- [ ] Нет таймаутов

## Связь с другими главами

- **Observability:** Логирование использования токенов — [Глава 19: Observability и Tracing](../19-observability-and-tracing/README.md)
- **Context Optimization:** Управление размером контекста — [Глава 13: Context Engineering](../13-context-engineering/README.md)
- **Agent Loop:** Базовый цикл агента — [Глава 04: Автономность и Циклы](../04-autonomy-and-loops/README.md)


