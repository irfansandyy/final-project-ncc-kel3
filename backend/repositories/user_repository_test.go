package repositories

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func Test_should_create_user_successfully_when_valid_input_provided(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := NewPostgresUserRepository(db)
	ctx := context.Background()
	now := time.Now()

	rows := sqlmock.NewRows([]string{"id", "email", "username", "password_hash", "created_at"}).
		AddRow(1, "test@example.com", "testuser", "hashedpassword", now)

	mock.ExpectQuery("^\\s*INSERT INTO users").
		WithArgs("test@example.com", "hashedpassword", "testuser").
		WillReturnRows(rows)

	user, err := repo.CreateUser(ctx, "test@example.com", "hashedpassword", "testuser")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if user.ID != 1 || user.Email != "test@example.com" {
		t.Errorf("unexpected user returned: %+v", user)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func Test_should_return_error_when_create_user_fails_on_db(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresUserRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("^\\s*INSERT INTO users").
		WithArgs("test@example.com", "hashedpassword", "testuser").
		WillReturnError(errors.New("db error"))

	_, err = repo.CreateUser(ctx, "test@example.com", "hashedpassword", "testuser")
	if err == nil || err.Error() != "db error" {
		t.Errorf("expected db error, got %v", err)
	}
}

func Test_should_return_user_when_get_by_email_exists(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresUserRepository(db)
	ctx := context.Background()
	now := time.Now()

	rows := sqlmock.NewRows([]string{"id", "email", "username", "password_hash", "created_at"}).
		AddRow(1, "test@example.com", "testuser", "hashedpassword", now)

	mock.ExpectQuery("^\\s*SELECT id, email, COALESCE\\(username, ''\\), password_hash, created_at").
		WithArgs("test@example.com").
		WillReturnRows(rows)

	user, err := repo.GetByEmail(ctx, "test@example.com")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if user.ID != 1 {
		t.Errorf("expected ID 1, got %d", user.ID)
	}
}

func Test_should_return_errusernotfound_when_get_by_email_does_not_exist(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresUserRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("^\\s*SELECT id, email, COALESCE\\(username, ''\\), password_hash, created_at").
		WithArgs("nonexistent@example.com").
		WillReturnError(sql.ErrNoRows)

	_, err = repo.GetByEmail(ctx, "nonexistent@example.com")
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func Test_should_return_error_when_get_by_email_fails_with_generic_error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresUserRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("^\\s*SELECT id, email, COALESCE\\(username, ''\\), password_hash, created_at").
		WithArgs("error@example.com").
		WillReturnError(errors.New("generic error"))

	_, err = repo.GetByEmail(ctx, "error@example.com")
	if err == nil || err.Error() != "generic error" {
		t.Errorf("expected generic error, got %v", err)
	}
}

func Test_should_return_user_when_get_by_id_exists(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresUserRepository(db)
	ctx := context.Background()
	now := time.Now()

	rows := sqlmock.NewRows([]string{"id", "email", "username", "password_hash", "created_at"}).
		AddRow(1, "test@example.com", "testuser", "hashedpassword", now)

	mock.ExpectQuery("^\\s*SELECT id, email, COALESCE\\(username, ''\\), password_hash, created_at").
		WithArgs(1).
		WillReturnRows(rows)

	user, err := repo.GetByID(ctx, 1)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if user.ID != 1 {
		t.Errorf("expected ID 1, got %d", user.ID)
	}
}

func Test_should_return_errusernotfound_when_get_by_id_does_not_exist(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresUserRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("^\\s*SELECT id, email, COALESCE\\(username, ''\\), password_hash, created_at").
		WithArgs(999).
		WillReturnError(sql.ErrNoRows)

	_, err = repo.GetByID(ctx, 999)
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func Test_should_return_error_when_get_by_id_fails_with_generic_error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresUserRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("^\\s*SELECT id, email, COALESCE\\(username, ''\\), password_hash, created_at").
		WithArgs(999).
		WillReturnError(errors.New("generic db error"))

	_, err = repo.GetByID(ctx, 999)
	if err == nil || err.Error() != "generic db error" {
		t.Errorf("expected generic db error, got %v", err)
	}
}

func Test_should_update_username_successfully_when_user_exists(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresUserRepository(db)
	ctx := context.Background()
	now := time.Now()

	rows := sqlmock.NewRows([]string{"id", "email", "username", "password_hash", "created_at"}).
		AddRow(1, "test@example.com", "newusername", "hashedpassword", now)

	mock.ExpectQuery("^\\s*UPDATE users").
		WithArgs("newusername", 1).
		WillReturnRows(rows)

	user, err := repo.UpdateUsername(ctx, 1, "newusername")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if user.Username != "newusername" {
		t.Errorf("expected username newusername, got %s", user.Username)
	}
}

func Test_should_return_errusernotfound_when_update_username_for_nonexistent_user(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresUserRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("^\\s*UPDATE users").
		WithArgs("newusername", 999).
		WillReturnError(sql.ErrNoRows)

	_, err = repo.UpdateUsername(ctx, 999, "newusername")
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func Test_should_return_error_when_update_username_fails_with_generic_error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	repo := NewPostgresUserRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("^\\s*UPDATE users").
		WithArgs("newusername", 999).
		WillReturnError(errors.New("db error"))

	_, err = repo.UpdateUsername(ctx, 999, "newusername")
	if err == nil || err.Error() != "db error" {
		t.Errorf("expected db error, got %v", err)
	}
}
