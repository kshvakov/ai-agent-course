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

// Entry — one record in long-term memory.
type Entry struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
}

// Store — note storage interface.
type Store interface {
	Save(ctx context.Context, key, value string) error
	Recall(ctx context.Context, query string) ([]Entry, error)
	Delete(ctx context.Context, key string) error
}

// FileStore — a simple JSON-backed store. Good for the lab; in production use SQLite/KV.
type FileStore struct {
	mu      sync.Mutex
	path    string
	entries []Entry
}

func NewFileStore(path string) (*FileStore, error) {
	s := &FileStore{path: path}
	// TODO 1: load entries from the file if it exists.
	// If the file doesn't exist — that's fine, start with an empty slice.
	_ = s
	return s, fmt.Errorf("not implemented")
}

func (s *FileStore) Save(ctx context.Context, key, value string) error {
	// TODO 2: lock mu, upsert by key, call flush().
	return fmt.Errorf("not implemented")
}

func (s *FileStore) Recall(ctx context.Context, query string) ([]Entry, error) {
	// TODO 3: substring search over key+value, limit 5 records.
	return nil, fmt.Errorf("not implemented")
}

func (s *FileStore) Delete(ctx context.Context, key string) error {
	// TODO 4: remove the record by key, call flush().
	return fmt.Errorf("not implemented")
}

func (s *FileStore) flush() error {
	// TODO 5: serialize s.entries to JSON and write to s.path.
	return fmt.Errorf("not implemented")
}

// Run — state of one agent run.
// system lives in messages[0] and DOES NOT change during the Run.
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

// Step appends user input, runs the loop with tool calls, and returns the final assistant text.
func (r *Run) Step(ctx context.Context, userInput string) (string, error) {
	r.messages = append(r.messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: userInput,
	})

	for {
		// TODO 6: proactive condense when r.lastTokens > contextMax * 0.80
		// (only when r.lastTokens > 0 — there's no value yet on the first step).

		resp, err := r.callLLM(ctx)
		if err != nil {
			// TODO 7: reactive condense on ContextOverflow + exactly one retry.
			// If the retry also fails with overflow — return the error to the caller.
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

// condense performs a SINGLE compaction of the middle of the history.
// Preserves messages[0] (system) and the tail (last >=N messages with intact tool pairs).
func (r *Run) condense(ctx context.Context) error {
	if r.condenseDone || len(r.messages) < 6 {
		return nil
	}

	// TODO 8:
	// 1. system := r.messages[0]
	// 2. tail := safeTail(r.messages, 4) — expand the boundary to the left if the
	//    first element of tail is a tool result without its assistant tool_call in the tail.
	// 3. head := r.messages[1 : len(r.messages)-len(tail)]
	// 4. summary, err := r.summarize(ctx, head)
	// 5. Assemble next = [system, user("Context of previous work:\n\n"+summary), tail...]
	// 6. r.messages = next; r.condenseDone = true

	return fmt.Errorf("not implemented")
}

// summarize calls the LLM to get a short summary of a chunk of history.
func (r *Run) summarize(ctx context.Context, head []openai.ChatCompletionMessage) (string, error) {
	// TODO 9: assemble input as linear text (role: content),
	// send a separate request with the system prompt from MANUAL.md
	// (section "Step 2: condense"), and return Choices[0].Message.Content.
	return "", fmt.Errorf("not implemented")
}

// dispatchTool routes memory_save / memory_recall / memory_delete calls.
// Returns the string that goes into the tool message's content.
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

// isContextOverflow returns true if the error looks like a model-window overflow.
// TODO 10: parse the OpenAI SDK error or check for the substring "context_length".
func isContextOverflow(err error) bool {
	return false
}

// jsonSchema — convenience helper for writing Parameters in tools.
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

	// Demo step. In a real lab this should be a REPL.
	answer, err := run.Step(ctx, "Remember that my name is Ivan and I'm responsible for the prod cluster.")
	if err != nil {
		fmt.Fprintln(os.Stderr, "step:", err)
		os.Exit(1)
	}
	fmt.Println(answer)
}
