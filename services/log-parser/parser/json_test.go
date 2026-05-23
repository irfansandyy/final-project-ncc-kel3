package parser

import (
	"testing"
)

func TestJSONParser_Parse(t *testing.T) {
	p := &JSONParser{}

	tests := []struct {
		name         string
		line         string
		wantLevel    string
		wantSource   string
		wantMessage  string
		wantMetadata map[string]interface{}
		wantErr      bool
	}{
		{
			name:        "Standard JSON",
			line:        `{"timestamp":"2024-03-15T10:30:00Z","level":"INFO","message":"Application started","source":"myapp","version":"1.2.3"}`,
			wantLevel:   "INFO",
			wantSource:  "myapp",
			wantMessage: "Application started",
			wantMetadata: map[string]interface{}{
				"format":  "json",
				"version": "1.2.3",
			},
			wantErr: false,
		},
		{
			name:        "Alternative field names",
			line:        `{"ts":"2024-03-15T10:31:00Z","severity":"ERROR","msg":"Database connection failed","host":"db-server","retry_count":3}`,
			wantLevel:   "ERROR",
			wantSource:  "db-server",
			wantMessage: "Database connection failed",
			wantMetadata: map[string]interface{}{
				"format":      "json",
				"retry_count": float64(3), // json.Unmarshal uses float64 for numbers
			},
			wantErr: false,
		},
		{
			name:        "Minimal JSON",
			line:        `{"message":"Simple log entry"}`,
			wantLevel:   "INFO", // default
			wantSource:  "unknown", // default
			wantMessage: "Simple log entry",
			wantMetadata: map[string]interface{}{
				"format": "json",
			},
			wantErr: false,
		},
		{
			name:    "Invalid JSON",
			line:    `{"message":"Unclosed quote}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := p.Parse(tt.line)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Level != tt.wantLevel {
					t.Errorf("Parse() Level = %v, want %v", got.Level, tt.wantLevel)
				}
				if got.Source != tt.wantSource {
					t.Errorf("Parse() Source = %v, want %v", got.Source, tt.wantSource)
				}
				if got.Message != tt.wantMessage {
					t.Errorf("Parse() Message = %v, want %v", got.Message, tt.wantMessage)
				}
				for k, v := range tt.wantMetadata {
					if got.Metadata[k] != v {
						t.Errorf("Parse() Metadata[%s] = %v, want %v", k, got.Metadata[k], v)
					}
				}
			}
		})
	}
}

func TestJSONParser_CanParse(t *testing.T) {
	p := &JSONParser{}

	tests := []struct {
		line string
		want bool
	}{
		{`{"valid":"json"}`, true},
		{`{"nested":{"valid":"json"}}`, true},
		{`not json`, false},
		{`{"invalid json}`, false},
		{` `, false},
		{``, false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			if got := p.CanParse(tt.line); got != tt.want {
				t.Errorf("CanParse() = %v, want %v", got, tt.want)
			}
		})
	}
}
