package parser

import (
	"testing"
)

func TestSyslogParser_Parse(t *testing.T) {
	p := &SyslogParser{}

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
			name:        "RFC5424 - Valid",
			line:        "<34>1 2024-03-15T10:30:00.000Z server1 sshd 1234 - - Failed password for root",
			wantLevel:   "CRITICAL", // pri 34 -> severity 2 -> CRITICAL
			wantSource:  "server1",
			wantMessage: "Failed password for root",
			wantMetadata: map[string]interface{}{
				"format":   "rfc5424",
				"app_name": "sshd",
				"proc_id":  "1234",
			},
			wantErr: false,
		},
		{
			name:        "RFC5424 - Valid with struct data",
			line:        "<165>1 2024-03-15T10:31:00Z server2 nginx 5678 - [exampleSDID@32473 iut=\"3\"] Connection refused",
			wantLevel:   "INFO", // pri 165 -> severity 5 -> INFO
			wantSource:  "server2",
			wantMessage: "Connection refused",
			wantMetadata: map[string]interface{}{
				"format":   "rfc5424",
				"app_name": "nginx",
				"proc_id":  "5678",
			},
			wantErr: false,
		},
		{
			name:        "BSD with PRI",
			line:        "<13>May 15 10:30:00 server1 sshd[1234]: Failed password for root",
			wantLevel:   "INFO", // pri 13 -> severity 5 -> INFO
			wantSource:  "server1",
			wantMessage: "Failed password for root",
			wantMetadata: map[string]interface{}{
				"format": "bsd",
				"app":    "sshd[1234]",
			},
			wantErr: false,
		},
		{
			name:        "BSD without PRI",
			line:        "May 15 10:30:00 server1 sshd[1234]: Failed password for root",
			wantLevel:   "INFO", // Default
			wantSource:  "server1",
			wantMessage: "Failed password for root",
			wantMetadata: map[string]interface{}{
				"format": "bsd",
				"app":    "sshd[1234]",
			},
			wantErr: false,
		},
		{
			name:    "Invalid format",
			line:    "This is not a syslog line",
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
				// Verify timestamp isn't zero
				if got.Timestamp.IsZero() {
					t.Errorf("Parse() Timestamp is zero")
				}
			}
		})
	}
}

func TestMapSeverityToLevel(t *testing.T) {
	tests := []struct {
		severity int
		want     string
	}{
		{0, "CRITICAL"}, // Emergency
		{1, "CRITICAL"}, // Alert
		{2, "CRITICAL"}, // Critical
		{3, "ERROR"},    // Error
		{4, "WARN"},     // Warning
		{5, "INFO"},     // Notice
		{6, "INFO"},     // Informational
		{7, "INFO"},     // Debug
		{8, "INFO"},     // Invalid, default to INFO
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.severity)), func(t *testing.T) {
			if got := mapSeverityToLevel(tt.severity); got != tt.want {
				t.Errorf("mapSeverityToLevel(%d) = %v, want %v", tt.severity, got, tt.want)
			}
		})
	}
}
