package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestProcessLogs(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open stub db: %v", err)
	}
	defer db.Close()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	t.Run("should_process_logs_when_found", func(t *testing.T) {
		mock.ExpectQuery("^SELECT r.id, r.source_id, r.line, ls.format").
			WillReturnRows(sqlmock.NewRows([]string{"id", "source_id", "line", "format"}).
				AddRow(1, 2, `{"message":"test"}`, "json"))

		// processLine will be called. It begins tx, inserts event, updates raw_logs, commits.
		mock.ExpectBegin()
		mock.ExpectExec("^INSERT INTO events").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("^UPDATE raw_logs SET processed = true").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		processLogs(context.Background(), db, logger)
	})

	t.Run("should_handle_db_query_error", func(t *testing.T) {
		mock.ExpectQuery("^SELECT r.id, r.source_id, r.line, ls.format").
			WillReturnError(sql.ErrConnDone)

		processLogs(context.Background(), db, logger)
	})

	t.Run("should_handle_scan_error", func(t *testing.T) {
		mock.ExpectQuery("^SELECT r.id, r.source_id, r.line, ls.format").
			WillReturnRows(sqlmock.NewRows([]string{"id", "source_id", "line", "format"}).
				AddRow("invalid_id", 2, `{"message":"test"}`, "json"))

		processLogs(context.Background(), db, logger)
	})
}

func TestProcessLine(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open stub db: %v", err)
	}
	defer db.Close()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	t.Run("should_handle_auto_detect_success", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec("^INSERT INTO events").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("^UPDATE raw_logs SET processed = true").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		processLine(context.Background(), db, logger, 1, 2, `{"message":"test"}`, "auto")
	})

	t.Run("should_handle_auto_detect_fallback_when_invalid", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec("^INSERT INTO events").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("^UPDATE raw_logs SET processed = true").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		processLine(context.Background(), db, logger, 1, 2, `not json`, "auto")
	})

	t.Run("should_handle_specific_format_success", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec("^INSERT INTO events").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("^UPDATE raw_logs SET processed = true").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		processLine(context.Background(), db, logger, 1, 2, `{"message":"test"}`, "json")
	})

	t.Run("should_handle_specific_format_fallback_when_invalid", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec("^INSERT INTO events").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("^UPDATE raw_logs SET processed = true").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		processLine(context.Background(), db, logger, 1, 2, `not json`, "json")
	})

	t.Run("should_handle_unknown_format_fallback", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec("^INSERT INTO events").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("^UPDATE raw_logs SET processed = true").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		processLine(context.Background(), db, logger, 1, 2, `{"message":"test"}`, "unknown_format")
	})

	t.Run("should_handle_tx_begin_error", func(t *testing.T) {
		mock.ExpectBegin().WillReturnError(sql.ErrConnDone)

		processLine(context.Background(), db, logger, 1, 2, `{"message":"test"}`, "auto")
	})

	t.Run("should_handle_insert_error", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec("^INSERT INTO events").WillReturnError(sql.ErrConnDone)
		mock.ExpectRollback()

		processLine(context.Background(), db, logger, 1, 2, `{"message":"test"}`, "auto")
	})

	t.Run("should_handle_update_error", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec("^INSERT INTO events").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("^UPDATE raw_logs SET processed = true").WillReturnError(sql.ErrConnDone)
		mock.ExpectRollback()

		processLine(context.Background(), db, logger, 1, 2, `{"message":"test"}`, "auto")
	})

	t.Run("should_handle_commit_error", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectExec("^INSERT INTO events").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("^UPDATE raw_logs SET processed = true").WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit().WillReturnError(sql.ErrConnDone)

		processLine(context.Background(), db, logger, 1, 2, `{"message":"test"}`, "auto")
	})
}
