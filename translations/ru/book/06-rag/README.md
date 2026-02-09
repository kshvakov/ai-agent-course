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

**Магия:**
> Агент "знает", что нужно поискать в базе знаний и сам находит нужную информацию

**Реальность:**

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
    Model:    "gpt-4o-mini",
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
    Model:    "gpt-4o-mini",
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

## Продвинутые техники RAG

Базовый RAG работает так: запрос пользователя → поиск → найденные документы → ответ. Но на практике этого недостаточно. Запрос может быть нечётким, поиск — неточным, а результаты — нерелевантными.

Рассмотрим техники, которые решают эти проблемы.

### Эволюция RAG

```
Basic RAG          Advanced RAG           Agentic RAG
┌──────────┐       ┌──────────────┐       ┌─────────────────┐
│ Query    │       │ Query        │       │ Agent решает:   │
│   ↓      │       │   ↓          │       │ - Искать или нет│
│ Search   │       │ Transform    │       │ - Где искать    │
│   ↓      │       │   ↓          │       │ - Достаточно ли │
│ Retrieve │       │ Route        │       │ - Искать ещё    │
│   ↓      │       │   ↓          │       │   ↓             │
│ Generate │       │ Hybrid Search│       │ Итеративный     │
│          │       │   ↓          │       │ поиск в цикле   │
│          │       │ Rerank       │       │                 │
│          │       │   ↓          │       │                 │
│          │       │ Generate     │       │                 │
└──────────┘       └──────────────┘       └─────────────────┘
```

### Query Transformation (Трансформация запроса)

**Проблема:** Пользователь пишет "сервер лежит". Поиск по этому запросу не найдёт документ "Процедура восстановления после отказа сервера".

**Решение:** Перед поиском трансформируем запрос — переформулируем, расширяем или разбиваем на под-запросы.

**Техника 1: Переформулировка (Query Rewriting)**

```go
// Инструмент для переформулировки запроса перед поиском
func rewriteQuery(originalQuery string, client *openai.Client) (string, error) {
    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: "gpt-4o-mini", // Дешёвая модель — задача простая
        Messages: []openai.ChatCompletionMessage{
            {
                Role: openai.ChatMessageRoleSystem,
                Content: `Rewrite the user query to be more specific for document search.
Return ONLY the rewritten query, nothing else.
Examples:
- "сервер лежит" → "процедура восстановления сервера после отказа"
- "медленно работает БД" → "диагностика производительности PostgreSQL высокая latency"`,
            },
            {Role: openai.ChatMessageRoleUser, Content: originalQuery},
        },
        Temperature: 0,
    })
    if err != nil {
        return originalQuery, err // Fallback на оригинал
    }
    return resp.Choices[0].Message.Content, nil
}
```

**Техника 2: Декомпозиция на под-запросы (Sub-Query Decomposition)**

Сложный запрос разбивается на несколько простых. Результаты объединяются.

```go
// "Сравни настройки nginx на prod и staging" разбивается на:
// 1. "настройки nginx production"
// 2. "настройки nginx staging"
subQueries := decomposeQuery(userQuery)
var allResults []SearchResult
for _, sq := range subQueries {
    results := searchKnowledgeBase(sq)
    allResults = append(allResults, results...)
}
```

**Техника 3: HyDE (Hypothetical Document Embeddings)**

Вместо поиска по запросу, просим модель сгенерировать **гипотетический ответ**. Затем ищем документы, похожие на этот ответ.

```go
// Шаг 1: Модель генерирует гипотетический документ
hypothetical := generateHypotheticalAnswer(query)
// query: "как настроить SSL"
// hypothetical: "Для настройки SSL на nginx: 1. Получите сертификат... 2. Добавьте в конфиг..."

// Шаг 2: Ищем документы, похожие на гипотетический ответ
embedding := embedText(hypothetical)
results := vectorDB.Search(embedding, topK=5)
// Находит реальные документы о настройке SSL, даже если они написаны другими словами
```

**Зачем это нужно:** Embedding гипотетического документа ближе к embedding реального документа, чем embedding короткого запроса.

### Routing (Маршрутизация запросов)

**Проблема:** У вас несколько источников данных: вики, SQL-база, API мониторинга. Запрос "время отклика сервера за последний час" не найдётся в вики — нужно идти в базу метрик.

**Решение:** Классифицируем запрос и направляем в нужный источник.

```go
type QueryRoute struct {
    Source   string // "wiki", "sql", "metrics_api", "vector_db"
    Query    string // Оригинальный или трансформированный запрос
    Reason   string // Почему этот источник
}

func routeQuery(query string, client *openai.Client) (QueryRoute, error) {
    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: "gpt-4o-mini",
        Messages: []openai.ChatCompletionMessage{
            {
                Role: openai.ChatMessageRoleSystem,
                Content: `Classify the query and route to the correct data source.
Available sources:
- "wiki": documentation, procedures, runbooks
- "sql": structured data, tables, historical records
- "metrics_api": real-time metrics, monitoring data
- "vector_db": semantic search in knowledge base

Return JSON: {"source": "...", "query": "...", "reason": "..."}`,
            },
            {Role: openai.ChatMessageRoleUser, Content: query},
        },
        Temperature: 0,
    })
    if err != nil {
        return QueryRoute{Source: "vector_db", Query: query}, err
    }

    var route QueryRoute
    json.Unmarshal([]byte(resp.Choices[0].Message.Content), &route)
    return route, nil
}

// Использование:
route, _ := routeQuery("время отклика сервера за последний час")
// route.Source = "metrics_api"
// route.Reason = "запрос о real-time метриках"
```

### Hybrid Search (Гибридный поиск)

**Проблема:** Векторный поиск хорошо ищет по смыслу, но плохо по точным терминам. Keyword-поиск — наоборот. Запрос "ошибка ORA-12154" нужно искать и по ключевому слову "ORA-12154", и по смыслу "ошибка подключения к Oracle".

**Решение:** Комбинируем оба подхода и объединяем результаты.

```go
type SearchResult struct {
    ChunkID string
    Text    string
    Score   float64
}

func hybridSearch(query string, topK int) []SearchResult {
    // 1. Keyword search (BM25)
    keywordResults := bm25Search(query, topK)

    // 2. Vector search (Semantic)
    embedding := embedQuery(query)
    vectorResults := vectorDB.Search(embedding, topK)

    // 3. Reciprocal Rank Fusion (RRF) — объединение результатов
    return reciprocalRankFusion(keywordResults, vectorResults, topK)
}

// RRF: комбинирует ранги из разных списков
func reciprocalRankFusion(lists ...[]SearchResult) []SearchResult {
    scores := make(map[string]float64) // chunkID → combined score
    k := 60.0 // Константа RRF (стандартное значение)

    for _, list := range lists {
        for rank, result := range list {
            scores[result.ChunkID] += 1.0 / (k + float64(rank+1))
        }
    }

    // Сортируем по combined score (убывание)
    // ... сортировка и возврат top-K ...
}
```

**Когда что работает лучше:**

| Тип запроса | Keyword | Vector | Hybrid |
|-------------|---------|--------|--------|
| "ORA-12154" | Отлично | Плохо | Отлично |
| "не могу подключиться к базе" | Плохо | Отлично | Отлично |
| "ORA-12154 не подключается" | Средне | Средне | Отлично |

### Reranking (Переранжирование)

**Проблема:** Поиск вернул 20 результатов. Не все одинаково релевантны. Передавать все 20 в контекст — расход токенов.

**Решение:** После первичного поиска переранжируем результаты с помощью более точной (но медленной) модели.

```go
func rerankResults(query string, results []SearchResult, topK int) []SearchResult {
    // Используем LLM для оценки релевантности каждого результата
    type scored struct {
        Result SearchResult
        Score  float64
    }
    var scored []scored

    for _, r := range results {
        score := scoreRelevance(query, r.Text) // cross-encoder или LLM
        scored = append(scored, scored{Result: r, Score: score})
    }

    // Сортируем по score (убывание)
    sort.Slice(scored, func(i, j int) bool {
        return scored[i].Score > scored[j].Score
    })

    // Возвращаем top-K
    result := make([]SearchResult, 0, topK)
    for i := 0; i < topK && i < len(scored); i++ {
        result = append(result, scored[i].Result)
    }
    return result
}
```

**Двухэтапный пайплайн:** Быстрый поиск (BM25/vector, top-100) → Точный reranking (cross-encoder, top-5).

### Self-RAG (Самооценка качества)

**Проблема:** Агент нашёл документы, но они могут быть нерелевантными, устаревшими или неполными. Базовый RAG этого не замечает.

**Решение:** Модель сама оценивает качество найденных документов и решает: ответить, уточнить запрос или искать ещё.

```go
type RetrievalAssessment struct {
    IsRelevant  bool   `json:"is_relevant"`  // Документы релевантны запросу?
    IsSufficient bool  `json:"is_sufficient"` // Достаточно информации для ответа?
    Action      string `json:"action"`        // "answer", "refine_query", "search_more"
    RefinedQuery string `json:"refined_query,omitempty"` // Уточнённый запрос (если нужно)
}

func assessRetrieval(query string, docs []SearchResult, client *openai.Client) (RetrievalAssessment, error) {
    docsText := formatDocs(docs)
    resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: "gpt-4o-mini",
        Messages: []openai.ChatCompletionMessage{
            {
                Role: openai.ChatMessageRoleSystem,
                Content: `Assess if the retrieved documents are relevant and sufficient to answer the query.
Return JSON: {"is_relevant": bool, "is_sufficient": bool, "action": "answer"|"refine_query"|"search_more", "refined_query": "..."}`,
            },
            {
                Role: openai.ChatMessageRoleUser,
                Content: fmt.Sprintf("Query: %s\n\nRetrieved documents:\n%s", query, docsText),
            },
        },
        Temperature: 0,
    })
    if err != nil {
        return RetrievalAssessment{IsRelevant: true, IsSufficient: true, Action: "answer"}, err
    }

    var assessment RetrievalAssessment
    json.Unmarshal([]byte(resp.Choices[0].Message.Content), &assessment)
    return assessment, nil
}
```

**Self-RAG в цикле:**

```go
func selfRAG(query string, maxAttempts int) ([]SearchResult, error) {
    currentQuery := query

    for attempt := 0; attempt < maxAttempts; attempt++ {
        docs := hybridSearch(currentQuery, 10)
        assessment, _ := assessRetrieval(currentQuery, docs)

        switch assessment.Action {
        case "answer":
            return docs, nil // Документы достаточны — отвечаем
        case "refine_query":
            currentQuery = assessment.RefinedQuery // Уточняем запрос
        case "search_more":
            // Расширяем поиск (больше top-K или другой источник)
            moreDocs := hybridSearch(currentQuery, 20)
            docs = append(docs, moreDocs...)
            return docs, nil
        }
    }
    return nil, fmt.Errorf("could not find sufficient documents after %d attempts", maxAttempts)
}
```

### Agentic RAG (RAG как инструмент агента)

**Проблема:** Self-RAG оценивает качество поиска, но не может принимать сложные решения: какой источник использовать, нужно ли комбинировать информацию из нескольких документов, или решить задачу multi-hop (ответ требует цепочки поисков).

**Решение:** RAG встраивается в Agent Loop (он же ReAct Loop — см. [Главу 04](../04-autonomy-and-loops/README.md)). Агент сам решает, когда искать, где искать и достаточно ли информации.

```go
// Agentic RAG: RAG — это просто инструменты агента
tools := []openai.Tool{
    // Инструмент 1: Поиск в документации
    {
        Function: &openai.FunctionDefinition{
            Name:        "search_docs",
            Description: "Search documentation and runbooks. Use when you need procedures or technical details.",
            Parameters:  searchParamsSchema,
        },
    },
    // Инструмент 2: SQL-запрос к базе метрик
    {
        Function: &openai.FunctionDefinition{
            Name:        "query_metrics",
            Description: "Query metrics database. Use when you need historical data or statistics.",
            Parameters:  sqlParamsSchema,
        },
    },
    // Инструмент 3: Поиск похожих инцидентов
    {
        Function: &openai.FunctionDefinition{
            Name:        "search_incidents",
            Description: "Search past incidents for similar issues and their resolutions.",
            Parameters:  searchParamsSchema,
        },
    },
}

// Агент сам решает:
// Итерация 1: search_docs("nginx 502 error troubleshooting")
// Итерация 2: query_metrics("SELECT avg(latency) FROM requests WHERE status=502 AND time > now()-1h")
// Итерация 3: search_incidents("nginx 502 upstream timeout")
// Итерация 4: Финальный ответ с объединением информации из всех источников
```

**Multi-hop RAG (цепочка поисков):**

```
Запрос: "Почему вчера упал сервис payments?"

Шаг 1: search_incidents("payments service outage yesterday")
       → "Инцидент INC-4521: payments упал из-за таймаута к БД"

Шаг 2: search_docs("payments database connection configuration")
       → "Payments использует PostgreSQL на db-prod-03, connection pool = 20"

Шаг 3: query_metrics("SELECT connections FROM pg_stat WHERE time = yesterday")
       → "Пик подключений: 150 (при лимите 100)"

Шаг 4: Финальный ответ: "Payments упал из-за исчерпания connection pool.
        Пик — 150 подключений при лимите 100. Рекомендация: увеличить pool до 200."
```

**Разница между подходами:**

| Подход | Кто принимает решения | Где логика |
|--------|----------------------|-----------|
| Basic RAG | Жёсткий pipeline | Код (hardcoded) |
| Advanced RAG | Pipeline с ветвлениями | Код + конфиг |
| Self-RAG | Модель оценивает качество | Модель (assessment) |
| Agentic RAG | Агент управляет всем | Agent Loop |

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

**Сдано:**
- [x] Агент ищет в базе знаний перед выполнением действий
- [x] Поисковые запросы конкретные и содержат ключевые слова
- [x] Документы разбиты на чанки подходящего размера
- [x] Инструмент поиска имеет четкое описание
- [x] System Prompt инструктирует агента использовать базу знаний
- [x] Для большого пространства инструментов используется tool retrieval (только релевантные tools в контексте)
- [x] Пайплайны валидируются перед выполнением (risk level, allowlist)
- [x] Опасные операции требуют подтверждения

**Не сдано:**
- [ ] Агент не ищет в базе знаний (использует только общие знания)
- [ ] Поисковые запросы слишком общие (не находит нужную информацию)
- [ ] Чанки слишком большие (не влезают в контекст)
- [ ] Все инструменты передаются в контекст без фильтрации (1000+ tools)
- [ ] Используется универсальный `run_shell` без контроля безопасности
- [ ] Пайплайны выполняются без валидации

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
- **MCP Resources:** MCP предоставляет стандартный механизм для доступа к данным (Resources) — [Model Context Protocol](https://modelcontextprotocol.io/)

## Что дальше?

После изучения RAG переходите к:
- **[07. Multi-Agent Systems](../07-multi-agent/README.md)** — как создать команду агентов


