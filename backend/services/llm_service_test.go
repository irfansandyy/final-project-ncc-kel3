package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"app-backend/models"
)

func Test_should_return_api_base_candidates_correctly(t *testing.T) {
	service := &LLMService{baseURL: "http://localhost/v1"}
	candidates := service.apiBaseCandidates()
	if len(candidates) == 0 {
		t.Errorf("expected candidates, got empty")
	}
}

func Test_should_estimate_tokens_correctly(t *testing.T) {
	tokens := estimateTokens("hello world")
	if tokens != 3 { // 11 / 4 + 1
		t.Errorf("expected 3, got %d", tokens)
	}
	if estimateTokens("") != 0 {
		t.Errorf("expected 0 for empty string")
	}
}

func Test_should_limit_messages_by_context(t *testing.T) {
	messages := []chatMessage{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: string(make([]byte, 2048))}, // ~512 tokens
		{Role: "assistant", Content: "hi"},
	}
	trimmed := limitMessagesByContext(messages, 100)
	if len(trimmed) != 2 {
		t.Errorf("expected 2 messages (system + last), got %d", len(trimmed))
	}
}

func Test_should_parse_completion_text(t *testing.T) {
	resp1 := []byte(`{"content": "hello"}`)
	text, err := parseCompletionText(resp1)
	if err != nil || text != "hello" {
		t.Errorf("expected hello, got %v, %v", text, err)
	}

	resp2 := []byte(`{"choices": [{"text": "world"}]}`)
	text, err = parseCompletionText(resp2)
	if err != nil || text != "world" {
		t.Errorf("expected world, got %v, %v", text, err)
	}

	resp3 := []byte(`{"choices": [{"message": {"content": "msg"}}]}`)
	text, err = parseCompletionText(resp3)
	if err != nil || text != "msg" {
		t.Errorf("expected msg, got %v, %v", text, err)
	}
}

func Test_should_return_error_parse_completion_text_invalid(t *testing.T) {
	resp := []byte(`{"invalid": true}`)
	_, err := parseCompletionText(resp)
	if err == nil {
		t.Errorf("expected error, got nil")
	}
}

func Test_should_generate_reply_successfully(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices": [{"message": {"content": "Reply"}}]}`))
	}))
	defer ts.Close()

	service := &LLMService{
		baseURL: ts.URL,
		model:   "test",
		client:  ts.Client(),
	}

	history := []models.Message{{Role: "user", Content: "hello"}}
	reply, err := service.GenerateReply(context.Background(), history, "hello")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if reply != "Reply" {
		t.Errorf("expected Reply, got %s", reply)
	}
}

func Test_should_use_fallback_completion_when_chat_fails(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/chat/completions") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte(`{"content": "Fallback Reply"}`))
	}))
	defer ts.Close()

	service := &LLMService{
		baseURL: ts.URL,
		model:   "test",
		client:  ts.Client(),
	}

	reply, err := service.GenerateReply(context.Background(), nil, "hello")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if reply != "Fallback Reply" {
		t.Errorf("expected Fallback Reply, got %s", reply)
	}
}

func Test_should_return_error_when_all_endpoints_fail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	service := &LLMService{
		baseURL: ts.URL,
		model:   "test",
		client:  ts.Client(),
	}

	_, err := service.GenerateReply(context.Background(), nil, "hello")
	if err == nil {
		t.Errorf("expected error, got nil")
	}
}

func Test_should_generate_reply_stream_successfully(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Stream\"}}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer ts.Close()

	service := &LLMService{
		baseURL: ts.URL,
		model:   "test",
		client:  ts.Client(),
	}

	tokens := []string{}
	reply, err := service.GenerateReplyStream(context.Background(), nil, "hello", func(s string) error {
		tokens = append(tokens, s)
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if reply != "Stream" {
		t.Errorf("expected Stream, got %s", reply)
	}
	if len(tokens) != 1 {
		t.Errorf("expected 1 token, got %d", len(tokens))
	}
}

func Test_should_fallback_to_generate_reply_when_stream_fails(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/chat/completions") {
			var body []byte
			r.Body.Read(body)
			// Check if stream is true
			w.WriteHeader(http.StatusNotFound)
			return
		}
		// wait, actually GenerateReplyStream's fallback calls GenerateReply, which tries /chat/completions again with stream=false
		// but since we simulate /chat/completions always 404, it will fallback to completion endpoint!
		w.Write([]byte(`{"content": "Fallback Stream"}`))
	}))
	defer ts.Close()

	service := &LLMService{
		baseURL: ts.URL,
		model:   "test",
		client:  ts.Client(),
	}

	tokens := []string{}
	reply, err := service.GenerateReplyStream(context.Background(), nil, "hello", func(s string) error {
		tokens = append(tokens, s)
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if reply != "Fallback Stream" {
		t.Errorf("expected Fallback Stream, got %s", reply)
	}
	if len(tokens) != 2 { // split space splits "Fallback" and "Stream"
		t.Errorf("expected 2 tokens, got %d", len(tokens))
	}
}

func Test_should_generate_title_successfully(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices": [{"message": {"content": "\"My Title\""}}]}`))
	}))
	defer ts.Close()

	service := &LLMService{
		baseURL: ts.URL,
		model:   "test",
		client:  ts.Client(),
	}

	title, err := service.GenerateTitle(context.Background(), "hello")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if title != "My Title" { // quotes stripped
		t.Errorf("expected My Title, got %s", title)
	}
}

func Test_should_fallback_title_generation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/chat/completions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte(`{"content": "Fallback Title"}`))
	}))
	defer ts.Close()

	service := &LLMService{
		baseURL: ts.URL,
		model:   "test",
		client:  ts.Client(),
	}

	title, err := service.GenerateTitle(context.Background(), "hello")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if title != "Fallback Title" {
		t.Errorf("expected Fallback Title, got %s", title)
	}
}

func Test_should_return_error_when_title_prompt_empty(t *testing.T) {
	service := &LLMService{}
	_, err := service.GenerateTitle(context.Background(), "  ")
	if err == nil {
		t.Errorf("expected error, got nil")
	}
}

func Test_should_pass_health_check_via_health_endpoint(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	service := &LLMService{
		baseURL: ts.URL,
		client:  ts.Client(),
	}

	err := service.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func Test_should_pass_health_check_via_models_fallback(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	service := &LLMService{
		baseURL: ts.URL,
		client:  ts.Client(),
	}

	err := service.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func Test_should_pass_health_check_via_probe_fallback(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" || r.URL.Path == "/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte(`{"content": "pong"}`))
	}))
	defer ts.Close()

	service := &LLMService{
		baseURL: ts.URL,
		client:  ts.Client(),
	}

	err := service.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func Test_should_fail_health_check_when_all_fail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	service := &LLMService{
		baseURL: ts.URL,
		client:  &http.Client{Timeout: time.Millisecond},
	}

	err := service.HealthCheck(context.Background())
	if err == nil {
		t.Errorf("expected error, got nil")
	}
}
