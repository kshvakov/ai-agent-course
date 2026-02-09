# 13. Context Engineering

## Зачем это нужно?

Контекстные окна ограничены. По мере роста диалога вам приходится решать, что оставлять в контексте, а что сжимать или выкидывать. Плохое управление контекстом тратит токены, теряет важное и путает агента.

В этой главе разберём техники управления контекстом: слои, саммаризация, отбор фактов и адаптивная сборка контекста.

### Реальный кейс

**Ситуация:** Долго идущий разговор с агентом. После 50 поворотов контекст — 50K токенов. Новый запрос нуждается в недавней информации, но она похоронена в истории.

**Проблема:**
- Включить всю историю: Превышает лимит контекста, дорого
- Включить только недавнее: Теряет важный контекст из раннего
- Нет стратегии: Агент путается или пропускает критическую информацию

**Решение:** Context engineering: слои контекста (рабочая память, саммари, факты), селективное извлечение релевантного и саммаризация старых частей диалога с сохранением ключевых фактов.

## Теория простыми словами

### Слои контекста

**Рабочая память (недавние повороты):**
- Последние N поворотов разговора
- Всегда включены
- Наиболее релевантны для текущей задачи

**Слой саммари:**
- Саммаризированные старые разговоры
- Сохраняет ключевые факты
- Уменьшает использование токенов

**Слой фактов:**
- Извлечённые важные факты из [долговременной памяти](../12-agent-memory/README.md)
- Предпочтения пользователя, решения, ограничения
- Постоянны между разговорами
- **Примечание:** Хранение и извлечение фактов описано в [Memory](../12-agent-memory/README.md), здесь описывается только их использование в контексте

> **Важно: Контекст как якорь (Anchoring Bias).**
> Если в слой фактов попадают **предпочтения пользователя** ("пользователь считает X", "нам нужен ответ Y") или **гипотезы** без подтверждения, они становятся сильным якорем для модели. Модель может сместить ответ в сторону этих предпочтений, даже если фактические данные указывают на другое.
>
> **Проблема:** Предпочтения и гипотезы, включённые в контекст как факты, могут исказить объективный анализ.
>
> **Решение:** Разделяйте типы записей: **Fact** (проверенные данные), **Preference** (предпочтения пользователя), **Hypothesis** (гипотезы). Включайте предпочтения и гипотезы в контекст только когда это уместно (персонализация), и исключайте их для аналитических задач, требующих объективности.

**Состояние задачи:**
- Прогресс текущей задачи
- Что сделано, что ожидает
- Позволяет возобновление

### Операции с контекстом

1. **Select** — Выбрать, что включить
2. **Summarize** — Сжать старую информацию
3. **Extract** — Извлечь ключевые факты
4. **Layer** — Организовать по важности/свежести

## Как это работает (пошагово)

### Шаг 1: Интерфейс Context Manager

```go
type ContextManager interface {
    AddMessage(msg openai.ChatCompletionMessage) error
    GetContext(maxTokens int) ([]openai.ChatCompletionMessage, error)
    Summarize() error
    ExtractFacts() ([]Fact, error)
}

type Fact struct {
    Key        string
    Value      string
    Source     string // Какой разговор
    Importance int    // 1-10
    Type       string // "fact", "preference", "hypothesis", "constraint"
}
```

### Шаг 2: Слоистый контекст

```go
type LayeredContext struct {
    workingMemory []openai.ChatCompletionMessage // Недавние повороты
    summary       string                          // Саммаризированная история
    facts         []Fact                          // Извлечённые факты
    maxWorking    int                             // Макс поворотов в рабочей памяти
}

func (c *LayeredContext) GetContext(maxTokens int) ([]openai.ChatCompletionMessage, error) {
    var messages []openai.ChatCompletionMessage
    
    // Добавляем системный промпт с фактами
    if len(c.facts) > 0 {
        factsContext := "Важные факты:\n"
        for _, fact := range c.facts {
            factsContext += fmt.Sprintf("- %s: %s\n", fact.Key, fact.Value)
        }
        messages = append(messages, openai.ChatCompletionMessage{
            Role:    "system",
            Content: factsContext,
        })
    }
    
    // Добавляем саммари, если есть
    if c.summary != "" {
        messages = append(messages, openai.ChatCompletionMessage{
            Role:    "system",
            Content: "Саммари предыдущего разговора: " + c.summary,
        })
    }
    
    // Добавляем рабочую память (недавние повороты)
    messages = append(messages, c.workingMemory...)
    
    // Обрезаем, если превышает maxTokens
    return truncateToTokenLimit(messages, maxTokens), nil
}
```

### Шаг 3: Саммаризация

```go
func (c *LayeredContext) Summarize(ctx context.Context, client *openai.Client) error {
    if len(c.workingMemory) <= c.maxWorking {
        return nil // Ещё не нужно саммаризировать
    }
    
    // Получаем старые сообщения для саммаризации
    oldMessages := c.workingMemory[:len(c.workingMemory)-c.maxWorking]
    
    // Создаём промпт для саммаризации
    prompt := "Саммаризируй этот разговор, сохраняя ключевые факты и решения:\n\n"
    for _, msg := range oldMessages {
        prompt += fmt.Sprintf("%s: %s\n", msg.Role, msg.Content)
    }
    
    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: openai.GPT3Dot5Turbo,
        Messages: []openai.ChatCompletionMessage{
            {Role: "system", Content: "Ты агент саммаризации. Извлекай ключевые факты и решения."},
            {Role: "user", Content: prompt},
        },
        Temperature: 0,
    })
    if err != nil {
        return err
    }
    
    c.summary = resp.Choices[0].Message.Content
    
    // Оставляем только недавние сообщения в рабочей памяти
    c.workingMemory = c.workingMemory[len(c.workingMemory)-c.maxWorking:]
    
    return nil
}
```

### Шаг 4: Использование фактов из памяти

**ВАЖНО:** Извлечение и хранение фактов происходит в [Memory](../12-agent-memory/README.md). Здесь мы только используем уже извлечённые факты при сборке контекста.

```go
func (c *LayeredContext) GetContext(maxTokens int, memory Memory, includePreferences bool) ([]openai.ChatCompletionMessage, error) {
    var messages []openai.ChatCompletionMessage
    
    // Получаем факты из памяти (не извлекаем здесь!)
    facts, _ := memory.Retrieve("user_preferences", 10)
    
    // Фильтруем факты по типу в зависимости от задачи
    var filteredFacts []Fact
    for _, fact := range facts {
        if fact.Type == "fact" || fact.Type == "constraint" {
            // Всегда включаем проверенные факты и ограничения
            filteredFacts = append(filteredFacts, fact)
        } else if includePreferences && (fact.Type == "preference" || fact.Type == "hypothesis") {
            // Предпочтения и гипотезы включаем только если это уместно (персонализация)
            filteredFacts = append(filteredFacts, fact)
        }
        // Иначе исключаем предпочтения/гипотезы для объективного анализа
    }
    
    // Добавляем системный промпт с фактами
    if len(filteredFacts) > 0 {
        factsContext := "Важные факты:\n"
        for _, fact := range filteredFacts {
            // Размечаем тип для ясности
            prefix := ""
            if fact.Type == "preference" {
                prefix = "[Предпочтение пользователя] "
            } else if fact.Type == "hypothesis" {
                prefix = "[Гипотеза] "
            }
            factsContext += fmt.Sprintf("- %s%s: %v\n", prefix, fact.Key, fact.Value)
        }
        messages = append(messages, openai.ChatCompletionMessage{
            Role:    "system",
            Content: factsContext,
        })
    }
    
    // Добавляем саммари, если есть
    if c.summary != "" {
        messages = append(messages, openai.ChatCompletionMessage{
            Role:    "system",
            Content: "Саммари предыдущего разговора: " + c.summary,
        })
    }
    
    // Добавляем рабочую память (недавние повороты)
    messages = append(messages, c.workingMemory...)
    
    // Обрезаем, если превышает maxTokens
    return truncateToTokenLimit(messages, maxTokens), nil
}
```

## Token Counting и truncateToTokenLimit

В предыдущих примерах мы вызывали `truncateToTokenLimit`, но не реализовали её. Разберём подсчёт токенов и обрезку контекста.

### Зачем считать токены?

Каждая модель имеет жёсткий лимит контекстного окна. Превысите — получите ошибку. Не добирёте — тратите деньги на пустое место. Точный подсчёт токенов позволяет использовать контекст максимально эффективно.

### Простой подсчёт: слова vs токены

Точный подсчёт требует токенизатора модели (например, `tiktoken` для OpenAI). Но для быстрой оценки подходит приближение: 1 токен ≈ 0.75 слова для английского текста, для русского — ближе к 0.5 слова (кириллица кодируется менее эффективно).

```go
// TokenCounter — интерфейс подсчёта токенов.
// Позволяет подменять реализацию: приближённую для тестов, точную для продакшена.
type TokenCounter interface {
    Count(text string) int
}

// WordBasedCounter — приближённый подсчёт по словам.
// Подходит для быстрой оценки без внешних зависимостей.
type WordBasedCounter struct {
    TokensPerWord float64 // Для английского ≈ 1.33, для русского ≈ 2.0
}

func (c *WordBasedCounter) Count(text string) int {
    words := len(strings.Fields(text))
    return int(float64(words) * c.TokensPerWord)
}

// TiktokenCounter — точный подсчёт через tiktoken.
// Используйте в продакшене для точного бюджетирования.
type TiktokenCounter struct {
    encoding *tiktoken.Encoding
}

func NewTiktokenCounter(model string) (*TiktokenCounter, error) {
    enc, err := tiktoken.EncodingForModel(model)
    if err != nil {
        return nil, fmt.Errorf("encoding for model %s: %w", model, err)
    }
    return &TiktokenCounter{encoding: enc}, nil
}

func (c *TiktokenCounter) Count(text string) int {
    return len(c.encoding.Encode(text, nil, nil))
}
```

### Лимиты моделей

Лимиты контекста зависят от модели. Держите их в конфигурации, а не в коде:

```go
// ModelLimits хранит ограничения конкретной модели.
var ModelLimits = map[string]int{
    "gpt-4o":      128_000,
    "gpt-4o-mini": 128_000,
    "gpt-4-turbo": 128_000,
    "gpt-3.5-turbo": 16_385,
    "claude-3-5-sonnet": 200_000,
}

// SafeLimit возвращает лимит с запасом на ответ модели.
// Оставляем место для генерации (maxOutputTokens).
func SafeLimit(model string, maxOutputTokens int) int {
    limit, ok := ModelLimits[model]
    if !ok {
        return 4096 // Безопасный дефолт
    }
    return limit - maxOutputTokens
}
```

### Реализация truncateToTokenLimit

Обрезаем контекст с конца, но системные сообщения и последний запрос пользователя сохраняем всегда:

```go
func truncateToTokenLimit(
    messages []openai.ChatCompletionMessage,
    maxTokens int,
    counter TokenCounter,
) []openai.ChatCompletionMessage {
    total := countMessages(messages, counter)
    if total <= maxTokens {
        return messages
    }

    // Разделяем: системные сообщения, середина, последнее сообщение пользователя
    var system []openai.ChatCompletionMessage
    var middle []openai.ChatCompletionMessage
    var last openai.ChatCompletionMessage

    for i, msg := range messages {
        if msg.Role == "system" {
            system = append(system, msg)
        } else if i == len(messages)-1 {
            last = msg
        } else {
            middle = append(middle, msg)
        }
    }

    // Считаем фиксированные части (системные + последний запрос)
    reserved := countMessages(system, counter) + counter.Count(last.Content) + 4 // +4 на метаданные

    // Обрезаем середину с начала (удаляем самые старые сообщения)
    budget := maxTokens - reserved
    var kept []openai.ChatCompletionMessage
    runningTotal := 0

    for i := len(middle) - 1; i >= 0; i-- {
        msgTokens := counter.Count(middle[i].Content) + 4
        if runningTotal+msgTokens > budget {
            break
        }
        runningTotal += msgTokens
        kept = append([]openai.ChatCompletionMessage{middle[i]}, kept...)
    }

    result := append(system, kept...)
    result = append(result, last)
    return result
}

func countMessages(messages []openai.ChatCompletionMessage, counter TokenCounter) int {
    total := 0
    for _, msg := range messages {
        total += counter.Count(msg.Content) + 4 // +4 токена на роль и разделители
    }
    return total
}
```

**Почему +4?** Каждое сообщение в API кодируется с метаданными: роль, разделители начала и конца. Для OpenAI это примерно 4 токена на сообщение.

## Продвинутые стратегии сжатия

Базовая саммаризация через LLM — только один из способов сжать контекст. Рассмотрим более точные подходы.

### Семантическое сжатие

Идея: оставляем смысл, выбрасываем «воду». Вместо пересказа всего разговора — извлекаем только то, что влияет на дальнейшие решения.

### Key-Value экстракция

Идея: превращаем длинный нарратив в структурированные пары ключ-значение. Компактнее саммари, проще для модели.

### Реализация

```go
// CompressionStrategy определяет способ сжатия.
type CompressionStrategy string

const (
    StrategySummarize CompressionStrategy = "summarize" // Обычная саммаризация
    StrategySemantic  CompressionStrategy = "semantic"   // Семантическое сжатие
    StrategyKeyValue  CompressionStrategy = "keyvalue"   // Key-Value экстракция
)

// compressContext сжимает сообщения выбранной стратегией.
func compressContext(
    ctx context.Context,
    client *openai.Client,
    messages []openai.ChatCompletionMessage,
    strategy CompressionStrategy,
) (string, error) {
    conversation := formatMessages(messages)

    prompts := map[CompressionStrategy]string{
        StrategySummarize: "Саммаризируй этот разговор. Сохрани ключевые факты и решения:\n\n" + conversation,

        StrategySemantic: `Сожми этот разговор до минимума.
Правила:
- Оставь ТОЛЬКО факты, решения и открытые вопросы
- Убери приветствия, благодарности, повторы
- Убери рассуждения, если есть итоговое решение
- Формат: одно утверждение на строку

Разговор:
` + conversation,

        StrategyKeyValue: `Извлеки ключевые факты из разговора в формате "ключ: значение".
Категории ключей:
- decision: принятое решение
- constraint: ограничение или требование
- action: выполненное действие
- open: нерешённый вопрос

Пример:
decision:database: Используем PostgreSQL
constraint:budget: Не более 100$ в месяц

Разговор:
` + conversation,
    }

    prompt, ok := prompts[strategy]
    if !ok {
        return "", fmt.Errorf("unknown strategy: %s", strategy)
    }

    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: openai.GPT4oMini,
        Messages: []openai.ChatCompletionMessage{
            {Role: "system", Content: "Ты сжимаешь контекст. Будь максимально кратким."},
            {Role: "user", Content: prompt},
        },
        Temperature: 0,
    })
    if err != nil {
        return "", err
    }
    return resp.Choices[0].Message.Content, nil
}

func formatMessages(messages []openai.ChatCompletionMessage) string {
    var b strings.Builder
    for _, msg := range messages {
        fmt.Fprintf(&b, "[%s]: %s\n", msg.Role, msg.Content)
    }
    return b.String()
}
```

### Когда какую стратегию выбирать

| Стратегия | Степень сжатия | Потеря информации | Когда использовать |
|---|---|---|---|
| `summarize` | Средняя (~3x) | Низкая | Нужен контекст для продолжения диалога |
| `semantic` | Высокая (~5-10x) | Средняя | Длинные обсуждения, нужна суть |
| `keyvalue` | Очень высокая (~10-20x) | Высокая (только факты) | Долгосрочное хранение, кросс-сессии |

## Инкрементальная суммаризация

### Проблема

Каждый раз суммаризировать всю историю — дорого. Если в разговоре 100 сообщений и мы суммаризируем каждые 10, к 10-й итерации мы перерабатываем всё заново. Это O(n²) по токенам.

### Решение: обновляем существующее саммари

Вместо суммаризации всей истории берём предыдущее саммари и дополняем его новыми сообщениями. Это O(n) по токенам.

```go
// incrementalSummarize обновляет существующее саммари новыми сообщениями.
// Вместо пересуммаризации всей истории — дополняет текущее саммари.
func incrementalSummarize(
    ctx context.Context,
    client *openai.Client,
    currentSummary string,
    newMessages []openai.ChatCompletionMessage,
) (string, error) {
    if len(newMessages) == 0 {
        return currentSummary, nil
    }

    newConversation := formatMessages(newMessages)

    var prompt string
    if currentSummary == "" {
        // Первая суммаризация
        prompt = "Суммаризируй этот разговор. Сохрани ключевые факты, решения и открытые вопросы:\n\n" + newConversation
    } else {
        // Обновление существующего саммари
        prompt = fmt.Sprintf(`Обнови саммари разговора с учётом новых сообщений.

Текущее саммари:
%s

Новые сообщения:
%s

Правила:
- Включи ВСЮ важную информацию из текущего саммари
- Добавь новые факты и решения из новых сообщений
- Если новые сообщения противоречат саммари — используй новую информацию
- Убери устаревшие пункты, если они были закрыты в новых сообщениях
- Сохрани компактный формат`, currentSummary, newConversation)
    }

    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: openai.GPT4oMini,
        Messages: []openai.ChatCompletionMessage{
            {Role: "system", Content: "Ты обновляешь саммари разговора. Будь точным и кратким."},
            {Role: "user", Content: prompt},
        },
        Temperature: 0,
    })
    if err != nil {
        return currentSummary, err // При ошибке сохраняем старое саммари
    }
    return resp.Choices[0].Message.Content, nil
}
```

### Использование в LayeredContext

```go
func (c *LayeredContext) SummarizeIncremental(ctx context.Context, client *openai.Client) error {
    if len(c.workingMemory) <= c.maxWorking {
        return nil
    }

    // Берём только сообщения, выходящие за рабочую память
    overflow := c.workingMemory[:len(c.workingMemory)-c.maxWorking]

    // Обновляем саммари инкрементально (не пересуммаризируем всё)
    updated, err := incrementalSummarize(ctx, client, c.summary, overflow)
    if err != nil {
        return err
    }

    c.summary = updated
    c.workingMemory = c.workingMemory[len(c.workingMemory)-c.maxWorking:]
    return nil
}
```

**Сравнение затрат:**

| Подход | Токенов на 100-е сообщение | Рост |
|---|---|---|
| Полная суммаризация | ~50K (вся история) | O(n²) |
| Инкрементальная | ~2K (саммари + 10 новых) | O(n) |

## Приоритизация контекста

Когда бюджет токенов ограничен, нужно решить: какие данные важнее. Не всё одинаково ценно — недавние сообщения важнее старых, ошибки важнее успешных результатов.

### Бюджет по слоям

Делим доступные токены между слоями контекста. Фиксированная доля гарантирует, что ни один слой не «съест» весь бюджет:

```go
// TokenBudget распределяет доступные токены между слоями контекста.
type TokenBudget struct {
    Total          int     // Общий бюджет (maxTokens модели - maxOutputTokens)
    SystemRatio    float64 // Доля для системного промпта (0.10-0.15)
    FactsRatio     float64 // Доля для фактов (0.10-0.15)
    SummaryRatio   float64 // Доля для саммари (0.15-0.20)
    WorkingRatio   float64 // Доля для рабочей памяти (0.50-0.65)
}

func (b TokenBudget) SystemBudget() int  { return int(float64(b.Total) * b.SystemRatio) }
func (b TokenBudget) FactsBudget() int   { return int(float64(b.Total) * b.FactsRatio) }
func (b TokenBudget) SummaryBudget() int { return int(float64(b.Total) * b.SummaryRatio) }
func (b TokenBudget) WorkingBudget() int { return int(float64(b.Total) * b.WorkingRatio) }
```

### Скоринг сообщений

Не все сообщения одинаково полезны. Оцениваем важность и отбираем в рамках бюджета:

```go
// ScoredMessage — сообщение с оценкой важности.
type ScoredMessage struct {
    Message    openai.ChatCompletionMessage
    Score      float64
    TokenCount int
}

// scoreMessage оценивает важность сообщения.
// Высокий скор = сообщение нужно сохранить.
func scoreMessage(msg openai.ChatCompletionMessage, position, total int) float64 {
    score := 0.0

    // 1. Свежесть: недавние сообщения важнее (0.0–0.4)
    recency := float64(position) / float64(total)
    score += recency * 0.4

    // 2. Роль: ответы ассистента с tool_calls важнее обычного текста
    if msg.Role == "tool" {
        score += 0.2 // Результаты вызовов инструментов важны
    }

    // 3. Содержание: ошибки и важные решения
    content := strings.ToLower(msg.Content)
    if strings.Contains(content, "error") || strings.Contains(content, "ошибка") {
        score += 0.3 // Ошибки важнее обычных сообщений
    }
    if strings.Contains(content, "решение") || strings.Contains(content, "выбрали") {
        score += 0.2 // Решения важно помнить
    }

    return score
}

// prioritizeContext собирает контекст с учётом бюджета и приоритетов.
func prioritizeContext(
    messages []openai.ChatCompletionMessage,
    facts []Fact,
    summary string,
    budget TokenBudget,
    counter TokenCounter,
) []openai.ChatCompletionMessage {
    var result []openai.ChatCompletionMessage

    // 1. Факты — в рамках бюджета
    if len(facts) > 0 {
        factsText := buildFactsText(facts, budget.FactsBudget(), counter)
        result = append(result, openai.ChatCompletionMessage{
            Role:    "system",
            Content: factsText,
        })
    }

    // 2. Саммари — обрезаем если не влезает
    if summary != "" {
        if counter.Count(summary) > budget.SummaryBudget() {
            // Саммари слишком длинное — обрезаем по предложениям
            summary = truncateText(summary, budget.SummaryBudget(), counter)
        }
        result = append(result, openai.ChatCompletionMessage{
            Role:    "system",
            Content: "Саммари предыдущего разговора:\n" + summary,
        })
    }

    // 3. Рабочая память — отбираем по скорингу
    scored := make([]ScoredMessage, len(messages))
    for i, msg := range messages {
        scored[i] = ScoredMessage{
            Message:    msg,
            Score:      scoreMessage(msg, i, len(messages)),
            TokenCount: counter.Count(msg.Content) + 4,
        }
    }

    // Последнее сообщение пользователя включаем всегда
    workingBudget := budget.WorkingBudget()
    if len(scored) > 0 {
        last := scored[len(scored)-1]
        workingBudget -= last.TokenCount
    }

    // Остальные сообщения — по убыванию скора, пока влезают
    sort.Slice(scored[:len(scored)-1], func(i, j int) bool {
        return scored[i].Score > scored[j].Score
    })

    var selected []ScoredMessage
    used := 0
    for _, sm := range scored[:len(scored)-1] {
        if used+sm.TokenCount > workingBudget {
            continue
        }
        selected = append(selected, sm)
        used += sm.TokenCount
    }

    // Восстанавливаем хронологический порядок
    sort.Slice(selected, func(i, j int) bool {
        return indexOfMessage(messages, selected[i].Message) <
            indexOfMessage(messages, selected[j].Message)
    })

    for _, sm := range selected {
        result = append(result, sm.Message)
    }

    // Последнее сообщение — всегда в конце
    if len(messages) > 0 {
        result = append(result, messages[len(messages)-1])
    }

    return result
}
```

### Пример бюджета

Для модели с 128K контекстом и `maxOutputTokens = 4096`:

| Слой | Доля | Токены |
|---|---|---|
| Системный промпт | 10% | ~12 400 |
| Факты | 10% | ~12 400 |
| Саммари | 20% | ~24 800 |
| Рабочая память | 60% | ~74 300 |
| **Итого на вход** | 100% | **~123 900** |
| Ответ модели | — | 4 096 |

## Типовые ошибки

### Ошибка 1: Нет саммаризации

**Симптом:** Контекст растёт бесконечно, достигая лимитов токенов.

**Причина:** Никогда не саммаризируются старые разговоры.

**Решение:** Реализуйте периодическую саммаризацию, когда рабочая память превышает порог.

### Ошибка 2: Слишком агрессивная саммаризация

**Симптом:** Важные детали потеряны в саммари, агент делает ошибки.

**Причина:** Саммари слишком сжата, факты не извлечены.

**Решение:** Извлекайте факты перед саммаризацией, сохраняйте их отдельно.

### Ошибка 3: Нет отбора фактов

**Симптом:** Включение нерелевантных фактов тратит токены.

**Причина:** Включение всех фактов независимо от релевантности.

**Решение:** Оценивайте факты по важности, включайте только высокооценённые факты.

### Ошибка 4: Предпочтения включены как факты

**Симптом:** Модель смещает ответ в сторону предпочтений пользователя, даже если фактические данные указывают на другое.

**Причина:** Предпочтения пользователя или гипотезы включены в контекст как факты без различения типов.

**Решение:**
```go
// ХОРОШО: Различаем типы
fact := Fact{
    Key:   "user_thinks_db_problem",
    Value: "Пользователь предполагает проблему в БД",
    Type:  "hypothesis", // Не "fact"!
}

// При сборке контекста для аналитической задачи:
if !includePreferences {
    // Исключаем гипотезы и предпочтения
    if fact.Type == "fact" || fact.Type == "constraint" {
        includeInContext(fact)
    }
}
```

**Практика:** Для аналитических задач (инциденты, диагностика) исключайте предпочтения и гипотезы из контекста. Включайте их только для персонализированных ответов (например, рекомендации на основе предпочтений пользователя).

## Мини-упражнения

### Упражнение 1: Реализуйте саммаризацию

Создайте функцию, которая саммаризирует историю разговора:

```go
func summarizeConversation(messages []openai.ChatCompletionMessage) (string, error) {
    // Используйте LLM для создания саммари
}
```

**Ожидаемый результат:**
- Саммари сохраняет ключевые факты
- Значительно уменьшает количество токенов
- Можно восстановить основные моменты

## Критерии сдачи / Чек-лист

**Сдано:**
- [x] Понимаете слои контекста
- [x] Можете саммаризировать разговоры
- [x] Извлекаете и сохраняете факты
- [x] Управляете контекстом в пределах лимитов токенов

**Не сдано:**
- [ ] Нет саммаризации, контекст растёт бесконечно
- [ ] Слишком агрессивная саммаризация, потеря фактов
- [ ] Нет отбора фактов, трата токенов

## Связь с другими главами

- **[Глава 11: State Management](../11-state-management/README.md)** — Состояние задачи используется при сборке контекста
- **[Глава 12: Системы Памяти Агента](../12-agent-memory/README.md)** — Факты из памяти используются в контексте (хранение/извлечение описано там)
- **[Глава 20: Cost & Latency Engineering](../20-cost-latency-engineering/README.md)** — Бюджеты токенов управляют политиками отбора контекста

**ВАЖНО:** Context Engineering фокусируется на **сборке контекста** из различных источников (память, состояние, retrieval). Хранение данных описано в соответствующих главах (Memory, State Management, RAG).

## Что дальше?

После освоения context engineering переходите к:
- **[14. Экосистема и Фреймворки](../14-ecosystem-and-frameworks/README.md)** — Узнайте о фреймворках для агентов


