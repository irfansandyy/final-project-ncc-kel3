package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func Test_should_log_request_details_when_handler_is_called(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	handler := RequestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/test-path", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected %d, got %d", http.StatusCreated, rr.Code)
	}

	logOutput := buf.String()
	if !strings.Contains(logOutput, "http_request") {
		t.Errorf("expected log to contain 'http_request', got: %q", logOutput)
	}
	if !strings.Contains(logOutput, "method=POST") {
		t.Errorf("expected log to contain 'method=POST', got: %q", logOutput)
	}
	if !strings.Contains(logOutput, "path=/test-path") {
		t.Errorf("expected log to contain 'path=/test-path', got: %q", logOutput)
	}
	if !strings.Contains(logOutput, "remote=192.168.1.1:1234") {
		t.Errorf("expected log to contain 'remote=192.168.1.1:1234', got: %q", logOutput)
	}
	if !strings.Contains(logOutput, "duration=") {
		t.Errorf("expected log to contain 'duration=', got: %q", logOutput)
	}
}
