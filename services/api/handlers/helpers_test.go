package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	t.Run("should_write_json_response_when_valid_data", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := map[string]string{"message": "hello"}

		writeJSON(w, http.StatusCreated, data)

		if w.Code != http.StatusCreated {
			t.Errorf("expected status 201, got %d", w.Code)
		}
		if w.Header().Get("Content-Type") != "application/json" {
			t.Errorf("expected content type application/json, got %s", w.Header().Get("Content-Type"))
		}

		var resp map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		if err != nil {
			t.Errorf("expected valid json, got error %v", err)
		}
		if resp["message"] != "hello" {
			t.Errorf("expected message hello, got %s", resp["message"])
		}
	})
}
