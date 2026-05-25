// File: services/log-parser/main_test.go
package main_test

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Stub types – adjust to match your real log-parser structs/functions.
// ---------------------------------------------------------------------------

type ParsedEvent struct {
	Timestamp  time.Time
	EventType  string
	SourceIP   string
	StatusCode int
	RawMessage string
}

func parseNginxAccessLog(line string) (*ParsedEvent, error) {
	if line == "" {
		return nil, fmt.Errorf("empty log line")
	}
	parts := strings.Fields(line)
	if len(parts) < 7 {
		return nil, fmt.Errorf("too few fields in nginx log: %q", line)
	}
	sourceIP := parts[0]
	statusCode := 0
	if _, err := fmt.Sscanf(parts[6], "%d", &statusCode); err != nil {
		return nil, fmt.Errorf("could not parse status code: %q", parts[6])
	}
	return &ParsedEvent{
		Timestamp:  time.Now(),
		EventType:  "http_access",
		SourceIP:   sourceIP,
		StatusCode: statusCode,
		RawMessage: line,
	}, nil
}

func classifyEvent(statusCode int) string {
	switch {
	case statusCode >= 500:
		return "error"
	case statusCode >= 400:
		return "warning"
	case statusCode >= 200 && statusCode < 300:
		return "info"
	default:
		return "unknown"
	}
}

func isPrivateIP(ip string) bool {
	return strings.HasPrefix(ip, "10.") ||
		strings.HasPrefix(ip, "192.168.") ||
		strings.HasPrefix(ip, "172.")
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestParseNginxAccessLog_ValidLine(t *testing.T) {
	line := `192.168.1.1 - - [15/Jan/2024:10:30:00 +0000] "GET /api/health HTTP/1.1" 200 512`
	event, err := parseNginxAccessLog(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.SourceIP != "192.168.1.1" {
		t.Errorf("expected source IP 192.168.1.1, got %q", event.SourceIP)
	}
	if event.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", event.StatusCode)
	}
	if event.EventType != "http_access" {
		t.Errorf("expected event type http_access, got %q", event.EventType)
	}
}

func TestParseNginxAccessLog_EmptyLine_ReturnsError(t *testing.T) {
	_, err := parseNginxAccessLog("")
	if err == nil {
		t.Error("expected error for empty line")
	}
}

func TestParseNginxAccessLog_TooFewFields_ReturnsError(t *testing.T) {
	_, err := parseNginxAccessLog("192.168.1.1 - -")
	if err == nil {
		t.Error("expected error for too few fields")
	}
}

func TestParseNginxAccessLog_PreservesRawMessage(t *testing.T) {
	line := `10.0.0.1 - - [15/Jan/2024:10:30:00 +0000] "POST /api/login HTTP/1.1" 401 128`
	event, _ := parseNginxAccessLog(line)
	if event.RawMessage != line {
		t.Errorf("RawMessage not preserved")
	}
}

func TestClassifyEvent_5xx_IsError(t *testing.T) {
	for _, code := range []int{500, 502, 503} {
		if classifyEvent(code) != "error" {
			t.Errorf("status %d should be 'error'", code)
		}
	}
}

func TestClassifyEvent_4xx_IsWarning(t *testing.T) {
	for _, code := range []int{400, 401, 403, 404} {
		if classifyEvent(code) != "warning" {
			t.Errorf("status %d should be 'warning'", code)
		}
	}
}

func TestClassifyEvent_2xx_IsInfo(t *testing.T) {
	for _, code := range []int{200, 201, 204} {
		if classifyEvent(code) != "info" {
			t.Errorf("status %d should be 'info'", code)
		}
	}
}

func TestClassifyEvent_Unknown(t *testing.T) {
	if classifyEvent(0) != "unknown" {
		t.Error("status 0 should be 'unknown'")
	}
}

func TestIsPrivateIP_10Block(t *testing.T) {
	if !isPrivateIP("10.0.0.1") {
		t.Error("10.0.0.1 should be private")
	}
}

func TestIsPrivateIP_192168Block(t *testing.T) {
	if !isPrivateIP("192.168.1.100") {
		t.Error("192.168.1.100 should be private")
	}
}

func TestIsPrivateIP_PublicIP_IsFalse(t *testing.T) {
	if isPrivateIP("8.8.8.8") {
		t.Error("8.8.8.8 should not be private")
	}
}
