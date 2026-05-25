package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func Test_should_allow_request_when_rate_limit_not_exceeded(t *testing.T) {
	handler := RateLimit(10, 1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rr.Code)
	}
}

func Test_should_return_429_when_rate_limit_exceeded(t *testing.T) {
	// Limit to 1 request per second, burst 1
	handler := RateLimit(1, 1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	// First request should pass
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req)
	if rr1.Code != http.StatusOK {
		t.Errorf("expected first request to be 200 OK, got %d", rr1.Code)
	}

	// Second request immediately after should fail
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req)
	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("expected second request to be 429 Too Many Requests, got %d", rr2.Code)
	}
	if rr2.Body.String() != "rate limit exceeded\n" {
		t.Errorf("unexpected body: %q", rr2.Body.String())
	}
}
