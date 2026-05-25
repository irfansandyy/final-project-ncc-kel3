package middleware

import (
	"app-backend/services"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func Test_should_return_401_when_missing_auth_header(t *testing.T) {
	authService := services.NewAuthService(nil, "secret", time.Hour)
	handler := JWTAuth(authService)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, rr.Code)
	}
	if rr.Body.String() != "missing authorization header\n" {
		t.Errorf("unexpected body: %q", rr.Body.String())
	}
}

func Test_should_return_401_when_invalid_auth_header_format(t *testing.T) {
	authService := services.NewAuthService(nil, "secret", time.Hour)
	handler := JWTAuth(authService)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "InvalidFormatToken")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, rr.Code)
	}
	if rr.Body.String() != "invalid authorization header\n" {
		t.Errorf("unexpected body: %q", rr.Body.String())
	}
}

func Test_should_return_401_when_token_is_invalid(t *testing.T) {
	authService := services.NewAuthService(nil, "secret", time.Hour)
	handler := JWTAuth(authService)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, rr.Code)
	}
	if rr.Body.String() != "invalid token\n" {
		t.Errorf("unexpected body: %q", rr.Body.String())
	}
}

func Test_should_call_next_handler_when_token_is_valid(t *testing.T) {
	authService := services.NewAuthService(nil, "secret", time.Hour)

	claims := services.Claims{
		UserID: 123,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte("secret"))

	handler := JWTAuth(authService)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := UserIDFromContext(r.Context())
		if !ok || userID != 123 {
			t.Errorf("expected user ID 123 in context, got %d (ok: %v)", userID, ok)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rr.Code)
	}
}

func Test_should_return_userid_when_present_in_context(t *testing.T) {
	ctx := context.WithValue(context.Background(), UserIDKey, int64(456))
	userID, ok := UserIDFromContext(ctx)
	if !ok || userID != 456 {
		t.Errorf("expected 456, true; got %d, %v", userID, ok)
	}
}

func Test_should_return_false_when_userid_missing_in_context(t *testing.T) {
	userID, ok := UserIDFromContext(context.Background())
	if ok || userID != 0 {
		t.Errorf("expected 0, false; got %d, %v", userID, ok)
	}
}
