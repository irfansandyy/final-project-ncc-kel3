package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func Test_Should_Return200_When_RequestIsWithinRateLimit(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Allow 100 requests per second with burst of 10 — plenty of room.
	handler := RateLimit(100, 10)(inner)

	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "ok")
	}
}

func Test_Should_Return429_When_BurstExceeded(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Very low rate: 0.001 RPS with burst of 2 — after 2 immediate requests, the 3rd should fail.
	handler := RateLimit(0.001, 2)(inner)

	// Exhaust the burst bucket.
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want %d", i+1, rec.Code, http.StatusOK)
		}
	}

	// This request should be rate-limited.
	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if !strings.Contains(rec.Body.String(), "rate limit exceeded") {
		t.Errorf("body should contain 'rate limit exceeded', got: %q", rec.Body.String())
	}
}

func Test_Should_Return429Immediately_When_BurstIsZero(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Burst of 0 means no tokens are ever available.
	handler := RateLimit(1, 0)(inner)

	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func Test_Should_AllowExactlyBurstRequests_When_BurstIsOne(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RateLimit(0.001, 1)(inner)

	// First request should succeed.
	req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("first request: status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Second request should be rate-limited.
	req2 := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("second request: status = %d, want %d", rec2.Code, http.StatusTooManyRequests)
	}
}

func Test_Should_CallNextHandler_When_WithinLimit(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := RateLimit(100, 10)(inner)

	req := httptest.NewRequest(http.MethodPost, "/submit", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("expected next handler to be called")
	}
}

func Test_Should_NotCallNextHandler_When_RateLimited(t *testing.T) {
	callCount := 0
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	})

	handler := RateLimit(0.001, 1)(inner)

	// First request passes through.
	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Second should be blocked — next handler should NOT be called again.
	req2 := httptest.NewRequest(http.MethodGet, "/api", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if callCount != 1 {
		t.Errorf("next handler called %d times, want exactly 1", callCount)
	}
	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec2.Code, http.StatusTooManyRequests)
	}
}
