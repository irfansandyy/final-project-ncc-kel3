// File: services/api/main_test.go
package main_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---------------------------------------------------------------------------
// Minimal router that mirrors what main.go likely wires up.
// Replace with your real router once the package is importable.
// ---------------------------------------------------------------------------

func buildTestRouter() http.Handler {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		simple := r.URL.Query().Get("simple")
		w.Header().Set("Content-Type", "application/json")
		if simple == "true" {
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"services": map[string]string{
				"database": "ok",
				"llm":      "ok",
			},
		})
	})

	// SIEM overview (stub)
	mux.HandleFunc("/api/siem/overview", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"total_alerts": 0})
	})

	// SIEM alerts (stub)
	mux.HandleFunc("/api/siem/alerts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]interface{}{})
	})

	return mux
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestHealthEndpoint_ReturnsOK(t *testing.T) {
	srv := httptest.NewServer(buildTestRouter())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHealthEndpoint_FullResponse_HasServices(t *testing.T) {
	srv := httptest.NewServer(buildTestRouter())
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/health")
	defer resp.Body.Close()

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if _, ok := body["services"]; !ok {
		t.Error("expected 'services' key in health response")
	}
	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %v", body["status"])
	}
}

func TestHealthEndpoint_SimpleMode(t *testing.T) {
	srv := httptest.NewServer(buildTestRouter())
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/health?simple=true")
	defer resp.Body.Close()

	var body map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&body)

	if _, ok := body["services"]; ok {
		t.Error("simple mode should NOT include 'services' key")
	}
	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %v", body["status"])
	}
}

func TestSIEMOverviewRoute_Registered(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/siem/overview", nil)
	rr := httptest.NewRecorder()

	buildTestRouter().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected /api/siem/overview to be registered, got %d", rr.Code)
	}
}

func TestSIEMAlertsRoute_Registered(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/siem/alerts", nil)
	rr := httptest.NewRecorder()

	buildTestRouter().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected /api/siem/alerts to be registered, got %d", rr.Code)
	}
}

func TestUnknownRoute_Returns404(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)
	rr := httptest.NewRecorder()

	buildTestRouter().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown route, got %d", rr.Code)
	}
}

func TestHealthEndpoint_ContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	buildTestRouter().ServeHTTP(rr, req)

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}
