package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

// --- ReloadRules ---

func Test_Should_Return200WithStatus_When_ReloadRulesTriggered(t *testing.T) {
	r := chi.NewRouter()
	r.Post("/rules/reload", ReloadRules(nil))

	req := httptest.NewRequest(http.MethodPost, "/rules/reload", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if resp["status"] != "reload_triggered" {
		t.Errorf("expected status 'reload_triggered', got %q", resp["status"])
	}
	if resp["timestamp"] == "" {
		t.Error("expected non-empty timestamp")
	}
	// Verify timestamp is valid RFC3339
	if _, err := time.Parse(time.RFC3339, resp["timestamp"]); err != nil {
		t.Errorf("timestamp not valid RFC3339: %v", err)
	}
}

func Test_Should_ReturnApplicationJSON_When_ReloadRulesCalled(t *testing.T) {
	r := chi.NewRouter()
	r.Post("/rules/reload", ReloadRules(nil))

	req := httptest.NewRequest(http.MethodPost, "/rules/reload", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", ct)
	}
}

func Test_Should_UpdateLastReloadTime_When_ReloadRulesTriggered(t *testing.T) {
	before := time.Now().UTC().Add(-1 * time.Second)

	r := chi.NewRouter()
	r.Post("/rules/reload", ReloadRules(nil))

	req := httptest.NewRequest(http.MethodPost, "/rules/reload", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	stored, ok := LastReloadTime.Load().(string)
	if !ok || stored == "" {
		t.Fatal("LastReloadTime not set after reload")
	}
	ts, err := time.Parse(time.RFC3339, stored)
	if err != nil {
		t.Fatalf("LastReloadTime not valid RFC3339: %v", err)
	}
	if ts.Before(before) {
		t.Errorf("LastReloadTime %v is before test start %v", ts, before)
	}
}

// --- ReloadStatus ---

func Test_Should_Return200WithLastReload_When_ReloadStatusCalled(t *testing.T) {
	// Trigger a reload first to ensure a known timestamp
	r := chi.NewRouter()
	r.Post("/rules/reload", ReloadRules(nil))
	r.Get("/rules/reload-status", ReloadStatus(nil))

	// Trigger reload
	req := httptest.NewRequest(http.MethodPost, "/rules/reload", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	var reloadResp map[string]string
	json.NewDecoder(rr.Body).Decode(&reloadResp)
	expectedTS := reloadResp["timestamp"]

	// Check status
	req2 := httptest.NewRequest(http.MethodGet, "/rules/reload-status", nil)
	rr2 := httptest.NewRecorder()
	r.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr2.Code)
	}
	ct := rr2.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", ct)
	}

	var statusResp map[string]interface{}
	if err := json.NewDecoder(rr2.Body).Decode(&statusResp); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	lastReload, ok := statusResp["last_reload"].(string)
	if !ok || lastReload == "" {
		t.Fatal("expected non-empty last_reload")
	}
	if lastReload != expectedTS {
		t.Errorf("expected last_reload %q, got %q", expectedTS, lastReload)
	}
}

func Test_Should_ReturnInitTimestamp_When_ReloadStatusCalledWithoutReload(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/rules/reload-status", ReloadStatus(nil))

	req := httptest.NewRequest(http.MethodGet, "/rules/reload-status", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&resp)
	lastReload, ok := resp["last_reload"].(string)
	if !ok || lastReload == "" {
		t.Error("expected non-empty last_reload even without explicit reload (init sets it)")
	}
}

// --- GetRule with invalid id ---

func Test_Should_Return400_When_GetRuleWithInvalidID(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/rules/{id}", GetRule(nil))

	req := httptest.NewRequest(http.MethodGet, "/rules/abc", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
	body := strings.TrimSpace(rr.Body.String())
	if body != "invalid id" {
		t.Errorf("expected body 'invalid id', got %q", body)
	}
}

func Test_Should_Return400_When_GetRuleWithEmptyID(t *testing.T) {
	// chi won't match an empty segment to {id}, so this tests the float/special char cases
	r := chi.NewRouter()
	r.Get("/rules/{id}", GetRule(nil))

	req := httptest.NewRequest(http.MethodGet, "/rules/3.14", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func Test_Should_Return400_When_GetRuleWithNegativeStringID(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/rules/{id}", GetRule(nil))

	req := httptest.NewRequest(http.MethodGet, "/rules/not-a-number", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func Test_Should_Return400_When_GetRuleWithOverflowID(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/rules/{id}", GetRule(nil))

	req := httptest.NewRequest(http.MethodGet, "/rules/99999999999999999999", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// --- DeleteRule with invalid id ---

func Test_Should_Return400_When_DeleteRuleWithInvalidID(t *testing.T) {
	r := chi.NewRouter()
	r.Delete("/rules/{id}", DeleteRule(nil))

	req := httptest.NewRequest(http.MethodDelete, "/rules/abc", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
	body := strings.TrimSpace(rr.Body.String())
	if body != "invalid id" {
		t.Errorf("expected body 'invalid id', got %q", body)
	}
}

func Test_Should_Return400_When_DeleteRuleWithSpecialChars(t *testing.T) {
	r := chi.NewRouter()
	r.Delete("/rules/{id}", DeleteRule(nil))

	req := httptest.NewRequest(http.MethodDelete, "/rules/!@%23", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// --- CreateRule validation ---

func Test_Should_Return400_When_CreateRuleWithInvalidJSON(t *testing.T) {
	r := chi.NewRouter()
	r.Post("/rules", CreateRule(nil))

	req := httptest.NewRequest(http.MethodPost, "/rules", strings.NewReader("not json"))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
	body := strings.TrimSpace(rr.Body.String())
	if body != "invalid request body" {
		t.Errorf("expected 'invalid request body', got %q", body)
	}
}

func Test_Should_Return400_When_CreateRuleWithEmptyBody(t *testing.T) {
	r := chi.NewRouter()
	r.Post("/rules", CreateRule(nil))

	req := httptest.NewRequest(http.MethodPost, "/rules", strings.NewReader(""))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func Test_Should_Return400_When_CreateRuleMissingName(t *testing.T) {
	r := chi.NewRouter()
	r.Post("/rules", CreateRule(nil))

	body := `{"severity":"INFO","condition":{"type":"keyword"}}`
	req := httptest.NewRequest(http.MethodPost, "/rules", strings.NewReader(body))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
	respBody := strings.TrimSpace(rr.Body.String())
	if respBody != "name is required" {
		t.Errorf("expected 'name is required', got %q", respBody)
	}
}

func Test_Should_Return400_When_CreateRuleWithInvalidSeverity(t *testing.T) {
	r := chi.NewRouter()
	r.Post("/rules", CreateRule(nil))

	invalidSeverities := []string{"LOW", "HIGH", "MEDIUM", "warning", "error", "", "UNKNOWN"}
	for _, sev := range invalidSeverities {
		t.Run(sev, func(t *testing.T) {
			body := `{"name":"test","severity":"` + sev + `","condition":{"type":"keyword"}}`
			req := httptest.NewRequest(http.MethodPost, "/rules", strings.NewReader(body))
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for severity %q, got %d", sev, rr.Code)
			}
			respBody := strings.TrimSpace(rr.Body.String())
			if respBody != "invalid severity" {
				t.Errorf("expected 'invalid severity', got %q", respBody)
			}
		})
	}
}

func Test_Should_Return400_When_CreateRuleMissingCondition(t *testing.T) {
	r := chi.NewRouter()
	r.Post("/rules", CreateRule(nil))

	body := `{"name":"test","severity":"INFO"}`
	req := httptest.NewRequest(http.MethodPost, "/rules", strings.NewReader(body))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
	respBody := strings.TrimSpace(rr.Body.String())
	if respBody != "condition is required" {
		t.Errorf("expected 'condition is required', got %q", respBody)
	}
}

func Test_Should_AcceptValidSeverities_When_CreateRuleWithAllValidValues(t *testing.T) {
	// These should pass validation but fail at DB call (nil db -> panic or error).
	// We test that they get PAST the validation stage, meaning no 400 with validation message.
	// With nil db, it will panic, so we use recover to check we got past validation.
	validSeverities := []string{"INFO", "WARN", "ERROR", "CRITICAL"}
	for _, sev := range validSeverities {
		t.Run(sev, func(t *testing.T) {
			r := chi.NewRouter()
			r.Post("/rules", CreateRule(nil))

			body := `{"name":"test","severity":"` + sev + `","condition":{"type":"keyword"}}`
			req := httptest.NewRequest(http.MethodPost, "/rules", strings.NewReader(body))
			rr := httptest.NewRecorder()

			// With nil db, the handler will panic when trying to call db.QueryRowContext.
			// A panic means we passed validation successfully.
			func() {
				defer func() {
					if rec := recover(); rec != nil {
						// Expected: nil db dereference means validation passed
					}
				}()
				r.ServeHTTP(rr, req)
			}()

			// If we didn't panic and got 400 with a validation message, that's wrong
			if rr.Code == http.StatusBadRequest {
				respBody := strings.TrimSpace(rr.Body.String())
				if respBody == "name is required" || respBody == "invalid severity" || respBody == "condition is required" {
					t.Errorf("severity %q should be valid but got validation error: %q", sev, respBody)
				}
			}
		})
	}
}

// --- UpdateRule validation ---

func Test_Should_Return400_When_UpdateRuleWithInvalidID(t *testing.T) {
	r := chi.NewRouter()
	r.Put("/rules/{id}", UpdateRule(nil))

	body := `{"name":"test","severity":"INFO","condition":{"type":"keyword"}}`
	req := httptest.NewRequest(http.MethodPut, "/rules/abc", strings.NewReader(body))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
	respBody := strings.TrimSpace(rr.Body.String())
	if respBody != "invalid id" {
		t.Errorf("expected 'invalid id', got %q", respBody)
	}
}

func Test_Should_Return400_When_UpdateRuleWithInvalidJSON(t *testing.T) {
	r := chi.NewRouter()
	r.Put("/rules/{id}", UpdateRule(nil))

	req := httptest.NewRequest(http.MethodPut, "/rules/1", strings.NewReader("bad json"))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
	respBody := strings.TrimSpace(rr.Body.String())
	if respBody != "invalid request body" {
		t.Errorf("expected 'invalid request body', got %q", respBody)
	}
}

func Test_Should_Return400_When_UpdateRuleMissingRequiredFields(t *testing.T) {
	r := chi.NewRouter()
	r.Put("/rules/{id}", UpdateRule(nil))

	tests := []struct {
		name string
		body string
	}{
		{"missing_name", `{"severity":"INFO","condition":{"type":"keyword"}}`},
		{"missing_severity", `{"name":"test","condition":{"type":"keyword"}}`},
		{"missing_condition", `{"name":"test","severity":"INFO"}`},
		{"all_empty", `{}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/rules/1", strings.NewReader(tc.body))
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", rr.Code)
			}
			respBody := strings.TrimSpace(rr.Body.String())
			if respBody != "name, severity, and condition are required" {
				t.Errorf("expected 'name, severity, and condition are required', got %q", respBody)
			}
		})
	}
}

func Test_Should_Return400_When_UpdateRuleWithInvalidSeverity(t *testing.T) {
	r := chi.NewRouter()
	r.Put("/rules/{id}", UpdateRule(nil))

	body := `{"name":"test","severity":"HIGH","condition":{"type":"keyword"}}`
	req := httptest.NewRequest(http.MethodPut, "/rules/1", strings.NewReader(body))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
	respBody := strings.TrimSpace(rr.Body.String())
	if respBody != "invalid severity" {
		t.Errorf("expected 'invalid severity', got %q", respBody)
	}
}

func Test_Should_Return400_When_UpdateRuleWithOverflowID(t *testing.T) {
	r := chi.NewRouter()
	r.Put("/rules/{id}", UpdateRule(nil))

	body := `{"name":"test","severity":"INFO","condition":{"type":"keyword"}}`
	req := httptest.NewRequest(http.MethodPut, "/rules/99999999999999999999", strings.NewReader(body))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}
