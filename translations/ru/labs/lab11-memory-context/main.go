package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"
)

// Entry — одна запись в long-term памяти.
type Entry struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
}

// Store — интерфейс хранилища заметок.
type Store interface {
	Save(ctx context.Context, key, value string) error
	Recall(ctx context.Context, query string) ([]Entry, error)
	Delete(ctx context.Context, key string) error
}

// FileStore — простое JSON-хранилище. Подходит для лабы; в проде берите SQLite/KV.
type FileStore struct {
	mu      sync.Mutex
	path    string
	entries []Entry
}

func NewFileStore(path string) (*FileStore, error) {
	s := &FileStore{path: path}
	// TODO 1: загрузите entries из файла, если он существует.
	// Если файла нет — это ок, начнём с пустого слайса.
	_ = s
	return s, fmt.Errorf("not implemented")
}

func (s *FileStore) Save(ctx context.Context, key, value string) error {
	// TODO 2: блокировка mu, upsert по key, вызов flush().
	return fmt.Errorf("not implemented")
}

func (s *FileStore) Recall(ctx context.Context, query string) ([]Entry, error) {
	// TODO 3: подстрочный поиск по key+value, лимит 5 записей.
	return nil, fmt.Errorf("not implemented")
}

func (s *FileStore) Delete(ctx context.Context, key string) error {
	// TODO 4: удалить запись по key, вызов flush().
	return fmt.Errorf("not implemented")
}

func (s *FileStore) flush() error {
	// TODO 5: сериализовать s.entries в JSON и записать в s.path.
	return fmt.Errorf("not implemented")
}

// Run — состояние одного запуска агента.
// system живёт в messages[0] и НЕ меняется в течение Run.
type Run struct {
	messages     []openai.ChatCompletionMessage
	lastTokens   int
	contextMax   int
	condenseDone bool

	client *openai.Client
	model  string
	tools  []openai.Tool
	store  Store
}

func NewRun(client *openai.Client, model string, contextMax int, store Store, systemPrompt string, tools []openai.Tool) *Run {
	return &Run{
		client:     client,
		model:      model,
		contextMax: contextMax,
		store:      store,
		tools:      tools,
		messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
		},
	}
}

// Step добавляет ввод пользователя, гоняет цикл с tool calls и возвращает финальный текст ассистента.
func (r *Run) Step(ctx context.Context, userInput string) (string, error) {
	r.messages = append(r.messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: userInput,
	})

	for {
		// TODO 6: проактивный condense, если r.lastTokens > contextMax * 0.80
		// (только когда r.lastTokens > 0 — на первом шаге значения ещё нет).

		resp, err := r.callLLM(ctx)
		if err != nil {
			// TODO 7: реактивный condense на ContextOverflow + ровно один повтор.
			// Если повтор тоже падает с overflow — верните ошибку наружу.
			return "", err
		}

		r.lastTokens = resp.Usage.PromptTokens
		msg := resp.Choices[0].Message
		r.messages = append(r.messages, msg)

		if len(msg.ToolCalls) == 0 {
			return msg.Content, nil
		}

		for _, tc := range msg.ToolCalls {
			result := r.dispatchTool(ctx, tc)
			r.messages = append(r.messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
				Content:    result,
			})
		}
	}
}

func (r *Run) callLLM(ctx context.Context) (openai.ChatCompletionResponse, error) {
	return r.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       r.model,
		Messages:    r.messages,
		Tools:       r.tools,
		Temperature: 0,
	})
}

// condense выполняет ОДНОКРАТНОЕ сжатие середины истории.
// Сохраняет messages[0] (system) и хвост (последние >=N сообщений с целыми tool-парами).
func (r *Run) condense(ctx context.Context) error {
	if r.condenseDone || len(r.messages) < 6 {
		return nil
	}

	// TODO 8:
	// 1. system := r.messages[0]
	// 2. tail := safeTail(r.messages, 4) — расширяйте границу влево, если первый
	//    элемент в tail — tool result без своего assistant tool_call в хвосте.
	// 3. head := r.messages[1 : len(r.messages)-len(tail)]
	// 4. summary, err := r.summarize(ctx, head)
	// 5. Соберите next = [system, user("Контекст предыдущей работы:\n\n"+summary), tail...]
	// 6. r.messages = next; r.condenseDone = true

	return fmt.Errorf("not implemented")
}

// summarize вызывает LLM, чтобы получить краткую справку по куску истории.
func (r *Run) summarize(ctx context.Context, head []openai.ChatCompletionMessage) (string, error) {
	// TODO 9: соберите input как линейный текст (роль: контент),
	// отправьте отдельный запрос с системным промптом из MANUAL.md
	// (раздел «Шаг 2: condense») и верните Choices[0].Message.Content.
	return "", fmt.Errorf("not implemented")
}

// dispatchTool разруливает вызовы memory_save / memory_recall / memory_delete.
// Возвращает строку, которая идёт в content tool-сообщения.
func (r *Run) dispatchTool(ctx context.Context, tc openai.ToolCall) string {
	switch tc.Function.Name {
	case "memory_save":
		var args struct {
			Key, Value string
		}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		if err := r.store.Save(ctx, args.Key, args.Value); err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return "ok"

	case "memory_recall":
		var args struct{ Query string }
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		hits, err := r.store.Recall(ctx, args.Query)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		out, _ := json.Marshal(hits)
		return string(out)

	case "memory_delete":
		var args struct{ Key string }
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		if err := r.store.Delete(ctx, args.Key); err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return "ok"
	}
	return fmt.Sprintf("unknown tool: %s", tc.Function.Name)
}

// isContextOverflow возвращает true, если ошибка похожа на переполнение окна модели.
// TODO 10: распарсите ошибку OpenAI SDK или проверьте подстроку "context_length".
func isContextOverflow(err error) bool {
	return false
}

// jsonSchema — хелпер для удобной записи Parameters в tools.
func jsonSchema(s string) json.RawMessage {
	return json.RawMessage(s)
}

func main() {
	token := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if token == "" {
		token = "dummy"
	}

	cfg := openai.DefaultConfig(token)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	client := openai.NewClientWithConfig(cfg)

	store, err := NewFileStore("memory.json")
	if err != nil {
		fmt.Fprintln(os.Stderr, "store init:", err)
		os.Exit(1)
	}

	tools := []openai.Tool{
		{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{
			Name:        "memory_save",
			Description: "Save a long-term note. Use for stable facts about the user or project.",
			Parameters: jsonSchema(`{
				"type":"object",
				"properties":{
					"key":{"type":"string"},
					"value":{"type":"string"}
				},
				"required":["key","value"]
			}`),
		}},
		{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{
			Name:        "memory_recall",
			Description: "Search long-term notes by query (substring).",
			Parameters: jsonSchema(`{
				"type":"object",
				"properties":{
					"query":{"type":"string"}
				},
				"required":["query"]
			}`),
		}},
		{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{
			Name:        "memory_delete",
			Description: "Delete a note by key.",
			Parameters: jsonSchema(`{
				"type":"object",
				"properties":{
					"key":{"type":"string"}
				},
				"required":["key"]
			}`),
		}},
	}

	systemPrompt := `You are an assistant.
You have memory_save / memory_recall / memory_delete tools that persist between sessions.
Use them for stable facts about the user and project. Don't store transient statuses.`

	run := NewRun(client, "gpt-4o-mini", 128_000, store, systemPrompt, tools)

	ctx := context.Background()

	// Демонстрационный шаг. В реальной лабе здесь должен быть REPL.
	answer, err := run.Step(ctx, "Запомни, что меня зовут Иван и я отвечаю за prod-кластер.")
	if err != nil {
		fmt.Fprintln(os.Stderr, "step:", err)
		os.Exit(1)
	}
	fmt.Println(answer)
}
