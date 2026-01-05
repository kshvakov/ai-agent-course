# RAG в продакшене

## Зачем это нужно?

RAG система работает, но документы устарели, поиск возвращает нерелевантные результаты, агент галлюцинирует. Без прод-готовности RAG вы не можете гарантировать качество ответов.

### Реальный кейс

**Ситуация:** RAG система использует документацию API. Документация обновляется каждый день, но RAG использует старую версию.

**Проблема:** Агент даёт устаревшие ответы, ссылаясь на старую документацию. Нет версионирования документов, нет проверки актуальности.

**Решение:** Версионирование документов, отслеживание freshness, grounding (требование ссылок на документы), fallback при ошибках retrieval.

## Теория простыми словами

### Что такое Freshness?

Freshness — это актуальность документа. Документ считается устаревшим, если он старше определённого возраста (например, 30 дней).

### Что такое Grounding?

Grounding — это требование, чтобы агент ссылался на найденные документы в ответе. Это снижает галлюцинации.

## Как это работает (пошагово)

### Шаг 1: Версионирование документов

Версионируйте документы в базе знаний:

```go
type DocumentVersion struct {
    ID        string    `json:"id"`
    Version   string    `json:"version"`
    Content   string    `json:"content"`
    UpdatedAt time.Time `json:"updated_at"`
}

func getDocumentVersion(id string, version string) (*DocumentVersion, error) {
    // Загружаем конкретную версию документа
    return nil, nil
}
```

### Шаг 2: Freshness проверка

Проверяйте актуальность документов:

```go
func checkFreshness(doc DocumentVersion, maxAge time.Duration) bool {
    age := time.Since(doc.UpdatedAt)
    return age < maxAge
}
```

### Шаг 3: Grounding

Требуйте ссылки на документы в ответе:

```go
func validateGrounding(answer string, documents []DocumentVersion) bool {
    // Проверяем, что ответ содержит ссылки на документы
    for _, doc := range documents {
        if strings.Contains(answer, doc.ID) {
            return true
        }
    }
    return false
}
```

## Где это встраивать в нашем коде

### Точка интеграции: RAG Retrieval

В `labs/lab07-rag/main.go` добавьте проверку freshness и grounding:

```go
func retrieveDocuments(query string) ([]DocumentVersion, error) {
    docs := searchDocuments(query)
    
    // Фильтруем устаревшие документы
    freshDocs := []DocumentVersion{}
    for _, doc := range docs {
        if checkFreshness(doc, 30*24*time.Hour) {
            freshDocs = append(freshDocs, doc)
        }
    }
    
    return freshDocs, nil
}
```

## Типовые ошибки

### Ошибка 1: Документы не версионируются

**Симптом:** Невозможно понять, какая версия документа использовалась в ответе.

**Решение:** Версионируйте документы и отслеживайте версию в ответах.

### Ошибка 2: Нет проверки актуальности

**Симптом:** Агент использует устаревшие документы.

**Решение:** Проверяйте freshness документов перед использованием.

## Критерии сдачи / Чек-лист

✅ **Сдано:**
- Документы версионируются
- Проверяется актуальность документов
- Требуется grounding в ответах

❌ **Не сдано:**
- Документы не версионируются
- Нет проверки актуальности

## Связь с другими главами

- **RAG:** Базовые концепции RAG — [Глава 07: RAG и База Знаний](../07-rag/README.md)

---

**Навигация:** [← Data и Privacy](data_privacy.md) | [Оглавление главы 12](README.md) | [Evals в CI/CD →](evals_in_cicd.md)

