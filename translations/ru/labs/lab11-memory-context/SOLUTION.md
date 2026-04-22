# Решение: Lab 11 — Memory и Context Management

## Разбор решения

Чему учит лаба:

1. **In-Run condense** — один раз за запуск, по `usage.PromptTokens > 80%` или реактивно на overflow.
2. **Long-term memory как инструменты** — `memory_save`, `memory_recall`, `memory_delete`. Никаких слоёв в system prompt.
3. **Сохранение пар `tool_call ↔ tool_result`** при сжатии истории.

Что **не делаем** (и почему написано подробно в [гл. 13](../../book/13-context-engineering/README.md#типовые-ошибки)):

- LayeredContext (Facts/Summary/Working).
- Авто-извлечение «фактов» из каждого сообщения.
- Динамический system prompt с состоянием.
- Скоринг и переупорядочивание сообщений.

## Полное решение

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
		Content: "Контекст предыдущей работы:\n\n" + summary,
	})
	next = append(next, tail...)

	r.messages = next
	r.condenseDone = true
	return nil
}

// safeTail возвращает >=N последних сообщений, расширяя границу влево,
// если первое сообщение в хвосте — tool result без своего assistant tool_call в хвосте.
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
			{Role: openai.ChatMessageRoleSystem, Content: `Ты сжимаешь рабочую переписку агента в краткую справку для следующего шага.
Сохрани:
1. Исходную задачу пользователя.
2. Решения, которые уже приняты, и обоснования.
3. Какие файлы / ресурсы прочитаны и что в них релевантно.
4. Что ещё осталось сделать.
Опусти вежливости и служебный шум.`},
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

`main` — как в `main.go` из задания (REPL-обёртка вокруг `Run.Step` для интерактивной проверки).

## На что обратить внимание при ревью

- `messages[0]` (system) **не модифицируется** в течение Run. Никаких склеек «system + facts + summary».
- Решение «сжимать» принимается по `r.lastTokens = resp.Usage.PromptTokens` — это значение приходит от провайдера и точное.
- `condense` гарантированно один раз в Run. На реактивной ветке после повторного overflow — fail, а не «попробуем ещё раз».
- `safeTail` сдвигает границу влево, если хвост начинается с `tool` сообщения без своего `assistant.tool_call`. Альтернатива — иметь на каждом ассистент-сообщении карту `ID → tool_results` и резать только по «assistant без pending tool_calls».
- Память — три отдельных tool'а. Содержимое **не** попадает в system prompt. Агент сам делает `memory_recall`, когда ему нужно.
- В summary мы складываем только `head` (между system и хвостом). System в summary не идёт — он и так первым в новом массиве.

## Ручная проверка

1. Заведите длинный диалог так, чтобы `usage.PromptTokens` перевалил за 80% от `contextMax` (можно временно занизить `contextMax` до 4_000 для быстрого триггера). Условие: `condense` сработал ровно один раз, `messages[0]` остался байт-в-байт прежним.
2. Сэмулируйте `ContextOverflowError` — убедитесь, что реактивная ветка делает condense + повтор и не уходит в бесконечный цикл.
3. Перезапустите процесс. В новой сессии спросите про факт, который сохраняли раньше — агент должен сам вызвать `memory_recall` и ответить.
4. Проверьте, что system prompt не содержит ни текущего времени, ни «текущего состояния», ни содержимого памяти. Только роль, правила, описание инструментов.
