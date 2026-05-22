package parser

import (
	"encoding/json"
	"fmt"
	"time"
)

func init() {
	Register(&JSONParser{})
}

// JSONParser parses arbitrary JSON logs, extracting well-known fields.
type JSONParser struct{}

func (p *JSONParser) Name() string {
	return "json"
}

func (p *JSONParser) CanParse(line string) bool {
	if len(line) < 2 {
		return false
	}
	// Fast check for JSON object
	return line[0] == '{' && line[len(line)-1] == '}' && json.Valid([]byte(line))
}

func (p *JSONParser) Parse(line string) (*ParsedEvent, error) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(line), &data); err != nil {
		return nil, fmt.Errorf("invalid json: %v", err)
	}

	metadata := map[string]interface{}{
		"format": "json",
	}

	// Extract standard fields
	ts := extractTimestamp(data)
	level := extractLevel(data)
	msg := extractMessage(data)
	source := extractSource(data)

	// Keep remaining fields in metadata
	for k, v := range data {
		metadata[k] = v
	}

	return &ParsedEvent{
		Timestamp: ts,
		Level:     level,
		Source:    source,
		Message:   msg,
		Raw:       line,
		Metadata:  metadata,
	}, nil
}

func extractTimestamp(data map[string]interface{}) time.Time {
	keys := []string{"timestamp", "time", "ts", "@timestamp"}
	for _, key := range keys {
		if val, ok := data[key]; ok {
			delete(data, key)
			if strVal, ok := val.(string); ok {
				if t, err := time.Parse(time.RFC3339Nano, strVal); err == nil {
					return t
				}
				if t, err := time.Parse(time.RFC3339, strVal); err == nil {
					return t
				}
			}
		}
	}
	return time.Now()
}

func extractLevel(data map[string]interface{}) string {
	keys := []string{"level", "severity", "log_level", "logLevel"}
	for _, key := range keys {
		if val, ok := data[key]; ok {
			delete(data, key)
			if strVal, ok := val.(string); ok {
				return NormalizeLevel(strVal)
			}
		}
	}
	return "INFO" // default
}

func extractMessage(data map[string]interface{}) string {
	keys := []string{"message", "msg", "log"}
	for _, key := range keys {
		if val, ok := data[key]; ok {
			delete(data, key)
			if strVal, ok := val.(string); ok {
				return strVal
			}
		}
	}
	return "No message field found" // default
}

func extractSource(data map[string]interface{}) string {
	keys := []string{"source", "host", "hostname", "app", "service"}
	for _, key := range keys {
		if val, ok := data[key]; ok {
			delete(data, key)
			if strVal, ok := val.(string); ok {
				return strVal
			}
		}
	}
	return "unknown" // default
}
