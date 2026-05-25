package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"app-backend/config"
	"app-backend/services"

	"github.com/DATA-DOG/go-sqlmock"
)

var llmHandler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func init() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		llmHandler(w, r)
	}))
	services.GetLLMService(config.Config{
		LLMBaseURL: ts.URL,
		LLMTimeout: 2 * time.Second,
	})
}

func Test_should_return_ok_when_simple_query_param_is_true(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	llm := services.GetLLMService(config.Config{})
	healthService := services.NewHealthService(db, llm)
	handler := NewHealthHandler(healthService)

	req, _ := http.NewRequest(http.MethodGet, "/health?simple=true", nil)
	rr := httptest.NewRecorder()

	handler.Health(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", resp["status"])
	}
}

func Test_should_return_ok_when_all_services_are_healthy(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("failed to open stub db: %v", err)
	}
	defer db.Close()

	mock.ExpectPing()

	llmHandler = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	llm := services.GetLLMService(config.Config{})
	healthService := services.NewHealthService(db, llm)
	handler := NewHealthHandler(healthService)

	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.Health(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", resp["status"])
	}
}

func Test_should_return_degraded_when_database_is_down(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("failed to open stub db: %v", err)
	}
	defer db.Close()

	mock.ExpectPing().WillReturnError(context.DeadlineExceeded)

	llmHandler = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	llm := services.GetLLMService(config.Config{})
	healthService := services.NewHealthService(db, llm)
	handler := NewHealthHandler(healthService)

	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.Health(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if resp["status"] != "degraded" {
		t.Errorf("expected status 'degraded', got %v", resp["status"])
	}
	servicesInfo := resp["services"].(map[string]any)
	if servicesInfo["database"] != "error" {
		t.Errorf("expected database status 'error', got %v", servicesInfo["database"])
	}
}

func Test_should_return_degraded_when_llm_is_down(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("failed to open stub db: %v", err)
	}
	defer db.Close()

	mock.ExpectPing()

	llmHandler = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}

	llm := services.GetLLMService(config.Config{})
	healthService := services.NewHealthService(db, llm)
	handler := NewHealthHandler(healthService)

	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	handler.Health(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if resp["status"] != "degraded" {
		t.Errorf("expected status 'degraded', got %v", resp["status"])
	}
	servicesInfo := resp["services"].(map[string]any)
	if servicesInfo["llm"] != "error" {
		t.Errorf("expected llm status 'error', got %v", servicesInfo["llm"])
	}
}
