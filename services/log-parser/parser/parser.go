package parser

import (
	"strings"
	"time"
)

// ParsedEvent is the normalized output from any parser plugin.
type ParsedEvent struct {
	Timestamp time.Time
	Level     string // INFO, WARN, ERROR, CRITICAL
	Source    string // hostname / app name
	Message   string
	Raw       string
	Metadata  map[string]interface{}
}

// Plugin interface that all parser plugins implement.
type Plugin interface {
	Name() string
	Parse(line string) (*ParsedEvent, error)
	CanParse(line string) bool
}

// Registry holds all registered parser plugins.
var registry []Plugin

// Register adds a plugin to the registry.
func Register(p Plugin) {
	registry = append(registry, p)
}

// GetPlugins returns all registered plugins.
func GetPlugins() []Plugin {
	return registry
}

// AutoDetect tries each plugin's CanParse and returns the first match.
func AutoDetect(line string) Plugin {
	for _, p := range registry {
		if p.CanParse(line) {
			return p
		}
	}
	return nil
}

// NormalizeLevel maps common level strings to our standard set.
func NormalizeLevel(level string) string {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "EMERG", "EMERGENCY", "PANIC", "FATAL", "CRIT", "CRITICAL", "ALERT":
		return "CRITICAL"
	case "ERR", "ERROR":
		return "ERROR"
	case "WARN", "WARNING":
		return "WARN"
	case "NOTICE", "INFO", "INFORMATIONAL":
		return "INFO"
	case "DEBUG", "TRACE":
		return "INFO"
	default:
		return "INFO"
	}
}
