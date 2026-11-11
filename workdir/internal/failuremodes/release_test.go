package failuremodes

import (
	"testing"
	"time"
)

const mib = 1024 * 1024

// curve returns memory in bytes at minute m for a warm-up then a leak.
func curve(warm, ramp, leak float64) func(m float64) float64 {
	return func(m float64) float64 {
		v := warm
		if m < ramp {
			v = warm * m / ramp
		}
		if m > ramp {
			v += leak * (m - ramp)
		}
		return (v + 20) * mib
	}
}

// runRelease feeds 30 minutes of v1, then rolls two pods to v2 and returns the
// first regression reported and when, in minutes after the rollout.
func runRelease(t *testing.T, withReference bool, v2 func(float64) float64) (Estimate, float64, bool) {
	t.Helper()
	tr := NewTracker()
	start := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	v1 := curve(60, 3, 0)
	now := start
	if withReference {
		for i := 0; i < 60; i++ {
			now = start.Add(time.Duration(i) * 30 * time.Second)
			for _, pod := range []string{"ns/app-a-1", "ns/app-a-2"} {
				tr.ObserveVersion(pod, "app-a")
				tr.Observe("ns/Deployment/app", pod, now, v1(float64(i)/2), 256*mib, 0)
			}
			tr.Worst("ns/Deployment/app", now)
		}
	}
	rollout := now.Add(30 * time.Second)
	for i := 0; i < 160; i++ {
		now = rollout.Add(time.Duration(i) * 30 * time.Second)
		for _, pod := range []string{"ns/app-b-1", "ns/app-b-2"} {
			tr.ObserveVersion(pod, "app-b")
			tr.Observe("ns/Deployment/app", pod, now, v2(float64(i)/2), 256*mib, 0)
		}
		if est, ok := tr.Worst("ns/Deployment/app", now); ok && est.Mode == ModeReleaseRegression {
			return est, float64(i) / 2, true
		}
	}
	return Estimate{}, 0, false
}

func TestReleaseRegression(t *testing.T) {
	cases := []struct {
		name      string
		reference bool
		v2        func(float64) float64
		want      bool
		within    float64
	}{
		{"leaking release", true, curve(60, 3, 3), true, 15},
		{"healthy release", true, curve(60, 3, 0), false, 0},
		{"heavier and stable, long warm-up", true, curve(120, 8, 0), false, 0},
		{"no previous version known", false, curve(60, 3, 3), false, 0},
		{"leak too slow for the horizon", true, curve(60, 3, 0.8), false, 0},
	}
	for _, tc := range cases {
		est, at, got := runRelease(t, tc.reference, tc.v2)
		if got != tc.want {
			t.Errorf("%s: reported = %v (%+v at %.1f min)", tc.name, got, est, at)
			continue
		}
		if !got {
			continue
		}
		if at > tc.within {
			t.Errorf("%s: reported at %.1f min, want within %.0f", tc.name, at, tc.within)
		}
		if est.Version != "app-b" || est.PreviousVersion != "app-a" || est.ETF <= 0 || est.ETF > releaseHorizon {
			t.Errorf("%s: estimate = %+v", tc.name, est)
		}
		if leak := est.Rate * 60 / mib; leak < 2.5 || leak > 3.5 {
			t.Errorf("%s: rate %.2f MiB/min, want about 3", tc.name, leak)
		}
	}
}

func TestReleaseRegression_PrunesOldProfiles(t *testing.T) {
	s := newReleaseState()
	now := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	s.profiles["old"] = versionProfile{rate: 1, at: now.Add(-48 * time.Hour)}
	s.profiles["recent"] = versionProfile{rate: 1, at: now}
	s.prune(now.Add(-time.Hour))
	if _, ok := s.profiles["old"]; ok {
		t.Error("a profile older than its TTL must be pruned")
	}
	if _, ok := s.profiles["recent"]; !ok {
		t.Error("a recent profile must be kept")
	}
}

// A pod long past its rollout that grows for a while is judged by saturation,
// not compared with the release before it.
func TestReleaseRegression_OnlyNearTheRollout(t *testing.T) {
	tr := NewTracker()
	start := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	flat := curve(60, 3, 0)
	now := start
	for i := 0; i < 60; i++ {
		now = start.Add(time.Duration(i) * 30 * time.Second)
		tr.ObserveVersion("ns/app-a-1", "app-a")
		tr.Observe("ns/Deployment/app", "ns/app-a-1", now, flat(float64(i)/2), 256*mib, 0)
		tr.Worst("ns/Deployment/app", now)
	}
	rollout := now.Add(30 * time.Second)
	burstAt := 3 * 60.0
	for i := 0; i < int((burstAt+30)*2); i++ {
		m := float64(i) / 2
		usage := flat(m)
		if m > burstAt {
			usage += 3 * mib * (m - burstAt)
		}
		now = rollout.Add(time.Duration(i) * 30 * time.Second)
		tr.ObserveVersion("ns/app-b-1", "app-b")
		tr.Observe("ns/Deployment/app", "ns/app-b-1", now, usage, 256*mib, 0)
		if est, ok := tr.Worst("ns/Deployment/app", now); ok && est.Mode == ModeReleaseRegression {
			t.Fatalf("release regression %.1f min after the rollout, want none past %s", m, releaseRecent)
		}
	}
}
