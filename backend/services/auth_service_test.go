package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"app-backend/models"
	"app-backend/repositories"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// ---------------------------------------------------------------------------
// Mock UserRepository
// ---------------------------------------------------------------------------

type mockUserRepo struct {
	createUserFn     func(ctx context.Context, email, passwordHash, username string) (models.User, error)
	getByEmailFn     func(ctx context.Context, email string) (models.User, error)
	getByIDFn        func(ctx context.Context, id int64) (models.User, error)
	updateUsernameFn func(ctx context.Context, id int64, username string) (models.User, error)
}

func (m *mockUserRepo) CreateUser(ctx context.Context, email, passwordHash, username string) (models.User, error) {
	if m.createUserFn != nil {
		return m.createUserFn(ctx, email, passwordHash, username)
	}
	return models.User{}, nil
}

func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (models.User, error) {
	if m.getByEmailFn != nil {
		return m.getByEmailFn(ctx, email)
	}
	return models.User{}, repositories.ErrUserNotFound
}

func (m *mockUserRepo) GetByID(ctx context.Context, id int64) (models.User, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return models.User{}, nil
}

func (m *mockUserRepo) UpdateUsername(ctx context.Context, id int64, username string) (models.User, error) {
	if m.updateUsernameFn != nil {
		return m.updateUsernameFn(ctx, id, username)
	}
	return models.User{}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const testSecret = "test-secret-key"
const testTTL = 24 * time.Hour

func newTestAuthService(repo *mockUserRepo) *AuthService {
	return NewAuthService(repo, testSecret, testTTL)
}

func hashPassword(t *testing.T, password string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	return string(h)
}

// ---------------------------------------------------------------------------
// NewAuthService
// ---------------------------------------------------------------------------

func Test_Should_CreateAuthService_When_ValidArgs(t *testing.T) {
	repo := &mockUserRepo{}
	svc := NewAuthService(repo, "secret", 1*time.Hour)
	if svc == nil {
		t.Fatal("expected non-nil AuthService")
	}
	if string(svc.jwtSecret) != "secret" {
		t.Errorf("expected jwtSecret 'secret', got %q", string(svc.jwtSecret))
	}
	if svc.tokenTTL != 1*time.Hour {
		t.Errorf("expected tokenTTL 1h, got %v", svc.tokenTTL)
	}
}

// ---------------------------------------------------------------------------
// Register
// ---------------------------------------------------------------------------

func Test_Should_ReturnErrEmailInUse_When_EmailAlreadyExists(t *testing.T) {
	repo := &mockUserRepo{
		getByEmailFn: func(_ context.Context, _ string) (models.User, error) {
			return models.User{ID: 1, Email: "exists@test.com"}, nil // no error means user exists
		},
	}
	svc := newTestAuthService(repo)

	_, err := svc.Register(context.Background(), "exists@test.com", "password123", "user1")
	if !errors.Is(err, ErrEmailInUse) {
		t.Fatalf("expected ErrEmailInUse, got %v", err)
	}
}

func Test_Should_ReturnError_When_GetByEmailReturnsUnexpectedError(t *testing.T) {
	dbErr := errors.New("db connection lost")
	repo := &mockUserRepo{
		getByEmailFn: func(_ context.Context, _ string) (models.User, error) {
			return models.User{}, dbErr
		},
	}
	svc := newTestAuthService(repo)

	_, err := svc.Register(context.Background(), "test@test.com", "pass", "user")
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected db error, got %v", err)
	}
}

func Test_Should_RegisterSuccessfully_When_ValidInput(t *testing.T) {
	expectedUser := models.User{ID: 42, Email: "new@test.com", Username: "myuser"}
	repo := &mockUserRepo{
		getByEmailFn: func(_ context.Context, _ string) (models.User, error) {
			return models.User{}, repositories.ErrUserNotFound
		},
		createUserFn: func(_ context.Context, email, passwordHash, username string) (models.User, error) {
			if email != "new@test.com" {
				t.Errorf("expected email new@test.com, got %s", email)
			}
			if username != "myuser" {
				t.Errorf("expected username myuser, got %s", username)
			}
			if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte("securepass")); err != nil {
				t.Errorf("password hash does not match")
			}
			return expectedUser, nil
		},
	}
	svc := newTestAuthService(repo)

	user, err := svc.Register(context.Background(), "new@test.com", "securepass", "myuser")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if user.ID != 42 {
		t.Errorf("expected user ID 42, got %d", user.ID)
	}
}

func Test_Should_DeriveUsernameFromEmail_When_UsernameIsEmpty(t *testing.T) {
	var capturedUsername string
	repo := &mockUserRepo{
		getByEmailFn: func(_ context.Context, _ string) (models.User, error) {
			return models.User{}, repositories.ErrUserNotFound
		},
		createUserFn: func(_ context.Context, _, _, username string) (models.User, error) {
			capturedUsername = username
			return models.User{Username: username}, nil
		},
	}
	svc := newTestAuthService(repo)

	_, err := svc.Register(context.Background(), "john.doe@example.com", "pass", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedUsername != "john.doe" {
		t.Errorf("expected username 'john.doe', got %q", capturedUsername)
	}
}

func Test_Should_DeriveUsernameFromEmail_When_UsernameIsWhitespace(t *testing.T) {
	var capturedUsername string
	repo := &mockUserRepo{
		getByEmailFn: func(_ context.Context, _ string) (models.User, error) {
			return models.User{}, repositories.ErrUserNotFound
		},
		createUserFn: func(_ context.Context, _, _, username string) (models.User, error) {
			capturedUsername = username
			return models.User{Username: username}, nil
		},
	}
	svc := newTestAuthService(repo)

	_, err := svc.Register(context.Background(), "alice@example.com", "pass", "   ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedUsername != "alice" {
		t.Errorf("expected username 'alice', got %q", capturedUsername)
	}
}

func Test_Should_FallbackToUser_When_EmailLocalPartIsEmpty(t *testing.T) {
	var capturedUsername string
	repo := &mockUserRepo{
		getByEmailFn: func(_ context.Context, _ string) (models.User, error) {
			return models.User{}, repositories.ErrUserNotFound
		},
		createUserFn: func(_ context.Context, _, _, username string) (models.User, error) {
			capturedUsername = username
			return models.User{Username: username}, nil
		},
	}
	svc := newTestAuthService(repo)

	_, err := svc.Register(context.Background(), "@example.com", "pass", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedUsername != "user" {
		t.Errorf("expected fallback username 'user', got %q", capturedUsername)
	}
}

// ---------------------------------------------------------------------------
// Login
// ---------------------------------------------------------------------------

func Test_Should_ReturnInvalidCredentials_When_UserNotFound(t *testing.T) {
	repo := &mockUserRepo{
		getByEmailFn: func(_ context.Context, _ string) (models.User, error) {
			return models.User{}, repositories.ErrUserNotFound
		},
	}
	svc := newTestAuthService(repo)

	_, _, err := svc.Login(context.Background(), "noone@test.com", "pass")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func Test_Should_ReturnError_When_LoginGetByEmailFails(t *testing.T) {
	dbErr := errors.New("timeout")
	repo := &mockUserRepo{
		getByEmailFn: func(_ context.Context, _ string) (models.User, error) {
			return models.User{}, dbErr
		},
	}
	svc := newTestAuthService(repo)

	_, _, err := svc.Login(context.Background(), "user@test.com", "pass")
	if !errors.Is(err, dbErr) {
		t.Fatalf("expected db error, got %v", err)
	}
}

func Test_Should_ReturnInvalidCredentials_When_WrongPassword(t *testing.T) {
	repo := &mockUserRepo{
		getByEmailFn: func(_ context.Context, _ string) (models.User, error) {
			return models.User{
				ID:           1,
				Email:        "user@test.com",
				PasswordHash: hashPassword(t, "correctpass"),
			}, nil
		},
	}
	svc := newTestAuthService(repo)

	_, _, err := svc.Login(context.Background(), "user@test.com", "wrongpass")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func Test_Should_ReturnTokenAndUser_When_LoginSuccess(t *testing.T) {
	pw := "correctpass"
	repo := &mockUserRepo{
		getByEmailFn: func(_ context.Context, _ string) (models.User, error) {
			return models.User{
				ID:           7,
				Email:        "user@test.com",
				Username:     "testuser",
				PasswordHash: hashPassword(t, pw),
			}, nil
		},
	}
	svc := newTestAuthService(repo)

	token, user, err := svc.Login(context.Background(), "user@test.com", pw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
	if user.ID != 7 {
		t.Errorf("expected user ID 7, got %d", user.ID)
	}

	// Verify the token can be parsed back
	userID, parseErr := svc.ParseToken(token)
	if parseErr != nil {
		t.Fatalf("failed to parse login token: %v", parseErr)
	}
	if userID != 7 {
		t.Errorf("token user ID = %d, want 7", userID)
	}
}

// ---------------------------------------------------------------------------
// ParseToken
// ---------------------------------------------------------------------------

func Test_Should_ReturnUserID_When_TokenIsValid(t *testing.T) {
	svc := newTestAuthService(&mockUserRepo{})

	claims := Claims{
		UserID: 99,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Subject:   "auth",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	userID, err := svc.ParseToken(tokenStr)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if userID != 99 {
		t.Errorf("expected userID 99, got %d", userID)
	}
}

func Test_Should_ReturnError_When_TokenIsExpired(t *testing.T) {
	svc := newTestAuthService(&mockUserRepo{})

	claims := Claims{
		UserID: 1,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			NotBefore: jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(testSecret))

	_, err := svc.ParseToken(tokenStr)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func Test_Should_ReturnError_When_TokenIsMalformed(t *testing.T) {
	svc := newTestAuthService(&mockUserRepo{})

	_, err := svc.ParseToken("not.a.valid.jwt.token")
	if err == nil {
		t.Fatal("expected error for malformed token")
	}
}

func Test_Should_ReturnError_When_TokenSignedWithWrongKey(t *testing.T) {
	svc := newTestAuthService(&mockUserRepo{})

	claims := Claims{
		UserID: 1,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte("wrong-secret-key"))

	_, err := svc.ParseToken(tokenStr)
	if err == nil {
		t.Fatal("expected error for wrong signing key")
	}
}

func Test_Should_ReturnError_When_TokenIsEmpty(t *testing.T) {
	svc := newTestAuthService(&mockUserRepo{})

	_, err := svc.ParseToken("")
	if err == nil {
		t.Fatal("expected error for empty token string")
	}
}

// ---------------------------------------------------------------------------
// GetProfile
// ---------------------------------------------------------------------------

func Test_Should_ReturnUser_When_GetProfileSucceeds(t *testing.T) {
	expected := models.User{ID: 5, Email: "profile@test.com", Username: "profuser"}
	repo := &mockUserRepo{
		getByIDFn: func(_ context.Context, id int64) (models.User, error) {
			if id != 5 {
				t.Errorf("expected id 5, got %d", id)
			}
			return expected, nil
		},
	}
	svc := newTestAuthService(repo)

	user, err := svc.GetProfile(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != 5 || user.Email != "profile@test.com" {
		t.Errorf("unexpected user: %+v", user)
	}
}

func Test_Should_ReturnError_When_GetProfileUserNotFound(t *testing.T) {
	repo := &mockUserRepo{
		getByIDFn: func(_ context.Context, _ int64) (models.User, error) {
			return models.User{}, repositories.ErrUserNotFound
		},
	}
	svc := newTestAuthService(repo)

	_, err := svc.GetProfile(context.Background(), 999)
	if !errors.Is(err, repositories.ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// UpdateUsername
// ---------------------------------------------------------------------------

func Test_Should_ReturnError_When_UsernameTooShort(t *testing.T) {
	svc := newTestAuthService(&mockUserRepo{})

	_, err := svc.UpdateUsername(context.Background(), 1, "a")
	if err == nil {
		t.Fatal("expected error for username too short")
	}
	if !strings.Contains(err.Error(), "between 2 and 32") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func Test_Should_ReturnError_When_UsernameTooLong(t *testing.T) {
	svc := newTestAuthService(&mockUserRepo{})

	longName := strings.Repeat("a", 33)
	_, err := svc.UpdateUsername(context.Background(), 1, longName)
	if err == nil {
		t.Fatal("expected error for username too long")
	}
	if !strings.Contains(err.Error(), "between 2 and 32") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func Test_Should_ReturnError_When_UsernameIsWhitespaceOnly(t *testing.T) {
	svc := newTestAuthService(&mockUserRepo{})

	_, err := svc.UpdateUsername(context.Background(), 1, "   ")
	if err == nil {
		t.Fatal("expected error for whitespace-only username")
	}
}

func Test_Should_UpdateUsername_When_ValidLength(t *testing.T) {
	expected := models.User{ID: 3, Username: "newname"}
	repo := &mockUserRepo{
		updateUsernameFn: func(_ context.Context, id int64, username string) (models.User, error) {
			if id != 3 {
				t.Errorf("expected id 3, got %d", id)
			}
			if username != "newname" {
				t.Errorf("expected username 'newname', got %q", username)
			}
			return expected, nil
		},
	}
	svc := newTestAuthService(repo)

	user, err := svc.UpdateUsername(context.Background(), 3, "newname")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Username != "newname" {
		t.Errorf("expected username 'newname', got %q", user.Username)
	}
}

func Test_Should_UpdateUsername_When_ExactlyMinLength(t *testing.T) {
	repo := &mockUserRepo{
		updateUsernameFn: func(_ context.Context, _ int64, _ string) (models.User, error) {
			return models.User{Username: "ab"}, nil
		},
	}
	svc := newTestAuthService(repo)

	_, err := svc.UpdateUsername(context.Background(), 1, "ab")
	if err != nil {
		t.Fatalf("expected no error for 2-char username, got %v", err)
	}
}

func Test_Should_UpdateUsername_When_ExactlyMaxLength(t *testing.T) {
	name := strings.Repeat("x", 32)
	repo := &mockUserRepo{
		updateUsernameFn: func(_ context.Context, _ int64, _ string) (models.User, error) {
			return models.User{Username: name}, nil
		},
	}
	svc := newTestAuthService(repo)

	_, err := svc.UpdateUsername(context.Background(), 1, name)
	if err != nil {
		t.Fatalf("expected no error for 32-char username, got %v", err)
	}
}

func Test_Should_TrimWhitespace_When_UpdateUsernameHasPadding(t *testing.T) {
	var capturedUsername string
	repo := &mockUserRepo{
		updateUsernameFn: func(_ context.Context, _ int64, username string) (models.User, error) {
			capturedUsername = username
			return models.User{Username: username}, nil
		},
	}
	svc := newTestAuthService(repo)

	_, err := svc.UpdateUsername(context.Background(), 1, "  hello  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedUsername != "hello" {
		t.Errorf("expected trimmed username 'hello', got %q", capturedUsername)
	}
}
