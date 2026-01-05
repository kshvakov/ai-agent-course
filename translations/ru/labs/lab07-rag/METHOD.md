# Методическое пособие: Lab 07 — RAG & Knowledge Base

## Зачем это нужно?

Обычный агент знает только то, чему его научили при тренировке (до даты cut-off). Он не знает ваши локальные инструкции типа "Как перезагружать сервер Phoenix согласно регламенту №5".

**RAG (Retrieval Augmented Generation)** — это механизм "подглядывания в шпаргалку". Агент сначала ищет информацию в базе знаний, а потом действует.

### Реальный кейс

**Ситуация:** Пользователь просит: "Перезагрузи сервер Phoenix согласно регламенту".

**Без RAG:**
- Агент: [Сразу рестартит сервер]
- Результат: Нарушение регламента (нужно было сначала создать бэкап)

**С RAG:**
- Агент: Ищет в базе знаний "Phoenix restart protocol"
- База знаний: "POLICY #12: Before restarting Phoenix, you MUST run backup_db"
- Агент: Создает бэкап → Рестартит сервер → Следует регламенту

**Разница:** RAG позволяет агенту использовать актуальную документацию.

## Теория простыми словами

### Как работает RAG?

1. **Задача:** "Перезагрузи сервер Phoenix согласно регламенту"
2. **Мысль агента:** "Я не знаю регламент. Надо поискать"
3. **Действие:** `search_knowledge_base("Phoenix restart protocol")`
4. **Результат:** "Файл `protocols.txt`: ...сначала выключить балансировщик, потом сервер..."
5. **Мысль агента:** "Ага, понял. Сначала выключаю балансировщик..."

### Простой RAG vs Векторный поиск

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
// Документы преобразуются в векторы (embeddings)
// Поиск похожих векторов по косинусному расстоянию
similarDocs := vectorDB.Search(queryEmbedding, topK=3)
```

## Алгоритм выполнения

### Шаг 1: Создание базы знаний

```go
var knowledgeBase = map[string]string{
    "restart_policy.txt": "POLICY #12: Before restarting any server, you MUST run 'backup_db'. Failure to do so is a violation.",
    "backup_guide.txt":   "To run backup, use tool 'run_backup'. It takes no arguments.",
}
```

### Шаг 2: Инструмент поиска

```go
func searchKnowledge(query string) string {
    var results []string
    for filename, content := range knowledgeBase {
        if strings.Contains(strings.ToLower(content), strings.ToLower(query)) {
            results = append(results, fmt.Sprintf("File: %s\nContent: %s", filename, content))
        }
    }
    if len(results) == 0 {
        return "No documents found."
    }
    return strings.Join(results, "\n---\n")
}
```

### Шаг 3: System Prompt с инструкцией

```go
systemPrompt := `You are a DevOps Agent.
CRITICAL: Always search knowledge base for policies before any restart action.
If you don't know the procedure, search first.`
```

### Шаг 4: Цикл агента

```go
// Агент получает задачу: "Restart server"
// Агент думает: "Нужно проверить регламент"
// Агент вызывает: search_knowledge_base("restart")
// Получает: "POLICY #12: ...MUST run backup..."
// Агент думает: "Нужно сделать бэкап сначала"
// Агент вызывает: run_backup()
// Агент вызывает: restart_server()
```

## Типовые ошибки

### Ошибка 1: Агент не ищет в базе знаний

**Симптом:** Агент сразу выполняет действие без поиска.

**Причина:** System Prompt недостаточно строгий.

**Решение:**
```go
// Усильте промпт:
"BEFORE any action, you MUST search knowledge base. This is mandatory."
```

### Ошибка 2: Поиск не находит документы

**Симптом:** `search_knowledge_base` возвращает "No documents found".

**Причина:** Запрос не совпадает с содержимым документов.

**Решение:**
1. Улучшите поиск (используйте несколько ключевых слов)
2. Добавьте больше документов в базу знаний
3. Используйте векторный поиск (в продакшене)

### Ошибка 3: Агент игнорирует найденную информацию

**Симптом:** Агент нашел регламент, но не следует ему.

**Причина:** Информация не добавлена в контекст правильно.

**Решение:**
```go
// Убедитесь, что результат поиска добавлен в историю:
messages = append(messages, ChatCompletionMessage{
    Role:    "tool",
    Content: searchResult,  // Агент должен это увидеть!
})
```

## Мини-упражнения

### Упражнение 1: Улучшите поиск

Реализуйте поиск по нескольким ключевым словам:

```go
func searchKnowledge(query string) string {
    keywords := strings.Fields(query)  // Разбиваем на слова
    // Ищем документы, содержащие хотя бы одно ключевое слово
    // ...
}
```

### Упражнение 2: Добавьте приоритет документов

Некоторые документы важнее других. Реализуйте ранжирование:

```go
type Document struct {
    Content string
    Priority int  // 1 = высокий, 3 = низкий
}
```

## Критерии сдачи

✅ **Сдано:**
- Агент ищет в базе знаний перед действием
- Поиск находит релевантные документы
- Агент следует найденным инструкциям
- Код компилируется и работает

❌ **Не сдано:**
- Агент не ищет в базе знаний
- Поиск не работает
- Агент игнорирует найденную информацию

---

**Следующий шаг:** После успешного прохождения Lab 07 переходите к [Lab 08: Multi-Agent](../lab08-multi-agent/README.md)

