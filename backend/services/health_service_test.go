package services

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func Test_should_return_ok_when_both_db_and_llm_are_healthy(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectPing()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	llm := &LLMService{
		baseURL: ts.URL,
		model:   "test",
		client:  ts.Client(),
	}

	service := NewHealthService(db, llm)
	status := service.Check(context.Background())

	if status.Status != "ok" {
		t.Errorf("expected status 'ok', got '%s'", status.Status)
	}
	if status.Services["database"] != "ok" {
		t.Errorf("expected database 'ok', got '%s'", status.Services["database"])
	}
	if status.Services["llm"] != "ok" {
		t.Errorf("expected llm 'ok', got '%s'", status.Services["llm"])
	}
	if status.Errors != nil {
		t.Errorf("expected no errors, got %v", status.Errors)
	}
}

func Test_should_return_degraded_when_db_is_down(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectPing().WillReturnError(errors.New("db error"))

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	llm := &LLMService{
		baseURL: ts.URL,
		model:   "test",
		client:  ts.Client(),
	}

	service := NewHealthService(db, llm)
	status := service.Check(context.Background())

	if status.Status != "degraded" {
		t.Errorf("expected status 'degraded', got '%s'", status.Status)
	}
	if status.Services["database"] != "error" {
		t.Errorf("expected database 'error', got '%s'", status.Services["database"])
	}
	if status.Services["llm"] != "ok" {
		t.Errorf("expected llm 'ok', got '%s'", status.Services["llm"])
	}
	if status.Errors["database"] != "db error" {
		t.Errorf("expected database error 'db error', got '%v'", status.Errors["database"])
	}
}

func Test_should_return_degraded_when_llm_is_down(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectPing()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	llm := &LLMService{
		baseURL: ts.URL,
		model:   "test",
		client:  ts.Client(),
	}

	service := NewHealthService(db, llm)
	status := service.Check(context.Background())

	if status.Status != "degraded" {
		t.Errorf("expected status 'degraded', got '%s'", status.Status)
	}
	if status.Services["database"] != "ok" {
		t.Errorf("expected database 'ok', got '%s'", status.Services["database"])
	}
	if status.Services["llm"] != "error" {
		t.Errorf("expected llm 'error', got '%s'", status.Services["llm"])
	}
	if status.Errors["llm"] == "" {
		t.Errorf("expected llm error to be populated")
	}
}

func Test_should_return_degraded_when_both_down(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectPing().WillReturnError(errors.New("db error"))

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	llm := &LLMService{
		baseURL: ts.URL,
		model:   "test",
		client:  ts.Client(),
	}

	service := NewHealthService(db, llm)
	status := service.Check(context.Background())

	if status.Status != "degraded" {
		t.Errorf("expected status 'degraded', got '%s'", status.Status)
	}
	if status.Services["database"] != "error" {
		t.Errorf("expected database 'error', got '%s'", status.Services["database"])
	}
	if status.Services["llm"] != "error" {
		t.Errorf("expected llm 'error', got '%s'", status.Services["llm"])
	}
	if len(status.Errors) != 2 {
		t.Errorf("expected 2 errors, got %v", len(status.Errors))
	}
}
