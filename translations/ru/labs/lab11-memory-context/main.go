package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// MemoryItem представляет элемент памяти
type MemoryItem struct {
	Key      string
	Value    string
	Importance int // 1-10, где 10 - очень важно
	Timestamp int64
}

// Memory интерфейс для хранения памяти
type Memory interface {
	Store(key string, value any, importance int) error
	Retrieve(query string, limit int) ([]MemoryItem, error)
	Forget(key string) error
}

// Fact представляет извлеченный факт
type Fact struct {
	Key         string
	Value       string
	Importance  int
}

// TODO 1: Реализуйте хранилище памяти
// Используйте файловое хранилище (JSON) или in-memory map
type FileMemory struct {
	items []MemoryItem
	file  string
}

func NewFileMemory(file string) *FileMemory {
	return &FileMemory{
		items: []MemoryItem{},
		file:  file,
	}
}

func (m *FileMemory) Store(key string, value any, importance int) error {
	// TODO: Сохраните факт в память
	// TODO: Сохраните в файл
	return fmt.Errorf("not implemented")
}

func (m *FileMemory) Retrieve(query string, limit int) ([]MemoryItem, error) {
	// TODO: Найдите факты по запросу
	// TODO: Отсортируйте по важности
	// TODO: Верните top N фактов
	return nil, fmt.Errorf("not implemented")
}

func (m *FileMemory) Forget(key string) error {
	// TODO: Удалите факт из памяти
	// TODO: Сохраните обновленную память в файл
	return fmt.Errorf("not implemented")
}

// TODO 2: Реализуйте извлечение фактов через LLM
// Используйте LLM для извлечения важных фактов из разговора
func extractFacts(ctx context.Context, client *openai.Client, conversation string) ([]Fact, error) {
	// TODO: Создайте промпт для извлечения фактов
	// TODO: Вызовите LLM
	// TODO: Распарсите ответ в структуру Fact
	// TODO: Верните факты
	
	return nil, fmt.Errorf("not implemented")
}

// TODO 3: Реализуйте саммаризацию разговора
// Используйте LLM для создания краткого резюме
func summarizeConversation(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage) (string, error) {
	// TODO: Соберите текст всех сообщений (кроме System)
	// TODO: Создайте промпт для саммаризации
	// TODO: Вызовите LLM для создания саммари
	// TODO: Верните результат
	
	return "", fmt.Errorf("not implemented")
}

// TODO 4: Реализуйте сборку слоистого контекста
// Соберите контекст из: System Prompt + Facts + Summary + Working Memory
func buildLayeredContext(
	systemPrompt string,
	memory Memory,
	summary string,
	workingMemory []openai.ChatCompletionMessage,
	query string,
) []openai.ChatCompletionMessage {
	// TODO: Извлеките релевантные факты из памяти
	// TODO: Соберите финальный контекст
	// TODO: Верните сообщения
	
	return nil
}

func main() {
	// Настройка клиента
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

	// Инициализация памяти
	memory := NewFileMemory("memory.json")

	// Тестовый разговор
	conversation := `Привет! Меня зовут Иван, я работаю DevOps инженером в компании TechCorp.
У нас есть сервер на Ubuntu 22.04.
Мы используем Docker для контейнеризации приложений.`

	fmt.Println("=== Lab 11: Memory and Context Management ===")
	fmt.Println("Извлечение фактов из разговора...\n")

	// TODO: Извлеките факты
	facts, err := extractFacts(ctx, client, conversation)
	if err != nil {
		fmt.Printf("Error extracting facts: %v\n", err)
		return
	}

	fmt.Printf("Extracted %d facts:\n", len(facts))
	for _, fact := range facts {
		fmt.Printf("  - %s: %s (importance: %d)\n", fact.Key, fact.Value, fact.Importance)
	}

	_ = memory
	_ = strings.ToLower
	_ = json.RawMessage{}
}

