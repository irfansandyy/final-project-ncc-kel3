package handlers

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gorilla/websocket"
)

func TestHub_Run(t *testing.T) {
	t.Run("should_run_and_exit_cleanly_on_context_cancel", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("failed to open stub db: %v", err)
		}
		defer db.Close()

		mock.ExpectQuery("^SELECT COALESCE\\(MAX\\(id\\), 0\\) FROM events").WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(10))
		mock.ExpectQuery("^SELECT COALESCE\\(MAX\\(id\\), 0\\) FROM alerts").WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(5))

		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		hub := NewHub(db, 5000, logger) // Poll every 5s so it won't hit polling before cancel

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		hub.Run(ctx) // Blocks until context is done

		if hub.lastEventID != 10 {
			t.Errorf("expected lastEventID to be 10, got %d", hub.lastEventID)
		}
		if hub.lastAlertID != 5 {
			t.Errorf("expected lastAlertID to be 5, got %d", hub.lastAlertID)
		}
	})
}

func TestHub_pollEvents(t *testing.T) {
	t.Run("should_update_lastEventID_and_broadcast", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("failed to open stub db: %v", err)
		}
		defer db.Close()

		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		hub := NewHub(db, 100, logger)

		mock.ExpectQuery("^SELECT id, source_id, timestamp, level, source, message, raw, metadata, created_at FROM events").
			WithArgs(0).
			WillReturnRows(sqlmock.NewRows([]string{"id", "source_id", "timestamp", "level", "source", "message", "raw", "metadata", "created_at"}).
				AddRow(1, 1, time.Now(), "INFO", "src", "msg", "raw", []byte(`{}`), time.Now()))

		go func() {
			select {
			case <-hub.broadcast:
			case <-time.After(1 * time.Second):
				t.Errorf("expected broadcast message")
			}
		}()

		hub.pollEvents(context.Background())

		if hub.lastEventID != 1 {
			t.Errorf("expected lastEventID to be 1, got %d", hub.lastEventID)
		}
	})

	t.Run("should_handle_db_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("failed to open stub db: %v", err)
		}
		defer db.Close()

		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		hub := NewHub(db, 100, logger)

		mock.ExpectQuery("^SELECT id, source_id, timestamp, level, source, message, raw, metadata, created_at FROM events").
			WithArgs(0).
			WillReturnError(sql.ErrConnDone)

		hub.pollEvents(context.Background()) // Should just log and return
	})
}

func TestHub_pollAlerts(t *testing.T) {
	t.Run("should_update_lastAlertID_and_broadcast", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("failed to open stub db: %v", err)
		}
		defer db.Close()

		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		hub := NewHub(db, 100, logger)

		mock.ExpectQuery("^SELECT id, rule_id, event_id, severity, status, message, metadata, created_at, updated_at FROM alerts").
			WithArgs(0).
			WillReturnRows(sqlmock.NewRows([]string{"id", "rule_id", "event_id", "severity", "status", "message", "metadata", "created_at", "updated_at"}).
				AddRow(1, 1, 1, "CRITICAL", "open", "msg", []byte(`{}`), time.Now(), time.Now()))

		go func() {
			select {
			case <-hub.broadcast:
			case <-time.After(1 * time.Second):
				t.Errorf("expected broadcast message")
			}
		}()

		hub.pollAlerts(context.Background())

		if hub.lastAlertID != 1 {
			t.Errorf("expected lastAlertID to be 1, got %d", hub.lastAlertID)
		}
	})

	t.Run("should_handle_db_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("failed to open stub db: %v", err)
		}
		defer db.Close()

		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		hub := NewHub(db, 100, logger)

		mock.ExpectQuery("^SELECT id, rule_id, event_id, severity, status, message, metadata, created_at, updated_at FROM alerts").
			WithArgs(0).
			WillReturnError(sql.ErrConnDone)

		hub.pollAlerts(context.Background()) // Should just log and return
	})
}

func TestHub_HandleWebSocket(t *testing.T) {
	t.Run("should_upgrade_connection", func(t *testing.T) {
		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		hub := NewHub(nil, 100, logger)

		go func() {
			// read the registration to avoid blocking
			<-hub.register
		}()

		server := httptest.NewServer(hub.HandleWebSocket())
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("failed to connect to websocket: %v", err)
		}
		defer ws.Close()
	})

	t.Run("should_return_error_for_non_websocket", func(t *testing.T) {
		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		hub := NewHub(nil, 100, logger)

		req := httptest.NewRequest("GET", "/ws", nil)
		w := httptest.NewRecorder()

		handler := hub.HandleWebSocket()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400 for non-websocket upgrade, got %d", w.Code)
		}
	})
}
