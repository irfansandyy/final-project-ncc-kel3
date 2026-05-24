package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
)

func TestListAlerts(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	t.Run("should_return_alerts_when_valid_request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/alerts?page=1&limit=10&severity=CRITICAL&status=open", nil)
		w := httptest.NewRecorder()

		mock.ExpectQuery("^SELECT COUNT\\(\\*\\) FROM \\(SELECT id").
			WithArgs("CRITICAL", "open").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		mock.ExpectQuery("^SELECT id, rule_id, event_id, severity, status, message, metadata, created_at, updated_at FROM alerts").
			WithArgs("CRITICAL", "open", 10, 0).
			WillReturnRows(sqlmock.NewRows([]string{"id", "rule_id", "event_id", "severity", "status", "message", "metadata", "created_at", "updated_at"}).
				AddRow(1, 2, 3, "CRITICAL", "open", "Test Alert", []byte(`{}`), time.Now(), time.Now()))

		handler := ListAlerts(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("should_return_500_when_count_query_fails", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/alerts", nil)
		w := httptest.NewRecorder()

		mock.ExpectQuery("^SELECT COUNT\\(\\*\\) FROM").WillReturnError(sql.ErrConnDone)

		handler := ListAlerts(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", w.Code)
		}
	})

	t.Run("should_return_500_when_fetch_query_fails", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/alerts", nil)
		w := httptest.NewRecorder()

		mock.ExpectQuery("^SELECT COUNT\\(\\*\\) FROM").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery("^SELECT id, rule_id, event_id, severity, status, message, metadata, created_at, updated_at FROM alerts").WillReturnError(sql.ErrConnDone)

		handler := ListAlerts(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", w.Code)
		}
	})
	
	t.Run("should_return_500_when_scan_fails", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/alerts", nil)
		w := httptest.NewRecorder()

		mock.ExpectQuery("^SELECT COUNT\\(\\*\\) FROM").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		// Returning invalid type for ID to force scan error
		mock.ExpectQuery("^SELECT id, rule_id, event_id, severity, status, message, metadata, created_at, updated_at FROM alerts").
			WillReturnRows(sqlmock.NewRows([]string{"id", "rule_id", "event_id", "severity", "status", "message", "metadata", "created_at", "updated_at"}).
				AddRow("invalid_id", 2, 3, "CRITICAL", "open", "Test", []byte(`{}`), time.Now(), time.Now()))

		handler := ListAlerts(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", w.Code)
		}
	})
}

func TestUpdateAlertStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	t.Run("should_return_400_when_id_is_invalid", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/alerts/abc", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "abc")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		
		w := httptest.NewRecorder()
		handler := UpdateAlertStatus(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("should_return_400_when_body_is_invalid", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/alerts/1", bytes.NewBufferString("invalid json"))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		
		w := httptest.NewRecorder()
		handler := UpdateAlertStatus(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("should_return_400_when_status_is_invalid", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/alerts/1", bytes.NewBufferString(`{"status":"invalid_status"}`))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		
		w := httptest.NewRecorder()
		handler := UpdateAlertStatus(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("should_return_404_when_alert_not_found", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/alerts/1", bytes.NewBufferString(`{"status":"acknowledged"}`))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		
		mock.ExpectQuery("^UPDATE alerts SET status").
			WithArgs("acknowledged", 1).
			WillReturnRows(sqlmock.NewRows([]string{})) // Empty rows will cause sql.ErrNoRows in QueryRow().Scan()

		w := httptest.NewRecorder()
		handler := UpdateAlertStatus(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})

	t.Run("should_return_500_when_db_error_occurs", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/alerts/1", bytes.NewBufferString(`{"status":"acknowledged"}`))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		
		mock.ExpectQuery("^UPDATE alerts SET status").
			WithArgs("acknowledged", 1).
			WillReturnError(sql.ErrConnDone)

		w := httptest.NewRecorder()
		handler := UpdateAlertStatus(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", w.Code)
		}
	})

	t.Run("should_return_updated_alert_when_valid", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/alerts/1", bytes.NewBufferString(`{"status":"acknowledged"}`))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		
		mock.ExpectQuery("^UPDATE alerts SET status").
			WithArgs("acknowledged", 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "rule_id", "event_id", "severity", "status", "message", "metadata", "created_at", "updated_at"}).
				AddRow(1, 2, 3, "CRITICAL", "acknowledged", "Test", []byte(`{}`), time.Now(), time.Now()))

		w := httptest.NewRecorder()
		handler := UpdateAlertStatus(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})
}
