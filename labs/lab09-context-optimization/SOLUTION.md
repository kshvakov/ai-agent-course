# Solution: Lab 09 — Context Optimization

## Walkthrough

What the lab teaches:

1. **`usage.PromptTokens` as the source of truth** — keep your own `estimateTokens` only for the pre-send estimate.
2. **A single threshold `0.80`** — the only trigger for the proactive `condense`.
3. **Reactive `condense`** on `ContextOverflowError` + exactly one retry.
4. **`safeTail`** — protects `tool_call ↔ tool_result` pairs while building the tail.
5. **One condense per Run** — a `condenseDone bool` flag.
6. **Summary as a `user` message**, not `system` — to avoid killing prompt cache.

## Full solution

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kshvakov/agent/llm"
)

// ---------------------- token accounting ----------------------

func estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	return len(text)/3 + 1
}

func estimateMessages(msgs []llm.Message) int {
	total := 4 // envelope overhead
	for _, m := range msgs {
		total += estimateTokens(m.Content) + 4
		for _, tc := range m.ToolCalls {
			total += estimateTokens(tc.Name) +
				estimateTokens(tc.Arguments) + 8
		}
	}
	return total
}

// ---------------------- Run + condense ----------------------

type Run struct {
	messages     []llm.Message
	lastTokens   int
	contextMax   int
	condenseDone bool

	client llm.Client
	model  string
	tools  []llm.ToolDef
}

func NewRun(client llm.Client, model string, contextMax int, systemPrompt string, tools []llm.ToolDef) *Run {
	return &Run{
		client:     client,
		model:      model,
		contextMax: contextMax,
		tools:      tools,
		messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
		},
	}
}

func (r *Run) Step(ctx context.Context, userInput string) (string, error) {
	r.messages = append(r.messages, llm.Message{
		Role: "user", Content: userInput,
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
			r.messages = append(r.messages, llm.Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    result,
			})
		}
	}
}

func (r *Run) callLLM(ctx context.Context) (*llm.Response, error) {
	return r.client.Complete(ctx, llm.Request{
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

func safeTail(msgs []llm.Message, n int) []llm.Message {
	if n > len(msgs)-1 {
		n = len(msgs) - 1
	}
	start := len(msgs) - n
	for start > 1 && msgs[start].Role == "tool" {
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

	next := make([]llm.Message, 0, 2+len(tail))
	next = append(next, system)
	next = append(next, llm.Message{
		Role:    "user",
		Content: "Context of previous work:\n\n" + summary,
	})
	next = append(next, tail...)

	r.messages = next
	r.condenseDone = true
	fmt.Println("  >>> condense done")
	return nil
}

func (r *Run) summarize(ctx context.Context, head []llm.Message) (string, error) {
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

	resp, err := r.client.Complete(ctx, llm.Request{
		Model:       r.model,
		Temperature: 0,
		Messages: []llm.Message{
			{Role: "system", Content: `You are compressing the agent's working transcript into a brief handoff for the next step.
Preserve:
1. The user's original task.
2. Decisions already made and the reasoning behind them.
3. Which files / resources have been read and what's relevant in them.
4. What still needs to be done.
Drop pleasantries and chatter.`},
			{Role: "user", Content: b.String()},
		},
	})
	if err != nil {
		return "", err
	}
	return resp.Choices[0].Message.Content, nil
}

// ---------------------- tools ----------------------

func (r *Run) dispatchTool(_ context.Context, tc llm.ToolCall) string {
	switch tc.Name {
	case "fake_lookup":
		var args struct {
			Query string `json:"query"`
		}
		_ = json.Unmarshal([]byte(tc.Arguments), &args)
		return fmt.Sprintf("result for %q: ok", args.Query)
	}
	return "unknown tool: " + tc.Name
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

`main` is the same as in the `main.go` from the task: a long dialogue with a deliberately lowered `contextMax = 4000` to trigger `condense` quickly.

## What to look at on review

- `r.lastTokens = resp.Usage.PromptTokens` after **every** response. This is the source of truth; the "compress or not" decision is made strictly from it.
- The proactive condition is checked **before** the request, based on `lastTokens` from the previous response. On the first step `lastTokens == 0` and the check is skipped — that's expected.
- Reactive branch: exactly one retry. After a second overflow — fail with `fmt.Errorf("overflow even after condense: %w", err)`.
- `safeTail` shifts the boundary to the left as long as the tail begins with a `tool` message. Simple defense; for production, additionally verify that every `tool` in the tail has a matching `assistant.tool_call` with the same `ID`.
- `messages[0]` (system) is **not** modified: in `next` we put the old object as-is. No `system + summary` glue.
- Summary is a `user` message, not `system`. Mixing data and instructions is a separate problem we don't buy into.
- `contextMax` comes from outside (`NewRun`). In the demo it's lowered to 4000; in production it comes from configuration / the model catalog.

## Hand check

1. Run the code. Around step 4-5, `actual` (`usage.PromptTokens`) crosses `threshold@80%` and the log shows `>>> condense done`. After that `condenseDone = true` and condense doesn't fire a second time.
2. On the final step ("What's my name and what's our stack?") the agent should answer correctly — because system stayed intact and the summary in the user message preserved the key facts.
3. Compare `estimated` and `actual` in the log. Drift of 10-30% is fine; your estimate isn't supposed to be precise. The point is the order of magnitude is the same.
4. Experiment: drop `safeTail` (just `r.messages[len(r.messages)-4:]`) and craft a scenario where the second-to-last message is a `tool`. On compression you'll get a 400 from the provider. Restore `safeTail` — works again.

---

**Next step:** in [Lab 11: Memory & Context](../lab11-memory-context/README.md), this baseline loop gains a second horizon — long-term cross-session memory (as `memory_save / memory_recall / memory_delete` tools).
