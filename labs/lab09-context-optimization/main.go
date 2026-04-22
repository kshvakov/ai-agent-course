package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// estimateTokens — a rough token-count estimate for a string.
// Used ONLY to decide "don't even send a clearly overflowing batch"
// before we get a provider response. The authoritative source for
// token counts in production is resp.Usage.PromptTokens.
func estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	// Empirical: for mixed RU/EN text, ~3 characters per token + 1 buffer.
	return len(text)/3 + 1
}

// estimateMessages — estimate over the whole history, accounting for
// tool calls and per-message envelope.
func estimateMessages(msgs []openai.ChatCompletionMessage) int {
	// TODO 1: walk the messages, add estimateTokens(content),
	// 4 tokens of per-message overhead, and for each ToolCall —
	// estimateTokens(name) + estimateTokens(args) + 8.
	return 0
}

// Run — state of one agent "run".
// system lives in messages[0] and DOES NOT change during the Run.
type Run struct {
	messages     []openai.ChatCompletionMessage
	lastTokens   int  // resp.Usage.PromptTokens from the previous response
	contextMax   int  // model window; take it from configuration, do not hardcode
	condenseDone bool // limit: one condense per Run

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

// Step appends user input, runs the loop with tool calls, and returns the final text.
func (r *Run) Step(ctx context.Context, userInput string) (string, error) {
	r.messages = append(r.messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: userInput,
	})

	for {
		// TODO 2: proactive condense.
		// If r.lastTokens > 0 && lastTokens > contextMax * 0.80 — call r.condense(ctx).

		resp, err := r.callLLM(ctx)
		if err != nil {
			// TODO 3: reactive condense on ContextOverflow + exactly one retry.
			// If the retry also fails with overflow — return a clear error.
			return "", err
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

// logUsage prints estimated vs actual for the "how accurate is my pre-send estimate" exercise.
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

// safeTail returns >=N trailing messages, expanding the boundary to the left
// if the tail begins with a tool result whose matching assistant tool_call
// is not in the tail.
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

// condense performs a SINGLE compaction of the middle of the history.
// Preserves messages[0] (system) and the tail (last >=4 messages with intact tool pairs).
func (r *Run) condense(ctx context.Context) error {
	if r.condenseDone || len(r.messages) < 6 {
		return nil
	}

	// TODO 4:
	// 1. system := r.messages[0]
	// 2. tail := safeTail(r.messages, 4)
	// 3. head := r.messages[1 : len(r.messages)-len(tail)]
	// 4. summary, err := r.summarize(ctx, head)
	// 5. next := [system, user("Context of previous work:\n\n"+summary), tail...]
	// 6. r.messages = next; r.condenseDone = true

	return fmt.Errorf("condense not implemented")
}

// summarize calls the LLM to get a short summary of a chunk of history.
func (r *Run) summarize(ctx context.Context, head []openai.ChatCompletionMessage) (string, error) {
	// TODO 5: assemble input as linear text ("role: content\n"),
	// call r.client with the system prompt from MANUAL.md (section "Step 4: condense"),
	// and return Choices[0].Message.Content.
	return "", fmt.Errorf("summarize not implemented")
}

// dispatchTool — a simple fake tool so the demo can exercise tool calls
// and verify pair-protection during condense.
func (r *Run) dispatchTool(ctx context.Context, tc openai.ToolCall) string {
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

// isContextOverflow returns true if the error looks like a context-window overflow.
// TODO 6: parse the OpenAI SDK error or check for the substrings
// "context_length" / "context window".
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

	tools := []openai.Tool{
		{Type: openai.ToolTypeFunction, Function: &openai.FunctionDefinition{
			Name:        "fake_lookup",
			Description: "Fake lookup tool used to exercise tool_call/tool_result pairs in the history.",
			Parameters: jsonSchema(`{
				"type":"object",
				"properties":{"query":{"type":"string"}},
				"required":["query"]
			}`),
		}},
	}

	systemPrompt := "You are an assistant. Answer briefly and to the point. If a lookup is needed — call fake_lookup."

	// contextMax is intentionally lowered to trigger a proactive condense quickly.
	// In production this value comes from model metadata or configuration.
	const contextMax = 4_000

	run := NewRun(client, "gpt-4o-mini", contextMax, systemPrompt, tools)

	ctx := context.Background()

	// A long dialogue with deliberately "fat" turns to push past the 80% threshold.
	steps := []string{
		"Hi! My name is Ivan, I'm a DevOps engineer at TechCorp. Stack: Ubuntu 22.04, Docker, Kubernetes, PostgreSQL, Redis, Nginx, GitLab CI, Terraform, Ansible, Vault, Prometheus, Grafana, ELK, PagerDuty, SonarQube, Bacula.",
		"Walk me through, step by step, how to bring up a single-node Kubernetes on bare Ubuntu for a PoC.",
		"Now lay out a migration plan for PostgreSQL 14 → 16 with minimum downtime in a Kubernetes environment.",
		"Which Prometheus metrics are critical for a production PostgreSQL cluster, and how do I alert on them?",
		"Best practices for storing secrets in Vault when consuming them from Kubernetes via the Vault Agent Injector.",
		"Compare Bacula and Restic for backing up a 10TB+ database — when to pick which?",
		"What's my name and what's our stack?", // memory check after compaction
	}

	for i, input := range steps {
		fmt.Printf("\n--- Step %d ---\nUser: %s\n", i+1, input)
		answer, err := run.Step(ctx, input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "step error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Assistant: %s\n", answer)
		if run.condenseDone {
			fmt.Println("  (condense already done in this Run)")
		}
	}
}
