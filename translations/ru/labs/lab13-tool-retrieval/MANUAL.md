# Методическое пособие: Lab 13 — Tool Retrieval & Pipeline Building

## Зачем это нужно?

В реальных сценариях агентам нужно работать с большим пространством инструментов. В Linux тысячи команд, и они могут комбинироваться в бесконечные пайплайны. Нельзя передать все инструменты модели — это неэффективно и приводит к худшему выбору инструментов.

**Tool RAG (Retrieval Augmented Generation для пространства действий)** решает это:
- Хранение каталога инструментов с метаданными
- Извлечение только релевантных инструментов перед планированием
- Построение пайплайнов для сложных многошаговых задач

### Реальный кейс

**Ситуация:** Агенту нужно провести траблшутинг логов. Пользователь просит: "Найди топ-10 самых частых ошибок"

**Без Tool Retrieval:**
- Агент получает 1000+ инструментов в контексте
- Модель плохо выбирает (слишком много опций)
- Высокая задержка (слишком много токенов)
- Риск галлюцинаций (вызов несуществующих инструментов)

**С Tool Retrieval:**
- Агент ищет в каталоге: "error filter sort"
- Получает 5 релевантных инструментов: `[grep, sort, uniq, head]`
- Строит пайплайн: `grep("ERROR") → sort() → uniq(-c) → head(10)`
- Выполняет пайплайн безопасно

**Разница:** Tool retrieval фильтрует пространство действий, делая агента более эффективным и безопасным.

## Теория простыми словами

### Как работает Tool Retrieval?

1. **Каталог инструментов:** Храним метаданные для каждого инструмента (имя, описание, теги, уровень риска)
2. **Поиск:** Перед планированием ищем в каталоге релевантные инструменты
3. **Фильтрация:** Возвращаем только top-k наиболее релевантных инструментов
4. **Выполнение:** Передаем отфильтрованные инструменты модели, модель строит пайплайн

### Pipeline DSL

Пайплайн — это JSON-структура, описывающая:
- **Steps:** Список вызовов инструментов в последовательности
- **Input/Output:** Как данные передаются между шагами
- **Risk Level:** Оценка безопасности

**Пример пайплайна:**
```json
{
    "steps": [
        {"tool": "grep", "args": {"pattern": "ERROR"}},
        {"tool": "sort", "args": {}},
        {"tool": "head", "args": {"lines": 10}}
    ],
    "risk_level": "safe"
}
```

## Алгоритм выполнения

### Шаг 1: Создание каталога инструментов

```go
type ToolDefinition struct {
    Name        string
    Description string
    Tags        []string  // "filter", "sort", "search"
    RiskLevel   string    // "safe", "moderate", "dangerous"
}

var toolCatalog = []ToolDefinition{
    {
        Name:        "grep",
        Description: "Search for patterns in text. Use for filtering lines.",
        Tags:        []string{"filter", "search", "text"},
        RiskLevel:   "safe",
    },
    {
        Name:        "sort",
        Description: "Sort lines of text alphabetically or numerically.",
        Tags:        []string{"sort", "order", "text"},
        RiskLevel:   "safe",
    },
    {
        Name:        "head",
        Description: "Show first N lines. Use for limiting output.",
        Tags:        []string{"limit", "filter", "text"},
        RiskLevel:   "safe",
    },
    // ... больше инструментов
}
```

### Шаг 2: Реализация поиска инструментов

```go
func searchToolCatalog(query string, topK int) []ToolDefinition {
    var results []ToolDefinition
    queryLower := strings.ToLower(query)
    
    // Простое совпадение ключевых слов (в продакшене - используйте embeddings)
    for _, tool := range toolCatalog {
        // Совпадение в описании
        if strings.Contains(strings.ToLower(tool.Description), queryLower) {
            results = append(results, tool)
            continue
        }
        // Совпадение в тегах
        for _, tag := range tool.Tags {
            if strings.Contains(strings.ToLower(tag), queryLower) {
                results = append(results, tool)
                break
            }
        }
    }
    
    // Возвращаем top-k
    if len(results) > topK {
        return results[:topK]
    }
    return results
}
```

### Шаг 3: Структура пайплайна

```go
type PipelineStep struct {
    Tool string                 `json:"tool"`
    Args map[string]interface{} `json:"args"`
}

type Pipeline struct {
    Steps         []PipelineStep `json:"steps"`
    RiskLevel     string          `json:"risk_level"`
    ExpectedOutput string         `json:"expected_output,omitempty"`
}
```

### Шаг 4: Выполнение пайплайна

```go
func executePipeline(pipelineJSON string, inputData string) (string, error) {
    var pipeline Pipeline
    if err := json.Unmarshal([]byte(pipelineJSON), &pipeline); err != nil {
        return "", err
    }
    
    // Валидация уровня риска
    if pipeline.RiskLevel == "dangerous" {
        return "", fmt.Errorf("dangerous pipeline requires confirmation")
    }
    
    // Выполнение шагов последовательно
    currentData := inputData
    for i, step := range pipeline.Steps {
        result, err := executeToolStep(step.Tool, step.Args, currentData)
        if err != nil {
            return "", fmt.Errorf("step %d (%s) failed: %v", i, step.Tool, err)
        }
        currentData = result
    }
    
    return currentData, nil
}

func executeToolStep(toolName string, args map[string]interface{}, input string) (string, error) {
    switch toolName {
    case "grep":
        pattern := args["pattern"].(string)
        return grepPattern(input, pattern), nil
    case "sort":
        return sortLines(input), nil
    case "head":
        lines := int(args["lines"].(float64))
        return headLines(input, lines), nil
    default:
        return "", fmt.Errorf("unknown tool: %s", toolName)
    }
}
```

### Шаг 5: Интеграция в агента

```go
tools := []openai.Tool{
    {
        Function: &openai.FunctionDefinition{
            Name: "search_tool_catalog",
            Description: "Search tool catalog for relevant tools. Use this BEFORE building pipelines.",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "query": {"type": "string"},
                    "top_k": {"type": "number", "default": 5}
                },
                "required": ["query"]
            }`),
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name: "execute_pipeline",
            Description: "Execute a pipeline of tools. Provide pipeline JSON with steps and risk_level.",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "pipeline": {"type": "string"},
                    "input_data": {"type": "string"}
                },
                "required": ["pipeline", "input_data"]
            }`),
        },
    },
}
```

## Типовые ошибки

### Ошибка 1: Поиск инструментов возвращает слишком много инструментов

**Симптом:** `search_tool_catalog` возвращает 50+ инструментов, что сводит на нет цель.

**Причина:** Поиск слишком широкий или topK слишком большой.

**Решение:**
```go
// ХОРОШО: Ограничить до top 5-10 наиболее релевантных
relevantTools := searchToolCatalog(query, topK=5)
```

### Ошибка 2: Pipeline JSON некорректен

**Симптом:** `execute_pipeline` падает с ошибкой парсинга JSON.

**Причина:** Модель генерирует некорректный JSON или отсутствуют обязательные поля.

**Решение:**
```go
// ХОРОШО: Валидировать JSON перед парсингом
if !json.Valid([]byte(pipelineJSON)) {
    return "", fmt.Errorf("invalid JSON")
}

var pipeline Pipeline
if err := json.Unmarshal([]byte(pipelineJSON), &pipeline); err != nil {
    return "", fmt.Errorf("failed to parse pipeline: %v", err)
}

// Валидировать обязательные поля
if len(pipeline.Steps) == 0 {
    return "", fmt.Errorf("pipeline has no steps")
}
```

### Ошибка 3: Шаги пайплайна не связываются

**Симптом:** Каждый шаг получает оригинальный вход вместо вывода предыдущего шага.

**Причина:** Не передается вывод шага N в шаг N+1.

**Решение:**
```go
// ХОРОШО: Правильно связывать шаги
currentData := inputData
for _, step := range pipeline.Steps {
    result, err := executeToolStep(step.Tool, step.Args, currentData)
    if err != nil {
        return "", err
    }
    currentData = result  // Использовать вывод как следующий вход
}
```

### Ошибка 4: Нет валидации риска

**Симптом:** Опасные пайплайны выполняются без подтверждения.

**Причина:** Не проверяется `risk_level` перед выполнением.

**Решение:**
```go
// ХОРОШО: Валидировать риск перед выполнением
if pipeline.RiskLevel == "dangerous" {
    return "", fmt.Errorf("dangerous pipeline requires human approval")
}
```

## Мини-упражнения

### Упражнение 1: Улучшите поиск инструментов

Реализуйте поиск, который ранжирует инструменты по релевантности (подсчет совпадений в описании + тегах):

```go
func searchToolCatalog(query string, topK int) []ToolDefinition {
    // Оценить каждый инструмент по количеству совпадений
    // Отсортировать по оценке
    // Вернуть top-k
}
```

### Упражнение 2: Добавьте валидацию пайплайна

Реализуйте валидацию, которая проверяет:
- Все инструменты в шагах существуют в каталоге
- Уровень риска установлен
- Шаги не пустые

```go
func validatePipeline(pipeline Pipeline, catalog []ToolDefinition) error {
    // Проверить существование инструментов
    // Проверить уровень риска
    // Проверить, что шаги не пустые
}
```

## Критерии сдачи

✅ **Сдано:**
- Поиск в каталоге инструментов находит релевантные инструменты (top-k)
- Агент строит корректный pipeline JSON
- Пайплайн выполняется с правильной связкой шагов
- Валидация риска работает
- Код компилируется и работает

❌ **Не сдано:**
- Поиск инструментов возвращает все инструменты
- Pipeline JSON некорректен
- Шаги не связываются (каждый получает оригинальный вход)
- Нет валидации риска
- Код не компилируется

---

**Следующий шаг:** После успешного прохождения Lab 13 переходите к прод-темам или [Lab 14: Evals в CI](../lab14-evals-in-ci/README.md) (если доступна)

