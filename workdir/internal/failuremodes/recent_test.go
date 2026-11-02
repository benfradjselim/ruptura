package failuremodes

import (
	"testing"
	"time"
)

func TestRecentFailures(t *testing.T) {
	start := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	tr := NewTracker()
	tr.RestoreFailures("a", []time.Time{start, start.Add(time.Hour)})
	tr.RestoreFailures("b", []time.Time{start})
	got := tr.RecentFailures(start.Add(30 * time.Minute))
	if len(got) != 1 || len(got["a"]) != 1 || !got["a"][0].Equal(start.Add(time.Hour)) {
		t.Errorf("failures = %v", got)
	}
}
