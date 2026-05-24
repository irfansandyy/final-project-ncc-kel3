package parser

import (
	"fmt"
	"testing"
	"time"
)

// mockPlugin implements the Plugin interface for testing.
type mockPlugin struct {
	name     string
	canParse func(string) bool
	parse    func(string) (*ParsedEvent, error)
}

func (m *mockPlugin) Name() string                       { return m.name }
func (m *mockPlugin) CanParse(line string) bool           { return m.canParse(line) }
func (m *mockPlugin) Parse(line string) (*ParsedEvent, error) { return m.parse(line) }

// resetRegistry clears the package-level registry for test isolation.
func resetRegistry() {
	registry = nil
}

// --- Register / GetPlugins ---

func Test_Should_ReturnEmptySlice_When_NoPluginsRegistered(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	plugins := GetPlugins()
	if plugins == nil {
		// nil is acceptable when nothing registered, but check length
		if len(plugins) != 0 {
			t.Errorf("expected 0 plugins, got %d", len(plugins))
		}
		return
	}
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(plugins))
	}
}

func Test_Should_RegisterSinglePlugin_When_CalledOnce(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	p := &mockPlugin{name: "test-plugin", canParse: func(string) bool { return false }}
	Register(p)

	plugins := GetPlugins()
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	if plugins[0].Name() != "test-plugin" {
		t.Errorf("expected plugin name 'test-plugin', got %q", plugins[0].Name())
	}
}

func Test_Should_RegisterMultiplePlugins_When_CalledMultipleTimes(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	for i := 0; i < 3; i++ {
		Register(&mockPlugin{
			name:     fmt.Sprintf("plugin-%d", i),
			canParse: func(string) bool { return false },
		})
	}

	plugins := GetPlugins()
	if len(plugins) != 3 {
		t.Fatalf("expected 3 plugins, got %d", len(plugins))
	}
	for i, p := range plugins {
		expected := fmt.Sprintf("plugin-%d", i)
		if p.Name() != expected {
			t.Errorf("plugin[%d]: expected name %q, got %q", i, expected, p.Name())
		}
	}
}

func Test_Should_ReturnSameSlice_When_GetPluginsCalledTwice(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	Register(&mockPlugin{name: "p1", canParse: func(string) bool { return false }})

	first := GetPlugins()
	second := GetPlugins()
	if len(first) != len(second) {
		t.Errorf("lengths differ: %d vs %d", len(first), len(second))
	}
}

// --- AutoDetect ---

func Test_Should_ReturnNil_When_NoPluginsCanParse(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	Register(&mockPlugin{name: "nope", canParse: func(string) bool { return false }})

	result := AutoDetect("some log line")
	if result != nil {
		t.Errorf("expected nil, got plugin %q", result.Name())
	}
}

func Test_Should_ReturnNil_When_RegistryEmpty(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	result := AutoDetect("any line")
	if result != nil {
		t.Errorf("expected nil for empty registry, got %q", result.Name())
	}
}

func Test_Should_ReturnFirstMatch_When_MultiplePluginsCanParse(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	Register(&mockPlugin{name: "first", canParse: func(string) bool { return true }})
	Register(&mockPlugin{name: "second", canParse: func(string) bool { return true }})

	result := AutoDetect("line")
	if result == nil {
		t.Fatal("expected a plugin, got nil")
	}
	if result.Name() != "first" {
		t.Errorf("expected 'first', got %q", result.Name())
	}
}

func Test_Should_ReturnSecondPlugin_When_FirstCannotParse(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	Register(&mockPlugin{name: "nope", canParse: func(string) bool { return false }})
	Register(&mockPlugin{name: "match", canParse: func(line string) bool { return line == "special" }})

	result := AutoDetect("special")
	if result == nil {
		t.Fatal("expected a plugin, got nil")
	}
	if result.Name() != "match" {
		t.Errorf("expected 'match', got %q", result.Name())
	}
}

func Test_Should_ReturnMatchingPlugin_When_CanParseReturnsTrue(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	expected := &ParsedEvent{
		Timestamp: time.Now(),
		Level:     "INFO",
		Source:    "test",
		Message:   "hello",
		Raw:       "raw line",
	}
	Register(&mockPlugin{
		name:     "json-parser",
		canParse: func(line string) bool { return line == `{"msg":"hello"}` },
		parse:    func(string) (*ParsedEvent, error) { return expected, nil },
	})

	p := AutoDetect(`{"msg":"hello"}`)
	if p == nil {
		t.Fatal("expected plugin, got nil")
	}
	if p.Name() != "json-parser" {
		t.Errorf("expected 'json-parser', got %q", p.Name())
	}

	// Verify the matched plugin can actually parse
	evt, err := p.Parse(`{"msg":"hello"}`)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if evt.Message != "hello" {
		t.Errorf("expected message 'hello', got %q", evt.Message)
	}
}

func Test_Should_ReturnNil_When_EmptyLine(t *testing.T) {
	resetRegistry()
	defer resetRegistry()

	Register(&mockPlugin{name: "strict", canParse: func(line string) bool { return len(line) > 5 }})

	result := AutoDetect("")
	if result != nil {
		t.Errorf("expected nil for empty line, got %q", result.Name())
	}
}

// --- NormalizeLevel ---

func Test_Should_ReturnCRITICAL_When_CriticalFamilyLevels(t *testing.T) {
	criticals := []string{
		"EMERG", "EMERGENCY", "PANIC", "FATAL", "CRIT", "CRITICAL", "ALERT",
	}
	for _, level := range criticals {
		t.Run(level, func(t *testing.T) {
			result := NormalizeLevel(level)
			if result != "CRITICAL" {
				t.Errorf("NormalizeLevel(%q) = %q, want CRITICAL", level, result)
			}
		})
	}
}

func Test_Should_ReturnERROR_When_ErrorFamilyLevels(t *testing.T) {
	errors := []string{"ERR", "ERROR"}
	for _, level := range errors {
		t.Run(level, func(t *testing.T) {
			result := NormalizeLevel(level)
			if result != "ERROR" {
				t.Errorf("NormalizeLevel(%q) = %q, want ERROR", level, result)
			}
		})
	}
}

func Test_Should_ReturnWARN_When_WarnFamilyLevels(t *testing.T) {
	warns := []string{"WARN", "WARNING"}
	for _, level := range warns {
		t.Run(level, func(t *testing.T) {
			result := NormalizeLevel(level)
			if result != "WARN" {
				t.Errorf("NormalizeLevel(%q) = %q, want WARN", level, result)
			}
		})
	}
}

func Test_Should_ReturnINFO_When_InfoFamilyLevels(t *testing.T) {
	infos := []string{"NOTICE", "INFO", "INFORMATIONAL"}
	for _, level := range infos {
		t.Run(level, func(t *testing.T) {
			result := NormalizeLevel(level)
			if result != "INFO" {
				t.Errorf("NormalizeLevel(%q) = %q, want INFO", level, result)
			}
		})
	}
}

func Test_Should_ReturnINFO_When_DebugOrTrace(t *testing.T) {
	debugs := []string{"DEBUG", "TRACE"}
	for _, level := range debugs {
		t.Run(level, func(t *testing.T) {
			result := NormalizeLevel(level)
			if result != "INFO" {
				t.Errorf("NormalizeLevel(%q) = %q, want INFO", level, result)
			}
		})
	}
}

func Test_Should_ReturnINFO_When_UnknownLevel(t *testing.T) {
	unknowns := []string{"VERBOSE", "SILLY", "FINE", "ALL", "OFF", "UNKNOWN", "foo", "123"}
	for _, level := range unknowns {
		t.Run(level, func(t *testing.T) {
			result := NormalizeLevel(level)
			if result != "INFO" {
				t.Errorf("NormalizeLevel(%q) = %q, want INFO", level, result)
			}
		})
	}
}

func Test_Should_ReturnINFO_When_EmptyString(t *testing.T) {
	result := NormalizeLevel("")
	if result != "INFO" {
		t.Errorf("NormalizeLevel(\"\") = %q, want INFO", result)
	}
}

func Test_Should_BeCaseInsensitive_When_LowerAndMixedCase(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"error", "ERROR"},
		{"Error", "ERROR"},
		{"eRrOr", "ERROR"},
		{"warn", "WARN"},
		{"Warning", "WARN"},
		{"critical", "CRITICAL"},
		{"fatal", "CRITICAL"},
		{"info", "INFO"},
		{"debug", "INFO"},
		{"trace", "INFO"},
		{"notice", "INFO"},
		{"panic", "CRITICAL"},
		{"emerg", "CRITICAL"},
		{"alert", "CRITICAL"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			result := NormalizeLevel(tc.input)
			if result != tc.expected {
				t.Errorf("NormalizeLevel(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func Test_Should_TrimWhitespace_When_InputHasSpaces(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{" ERROR ", "ERROR"},
		{"  WARN  ", "WARN"},
		{"\tCRITICAL\t", "CRITICAL"},
		{"\n INFO \n", "INFO"},
		{"  debug  ", "INFO"},
		{" ", "INFO"},       // whitespace-only -> empty -> default
		{"\t\n", "INFO"},   // whitespace-only -> default
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%q", tc.input), func(t *testing.T) {
			result := NormalizeLevel(tc.input)
			if result != tc.expected {
				t.Errorf("NormalizeLevel(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}
