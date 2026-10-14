package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func okResponse(content string) string {
	return `{"choices":[{"index":0,"message":{"role":"assistant","content":"` + content + `"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2}}`
}

func TestNewOpenAI_OptionsCompose(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, okResponse("ok"))
	}))
	defer srv.Close()

	t.Setenv("LLM_API_KEY", "env-key")
	t.Setenv("LLM_BASE_URL", "http://env-must-not-be-used.invalid")

	// WithBaseURL first: the explicit key must survive later option application.
	c := NewOpenAI(WithBaseURL(srv.URL), WithAPIKey("explicit-key"))
	if _, err := c.Complete(context.Background(), Request{Model: "m", Messages: []Message{{Role: "user", Content: "hi"}}}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if gotAuth != "Bearer explicit-key" {
		t.Fatalf("Authorization = %q, want %q", gotAuth, "Bearer explicit-key")
	}
}

func TestComplete_Mapping(t *testing.T) {
	var req openaiRequestProbe
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &req)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"index":0,"message":{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_time","arguments":"{\"tz\":\"UTC\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":19,"completion_tokens":7,"prompt_tokens_details":{"cached_tokens":11}}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	resp, err := c.Complete(context.Background(), Request{
		Model: "test-model",
		Messages: []Message{
			{Role: "system", Content: "be brief"},
			{Role: "user", Content: "what time is it?"},
		},
		Tools: []ToolDef{{
			Name:        "get_time",
			Description: "current time",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"tz":{"type":"string"}}}`),
		}},
		Temperature: 0.2,
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if req.Model != "test-model" {
		t.Errorf("wire model = %q, want test-model", req.Model)
	}
	if req.Temperature == 0 || req.Temperature > 0.3 {
		t.Errorf("wire temperature = %v, want ~0.2", req.Temperature)
	}
	if len(req.Tools) != 1 || req.Tools[0].Function.Name != "get_time" {
		t.Errorf("wire tools = %+v, want one get_time function", req.Tools)
	}

	msg := resp.Choices[0].Message
	if msg.Role != "assistant" || len(msg.ToolCalls) != 1 {
		t.Fatalf("message = %+v, want assistant with one tool call", msg)
	}
	tc := msg.ToolCalls[0]
	if tc.ID != "call_1" || tc.Name != "get_time" || tc.Arguments != `{"tz":"UTC"}` {
		t.Errorf("tool call = %+v", tc)
	}
	if resp.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("finish reason = %q", resp.Choices[0].FinishReason)
	}
	if resp.Usage.PromptTokens != 19 || resp.Usage.CompletionTokens != 7 {
		t.Errorf("usage = %+v", resp.Usage)
	}
	if resp.Usage.CacheReadTokens != 11 {
		t.Errorf("CacheReadTokens = %d, want 11 (from prompt_tokens_details)", resp.Usage.CacheReadTokens)
	}
}

func TestComplete_RetriesTransient(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			io.WriteString(w, `{"error":{"message":"rate limited"}}`)
			return
		}
		io.WriteString(w, okResponse("recovered"))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	resp, err := c.Complete(context.Background(), Request{Model: "m", Messages: []Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Choices[0].Message.Content != "recovered" {
		t.Errorf("content = %q", resp.Choices[0].Message.Content)
	}
	if calls != 3 {
		t.Errorf("requests = %d, want 3 (two transient 429s then success)", calls)
	}
}

func TestComplete_NoRetryOnRequestError(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"error":{"message":"bad request"}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	if _, err := c.Complete(context.Background(), Request{Model: "m", Messages: []Message{{Role: "user", Content: "hi"}}}); err == nil {
		t.Fatal("Complete: want error on 400")
	}
	if calls != 1 {
		t.Errorf("requests = %d, want 1 (400 is not transient)", calls)
	}
}

func TestComplete_RetriesNonJSONErrorBody(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "text/html")
		if calls < 2 {
			// Gateway HTML page: go-openai wraps this in RequestError, not APIError.
			w.WriteHeader(http.StatusBadGateway)
			io.WriteString(w, "<html><body>502 Bad Gateway</body></html>")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, okResponse("recovered"))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	resp, err := c.Complete(context.Background(), Request{Model: "m", Messages: []Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if calls != 2 {
		t.Errorf("requests = %d, want 2 (HTML 502 is transient)", calls)
	}
	if resp.Choices[0].Message.Content != "recovered" {
		t.Errorf("content = %q", resp.Choices[0].Message.Content)
	}
}

func TestComplete_TemperatureZeroSent(t *testing.T) {
	var raw map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &raw)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, okResponse("ok"))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	if _, err := c.Complete(context.Background(), Request{
		Model:       "m",
		Messages:    []Message{{Role: "user", Content: "hi"}},
		Temperature: 0, // deterministic behavior must reach the wire
	}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	temp, ok := raw["temperature"].(float64)
	if !ok {
		t.Fatalf("wire temperature missing: %v", raw["temperature"])
	}
	// 0 is sent as math.SmallestNonzeroFloat32 (upstream workaround for
	// omitempty): present on the wire, indistinguishable from 0 for samplers.
	if temp <= 0 || temp > 1e-6 {
		t.Errorf("wire temperature = %v, want ~0 (1e-45)", temp)
	}
}

func TestStream_RecvUntilEOF(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"po\"}}]}\n\n")
		io.WriteString(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ng\"}}]}\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	s, err := c.Stream(context.Background(), Request{Model: "m", Messages: []Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	defer s.Close()

	var text string
	for {
		d, err := s.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Recv: %v", err)
		}
		text += d.Text
	}
	if text != "pong" {
		t.Errorf("streamed text = %q, want pong", text)
	}
}

func TestLoadConfig(t *testing.T) {
	t.Setenv("LLM_API_KEY", "k")
	t.Setenv("LLM_BASE_URL", "http://example.invalid")
	t.Setenv("LLM_MODEL", "") // unset default
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Model != DefaultModel {
		t.Errorf("Model = %q, want default %q", cfg.Model, DefaultModel)
	}

	t.Setenv("LLM_MODEL", "custom")
	cfg, err = LoadConfig()
	if err != nil || cfg.Model != "custom" || cfg.APIKey != "k" || cfg.BaseURL != "http://example.invalid" {
		t.Errorf("cfg = %+v, err = %v", cfg, err)
	}

	t.Setenv("LLM_API_KEY", "")
	if _, err := LoadConfig(); err == nil {
		t.Error("want error when LLM_API_KEY is not set")
	}
}

// newTestClient builds a client pointed at a test server; the key is a
// placeholder since test servers do not validate it.
func newTestClient(t *testing.T, url string) *OpenAIClient {
	t.Helper()
	return NewOpenAI(WithAPIKey("test-key"), WithBaseURL(url))
}

// openaiRequestProbe mirrors the wire fields the mapping tests assert on.
type openaiRequestProbe struct {
	Model       string     `json:"model"`
	Temperature float64    `json:"temperature"`
	Messages    []struct{} `json:"messages"`
	Tools       []struct {
		Type     string `json:"type"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	} `json:"tools"`
}
