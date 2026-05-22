package collector

import (
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// waitForLines blocks until the expected number of lines have been collected,
// or until the timeout expires.
func waitForLines(t *testing.T, mu *sync.Mutex, collected *[]string, expected int, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			mu.Lock()
			got := len(*collected)
			mu.Unlock()
			t.Fatalf("timed out waiting for %d lines, got %d", expected, got)
			return
		case <-time.After(50 * time.Millisecond):
			mu.Lock()
			got := len(*collected)
			mu.Unlock()
			if got >= expected {
				return
			}
		}
	}
}

func TestWatcher_NewLines(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	// Create the file first so AddPath can open it.
	f, err := os.Create(logFile)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}

	var mu sync.Mutex
	var collected []string

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	w, err := NewWatcher(func(sourceID int64, line string) {
		mu.Lock()
		collected = append(collected, line)
		mu.Unlock()
	}, logger)
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}

	if err := w.AddPath(1, logFile); err != nil {
		t.Fatalf("add path: %v", err)
	}

	go w.Start()
	defer w.Stop()

	// Give the watcher a moment to initialise.
	time.Sleep(200 * time.Millisecond)

	// Write lines to the file.
	lines := []string{"line one", "line two", "line three"}
	for _, l := range lines {
		if _, err := f.WriteString(l + "\n"); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if err := f.Sync(); err != nil {
		t.Fatalf("sync: %v", err)
	}

	waitForLines(t, &mu, &collected, 3, 5*time.Second)

	mu.Lock()
	defer mu.Unlock()

	for i, want := range lines {
		if i >= len(collected) {
			t.Fatalf("missing line %d: want %q", i, want)
		}
		if collected[i] != want {
			t.Errorf("line %d: got %q, want %q", i, collected[i], want)
		}
	}
}

func TestWatcher_FileRotation(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "rotate.log")

	f, err := os.Create(logFile)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}

	var mu sync.Mutex
	var collected []string

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	w, err := NewWatcher(func(sourceID int64, line string) {
		mu.Lock()
		collected = append(collected, line)
		mu.Unlock()
	}, logger)
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}

	if err := w.AddPath(2, logFile); err != nil {
		t.Fatalf("add path: %v", err)
	}

	go w.Start()
	defer w.Stop()

	time.Sleep(200 * time.Millisecond)

	// Write some initial lines.
	if _, err := f.WriteString("before rotation\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = f.Sync()

	waitForLines(t, &mu, &collected, 1, 5*time.Second)

	// Simulate rotation by truncating the file.
	if err := f.Truncate(0); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatalf("seek: %v", err)
	}

	// Write new content after truncation.
	if _, err := f.WriteString("after rotation\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = f.Sync()

	waitForLines(t, &mu, &collected, 2, 5*time.Second)

	mu.Lock()
	defer mu.Unlock()

	if len(collected) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(collected))
	}
	if collected[0] != "before rotation" {
		t.Errorf("line 0: got %q, want %q", collected[0], "before rotation")
	}
	if collected[1] != "after rotation" {
		t.Errorf("line 1: got %q, want %q", collected[1], "after rotation")
	}
}

func TestWatcher_AddPathDynamic(t *testing.T) {
	dir := t.TempDir()
	logFile1 := filepath.Join(dir, "first.log")
	logFile2 := filepath.Join(dir, "second.log")

	f1, err := os.Create(logFile1)
	if err != nil {
		t.Fatalf("create file1: %v", err)
	}
	f2, err := os.Create(logFile2)
	if err != nil {
		t.Fatalf("create file2: %v", err)
	}

	var mu sync.Mutex
	sourceLines := make(map[int64][]string)

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	w, err := NewWatcher(func(sourceID int64, line string) {
		mu.Lock()
		sourceLines[sourceID] = append(sourceLines[sourceID], line)
		mu.Unlock()
	}, logger)
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}

	// Add only the first file initially.
	if err := w.AddPath(10, logFile1); err != nil {
		t.Fatalf("add path 1: %v", err)
	}

	go w.Start()
	defer w.Stop()

	time.Sleep(200 * time.Millisecond)

	// Write to file 1.
	if _, err := f1.WriteString("from first\n"); err != nil {
		t.Fatalf("write f1: %v", err)
	}
	_ = f1.Sync()

	// Dynamically add the second file.
	if err := w.AddPath(20, logFile2); err != nil {
		t.Fatalf("add path 2: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	// Write to file 2.
	if _, err := f2.WriteString("from second\n"); err != nil {
		t.Fatalf("write f2: %v", err)
	}
	_ = f2.Sync()

	// Wait for lines from both sources.
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			mu.Lock()
			t.Fatalf("timed out: source lines = %v", sourceLines)
			mu.Unlock()
			return
		case <-time.After(50 * time.Millisecond):
			mu.Lock()
			got1 := len(sourceLines[10])
			got2 := len(sourceLines[20])
			mu.Unlock()
			if got1 >= 1 && got2 >= 1 {
				goto done
			}
		}
	}
done:

	mu.Lock()
	defer mu.Unlock()

	if sourceLines[10][0] != "from first" {
		t.Errorf("source 10: got %q, want %q", sourceLines[10][0], "from first")
	}
	if sourceLines[20][0] != "from second" {
		t.Errorf("source 20: got %q, want %q", sourceLines[20][0], "from second")
	}
}

func TestWatcher_GracefulShutdown(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "shutdown.log")

	if _, err := os.Create(logFile); err != nil {
		t.Fatalf("create file: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	w, err := NewWatcher(func(sourceID int64, line string) {}, logger)
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}

	if err := w.AddPath(99, logFile); err != nil {
		t.Fatalf("add path: %v", err)
	}

	done := make(chan struct{})
	go func() {
		w.Start()
		close(done)
	}()

	// Give the watcher time to start.
	time.Sleep(200 * time.Millisecond)

	// Stop should return promptly and cause Start() to exit.
	w.Stop()

	select {
	case <-done:
		// Success — Start() returned.
	case <-time.After(3 * time.Second):
		t.Fatal("Start() did not return after Stop()")
	}
}
