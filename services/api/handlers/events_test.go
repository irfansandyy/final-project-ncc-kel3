package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
)

func TestListEvents(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	t.Run("should_return_events_when_valid_request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/events?page=1&limit=10&level=ERROR&source=nginx", nil)
		w := httptest.NewRecorder()

		mock.ExpectQuery("^SELECT COUNT\\(\\*\\) FROM \\(SELECT id").
			WithArgs("ERROR", "nginx").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		mock.ExpectQuery("^SELECT id, source_id, timestamp, level, source, message, raw, metadata, created_at FROM events").
			WithArgs("ERROR", "nginx", 10, 0).
			WillReturnRows(sqlmock.NewRows([]string{"id", "source_id", "timestamp", "level", "source", "message", "raw", "metadata", "created_at"}).
				AddRow(1, 2, time.Now(), "ERROR", "nginx", "Test Message", "raw message", []byte(`{}`), time.Now()))

		handler := ListEvents(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("should_return_500_when_count_query_fails", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/events", nil)
		w := httptest.NewRecorder()

		mock.ExpectQuery("^SELECT COUNT\\(\\*\\) FROM").WillReturnError(sql.ErrConnDone)

		handler := ListEvents(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", w.Code)
		}
	})

	t.Run("should_return_500_when_fetch_query_fails", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/events", nil)
		w := httptest.NewRecorder()

		mock.ExpectQuery("^SELECT COUNT\\(\\*\\) FROM").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery("^SELECT id, source_id, timestamp, level, source, message, raw, metadata, created_at FROM events").WillReturnError(sql.ErrConnDone)

		handler := ListEvents(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", w.Code)
		}
	})

	t.Run("should_return_500_when_scan_fails", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/events", nil)
		w := httptest.NewRecorder()

		mock.ExpectQuery("^SELECT COUNT\\(\\*\\) FROM").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		// Return invalid type for ID
		mock.ExpectQuery("^SELECT id, source_id, timestamp, level, source, message, raw, metadata, created_at FROM events").
			WillReturnRows(sqlmock.NewRows([]string{"id", "source_id", "timestamp", "level", "source", "message", "raw", "metadata", "created_at"}).
				AddRow("invalid_id", 2, time.Now(), "ERROR", "nginx", "Test Message", "raw message", []byte(`{}`), time.Now()))

		handler := ListEvents(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", w.Code)
		}
	})
}

func TestGetEvent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	t.Run("should_return_400_when_id_is_invalid", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/events/abc", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "abc")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		
		w := httptest.NewRecorder()
		handler := GetEvent(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("should_return_404_when_event_not_found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/events/1", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		
		mock.ExpectQuery("^SELECT id, source_id, timestamp, level, source, message, raw, metadata, created_at FROM events WHERE id = \\$1").
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{}))

		w := httptest.NewRecorder()
		handler := GetEvent(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})

	t.Run("should_return_500_when_db_error_occurs", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/events/1", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		
		mock.ExpectQuery("^SELECT id, source_id, timestamp, level, source, message, raw, metadata, created_at FROM events WHERE id = \\$1").
			WithArgs(1).
			WillReturnError(sql.ErrConnDone)

		w := httptest.NewRecorder()
		handler := GetEvent(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", w.Code)
		}
	})

	t.Run("should_return_event_when_valid", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/events/1", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		
		mock.ExpectQuery("^SELECT id, source_id, timestamp, level, source, message, raw, metadata, created_at FROM events WHERE id = \\$1").
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "source_id", "timestamp", "level", "source", "message", "raw", "metadata", "created_at"}).
				AddRow(1, 2, time.Now(), "INFO", "test", "test message", "raw", []byte(`{}`), time.Now()))

		w := httptest.NewRecorder()
		handler := GetEvent(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})
}
