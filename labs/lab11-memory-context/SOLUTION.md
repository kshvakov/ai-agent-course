# Solution: Lab 11 — Memory and Context Management

## Walkthrough

What the lab teaches:

1. **In-Run condense** — once per Run, on `usage.PromptTokens > 80%` or reactively on overflow.
2. **Long-term memory as tools** — `memory_save`, `memory_recall`, `memory_delete`. No layers in the system prompt.
3. **Preserving `tool_call ↔ tool_result` pairs** when compressing history.

What we **don't** do (and why is spelled out in [Ch. 13](../../book/13-context-engineering/README.md#common-errors)):

- LayeredContext (Facts/Summary/Working).
- Auto-extraction of "facts" from every message.
- A dynamic system prompt with state.
- Message scoring and reordering.

## Full solution

```go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"
)

// ---------------------- long-term memory ----------------------

type Entry struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
}

type Store interface {
	Save(ctx context.Context, key, value string) error
	Recall(ctx context.Context, query string) ([]Entry, error)
	Delete(ctx context.Context, key string) error
}

type FileStore struct {
	mu      sync.Mutex
	path    string
	entries []Entry
}

func NewFileStore(path string) (*FileStore, error) {
	s := &FileStore{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(data, &s.entries); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return s, nil
}

func (s *FileStore) Save(_ context.Context, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for i := range s.entries {
		if s.entries[i].Key == key {
			s.entries[i].Value = value
			s.entries[i].CreatedAt = now
			return s.flush()
		}
	}
	s.entries = append(s.entries, Entry{Key: key, Value: value, CreatedAt: now})
	return s.flush()
}

func (s *FileStore) Recall(_ context.Context, query string) ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	q := strings.ToLower(query)
	var hits []Entry
	for _, e := range s.entries {
		if q == "" || strings.Contains(strings.ToLower(e.Key+" "+e.Value), q) {
			hits = append(hits, e)
			if len(hits) >= 5 {
				break
			}
		}
	}
	return hits, nil
}

func (s *FileStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.entries[:0]
	for _, e := range s.entries {
		if e.Key != key {
			out = append(out, e)
		}
	}
	s.entries = out
	return s.flush()
}

func (s *FileStore) flush() error {
	data, err := json.MarshalIndent(s.entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

// ---------------------- Run + condense ----------------------

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

func (r *Run) Step(ctx context.Context, userInput string) (string, error) {
	r.messages = append(r.messages, openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleUser, Content: userInput,
	})

	for {
		if r.lastTokens > 0 && float64(r.lastTokens) > float64(r.contextMax)*0.80 {
			if err := r.condense(ctx); err != nil {
				return "", err
			}
		}

		resp, err := r.callLLM(ctx)
		if err != nil {
			if !isContextOverflow(err) {
				return "", err
			}
			if cerr := r.condense(ctx); cerr != nil {
				return "", cerr
			}
			resp, err = r.callLLM(ctx)
			if err != nil {
				return "", fmt.Errorf("overflow even after condense: %w", err)
			}
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

func (r *Run) condense(ctx context.Context) error {
	if r.condenseDone || len(r.messages) < 6 {
		return nil
	}

	system := r.messages[0]
	tail := safeTail(r.messages, 4)
	head := r.messages[1 : len(r.messages)-len(tail)]

	summary, err := r.summarize(ctx, head)
	if err != nil {
		return err
	}

	next := make([]openai.ChatCompletionMessage, 0, 2+len(tail))
	next = append(next, system)
	next = append(next, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "Context of previous work:\n\n" + summary,
	})
	next = append(next, tail...)

	r.messages = next
	r.condenseDone = true
	return nil
}

// safeTail returns >=N trailing messages, expanding the boundary to the left
// if the first message in the tail is a tool result without its assistant tool_call in the tail.
func safeTail(msgs []openai.ChatCompletionMessage, n int) []openai.ChatCompletionMessage {
	if n > len(msgs)-1 {
		n = len(msgs) - 1
	}
	start := len(msgs) - n
	for start > 1 && msgs[start].Role == openai.ChatMessageRoleTool {
		start--
	}
	return msgs[start:]
}

func (r *Run) summarize(ctx context.Context, head []openai.ChatCompletionMessage) (string, error) {
	var b strings.Builder
	for _, m := range head {
		if m.Content == "" {
			continue
		}
		b.WriteString(string(m.Role))
		b.WriteString(": ")
		b.WriteString(m.Content)
		b.WriteString("\n")
	}

	resp, err := r.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       r.model,
		Temperature: 0,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: `You are compressing the agent's working transcript into a brief handoff for the next step.
Preserve:
1. The user's original task.
2. Decisions already made and the reasoning behind them.
3. Which files / resources have been read and what's relevant in them.
4. What still needs to be done.
Drop pleasantries and chatter.`},
			{Role: openai.ChatMessageRoleUser, Content: b.String()},
		},
	})
	if err != nil {
		return "", err
	}
	return resp.Choices[0].Message.Content, nil
}

// ---------------------- tools dispatching ----------------------

func (r *Run) dispatchTool(ctx context.Context, tc openai.ToolCall) string {
	switch tc.Function.Name {
	case "memory_save":
		var args struct{ Key, Value string }
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			return "error: " + err.Error()
		}
		if err := r.store.Save(ctx, args.Key, args.Value); err != nil {
			return "error: " + err.Error()
		}
		return "ok"

	case "memory_recall":
		var args struct{ Query string }
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			return "error: " + err.Error()
		}
		hits, err := r.store.Recall(ctx, args.Query)
		if err != nil {
			return "error: " + err.Error()
		}
		out, _ := json.Marshal(hits)
		return string(out)

	case "memory_delete":
		var args struct{ Key string }
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			return "error: " + err.Error()
		}
		if err := r.store.Delete(ctx, args.Key); err != nil {
			return "error: " + err.Error()
		}
		return "ok"
	}
	return "unknown tool: " + tc.Function.Name
}

func isContextOverflow(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "context_length") ||
		strings.Contains(msg, "maximum context") ||
		strings.Contains(msg, "context window")
}

func jsonSchema(s string) json.RawMessage { return json.RawMessage(s) }
```

`main` is the same as the `main.go` in the task (a REPL wrapper around `Run.Step` for interactive checks).

## What to look at on review

- `messages[0]` (system) is **not modified** during the Run. No `system + facts + summary` glue.
- The "compress" decision is made from `r.lastTokens = resp.Usage.PromptTokens` — this value comes from the provider and is precise.
- `condense` runs at most once per Run. On the reactive branch, after a second overflow — fail, not "let's try again".
- `safeTail` shifts the boundary to the left if the tail starts with a `tool` message that has no matching `assistant.tool_call`. An alternative is to keep, on every assistant message, a map `ID → tool_results` and cut only at "assistant without pending tool_calls".
- Memory — three separate tools. The contents do **not** end up in the system prompt. The agent calls `memory_recall` itself when it needs to.
- In `summary` we feed only `head` (between system and tail). System doesn't go into the summary — it's already first in the new array.

## Hand check

1. Run a long dialogue so that `usage.PromptTokens` crosses 80% of `contextMax` (you can temporarily lower `contextMax` to 4_000 for a fast trigger). Condition: `condense` fired exactly once, `messages[0]` stayed byte-for-byte the same.
2. Simulate `ContextOverflowError` — verify that the reactive branch does condense + retry and doesn't enter an infinite loop.
3. Restart the process. In a new session, ask about a fact you saved earlier — the agent should call `memory_recall` itself and answer.
4. Verify that the system prompt contains no current time, no "current state", and no memory contents. Only role, rules, and the tool descriptions.
