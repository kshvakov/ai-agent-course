# 06. RAG и База Знаний

## Зачем это нужно?

Обычный агент знает только то, чему его научили при тренировке (до даты cut-off). Ваши локальные инструкции вроде "как перезагружать сервер Phoenix по регламенту №5" он сам по себе не знает.

**RAG (Retrieval Augmented Generation)** — это способ "подглядеть в шпаргалку". Агент сначала находит нужный кусок в базе знаний, а уже потом действует.

Без RAG агент не сможет опираться на вашу документацию, регламенты и базу знаний. С RAG — сможет: найдёт нужное и выполнит шаги по вашим правилам.

### Реальный кейс

**Ситуация:** Пользователь пишет: "Перезагрузи сервер Phoenix согласно регламенту"

**Проблема:** Агент не знает регламента перезагрузки сервера Phoenix. Он может выполнить стандартную перезагрузку, которая не соответствует вашим процедурам.

**Решение:** С RAG агент сначала находит регламент в базе знаний и только потом действует. Он достаёт документ "Регламент перезагрузки сервера Phoenix: 1. Выключить балансировщик 2. Перезагрузить сервер 3. Включить балансировщик" и следует шагам.

## Теория простыми словами

### Как работает RAG?

1. **Агент получает запрос** от пользователя
2. **Агент ищет информацию** в базе знаний через инструмент поиска
3. **База знаний возвращает** релевантные документы
4. **Агент использует информацию** для выполнения действия

## Как работает RAG? — Магия vs Реальность

**❌ Магия:**
> Агент "знает", что нужно поискать в базе знаний и сам находит нужную информацию

**✅ Реальность:**

### Полный протокол RAG

**Шаг 1: Запрос пользователя**

```go
messages := []openai.ChatCompletionMessage{
    {Role: "system", Content: "You are a DevOps assistant. Always search knowledge base before actions."},
    {Role: "user", Content: "Перезагрузи сервер Phoenix согласно регламенту"},
}
```

**Шаг 2: Модель видит описание инструмента поиска**

```go
tools := []openai.Tool{
    {
        Function: &openai.FunctionDefinition{
            Name: "search_knowledge_base",
            Description: "Search the knowledge base for documentation, protocols, and procedures. Use this BEFORE performing actions that require specific procedures.",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "query": {"type": "string", "description": "Search query"}
                },
                "required": ["query"]
            }`),
        },
    },
    {
        Function: &openai.FunctionDefinition{
            Name: "restart_server",
            Description: "Restart a server",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "hostname": {"type": "string"}
                },
                "required": ["hostname"]
            }`),
        },
    },
}
```

**Шаг 3: Модель генерирует tool call для поиска**

```go
resp1, _ := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
    Model:    openai.GPT3Dot5Turbo,
    Messages: messages,
    Tools:    tools,
})

msg1 := resp1.Choices[0].Message
// msg1.ToolCalls = [{
//     Function: {
//         Name: "search_knowledge_base",
//         Arguments: "{\"query\": \"Phoenix restart protocol\"}"
//     }
// }]
```

**Почему модель сгенерировала tool_call на поиск?**
- System Prompt говорит: "Always search knowledge base before actions"
- Description инструмента говорит: "Use this BEFORE performing actions"
- Модель видит слово "регламенту" в запросе и связывает это с инструментом поиска

**Шаг 4: Runtime (ваш код) выполняет поиск**

> **Примечание:** Runtime — это код агента, который вы пишете на Go. См. [Главу 00: Предисловие](../00-preface/README.md#runtime-среда-выполнения) для определения.

```go
func searchKnowledgeBase(query string) string {
    // Простой поиск по ключевым словам (в продакшене - векторный поиск)
    knowledgeBase := map[string]string{
        "protocols.txt": "Регламент перезагрузки сервера Phoenix:\n1. Выключить балансировщик\n2. Перезагрузить сервер\n3. Включить балансировщик",
    }
    
    for filename, content := range knowledgeBase {
        if strings.Contains(strings.ToLower(content), strings.ToLower(query)) {
            return fmt.Sprintf("File: %s\nContent: %s", filename, content)
        }
    }
    return "No documents found"
}

result1 := searchKnowledgeBase("Phoenix restart protocol")
// result1 = "File: protocols.txt\nContent: Регламент перезагрузки сервера Phoenix:\n1. Выключить балансировщик..."
```

**Шаг 5: Результат поиска добавляется в контекст**

```go
messages = append(messages, openai.ChatCompletionMessage{
    Role:       "tool",
    Content:    result1,  // Вся найденная документация!
    ToolCallID: msg1.ToolCalls[0].ID,
})
// Теперь messages содержит:
// [system, user, assistant(tool_call: search_kb), tool("File: protocols.txt\nContent: ...")]
```

**Шаг 6: Модель видит документацию и действует**

```go
// Отправляем обновленную историю (с документацией!) в модель
resp2, _ := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
    Model:    openai.GPT3Dot5Turbo,
    Messages: messages,  // Модель видит найденную документацию!
    Tools:    tools,
})

msg2 := resp2.Choices[0].Message
// Модель видит в контексте:
// "Регламент перезагрузки сервера Phoenix:\n1. Выключить балансировщик..."
// Модель генерирует tool calls согласно регламенту:

// msg2.ToolCalls = [
//     {Function: {Name: "restart_server", Arguments: "{\"hostname\": \"phoenix\"}"}},
//     // Или сначала выключить балансировщик, потом сервер
// ]
```

**Что происходит на деле:**

1. **Модель не "знает" регламент** — она видит его в контексте после поиска
2. **Поиск — это обычный tool** — такой же, как `ping` или `restart_service`
3. **Результат поиска добавляется в `messages[]`** — модель видит его как новое сообщение
4. **Модель генерирует действия на основе контекста** — она видит документацию и следует ей

**Суть:** RAG — это не "знание из воздуха", а способ добавить релевантную информацию в контекст модели через обычный tool call.

## Простой RAG vs Векторный поиск

В этой лабе мы реализуем **простой RAG** (поиск по ключевым словам). В продакшене используется **векторный поиск** (Semantic Search), который ищет по смыслу, а не по словам.

**Простой RAG (Lab 07):**
```go
// Поиск по вхождению подстроки
if strings.Contains(content, query) {
    return content
}
```

**Векторный поиск (продакшен):**
```go
// 1. Документы разбиваются на чанки и преобразуются в векторы (embeddings)
chunks := []Chunk{
    {ID: "chunk_1", Text: "Регламент перезагрузки Phoenix...", Embedding: [1536]float32{...}},
    {ID: "chunk_2", Text: "Шаг 2: Выключить балансировщик...", Embedding: [1536]float32{...}},
}

// 2. Запрос пользователя тоже преобразуется в вектор
queryEmbedding := embedQuery("Phoenix restart protocol")  // [1536]float32{...}

// 3. Поиск похожих векторов по косинусному расстоянию
similarDocs := vectorDB.Search(queryEmbedding, topK=3)
// Возвращает 3 наиболее похожих чанка по смыслу (не по словам!)

// 4. Результат добавляется в контекст модели так же, как в простом RAG
result := formatChunks(similarDocs)  // "Chunk 1: ...\nChunk 2: ...\nChunk 3: ..."
messages = append(messages, openai.ChatCompletionMessage{
    Role:    "tool",
    Content: result,
})
```

**Почему векторный поиск лучше:**
- Ищет по **смыслу**, а не по словам
- Найдет "restart Phoenix" даже если в документе написано "перезагрузка сервера Phoenix"
- Работает с синонимами и разными формулировками

## Чанкинг (Chunking)

Документы разбиваются на чанки (куски) для эффективного поиска.

**Пример:**
```
Документ: "Регламент перезагрузки сервера Phoenix..."
Чанк 1: "Регламент перезагрузки сервера Phoenix: шаг 1..."
Чанк 2: "Шаг 2: Выключить балансировщик..."
Чанк 3: "Шаг 3: Перезагрузить сервер..."
```

## RAG для пространства действий (Tool Retrieval)

До сих пор мы говорили о RAG для **документов** (регламенты, инструкции). Но RAG нужен и для **пространства действий** — когда у агента потенциально бесконечное количество инструментов.

### Проблема: "Бесконечные" инструменты

**Ситуация:** Агент должен работать с Linux-командами для траблшутинга. В Linux тысячи команд (`grep`, `awk`, `sed`, `jq`, `sort`, `uniq`, `head`, `tail` и т.д.), и они комбинируются в пайплайны.

**Проблема:**
- Нельзя передать все команды в `tools[]` — это тысячи токенов
- Модель хуже выбирает из большого списка (больше галлюцинаций)
- Задержка растет (больше токенов = медленнее)
- Нет контроля безопасности (какие команды опасны?)

**Наивное решение (не работает):**
```go
// ПЛОХО: Один универсальный инструмент для всего
tools := []openai.Tool{
    {
        Function: &openai.FunctionDefinition{
            Name: "run_shell",
            Description: "Execute any shell command",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "command": {"type": "string"}
                }
            }`),
        },
    },
}
```

**Почему это плохо:**
- Нет валидации (можно выполнить `rm -rf /`)
- Нет аудита (непонятно, какие команды использовались)
- Нет контроля (модель может вызвать что угодно)

### Решение: Tool RAG (Action-Space Retrieval)

**Идея:** Храним **каталог инструментов** и перед планированием извлекаем только релевантные.

**Как это работает:**

1. **Каталог инструментов** хранит метаданные каждого инструмента:
   - Имя и описание
   - Теги/категории (например, "text-processing", "network", "filesystem")
   - Параметры и их типы
   - Уровень риска (safe/moderate/dangerous)
   - Примеры использования

2. **Перед планированием** агент ищет релевантные инструменты:
   - По запросу пользователя ("найди ошибки в логах")
   - Извлекает top-k инструментов (например, `grep`, `tail`, `jq`)
   - Добавляет только их схемы в `tools[]`

3. **Для пайплайнов** используем двухуровневый контракт:
   - **JSON DSL** описывает план пайплайна (steps, stdin/stdout, ожидания)
   - **Runtime** маппит DSL в tool calls или выполняет через один `execute_pipeline`

### Пример: Tool RAG для Linux-команд

**Шаг 1: Каталог инструментов**

```go
type ToolDefinition struct {
    Name        string
    Description string
    Tags        []string  // "text-processing", "filtering", "sorting"
    RiskLevel   string    // "safe", "moderate", "dangerous"
    Schema      json.RawMessage
}

var toolCatalog = []ToolDefinition{
    {
        Name:        "grep",
        Description: "Search for patterns in text. Use for filtering lines matching a pattern.",
        Tags:        []string{"text-processing", "filtering", "search"},
        RiskLevel:   "safe",
        Schema: json.RawMessage(`{
            "type": "object",
            "properties": {
                "pattern": {"type": "string"},
                "input": {"type": "string"}
            }
        }`),
    },
    {
        Name:        "sort",
        Description: "Sort lines of text. Use for ordering output.",
        Tags:        []string{"text-processing", "sorting"},
        RiskLevel:   "safe",
        Schema: json.RawMessage(`{
            "type": "object",
            "properties": {
                "input": {"type": "string"}
            }
        }`),
    },
    {
        Name:        "head",
        Description: "Show first N lines. Use for limiting output.",
        Tags:        []string{"text-processing", "filtering"},
        RiskLevel:   "safe",
        Schema: json.RawMessage(`{
            "type": "object",
            "properties": {
                "lines": {"type": "number"},
                "input": {"type": "string"}
            }
        }`),
    },
    // ... еще сотни инструментов
}
```

**Шаг 2: Поиск релевантных инструментов**

Для поиска инструментов можно использовать два подхода: простой поиск по ключевым словам (для обучения) и векторный поиск (для продакшена).

**Простой поиск (Lab 13):**
```go
func searchToolCatalog(query string, topK int) []ToolDefinition {
    // Простой поиск по описанию и тегам
    var results []ToolDefinition
    
    queryLower := strings.ToLower(query)
    for _, tool := range toolCatalog {
        // Ищем по описанию
        if strings.Contains(strings.ToLower(tool.Description), queryLower) {
            results = append(results, tool)
            continue
        }
        // Ищем по тегам
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

**Векторный поиск (продакшен):**
```go
// 1. Инструменты преобразуются в векторы (embeddings)
toolEmbeddings := []ToolEmbedding{
    {
        Tool:      toolCatalog[0], // grep
        Embedding: embedText("Search for patterns in text. Use for filtering lines matching a pattern."), // [1536]float32{...}
    },
    {
        Tool:      toolCatalog[1], // sort
        Embedding: embedText("Sort lines of text. Use for ordering output."), // [1536]float32{...}
    },
    // ... все инструменты
}

// 2. Запрос пользователя тоже преобразуется в вектор
queryEmbedding := embedQuery("find errors in logs")  // [1536]float32{...}

// 3. Поиск похожих векторов по косинусному расстоянию
similarTools := vectorDB.Search(queryEmbedding, topK=5)
// Возвращает 5 наиболее похожих инструментов по смыслу (не по словам!)

// 4. Результат используется так же, как в простом поиске
relevantTools := extractTools(similarTools)  // [grep, tail, jq, ...]
```

**Почему векторный поиск лучше для инструментов:**
- Ищет по **смыслу**, а не по словам
- Найдет `grep` даже если запрос "фильтровать строки по паттерну" (без слова "grep")
- Работает с синонимами и разными формулировками
- Особенно важен для больших каталогов (1000+ инструментов)

**Пример использования:**
```go
userQuery := "найди ошибки в логах"
relevantTools := searchToolCatalog("error log filter", 5)
// Возвращает: [grep, tail, jq, ...] - только релевантные!
```

**Шаг 3: Добавляем только релевантные tools в контекст**

```go
// Вместо передачи всех 1000+ инструментов
relevantTools := searchToolCatalog(userQuery, 5)

// Преобразуем в формат OpenAI
tools := make([]openai.Tool, 0, len(relevantTools))
for _, toolDef := range relevantTools {
    tools = append(tools, openai.Tool{
        Type: openai.ToolTypeFunction,
        Function: &openai.FunctionDefinition{
            Name:        toolDef.Name,
            Description: toolDef.Description,
            Parameters:  toolDef.Schema,
        },
    })
}

// Теперь tools содержит только 5 релевантных инструментов вместо 1000+
```

### Пайплайны: JSON DSL + Runtime

Для сложных задач (например, "найди топ-10 ошибок в логах") агент должен строить **пайплайны** из нескольких команд.

**Подход 1: JSON DSL пайплайна**

Агент генерирует формализованный план пайплайна:

```go
type PipelineStep struct {
    Tool    string                 `json:"tool"`
    Args    map[string]interface{} `json:"args"`
    Input   string                 `json:"input,omitempty"`  // stdin от предыдущего шага
    Output  string                 `json:"output,omitempty"` // ожидаемый формат
}

type Pipeline struct {
    Steps         []PipelineStep `json:"steps"`
    ExpectedOutput string        `json:"expected_output"`
    RiskLevel     string         `json:"risk_level"` // "safe", "moderate", "dangerous"
}

// Пример: Агент генерирует такой JSON
pipelineJSON := `{
    "steps": [
        {
            "tool": "grep",
            "args": {"pattern": "ERROR"},
            "input": "logs.txt"
        },
        {
            "tool": "sort",
            "args": {},
            "input": "{{step_0.output}}"
        },
        {
            "tool": "head",
            "args": {"lines": 10},
            "input": "{{step_1.output}}"
        }
    ],
    "expected_output": "Top 10 error lines, sorted",
    "risk_level": "safe"
}`
```

**Подход 2: Runtime выполняет пайплайн**

```go
func executePipeline(pipelineJSON string, inputData string) (string, error) {
    var pipeline Pipeline
    if err := json.Unmarshal([]byte(pipelineJSON), &pipeline); err != nil {
        return "", err
    }
    
    // Валидация: проверяем риск
    if pipeline.RiskLevel == "dangerous" {
        return "", fmt.Errorf("dangerous pipeline requires confirmation")
    }
    
    // Выполняем шаги последовательно
    currentInput := inputData
    for i, step := range pipeline.Steps {
        // Подставляем результат предыдущего шага
        if strings.Contains(step.Input, "{{step_") {
            step.Input = currentInput
        }
        
        // Выполняем шаг (в реальности - вызов соответствующего инструмента)
        result, err := executeToolStep(step.Tool, step.Args, step.Input)
        if err != nil {
            return "", fmt.Errorf("step %d failed: %v", i, err)
        }
        
        currentInput = result
    }
    
    return currentInput, nil
}
```

**Подход 3: Инструмент `execute_pipeline`**

Агент вызывает один инструмент с JSON пайплайна:

```go
tools := []openai.Tool{
    {
        Function: &openai.FunctionDefinition{
            Name: "execute_pipeline",
            Description: "Execute a pipeline of tools. Provide pipeline JSON with steps, expected output, and risk level.",
            Parameters: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "pipeline": {"type": "string", "description": "JSON pipeline definition"},
                    "input_data": {"type": "string", "description": "Input data (e.g., log file content)"}
                },
                "required": ["pipeline", "input_data"]
            }`),
        },
    },
}

// Агент генерирует tool call:
// execute_pipeline({
//     "pipeline": "{\"steps\":[...], \"risk_level\": \"safe\"}",
//     "input_data": "log content here"
// })
```

### Практические паттерны

**Tool Discovery через Tool Servers:**

В production инструменты часто предоставляются через [Tool Servers](../18-tool-protocols-and-servers/README.md). Каталог можно получать динамически:

```go
// Tool Server предоставляет ListTools()
toolServer := connectToToolServer("http://localhost:8080")
allTools, _ := toolServer.ListTools()

// Фильтруем по задаче
relevantTools := filterToolsByQuery(allTools, userQuery, topK=5)
```

**Валидация и безопасность:**

```go
func validatePipeline(pipeline Pipeline) error {
    // Проверяем риск
    if pipeline.RiskLevel == "dangerous" {
        return fmt.Errorf("dangerous pipeline requires human approval")
    }
    
    // Проверяем allowlist инструментов
    allowedTools := map[string]bool{
        "grep": true, "sort": true, "head": true,
        // rm, dd и другие опасные - НЕ в allowlist
    }
    
    for _, step := range pipeline.Steps {
        if !allowedTools[step.Tool] {
            return fmt.Errorf("tool %s not allowed", step.Tool)
        }
    }
    
    return nil
}
```

**Наблюдаемость:**

```go
// Логируем выбранные инструменты и причины
log.Printf("Tool retrieval: query=%s, selected=%v, reason=%s",
    userQuery,
    []string{"grep", "sort", "head"},
    "matched tags: text-processing, filtering")

// Сохраняем pipeline JSON для аудита
auditLog.StorePipeline(userID, pipelineJSON, result)
```

## Типовые ошибки

### Ошибка 1: Агент не ищет в базе знаний

**Симптом:** Агент выполняет действия без поиска в базе знаний, используя только общие знания.

**Причина:** System Prompt не инструктирует агента искать в базе знаний, или описание инструмента поиска недостаточно четкое.

**Решение:**
```go
// ХОРОШО: System Prompt требует поиск
systemPrompt := `... Always search knowledge base before performing actions that require specific procedures.`

// ХОРОШО: Четкое описание инструмента
Description: "Search the knowledge base for documentation, protocols, and procedures. Use this BEFORE performing actions that require specific procedures."
```

### Ошибка 2: Плохой поисковый запрос

**Симптом:** Агент не находит нужную информацию в базе знаний.

**Причина:** Поисковый запрос слишком общий или не содержит ключевых слов из документа.

**Решение:**
```go
// ПЛОХО: Слишком общий запрос
query := "server"

// ХОРОШО: Конкретный запрос с ключевыми словами
query := "Phoenix server restart protocol"
```

### Ошибка 3: Чанки слишком большие

**Симптом:** Поиск возвращает слишком большие документы, которые не влезают в контекст.

**Причина:** Размер чанков слишком большой (больше контекстного окна).

**Решение:**
```go
// ХОРОШО: Размер чанка ~500-1000 токенов
chunkSize := 500  // Токенов
```

### Ошибка 4: Передача всех инструментов в контекст

**Симптом:** Агент получает список из 1000+ инструментов, модель хуже выбирает, растет задержка.

**Причина:** Все инструменты передаются в `tools[]` без фильтрации.

**Решение:**
```go
// ПЛОХО: Все инструменты
tools := getAllTools()  // 1000+ инструментов

// ХОРОШО: Только релевантные
userQuery := "найди ошибки в логах"
relevantTools := searchToolCatalog(userQuery, topK=5)  // Только 5 релевантных
tools := convertToOpenAITools(relevantTools)
```

### Ошибка 5: Универсальный `run_shell` без контроля

**Симптом:** Агент использует один инструмент `run_shell(command)` для всех команд. Нет валидации, нет аудита.

**Причина:** Упрощение архитектуры за счет безопасности.

**Решение:**
```go
// ПЛОХО: Универсальный shell
tools := []openai.Tool{{
    Function: &openai.FunctionDefinition{
        Name: "run_shell",
        Description: "Execute any shell command",
    },
}}

// ХОРОШО: Конкретные инструменты + pipeline DSL
tools := []openai.Tool{
    {Function: &openai.FunctionDefinition{Name: "grep", ...}},
    {Function: &openai.FunctionDefinition{Name: "sort", ...}},
    {Function: &openai.FunctionDefinition{Name: "execute_pipeline", ...}},
}

// Pipeline JSON валидируется перед выполнением
if err := validatePipeline(pipeline); err != nil {
    return err
}
```

### Ошибка 6: Нет валидации пайплайна

**Симптом:** Агент генерирует пайплайн с опасными командами (`rm -rf`, `dd`), которые выполняются без проверки.

**Причина:** Нет проверки risk level и allowlist перед выполнением.

**Решение:**
```go
// ХОРОШО: Валидация перед выполнением
func executePipeline(pipelineJSON string) error {
    var pipeline Pipeline
    json.Unmarshal([]byte(pipelineJSON), &pipeline)
    
    // Проверяем риск
    if pipeline.RiskLevel == "dangerous" {
        return fmt.Errorf("dangerous pipeline requires confirmation")
    }
    
    // Проверяем allowlist
    allowedTools := map[string]bool{"grep": true, "sort": true}
    for _, step := range pipeline.Steps {
        if !allowedTools[step.Tool] {
            return fmt.Errorf("tool %s not allowed", step.Tool)
        }
    }
    
    // Выполняем только после валидации
    return runValidatedPipeline(pipeline)
}
```

## Мини-упражнения

### Упражнение 1: Реализуйте простой поиск

Реализуйте функцию простого поиска по ключевым словам:

```go
func searchKnowledgeBase(query string) string {
    // Простой поиск по ключевым словам
    // Верните релевантные документы
}
```

**Ожидаемый результат:**
- Функция находит документы, содержащие ключевые слова из запроса
- Функция возвращает первые N релевантных документов

### Упражнение 2: Реализуйте чанкинг

Реализуйте функцию разбиения документа на чанки:

```go
func chunkDocument(text string, chunkSize int) []string {
    // Разбейте документ на чанки размером chunkSize токенов
    // Верните список чанков
}
```

**Ожидаемый результат:**
- Функция разбивает документ на чанки заданного размера
- Чанки не перекрываются (или перекрываются минимально)

### Упражнение 3: Реализуйте поиск инструментов

Реализуйте функцию поиска релевантных инструментов в каталоге:

```go
func searchToolCatalog(query string, catalog []ToolDefinition, topK int) []ToolDefinition {
    // Ищите по описанию и тегам
    // Верните top-k наиболее релевантных инструментов
}
```

**Ожидаемый результат:**
- Функция находит инструменты, релевантные запросу
- Возвращает не более topK инструментов
- Учитывает описание и теги инструментов

**Для продвинутых:** Реализуйте векторный поиск для инструментов (аналогично векторному поиску документов выше). Это особенно полезно для больших каталогов (1000+ инструментов).

### Упражнение 4: Валидация пайплайна

Реализуйте функцию валидации пайплайна перед выполнением:

```go
func validatePipeline(pipeline Pipeline, allowedTools map[string]bool) error {
    // Проверьте risk level
    // Проверьте allowlist инструментов
    // Верните ошибку, если пайплайн небезопасен
}
```

**Ожидаемый результат:**
- Функция возвращает ошибку для dangerous пайплайнов
- Функция возвращает ошибку, если используются неразрешенные инструменты
- Функция возвращает nil для безопасных пайплайнов

## Критерии сдачи / Чек-лист

✅ **Сдано:**
- Агент ищет в базе знаний перед выполнением действий
- Поисковые запросы конкретные и содержат ключевые слова
- Документы разбиты на чанки подходящего размера
- Инструмент поиска имеет четкое описание
- System Prompt инструктирует агента использовать базу знаний
- Для большого пространства инструментов используется tool retrieval (только релевантные tools в контексте)
- Пайплайны валидируются перед выполнением (risk level, allowlist)
- Опасные операции требуют подтверждения

❌ **Не сдано:**
- Агент не ищет в базе знаний (использует только общие знания)
- Поисковые запросы слишком общие (не находит нужную информацию)
- Чанки слишком большие (не влезают в контекст)
- Все инструменты передаются в контекст без фильтрации (1000+ tools)
- Используется универсальный `run_shell` без контроля безопасности
- Пайплайны выполняются без валидации

## Прод-заметки

При использовании RAG в продакшене учитывайте:

- **Версионирование документов:** Отслеживайте версии документов и дату обновления (`updated_at`). Это помогает понять, какая версия использовалась в ответе.
- **Freshness (актуальность):** Фильтруйте устаревшие документы (например, старше 30 дней) перед использованием в контексте.
- **Grounding:** Требуйте от агента ссылаться на найденные документы в ответе. Это снижает галлюцинации и повышает доверие.

Подробнее о прод-готовности: [Глава 19: Observability](../19-observability-and-tracing/README.md), [Глава 23: Evals в CI/CD](../23-evals-in-cicd/README.md).

## Связь с другими главами

- **Инструменты:** Как инструмент поиска интегрируется в агента, см. [Главу 03: Инструменты](../03-tools-and-function-calling/README.md). Проблема большого списка инструментов решается через tool retrieval (см. раздел "RAG для пространства действий" выше).
- **Автономность:** Как RAG работает в цикле агента, см. [Главу 04: Автономность](../04-autonomy-and-loops/README.md)
- **Tool Servers:** Как получать каталог инструментов динамически через tool servers, см. [Главу 18: Протоколы Инструментов](../18-tool-protocols-and-servers/README.md)

## Что дальше?

После изучения RAG переходите к:
- **[07. Multi-Agent Systems](../07-multi-agent/README.md)** — как создать команду агентов

---

**Навигация:** [← Безопасность](../05-safety-and-hitl/README.md) | [Оглавление](../README.md) | [Multi-Agent →](../07-multi-agent/README.md)

