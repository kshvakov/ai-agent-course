# 07. RAG и База Знаний

Обычный агент знает только то, чему его научили при тренировке (до даты cut-off). Он не знает ваши локальные инструкции типа "Как перезагружать сервер Phoenix согласно регламенту №5".

**RAG (Retrieval Augmented Generation)** — это механизм "подглядывания в шпаргалку". Агент сначала ищет информацию в базе знаний, а потом действует.

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

**Почему модель вызвала поиск?**
- System Prompt говорит: "Always search knowledge base before actions"
- Description инструмента говорит: "Use this BEFORE performing actions"
- Модель видит слово "регламенту" в запросе и связывает это с поиском

**Шаг 4: Runtime выполняет поиск**

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

**Почему это не магия:**

1. **Модель не "знает" регламент** — она видит его в контексте после поиска
2. **Поиск — это обычный tool** — такой же, как `ping` или `restart_service`
3. **Результат поиска добавляется в `messages[]`** — модель видит его как новое сообщение
4. **Модель генерирует действия на основе контекста** — она видит документацию и следует ей

**Ключевой момент:** RAG — это не магия "знания", а механизм добавления релевантной информации в контекст модели через обычный tool call.

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

## Что дальше?

После изучения RAG переходите к:
- **[08. Multi-Agent Systems](../08-multi-agent/README.md)** — как создать команду агентов

---

**Навигация:** [← Безопасность](../06-safety-and-hitl/README.md) | [Оглавление](../README.md) | [Multi-Agent →](../08-multi-agent/README.md)

