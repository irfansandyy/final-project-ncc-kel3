package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"app-backend/models"
)

// ---------------------------------------------------------------------------
// estimateTokens
// ---------------------------------------------------------------------------

func Test_Should_ReturnZero_When_ContentIsEmpty(t *testing.T) {
	result := estimateTokens("")
	if result != 0 {
		t.Errorf("expected 0, got %d", result)
	}
}

func Test_Should_ReturnCharCountDiv4Plus1_When_ContentNonEmpty(t *testing.T) {
	// 12 chars => 12/4 + 1 = 4
	result := estimateTokens("hello world!")
	expected := len([]rune("hello world!"))/4 + 1
	if result != expected {
		t.Errorf("expected %d, got %d", expected, result)
	}
}

func Test_Should_ReturnOneToken_When_SingleChar(t *testing.T) {
	// 1 char => 1/4 + 1 = 1
	result := estimateTokens("a")
	if result != 1 {
		t.Errorf("expected 1, got %d", result)
	}
}

func Test_Should_HandleUnicode_When_EstimatingTokens(t *testing.T) {
	// 3 runes: "日本語" => 3/4 + 1 = 1
	result := estimateTokens("日本語")
	expected := 3/4 + 1
	if result != expected {
		t.Errorf("expected %d, got %d", expected, result)
	}
}

// ---------------------------------------------------------------------------
// messagesToPrompt
// ---------------------------------------------------------------------------

func Test_Should_FormatSystemMessage_When_RoleIsSystem(t *testing.T) {
	msgs := []chatMessage{{Role: "system", Content: "Be helpful"}}
	result := messagesToPrompt(msgs)
	if !strings.Contains(result, "System: Be helpful") {
		t.Errorf("expected system prefix, got %q", result)
	}
	if !strings.HasSuffix(result, "Assistant:") {
		t.Errorf("expected to end with 'Assistant:', got %q", result)
	}
}

func Test_Should_FormatUserMessage_When_RoleIsUser(t *testing.T) {
	msgs := []chatMessage{{Role: "user", Content: "Hello"}}
	result := messagesToPrompt(msgs)
	if !strings.Contains(result, "User: Hello") {
		t.Errorf("expected User prefix, got %q", result)
	}
}

func Test_Should_FormatAssistantMessage_When_RoleIsAssistant(t *testing.T) {
	msgs := []chatMessage{{Role: "assistant", Content: "Hi there"}}
	result := messagesToPrompt(msgs)
	if !strings.Contains(result, "Assistant: Hi there") {
		t.Errorf("expected Assistant prefix, got %q", result)
	}
}

func Test_Should_FormatMultipleMessages_When_MixedRoles(t *testing.T) {
	msgs := []chatMessage{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "u1"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "u2"},
	}
	result := messagesToPrompt(msgs)

	expected := "System: sys\nUser: u1\nAssistant: a1\nUser: u2\nAssistant:"
	if result != expected {
		t.Errorf("got:\n%s\nwant:\n%s", result, expected)
	}
}

func Test_Should_FormatDefaultAsUser_When_UnknownRole(t *testing.T) {
	msgs := []chatMessage{{Role: "tool", Content: "data"}}
	result := messagesToPrompt(msgs)
	if !strings.Contains(result, "User: data") {
		t.Errorf("expected unknown role to default to User, got %q", result)
	}
}

func Test_Should_ReturnOnlyAssistant_When_NoMessages(t *testing.T) {
	result := messagesToPrompt(nil)
	if result != "Assistant:" {
		t.Errorf("expected 'Assistant:', got %q", result)
	}
}

// ---------------------------------------------------------------------------
// parseCompletionText
// ---------------------------------------------------------------------------

func Test_Should_ReturnContent_When_ContentFieldSet(t *testing.T) {
	body := `{"content":"Hello from LLM"}`
	text, err := parseCompletionText([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "Hello from LLM" {
		t.Errorf("expected 'Hello from LLM', got %q", text)
	}
}

func Test_Should_ReturnChoicesText_When_ContentEmptyButChoicesTextSet(t *testing.T) {
	body := `{"content":"","choices":[{"text":"from choices"}]}`
	text, err := parseCompletionText([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "from choices" {
		t.Errorf("expected 'from choices', got %q", text)
	}
}

func Test_Should_ReturnChoicesMessageContent_When_OnlyMessageContentSet(t *testing.T) {
	body := `{"content":"","choices":[{"text":"","message":{"role":"assistant","content":"from message"}}]}`
	text, err := parseCompletionText([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "from message" {
		t.Errorf("expected 'from message', got %q", text)
	}
}

func Test_Should_ReturnError_When_NoTextAtAll(t *testing.T) {
	body := `{"content":"","choices":[{"text":"","message":{"content":""}}]}`
	_, err := parseCompletionText([]byte(body))
	if err == nil {
		t.Fatal("expected error when no text found")
	}
	if !strings.Contains(err.Error(), "no text") {
		t.Errorf("unexpected error: %v", err)
	}
}

func Test_Should_ReturnError_When_EmptyChoices(t *testing.T) {
	body := `{"content":"","choices":[]}`
	_, err := parseCompletionText([]byte(body))
	if err == nil {
		t.Fatal("expected error when choices are empty")
	}
}

func Test_Should_ReturnError_When_InvalidJSON(t *testing.T) {
	_, err := parseCompletionText([]byte("not json"))
	if err == nil {
		t.Fatal("expected JSON parse error")
	}
}

func Test_Should_ReturnContent_When_ContentHasWhitespace(t *testing.T) {
	body := `{"content":"  trimmed  "}`
	text, err := parseCompletionText([]byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "trimmed" {
		t.Errorf("expected 'trimmed', got %q", text)
	}
}

// ---------------------------------------------------------------------------
// limitMessagesByContext
// ---------------------------------------------------------------------------

func Test_Should_ReturnAsIs_When_SingleMessage(t *testing.T) {
	msgs := []chatMessage{{Role: "system", Content: "hello"}}
	result := limitMessagesByContext(msgs, 4096)
	if len(result) != 1 {
		t.Errorf("expected 1 message, got %d", len(result))
	}
}

func Test_Should_ReturnAsIs_When_EmptyMessages(t *testing.T) {
	result := limitMessagesByContext(nil, 4096)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func Test_Should_ReturnAsIs_When_CtxSizeZero(t *testing.T) {
	msgs := []chatMessage{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "hello"},
	}
	result := limitMessagesByContext(msgs, 0)
	if len(result) != 2 {
		t.Errorf("expected 2 messages unchanged, got %d", len(result))
	}
}

func Test_Should_ReturnAsIs_When_CtxSizeNegative(t *testing.T) {
	msgs := []chatMessage{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "hello"},
	}
	result := limitMessagesByContext(msgs, -1)
	if len(result) != 2 {
		t.Errorf("expected 2 messages unchanged, got %d", len(result))
	}
}

func Test_Should_KeepAllMessages_When_WithinBudget(t *testing.T) {
	msgs := []chatMessage{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello"},
	}
	result := limitMessagesByContext(msgs, 4096)
	if len(result) != 3 {
		t.Errorf("expected 3 messages, got %d", len(result))
	}
}

func Test_Should_TrimOlderMessages_When_ExceedingBudget(t *testing.T) {
	// Create messages that exceed a small context size
	msgs := []chatMessage{
		{Role: "system", Content: "system prompt"},
		{Role: "user", Content: strings.Repeat("a", 2000)},   // old message, large
		{Role: "assistant", Content: strings.Repeat("b", 2000)}, // old message, large
		{Role: "user", Content: "recent question"},              // most recent
	}

	// With a small ctxSize, should keep system + most recent
	result := limitMessagesByContext(msgs, 600)

	if len(result) < 2 {
		t.Fatalf("expected at least 2 messages (system + recent), got %d", len(result))
	}
	// First message must always be system
	if result[0].Role != "system" {
		t.Errorf("first message should be system, got %q", result[0].Role)
	}
	// Last message should be the most recent user question
	last := result[len(result)-1]
	if last.Content != "recent question" {
		t.Errorf("expected last message 'recent question', got %q", last.Content)
	}
}

func Test_Should_AlwaysKeepFirstMessage_When_ContextTooSmall(t *testing.T) {
	msgs := []chatMessage{
		{Role: "system", Content: strings.Repeat("x", 1000)},
		{Role: "user", Content: strings.Repeat("y", 1000)},
	}
	// Even with a tiny context, the first (system) message must be kept, plus at
	// least the most recent non-system message (the code ensures recentReverse has
	// at least 1 element when the budget overflows on the very first iteration).
	result := limitMessagesByContext(msgs, 100)
	if len(result) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(result))
	}
	if result[0].Role != "system" {
		t.Errorf("first message should be system, got %q", result[0].Role)
	}
}

// ---------------------------------------------------------------------------
// openAIEndpointCandidates
// ---------------------------------------------------------------------------

func Test_Should_PrependV1_When_BaseHasNoV1(t *testing.T) {
	candidates := openAIEndpointCandidates("http://localhost:8080", "/chat/completions")
	if len(candidates) < 2 {
		t.Fatalf("expected at least 2 candidates, got %d", len(candidates))
	}
	if candidates[0] != "http://localhost:8080/v1/chat/completions" {
		t.Errorf("expected v1 first, got %q", candidates[0])
	}
	if candidates[1] != "http://localhost:8080/chat/completions" {
		t.Errorf("expected base second, got %q", candidates[1])
	}
}

func Test_Should_UseBaseDirectly_When_BaseEndsWithV1(t *testing.T) {
	candidates := openAIEndpointCandidates("http://localhost:8080/v1", "/chat/completions")
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d: %v", len(candidates), candidates)
	}
	if candidates[0] != "http://localhost:8080/v1/chat/completions" {
		t.Errorf("unexpected candidate: %q", candidates[0])
	}
}

func Test_Should_UseBaseDirectly_When_BaseEndsWithEnginesV1(t *testing.T) {
	candidates := openAIEndpointCandidates("http://localhost/engines/v1", "/models")
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d: %v", len(candidates), candidates)
	}
	if candidates[0] != "http://localhost/engines/v1/models" {
		t.Errorf("unexpected candidate: %q", candidates[0])
	}
}

func Test_Should_StripTrailingSlash_When_BaseHasTrailingSlash(t *testing.T) {
	candidates := openAIEndpointCandidates("http://localhost:8080/", "/chat/completions")
	for _, c := range candidates {
		if strings.Contains(c, "//chat") {
			t.Errorf("candidate should not have double slash: %q", c)
		}
	}
}

// ---------------------------------------------------------------------------
// consumeChatCompletionStream
// ---------------------------------------------------------------------------

func Test_Should_CollectTokens_When_ValidSSEStream(t *testing.T) {
	sseData := `data: {"choices":[{"delta":{"content":"Hello"}}]}

data: {"choices":[{"delta":{"content":" world"}}]}

data: [DONE]
`
	reader := strings.NewReader(sseData)
	var tokens []string
	reply, err := consumeChatCompletionStream(reader, func(token string) error {
		tokens = append(tokens, token)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", reply)
	}
	if len(tokens) != 2 {
		t.Errorf("expected 2 tokens, got %d", len(tokens))
	}
}

func Test_Should_StopAtDone_When_DoneLineReceived(t *testing.T) {
	sseData := `data: {"choices":[{"delta":{"content":"before"}}]}

data: [DONE]

data: {"choices":[{"delta":{"content":"after"}}]}
`
	reader := strings.NewReader(sseData)
	reply, err := consumeChatCompletionStream(reader, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "before" {
		t.Errorf("expected 'before', got %q", reply)
	}
}

func Test_Should_SkipInvalidJSON_When_MalformedChunk(t *testing.T) {
	sseData := `data: not-json

data: {"choices":[{"delta":{"content":"ok"}}]}

data: [DONE]
`
	reader := strings.NewReader(sseData)
	reply, err := consumeChatCompletionStream(reader, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "ok" {
		t.Errorf("expected 'ok', got %q", reply)
	}
}

func Test_Should_SkipEmptyDelta_When_DeltaContentIsEmpty(t *testing.T) {
	sseData := `data: {"choices":[{"delta":{"content":""}}]}

data: {"choices":[{"delta":{"content":"content"}}]}

data: [DONE]
`
	reader := strings.NewReader(sseData)
	var count int
	reply, err := consumeChatCompletionStream(reader, func(token string) error {
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "content" {
		t.Errorf("expected 'content', got %q", reply)
	}
	if count != 1 {
		t.Errorf("expected onToken called once, called %d times", count)
	}
}

func Test_Should_ReturnEmpty_When_NoDataLines(t *testing.T) {
	sseData := "comment: ignore\n\n"
	reader := strings.NewReader(sseData)
	reply, err := consumeChatCompletionStream(reader, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "" {
		t.Errorf("expected empty reply, got %q", reply)
	}
}

func Test_Should_ReturnError_When_OnTokenFails(t *testing.T) {
	sseData := `data: {"choices":[{"delta":{"content":"Hello"}}]}

data: [DONE]
`
	reader := strings.NewReader(sseData)
	expectedErr := fmt.Errorf("write error")
	_, err := consumeChatCompletionStream(reader, func(_ string) error {
		return expectedErr
	})
	if err == nil {
		t.Fatal("expected error from onToken")
	}
	if err.Error() != expectedErr.Error() {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
}

func Test_Should_UseTextFallback_When_DeltaIsEmptyButTextSet(t *testing.T) {
	sseData := `data: {"choices":[{"delta":{"content":""},"text":"fallback-text"}]}

data: [DONE]
`
	reader := strings.NewReader(sseData)
	reply, err := consumeChatCompletionStream(reader, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "fallback-text" {
		t.Errorf("expected 'fallback-text', got %q", reply)
	}
}

func Test_Should_UseMessageFallback_When_DeltaAndTextEmpty(t *testing.T) {
	sseData := `data: {"choices":[{"delta":{"content":""},"text":"","message":{"content":"msg-content"}}]}

data: [DONE]
`
	reader := strings.NewReader(sseData)
	reply, err := consumeChatCompletionStream(reader, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "msg-content" {
		t.Errorf("expected 'msg-content', got %q", reply)
	}
}

func Test_Should_SkipEmptyDataField_When_DataLineIsEmpty(t *testing.T) {
	sseData := "data: \n\ndata: [DONE]\n"
	reader := strings.NewReader(sseData)
	reply, err := consumeChatCompletionStream(reader, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "" {
		t.Errorf("expected empty reply, got %q", reply)
	}
}

func Test_Should_SkipEmptyChoices_When_ChunkHasNoChoices(t *testing.T) {
	sseData := `data: {"choices":[]}

data: {"choices":[{"delta":{"content":"ok"}}]}

data: [DONE]
`
	reader := strings.NewReader(sseData)
	reply, err := consumeChatCompletionStream(reader, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "ok" {
		t.Errorf("expected 'ok', got %q", reply)
	}
}

// ---------------------------------------------------------------------------
// Helpers for HTTP tests
// ---------------------------------------------------------------------------

func newTestLLMService(serverURL string) *LLMService {
	return &LLMService{
		baseURL: serverURL,
		model:   "test-model",
		ctxSize: 4096,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

// ---------------------------------------------------------------------------
// HealthCheck
// ---------------------------------------------------------------------------

func Test_Should_ReturnNil_When_HealthEndpointReturns200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/health") {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	svc := newTestLLMService(server.URL)
	err := svc.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func Test_Should_ReturnNil_When_ModelsEndpointReturns200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/models") {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	svc := newTestLLMService(server.URL)
	err := svc.HealthCheck(context.Background())
	if err != nil {
		t.Fatalf("expected nil from models fallback, got %v", err)
	}
}

func Test_Should_ReturnError_When_AllHealthEndpointsFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	svc := newTestLLMService(server.URL)
	err := svc.HealthCheck(context.Background())
	if err == nil {
		t.Fatal("expected error when all endpoints fail")
	}
}

// ---------------------------------------------------------------------------
// GenerateReply
// ---------------------------------------------------------------------------

func Test_Should_ReturnReply_When_ChatCompletionSucceeds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()

		var req chatCompletionRequest
		if err := json.Unmarshal(body, &req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		resp := chatCompletionResponse{
			Choices: []struct {
				Message chatMessage `json:"message"`
			}{
				{Message: chatMessage{Role: "assistant", Content: "test reply"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc := newTestLLMService(server.URL)
	reply, err := svc.GenerateReply(context.Background(), nil, "Hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "test reply" {
		t.Errorf("expected 'test reply', got %q", reply)
	}
}

func Test_Should_ReturnError_When_ChatCompletionReturnsNoChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatCompletionResponse{Choices: nil}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc := newTestLLMService(server.URL)
	_, err := svc.GenerateReply(context.Background(), nil, "Hello")
	if err == nil {
		t.Fatal("expected error for no choices")
	}
}

func Test_Should_AvoidDuplicatePrompt_When_LastHistoryMatchesUserPrompt(t *testing.T) {
	var receivedReq chatCompletionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()
		json.Unmarshal(body, &receivedReq)

		resp := chatCompletionResponse{
			Choices: []struct {
				Message chatMessage `json:"message"`
			}{
				{Message: chatMessage{Role: "assistant", Content: "ok"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc := newTestLLMService(server.URL)
	history := []models.Message{
		{Role: "user", Content: "hello there"},
	}
	_, err := svc.GenerateReply(context.Background(), history, "hello there")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should NOT have duplicate "hello there" — the prompt matches last history
	userMsgCount := 0
	for _, m := range receivedReq.Messages {
		if m.Role == "user" && m.Content == "hello there" {
			userMsgCount++
		}
	}
	if userMsgCount != 1 {
		t.Errorf("expected 1 user message 'hello there', got %d", userMsgCount)
	}
}

func Test_Should_AppendPrompt_When_LastHistoryDiffersFromPrompt(t *testing.T) {
	var receivedReq chatCompletionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()
		json.Unmarshal(body, &receivedReq)

		resp := chatCompletionResponse{
			Choices: []struct {
				Message chatMessage `json:"message"`
			}{
				{Message: chatMessage{Role: "assistant", Content: "ok"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc := newTestLLMService(server.URL)
	history := []models.Message{
		{Role: "user", Content: "first question"},
	}
	_, err := svc.GenerateReply(context.Background(), history, "second question")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	userMsgCount := 0
	for _, m := range receivedReq.Messages {
		if m.Role == "user" {
			userMsgCount++
		}
	}
	if userMsgCount != 2 {
		t.Errorf("expected 2 user messages, got %d", userMsgCount)
	}
}

func Test_Should_FilterNonUserAssistantRoles_When_HistoryHasSystemMessages(t *testing.T) {
	var receivedReq chatCompletionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()
		json.Unmarshal(body, &receivedReq)

		resp := chatCompletionResponse{
			Choices: []struct {
				Message chatMessage `json:"message"`
			}{
				{Message: chatMessage{Role: "assistant", Content: "ok"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc := newTestLLMService(server.URL)
	history := []models.Message{
		{Role: "system", Content: "should be filtered"},
		{Role: "user", Content: "kept"},
	}
	_, err := svc.GenerateReply(context.Background(), history, "new question")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The only "system" message should be the one injected by the service itself
	for _, m := range receivedReq.Messages {
		if m.Role == "system" && m.Content == "should be filtered" {
			t.Error("history system message should have been filtered out")
		}
	}
}

func Test_Should_ReturnError_When_ServerReturns500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	defer server.Close()

	svc := newTestLLMService(server.URL)
	_, err := svc.GenerateReply(context.Background(), nil, "Hello")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

// ---------------------------------------------------------------------------
// GenerateReplyStream
// ---------------------------------------------------------------------------

func Test_Should_StreamTokens_When_SSEStreamSucceeds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\" World\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	svc := newTestLLMService(server.URL)
	var tokens []string
	reply, err := svc.GenerateReplyStream(context.Background(), nil, "Hi", func(token string) error {
		tokens = append(tokens, token)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", reply)
	}
	if len(tokens) != 2 {
		t.Errorf("expected 2 tokens, got %d", len(tokens))
	}
}

func Test_Should_FallbackToNonStream_When_StreamReturns404(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()
		var req map[string]interface{}
		json.Unmarshal(body, &req)

		isStream, _ := req["stream"].(bool)
		if isStream {
			// Stream request: return 404
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("not found"))
			return
		}

		// Non-stream fallback request: return success
		if strings.Contains(r.URL.Path, "/chat/completions") {
			resp := chatCompletionResponse{
				Choices: []struct {
					Message chatMessage `json:"message"`
				}{
					{Message: chatMessage{Role: "assistant", Content: "fallback reply"}},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	svc := newTestLLMService(server.URL)
	var tokens []string
	reply, err := svc.GenerateReplyStream(context.Background(), nil, "Hi", func(token string) error {
		tokens = append(tokens, token)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "fallback reply" {
		t.Errorf("expected 'fallback reply', got %q", reply)
	}
	// Tokens should have been sent via onToken from the fallback path
	if len(tokens) == 0 {
		t.Error("expected tokens to be delivered from fallback path")
	}
}

// ---------------------------------------------------------------------------
// GenerateTitle
// ---------------------------------------------------------------------------

func Test_Should_ReturnTitle_When_TitleGenerationSucceeds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatCompletionResponse{
			Choices: []struct {
				Message chatMessage `json:"message"`
			}{
				{Message: chatMessage{Role: "assistant", Content: "\"My Chat Title\""}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc := newTestLLMService(server.URL)
	title, err := svc.GenerateTitle(context.Background(), "Tell me about Go programming")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if title != "My Chat Title" {
		t.Errorf("expected 'My Chat Title', got %q", title)
	}
}

func Test_Should_ReturnError_When_TitlePromptIsEmpty(t *testing.T) {
	svc := newTestLLMService("http://localhost:99999")
	_, err := svc.GenerateTitle(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty prompt")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("unexpected error: %v", err)
	}
}

func Test_Should_ReturnError_When_TitlePromptIsWhitespace(t *testing.T) {
	svc := newTestLLMService("http://localhost:99999")
	_, err := svc.GenerateTitle(context.Background(), "   ")
	if err == nil {
		t.Fatal("expected error for whitespace-only prompt")
	}
}

func Test_Should_TruncateTitle_When_TitleExceeds72Runes(t *testing.T) {
	longTitle := strings.Repeat("A", 100)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatCompletionResponse{
			Choices: []struct {
				Message chatMessage `json:"message"`
			}{
				{Message: chatMessage{Role: "assistant", Content: longTitle}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc := newTestLLMService(server.URL)
	title, err := svc.GenerateTitle(context.Background(), "test prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len([]rune(title)) > 72 {
		t.Errorf("expected title truncated to 72 runes, got %d", len([]rune(title)))
	}
}

func Test_Should_ReturnError_When_TitleEndpointReturnsEmptyContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatCompletionResponse{
			Choices: []struct {
				Message chatMessage `json:"message"`
			}{
				{Message: chatMessage{Role: "assistant", Content: ""}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc := newTestLLMService(server.URL)
	_, err := svc.GenerateTitle(context.Background(), "test prompt")
	if err == nil {
		t.Fatal("expected error for empty title content")
	}
}

func Test_Should_ReturnError_When_TitleEndpointReturnsNoChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := chatCompletionResponse{Choices: nil}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc := newTestLLMService(server.URL)
	_, err := svc.GenerateTitle(context.Background(), "test prompt")
	if err == nil {
		t.Fatal("expected error for no choices")
	}
}

// ---------------------------------------------------------------------------
// tryCompletionFallback
// ---------------------------------------------------------------------------

func Test_Should_ReturnText_When_CompletionFallbackSucceeds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := completionResponse{Content: "fallback text"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc := newTestLLMService(server.URL)
	text, err := svc.tryCompletionFallback(context.Background(), "User: hi\nAssistant:", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "fallback text" {
		t.Errorf("expected 'fallback text', got %q", text)
	}
}

func Test_Should_ReturnError_When_CompletionFallbackReturns404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	svc := newTestLLMService(server.URL)
	_, err := svc.tryCompletionFallback(context.Background(), "prompt", 100)
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func Test_Should_ReturnError_When_CompletionFallbackReturnsNoText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := completionResponse{Content: ""}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	svc := newTestLLMService(server.URL)
	_, err := svc.tryCompletionFallback(context.Background(), "prompt", 100)
	if err == nil {
		t.Fatal("expected error for empty completion text")
	}
}

func Test_Should_ReturnError_When_CompletionFallbackServerDown(t *testing.T) {
	// Use a non-routable address to force connection failure
	svc := newTestLLMService("http://192.0.2.1:1")
	svc.client = &http.Client{Timeout: 100 * time.Millisecond}
	_, err := svc.tryCompletionFallback(context.Background(), "prompt", 100)
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

// ---------------------------------------------------------------------------
// apiBaseCandidates
// ---------------------------------------------------------------------------

func Test_Should_ReturnMultipleCandidates_When_BaseEndsWithEnginesV1(t *testing.T) {
	svc := &LLMService{baseURL: "http://localhost/engines/v1"}
	candidates := svc.apiBaseCandidates()
	if len(candidates) < 2 {
		t.Fatalf("expected multiple candidates, got %d: %v", len(candidates), candidates)
	}
	if candidates[0] != "http://localhost/engines/v1" {
		t.Errorf("first candidate should be base, got %q", candidates[0])
	}
}

func Test_Should_ReturnBasePlusEngines_When_PlainBase(t *testing.T) {
	svc := &LLMService{baseURL: "http://localhost:8080"}
	candidates := svc.apiBaseCandidates()
	found := false
	for _, c := range candidates {
		if c == "http://localhost:8080/engines" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected engines candidate, got %v", candidates)
	}
}

func Test_Should_DeduplicateCandidates_When_DuplicatesExist(t *testing.T) {
	svc := &LLMService{baseURL: "http://localhost/v1/"}
	candidates := svc.apiBaseCandidates()
	seen := map[string]bool{}
	for _, c := range candidates {
		if seen[c] {
			t.Errorf("duplicate candidate: %q", c)
		}
		seen[c] = true
	}
}

func Test_Should_HandleEnginesSuffix_When_BaseEndsWithEngines(t *testing.T) {
	svc := &LLMService{baseURL: "http://localhost/engines"}
	candidates := svc.apiBaseCandidates()
	if len(candidates) < 2 {
		t.Fatalf("expected multiple candidates, got %d", len(candidates))
	}
}
