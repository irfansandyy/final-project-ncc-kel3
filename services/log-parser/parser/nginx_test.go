package parser

import (
	"strconv"
	"testing"
)

func TestNginxParser_Parse(t *testing.T) {
	p := &NginxParser{}

	tests := []struct {
		name         string
		line         string
		wantLevel    string
		wantMessage  string
		wantMetadata map[string]interface{}
		wantErr      bool
	}{
		{
			name:        "Valid 200 GET",
			line:        `192.168.1.1 - - [15/Mar/2024:10:30:00 +0000] "GET /api/health HTTP/1.1" 200 1234 "-" "curl/7.68.0"`,
			wantLevel:   "INFO",
			wantMessage: "GET /api/health HTTP/1.1",
			wantMetadata: map[string]interface{}{
				"format":      "nginx",
				"remote_addr": "192.168.1.1",
				"status":      200,
				"method":      "GET",
				"uri":         "/api/health",
				"bytes_sent":  1234,
				"user_agent":  "curl/7.68.0",
			},
			wantErr: false,
		},
		{
			name:        "Valid 401 POST",
			line:        `10.0.0.5 - admin [15/Mar/2024:10:30:01 +0000] "POST /api/login HTTP/1.1" 401 89 "https://example.com" "Mozilla/5.0"`,
			wantLevel:   "WARN",
			wantMessage: "POST /api/login HTTP/1.1",
			wantMetadata: map[string]interface{}{
				"format":      "nginx",
				"remote_addr": "10.0.0.5",
				"status":      401,
				"method":      "POST",
				"uri":         "/api/login",
				"bytes_sent":  89,
				"referer":     "https://example.com",
				"user_agent":  "Mozilla/5.0",
			},
			wantErr: false,
		},
		{
			name:        "Valid 500 DELETE",
			line:        `192.168.1.100 - - [15/Mar/2024:10:30:03 +0000] "DELETE /api/users/5 HTTP/1.1" 500 45 "-" "HTTPie/3.2"`,
			wantLevel:   "ERROR",
			wantMessage: "DELETE /api/users/5 HTTP/1.1",
			wantMetadata: map[string]interface{}{
				"format":      "nginx",
				"remote_addr": "192.168.1.100",
				"status":      500,
				"method":      "DELETE",
				"uri":         "/api/users/5",
				"bytes_sent":  45,
				"user_agent":  "HTTPie/3.2",
			},
			wantErr: false,
		},
		{
			name:    "Invalid format",
			line:    "This is not an nginx line",
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

func TestMapHTTPStatusToLevel(t *testing.T) {
	tests := []struct {
		status int
		want   string
	}{
		{200, "INFO"},
		{204, "INFO"},
		{301, "INFO"},
		{400, "WARN"},
		{401, "WARN"},
		{404, "WARN"},
		{500, "ERROR"},
		{502, "ERROR"},
		{503, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.status), func(t *testing.T) {
			if got := mapHTTPStatusToLevel(tt.status); got != tt.want {
				t.Errorf("mapHTTPStatusToLevel(%d) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}
