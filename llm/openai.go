package llm

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// Default model when LLM_MODEL is not set.
const DefaultModel = "glm-5.3"

// OpenAIClient implements Client over any OpenAI-compatible API.
type OpenAIClient struct {
	client *openai.Client
	model  string
}

// options accumulates settings applied by Option functions; the underlying
// openai.Client is built once from them in NewOpenAI.
type options struct {
	apiKey  string // empty: LLM_API_KEY
	baseURL string // empty: LLM_BASE_URL
	model   string // empty: LLM_MODEL, then DefaultModel
}

// Option configures OpenAIClient.
type Option func(*options)

// WithAPIKey overrides the API key (default: LLM_API_KEY).
func WithAPIKey(key string) Option {
	return func(o *options) { o.apiKey = key }
}

// WithBaseURL overrides the base URL (default: LLM_BASE_URL).
func WithBaseURL(url string) Option {
	return func(o *options) { o.baseURL = url }
}

// WithModel overrides the model (default: LLM_MODEL or "glm-5.3").
func WithModel(model string) Option {
	return func(o *options) { o.model = model }
}

// NewOpenAI creates a client; defaults come from LLM_API_KEY / LLM_BASE_URL /
// LLM_MODEL (fallback model: glm-5.3).
func NewOpenAI(opts ...Option) *OpenAIClient {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if o.apiKey == "" {
		o.apiKey = os.Getenv("LLM_API_KEY")
	}
	if o.baseURL == "" {
		o.baseURL = os.Getenv("LLM_BASE_URL")
	}
	cfg := openai.DefaultConfig(o.apiKey)
	if o.baseURL != "" {
		cfg.BaseURL = o.baseURL
	}
	if o.model == "" {
		o.model = os.Getenv("LLM_MODEL")
	}
	if o.model == "" {
		o.model = DefaultModel
	}
	return &OpenAIClient{client: openai.NewClientWithConfig(cfg), model: o.model}
}

func toOpenAIRequest(req Request, model string) openai.ChatCompletionRequest {
	oaiReq := openai.ChatCompletionRequest{
		Model:       model,
		Messages:    make([]openai.ChatCompletionMessage, 0, len(req.Messages)),
		Temperature: float32(req.Temperature),
	}
	for _, m := range req.Messages {
		msg := openai.ChatCompletionMessage{Role: m.Role, Content: m.Content, ToolCallID: m.ToolCallID}
		for _, tc := range m.ToolCalls {
			msg.ToolCalls = append(msg.ToolCalls, openai.ToolCall{
				ID:   tc.ID,
				Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{
					Name:      tc.Name,
					Arguments: tc.Arguments,
				},
			})
		}
		oaiReq.Messages = append(oaiReq.Messages, msg)
	}
	for _, t := range req.Tools {
		oaiReq.Tools = append(oaiReq.Tools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	return oaiReq
}

func fromOpenAIUsage(u openai.Usage) Usage {
	usage := Usage{
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
	}
	if u.PromptTokensDetails != nil {
		usage.CacheReadTokens = u.PromptTokensDetails.CachedTokens
	}
	return usage
}

func fromOpenAIMessage(m openai.ChatCompletionMessage) Message {
	msg := Message{Role: m.Role, Content: m.Content, ToolCallID: m.ToolCallID}
	for _, tc := range m.ToolCalls {
		if tc.Function.Name == "" && tc.Function.Arguments == "" {
			continue
		}
		msg.ToolCalls = append(msg.ToolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}
	return msg
}

// maxAttempts is the retry budget for transient errors (canon of ch. 04: 3 attempts).
const maxAttempts = 3

// Complete sends the request and maps the first response back.
// Transient errors (429, 5xx, network timeouts) are retried here with
// exponential backoff — the agent loop knows nothing about it.
func (c *OpenAIClient) Complete(ctx context.Context, req Request) (*Response, error) {
	model := req.Model
	if model == "" {
		model = c.model
	}
	oaiReq := toOpenAIRequest(req, model)

	var resp openai.ChatCompletionResponse
	var err error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<(attempt-1)) * 500 * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}
		resp, err = c.client.CreateChatCompletion(ctx, oaiReq)
		if err == nil || !isTransient(err) {
			break
		}
	}
	if err != nil {
		return nil, err
	}
	out := &Response{Usage: fromOpenAIUsage(resp.Usage)}
	for _, ch := range resp.Choices {
		out.Choices = append(out.Choices, Choice{
			Message:      fromOpenAIMessage(ch.Message),
			FinishReason: string(ch.FinishReason),
		})
	}
	return out, nil
}

// isTransient reports whether the error is worth a retry: HTTP 429/5xx
// or a network-level failure. Request errors (400, 401) and context
// cancellation are not retried.
func isTransient(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// RequestError is returned for 429/5xx whose body does not parse as
	// {"error":{...}} (HTML gateway pages, {"error":"string"}) — the status
	// code lives only on RequestError, so APIError alone is not enough.
	var reqErr *openai.RequestError
	if errors.As(err, &reqErr) {
		return reqErr.HTTPStatusCode == http.StatusTooManyRequests || reqErr.HTTPStatusCode >= 500
	}
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		return apiErr.HTTPStatusCode == http.StatusTooManyRequests || apiErr.HTTPStatusCode >= 500
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

// openaiStream adapts *openai.ChatCompletionStream to the Stream interface.
type openaiStream struct {
	inner *openai.ChatCompletionStream
}

func (s *openaiStream) Recv() (Delta, error) {
	resp, err := s.inner.Recv()
	if err != nil {
		return Delta{}, err // includes io.EOF at the end of the stream
	}
	if len(resp.Choices) == 0 {
		return Delta{}, errors.New("llm: stream response has no choices")
	}
	return Delta{Text: resp.Choices[0].Delta.Content}, nil
}

func (s *openaiStream) Close() error {
	return s.inner.Close()
}

// Stream opens a streaming completion; Recv returns io.EOF when finished.
func (c *OpenAIClient) Stream(ctx context.Context, req Request) (Stream, error) {
	model := req.Model
	if model == "" {
		model = c.model
	}
	inner, err := c.client.CreateChatCompletionStream(ctx, toOpenAIRequest(req, model))
	if err != nil {
		return nil, fmt.Errorf("llm: create stream: %w", err)
	}
	return &openaiStream{inner: inner}, nil
}
