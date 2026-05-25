package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newRequest is a small helper to build an *http.Request with an optional JSON body.
func newRequest(t *testing.T, method, target, body string) *http.Request {
	t.Helper()
	var req *http.Request
	var err error
	if body != "" {
		req, err = http.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(method, target, nil)
	}
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	return req
}

// ---------------------------------------------------------------------------
// Stub handler – replace with your real handlers.GetAlerts / handlers.CreateAlert
// once the package is importable here.
// ---------------------------------------------------------------------------

// stubGetAlerts mimics the shape of your real GetAlerts handler so the test
// logic is ready to swap in without changes.
func stubGetAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode([]map[string]interface{}{
		{"id": "1", "severity": "high", "message": "Suspicious login"},
	})
}

func stubCreateAlert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if _, ok := body["severity"]; !ok {
		http.Error(w, "missing severity", http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	body["id"] = "generated-id"
	_ = json.NewEncoder(w).Encode(body)
}

func stubDeleteAlert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestGetAlerts_ReturnsOK(t *testing.T) {
	req := newRequest(t, http.MethodGet, "/api/siem/alerts", "")
	rr := httptest.NewRecorder()

	stubGetAlerts(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestGetAlerts_ReturnsJSONArray(t *testing.T) {
	req := newRequest(t, http.MethodGet, "/api/siem/alerts", "")
	rr := httptest.NewRecorder()

	stubGetAlerts(rr, req)

	var result []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("response body is not valid JSON array: %v", err)
	}
	if len(result) == 0 {
		t.Error("expected at least one alert in response")
	}
}

func TestGetAlerts_WrongMethod_Returns405(t *testing.T) {
	req := newRequest(t, http.MethodPost, "/api/siem/alerts", "")
	rr := httptest.NewRecorder()

	stubGetAlerts(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestCreateAlert_ValidPayload_Returns201(t *testing.T) {
	body := `{"severity":"critical","message":"Port scan detected","source_ip":"10.0.0.1"}`
	req := newRequest(t, http.MethodPost, "/api/siem/alerts", body)
	rr := httptest.NewRecorder()

	stubCreateAlert(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rr.Code)
	}
}

func TestCreateAlert_MissingSeverity_Returns422(t *testing.T) {
	body := `{"message":"missing severity field"}`
	req := newRequest(t, http.MethodPost, "/api/siem/alerts", body)
	rr := httptest.NewRecorder()

	stubCreateAlert(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rr.Code)
	}
}

func TestCreateAlert_MalformedJSON_Returns400(t *testing.T) {
	req := newRequest(t, http.MethodPost, "/api/siem/alerts", `{not json}`)
	rr := httptest.NewRecorder()

	stubCreateAlert(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestCreateAlert_ResponseBodyContainsID(t *testing.T) {
	body := `{"severity":"low","message":"Info event"}`
	req := newRequest(t, http.MethodPost, "/api/siem/alerts", body)
	rr := httptest.NewRecorder()

	stubCreateAlert(rr, req)

	var result map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("could not decode response: %v", err)
	}
	if _, ok := result["id"]; !ok {
		t.Error("response missing 'id' field")
	}
}

func TestDeleteAlert_Returns204(t *testing.T) {
	req := newRequest(t, http.MethodDelete, "/api/siem/alerts/1", "")
	rr := httptest.NewRecorder()

	stubDeleteAlert(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rr.Code)
	}
}

func TestDeleteAlert_WrongMethod_Returns405(t *testing.T) {
	req := newRequest(t, http.MethodGet, "/api/siem/alerts/1", "")
	rr := httptest.NewRecorder()

	stubDeleteAlert(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}
