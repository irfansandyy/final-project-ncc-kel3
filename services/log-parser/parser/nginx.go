package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

func init() {
	Register(&NginxParser{})
}

// NginxParser parses nginx combined access log format.
type NginxParser struct{}

func (p *NginxParser) Name() string {
	return "nginx"
}

var (
	// Combined Log Format:
	// $remote_addr - $remote_user [$time_local] "$request" $status $body_bytes_sent "$http_referer" "$http_user_agent"
	// Example: 192.168.1.1 - - [15/Mar/2024:10:30:00 +0000] "GET /api/health HTTP/1.1" 200 1234 "-" "curl/7.68.0"
	nginxRegex = regexp.MustCompile(`^(\S+) \S+ (\S+) \[([^\]]+)\] "([^"]+)" (\d{3}) (\d+|-) "([^"]*)" "([^"]*)"$`)
)

func (p *NginxParser) CanParse(line string) bool {
	return nginxRegex.MatchString(line)
}

func (p *NginxParser) Parse(line string) (*ParsedEvent, error) {
	matches := nginxRegex.FindStringSubmatch(line)
	if matches == nil {
		return nil, fmt.Errorf("line does not match nginx combined format")
	}

	// matches: [full, remote_addr, remote_user, time_local, request, status, body_bytes, referer, user_agent]
	remoteAddr := matches[1]
	timeStr := matches[3]
	request := matches[4]
	statusStr := matches[5]
	bytesStr := matches[6]
	referer := matches[7]
	userAgent := matches[8]

	status, _ := strconv.Atoi(statusStr)
	level := mapHTTPStatusToLevel(status)

	// Nginx time format: 15/Mar/2024:10:30:00 +0000
	ts, err := time.Parse("02/Jan/2006:15:04:05 -0700", timeStr)
	if err != nil {
		ts = time.Now()
	}

	// Extract method and uri from request (e.g. "GET /api/health HTTP/1.1")
	var method, uri string
	reqParts := regexp.MustCompile(`^(\S+)\s+(\S+)`).FindStringSubmatch(request)
	if len(reqParts) == 3 {
		method = reqParts[1]
		uri = reqParts[2]
	}

	bytesSent := 0
	if bytesStr != "-" {
		bytesSent, _ = strconv.Atoi(bytesStr)
	}

	metadata := map[string]interface{}{
		"format":      "nginx",
		"remote_addr": remoteAddr,
		"status":      status,
		"method":      method,
		"uri":         uri,
		"bytes_sent":  bytesSent,
	}

	if referer != "-" && referer != "" {
		metadata["referer"] = referer
	}
	if userAgent != "-" && userAgent != "" {
		metadata["user_agent"] = userAgent
	}

	return &ParsedEvent{
		Timestamp: ts,
		Level:     level,
		Source:    "nginx", // Default source for nginx logs
		Message:   request,
		Raw:       line,
		Metadata:  metadata,
	}, nil
}

func mapHTTPStatusToLevel(status int) string {
	if status >= 500 {
		return "ERROR"
	} else if status >= 400 {
		return "WARN"
	}
	return "INFO"
}
