package failuremodes

import (
	"testing"
	"time"
)

func TestTracker_StartupStuck(t *testing.T) {
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		reason  string
		message string
		seenAt  time.Time
		now     time.Time
		want    bool
		wantWhy string
	}{
		{"mount failure", "FailedMount", "", start, start.Add(time.Minute), true, "FailedMount"},
		{"scheduling failure", "FailedScheduling", "", start, start.Add(time.Minute), true, "FailedScheduling"},
		{"image pull failure", "Failed", "Error: ErrImagePull", start, start.Add(time.Minute), true, "ImagePull"},
		{"image backoff", "Failed", "Error: ImagePullBackOff", start, start.Add(time.Minute), true, "ImagePull"},
		{"waiting to pull again", "BackOff", "Back-off pulling image \"x\"", start, start.Add(time.Minute), true, "ImagePull"},
		{"crash backoff is not a start failure", "BackOff", "Back-off restarting failed container", start, start.Add(time.Minute), false, ""},
		{"another Failed event", "Failed", "Error: failed to start", start, start.Add(time.Minute), false, ""},
		{"unrelated reason", "Unhealthy", "", start, start.Add(time.Minute), false, ""},
		{"event too old", "FailedMount", "", start, start.Add(startupEventMaxAge + time.Second), false, ""},
	}
	for _, c := range cases {
		tr := NewTracker()
		tr.ObserveEvent("w", "n/p", c.reason, c.message, c.seenAt)
		est, ok := tr.Worst("w", c.now)
		if ok != c.want {
			t.Errorf("%s: found = %v, want %v", c.name, ok, c.want)
			continue
		}
		if ok && (est.Mode != ModeStartupStuck || est.Pod != "n/p" || est.Reason != c.wantWhy) {
			t.Errorf("%s: estimate = %+v", c.name, est)
		}
	}
}

func TestTracker_PruneKeepsFreshEventState(t *testing.T) {
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	tr := NewTracker()
	tr.ObserveEvent("w", "n/p", "FailedMount", "", now)
	tr.Prune(now.Add(-time.Minute))
	if _, ok := tr.Worst("w", now); !ok {
		t.Error("a fresh startup failure was pruned")
	}
}

func TestSaturation_Shapes(t *testing.T) {
	const step = 30 * time.Second
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	stepLeak := func(minutes int) float64 {
		level, elapsed, pause := 0.1, 0, 3
		for elapsed+pause <= minutes {
			elapsed += pause
			level += 0.19
			pause = 3 + (pause % 9)
		}
		return level
	}
	sawtooth := func(minutes int) float64 {
		phase := minutes % 12
		if phase >= 10 {
			return 0.1
		}
		return 0.1 + 0.075*float64(phase)
	}
	steady := func(minutes int) float64 { return 0.3 }
	longPlateau := func(minutes int) float64 {
		switch {
		case minutes < 3:
			return 0.19
		case minutes < 6:
			return 0.38
		case minutes < 15:
			return 0.57
		}
		return 0.76
	}
	cases := []struct {
		name  string
		usage func(int) float64
		from  int
		until int
		want  bool
	}{
		{"irregular step leak warns before the limit", stepLeak, 0, 22, true},
		{"long plateau before the last steps warns early", longPlateau, 0, 16, true},
		{"sawtooth stays silent once a drop was seen", sawtooth, 15, 180, false},
		{"flat usage stays silent", steady, 0, 60, false},
	}
	for _, c := range cases {
		s := NewSaturation()
		warned := false
		for i := 0; i < c.until*2; i++ {
			at := start.Add(time.Duration(i) * step)
			minutes := i / 2
			if _, ok := s.Observe(at, c.usage(minutes)*100, 100, 0); ok && minutes >= c.from {
				warned = true
			}
		}
		if warned != c.want {
			t.Errorf("%s: warned = %v, want %v", c.name, warned, c.want)
		}
	}
}
