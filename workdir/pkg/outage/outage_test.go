package outage

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestLog_OneLinePerOutage(t *testing.T) {
	var lines []string
	l := New("Kubernetes API", func(format string, args ...interface{}) { lines = append(lines, fmt.Sprintf(format, args...)) })
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return now }

	l.Succeeded()
	for i := 0; i < 40; i++ {
		l.Failed("list pods", errors.New("connection refused"))
		now = now.Add(3 * time.Second)
	}
	l.Succeeded()
	l.Succeeded()
	l.Failed("watch pods", errors.New("EOF"))
	l.Succeeded()

	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4: %q", len(lines), lines)
	}
	if !strings.Contains(lines[0], "Kubernetes API unreachable (list pods: connection refused)") {
		t.Errorf("first line %q", lines[0])
	}
	if !strings.Contains(lines[1], "reachable again after 2m0s, 40 failed calls") {
		t.Errorf("second line %q", lines[1])
	}
}
