package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/fpncc/log-parser/parser"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	// 1. Setup logging
	logLevelStr := os.Getenv("LOG_LEVEL")
	var level slog.Level
	if err := level.UnmarshalText([]byte(logLevelStr)); err != nil {
		level = slog.LevelInfo
	}
	
	serviceName := os.Getenv("SERVICE_NAME")
	if serviceName == "" {
		serviceName = "log-parser"
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})).With("service", serviceName)
	slog.SetDefault(logger)

	// 2. Load Config
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		logger.Error("POSTGRES_DSN environment variable required")
		os.Exit(1)
	}

	pollIntervalMs := 500
	if val := os.Getenv("POLL_INTERVAL_MS"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			pollIntervalMs = parsed
		}
	}
	pollInterval := time.Duration(pollIntervalMs) * time.Millisecond

	// 3. Connect to DB — retry for up to 60 s so we tolerate slow schema init
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		logger.Error("failed to open db connection", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	const maxDBRetries = 12
	const dbRetryDelay = 5 * time.Second
	for attempt := 1; attempt <= maxDBRetries; attempt++ {
		pingErr := db.Ping()
		if pingErr == nil {
			logger.Info("connected to database", "attempt", attempt)
			break
		}
		if attempt == maxDBRetries {
			logger.Error("failed to connect to database after retries",
				"error", pingErr,
				"attempts", maxDBRetries,
			)
			os.Exit(1)
		}
		logger.Warn("database not ready, retrying",
			"error", pingErr,
			"attempt", attempt,
			"retry_in", dbRetryDelay,
		)
		time.Sleep(dbRetryDelay)
	}

	// Write health sentinel so Docker HEALTHCHECK can verify steady state.
	if err := os.WriteFile("/tmp/healthy", []byte("ok"), 0644); err != nil {
		logger.Warn("failed to write health file", "error", err)
	}

	// 4. Start polling loop
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		logger.Info("shutting down")
		cancel()
	}()

	logger.Info("starting log parser loop", "poll_interval_ms", pollIntervalMs)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("parser loop stopped")
			return
		case <-ticker.C:
			processLogs(ctx, db, logger)
		}
	}
}

func processLogs(ctx context.Context, db *sql.DB, logger *slog.Logger) {
	// Find up to 100 unprocessed logs
	query := `
		SELECT r.id, r.source_id, r.line, ls.format 
		FROM raw_logs r 
		JOIN log_sources ls ON r.source_id = ls.id 
		WHERE r.processed = false 
		ORDER BY r.id 
		LIMIT 100
	`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		logger.Error("failed to query raw_logs", "error", err)
		return
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		var id, sourceID int64
		var line, format string
		if err := rows.Scan(&id, &sourceID, &line, &format); err != nil {
			logger.Error("failed to scan row", "error", err)
			continue
		}

		// Process line
		processLine(ctx, db, logger, id, sourceID, line, format)
		count++
	}

	if err := rows.Err(); err != nil {
		logger.Error("row iteration error", "error", err)
	}

	if count > 0 {
		logger.Debug("processed batch", "count", count)
	}
}

func processLine(ctx context.Context, db *sql.DB, logger *slog.Logger, id, sourceID int64, line, format string) {
	var p parser.Plugin

	// Find parser
	if format == "auto" {
		p = parser.AutoDetect(line)
	} else {
		for _, plugin := range parser.GetPlugins() {
			if plugin.Name() == format {
				p = plugin
				break
			}
		}
	}

	var parsed *parser.ParsedEvent
	var err error

	if p != nil {
		parsed, err = p.Parse(line)
		if err != nil {
			logger.Warn("parser failed", "error", err, "format", format, "raw_id", id)
			// Fallback to unparsed event
		}
	} else {
		logger.Warn("no suitable parser found", "format", format, "raw_id", id)
		// Fallback to unparsed event
	}

	if parsed == nil {
		parsed = &parser.ParsedEvent{
			Timestamp: time.Now(),
			Level:     "INFO",
			Source:    "unknown",
			Message:   line,
			Raw:       line,
			Metadata:  map[string]interface{}{"parse_error": "no parser matched or parse failed"},
		}
	}

	// Insert into events and mark raw_logs processed
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", "error", err)
		return
	}
	defer tx.Rollback()

	metadataBytes, _ := json.Marshal(parsed.Metadata)

	insertQuery := `
		INSERT INTO events (source_id, timestamp, level, source, message, raw, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err = tx.ExecContext(ctx, insertQuery, sourceID, parsed.Timestamp, parsed.Level, parsed.Source, parsed.Message, parsed.Raw, metadataBytes)
	if err != nil {
		logger.Error("failed to insert event", "error", err)
		return
	}

	updateQuery := `UPDATE raw_logs SET processed = true WHERE id = $1`
	_, err = tx.ExecContext(ctx, updateQuery, id)
	if err != nil {
		logger.Error("failed to mark raw_log processed", "error", err)
		return
	}

	if err := tx.Commit(); err != nil {
		logger.Error("failed to commit transaction", "error", err)
		return
	}
	
	logger.Debug("event processed", "raw_id", id, "event_level", parsed.Level, "event_source", parsed.Source)
}
