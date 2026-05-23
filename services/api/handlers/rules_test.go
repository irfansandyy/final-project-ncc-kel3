package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestCreateRule_MissingName(t *testing.T) {
	t.Skip("requires database connection")
}

func TestCreateRule_InvalidSeverity(t *testing.T) {
	t.Skip("requires database connection")
}

func TestCreateRule_MissingConditionType(t *testing.T) {
	t.Skip("requires database connection")
}

func TestListRules_ResponseShape(t *testing.T) {
	t.Skip("requires database connection")
}

func TestReloadRules_Trigger(t *testing.T) {
	r := chi.NewRouter()
	r.Post("/api/siem/rules/reload", ReloadRules(nil))

	req := httptest.NewRequest(http.MethodPost, "/api/siem/rules/reload", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "reload_triggered" {
		t.Errorf("expected status 'reload_triggered', got %q", resp["status"])
	}
	if resp["timestamp"] == "" {
		t.Error("expected non-empty timestamp")
	}

	r2 := chi.NewRouter()
	r2.Get("/api/siem/rules/reload-status", ReloadStatus(nil))

	req2 := httptest.NewRequest(http.MethodGet, "/api/siem/rules/reload-status", nil)
	w2 := httptest.NewRecorder()
	r2.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w2.Code)
	}

	var statusResp map[string]interface{}
	if err := json.NewDecoder(w2.Body).Decode(&statusResp); err != nil {
		t.Fatalf("failed to decode status response: %v", err)
	}
	if statusResp["last_reload"] == "" {
		t.Error("expected non-empty last_reload after trigger")
	}
}
