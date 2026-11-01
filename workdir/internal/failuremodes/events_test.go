package failuremodes

import (
	"testing"
	"time"
)

func TestTracker_StartupStuck(t *testing.T) {
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		reason string
		seenAt time.Time
		now    time.Time
		want   bool
	}{
		{"mount failure", "FailedMount", start, start.Add(time.Minute), true},
		{"scheduling failure", "FailedScheduling", start, start.Add(time.Minute), true},
		{"image pull failure", "ErrImagePull", start, start.Add(time.Minute), true},
		{"unrelated reason", "Unhealthy", start, start.Add(time.Minute), false},
		{"event too old", "FailedMount", start, start.Add(startupEventMaxAge + time.Second), false},
	}
	for _, c := range cases {
		tr := NewTracker()
		tr.ObserveEvent("w", "n/p", c.reason, c.seenAt)
		est, ok := tr.Worst("w", c.now)
		if ok != c.want {
			t.Errorf("%s: found = %v, want %v", c.name, ok, c.want)
			continue
		}
		if ok && (est.Mode != ModeStartupStuck || est.Pod != "n/p" || est.Reason != c.reason) {
			t.Errorf("%s: estimate = %+v", c.name, est)
		}
	}
}

func TestTracker_PruneKeepsFreshEventState(t *testing.T) {
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	tr := NewTracker()
	tr.ObserveEvent("w", "n/p", "FailedMount", now)
	tr.Prune(now.Add(-time.Minute))
	if _, ok := tr.Worst("w", now); !ok {
		t.Error("a fresh startup failure was pruned")
	}
}
