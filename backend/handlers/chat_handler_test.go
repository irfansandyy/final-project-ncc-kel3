package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"app-backend/config"
	"app-backend/middleware"
	"app-backend/models"
	"app-backend/repositories"
	"app-backend/services"

	"github.com/go-chi/chi/v5"
)

type mockChatRepo struct {
	chats    map[int64]*models.Chat
	messages map[int64][]*models.Message
	nextID   int64
	nextMsg  int64
}

func newMockChatRepo() *mockChatRepo {
	return &mockChatRepo{
		chats:    make(map[int64]*models.Chat),
		messages: make(map[int64][]*models.Message),
		nextID:   1,
		nextMsg:  1,
	}
}

func (m *mockChatRepo) CreateChat(ctx context.Context, userID int64, title string) (models.Chat, error) {
	if userID == 999 {
		return models.Chat{}, repositories.ErrUserNotFound
	}
	chat := models.Chat{
		ID:        m.nextID,
		UserID:    userID,
		Title:     title,
		Slug:      fmt.Sprintf("slug-%d", m.nextID),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.chats[m.nextID] = &chat
	m.nextID++
	return chat, nil
}

func (m *mockChatRepo) ListChatsByUser(ctx context.Context, userID int64) ([]models.Chat, error) {
	var userChats []models.Chat
	for _, chat := range m.chats {
		if chat.UserID == userID {
			userChats = append(userChats, *chat)
		}
	}
	return userChats, nil
}

func (m *mockChatRepo) GetChatByID(ctx context.Context, chatID, userID int64) (models.Chat, error) {
	chat, exists := m.chats[chatID]
	if !exists || chat.UserID != userID {
		return models.Chat{}, repositories.ErrChatNotFound
	}
	return *chat, nil
}

func (m *mockChatRepo) GetChatBySlug(ctx context.Context, slug string, userID int64) (models.Chat, error) {
	for _, chat := range m.chats {
		if chat.Slug == slug && chat.UserID == userID {
			return *chat, nil
		}
	}
	return models.Chat{}, repositories.ErrChatNotFound
}

func (m *mockChatRepo) UpdateChatTitle(ctx context.Context, chatID, userID int64, title string) (models.Chat, error) {
	chat, exists := m.chats[chatID]
	if !exists || chat.UserID != userID {
		return models.Chat{}, repositories.ErrChatNotFound
	}
	chat.Title = title
	return *chat, nil
}

func (m *mockChatRepo) CreateMessage(ctx context.Context, chatID int64, role, content string) (models.Message, error) {
	msg := models.Message{
		ID:        m.nextMsg,
		ChatID:    chatID,
		Role:      role,
		Content:   content,
		CreatedAt: time.Now(),
	}
	m.messages[chatID] = append(m.messages[chatID], &msg)
	m.nextMsg++
	return msg, nil
}

func (m *mockChatRepo) ListMessagesByChat(ctx context.Context, chatID, userID int64, limit int) ([]models.Message, error) {
	chat, exists := m.chats[chatID]
	if !exists || chat.UserID != userID {
		return nil, repositories.ErrChatNotFound
	}

	var msgs []models.Message
	for _, msg := range m.messages[chatID] {
		msgs = append(msgs, *msg)
	}
	return msgs, nil
}

func (m *mockChatRepo) UpdateChatTimestamp(ctx context.Context, chatID int64) error {
	if chat, exists := m.chats[chatID]; exists {
		chat.UpdatedAt = time.Now()
		return nil
	}
	return repositories.ErrChatNotFound
}

func setupChatHandler() (*ChatHandler, *mockChatRepo) {
	repo := newMockChatRepo()
	llmHandler = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Mock LLM response
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": "hello world",
					},
				},
			},
		})
	}

	llm := services.GetLLMService(config.Config{})
	chatService := services.NewChatService(repo, llm)
	return NewChatHandler(chatService), repo
}

func addContextParams(req *http.Request, params map[string]string) *http.Request {
	routeCtx := chi.NewRouteContext()
	for k, v := range params {
		routeCtx.URLParams.Add(k, v)
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
}

func Test_should_list_chats_when_authenticated(t *testing.T) {
	handler, repo := setupChatHandler()
	repo.CreateChat(context.Background(), 1, "test chat")

	req, _ := http.NewRequest(http.MethodGet, "/chats", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, int64(1))
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ListChats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected %v, got %v", http.StatusOK, rr.Code)
	}

	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	items := resp["items"].([]any)
	if len(items) != 1 {
		t.Errorf("expected 1 chat, got %v", len(items))
	}
}

func Test_should_fail_list_chats_when_unauthenticated(t *testing.T) {
	handler, _ := setupChatHandler()

	req, _ := http.NewRequest(http.MethodGet, "/chats", nil)
	rr := httptest.NewRecorder()
	handler.ListChats(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected %v, got %v", http.StatusUnauthorized, rr.Code)
	}
}

func Test_should_create_chat_when_authenticated(t *testing.T) {
	handler, _ := setupChatHandler()

	body := map[string]string{
		"title": "new chat",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPost, "/chats", bytes.NewBuffer(jsonBody))
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, int64(1))
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.CreateChat(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected %v, got %v", http.StatusCreated, rr.Code)
	}
}

func Test_should_fail_create_chat_when_unauthenticated(t *testing.T) {
	handler, _ := setupChatHandler()

	body := map[string]string{
		"title": "new chat",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPost, "/chats", bytes.NewBuffer(jsonBody))
	rr := httptest.NewRecorder()
	handler.CreateChat(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected %v, got %v", http.StatusUnauthorized, rr.Code)
	}
}

func Test_should_fail_create_chat_when_invalid_body(t *testing.T) {
	handler, _ := setupChatHandler()

	req, _ := http.NewRequest(http.MethodPost, "/chats", bytes.NewBuffer([]byte("invalid")))
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, int64(1))
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.CreateChat(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %v, got %v", http.StatusBadRequest, rr.Code)
	}
}

func Test_should_fail_create_chat_when_user_not_found(t *testing.T) {
	handler, _ := setupChatHandler()

	body := map[string]string{
		"title": "new chat",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPost, "/chats", bytes.NewBuffer(jsonBody))
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, int64(999)) // 999 triggers ErrUserNotFound
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.CreateChat(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected %v, got %v", http.StatusUnauthorized, rr.Code)
	}
}

func Test_should_list_messages_when_valid(t *testing.T) {
	handler, repo := setupChatHandler()
	chat, _ := repo.CreateChat(context.Background(), 1, "test chat")
	repo.CreateMessage(context.Background(), chat.ID, "user", "hi")

	req, _ := http.NewRequest(http.MethodGet, "/chats/"+chat.Slug+"/messages", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, int64(1))
	req = req.WithContext(ctx)
	req = addContextParams(req, map[string]string{"chatSlug": chat.Slug})

	rr := httptest.NewRecorder()
	handler.ListMessages(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected %v, got %v", http.StatusOK, rr.Code)
	}
}

func Test_should_fail_list_messages_when_unauthenticated(t *testing.T) {
	handler, _ := setupChatHandler()

	req, _ := http.NewRequest(http.MethodGet, "/chats/slug/messages", nil)
	rr := httptest.NewRecorder()
	handler.ListMessages(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected %v, got %v", http.StatusUnauthorized, rr.Code)
	}
}

func Test_should_fail_list_messages_when_missing_slug(t *testing.T) {
	handler, _ := setupChatHandler()

	req, _ := http.NewRequest(http.MethodGet, "/chats//messages", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, int64(1))
	req = req.WithContext(ctx)
	req = addContextParams(req, map[string]string{"chatSlug": "  "}) // empty

	rr := httptest.NewRecorder()
	handler.ListMessages(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %v, got %v", http.StatusBadRequest, rr.Code)
	}
}

func Test_should_fail_list_messages_when_chat_not_found(t *testing.T) {
	handler, _ := setupChatHandler()

	req, _ := http.NewRequest(http.MethodGet, "/chats/invalid-slug/messages", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, int64(1))
	req = req.WithContext(ctx)
	req = addContextParams(req, map[string]string{"chatSlug": "invalid-slug"})

	rr := httptest.NewRecorder()
	handler.ListMessages(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected %v, got %v", http.StatusNotFound, rr.Code)
	}
}

func Test_should_send_message_when_valid(t *testing.T) {
	handler, repo := setupChatHandler()
	chat, _ := repo.CreateChat(context.Background(), 1, "test chat")

	body := map[string]string{
		"content": "hi",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPost, "/chats/"+chat.Slug+"/messages", bytes.NewBuffer(jsonBody))
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, int64(1))
	req = req.WithContext(ctx)
	req = addContextParams(req, map[string]string{"chatSlug": chat.Slug})

	rr := httptest.NewRecorder()
	handler.SendMessage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected %v, got %v", http.StatusOK, rr.Code)
	}
}

func Test_should_fail_send_message_when_unauthenticated(t *testing.T) {
	handler, _ := setupChatHandler()

	req, _ := http.NewRequest(http.MethodPost, "/chats/slug/messages", nil)
	rr := httptest.NewRecorder()
	handler.SendMessage(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected %v, got %v", http.StatusUnauthorized, rr.Code)
	}
}

func Test_should_fail_send_message_when_missing_slug(t *testing.T) {
	handler, _ := setupChatHandler()

	req, _ := http.NewRequest(http.MethodPost, "/chats//messages", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, int64(1))
	req = req.WithContext(ctx)
	req = addContextParams(req, map[string]string{"chatSlug": "  "})

	rr := httptest.NewRecorder()
	handler.SendMessage(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %v, got %v", http.StatusBadRequest, rr.Code)
	}
}

func Test_should_fail_send_message_when_invalid_body(t *testing.T) {
	handler, repo := setupChatHandler()
	chat, _ := repo.CreateChat(context.Background(), 1, "test chat")

	req, _ := http.NewRequest(http.MethodPost, "/chats/"+chat.Slug+"/messages", bytes.NewBuffer([]byte("invalid")))
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, int64(1))
	req = req.WithContext(ctx)
	req = addContextParams(req, map[string]string{"chatSlug": chat.Slug})

	rr := httptest.NewRecorder()
	handler.SendMessage(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %v, got %v", http.StatusBadRequest, rr.Code)
	}
}

func Test_should_fail_send_message_when_chat_not_found(t *testing.T) {
	handler, _ := setupChatHandler()

	body := map[string]string{
		"content": "hi",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPost, "/chats/invalid-slug/messages", bytes.NewBuffer(jsonBody))
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, int64(1))
	req = req.WithContext(ctx)
	req = addContextParams(req, map[string]string{"chatSlug": "invalid-slug"})

	rr := httptest.NewRecorder()
	handler.SendMessage(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected %v, got %v", http.StatusNotFound, rr.Code)
	}
}

func Test_should_send_message_stream_when_valid(t *testing.T) {
	handler, repo := setupChatHandler()
	chat, _ := repo.CreateChat(context.Background(), 1, "test chat")

	llmHandler = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// Simulate SSE from LLM Service
		w.Write([]byte("data: {\"choices\": [{\"delta\": {\"content\": \"hello\"}}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}

	body := map[string]string{
		"content": "hi",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPost, "/chats/"+chat.Slug+"/messages/stream", bytes.NewBuffer(jsonBody))
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, int64(1))
	req = req.WithContext(ctx)
	req = addContextParams(req, map[string]string{"chatSlug": chat.Slug})

	rr := httptest.NewRecorder()
	handler.SendMessageStream(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected %v, got %v", http.StatusOK, rr.Code)
	}
	if rr.Body.Len() == 0 {
		t.Errorf("expected stream data, got empty")
	}
}

func Test_should_fail_send_message_stream_when_unauthenticated(t *testing.T) {
	handler, _ := setupChatHandler()

	req, _ := http.NewRequest(http.MethodPost, "/chats/slug/messages/stream", nil)
	rr := httptest.NewRecorder()
	handler.SendMessageStream(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected %v, got %v", http.StatusUnauthorized, rr.Code)
	}
}

func Test_should_fail_send_message_stream_when_missing_slug(t *testing.T) {
	handler, _ := setupChatHandler()

	req, _ := http.NewRequest(http.MethodPost, "/chats//messages/stream", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, int64(1))
	req = req.WithContext(ctx)
	req = addContextParams(req, map[string]string{"chatSlug": "  "})

	rr := httptest.NewRecorder()
	handler.SendMessageStream(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %v, got %v", http.StatusBadRequest, rr.Code)
	}
}

func Test_should_fail_send_message_stream_when_invalid_body(t *testing.T) {
	handler, repo := setupChatHandler()
	chat, _ := repo.CreateChat(context.Background(), 1, "test chat")

	req, _ := http.NewRequest(http.MethodPost, "/chats/"+chat.Slug+"/messages/stream", bytes.NewBuffer([]byte("invalid")))
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, int64(1))
	req = req.WithContext(ctx)
	req = addContextParams(req, map[string]string{"chatSlug": chat.Slug})

	rr := httptest.NewRecorder()
	handler.SendMessageStream(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %v, got %v", http.StatusBadRequest, rr.Code)
	}
}

func Test_should_fail_send_message_stream_when_chat_not_found(t *testing.T) {
	handler, _ := setupChatHandler()

	body := map[string]string{
		"content": "hi",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPost, "/chats/invalid/messages/stream", bytes.NewBuffer(jsonBody))
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, int64(1))
	req = req.WithContext(ctx)
	req = addContextParams(req, map[string]string{"chatSlug": "invalid"})

	rr := httptest.NewRecorder()
	handler.SendMessageStream(rr, req)

	// Since httptest.ResponseRecorder does implement http.Flusher (since go 1.7), streaming will attempt to start.
	// But because the chat isn't found, the service will return an error, written as SSE "error".
	if rr.Code != http.StatusOK {
		t.Errorf("expected %v, got %v", http.StatusOK, rr.Code)
	}

	if !bytes.Contains(rr.Body.Bytes(), []byte("event: error")) {
		t.Errorf("expected error event, got %s", rr.Body.String())
	}
}

func Test_should_fail_send_message_stream_when_flusher_not_supported(t *testing.T) {
	handler, repo := setupChatHandler()
	chat, _ := repo.CreateChat(context.Background(), 1, "test chat")

	body := map[string]string{
		"content": "hi",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPost, "/chats/"+chat.Slug+"/messages/stream", bytes.NewBuffer(jsonBody))
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, int64(1))
	req = req.WithContext(ctx)
	req = addContextParams(req, map[string]string{"chatSlug": chat.Slug})

	type noFlusher struct {
		http.ResponseWriter
	}

	rr := httptest.NewRecorder()
	nf := noFlusher{rr}
	handler.SendMessageStream(nf, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected %v, got %v", http.StatusInternalServerError, rr.Code)
	}
}
