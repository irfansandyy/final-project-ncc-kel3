package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"app-backend/models"
	"app-backend/repositories"
	"app-backend/services"

	"github.com/golang-jwt/jwt/v5"
)

// --- mock UserRepository ---

type mockUserRepo struct{}

func (m *mockUserRepo) CreateUser(_ context.Context, email, passwordHash, username string) (models.User, error) {
	return models.User{}, nil
}

func (m *mockUserRepo) GetByEmail(_ context.Context, email string) (models.User, error) {
	return models.User{}, repositories.ErrUserNotFound
}

func (m *mockUserRepo) GetByID(_ context.Context, id int64) (models.User, error) {
	return models.User{ID: id, Email: "test@test.com", Username: "tester"}, nil
}

func (m *mockUserRepo) UpdateUsername(_ context.Context, id int64, username string) (models.User, error) {
	return models.User{ID: id, Username: username}, nil
}

// --- helpers ---

const testSecret = "test-jwt-secret"

func newTestAuthService() *services.AuthService {
	return services.NewAuthService(&mockUserRepo{}, testSecret, time.Hour)
}

func makeValidToken(userID int64) string {
	now := time.Now()
	claims := services.Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Subject:   "auth",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(testSecret))
	return signed
}

func makeExpiredToken(userID int64) string {
	past := time.Now().Add(-2 * time.Hour)
	claims := services.Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(past.Add(time.Minute)),
			IssuedAt:  jwt.NewNumericDate(past),
			NotBefore: jwt.NewNumericDate(past),
			Subject:   "auth",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(testSecret))
	return signed
}

func makeTokenWithWrongSecret(userID int64) string {
	now := time.Now()
	claims := services.Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Subject:   "auth",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte("wrong-secret"))
	return signed
}

// dummyHandler records that it was called and stores the userID from context.
func dummyHandler(called *bool, capturedUserID *int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		uid, ok := UserIDFromContext(r.Context())
		if ok {
			*capturedUserID = uid
		}
		w.WriteHeader(http.StatusOK)
	})
}

// --- JWTAuth tests ---

func Test_Should_Return401_When_AuthorizationHeaderMissing(t *testing.T) {
	authSvc := newTestAuthService()
	mw := JWTAuth(authSvc)

	called := false
	var uid int64
	handler := mw(dummyHandler(&called, &uid))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if called {
		t.Error("next handler should NOT have been called")
	}
}

func Test_Should_Return401_When_AuthorizationHeaderHasNoBearerPrefix(t *testing.T) {
	authSvc := newTestAuthService()
	mw := JWTAuth(authSvc)

	called := false
	var uid int64
	handler := mw(dummyHandler(&called, &uid))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Token some-token-value")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if called {
		t.Error("next handler should NOT have been called")
	}
}

func Test_Should_Return401_When_AuthorizationHeaderIsBearerOnly(t *testing.T) {
	authSvc := newTestAuthService()
	mw := JWTAuth(authSvc)

	called := false
	var uid int64
	handler := mw(dummyHandler(&called, &uid))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if called {
		t.Error("next handler should NOT have been called")
	}
}

func Test_Should_Return401_When_TokenIsInvalid(t *testing.T) {
	authSvc := newTestAuthService()
	mw := JWTAuth(authSvc)

	called := false
	var uid int64
	handler := mw(dummyHandler(&called, &uid))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer this-is-not-a-jwt")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if called {
		t.Error("next handler should NOT have been called")
	}
}

func Test_Should_Return401_When_TokenIsExpired(t *testing.T) {
	authSvc := newTestAuthService()
	mw := JWTAuth(authSvc)

	called := false
	var uid int64
	handler := mw(dummyHandler(&called, &uid))

	token := makeExpiredToken(42)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if called {
		t.Error("next handler should NOT have been called")
	}
}

func Test_Should_Return401_When_TokenSignedWithWrongSecret(t *testing.T) {
	authSvc := newTestAuthService()
	mw := JWTAuth(authSvc)

	called := false
	var uid int64
	handler := mw(dummyHandler(&called, &uid))

	token := makeTokenWithWrongSecret(42)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if called {
		t.Error("next handler should NOT have been called")
	}
}

func Test_Should_CallNextAndSetUserID_When_TokenIsValid(t *testing.T) {
	authSvc := newTestAuthService()
	mw := JWTAuth(authSvc)

	called := false
	var capturedUID int64
	handler := mw(dummyHandler(&called, &capturedUID))

	token := makeValidToken(99)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !called {
		t.Error("next handler should have been called")
	}
	if capturedUID != 99 {
		t.Errorf("userID = %d, want 99", capturedUID)
	}
}

func Test_Should_AcceptBearerCaseInsensitive_When_LowercaseBearer(t *testing.T) {
	authSvc := newTestAuthService()
	mw := JWTAuth(authSvc)

	called := false
	var capturedUID int64
	handler := mw(dummyHandler(&called, &capturedUID))

	token := makeValidToken(7)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !called {
		t.Error("next handler should have been called")
	}
	if capturedUID != 7 {
		t.Errorf("userID = %d, want 7", capturedUID)
	}
}

// --- UserIDFromContext tests ---

func Test_Should_ReturnUserIDAndTrue_When_ContextHasValidUserID(t *testing.T) {
	// Reconstruct the context the same way the middleware does.
	ctx := context.WithValue(context.Background(), userIDContextKey, int64(42))

	uid, ok := UserIDFromContext(ctx)
	if !ok {
		t.Error("expected ok=true")
	}
	if uid != 42 {
		t.Errorf("userID = %d, want 42", uid)
	}
}

func Test_Should_ReturnZeroAndFalse_When_ContextIsEmpty(t *testing.T) {
	uid, ok := UserIDFromContext(context.Background())
	if ok {
		t.Error("expected ok=false for empty context")
	}
	if uid != 0 {
		t.Errorf("userID = %d, want 0", uid)
	}
}

func Test_Should_ReturnZeroAndFalse_When_ContextHasWrongType(t *testing.T) {
	// Store a string instead of int64.
	ctx := context.WithValue(context.Background(), userIDContextKey, "not-an-int64")

	uid, ok := UserIDFromContext(ctx)
	if ok {
		t.Error("expected ok=false when value is wrong type")
	}
	if uid != 0 {
		t.Errorf("userID = %d, want 0", uid)
	}
}

func Test_Should_ReturnZeroAndFalse_When_ContextHasWrongKey(t *testing.T) {
	// Use a different key.
	ctx := context.WithValue(context.Background(), contextKey("other_key"), int64(42))

	uid, ok := UserIDFromContext(ctx)
	if ok {
		t.Error("expected ok=false when key is different")
	}
	if uid != 0 {
		t.Errorf("userID = %d, want 0", uid)
	}
}

// Integration-style: run request through the full middleware chain and then extract user from context.
func Test_Should_RoundTripUserIDThroughMiddleware_When_ValidToken(t *testing.T) {
	authSvc := newTestAuthService()
	mw := JWTAuth(authSvc)

	var extractedUID int64
	var extractedOK bool

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		extractedUID, extractedOK = UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := mw(inner)

	token := makeValidToken(555)
	req := httptest.NewRequest(http.MethodPost, "/api/data", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !extractedOK {
		t.Fatal("expected UserIDFromContext to return ok=true")
	}
	if extractedUID != 555 {
		t.Errorf("extractedUID = %d, want 555", extractedUID)
	}
}
