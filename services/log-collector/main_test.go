// File: services/log-collector/main_test.go
package main_test

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Stub types – replace with real imports from your package once exportable.
// ---------------------------------------------------------------------------

type LogEntry struct {
	Timestamp time.Time
	Level     string
	Source    string
	Message   string
	RawLine   string
}

func parseLogLine(raw string) (*LogEntry, error) {
	parts := strings.SplitN(raw, " ", 4)
	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid log line: %q", raw)
	}
	ts, err := time.Parse(time.RFC3339, parts[0])
	if err != nil {
		return nil, fmt.Errorf("bad timestamp: %w", err)
	}
	return &LogEntry{
		Timestamp: ts,
		Level:     parts[1],
		Source:    parts[2],
		Message:   parts[3],
		RawLine:   raw,
	}, nil
}

func isHighSeverity(level string) bool {
	switch strings.ToUpper(level) {
	case "CRITICAL", "ERROR":
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestParseLogLine_ValidLine(t *testing.T) {
	raw := "2024-01-15T10:30:00Z ERROR nginx Request timeout"
	entry, err := parseLogLine(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Level != "ERROR" {
		t.Errorf("expected level ERROR, got %q", entry.Level)
	}
	if entry.Source != "nginx" {
		t.Errorf("expected source nginx, got %q", entry.Source)
	}
	if entry.Message != "Request timeout" {
		t.Errorf("unexpected message: %q", entry.Message)
	}
}

func TestParseLogLine_InvalidFormat_ReturnsError(t *testing.T) {
	_, err := parseLogLine("not-enough-parts")
	if err == nil {
		t.Error("expected error for malformed line, got nil")
	}
}

func TestParseLogLine_BadTimestamp_ReturnsError(t *testing.T) {
	_, err := parseLogLine("not-a-time ERROR svc msg")
	if err == nil {
		t.Error("expected error for bad timestamp, got nil")
	}
}

func TestParseLogLine_PreservesRawLine(t *testing.T) {
	raw := "2024-01-15T10:30:00Z INFO app Starting up"
	entry, _ := parseLogLine(raw)
	if entry.RawLine != raw {
		t.Errorf("RawLine not preserved: got %q", entry.RawLine)
	}
}

func TestIsHighSeverity_Critical(t *testing.T) {
	if !isHighSeverity("CRITICAL") {
		t.Error("CRITICAL should be high severity")
	}
}

func TestIsHighSeverity_Error(t *testing.T) {
	if !isHighSeverity("ERROR") {
		t.Error("ERROR should be high severity")
	}
}

func TestIsHighSeverity_Info_IsFalse(t *testing.T) {
	if isHighSeverity("INFO") {
		t.Error("INFO should not be high severity")
	}
}

func TestIsHighSeverity_Warning_IsFalse(t *testing.T) {
	if isHighSeverity("WARNING") {
		t.Error("WARNING should not be high severity")
	}
}

func TestIsHighSeverity_CaseInsensitive(t *testing.T) {
	if !isHighSeverity("critical") {
		t.Error("lowercase 'critical' should still be high severity")
	}
	if !isHighSeverity("error") {
		t.Error("lowercase 'error' should still be high severity")
	}
}

func TestParseLogLine_EmptyString_ReturnsError(t *testing.T) {
	_, err := parseLogLine("")
	if err == nil {
		t.Error("expected error for empty string")
	}
}
