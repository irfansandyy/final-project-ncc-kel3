package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func init() {
	Register(&SyslogParser{})
}

// SyslogParser parses RFC5424 and BSD/RFC3164 syslog formats.
type SyslogParser struct{}

func (p *SyslogParser) Name() string {
	return "syslog"
}

// Regexes for detection and parsing
var (
	// RFC5424: <PRI>VERSION TIMESTAMP HOSTNAME APP-NAME PROCID MSGID STRUCTURED-DATA MSG
	// Example: <34>1 2024-03-15T10:30:00.000Z server1 sshd 1234 - - Failed password
	rfc5424Regex = regexp.MustCompile(`^<(\d+)>\d+ (\S+) (\S+) (\S+) (\S+) (\S+) (-|\[.*?\](?: \[.*?\])*) (.*)$`)

	// BSD with PRI: <PRI>TIMESTAMP HOSTNAME APP: MSG
	// Example: <13>May 15 10:30:00 server1 sshd[1234]: Failed password
	bsdWithPRIRegex = regexp.MustCompile(`^<(\d+)>([A-Z][a-z]{2}\s+\d+\s+\d{2}:\d{2}:\d{2})\s+(\S+)\s+(.*?):\s+(.*)$`)

	// BSD without PRI: TIMESTAMP HOSTNAME APP: MSG
	// Example: May 15 10:30:00 server1 sshd[1234]: Failed password
	bsdNoPRIRegex = regexp.MustCompile(`^([A-Z][a-z]{2}\s+\d+\s+\d{2}:\d{2}:\d{2})\s+(\S+)\s+(.*?):\s+(.*)$`)
)

func (p *SyslogParser) CanParse(line string) bool {
	return rfc5424Regex.MatchString(line) ||
		bsdWithPRIRegex.MatchString(line) ||
		bsdNoPRIRegex.MatchString(line)
}

func (p *SyslogParser) Parse(line string) (*ParsedEvent, error) {
	if matches := rfc5424Regex.FindStringSubmatch(line); matches != nil {
		return p.parseRFC5424(matches, line)
	}
	if matches := bsdWithPRIRegex.FindStringSubmatch(line); matches != nil {
		return p.parseBSDWithPRI(matches, line)
	}
	if matches := bsdNoPRIRegex.FindStringSubmatch(line); matches != nil {
		return p.parseBSDNoPRI(matches, line)
	}
	return nil, fmt.Errorf("line does not match any syslog format")
}

func (p *SyslogParser) parseRFC5424(matches []string, raw string) (*ParsedEvent, error) {
	// matches: [full, pri, timestamp, hostname, appName, procID, msgID, structData, msg]
	pri, _ := strconv.Atoi(matches[1])
	severity := pri & 7
	level := mapSeverityToLevel(severity)

	ts, err := time.Parse(time.RFC3339Nano, matches[2])
	if err != nil {
		// Fallback if parsing fails
		ts = time.Now()
	}

	metadata := map[string]interface{}{
		"format": "rfc5424",
	}
	if matches[4] != "-" {
		metadata["app_name"] = matches[4]
	}
	if matches[5] != "-" {
		metadata["proc_id"] = matches[5]
	}
	if matches[6] != "-" {
		metadata["msg_id"] = matches[6]
	}

	return &ParsedEvent{
		Timestamp: ts,
		Level:     level,
		Source:    matches[3],
		Message:   strings.TrimSpace(matches[8]),
		Raw:       raw,
		Metadata:  metadata,
	}, nil
}

func (p *SyslogParser) parseBSDWithPRI(matches []string, raw string) (*ParsedEvent, error) {
	// matches: [full, pri, timestamp, hostname, app, msg]
	pri, _ := strconv.Atoi(matches[1])
	severity := pri & 7
	level := mapSeverityToLevel(severity)

	ts := parseBSDTime(matches[2])

	return &ParsedEvent{
		Timestamp: ts,
		Level:     level,
		Source:    matches[3],
		Message:   strings.TrimSpace(matches[5]),
		Raw:       raw,
		Metadata: map[string]interface{}{
			"format": "bsd",
			"app":    matches[4],
		},
	}, nil
}

func (p *SyslogParser) parseBSDNoPRI(matches []string, raw string) (*ParsedEvent, error) {
	// matches: [full, timestamp, hostname, app, msg]
	ts := parseBSDTime(matches[1])

	return &ParsedEvent{
		Timestamp: ts,
		Level:     "INFO", // Default level if no PRI
		Source:    matches[2],
		Message:   strings.TrimSpace(matches[4]),
		Raw:       raw,
		Metadata: map[string]interface{}{
			"format": "bsd",
			"app":    matches[3],
		},
	}, nil
}

func mapSeverityToLevel(severity int) string {
	switch severity {
	case 0, 1, 2:
		return "CRITICAL"
	case 3:
		return "ERROR"
	case 4:
		return "WARN"
	case 5, 6, 7:
		return "INFO"
	default:
		return "INFO"
	}
}

func parseBSDTime(tsStr string) time.Time {
	// BSD time doesn't have a year. Parse it and set the year to current year.
	// Format: "Jan  2 15:04:05"
	layout := time.Stamp
	t, err := time.Parse(layout, tsStr)
	if err != nil {
		return time.Now()
	}
	
	now := time.Now()
	return time.Date(now.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, now.Location())
}
