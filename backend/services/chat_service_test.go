package services

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"app-backend/repositories"

	"github.com/DATA-DOG/go-sqlmock"
)

func Test_should_create_chat_with_default_title_when_empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	chatRepo := repositories.NewPostgresChatRepository(db)
	service := NewChatService(chatRepo, nil)

	rows := sqlmock.NewRows([]string{"id", "user_id", "title", "slug", "created_at", "updated_at"}).
		AddRow(1, 1, "New Chat", "slug-1", time.Now(), time.Now())

	mock.ExpectQuery(`INSERT INTO chats \(user_id, title, slug\) VALUES \(\$1, \$2, \$3\) RETURNING id, user_id, title, COALESCE\(slug, ''\), created_at, updated_at`).
		WithArgs(1, "New Chat", sqlmock.AnyArg()).
		WillReturnRows(rows)

	chat, err := service.CreateChat(context.Background(), 1, "   ")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if chat.Title != "New Chat" {
		t.Errorf("expected 'New Chat', got %s", chat.Title)
	}
}

func Test_should_create_chat_with_provided_title(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	chatRepo := repositories.NewPostgresChatRepository(db)
	service := NewChatService(chatRepo, nil)

	rows := sqlmock.NewRows([]string{"id", "user_id", "title", "slug", "created_at", "updated_at"}).
		AddRow(1, 1, "My Chat", "slug-1", time.Now(), time.Now())

	mock.ExpectQuery(`INSERT INTO chats \(user_id, title, slug\) VALUES \(\$1, \$2, \$3\) RETURNING id, user_id, title, COALESCE\(slug, ''\), created_at, updated_at`).
		WithArgs(1, "My Chat", sqlmock.AnyArg()).
		WillReturnRows(rows)

	chat, err := service.CreateChat(context.Background(), 1, "My Chat")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if chat.Title != "My Chat" {
		t.Errorf("expected 'My Chat', got %s", chat.Title)
	}
}

func Test_should_list_chats(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	chatRepo := repositories.NewPostgresChatRepository(db)
	service := NewChatService(chatRepo, nil)

	rows := sqlmock.NewRows([]string{"id", "user_id", "title", "slug", "created_at", "updated_at"}).
		AddRow(1, 1, "Chat 1", "slug-1", time.Now(), time.Now())

	mock.ExpectQuery(`SELECT id, user_id, title, COALESCE\(slug, 'chat-' \|\| id::text\), created_at, updated_at FROM chats WHERE user_id = \$1 ORDER BY updated_at DESC`).
		WithArgs(1).
		WillReturnRows(rows)

	chats, err := service.ListChats(context.Background(), 1)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(chats) != 1 {
		t.Errorf("expected 1 chat, got %d", len(chats))
	}
}

func Test_should_return_error_when_list_messages_chat_not_found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	chatRepo := repositories.NewPostgresChatRepository(db)
	service := NewChatService(chatRepo, nil)

	mock.ExpectQuery(`SELECT id, user_id, title, COALESCE\(slug, 'chat-' \|\| id::text\), created_at, updated_at FROM chats WHERE COALESCE\(slug, 'chat-' \|\| id::text\) = \$1 AND user_id = \$2`).
		WithArgs("slug-1", 1).
		WillReturnError(sql.ErrNoRows)

	_, err = service.ListMessages(context.Background(), 1, "slug-1")
	if !errors.Is(err, repositories.ErrChatNotFound) {
		t.Errorf("expected ErrChatNotFound, got %v", err)
	}
}

func Test_should_list_messages_successfully(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	chatRepo := repositories.NewPostgresChatRepository(db)
	service := NewChatService(chatRepo, nil)

	chatRows := sqlmock.NewRows([]string{"id", "user_id", "title", "slug", "created_at", "updated_at"}).
		AddRow(1, 1, "Chat 1", "slug-1", time.Now(), time.Now())
	mock.ExpectQuery(`SELECT id, user_id, title, COALESCE\(slug, 'chat-' \|\| id::text\), created_at, updated_at FROM chats WHERE COALESCE\(slug, 'chat-' \|\| id::text\) = \$1 AND user_id = \$2`).
		WithArgs("slug-1", 1).
		WillReturnRows(chatRows)

	mock.ExpectQuery(`SELECT id, user_id, title, COALESCE\(slug, 'chat-' \|\| id::text\), created_at, updated_at FROM chats WHERE id = \$1 AND user_id = \$2`).
		WithArgs(1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "slug", "created_at", "updated_at"}).AddRow(1, 1, "Chat 1", "slug-1", time.Now(), time.Now()))

	msgRows := sqlmock.NewRows([]string{"id", "chat_id", "role", "content", "created_at"}).
		AddRow(1, 1, "user", "hi", time.Now())
	mock.ExpectQuery(`SELECT id, chat_id, role, content, created_at FROM messages WHERE chat_id = \$1 ORDER BY created_at ASC`).
		WithArgs(1).
		WillReturnRows(msgRows)

	messages, err := service.ListMessages(context.Background(), 1, "slug-1")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(messages))
	}
}

func Test_should_return_error_when_send_message_empty(t *testing.T) {
	service := NewChatService(nil, nil)
	_, _, err := service.SendMessage(context.Background(), 1, "slug", "  ")
	if err == nil || err.Error() != "message content is required" {
		t.Errorf("expected error message content is required, got %v", err)
	}
}

func Test_should_send_message_successfully(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices":[{"message":{"content":"Hello user"}}]}`))
	}))
	defer ts.Close()

	llm := &LLMService{
		baseURL: ts.URL,
		model:   "test",
		client:  ts.Client(),
	}

	chatRepo := repositories.NewPostgresChatRepository(db)
	service := NewChatService(chatRepo, llm)

	chatRows := sqlmock.NewRows([]string{"id", "user_id", "title", "slug", "created_at", "updated_at"}).
		AddRow(1, 1, "Chat 1", "slug-1", time.Now(), time.Now())
	mock.ExpectQuery(`SELECT id, user_id, title, COALESCE\(slug, 'chat-' \|\| id::text\), created_at, updated_at FROM chats WHERE COALESCE\(slug, 'chat-' \|\| id::text\) = \$1 AND user_id = \$2`).
		WithArgs("slug-1", 1).
		WillReturnRows(chatRows)

	// ListMessagesByChat to check if hadMessages
	mock.ExpectQuery(`SELECT id, user_id, title, COALESCE\(slug, 'chat-' \|\| id::text\), created_at, updated_at FROM chats WHERE id = \$1 AND user_id = \$2`).
		WithArgs(1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "slug", "created_at", "updated_at"}).AddRow(1, 1, "Chat 1", "slug-1", time.Now(), time.Now()))
	msgRows1 := sqlmock.NewRows([]string{"id", "chat_id", "role", "content", "created_at"}).
		AddRow(1, 1, "user", "hi", time.Now())
	mock.ExpectQuery(`SELECT id, chat_id, role, content, created_at FROM messages WHERE chat_id = \$1 ORDER BY created_at ASC LIMIT \$2`).
		WithArgs(1, 1).
		WillReturnRows(msgRows1)

	// Create user message
	mock.ExpectQuery(`INSERT INTO messages \(chat_id, role, content\) VALUES \(\$1, \$2, \$3\) RETURNING id, chat_id, role, content, created_at`).
		WithArgs(1, "user", "hi").
		WillReturnRows(sqlmock.NewRows([]string{"id", "chat_id", "role", "content", "created_at"}).AddRow(2, 1, "user", "hi", time.Now()))

	// List history for LLM
	mock.ExpectQuery(`SELECT id, user_id, title, COALESCE\(slug, 'chat-' \|\| id::text\), created_at, updated_at FROM chats WHERE id = \$1 AND user_id = \$2`).
		WithArgs(1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "slug", "created_at", "updated_at"}).AddRow(1, 1, "Chat 1", "slug-1", time.Now(), time.Now()))
	msgRows2 := sqlmock.NewRows([]string{"id", "chat_id", "role", "content", "created_at"}).
		AddRow(1, 1, "user", "hi", time.Now())
	mock.ExpectQuery(`SELECT id, chat_id, role, content, created_at FROM messages WHERE chat_id = \$1 ORDER BY created_at ASC LIMIT \$2`).
		WithArgs(1, 20).
		WillReturnRows(msgRows2)

	// Create assistant message
	mock.ExpectQuery(`INSERT INTO messages \(chat_id, role, content\) VALUES \(\$1, \$2, \$3\) RETURNING id, chat_id, role, content, created_at`).
		WithArgs(1, "assistant", "Hello user").
		WillReturnRows(sqlmock.NewRows([]string{"id", "chat_id", "role", "content", "created_at"}).AddRow(3, 1, "assistant", "Hello user", time.Now()))

	// Update timestamp
	mock.ExpectExec(`UPDATE chats SET updated_at = NOW\(\) WHERE id = \$1`).
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(1, 1))

	uMsg, aMsg, err := service.SendMessage(context.Background(), 1, "slug-1", "hi")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if uMsg.Content != "hi" {
		t.Errorf("expected user msg hi, got %s", uMsg.Content)
	}
	if aMsg.Content != "Hello user" {
		t.Errorf("expected assist msg, got %s", aMsg.Content)
	}
}

func Test_should_return_error_when_send_message_stream_empty(t *testing.T) {
	service := NewChatService(nil, nil)
	_, _, err := service.SendMessageStream(context.Background(), 1, "slug", "  ", nil)
	if err == nil || err.Error() != "message content is required" {
		t.Errorf("expected error message content is required, got %v", err)
	}
}

func Test_should_send_message_stream_successfully(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hello \"}}]}\n\n"))
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"stream\"}}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer ts.Close()

	llm := &LLMService{
		baseURL: ts.URL,
		model:   "test",
		client:  ts.Client(),
	}

	chatRepo := repositories.NewPostgresChatRepository(db)
	service := NewChatService(chatRepo, llm)

	chatRows := sqlmock.NewRows([]string{"id", "user_id", "title", "slug", "created_at", "updated_at"}).
		AddRow(1, 1, "Chat 1", "slug-1", time.Now(), time.Now())
	mock.ExpectQuery(`SELECT id, user_id, title, COALESCE\(slug, 'chat-' \|\| id::text\), created_at, updated_at FROM chats WHERE COALESCE\(slug, 'chat-' \|\| id::text\) = \$1 AND user_id = \$2`).
		WithArgs("slug-1", 1).
		WillReturnRows(chatRows)

	mock.ExpectQuery(`SELECT id, user_id, title, COALESCE\(slug, 'chat-' \|\| id::text\), created_at, updated_at FROM chats WHERE id = \$1 AND user_id = \$2`).
		WithArgs(1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "slug", "created_at", "updated_at"}).AddRow(1, 1, "Chat 1", "slug-1", time.Now(), time.Now()))
	msgRows1 := sqlmock.NewRows([]string{"id", "chat_id", "role", "content", "created_at"}).
		AddRow(1, 1, "user", "hi", time.Now())
	mock.ExpectQuery(`SELECT id, chat_id, role, content, created_at FROM messages WHERE chat_id = \$1 ORDER BY created_at ASC LIMIT \$2`).
		WithArgs(1, 1).
		WillReturnRows(msgRows1)

	mock.ExpectQuery(`INSERT INTO messages \(chat_id, role, content\) VALUES \(\$1, \$2, \$3\) RETURNING id, chat_id, role, content, created_at`).
		WithArgs(1, "user", "hi").
		WillReturnRows(sqlmock.NewRows([]string{"id", "chat_id", "role", "content", "created_at"}).AddRow(2, 1, "user", "hi", time.Now()))

	mock.ExpectQuery(`SELECT id, user_id, title, COALESCE\(slug, 'chat-' \|\| id::text\), created_at, updated_at FROM chats WHERE id = \$1 AND user_id = \$2`).
		WithArgs(1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "title", "slug", "created_at", "updated_at"}).AddRow(1, 1, "Chat 1", "slug-1", time.Now(), time.Now()))
	msgRows2 := sqlmock.NewRows([]string{"id", "chat_id", "role", "content", "created_at"}).
		AddRow(1, 1, "user", "hi", time.Now())
	mock.ExpectQuery(`SELECT id, chat_id, role, content, created_at FROM messages WHERE chat_id = \$1 ORDER BY created_at ASC LIMIT \$2`).
		WithArgs(1, 20).
		WillReturnRows(msgRows2)

	mock.ExpectQuery(`INSERT INTO messages \(chat_id, role, content\) VALUES \(\$1, \$2, \$3\) RETURNING id, chat_id, role, content, created_at`).
		WithArgs(1, "assistant", "Hello stream").
		WillReturnRows(sqlmock.NewRows([]string{"id", "chat_id", "role", "content", "created_at"}).AddRow(3, 1, "assistant", "Hello stream", time.Now()))

	mock.ExpectExec(`UPDATE chats SET updated_at = NOW\(\) WHERE id = \$1`).
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(1, 1))

	tokens := []string{}
	uMsg, aMsg, err := service.SendMessageStream(context.Background(), 1, "slug-1", "hi", func(t string) error {
		tokens = append(tokens, t)
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if uMsg.Content != "hi" {
		t.Errorf("expected user msg hi, got %s", uMsg.Content)
	}
	if aMsg.Content != "Hello stream" {
		t.Errorf("expected assist msg, got %s", aMsg.Content)
	}
	if len(tokens) != 2 {
		t.Errorf("expected 2 tokens, got %d", len(tokens))
	}
}
