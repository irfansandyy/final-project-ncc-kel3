package repositories

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5/pgconn"
)

func Test_should_create_chat_successfully_when_valid_input_provided(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresChatRepository(db)
	ctx := context.Background()
	now := time.Now()

	rows := sqlmock.NewRows([]string{"id", "user_id", "title", "slug", "created_at", "updated_at"}).
		AddRow(1, 2, "Test Chat", "generated-slug-123", now, now)

	// Since generateChatSlug uses crypto/rand, we can't easily predict the slug in the test,
	// so we match any slug argument
	mock.ExpectQuery("^\\s*INSERT INTO chats").
		WithArgs(2, "Test Chat", sqlmock.AnyArg()).
		WillReturnRows(rows)

	chat, err := repo.CreateChat(ctx, 2, "Test Chat")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if chat.ID != 1 || chat.Title != "Test Chat" {
		t.Errorf("unexpected chat returned: %+v", chat)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func Test_should_return_errusernotfound_when_create_chat_fails_with_foreign_key_violation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresChatRepository(db)
	ctx := context.Background()

	pgErr := &pgconn.PgError{Code: "23503", ConstraintName: "chats_user_id_fkey"}

	mock.ExpectQuery("^\\s*INSERT INTO chats").
		WithArgs(999, "Test Chat", sqlmock.AnyArg()).
		WillReturnError(pgErr)

	_, err = repo.CreateChat(ctx, 999, "Test Chat")
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func Test_should_return_error_when_create_chat_fails_with_generic_error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresChatRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("^\\s*INSERT INTO chats").
		WithArgs(2, "Test Chat", sqlmock.AnyArg()).
		WillReturnError(errors.New("db error"))

	_, err = repo.CreateChat(ctx, 2, "Test Chat")
	if err == nil || err.Error() != "db error" {
		t.Errorf("expected db error, got %v", err)
	}
}

func Test_should_list_chats_by_user_successfully(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresChatRepository(db)
	ctx := context.Background()
	now := time.Now()

	rows := sqlmock.NewRows([]string{"id", "user_id", "title", "slug", "created_at", "updated_at"}).
		AddRow(1, 2, "Chat 1", "slug1", now, now).
		AddRow(2, 2, "Chat 2", "slug2", now, now)

	mock.ExpectQuery("^\\s*SELECT id, user_id, title, COALESCE\\(slug, 'chat-' \\|\\| id::text\\), created_at, updated_at").
		WithArgs(2).
		WillReturnRows(rows)

	chats, err := repo.ListChatsByUser(ctx, 2)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if len(chats) != 2 {
		t.Errorf("expected 2 chats, got %d", len(chats))
	}
}

func Test_should_return_error_when_list_chats_by_user_query_fails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresChatRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("^\\s*SELECT id, user_id, title").
		WithArgs(2).
		WillReturnError(errors.New("db error"))

	_, err = repo.ListChatsByUser(ctx, 2)
	if err == nil || err.Error() != "db error" {
		t.Errorf("expected db error, got %v", err)
	}
}

func Test_should_return_error_when_list_chats_by_user_rows_error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresChatRepository(db)
	ctx := context.Background()
	now := time.Now()

	rows := sqlmock.NewRows([]string{"id", "user_id", "title", "slug", "created_at", "updated_at"}).
		AddRow(1, 2, "Chat 1", "slug1", now, now).
		RowError(0, errors.New("row error"))

	mock.ExpectQuery("^\\s*SELECT id, user_id, title").
		WithArgs(2).
		WillReturnRows(rows)

	_, err = repo.ListChatsByUser(ctx, 2)
	if err == nil || err.Error() != "row error" {
		t.Errorf("expected row error, got %v", err)
	}
}

func Test_should_return_chat_when_get_chat_by_id_exists(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresChatRepository(db)
	ctx := context.Background()
	now := time.Now()

	rows := sqlmock.NewRows([]string{"id", "user_id", "title", "slug", "created_at", "updated_at"}).
		AddRow(1, 2, "Chat 1", "slug1", now, now)

	mock.ExpectQuery("^\\s*SELECT id, user_id, title, COALESCE\\(slug, 'chat-' \\|\\| id::text\\), created_at, updated_at").
		WithArgs(1, 2).
		WillReturnRows(rows)

	chat, err := repo.GetChatByID(ctx, 1, 2)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if chat.ID != 1 {
		t.Errorf("expected ID 1, got %d", chat.ID)
	}
}

func Test_should_return_errchatnotfound_when_get_chat_by_id_does_not_exist(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresChatRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("^\\s*SELECT id, user_id, title").
		WithArgs(1, 2).
		WillReturnError(sql.ErrNoRows)

	_, err = repo.GetChatByID(ctx, 1, 2)
	if !errors.Is(err, ErrChatNotFound) {
		t.Errorf("expected ErrChatNotFound, got %v", err)
	}
}

func Test_should_return_error_when_get_chat_by_id_fails_with_generic_error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresChatRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("^\\s*SELECT id, user_id, title").
		WithArgs(1, 2).
		WillReturnError(errors.New("db error"))

	_, err = repo.GetChatByID(ctx, 1, 2)
	if err == nil || err.Error() != "db error" {
		t.Errorf("expected db error, got %v", err)
	}
}

func Test_should_return_chat_when_get_chat_by_slug_exists(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresChatRepository(db)
	ctx := context.Background()
	now := time.Now()

	rows := sqlmock.NewRows([]string{"id", "user_id", "title", "slug", "created_at", "updated_at"}).
		AddRow(1, 2, "Chat 1", "slug1", now, now)

	mock.ExpectQuery("^\\s*SELECT id, user_id, title, COALESCE\\(slug, 'chat-' \\|\\| id::text\\), created_at, updated_at").
		WithArgs("slug1", 2).
		WillReturnRows(rows)

	chat, err := repo.GetChatBySlug(ctx, "slug1", 2)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if chat.ID != 1 {
		t.Errorf("expected ID 1, got %d", chat.ID)
	}
}

func Test_should_return_errchatnotfound_when_get_chat_by_slug_does_not_exist(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresChatRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("^\\s*SELECT id, user_id, title").
		WithArgs("slug1", 2).
		WillReturnError(sql.ErrNoRows)

	_, err = repo.GetChatBySlug(ctx, "slug1", 2)
	if !errors.Is(err, ErrChatNotFound) {
		t.Errorf("expected ErrChatNotFound, got %v", err)
	}
}

func Test_should_return_error_when_get_chat_by_slug_fails_with_generic_error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresChatRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("^\\s*SELECT id, user_id, title").
		WithArgs("slug1", 2).
		WillReturnError(errors.New("db error"))

	_, err = repo.GetChatBySlug(ctx, "slug1", 2)
	if err == nil || err.Error() != "db error" {
		t.Errorf("expected db error, got %v", err)
	}
}

func Test_should_update_chat_title_successfully_when_chat_exists(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresChatRepository(db)
	ctx := context.Background()
	now := time.Now()

	rows := sqlmock.NewRows([]string{"id", "user_id", "title", "slug", "created_at", "updated_at"}).
		AddRow(1, 2, "New Title", "slug1", now, now)

	mock.ExpectQuery("^\\s*UPDATE chats").
		WithArgs("New Title", 1, 2).
		WillReturnRows(rows)

	chat, err := repo.UpdateChatTitle(ctx, 1, 2, "New Title")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if chat.Title != "New Title" {
		t.Errorf("expected New Title, got %s", chat.Title)
	}
}

func Test_should_return_errchatnotfound_when_update_chat_title_for_nonexistent_chat(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresChatRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("^\\s*UPDATE chats").
		WithArgs("New Title", 1, 2).
		WillReturnError(sql.ErrNoRows)

	_, err = repo.UpdateChatTitle(ctx, 1, 2, "New Title")
	if !errors.Is(err, ErrChatNotFound) {
		t.Errorf("expected ErrChatNotFound, got %v", err)
	}
}

func Test_should_return_error_when_update_chat_title_fails_with_generic_error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresChatRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("^\\s*UPDATE chats").
		WithArgs("New Title", 1, 2).
		WillReturnError(errors.New("db error"))

	_, err = repo.UpdateChatTitle(ctx, 1, 2, "New Title")
	if err == nil || err.Error() != "db error" {
		t.Errorf("expected db error, got %v", err)
	}
}

func Test_should_create_message_successfully_when_valid_input_provided(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresChatRepository(db)
	ctx := context.Background()
	now := time.Now()

	rows := sqlmock.NewRows([]string{"id", "chat_id", "role", "content", "created_at"}).
		AddRow(1, 1, "user", "Hello", now)

	mock.ExpectQuery("^\\s*INSERT INTO messages").
		WithArgs(1, "user", "Hello").
		WillReturnRows(rows)

	msg, err := repo.CreateMessage(ctx, 1, "user", "Hello")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if msg.ID != 1 || msg.Content != "Hello" {
		t.Errorf("unexpected message returned: %+v", msg)
	}
}

func Test_should_return_error_when_create_message_fails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresChatRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("^\\s*INSERT INTO messages").
		WithArgs(1, "user", "Hello").
		WillReturnError(errors.New("db error"))

	_, err = repo.CreateMessage(ctx, 1, "user", "Hello")
	if err == nil || err.Error() != "db error" {
		t.Errorf("expected db error, got %v", err)
	}
}

func Test_should_list_messages_by_chat_successfully(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresChatRepository(db)
	ctx := context.Background()
	now := time.Now()

	// GetChatByID mock
	chatRows := sqlmock.NewRows([]string{"id", "user_id", "title", "slug", "created_at", "updated_at"}).
		AddRow(1, 2, "Chat 1", "slug1", now, now)

	mock.ExpectQuery("^\\s*SELECT id, user_id, title").
		WithArgs(1, 2).
		WillReturnRows(chatRows)

	msgRows := sqlmock.NewRows([]string{"id", "chat_id", "role", "content", "created_at"}).
		AddRow(1, 1, "user", "Hello", now).
		AddRow(2, 1, "bot", "Hi", now)

	mock.ExpectQuery("^\\s*SELECT id, chat_id, role, content, created_at").
		WithArgs(1).
		WillReturnRows(msgRows)

	msgs, err := repo.ListMessagesByChat(ctx, 1, 2, 0)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if len(msgs) != 2 {
		t.Errorf("expected 2 messages, got %d", len(msgs))
	}
}

func Test_should_list_messages_by_chat_successfully_with_limit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresChatRepository(db)
	ctx := context.Background()
	now := time.Now()

	// GetChatByID mock
	chatRows := sqlmock.NewRows([]string{"id", "user_id", "title", "slug", "created_at", "updated_at"}).
		AddRow(1, 2, "Chat 1", "slug1", now, now)

	mock.ExpectQuery("^\\s*SELECT id, user_id, title").
		WithArgs(1, 2).
		WillReturnRows(chatRows)

	msgRows := sqlmock.NewRows([]string{"id", "chat_id", "role", "content", "created_at"}).
		AddRow(1, 1, "user", "Hello", now)

	mock.ExpectQuery("^\\s*SELECT id, chat_id, role, content, created_at").
		WithArgs(1, 10).
		WillReturnRows(msgRows)

	msgs, err := repo.ListMessagesByChat(ctx, 1, 2, 10)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if len(msgs) != 1 {
		t.Errorf("expected 1 messages, got %d", len(msgs))
	}
}

func Test_should_return_error_when_list_messages_by_chat_chat_not_found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresChatRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("^\\s*SELECT id, user_id, title").
		WithArgs(1, 2).
		WillReturnError(sql.ErrNoRows)

	_, err = repo.ListMessagesByChat(ctx, 1, 2, 0)
	if !errors.Is(err, ErrChatNotFound) {
		t.Errorf("expected ErrChatNotFound, got %v", err)
	}
}

func Test_should_return_error_when_list_messages_by_chat_query_fails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresChatRepository(db)
	ctx := context.Background()
	now := time.Now()

	chatRows := sqlmock.NewRows([]string{"id", "user_id", "title", "slug", "created_at", "updated_at"}).
		AddRow(1, 2, "Chat 1", "slug1", now, now)

	mock.ExpectQuery("^\\s*SELECT id, user_id, title").
		WithArgs(1, 2).
		WillReturnRows(chatRows)

	mock.ExpectQuery("^\\s*SELECT id, chat_id, role, content, created_at").
		WithArgs(1).
		WillReturnError(errors.New("db error"))

	_, err = repo.ListMessagesByChat(ctx, 1, 2, 0)
	if err == nil || err.Error() != "db error" {
		t.Errorf("expected db error, got %v", err)
	}
}

func Test_should_return_error_when_list_messages_by_chat_rows_error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresChatRepository(db)
	ctx := context.Background()
	now := time.Now()

	chatRows := sqlmock.NewRows([]string{"id", "user_id", "title", "slug", "created_at", "updated_at"}).
		AddRow(1, 2, "Chat 1", "slug1", now, now)

	mock.ExpectQuery("^\\s*SELECT id, user_id, title").
		WithArgs(1, 2).
		WillReturnRows(chatRows)

	msgRows := sqlmock.NewRows([]string{"id", "chat_id", "role", "content", "created_at"}).
		AddRow(1, 1, "user", "Hello", now).
		RowError(0, errors.New("row error"))

	mock.ExpectQuery("^\\s*SELECT id, chat_id, role, content, created_at").
		WithArgs(1).
		WillReturnRows(msgRows)

	_, err = repo.ListMessagesByChat(ctx, 1, 2, 0)
	if err == nil || err.Error() != "row error" {
		t.Errorf("expected row error, got %v", err)
	}
}

func Test_should_update_chat_timestamp_successfully(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresChatRepository(db)
	ctx := context.Background()

	mock.ExpectExec("^\\s*UPDATE chats SET updated_at = NOW\\(\\) WHERE id = \\$1").
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.UpdateChatTimestamp(ctx, 1)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func Test_should_return_error_when_update_chat_timestamp_fails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresChatRepository(db)
	ctx := context.Background()

	mock.ExpectExec("^\\s*UPDATE chats").
		WithArgs(1).
		WillReturnError(errors.New("db error"))

	err = repo.UpdateChatTimestamp(ctx, 1)
	if err == nil || err.Error() != "db error" {
		t.Errorf("expected db error, got %v", err)
	}
}
