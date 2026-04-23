package collector

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LogEntry represents a single log line collected from the system
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Host      string    `json:"host"`
}

// LogCollector tails log files and classifies lines
type LogCollector struct {
	host    string
	sources []string
	offsets map[string]int64 // last read offset per file
}

// NewLogCollector creates a log collector for the given host
func NewLogCollector(host string, sources []string) *LogCollector {
	if len(sources) == 0 {
		sources = defaultLogSources()
	}
	return &LogCollector{
		host:    host,
		sources: sources,
		offsets: make(map[string]int64),
	}
}

// Collect reads new log lines from all sources since last call
func (lc *LogCollector) Collect() ([]LogEntry, error) {
	var entries []LogEntry
	now := time.Now()

	for _, pattern := range lc.sources {
		files, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, path := range files {
			newEntries := lc.readNewLines(path, now)
			entries = append(entries, newEntries...)
		}
	}

	return entries, nil
}

// ErrorRate returns the proportion of error/warn lines in recently collected entries
func ErrorRate(entries []LogEntry) float64 {
	if len(entries) == 0 {
		return 0
	}
	errCount := 0
	for _, e := range entries {
		if e.Level == "error" || e.Level == "fatal" || e.Level == "critical" {
			errCount++
		}
	}
	return float64(errCount) / float64(len(entries))
}

func (lc *LogCollector) readNewLines(path string, now time.Time) []LogEntry {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()

	offset := lc.offsets[path]
	if _, err := f.Seek(offset, 0); err != nil {
		return nil
	}

	source := filepath.Base(path)
	var entries []LogEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 64*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		entries = append(entries, LogEntry{
			Timestamp: now,
			Source:    source,
			Level:     classifyLine(line),
			Message:   truncate(line, 512),
			Host:      lc.host,
		})
	}

	// Update offset to current position
	pos, _ := f.Seek(0, 1)
	lc.offsets[path] = pos

	return entries
}

func classifyLine(line string) string {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "fatal") || strings.Contains(lower, "critical") || strings.Contains(lower, "panic"):
		return "fatal"
	case strings.Contains(lower, "error") || strings.Contains(lower, "err ") || strings.Contains(lower, "exception"):
		return "error"
	case strings.Contains(lower, "warn"):
		return "warn"
	case strings.Contains(lower, "debug"):
		return "debug"
	default:
		return "info"
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func defaultLogSources() []string {
	return []string{
		"/var/log/syslog",
		"/var/log/messages",
		"/var/log/kern.log",
		"/var/log/auth.log",
	}
}
