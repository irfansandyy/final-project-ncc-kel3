package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fpncc/log-collector/collector"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// logEntry represents a single log line to be inserted into raw_logs.
type logEntry struct {
	sourceID int64
	line     string
}

func main() {
	// ── Configuration ──────────────────────────────────────────────────
	serviceName := envOrDefault("SERVICE_NAME", "log-collector")
	pgDSN := os.Getenv("POSTGRES_DSN")
	logLevel := envOrDefault("LOG_LEVEL", "info")
	reloadIntervalStr := envOrDefault("RELOAD_INTERVAL_SECONDS", "30")

	reloadInterval, err := strconv.Atoi(reloadIntervalStr)
	if err != nil {
		reloadInterval = 30
	}

	// ── Logger ─────────────────────────────────────────────────────────
	var level slog.Level
	switch strings.ToLower(logLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})).With(slog.String("service", serviceName))

	slog.SetDefault(logger)

	logger.Info("starting log-collector",
		slog.String("log_level", logLevel),
		slog.Int("reload_interval_seconds", reloadInterval),
	)

	if pgDSN == "" {
		logger.Error("POSTGRES_DSN environment variable is required")
		os.Exit(1)
	}

	// ── Database — retry until DB is ready ─────────────────────────────
	// The container starts immediately after db reports healthy, but
	// PostgreSQL may still be initialising schemas. Retry for up to 60s.
	db, err := sql.Open("pgx", pgDSN)
	if err != nil {
		logger.Error("failed to open database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	const maxDBRetries = 12
	const dbRetryDelay = 5 * time.Second

	for attempt := 1; attempt <= maxDBRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		pingErr := db.PingContext(ctx)
		cancel()
		if pingErr == nil {
			logger.Info("connected to database", slog.Int("attempt", attempt))
			break
		}
		if attempt == maxDBRetries {
			logger.Error("failed to connect to database after retries",
				slog.String("error", pingErr.Error()),
				slog.Int("attempts", maxDBRetries),
			)
			os.Exit(1)
		}
		logger.Warn("database not ready, retrying",
			slog.String("error", pingErr.Error()),
			slog.Int("attempt", attempt),
			slog.Duration("retry_in", dbRetryDelay),
		)
		time.Sleep(dbRetryDelay)
	}

	// ── Batch insert channel ───────────────────────────────────────────
	entryCh := make(chan logEntry, 1000)

	handler := func(sourceID int64, line string) {
		select {
		case entryCh <- logEntry{sourceID: sourceID, line: line}:
		default:
			logger.Warn("entry channel full, dropping line",
				slog.Int64("source_id", sourceID),
			)
		}
	}

	// ── Watcher ────────────────────────────────────────────────────────
	w, err := collector.NewWatcher(handler, logger)
	if err != nil {
		logger.Error("failed to create watcher", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Load initial sources — warn on failure instead of exiting.
	// Sources may point to files that don't exist yet (e.g. backend.log
	// is only created once the backend service writes its first line).
	knownSources := make(map[int64]bool)
	if err := loadSources(db, w, knownSources, logger); err != nil {
		logger.Warn("failed to load initial sources (will retry on next reload interval)",
			slog.String("error", err.Error()),
		)
	}

	go w.Start()

	// ── Batch insert goroutine ─────────────────────────────────────────
	var batchWg sync.WaitGroup
	batchCtx, batchCancel := context.WithCancel(context.Background())

	batchWg.Add(1)
	go func() {
		defer batchWg.Done()
		runBatchInserter(batchCtx, db, entryCh, logger)
	}()

	// ── Hot-reload goroutine ───────────────────────────────────────────
	var reloadWg sync.WaitGroup
	reloadCtx, reloadCancel := context.WithCancel(context.Background())

	reloadWg.Add(1)
	go func() {
		defer reloadWg.Done()
		ticker := time.NewTicker(time.Duration(reloadInterval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-reloadCtx.Done():
				return
			case <-ticker.C:
				if err := loadSources(db, w, knownSources, logger); err != nil {
					logger.Error("failed to reload sources", slog.String("error", err.Error()))
				}
			}
		}
	}()

	// ── Health file — write a sentinel so the healthcheck can verify ──
	// Writing /tmp/healthy lets the Docker HEALTHCHECK confirm the process
	// reached steady state without needing curl/wget in the image.
	if err := os.WriteFile("/tmp/healthy", []byte("ok"), 0644); err != nil {
		logger.Warn("failed to write health file", slog.String("error", err.Error()))
	}

	logger.Info("log-collector running")

	// ── Graceful shutdown ──────────────────────────────────────────────
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	logger.Info("received shutdown signal", slog.String("signal", sig.String()))

	// 1. Stop the hot-reload goroutine.
	reloadCancel()
	reloadWg.Wait()

	// 2. Stop the file watcher (no more lines will be produced).
	w.Stop()

	// 3. Close the entry channel so the batch inserter drains and exits.
	close(entryCh)
	batchCancel()
	batchWg.Wait()

	// 4. Remove health sentinel on clean shutdown.
	_ = os.Remove("/tmp/healthy")

	logger.Info("log-collector shut down gracefully")
}

// loadSources queries log_sources for enabled entries and adds any new ones
// to the watcher. knownSources tracks IDs already added to avoid duplicates.
// Files that don't exist yet are skipped with a warning (not a fatal error).
func loadSources(db *sql.DB, w *collector.Watcher, known map[int64]bool, logger *slog.Logger) error {
	rows, err := db.Query("SELECT id, file_path FROM log_sources WHERE enabled = true")
	if err != nil {
		return fmt.Errorf("query log_sources: %w", err)
	}
	defer rows.Close()

	added := 0
	for rows.Next() {
		var id int64
		var filePath string
		if err := rows.Scan(&id, &filePath); err != nil {
			return fmt.Errorf("scan row: %w", err)
		}

		if known[id] {
			continue
		}

		if err := w.AddPath(id, filePath); err != nil {
			// Log as warn — the file may not exist yet (e.g. backend.log
			// before the backend has started writing). It will be retried
			// on the next hot-reload tick.
			logger.Warn("failed to add source (will retry)",
				slog.Int64("source_id", id),
				slog.String("path", filePath),
				slog.String("error", err.Error()),
			)
			continue
		}

		known[id] = true
		added++
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows iteration: %w", err)
	}

	if added > 0 {
		logger.Info("loaded new sources",
			slog.Int("count", added),
			slog.Int("total", len(known)),
		)
	}

	return nil
}

// runBatchInserter collects log entries from the channel and batch-inserts them
// into raw_logs. It flushes when the batch reaches 50 entries or every 100ms,
// whichever comes first.
func runBatchInserter(ctx context.Context, db *sql.DB, entries <-chan logEntry, logger *slog.Logger) {
	const (
		maxBatchSize  = 50
		flushInterval = 100 * time.Millisecond
	)

	batch := make([]logEntry, 0, maxBatchSize)
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}

		if err := insertBatch(ctx, db, batch); err != nil {
			logger.Error("batch insert failed",
				slog.String("error", err.Error()),
				slog.Int("batch_size", len(batch)),
			)
		} else {
			logger.Debug("batch inserted",
				slog.Int("batch_size", len(batch)),
			)
		}

		batch = batch[:0]
	}

	for {
		select {
		case entry, ok := <-entries:
			if !ok {
				// Channel closed — flush remaining and exit.
				flush()
				return
			}
			batch = append(batch, entry)
			if len(batch) >= maxBatchSize {
				flush()
			}

		case <-ticker.C:
			flush()

		case <-ctx.Done():
			// Drain any remaining entries in the channel.
			for entry := range entries {
				batch = append(batch, entry)
				if len(batch) >= maxBatchSize {
					flush()
				}
			}
			flush()
			return
		}
	}
}

// insertBatch performs a single multi-row INSERT into raw_logs.
func insertBatch(ctx context.Context, db *sql.DB, batch []logEntry) error {
	if len(batch) == 0 {
		return nil
	}

	// Build a multi-row INSERT: INSERT INTO raw_logs(source_id, line) VALUES ($1,$2),($3,$4),...
	var sb strings.Builder
	sb.WriteString("INSERT INTO raw_logs(source_id, line) VALUES ")

	args := make([]interface{}, 0, len(batch)*2)
	for i, entry := range batch {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf("($%d,$%d)", i*2+1, i*2+2))
		args = append(args, entry.sourceID, entry.line)
	}

	insertCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := db.ExecContext(insertCtx, sb.String(), args...)
	return err
}

// envOrDefault returns the value of the environment variable named by key,
// or defaultVal if the variable is not set or empty.
func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
