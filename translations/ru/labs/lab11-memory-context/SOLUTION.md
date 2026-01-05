# Решение: Lab 11 — Memory и Context Management

## 📝 Разбор решения

### Ключевые моменты

1. **Извлечение фактов:** Используйте LLM для извлечения только важных фактов (важность >= 5).

2. **Саммаризация:** Сохраняйте важные факты в промпте саммаризации.

3. **Слои контекста:** Собирайте контекст из System + Facts + Summary + Working Memory.

4. **Релевантный поиск:** Извлекайте факты на основе текущего запроса.

### 🔍 Полное решение

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

type MemoryItem struct {
	Key        string
	Value      string
	Importance int
	Timestamp  int64
}

type Memory interface {
	Store(key string, value any, importance int) error
	Retrieve(query string, limit int) ([]MemoryItem, error)
	Forget(key string) error
}

type Fact struct {
	Key        string `json:"key"`
	Value      string `json:"value"`
	Importance int    `json:"importance"`
}

type FileMemory struct {
	items []MemoryItem
	file  string
}

func NewFileMemory(file string) *FileMemory {
	m := &FileMemory{
		items: []MemoryItem{},
		file:  file,
	}
	m.load()
	return m
}

func (m *FileMemory) load() {
	data, err := os.ReadFile(m.file)
	if err != nil {
		return
	}
	json.Unmarshal(data, &m.items)
}

func (m *FileMemory) save() error {
	data, err := json.MarshalIndent(m.items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.file, data, 0644)
}

func (m *FileMemory) Store(key string, value any, importance int) error {
	item := MemoryItem{
		Key:        key,
		Value:      fmt.Sprintf("%v", value),
		Importance: importance,
		Timestamp:  time.Now().Unix(),
	}
	m.items = append(m.items, item)
	return m.save()
}

func (m *FileMemory) Retrieve(query string, limit int) ([]MemoryItem, error) {
	var results []MemoryItem
	queryLower := strings.ToLower(query)

	for _, item := range m.items {
		if strings.Contains(strings.ToLower(item.Value), queryLower) ||
			strings.Contains(strings.ToLower(item.Key), queryLower) {
			results = append(results, item)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Importance > results[j].Importance
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

func (m *FileMemory) Forget(key string) error {
	var newItems []MemoryItem
	for _, item := range m.items {
		if item.Key != key {
			newItems = append(newItems, item)
		}
	}
	m.items = newItems
	return m.save()
}

func extractFacts(ctx context.Context, client *openai.Client, conversation string) ([]Fact, error) {
	prompt := fmt.Sprintf(`Извлеки важные факты из этого разговора.

Разговор:
%s

Верни факты в формате JSON:
{
  "facts": [
    {"key": "user_name", "value": "Иван", "importance": 10},
    {"key": "company", "value": "TechCorp", "importance": 8}
  ]
}

Важность: 1-10, где 10 - очень важно (имя пользователя, предпочтения),
1-3 - временная информация (статус сервера). Извлекай только факты с важностью >= 5.`, conversation)

	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo,
		Messages: []openai.ChatCompletionMessage{
			{Role: "user", Content: prompt},
		},
		Temperature: 0,
	})
	if err != nil {
		return nil, err
	}

	var data struct {
		Facts []Fact `json:"facts"`
	}
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &data); err != nil {
		return nil, err
	}

	return data.Facts, nil
}

func summarizeConversation(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage) (string, error) {
	var textParts []string
	for _, msg := range messages {
		if msg.Role != openai.ChatMessageRoleSystem && msg.Content != "" {
			textParts = append(textParts, fmt.Sprintf("%s: %s", msg.Role, msg.Content))
		}
	}
	conversationText := strings.Join(textParts, "\n")

	prompt := fmt.Sprintf(`Создай краткое резюме этого разговора, сохранив только:
1. Важные принятые решения
2. Ключевые обнаруженные факты (имя пользователя, предпочтения)
3. Текущее состояние задачи

Разговор:
%s

Резюме (максимум 200 слов):`, conversationText)

	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo,
		Messages: []openai.ChatCompletionMessage{
			{Role: "user", Content: prompt},
		},
		Temperature: 0,
	})
	if err != nil {
		return "", err
	}

	return resp.Choices[0].Message.Content, nil
}

func buildLayeredContext(
	systemPrompt string,
	memory Memory,
	summary string,
	workingMemory []openai.ChatCompletionMessage,
	query string,
) []openai.ChatCompletionMessage {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
	}

	// Facts layer
	facts, _ := memory.Retrieve(query, 5)
	if len(facts) > 0 {
		var factTexts []string
		for _, fact := range facts {
			factTexts = append(factTexts, fmt.Sprintf("- %s: %s", fact.Key, fact.Value))
		}
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: "Important facts:\n" + strings.Join(factTexts, "\n"),
		})
	}

	// Summary layer
	if summary != "" {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: "Summary of previous conversation:\n" + summary,
		})
	}

	// Working memory
	messages = append(messages, workingMemory...)

	return messages
}

func main() {
	token := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if token == "" {
		token = "dummy"
	}

	config := openai.DefaultConfig(token)
	if baseURL != "" {
		config.BaseURL = baseURL
	}
	client := openai.NewClientWithConfig(config)

	ctx := context.Background()

	memory := NewFileMemory("memory.json")

	conversation := `Привет! Меня зовут Иван, я работаю DevOps инженером в компании TechCorp.
У нас есть сервер на Ubuntu 22.04.
Мы используем Docker для контейнеризации приложений.`

	facts, err := extractFacts(ctx, client, conversation)
	if err != nil {
		panic(err)
	}

	for _, fact := range facts {
		memory.Store(fact.Key, fact.Value, fact.Importance)
	}

	fmt.Printf("Extracted and stored %d facts\n", len(facts))
}
```

