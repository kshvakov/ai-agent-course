package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// MemoryItem represents a memory item
type MemoryItem struct {
	Key        string
	Value     string
	Importance int // 1-10, where 10 is very important
	Timestamp  int64
}

// Memory interface for storing memory
type Memory interface {
	Store(key string, value any, importance int) error
	Retrieve(query string, limit int) ([]MemoryItem, error)
	Forget(key string) error
}

// Fact represents an extracted fact
type Fact struct {
	Key        string
	Value      string
	Importance int
}

// TODO 1: Implement memory storage
// Use file-based storage (JSON) or in-memory map
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
	// TODO: Save fact to memory
	// TODO: Save to file
	return fmt.Errorf("not implemented")
}

func (m *FileMemory) Retrieve(query string, limit int) ([]MemoryItem, error) {
	// TODO: Find facts by query
	// TODO: Sort by importance
	// TODO: Return top N facts
	return nil, fmt.Errorf("not implemented")
}

func (m *FileMemory) Forget(key string) error {
	// TODO: Delete fact from memory
	// TODO: Save updated memory to file
	return fmt.Errorf("not implemented")
}

// TODO 2: Implement fact extraction via LLM
// Use LLM to extract important facts from conversation
func extractFacts(ctx context.Context, client *openai.Client, conversation string) ([]Fact, error) {
	// TODO: Create prompt for fact extraction
	// TODO: Call LLM
	// TODO: Parse response into Fact structure
	// TODO: Return facts
	
	return nil, fmt.Errorf("not implemented")
}

// TODO 3: Implement conversation summarization
// Use LLM to create brief summary
func summarizeConversation(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage) (string, error) {
	// TODO: Collect text of all messages (except System)
	// TODO: Create summarization prompt
	// TODO: Call LLM to create summary
	// TODO: Return result
	
	return "", fmt.Errorf("not implemented")
}

// TODO 4: Implement layered context assembly
// Assemble context from: System Prompt + Facts + Summary + Working Memory
func buildLayeredContext(
	systemPrompt string,
	memory Memory,
	summary string,
	workingMemory []openai.ChatCompletionMessage,
	query string,
) []openai.ChatCompletionMessage {
	// TODO: Retrieve relevant facts from memory
	// TODO: Assemble final context
	// TODO: Return messages
	
	return nil
}

func main() {
	// Client setup
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

	// Initialize memory
	memory := NewFileMemory("memory.json")

	// Test conversation
	conversation := `Hello! My name is Ivan, I work as a DevOps engineer at TechCorp.
We have a server on Ubuntu 22.04.
We use Docker for application containerization.`

	fmt.Println("=== Lab 11: Memory and Context Management ===")
	fmt.Println("Extracting facts from conversation...\n")

	// TODO: Extract facts
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
