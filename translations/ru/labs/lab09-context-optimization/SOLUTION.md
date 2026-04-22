# Решение: Lab 09 — Context Optimization

## Разбор решения

Что лаба учит:

1. **`usage.PromptTokens` как первоисточник** — свой `estimateTokens` оставляем только для прикидки до отправки.
2. **Один порог `0.80`** — единственный триггер проактивного `condense`.
3. **Реактивный `condense`** на `ContextOverflowError` + ровно один повтор.
4. **`safeTail`** — защищает пары `tool_call ↔ tool_result` при формировании хвоста.
5. **Лимит «один condense на Run»** — флаг `condenseDone bool`.
6. **Summary как `user`-сообщение**, не `system` — чтобы не убивать prompt cache.

## Полное решение

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// ---------------------- token accounting ----------------------

func estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return len(text)/3 + 1
}

func estimateMessages(msgs []openai.ChatCompletionMessage) int {
	total := 4 // envelope overhead
	for _, m := range msgs {
		total += estimateTokens(m.Content) + 4
		for _, tc := range m.ToolCalls {
			total += estimateTokens(tc.Function.Name) +
				estimateTokens(tc.Function.Arguments) + 8
		}
	}
	return total
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
}

func NewRun(client *openai.Client, model string, contextMax int, systemPrompt string, tools []openai.Tool) *Run {
	return &Run{
		client:     client,
		model:      model,
		contextMax: contextMax,
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
		r.logUsage()

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

func (r *Run) logUsage() {
	estimated := estimateMessages(r.messages)
	threshold := int(float64(r.contextMax) * 0.80)
	delta := r.lastTokens - estimated
	pct := 0.0
	if estimated > 0 {
		pct = float64(delta) * 100 / float64(estimated)
	}
	fmt.Printf("  usage: estimated=%d actual=%d (Δ=%+d, %+.1f%%) threshold@80%%=%d\n",
		estimated, r.lastTokens, delta, pct, threshold)
}

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
	fmt.Println("  >>> condense done")
	return nil
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

// ---------------------- tools ----------------------

func (r *Run) dispatchTool(_ context.Context, tc openai.ToolCall) string {
	switch tc.Function.Name {
	case "fake_lookup":
		var args struct {
			Query string `json:"query"`
		}
		_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
		return fmt.Sprintf("result for %q: ok", args.Query)
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

`main` — как в `main.go` из задания: длинный диалог с занижённым `contextMax = 4000`, чтобы быстро триггернуть `condense`.

## На что обратить внимание при ревью

- `r.lastTokens = resp.Usage.PromptTokens` после **каждого** ответа. Это первоисточник; решение «сжимать или нет» принимается строго по нему.
- Проактивный условие проверяется **перед** запросом, на основе `lastTokens` с прошлого ответа. На первом шаге `lastTokens == 0`, проверка пропускается — это нормально.
- Реактивная ветка: ровно один повтор. После повторного overflow — fail с `fmt.Errorf("overflow even after condense: %w", err)`.
- `safeTail` сдвигает границу влево, пока хвост начинается с `tool` сообщения. Простая защита; для production лучше дополнительно проверять, что у каждого `tool` в хвосте есть свой `assistant.tool_call` с тем же `ID`.
- `messages[0]` (system) **не** меняется: в `next` мы кладём старый объект целиком. Никаких склеек «system + summary».
- Summary — `user` сообщение, не `system`. Смешивание данных и инструкций — отдельная проблема, которую мы не покупаем.
- `contextMax` приходит снаружи (`NewRun`). В демо занижен до 4000, в проде — из конфигурации/каталога модели.

## Ручная проверка

1. Запустите код. Уже к 4-5 шагу `actual` (`usage.PromptTokens`) перевалит за `threshold@80%` и в логе появится `>>> condense done`. Дальше `condenseDone = true` и второй раз condense не сработает.
2. На последнем шаге («Как меня зовут и какой у нас стек?») агент должен ответить корректно — потому что system остался цел, а summary в user-сообщении сохранил основные факты.
3. Сравните `estimated` и `actual` в логе. Видите расхождение в 10-30% — это нормально, своя оценка не должна быть точной. Главное — порядок величины тот же.
4. Поэкспериментируйте: уберите `safeTail` (просто `r.messages[len(r.messages)-4:]`) и сгенерируйте сценарий, где предпоследнее сообщение — `tool`. На сжатии получите 400 от провайдера. Восстановите `safeTail` — заработает.

---

**Следующий шаг:** в [Lab 11: Memory & Context](../lab11-memory-context/README.md) к этому базовому циклу добавляется второй горизонт — долгосрочная память между сессиями (как инструменты `memory_save / memory_recall / memory_delete`).
