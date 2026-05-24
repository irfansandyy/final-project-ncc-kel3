package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func Test_Should_CallNextHandler_When_RequestLoggerIsUsed(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := RequestLogger(logger)(inner)

	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("expected next handler to be called")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func Test_Should_LogMethodAndPath_When_RequestIsProcessed(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RequestLogger(logger)(inner)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "POST") {
		t.Errorf("log should contain method POST, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "/api/v1/test") {
		t.Errorf("log should contain path /api/v1/test, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "http_request") {
		t.Errorf("log should contain message http_request, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "duration") {
		t.Errorf("log should contain duration field, got: %s", logOutput)
	}
}

func Test_Should_LogDifferentMethods_When_DifferentHTTPMethodsUsed(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, nil))

			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := RequestLogger(logger)(inner)

			req := httptest.NewRequest(method, "/test", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			logOutput := buf.String()
			if !strings.Contains(logOutput, method) {
				t.Errorf("log should contain method %s, got: %s", method, logOutput)
			}
		})
	}
}

func Test_Should_LogRemoteAddr_When_RequestHasRemoteAddr(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RequestLogger(logger)(inner)

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "192.168.1.1:12345") {
		t.Errorf("log should contain remote addr, got: %s", logOutput)
	}
}
