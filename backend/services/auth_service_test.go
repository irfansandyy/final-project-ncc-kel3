package services

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"app-backend/repositories"

	"github.com/DATA-DOG/go-sqlmock"
	"golang.org/x/crypto/bcrypt"
)

func Test_should_return_error_when_registering_existing_email(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	userRepo := repositories.NewPostgresUserRepository(db)
	service := NewAuthService(userRepo, "secret", time.Hour)

	rows := sqlmock.NewRows([]string{"id", "email", "username", "password_hash", "created_at"}).
		AddRow(1, "test@test.com", "test", "hash", time.Now())
	mock.ExpectQuery(`SELECT id, email, COALESCE\(username, ''\), password_hash, created_at FROM users WHERE email = \$1`).
		WithArgs("test@test.com").
		WillReturnRows(rows)

	_, err = service.Register(context.Background(), "test@test.com", "pass", "test")
	if !errors.Is(err, ErrEmailInUse) {
		t.Errorf("expected ErrEmailInUse, got %v", err)
	}
}

func Test_should_return_error_when_get_email_fails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	userRepo := repositories.NewPostgresUserRepository(db)
	service := NewAuthService(userRepo, "secret", time.Hour)

	mock.ExpectQuery(`SELECT id, email, COALESCE\(username, ''\), password_hash, created_at FROM users WHERE email = \$1`).
		WithArgs("test@test.com").
		WillReturnError(errors.New("db error"))

	_, err = service.Register(context.Background(), "test@test.com", "pass", "test")
	if err == nil || err.Error() != "db error" {
		t.Errorf("expected db error, got %v", err)
	}
}

func Test_should_return_error_when_password_too_long(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	userRepo := repositories.NewPostgresUserRepository(db)
	service := NewAuthService(userRepo, "secret", time.Hour)

	mock.ExpectQuery(`SELECT id, email, COALESCE\(username, ''\), password_hash, created_at FROM users WHERE email = \$1`).
		WithArgs("test@test.com").
		WillReturnError(sql.ErrNoRows) // ErrUserNotFound

	longPassword := string(make([]byte, 73)) // bcrypt max length is 72
	_, err = service.Register(context.Background(), "test@test.com", longPassword, "test")
	if err == nil {
		t.Errorf("expected bcrypt error for too long password, got nil")
	}
}

func Test_should_create_user_with_email_prefix_when_username_empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	userRepo := repositories.NewPostgresUserRepository(db)
	service := NewAuthService(userRepo, "secret", time.Hour)

	mock.ExpectQuery(`SELECT id, email, COALESCE\(username, ''\), password_hash, created_at FROM users WHERE email = \$1`).
		WithArgs("test@test.com").
		WillReturnError(sql.ErrNoRows)

	rows := sqlmock.NewRows([]string{"id", "email", "username", "password_hash", "created_at"}).
		AddRow(1, "test@test.com", "test", "hash", time.Now())
	mock.ExpectQuery(`INSERT INTO users \(email, password_hash, username\) VALUES \(\$1, \$2, \$3\) RETURNING id, email, username, password_hash, created_at`).
		WithArgs("test@test.com", sqlmock.AnyArg(), "test").
		WillReturnRows(rows)

	user, err := service.Register(context.Background(), "test@test.com", "pass", "")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if user.Username != "test" {
		t.Errorf("expected username 'test', got '%s'", user.Username)
	}
}

func Test_should_create_user_with_default_username_when_email_has_no_prefix(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	userRepo := repositories.NewPostgresUserRepository(db)
	service := NewAuthService(userRepo, "secret", time.Hour)

	mock.ExpectQuery(`SELECT id, email, COALESCE\(username, ''\), password_hash, created_at FROM users WHERE email = \$1`).
		WithArgs("@test.com").
		WillReturnError(sql.ErrNoRows)

	rows := sqlmock.NewRows([]string{"id", "email", "username", "password_hash", "created_at"}).
		AddRow(1, "@test.com", "user", "hash", time.Now())
	mock.ExpectQuery(`INSERT INTO users \(email, password_hash, username\) VALUES \(\$1, \$2, \$3\) RETURNING id, email, username, password_hash, created_at`).
		WithArgs("@test.com", sqlmock.AnyArg(), "user").
		WillReturnRows(rows)

	user, err := service.Register(context.Background(), "@test.com", "pass", "  ")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if user.Username != "user" {
		t.Errorf("expected username 'user', got '%s'", user.Username)
	}
}

func Test_should_return_invalid_credentials_when_login_not_found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	userRepo := repositories.NewPostgresUserRepository(db)
	service := NewAuthService(userRepo, "secret", time.Hour)

	mock.ExpectQuery(`SELECT id, email, COALESCE\(username, ''\), password_hash, created_at FROM users WHERE email = \$1`).
		WithArgs("test@test.com").
		WillReturnError(sql.ErrNoRows)

	_, _, err = service.Login(context.Background(), "test@test.com", "pass")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func Test_should_return_error_when_login_repo_fails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	userRepo := repositories.NewPostgresUserRepository(db)
	service := NewAuthService(userRepo, "secret", time.Hour)

	mock.ExpectQuery(`SELECT id, email, COALESCE\(username, ''\), password_hash, created_at FROM users WHERE email = \$1`).
		WithArgs("test@test.com").
		WillReturnError(errors.New("db error"))

	_, _, err = service.Login(context.Background(), "test@test.com", "pass")
	if err == nil || err.Error() != "db error" {
		t.Errorf("expected db error, got %v", err)
	}
}

func Test_should_return_invalid_credentials_when_wrong_password(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	userRepo := repositories.NewPostgresUserRepository(db)
	service := NewAuthService(userRepo, "secret", time.Hour)

	hashBytes, _ := bcrypt.GenerateFromPassword([]byte("correct"), bcrypt.DefaultCost)
	rows := sqlmock.NewRows([]string{"id", "email", "username", "password_hash", "created_at"}).
		AddRow(1, "test@test.com", "test", string(hashBytes), time.Now())
	mock.ExpectQuery(`SELECT id, email, COALESCE\(username, ''\), password_hash, created_at FROM users WHERE email = \$1`).
		WithArgs("test@test.com").
		WillReturnRows(rows)

	_, _, err = service.Login(context.Background(), "test@test.com", "wrong")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func Test_should_return_token_when_login_success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	userRepo := repositories.NewPostgresUserRepository(db)
	service := NewAuthService(userRepo, "secret", time.Hour)

	hashBytes, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.DefaultCost)
	rows := sqlmock.NewRows([]string{"id", "email", "username", "password_hash", "created_at"}).
		AddRow(1, "test@test.com", "test", string(hashBytes), time.Now())
	mock.ExpectQuery(`SELECT id, email, COALESCE\(username, ''\), password_hash, created_at FROM users WHERE email = \$1`).
		WithArgs("test@test.com").
		WillReturnRows(rows)

	token, user, err := service.Login(context.Background(), "test@test.com", "pass")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if token == "" {
		t.Errorf("expected token, got empty")
	}
	if user.ID != 1 {
		t.Errorf("expected user id 1, got %d", user.ID)
	}
}

func Test_should_return_error_when_parse_invalid_token(t *testing.T) {
	service := NewAuthService(nil, "secret", time.Hour)
	_, err := service.ParseToken("invalid")
	if err == nil {
		t.Errorf("expected error for invalid token")
	}
}

func Test_should_parse_token_successfully(t *testing.T) {
	service := NewAuthService(nil, "secret", time.Hour)
	token, _ := service.createJWT(123)
	userID, err := service.ParseToken(token)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if userID != 123 {
		t.Errorf("expected user id 123, got %d", userID)
	}
}

func Test_should_return_error_when_parse_invalid_signature(t *testing.T) {
	service1 := NewAuthService(nil, "secret1", time.Hour)
	service2 := NewAuthService(nil, "secret2", time.Hour)
	token, _ := service1.createJWT(123)
	_, err := service2.ParseToken(token)
	if err == nil {
		t.Errorf("expected error for invalid signature")
	}
}

func Test_should_return_profile(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	userRepo := repositories.NewPostgresUserRepository(db)
	service := NewAuthService(userRepo, "secret", time.Hour)

	rows := sqlmock.NewRows([]string{"id", "email", "username", "password_hash", "created_at"}).
		AddRow(1, "test@test.com", "test", "hash", time.Now())
	mock.ExpectQuery(`SELECT id, email, COALESCE\(username, ''\), password_hash, created_at FROM users WHERE id = \$1`).
		WithArgs(1).
		WillReturnRows(rows)

	user, err := service.GetProfile(context.Background(), 1)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if user.ID != 1 {
		t.Errorf("expected user id 1, got %d", user.ID)
	}
}

func Test_should_return_error_when_update_username_invalid(t *testing.T) {
	service := NewAuthService(nil, "secret", time.Hour)
	_, err := service.UpdateUsername(context.Background(), 1, "a")
	if err == nil || err.Error() != "username must be between 2 and 32 characters" {
		t.Errorf("expected length error, got %v", err)
	}

	longName := string(make([]byte, 33))
	_, err = service.UpdateUsername(context.Background(), 1, longName)
	if err == nil || err.Error() != "username must be between 2 and 32 characters" {
		t.Errorf("expected length error, got %v", err)
	}
}

func Test_should_update_username_successfully(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	userRepo := repositories.NewPostgresUserRepository(db)
	service := NewAuthService(userRepo, "secret", time.Hour)

	rows := sqlmock.NewRows([]string{"id", "email", "username", "password_hash", "created_at"}).
		AddRow(1, "test@test.com", "newname", "hash", time.Now())
	mock.ExpectQuery(`UPDATE users SET username = \$1 WHERE id = \$2 RETURNING id, email, COALESCE\(username, ''\), password_hash, created_at`).
		WithArgs("newname", 1).
		WillReturnRows(rows)

	user, err := service.UpdateUsername(context.Background(), 1, "newname")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if user.Username != "newname" {
		t.Errorf("expected 'newname', got %s", user.Username)
	}
}
