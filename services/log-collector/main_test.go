package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/fpncc/log-collector/collector"
)

func TestEnvOrDefault(t *testing.T) {
	os.Clearenv()
	if got := envOrDefault("TEST_ENV", "default"); got != "default" {
		t.Errorf("expected default, got %s", got)
	}
	os.Setenv("TEST_ENV", "value")
	if got := envOrDefault("TEST_ENV", "default"); got != "value" {
		t.Errorf("expected value, got %s", got)
	}
}

func TestLoadSources(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open stub db: %v", err)
	}
	defer db.Close()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	w, _ := collector.NewWatcher(func(sourceID int64, line string) {}, logger)
	defer w.Stop()

	t.Run("should_load_sources_successfully", func(t *testing.T) {
		mock.ExpectQuery("^SELECT id, file_path FROM log_sources WHERE enabled = true").
			WillReturnRows(sqlmock.NewRows([]string{"id", "file_path"}).
				AddRow(1, "/tmp/test_file1.log").
				AddRow(2, "/tmp/test_file2.log"))

		known := make(map[int64]bool)

		os.WriteFile("/tmp/test_file1.log", []byte{}, 0644)
		os.WriteFile("/tmp/test_file2.log", []byte{}, 0644)
		defer os.Remove("/tmp/test_file1.log")
		defer os.Remove("/tmp/test_file2.log")

		err := loadSources(db, w, known, logger)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if !known[1] || !known[2] {
			t.Errorf("expected known map to have 1 and 2, got %v", known)
		}
	})

	t.Run("should_handle_db_error", func(t *testing.T) {
		mock.ExpectQuery("^SELECT id, file_path FROM log_sources WHERE enabled = true").
			WillReturnError(sql.ErrConnDone)

		known := make(map[int64]bool)
		err := loadSources(db, w, known, logger)
		if err == nil {
			t.Errorf("expected error, got nil")
		}
	})

	t.Run("should_handle_scan_error", func(t *testing.T) {
		mock.ExpectQuery("^SELECT id, file_path FROM log_sources WHERE enabled = true").
			WillReturnRows(sqlmock.NewRows([]string{"id", "file_path"}).
				AddRow("invalid_id", "/tmp/file1.log"))

		known := make(map[int64]bool)
		err := loadSources(db, w, known, logger)
		if err == nil {
			t.Errorf("expected error, got nil")
		}
	})

	t.Run("should_skip_known_sources", func(t *testing.T) {
		mock.ExpectQuery("^SELECT id, file_path FROM log_sources WHERE enabled = true").
			WillReturnRows(sqlmock.NewRows([]string{"id", "file_path"}).
				AddRow(1, "/tmp/file1.log"))

		known := map[int64]bool{1: true}
		err := loadSources(db, w, known, logger)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("should_handle_add_path_error", func(t *testing.T) {
		mock.ExpectQuery("^SELECT id, file_path FROM log_sources WHERE enabled = true").
			WillReturnRows(sqlmock.NewRows([]string{"id", "file_path"}).
				AddRow(3, "/tmp/non_existent_file.log"))

		known := make(map[int64]bool)
		err := loadSources(db, w, known, logger)
		if err != nil {
			t.Errorf("expected no error, just log warning, got %v", err)
		}
		if known[3] {
			t.Errorf("expected known to not have 3, got %v", known)
		}
	})
}

func TestInsertBatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open stub db: %v", err)
	}
	defer db.Close()

	t.Run("should_insert_successfully", func(t *testing.T) {
		batch := []logEntry{
			{sourceID: 1, line: "line1"},
			{sourceID: 1, line: "line2"},
		}

		mock.ExpectExec("^INSERT INTO raw_logs\\(source_id, line\\) VALUES \\(\\$1,\\$2\\),\\(\\$3,\\$4\\)").
			WithArgs(1, "line1", 1, "line2").
			WillReturnResult(sqlmock.NewResult(2, 2))

		err := insertBatch(context.Background(), db, batch)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("should_return_nil_for_empty_batch", func(t *testing.T) {
		err := insertBatch(context.Background(), db, []logEntry{})
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("should_return_error_when_db_fails", func(t *testing.T) {
		batch := []logEntry{{sourceID: 1, line: "line1"}}
		mock.ExpectExec("^INSERT INTO raw_logs").WillReturnError(sql.ErrConnDone)

		err := insertBatch(context.Background(), db, batch)
		if err == nil {
			t.Errorf("expected error, got nil")
		}
	})
}

func TestRunBatchInserter(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open stub db: %v", err)
	}
	defer db.Close()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	t.Run("should_insert_when_channel_closed", func(t *testing.T) {
		ch := make(chan logEntry, 2)
		ch <- logEntry{sourceID: 1, line: "line1"}
		ch <- logEntry{sourceID: 1, line: "line2"}
		close(ch)

		mock.ExpectExec("^INSERT INTO raw_logs").WillReturnResult(sqlmock.NewResult(2, 2))

		runBatchInserter(context.Background(), db, ch, logger)
	})

	t.Run("should_insert_when_context_cancelled", func(t *testing.T) {
		ch := make(chan logEntry, 2)
		ch <- logEntry{sourceID: 1, line: "line1"}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // immediately cancel
		close(ch) // Close channel so range exits

		mock.ExpectExec("^INSERT INTO raw_logs").WillReturnResult(sqlmock.NewResult(1, 1))

		runBatchInserter(ctx, db, ch, logger)
	})

	t.Run("should_handle_db_error_on_flush", func(t *testing.T) {
		ch := make(chan logEntry, 1)
		ch <- logEntry{sourceID: 1, line: "line1"}
		close(ch)

		mock.ExpectExec("^INSERT INTO raw_logs").WillReturnError(sql.ErrConnDone)

		runBatchInserter(context.Background(), db, ch, logger) // Should just log error
	})
}
