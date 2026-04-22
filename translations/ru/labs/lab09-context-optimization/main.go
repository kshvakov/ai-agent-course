package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// estimateTokens — грубая прикидка количества токенов в строке.
// Используется ТОЛЬКО для решения «не отправить заведомо переполненный батч»
// до получения ответа провайдера. Первоисточник числа токенов в проде —
// resp.Usage.PromptTokens.
func estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	// Эмпирически: для смешанного RU/EN ~3 символа на токен + 1 запас.
	return len(text)/3 + 1
}

// estimateMessages — прикидка для всей истории, с учётом tool calls и envelope.
func estimateMessages(msgs []openai.ChatCompletionMessage) int {
	// TODO 1: пройтись по сообщениям, прибавить estimateTokens(content),
	// 4 токена per-message overhead, для каждого ToolCall — estimateTokens(name)
	// + estimateTokens(args) + 8.
	return 0
}

// Run — состояние одного «прогона» агента.
// system живёт в messages[0] и НЕ меняется в течение Run.
type Run struct {
	messages     []openai.ChatCompletionMessage
	lastTokens   int  // resp.Usage.PromptTokens с прошлого ответа
	contextMax   int  // окно модели; берите из конфигурации, не хардкод
	condenseDone bool // лимит: один condense на Run

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

// Step добавляет ввод пользователя, гоняет цикл с tool calls, возвращает финальный текст.
func (r *Run) Step(ctx context.Context, userInput string) (string, error) {
	r.messages = append(r.messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: userInput,
	})

	for {
		// TODO 2: проактивный condense.
		// Если r.lastTokens > 0 && lastTokens > contextMax * 0.80 — вызвать r.condense(ctx).

		resp, err := r.callLLM(ctx)
		if err != nil {
			// TODO 3: реактивный condense на ContextOverflow + ровно один повтор.
			// Если повтор тоже падает с overflow — fail с понятной ошибкой.
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

// logUsage печатает estimated vs actual для упражнения «насколько точна моя прикидка».
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

// safeTail возвращает >=N последних сообщений, расширяя границу влево,
// если хвост начинается с tool result без своего assistant tool_call в хвосте.
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

// condense выполняет ОДНОКРАТНОЕ сжатие середины истории.
// Сохраняет messages[0] (system) и хвост (последние >=4 сообщения с целыми tool-парами).
func (r *Run) condense(ctx context.Context) error {
	if r.condenseDone || len(r.messages) < 6 {
		return nil
	}

	// TODO 4:
	// 1. system := r.messages[0]
	// 2. tail := safeTail(r.messages, 4)
	// 3. head := r.messages[1 : len(r.messages)-len(tail)]
	// 4. summary, err := r.summarize(ctx, head)
	// 5. next := [system, user("Контекст предыдущей работы:\n\n"+summary), tail...]
	// 6. r.messages = next; r.condenseDone = true

	return fmt.Errorf("condense not implemented")
}

// summarize вызывает LLM, чтобы получить краткую справку по куску истории.
func (r *Run) summarize(ctx context.Context, head []openai.ChatCompletionMessage) (string, error) {
	// TODO 5: соберите input как линейный текст ("role: content\n"),
	// вызовите r.client с системным промптом из MANUAL.md (раздел «Шаг 4: condense»),
	// верните Choices[0].Message.Content.
	return "", fmt.Errorf("summarize not implemented")
}

// dispatchTool — пример простого fake-инструмента, чтобы демо могло крутить tool calls
// и проверять защиту пар при condense.
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

// isContextOverflow возвращает true, если ошибка похожа на переполнение окна.
// TODO 6: распарсите ошибку OpenAI SDK или проверьте подстроку "context_length"/"context window".
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

	systemPrompt := "Ты ассистент. Отвечай кратко и по существу. Если нужен поиск — вызывай fake_lookup."

	// contextMax намеренно занижен, чтобы быстро триггернуть проактивный condense.
	// В проде значение приходит из метаданных модели или конфигурации.
	const contextMax = 4_000

	run := NewRun(client, "gpt-4o-mini", contextMax, systemPrompt, tools)

	ctx := context.Background()

	// Длинный диалог с заведомо «толстыми» репликами, чтобы пробить порог 80%.
	steps := []string{
		"Привет! Меня зовут Иван, я DevOps-инженер в TechCorp. Стек: Ubuntu 22.04, Docker, Kubernetes, PostgreSQL, Redis, Nginx, GitLab CI, Terraform, Ansible, Vault, Prometheus, Grafana, ELK, PagerDuty, SonarQube, Bacula.",
		"Опиши пошагово, как поднять single-node Kubernetes на голом Ubuntu для PoC.",
		"Теперь распиши план миграции PostgreSQL 14 → 16 с минимальным downtime в Kubernetes-окружении.",
		"Какие метрики Prometheus критичны для production-кластера PostgreSQL и как их алертить?",
		"Расскажи про best practices хранения секретов в Vault при использовании из Kubernetes через Vault Agent Injector.",
		"Сравни Bacula и Restic для backup БД 10TB+ — что в каких случаях выбрать?",
		"Как меня зовут и какой у нас стек?", // проверка памяти после сжатия
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
