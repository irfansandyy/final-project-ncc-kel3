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

func TestListRules_DB(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	t.Run("should_return_rules_when_valid_request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/rules", nil)
		w := httptest.NewRecorder()

		mock.ExpectQuery("^SELECT id, name, description, condition, severity, action, enabled, version, created_at, updated_at FROM rules ORDER BY id ASC").
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "condition", "severity", "action", "enabled", "version", "created_at", "updated_at"}).
				AddRow(1, "Rule1", "Desc", []byte(`{"type":"keyword"}`), "INFO", []byte(`{}`), true, 1, time.Now(), time.Now()))

		handler := ListRules(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("should_return_500_when_db_error", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/rules", nil)
		w := httptest.NewRecorder()

		mock.ExpectQuery("^SELECT id, name, description, condition, severity, action, enabled, version, created_at, updated_at FROM rules").
			WillReturnError(sql.ErrConnDone)

		handler := ListRules(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", w.Code)
		}
	})
	
	t.Run("should_return_500_when_scan_fails", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/rules", nil)
		w := httptest.NewRecorder()

		mock.ExpectQuery("^SELECT id, name, description, condition, severity, action, enabled, version, created_at, updated_at FROM rules").
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "condition", "severity", "action", "enabled", "version", "created_at", "updated_at"}).
				AddRow("invalid_id", "Rule1", "Desc", []byte(`{"type":"keyword"}`), "INFO", []byte(`{}`), true, 1, time.Now(), time.Now()))

		handler := ListRules(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", w.Code)
		}
	})
}

func TestGetRule_DB(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	t.Run("should_return_rule_when_found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/rules/1", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		mock.ExpectQuery("^SELECT id, name, description, condition, severity, action, enabled, version, created_at, updated_at FROM rules WHERE id = \\$1").
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "condition", "severity", "action", "enabled", "version", "created_at", "updated_at"}).
				AddRow(1, "Rule1", "Desc", []byte(`{"type":"keyword"}`), "INFO", []byte(`{}`), true, 1, time.Now(), time.Now()))

		w := httptest.NewRecorder()
		handler := GetRule(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("should_return_404_when_not_found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/rules/1", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		mock.ExpectQuery("^SELECT id, name, description, condition, severity, action, enabled, version, created_at, updated_at FROM rules WHERE id = \\$1").
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{}))

		w := httptest.NewRecorder()
		handler := GetRule(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})
}

func TestCreateRule_DB(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	t.Run("should_create_rule_and_return_201_when_valid", func(t *testing.T) {
		body := `{"name":"test", "severity":"INFO", "condition":{"type":"keyword"}, "enabled":false}`
		req := httptest.NewRequest("POST", "/rules", bytes.NewBufferString(body))
		w := httptest.NewRecorder()

		mock.ExpectQuery("^INSERT INTO rules").
			WithArgs("test", "", []byte(`{"type":"keyword"}`), "INFO", []byte(`{}`), false).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "condition", "severity", "action", "enabled", "version", "created_at", "updated_at"}).
				AddRow(1, "test", "", []byte(`{"type":"keyword"}`), "INFO", []byte(`{}`), false, 1, time.Now(), time.Now()))

		handler := CreateRule(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("expected status 201, got %d", w.Code)
		}
	})

	t.Run("should_return_500_when_insert_fails", func(t *testing.T) {
		body := `{"name":"test", "severity":"INFO", "condition":{"type":"keyword"}}`
		req := httptest.NewRequest("POST", "/rules", bytes.NewBufferString(body))
		w := httptest.NewRecorder()

		mock.ExpectQuery("^INSERT INTO rules").WillReturnError(sql.ErrConnDone)

		handler := CreateRule(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", w.Code)
		}
	})
}

func TestUpdateRule_DB(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	t.Run("should_update_rule_and_return_200_when_valid", func(t *testing.T) {
		body := `{"name":"test2", "severity":"WARN", "condition":{"type":"keyword"}, "enabled":false}`
		req := httptest.NewRequest("PUT", "/rules/1", bytes.NewBufferString(body))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		mock.ExpectQuery("^UPDATE rules SET name").
			WithArgs("test2", "", []byte(`{"type":"keyword"}`), "WARN", []byte(`{}`), false, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "condition", "severity", "action", "enabled", "version", "created_at", "updated_at"}).
				AddRow(1, "test2", "", []byte(`{"type":"keyword"}`), "WARN", []byte(`{}`), false, 2, time.Now(), time.Now()))

		handler := UpdateRule(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	t.Run("should_return_404_when_update_not_found", func(t *testing.T) {
		body := `{"name":"test2", "severity":"WARN", "condition":{"type":"keyword"}}`
		req := httptest.NewRequest("PUT", "/rules/1", bytes.NewBufferString(body))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		mock.ExpectQuery("^UPDATE rules SET name").WillReturnRows(sqlmock.NewRows([]string{}))

		handler := UpdateRule(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})
	
	t.Run("should_return_500_when_update_error", func(t *testing.T) {
		body := `{"name":"test2", "severity":"WARN", "condition":{"type":"keyword"}}`
		req := httptest.NewRequest("PUT", "/rules/1", bytes.NewBufferString(body))
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		mock.ExpectQuery("^UPDATE rules SET name").WillReturnError(sql.ErrConnDone)

		handler := UpdateRule(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", w.Code)
		}
	})
}

func TestDeleteRule_DB(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	t.Run("should_return_204_when_delete_succeeds", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/rules/1", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		
		mock.ExpectExec("^DELETE FROM rules WHERE id = \\$1").
			WithArgs(1).
			WillReturnResult(sqlmock.NewResult(0, 1))

		w := httptest.NewRecorder()
		handler := DeleteRule(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("expected status 204, got %d", w.Code)
		}
	})

	t.Run("should_return_409_when_delete_fails_with_conflict", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/rules/1", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		
		mock.ExpectExec("^DELETE FROM rules WHERE id = \\$1").
			WithArgs(1).
			WillReturnError(sql.ErrConnDone)

		w := httptest.NewRecorder()
		handler := DeleteRule(db)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusConflict {
			t.Errorf("expected status 409, got %d", w.Code)
		}
	})
}
