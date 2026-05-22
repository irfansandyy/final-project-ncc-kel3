package collector

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// LineHandler is invoked for every new line read from a watched file.
type LineHandler func(sourceID int64, line string)

// fileState tracks the read offset for a single watched file.
type fileState struct {
	sourceID int64
	path     string
	offset   int64
}

// Watcher monitors a set of files for new lines using fsnotify.
type Watcher struct {
	handler LineHandler
	logger  *slog.Logger
	watcher *fsnotify.Watcher
	files   map[string]*fileState
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewWatcher creates a new Watcher. The handler is called for every new line
// read from any watched file. The logger is used for structured log output.
func NewWatcher(handler LineHandler, logger *slog.Logger) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Watcher{
		handler: handler,
		logger:  logger,
		watcher: fsw,
		files:   make(map[string]*fileState),
		ctx:     ctx,
		cancel:  cancel,
	}, nil
}

// AddPath registers a new file to watch. The file is opened, seeked to the
// end (so only future writes are captured), and added to the fsnotify watcher.
// It is safe to call AddPath while the watcher is running.
func (w *Watcher) AddPath(sourceID int64, path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, exists := w.files[path]; exists {
		w.logger.Info("path already watched, skipping",
			slog.String("path", path),
			slog.Int64("source_id", sourceID),
		)
		return nil
	}

	// Open the file to determine its current size (seek to end).
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	endOffset, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}

	w.files[path] = &fileState{
		sourceID: sourceID,
		path:     path,
		offset:   endOffset,
	}

	if err := w.watcher.Add(path); err != nil {
		delete(w.files, path)
		return err
	}

	w.logger.Info("watching file",
		slog.String("path", path),
		slog.Int64("source_id", sourceID),
		slog.Int64("offset", endOffset),
	)

	return nil
}

// Start begins the event loop that listens for fsnotify events and reads new
// lines from modified files. It blocks until Stop() is called or the context
// is cancelled. Typically called in a goroutine: go watcher.Start()
func (w *Watcher) Start() {
	w.wg.Add(1)
	defer w.wg.Done()

	w.logger.Info("watcher started")

	for {
		select {
		case <-w.ctx.Done():
			w.logger.Info("watcher stopping due to context cancellation")
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				w.logger.Info("watcher events channel closed")
				return
			}

			if event.Has(fsnotify.Write) {
				w.handleWriteEvent(event.Name)
			}

			// Handle file removal / rename (rotation via mv).
			if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
				w.handleRotation(event.Name)
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				w.logger.Info("watcher errors channel closed")
				return
			}
			w.logger.Error("fsnotify error", slog.String("error", err.Error()))
		}
	}
}

// Stop gracefully shuts down the watcher. It cancels the context, closes the
// fsnotify watcher, and waits for the event loop to exit.
func (w *Watcher) Stop() {
	w.logger.Info("stopping watcher")
	w.cancel()
	_ = w.watcher.Close()
	w.wg.Wait()
	w.logger.Info("watcher stopped")
}

// handleWriteEvent reads new lines from a file after a write event.
func (w *Watcher) handleWriteEvent(path string) {
	w.mu.RLock()
	state, exists := w.files[path]
	w.mu.RUnlock()

	if !exists {
		return
	}

	f, err := os.Open(path)
	if err != nil {
		w.logger.Error("failed to open file for reading",
			slog.String("path", path),
			slog.String("error", err.Error()),
		)
		return
	}
	defer f.Close()

	// Detect file truncation (rotation). If the file is now smaller than our
	// recorded offset, reset to the beginning.
	info, err := f.Stat()
	if err != nil {
		w.logger.Error("failed to stat file",
			slog.String("path", path),
			slog.String("error", err.Error()),
		)
		return
	}

	currentOffset := state.offset
	if info.Size() < currentOffset {
		w.logger.Warn("file truncated, resetting offset",
			slog.String("path", path),
			slog.Int64("old_offset", currentOffset),
			slog.Int64("new_size", info.Size()),
		)
		currentOffset = 0
	}

	// Seek to the last known offset.
	if _, err := f.Seek(currentOffset, io.SeekStart); err != nil {
		w.logger.Error("failed to seek file",
			slog.String("path", path),
			slog.String("error", err.Error()),
		)
		return
	}

	scanner := bufio.NewScanner(f)
	linesRead := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		w.handler(state.sourceID, line)
		linesRead++
	}

	if err := scanner.Err(); err != nil {
		w.logger.Error("scanner error",
			slog.String("path", path),
			slog.String("error", err.Error()),
		)
	}

	// Update offset to current position.
	newOffset, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		w.logger.Error("failed to get current offset",
			slog.String("path", path),
			slog.String("error", err.Error()),
		)
		return
	}

	w.mu.Lock()
	state.offset = newOffset
	w.mu.Unlock()

	if linesRead > 0 {
		w.logger.Debug("read new lines",
			slog.String("path", path),
			slog.Int("lines", linesRead),
			slog.Int64("offset", newOffset),
		)
	}
}

// handleRotation handles file removal or rename by attempting to re-watch the
// path after a short delay. This covers the common logrotate pattern where a
// file is moved away and a new file is created at the same path.
func (w *Watcher) handleRotation(path string) {
	w.mu.RLock()
	state, exists := w.files[path]
	w.mu.RUnlock()

	if !exists {
		return
	}

	w.logger.Info("file removed/renamed, attempting re-watch",
		slog.String("path", path),
		slog.Int64("source_id", state.sourceID),
	)

	// Try to re-add the file after a short delay — the new file may not exist
	// yet immediately after rotation.
	go func() {
		for attempts := 0; attempts < 10; attempts++ {
			select {
			case <-w.ctx.Done():
				return
			case <-time.After(500 * time.Millisecond):
			}

			if _, err := os.Stat(path); err != nil {
				continue
			}

			// File exists again. Reset offset and re-add to fsnotify.
			w.mu.Lock()
			state.offset = 0
			w.mu.Unlock()

			if err := w.watcher.Add(path); err != nil {
				w.logger.Error("failed to re-add file after rotation",
					slog.String("path", path),
					slog.String("error", err.Error()),
				)
				return
			}

			w.logger.Info("re-watching file after rotation",
				slog.String("path", path),
				slog.Int64("source_id", state.sourceID),
			)
			return
		}

		w.logger.Warn("gave up re-watching file after rotation",
			slog.String("path", path),
		)
	}()
}
