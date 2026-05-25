package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ---------------------------------------------------------------------------
// Stub handler – replace with your real handlers.GetOverview
// ---------------------------------------------------------------------------

type overviewResponse struct {
	TotalAlerts    int    `json:"total_alerts"`
	CriticalAlerts int    `json:"critical_alerts"`
	LogsIngested   int64  `json:"logs_ingested"`
	Status         string `json:"status"`
}

func stubGetOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(overviewResponse{
		TotalAlerts:    42,
		CriticalAlerts: 5,
		LogsIngested:   100000,
		Status:         "healthy",
	})
}

func stubGetOverviewNoAuth(w http.ResponseWriter, r *http.Request) {
	// Simulates middleware rejecting unauthenticated request
	token := r.Header.Get("Authorization")
	if token == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	stubGetOverview(w, r)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestGetOverview_ReturnsOK(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/api/siem/overview", nil)
	rr := httptest.NewRecorder()

	stubGetOverview(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestGetOverview_ContentTypeIsJSON(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/api/siem/overview", nil)
	rr := httptest.NewRecorder()

	stubGetOverview(rr, req)

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

func TestGetOverview_ResponseHasRequiredFields(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/api/siem/overview", nil)
	rr := httptest.NewRecorder()

	stubGetOverview(rr, req)

	var resp overviewResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status == "" {
		t.Error("expected non-empty status field")
	}
	if resp.TotalAlerts < 0 {
		t.Error("total_alerts should not be negative")
	}
	if resp.CriticalAlerts < 0 {
		t.Error("critical_alerts should not be negative")
	}
}

func TestGetOverview_CriticalNotExceedTotal(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/api/siem/overview", nil)
	rr := httptest.NewRecorder()

	stubGetOverview(rr, req)

	var resp overviewResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)

	if resp.CriticalAlerts > resp.TotalAlerts {
		t.Errorf("critical_alerts (%d) should not exceed total_alerts (%d)",
			resp.CriticalAlerts, resp.TotalAlerts)
	}
}

func TestGetOverview_WrongMethod_Returns405(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "/api/siem/overview", nil)
	rr := httptest.NewRecorder()

	stubGetOverview(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestGetOverview_NoToken_Returns401(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/api/siem/overview", nil)
	rr := httptest.NewRecorder()

	stubGetOverviewNoAuth(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestGetOverview_WithToken_Returns200(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/api/siem/overview", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rr := httptest.NewRecorder()

	stubGetOverviewNoAuth(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}
