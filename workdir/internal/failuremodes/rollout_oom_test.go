package failuremodes

import (
	"testing"
	"time"
)

func TestTracker_RolloutStuck(t *testing.T) {
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		stalled bool
		age     time.Duration
		want    bool
	}{
		{"stalled rollout", true, 0, true},
		{"healthy rollout", false, 0, false},
		{"stale reading", true, estimateMaxAge + time.Second, false},
	}
	for _, c := range cases {
		tr := NewTracker()
		tr.ObserveRollout("w", c.stalled, now.Add(-c.age))
		est, ok := tr.Worst("w", now)
		if ok != c.want {
			t.Errorf("%s: found = %v, want %v", c.name, ok, c.want)
		}
		if ok && est.Mode != ModeRolloutStuck {
			t.Errorf("%s: estimate = %+v", c.name, est)
		}
	}
	tr := NewTracker()
	tr.ObserveRollout("w", true, now)
	tr.ObserveRollout("w", false, now.Add(time.Minute))
	if _, ok := tr.Worst("w", now.Add(time.Minute)); ok {
		t.Error("a recovered rollout must clear")
	}
}

func TestTracker_OOMKilled(t *testing.T) {
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	observe := func(tr *Tracker, oom bool, restarts int, at time.Time) {
		tr.ObserveRestarts("w", "n/p", at, restarts)
		tr.ObserveOOM("w", "n/p", oom, at)
	}
	cases := []struct {
		name     string
		oom      bool
		restarts int
		at       time.Time
		want     bool
	}{
		{"recent OOM", true, 2, now, true},
		{"old OOM with no new restart", true, 1, now, false},
		{"other restart", false, 2, now, false},
		{"OOM outside the window", true, 2, now.Add(-oomRecentWindow - time.Minute), false},
	}
	for _, c := range cases {
		tr := NewTracker()
		observe(tr, c.oom, 1, c.at.Add(-time.Minute))
		observe(tr, c.oom, c.restarts, c.at)
		est, ok := tr.Worst("w", now)
		if ok != c.want {
			t.Errorf("%s: found = %v, want %v (%+v)", c.name, ok, c.want, est)
		}
		if ok && (est.Mode != ModeOOMKilled || est.Pod != "n/p") {
			t.Errorf("%s: estimate = %+v", c.name, est)
		}
	}
}

func TestWorst_ForecastOutranksPastOOM(t *testing.T) {
	if (Estimate{Mode: ModeOOMKilled}).urgency() <= (Estimate{Mode: ModeSaturation, ETF: 12 * time.Hour}).urgency() {
		t.Error("a forecast must outrank a past OOM")
	}
}
