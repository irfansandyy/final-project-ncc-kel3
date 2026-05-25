package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"app-backend/middleware"
	"app-backend/models"
	"app-backend/repositories"
	"app-backend/services"

	"golang.org/x/crypto/bcrypt"
)

type mockUserRepo struct {
	users  map[int64]*models.User
	emails map[string]int64
	nextID int64
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		users:  make(map[int64]*models.User),
		emails: make(map[string]int64),
		nextID: 1,
	}
}

func (m *mockUserRepo) CreateUser(ctx context.Context, email, passwordHash, username string) (models.User, error) {
	if _, exists := m.emails[email]; exists {
		// Postgres returns a duplicate key error, we'll just simulate a generic error that the service translates
		return models.User{}, services.ErrEmailInUse // Service uses its own error if it detects duplicate
	}
	user := models.User{
		ID:           m.nextID,
		Email:        email,
		Username:     username,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now(),
	}
	m.users[m.nextID] = &user
	m.emails[email] = m.nextID
	m.nextID++
	return user, nil
}

func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (models.User, error) {
	id, exists := m.emails[email]
	if !exists {
		return models.User{}, repositories.ErrUserNotFound
	}
	return *m.users[id], nil
}

func (m *mockUserRepo) GetByID(ctx context.Context, id int64) (models.User, error) {
	user, exists := m.users[id]
	if !exists {
		return models.User{}, repositories.ErrUserNotFound
	}
	return *user, nil
}

func (m *mockUserRepo) UpdateUsername(ctx context.Context, id int64, username string) (models.User, error) {
	user, exists := m.users[id]
	if !exists {
		return models.User{}, repositories.ErrUserNotFound
	}
	user.Username = username
	return *user, nil
}

func setupAuthHandler() (*AuthHandler, *mockUserRepo) {
	repo := newMockUserRepo()
	authService := services.NewAuthService(repo, "secret", time.Hour)
	handler := NewAuthHandler(authService)
	return handler, repo
}

func Test_should_register_user_when_valid_input(t *testing.T) {
	handler, _ := setupAuthHandler()

	body := map[string]string{
		"email":    "test@example.com",
		"password": "password123",
		"username": "tester",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(jsonBody))
	rr := httptest.NewRecorder()

	handler.Register(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected %v, got %v", http.StatusCreated, rr.Code)
	}

	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["email"] != "test@example.com" {
		t.Errorf("expected email test@example.com, got %v", resp["email"])
	}
}

func Test_should_fail_register_when_invalid_body(t *testing.T) {
	handler, _ := setupAuthHandler()

	req, _ := http.NewRequest(http.MethodPost, "/register", bytes.NewBuffer([]byte("invalid json")))
	rr := httptest.NewRecorder()

	handler.Register(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %v, got %v", http.StatusBadRequest, rr.Code)
	}
}

func Test_should_fail_register_when_email_empty(t *testing.T) {
	handler, _ := setupAuthHandler()

	body := map[string]string{
		"email":    "",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(jsonBody))
	rr := httptest.NewRecorder()

	handler.Register(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %v, got %v", http.StatusBadRequest, rr.Code)
	}
}

func Test_should_fail_register_when_username_invalid(t *testing.T) {
	handler, _ := setupAuthHandler()

	body := map[string]string{
		"email":    "test@example.com",
		"password": "password123",
		"username": "a", // too short
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(jsonBody))
	rr := httptest.NewRecorder()

	handler.Register(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %v, got %v", http.StatusBadRequest, rr.Code)
	}
}

func Test_should_login_user_when_credentials_valid(t *testing.T) {
	handler, repo := setupAuthHandler()
	hashed, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	repo.CreateUser(context.Background(), "test@example.com", string(hashed), "tester")

	body := map[string]string{
		"email":    "test@example.com",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(jsonBody))
	rr := httptest.NewRecorder()

	handler.Login(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected %v, got %v", http.StatusOK, rr.Code)
	}

	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["token"] == nil {
		t.Errorf("expected token, got nil")
	}
}

func Test_should_fail_login_when_invalid_body(t *testing.T) {
	handler, _ := setupAuthHandler()

	req, _ := http.NewRequest(http.MethodPost, "/login", bytes.NewBuffer([]byte("invalid json")))
	rr := httptest.NewRecorder()

	handler.Login(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %v, got %v", http.StatusBadRequest, rr.Code)
	}
}

func Test_should_fail_login_when_credentials_invalid(t *testing.T) {
	handler, repo := setupAuthHandler()
	hashed, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	repo.CreateUser(context.Background(), "test@example.com", string(hashed), "tester")

	body := map[string]string{
		"email":    "test@example.com",
		"password": "wrongpassword",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(jsonBody))
	rr := httptest.NewRecorder()

	handler.Login(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected %v, got %v", http.StatusUnauthorized, rr.Code)
	}
}

func Test_should_return_user_profile_when_authenticated(t *testing.T) {
	handler, repo := setupAuthHandler()
	hashed, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	repo.CreateUser(context.Background(), "test@example.com", string(hashed), "tester")

	req, _ := http.NewRequest(http.MethodGet, "/me", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, int64(1))
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.Me(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected %v, got %v", http.StatusOK, rr.Code)
	}
}

func Test_should_fail_me_when_not_authenticated(t *testing.T) {
	handler, _ := setupAuthHandler()

	req, _ := http.NewRequest(http.MethodGet, "/me", nil)
	rr := httptest.NewRecorder()

	handler.Me(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected %v, got %v", http.StatusUnauthorized, rr.Code)
	}
}

func Test_should_fail_me_when_user_not_found(t *testing.T) {
	handler, _ := setupAuthHandler()

	req, _ := http.NewRequest(http.MethodGet, "/me", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, int64(999))
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.Me(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected %v, got %v", http.StatusInternalServerError, rr.Code)
	}
}

func Test_should_update_username_when_authenticated(t *testing.T) {
	handler, repo := setupAuthHandler()
	hashed, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	repo.CreateUser(context.Background(), "test@example.com", string(hashed), "tester")

	body := map[string]string{
		"username": "newtester",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPut, "/me/username", bytes.NewBuffer(jsonBody))
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, int64(1))
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.UpdateUsername(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected %v, got %v", http.StatusOK, rr.Code)
	}

	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["username"] != "newtester" {
		t.Errorf("expected username newtester, got %v", resp["username"])
	}
}

func Test_should_fail_update_username_when_not_authenticated(t *testing.T) {
	handler, _ := setupAuthHandler()

	body := map[string]string{
		"username": "newtester",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPut, "/me/username", bytes.NewBuffer(jsonBody))
	rr := httptest.NewRecorder()

	handler.UpdateUsername(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected %v, got %v", http.StatusUnauthorized, rr.Code)
	}
}

func Test_should_fail_update_username_when_invalid_body(t *testing.T) {
	handler, _ := setupAuthHandler()

	req, _ := http.NewRequest(http.MethodPut, "/me/username", bytes.NewBuffer([]byte("invalid json")))
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, int64(1))
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.UpdateUsername(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %v, got %v", http.StatusBadRequest, rr.Code)
	}
}

func Test_should_fail_update_username_when_user_not_found(t *testing.T) {
	handler, _ := setupAuthHandler()

	body := map[string]string{
		"username": "newtester",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest(http.MethodPut, "/me/username", bytes.NewBuffer(jsonBody))
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, int64(999))
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.UpdateUsername(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected %v, got %v", http.StatusBadRequest, rr.Code)
	}
}
