# Модель и декодинг

## Зачем это нужно?

Агент работает, но иногда возвращает "ломаный" JSON в tool calls или ведёт себя непредсказуемо. Без правильного выбора модели и настройки декодинга вы не можете гарантировать качество и детерминизм.

### Реальный кейс

**Ситуация:** Агент использует локальную модель для tool calling. Иногда модель возвращает невалидный JSON, и агент падает.

**Проблема:** Модель не подходит для tool calling, нет детерминизма (Temperature > 0), нет JSON mode.

**Решение:** Capability benchmark перед разработкой, детерминизм (Temperature = 0), JSON mode для structured outputs, выбор модели под задачу.

## Теория простыми словами

### Что такое Capability Benchmark?

Capability Benchmark — это набор тестов для проверки модели перед разработкой. Проверяет: JSON generation, Instruction following, Function calling.

### Что такое детерминизм?

Детерминизм — это предсказуемость вывода. При Temperature = 0 модель всегда возвращает одинаковый результат для одинакового входа.

### Что такое JSON mode?

JSON mode — это режим, где модель гарантированно возвращает валидный JSON. Это снижает вероятность "ломаного" JSON в tool calls.

## Как это работает (пошагово)

### Шаг 1: Capability Benchmark

Проверьте модель перед разработкой (см. `labs/lab00-capability-check/main.go`):

```go
func runCapabilityBenchmark(model string) (bool, error) {
    // Проверяем JSON generation
    jsonTest := testJSONGeneration(model)
    
    // Проверяем Instruction following
    instructionTest := testInstructionFollowing(model)
    
    // Проверяем Function calling
    functionTest := testFunctionCalling(model)
    
    return jsonTest && instructionTest && functionTest, nil
}
```

### Шаг 2: Детерминизм

Используйте Temperature = 0 для tool calling:

```go
func createToolCallRequest(messages []openai.ChatCompletionMessage, tools []openai.Tool) openai.ChatCompletionRequest {
    return openai.ChatCompletionRequest{
        Model:       "gpt-4",
        Messages:    messages,
        Tools:       tools,
        Temperature: 0, // Детерминизм для tool calling
    }
}
```

### Шаг 3: JSON Mode

Используйте JSON mode для structured outputs:

```go
func createStructuredRequest(messages []openai.ChatCompletionMessage) openai.ChatCompletionRequest {
    return openai.ChatCompletionRequest{
        Model: "gpt-4-turbo",
        Messages: messages,
        ResponseFormat: &openai.ChatCompletionResponseFormat{
            Type: openai.ChatCompletionResponseFormatTypeJSONObject,
        },
        Temperature: 0,
    }
}
```

### Шаг 4: Выбор модели под задачу

Используйте более дешёвые модели для простых задач:

```go
func selectModel(taskComplexity string) string {
    switch taskComplexity {
    case "simple":
        return openai.GPT3Dot5Turbo // Дешевле и быстрее
    case "complex":
        return openai.GPT4 // Лучше качество, но дороже
    default:
        return openai.GPT3Dot5Turbo
    }
}
```

## Где это встраивать в нашем коде

### Точка интеграции 1: Capability Check

В `labs/lab00-capability-check/main.go` уже есть benchmark. Используйте его перед разработкой.

### Точка интеграции 2: Tool Calling

В `labs/lab02-tools/main.go` установите Temperature = 0:

```go
req := openai.ChatCompletionRequest{
    Model:       openai.GPT3Dot5Turbo,
    Messages:    messages,
    Tools:       tools,
    Temperature: 0, // Детерминизм
}
```

### Точка интеграции 3: SOP и детерминизм

В `labs/lab06-incident/SOLUTION.md` уже используется Temperature = 0 для детерминизма.

## Типовые ошибки

### Ошибка 1: Модель не проверена перед разработкой

**Симптом:** Модель не подходит для tool calling, возвращает невалидный JSON.

**Решение:** Запустите capability benchmark перед разработкой.

### Ошибка 2: Temperature > 0 для tool calling

**Симптом:** Агент ведёт себя непредсказуемо, одинаковые запросы дают разные результаты.

**Решение:** Используйте Temperature = 0 для tool calling.

### Ошибка 3: Нет JSON mode

**Симптом:** Модель возвращает "ломаный" JSON в tool calls.

**Решение:** Используйте JSON mode, если модель поддерживает.

## Критерии сдачи / Чек-лист

✅ **Сдано:**
- Модель проверена через capability benchmark
- Temperature = 0 для tool calling
- JSON mode используется для structured outputs
- Модель выбирается под задачу

❌ **Не сдано:**
- Модель не проверена
- Temperature > 0 для tool calling
- Нет JSON mode

## Связь с другими главами

- **Capability Benchmark:** Проверка модели — [Lab 00: Capability Check](../../labs/lab00-capability-check/METHOD.md)
- **Физика LLM:** Фундаментальные концепции — [Глава 01: Физика LLM](../01-llm-fundamentals/README.md)
- **Function Calling:** Tool calling — [Глава 04: Инструменты и Function Calling](../04-tools-and-function-calling/README.md)
- **Cost Engineering:** Выбор модели для оптимизации стоимости — [Cost & Latency Engineering](cost_latency.md)

---

**Навигация:** [← Multi-Agent в продакшене](multi_agent_in_prod.md) | [Оглавление главы 12](README.md)

