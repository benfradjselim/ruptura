package collector_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/benfradjselim/kairo-core/internal/collector"
)

// --- LogCollector tests ---

func TestErrorRate(t *testing.T) {
	entries := []collector.LogEntry{
		{Level: "info"},
		{Level: "info"},
		{Level: "error"},
		{Level: "fatal"},
		{Level: "warn"},
	}
	got := collector.ErrorRate(entries)
	// 2 error/fatal out of 5 = 0.4
	if got < 0.39 || got > 0.41 {
		t.Errorf("ErrorRate = %.3f; want ~0.4", got)
	}
}

func TestErrorRateEmpty(t *testing.T) {
	if r := collector.ErrorRate(nil); r != 0 {
		t.Errorf("ErrorRate(nil) = %.3f; want 0", r)
	}
}

func TestLogCollectorReadFile(t *testing.T) {
	// Create a temp log file with known content
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")
	content := "INFO starting server\nERROR database connection failed\nWARN high latency\nDEBUG query executed\nFATAL out of memory\n"
	if err := os.WriteFile(logFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	lc := collector.NewLogCollector("testhost", []string{logFile})
	entries, err := lc.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if len(entries) != 5 {
		t.Errorf("collected %d entries; want 5", len(entries))
	}

	// Classify levels
	levels := make(map[string]int)
	for _, e := range entries {
		levels[e.Level]++
	}
	if levels["error"] != 1 {
		t.Errorf("error count = %d; want 1", levels["error"])
	}
	if levels["fatal"] != 1 {
		t.Errorf("fatal count = %d; want 1", levels["fatal"])
	}
	if levels["warn"] != 1 {
		t.Errorf("warn count = %d; want 1", levels["warn"])
	}
}

func TestLogCollectorIncrementalRead(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "incremental.log")
	f, err := os.Create(logFile)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	lc := collector.NewLogCollector("host", []string{logFile})

	// First collect — empty file
	e1, _ := lc.Collect()
	if len(e1) != 0 {
		t.Errorf("first collect: got %d entries; want 0", len(e1))
	}

	// Write some lines
	f.WriteString("INFO line1\nERROR line2\n")
	f.Sync()

	e2, _ := lc.Collect()
	if len(e2) != 2 {
		t.Errorf("second collect: got %d entries; want 2", len(e2))
	}

	// Write more lines; should only read new ones
	f.WriteString("WARN line3\n")
	f.Sync()

	e3, _ := lc.Collect()
	if len(e3) != 1 {
		t.Errorf("third collect: got %d entries; want 1 (incremental)", len(e3))
	}
	f.Close()
}

func TestLogCollectorMissingFile(t *testing.T) {
	lc := collector.NewLogCollector("host", []string{"/nonexistent/path/log.log"})
	entries, err := lc.Collect()
	if err != nil {
		t.Errorf("Collect with missing file should not error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for missing file, got %d", len(entries))
	}
}

func TestLogCollectorTruncate(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "long.log")

	// Write a very long line (>512 chars)
	longLine := make([]byte, 600)
	for i := range longLine {
		longLine[i] = 'A'
	}
	if err := os.WriteFile(logFile, append(longLine, '\n'), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	lc := collector.NewLogCollector("host", []string{logFile})
	entries, _ := lc.Collect()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	// Message should be truncated (≤ 512 chars + ellipsis byte)
	if len(entries[0].Message) > 520 {
		t.Errorf("message not truncated: len=%d", len(entries[0].Message))
	}
}

// --- ContainerCollector tests ---

func TestContainerCollectorNoDocker(t *testing.T) {
	// On CI/dev without Docker socket, should fall back gracefully
	cc := collector.NewContainerCollector("testhost")
	metrics, err := cc.Collect()
	// Either returns empty list or an error — must not panic
	if err != nil {
		t.Logf("Collect (no docker): %v (expected in CI)", err)
	}
	// metrics may be nil/empty — just check no panic
	_ = metrics
}

// --- SystemCollector smoke test ---

func TestSystemCollectorCollect(t *testing.T) {
	sc := collector.NewSystemCollector("testhost")
	metrics, err := sc.Collect()
	if err != nil {
		t.Fatalf("SystemCollector.Collect: %v", err)
	}
	if len(metrics) == 0 {
		t.Error("expected at least one system metric")
	}

	// Verify key metrics are present
	found := make(map[string]bool)
	for _, m := range metrics {
		found[m.Name] = true
	}
	required := []string{"cpu_percent", "memory_percent", "load_avg_1"}
	for _, name := range required {
		if !found[name] {
			t.Errorf("missing required metric: %s", name)
		}
	}
}

func TestSystemCollectorHost(t *testing.T) {
	sc := collector.NewSystemCollector("myhost")
	metrics, err := sc.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	for _, m := range metrics {
		if m.Host != "myhost" {
			t.Errorf("metric %s has host=%q; want 'myhost'", m.Name, m.Host)
		}
	}
}

func TestSystemCollectorConcurrent(t *testing.T) {
	sc := collector.NewSystemCollector("host")
	// Fire multiple concurrent Collect calls to exercise the mutex
	done := make(chan struct{}, 5)
	for i := 0; i < 5; i++ {
		go func() {
			sc.Collect() //nolint:errcheck — testing concurrency safety
			done <- struct{}{}
		}()
	}
	for i := 0; i < 5; i++ {
		<-done
	}
}

func TestCollectSystemMetrics(t *testing.T) {
	sc := collector.NewSystemCollector("syshost")
	sm, err := sc.CollectSystemMetrics()
	if err != nil {
		t.Fatalf("CollectSystemMetrics: %v", err)
	}
	if sm == nil {
		t.Fatal("CollectSystemMetrics returned nil")
	}
	if sm.Host != "syshost" {
		t.Errorf("Host = %q; want syshost", sm.Host)
	}
	if sm.CPUPercent < 0 || sm.CPUPercent > 100 {
		t.Errorf("CPUPercent = %v; want [0,100]", sm.CPUPercent)
	}
	if sm.MemoryPercent < 0 || sm.MemoryPercent > 100 {
		t.Errorf("MemoryPercent = %v; want [0,100]", sm.MemoryPercent)
	}
}

func TestLogCollectorDefaultSources(t *testing.T) {
	// nil sources → uses defaultLogSources(); should not panic
	lc := collector.NewLogCollector("host", nil)
	entries, err := lc.Collect()
	if err != nil {
		t.Fatalf("Collect with default sources: %v", err)
	}
	// Default paths (/var/log/syslog etc.) may or may not exist — just verify no panic
	_ = entries
}
