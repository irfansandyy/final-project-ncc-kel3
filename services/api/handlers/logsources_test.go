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

func TestListLogSources(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	t.Run("should_return_sources_when_valid_request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/log-sources", nil)
		w := httptest.NewRecorder()

		mock.ExpectQuery("^SELECT id, name, file_path, format, enabled, created_at, updated_at FROM log_sources ORDER BY id ASC").
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "file_path", "format", "enabled", "created_at", "updated_at"}).
				AddRow(1, "Test Source", "/var/log/test.log", "syslog", true, time.Now(), time.Now()))

		handler := ListLogSources(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("should_return_500_when_db_query_fails", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/log-sources", nil)
		w := httptest.NewRecorder()

		mock.ExpectQuery("^SELECT id, name, file_path, format, enabled, created_at, updated_at FROM log_sources ORDER BY id ASC").
			WillReturnError(sql.ErrConnDone)

		handler := ListLogSources(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", w.Code)
		}
	})

	t.Run("should_return_500_when_scan_fails", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/log-sources", nil)
		w := httptest.NewRecorder()

		mock.ExpectQuery("^SELECT id, name, file_path, format, enabled, created_at, updated_at FROM log_sources ORDER BY id ASC").
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "file_path", "format", "enabled", "created_at", "updated_at"}).
				AddRow("invalid_id", "Test Source", "/var/log/test.log", "syslog", true, time.Now(), time.Now()))

		handler := ListLogSources(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", w.Code)
		}
	})
}

func TestCreateLogSource(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	t.Run("should_return_400_when_body_is_invalid", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/log-sources", bytes.NewBufferString("invalid json"))
		w := httptest.NewRecorder()
		
		handler := CreateLogSource(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("should_return_400_when_missing_required_fields", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/log-sources", bytes.NewBufferString(`{"name":""}`))
		w := httptest.NewRecorder()
		
		handler := CreateLogSource(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("should_return_400_when_format_is_invalid", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/log-sources", bytes.NewBufferString(`{"name":"test", "file_path":"/tmp/test.log", "format":"invalid"}`))
		w := httptest.NewRecorder()
		
		handler := CreateLogSource(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("should_use_default_format_and_return_201_when_valid", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/log-sources", bytes.NewBufferString(`{"name":"test", "file_path":"/tmp/test.log"}`))
		w := httptest.NewRecorder()
		
		mock.ExpectQuery("^INSERT INTO log_sources").
			WithArgs("test", "/tmp/test.log", "auto", true).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "file_path", "format", "enabled", "created_at", "updated_at"}).
				AddRow(1, "test", "/tmp/test.log", "auto", true, time.Now(), time.Now()))

		handler := CreateLogSource(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("expected status 201, got %d", w.Code)
		}
	})

	t.Run("should_return_500_when_db_insert_fails", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/log-sources", bytes.NewBufferString(`{"name":"test", "file_path":"/tmp/test.log", "format":"syslog"}`))
		w := httptest.NewRecorder()
		
		mock.ExpectQuery("^INSERT INTO log_sources").
			WithArgs("test", "/tmp/test.log", "syslog", true).
			WillReturnError(sql.ErrConnDone)

		handler := CreateLogSource(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", w.Code)
		}
	})
}

func TestDeleteLogSource(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	t.Run("should_return_400_when_id_is_invalid", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/log-sources/abc", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "abc")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		
		w := httptest.NewRecorder()
		handler := DeleteLogSource(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("should_return_409_when_db_exec_fails", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/log-sources/1", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		
		mock.ExpectExec("^DELETE FROM log_sources WHERE id = \\$1").
			WithArgs(1).
			WillReturnError(sql.ErrConnDone)

		w := httptest.NewRecorder()
		handler := DeleteLogSource(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusConflict {
			t.Errorf("expected status 409, got %d", w.Code)
		}
	})

	t.Run("should_return_204_when_delete_succeeds", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/log-sources/1", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		
		mock.ExpectExec("^DELETE FROM log_sources WHERE id = \\$1").
			WithArgs(1).
			WillReturnResult(sqlmock.NewResult(0, 1))

		w := httptest.NewRecorder()
		handler := DeleteLogSource(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("expected status 204, got %d", w.Code)
		}
	})
}
