package failuremodes

import (
	"testing"
	"time"
)

func TestTracker_ReplicasUnavailable(t *testing.T) {
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		missing []int
		want    bool
	}{
		{"all available", []int{0, 0, 0, 0, 0, 0, 0}, false},
		{"short rollout gap", []int{0, 1, 1, 0, 0, 0, 0}, false},
		{"sustained gap", []int{1, 1, 1, 1, 1, 1, 1}, true},
		{"gap cleared", []int{1, 1, 1, 1, 1, 1, 0}, false},
	}
	for _, c := range cases {
		tr := NewTracker()
		at := start
		for _, missing := range c.missing {
			tr.ObserveReplicas("n/Deployment/w", missing, at)
			at = at.Add(time.Minute)
		}
		est, ok := tr.Worst("n/Deployment/w", at)
		if ok != c.want {
			t.Errorf("%s: found = %v, want %v", c.name, ok, c.want)
			continue
		}
		if ok && (est.Mode != ModeReplicasUnavailable || est.Usage != 1) {
			t.Errorf("%s: estimate = %+v", c.name, est)
		}
	}
}

func TestTracker_ReplicaReadingExpires(t *testing.T) {
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	tr := NewTracker()
	for i := 0; i < 7; i++ {
		tr.ObserveReplicas("w", 1, start.Add(time.Duration(i)*time.Minute))
	}
	if _, ok := tr.Worst("w", start.Add(6*time.Minute+estimateMaxAge+time.Second)); ok {
		t.Error("a stale replica reading must not stay active")
	}
}
