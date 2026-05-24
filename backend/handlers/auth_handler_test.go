package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"app-backend/middleware"
	"app-backend/models"
	"app-backend/repositories"
	"app-backend/services"

	"golang.org/x/crypto/bcrypt"
)

// ---------------------------------------------------------------------------
// Mock UserRepository
// ---------------------------------------------------------------------------

type mockUserRepo struct {
	users  map[string]models.User // email -> user
	nextID int64
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		users:  make(map[string]models.User),
		nextID: 1,
	}
}

func (m *mockUserRepo) CreateUser(_ context.Context, email, passwordHash, username string) (models.User, error) {
	user := models.User{
		ID:           m.nextID,
		Email:        email,
		PasswordHash: passwordHash,
		Username:     username,
		CreatedAt:    time.Now(),
	}
	m.nextID++
	m.users[email] = user
	return user, nil
}

func (m *mockUserRepo) GetByEmail(_ context.Context, email string) (models.User, error) {
	user, ok := m.users[email]
	if !ok {
		return models.User{}, repositories.ErrUserNotFound
	}
	return user, nil
}

func (m *mockUserRepo) GetByID(_ context.Context, id int64) (models.User, error) {
	for _, user := range m.users {
		if user.ID == id {
			return user, nil
		}
	}
	return models.User{}, repositories.ErrUserNotFound
}

func (m *mockUserRepo) UpdateUsername(_ context.Context, id int64, username string) (models.User, error) {
	for email, user := range m.users {
		if user.ID == id {
			user.Username = username
			m.users[email] = user
			return user, nil
		}
	}
	return models.User{}, repositories.ErrUserNotFound
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const testJWTSecret = "test-secret-key-for-unit-tests"
const testTokenTTL = 24 * time.Hour

func setupAuthTest() (*AuthHandler, *services.AuthService, *mockUserRepo) {
	repo := newMockUserRepo()
	authService := services.NewAuthService(repo, testJWTSecret, testTokenTTL)
	handler := NewAuthHandler(authService)
	return handler, authService, repo
}

// seedUser pre-registers a user in the mock repo with a bcrypt password hash.
func seedUser(repo *mockUserRepo, email, password, username string) models.User {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	user := models.User{
		ID:           repo.nextID,
		Email:        email,
		PasswordHash: string(hash),
		Username:     username,
		CreatedAt:    time.Now(),
	}
	repo.nextID++
	repo.users[email] = user
	return user
}

// doRequest is a convenience helper to create a request, record the response,
// and return the recorder.
func doRequest(handler http.HandlerFunc, method, path string, body any) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		data, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(data)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// doAuthenticatedRequest wraps the handler with real JWTAuth middleware so
// the context key is set the same way it would be in production.
func doAuthenticatedRequest(
	authService *services.AuthService,
	handler http.HandlerFunc,
	method, path, token string,
	body any,
) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		data, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(data)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()

	// Wrap in real JWT middleware
	wrapped := middleware.JWTAuth(authService)(http.HandlerFunc(handler))
	wrapped.ServeHTTP(rec, req)
	return rec
}

func parseBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	return body
}

// ---------------------------------------------------------------------------
// Register Tests
// ---------------------------------------------------------------------------

func Test_Should_Return201_When_RegisterWithValidData(t *testing.T) {
	h, _, _ := setupAuthTest()
	rec := doRequest(h.Register, http.MethodPost, "/auth/register", map[string]string{
		"email":    "test@example.com",
		"password": "securepassword",
		"username": "testuser",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	body := parseBody(t, rec)
	if body["email"] != "test@example.com" {
		t.Errorf("expected email test@example.com, got %v", body["email"])
	}
	if body["username"] != "testuser" {
		t.Errorf("expected username testuser, got %v", body["username"])
	}
	if body["id"] == nil || body["id"].(float64) < 1 {
		t.Errorf("expected a valid id, got %v", body["id"])
	}
}

func Test_Should_Return201_When_RegisterWithoutUsername(t *testing.T) {
	h, _, _ := setupAuthTest()
	rec := doRequest(h.Register, http.MethodPost, "/auth/register", map[string]string{
		"email":    "user@domain.com",
		"password": "longpassword",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	body := parseBody(t, rec)
	// Username should be derived from email (the part before @)
	if body["username"] != "user" {
		t.Errorf("expected username 'user' (from email), got %v", body["username"])
	}
}

func Test_Should_Return400_When_RegisterWithInvalidJSON(t *testing.T) {
	h, _, _ := setupAuthTest()
	req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	body := parseBody(t, rec)
	if body["error"] == nil {
		t.Error("expected an error message in response")
	}
}

func Test_Should_Return400_When_RegisterWithEmptyEmail(t *testing.T) {
	h, _, _ := setupAuthTest()
	rec := doRequest(h.Register, http.MethodPost, "/auth/register", map[string]string{
		"email":    "",
		"password": "securepassword",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func Test_Should_Return400_When_RegisterWithWhitespaceOnlyEmail(t *testing.T) {
	h, _, _ := setupAuthTest()
	rec := doRequest(h.Register, http.MethodPost, "/auth/register", map[string]string{
		"email":    "   ",
		"password": "securepassword",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func Test_Should_Return400_When_RegisterWithShortPassword(t *testing.T) {
	h, _, _ := setupAuthTest()
	rec := doRequest(h.Register, http.MethodPost, "/auth/register", map[string]string{
		"email":    "test@example.com",
		"password": "short",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func Test_Should_Return400_When_RegisterWithPasswordExactly7Chars(t *testing.T) {
	h, _, _ := setupAuthTest()
	rec := doRequest(h.Register, http.MethodPost, "/auth/register", map[string]string{
		"email":    "test@example.com",
		"password": "1234567",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for 7-char password, got %d", rec.Code)
	}
}

func Test_Should_Return201_When_RegisterWithPasswordExactly8Chars(t *testing.T) {
	h, _, _ := setupAuthTest()
	rec := doRequest(h.Register, http.MethodPost, "/auth/register", map[string]string{
		"email":    "test@example.com",
		"password": "12345678",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201 for 8-char password, got %d: %s", rec.Code, rec.Body.String())
	}
}

func Test_Should_Return409_When_RegisterWithDuplicateEmail(t *testing.T) {
	h, _, repo := setupAuthTest()
	seedUser(repo, "dupe@example.com", "password123", "existing")

	rec := doRequest(h.Register, http.MethodPost, "/auth/register", map[string]string{
		"email":    "dupe@example.com",
		"password": "password123",
		"username": "newuser",
	})

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func Test_Should_Return400_When_RegisterWithUsernameTooShort(t *testing.T) {
	h, _, _ := setupAuthTest()
	rec := doRequest(h.Register, http.MethodPost, "/auth/register", map[string]string{
		"email":    "test@example.com",
		"password": "securepassword",
		"username": "a",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for 1-char username, got %d: %s", rec.Code, rec.Body.String())
	}

	body := parseBody(t, rec)
	if !strings.Contains(body["error"].(string), "username must be between 2 and 32") {
		t.Errorf("unexpected error message: %v", body["error"])
	}
}

func Test_Should_Return400_When_RegisterWithUsernameTooLong(t *testing.T) {
	h, _, _ := setupAuthTest()
	longUsername := strings.Repeat("a", 33)
	rec := doRequest(h.Register, http.MethodPost, "/auth/register", map[string]string{
		"email":    "test@example.com",
		"password": "securepassword",
		"username": longUsername,
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for 33-char username, got %d: %s", rec.Code, rec.Body.String())
	}
}

func Test_Should_Return201_When_RegisterWithUsername2Chars(t *testing.T) {
	h, _, _ := setupAuthTest()
	rec := doRequest(h.Register, http.MethodPost, "/auth/register", map[string]string{
		"email":    "test@example.com",
		"password": "securepassword",
		"username": "ab",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201 for 2-char username, got %d: %s", rec.Code, rec.Body.String())
	}
}

func Test_Should_Return201_When_RegisterWithUsername32Chars(t *testing.T) {
	h, _, _ := setupAuthTest()
	username32 := strings.Repeat("a", 32)
	rec := doRequest(h.Register, http.MethodPost, "/auth/register", map[string]string{
		"email":    "test@example.com",
		"password": "securepassword",
		"username": username32,
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201 for 32-char username, got %d: %s", rec.Code, rec.Body.String())
	}
}

func Test_Should_NormalizeEmail_When_RegisterWithUppercaseEmail(t *testing.T) {
	h, _, _ := setupAuthTest()
	rec := doRequest(h.Register, http.MethodPost, "/auth/register", map[string]string{
		"email":    "TEST@EXAMPLE.COM",
		"password": "securepassword",
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	body := parseBody(t, rec)
	if body["email"] != "test@example.com" {
		t.Errorf("expected lowercased email, got %v", body["email"])
	}
}

// ---------------------------------------------------------------------------
// Login Tests
// ---------------------------------------------------------------------------

func Test_Should_Return200WithToken_When_LoginWithCorrectCredentials(t *testing.T) {
	h, _, repo := setupAuthTest()
	seedUser(repo, "login@example.com", "password123", "loginuser")

	rec := doRequest(h.Login, http.MethodPost, "/auth/login", map[string]string{
		"email":    "login@example.com",
		"password": "password123",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := parseBody(t, rec)
	if body["token"] == nil || body["token"].(string) == "" {
		t.Error("expected a non-empty token")
	}
	user, ok := body["user"].(map[string]any)
	if !ok {
		t.Fatal("expected user object in response")
	}
	if user["email"] != "login@example.com" {
		t.Errorf("expected email login@example.com, got %v", user["email"])
	}
}

func Test_Should_Return400_When_LoginWithInvalidJSON(t *testing.T) {
	h, _, _ := setupAuthTest()
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Login(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func Test_Should_Return401_When_LoginWithWrongPassword(t *testing.T) {
	h, _, repo := setupAuthTest()
	seedUser(repo, "login@example.com", "correctpass", "loginuser")

	rec := doRequest(h.Login, http.MethodPost, "/auth/login", map[string]string{
		"email":    "login@example.com",
		"password": "wrongpassword",
	})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func Test_Should_Return401_When_LoginWithNonexistentUser(t *testing.T) {
	h, _, _ := setupAuthTest()
	rec := doRequest(h.Login, http.MethodPost, "/auth/login", map[string]string{
		"email":    "nobody@example.com",
		"password": "password123",
	})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func Test_Should_NormalizeEmail_When_LoginWithUppercaseEmail(t *testing.T) {
	h, _, repo := setupAuthTest()
	seedUser(repo, "login@example.com", "password123", "loginuser")

	rec := doRequest(h.Login, http.MethodPost, "/auth/login", map[string]string{
		"email":    "LOGIN@EXAMPLE.COM",
		"password": "password123",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 after email normalization, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Me Tests
// ---------------------------------------------------------------------------

func Test_Should_Return200_When_MeWithValidToken(t *testing.T) {
	h, authService, repo := setupAuthTest()
	seedUser(repo, "me@example.com", "password123", "meuser")

	// Login to get a valid token
	token, _, err := authService.Login(context.Background(), "me@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to login for token: %v", err)
	}

	rec := doAuthenticatedRequest(authService, h.Me, http.MethodGet, "/auth/me", token, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := parseBody(t, rec)
	if body["email"] != "me@example.com" {
		t.Errorf("expected email me@example.com, got %v", body["email"])
	}
	if body["username"] != "meuser" {
		t.Errorf("expected username meuser, got %v", body["username"])
	}
}

func Test_Should_Return401_When_MeWithoutToken(t *testing.T) {
	h, authService, _ := setupAuthTest()

	rec := doAuthenticatedRequest(authService, h.Me, http.MethodGet, "/auth/me", "", nil)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func Test_Should_Return401_When_MeWithInvalidToken(t *testing.T) {
	h, authService, _ := setupAuthTest()

	rec := doAuthenticatedRequest(authService, h.Me, http.MethodGet, "/auth/me", "invalid-token-string", nil)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func Test_Should_Return401_When_MeWithMissingUserContext(t *testing.T) {
	h, _, _ := setupAuthTest()

	// Call handler directly without middleware — no user_id in context
	rec := doRequest(h.Me, http.MethodGet, "/auth/me", nil)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", rec.Code, rec.Body.String())
	}

	body := parseBody(t, rec)
	errMsg, ok := body["error"].(string)
	if !ok || !strings.Contains(errMsg, "missing user context") {
		t.Errorf("expected 'missing user context' error, got %v", body["error"])
	}
}

// ---------------------------------------------------------------------------
// UpdateUsername Tests
// ---------------------------------------------------------------------------

func Test_Should_Return200_When_UpdateUsernameWithValidData(t *testing.T) {
	h, authService, repo := setupAuthTest()
	seedUser(repo, "update@example.com", "password123", "oldname")

	token, _, err := authService.Login(context.Background(), "update@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}

	rec := doAuthenticatedRequest(authService, h.UpdateUsername, http.MethodPut, "/auth/username", token,
		map[string]string{"username": "newname"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := parseBody(t, rec)
	if body["username"] != "newname" {
		t.Errorf("expected username newname, got %v", body["username"])
	}
}

func Test_Should_Return401_When_UpdateUsernameWithoutToken(t *testing.T) {
	h, authService, _ := setupAuthTest()

	rec := doAuthenticatedRequest(authService, h.UpdateUsername, http.MethodPut, "/auth/username", "",
		map[string]string{"username": "newname"})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func Test_Should_Return401_When_UpdateUsernameWithMissingUserContext(t *testing.T) {
	h, _, _ := setupAuthTest()

	// Directly calling the handler without the middleware
	rec := doRequest(h.UpdateUsername, http.MethodPut, "/auth/username",
		map[string]string{"username": "newname"})

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func Test_Should_Return400_When_UpdateUsernameWithInvalidJSON(t *testing.T) {
	h, authService, repo := setupAuthTest()
	seedUser(repo, "update@example.com", "password123", "oldname")

	token, _, err := authService.Login(context.Background(), "update@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/auth/username", strings.NewReader("{bad-json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	wrapped := middleware.JWTAuth(authService)(http.HandlerFunc(h.UpdateUsername))
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func Test_Should_Return400_When_UpdateUsernameWithTooShort(t *testing.T) {
	h, authService, repo := setupAuthTest()
	seedUser(repo, "update@example.com", "password123", "oldname")

	token, _, err := authService.Login(context.Background(), "update@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}

	rec := doAuthenticatedRequest(authService, h.UpdateUsername, http.MethodPut, "/auth/username", token,
		map[string]string{"username": "a"})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for 1-char username, got %d: %s", rec.Code, rec.Body.String())
	}
}

func Test_Should_Return400_When_UpdateUsernameWithTooLong(t *testing.T) {
	h, authService, repo := setupAuthTest()
	seedUser(repo, "update@example.com", "password123", "oldname")

	token, _, err := authService.Login(context.Background(), "update@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}

	longName := strings.Repeat("x", 33)
	rec := doAuthenticatedRequest(authService, h.UpdateUsername, http.MethodPut, "/auth/username", token,
		map[string]string{"username": longName})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for 33-char username, got %d: %s", rec.Code, rec.Body.String())
	}
}

func Test_Should_Return400_When_UpdateUsernameWithEmptyUsername(t *testing.T) {
	h, authService, repo := setupAuthTest()
	seedUser(repo, "update@example.com", "password123", "oldname")

	token, _, err := authService.Login(context.Background(), "update@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}

	rec := doAuthenticatedRequest(authService, h.UpdateUsername, http.MethodPut, "/auth/username", token,
		map[string]string{"username": ""})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for empty username, got %d: %s", rec.Code, rec.Body.String())
	}
}

func Test_Should_Return200_When_UpdateUsernameWithBoundary2Chars(t *testing.T) {
	h, authService, repo := setupAuthTest()
	seedUser(repo, "update@example.com", "password123", "oldname")

	token, _, err := authService.Login(context.Background(), "update@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}

	rec := doAuthenticatedRequest(authService, h.UpdateUsername, http.MethodPut, "/auth/username", token,
		map[string]string{"username": "ab"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for 2-char username, got %d: %s", rec.Code, rec.Body.String())
	}
}

func Test_Should_Return200_When_UpdateUsernameWithBoundary32Chars(t *testing.T) {
	h, authService, repo := setupAuthTest()
	seedUser(repo, "update@example.com", "password123", "oldname")

	token, _, err := authService.Login(context.Background(), "update@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}

	name32 := strings.Repeat("z", 32)
	rec := doAuthenticatedRequest(authService, h.UpdateUsername, http.MethodPut, "/auth/username", token,
		map[string]string{"username": name32})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for 32-char username, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// writeJSON / writeJSONError (exercised indirectly via handlers above,
// but we verify Content-Type header explicitly)
// ---------------------------------------------------------------------------

func Test_Should_SetContentTypeJSON_When_AnyHandlerResponds(t *testing.T) {
	h, _, _ := setupAuthTest()
	rec := doRequest(h.Register, http.MethodPost, "/auth/register", map[string]string{
		"email":    "ct@example.com",
		"password": "securepassword",
	})

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}
}

func Test_Should_SetContentTypeJSON_When_ErrorResponse(t *testing.T) {
	h, _, _ := setupAuthTest()
	rec := doRequest(h.Register, http.MethodPost, "/auth/register", map[string]string{
		"email":    "",
		"password": "short",
	})

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json on error, got %s", ct)
	}

	body := parseBody(t, rec)
	if body["error"] == nil {
		t.Error("expected error key in JSON error response")
	}
}

// ---------------------------------------------------------------------------
// Integration: full Register + Login flow
// ---------------------------------------------------------------------------

func Test_Should_LoginSuccessfully_When_UserJustRegistered(t *testing.T) {
	h, _, _ := setupAuthTest()

	// Register
	regRec := doRequest(h.Register, http.MethodPost, "/auth/register", map[string]string{
		"email":    "flow@example.com",
		"password": "password123",
		"username": "flowuser",
	})
	if regRec.Code != http.StatusCreated {
		t.Fatalf("register failed: %d %s", regRec.Code, regRec.Body.String())
	}

	// Login with same credentials
	loginRec := doRequest(h.Login, http.MethodPost, "/auth/login", map[string]string{
		"email":    "flow@example.com",
		"password": "password123",
	})
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login after register failed: %d %s", loginRec.Code, loginRec.Body.String())
	}

	body := parseBody(t, loginRec)
	if body["token"] == nil || body["token"].(string) == "" {
		t.Error("expected token after login")
	}
}
