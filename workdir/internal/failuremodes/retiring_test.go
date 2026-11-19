package failuremodes

import (
	"testing"
	"time"
)

// TestTracker_RetiringPods replays the end of an F2 round: a leaking release is
// rolled back. Once its pods are marked retiring, neither their saturation nor
// the release regression they carry may be reported, while v1 stays quiet and
// a leaking pod that is not retiring is still reported.
func TestTracker_RetiringPods(t *testing.T) {
	tests := []struct {
		name     string
		retiring bool
		want     bool
	}{
		{"bad release still current", false, true},
		{"bad release rolled back", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewTracker()
			const workload = "ns/Deployment/app"
			start := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
			v1, bad := curve(60, 3, 0), curve(60, 3, 3)
			now := start
			for i := 0; i < 60; i++ {
				now = start.Add(time.Duration(i) * 30 * time.Second)
				for _, pod := range []string{"ns/app-a-1", "ns/app-a-2"} {
					tr.ObserveVersion(pod, "app-a")
					tr.Observe(workload, pod, now, v1(float64(i)/2), 256*mib, 0)
				}
				tr.Worst(workload, now)
			}
			rollout := now.Add(30 * time.Second)
			for i := 0; i < 40; i++ {
				now = rollout.Add(time.Duration(i) * 30 * time.Second)
				for _, pod := range []string{"ns/app-b-1", "ns/app-b-2"} {
					tr.ObserveVersion(pod, "app-b")
					tr.Observe(workload, pod, now, bad(float64(i)/2), 256*mib, 0)
				}
				tr.Worst(workload, now)
			}
			if est, ok := tr.Worst(workload, now); !ok || (est.Mode != ModeReleaseRegression && est.Mode != ModeSaturation) {
				t.Fatalf("leaking release not reported before rollback: %+v %v", est, ok)
			}
			// Rollback: the bad pods keep reporting memory while they terminate.
			undo := now
			for i := 1; i <= 6; i++ {
				now = undo.Add(time.Duration(i) * 30 * time.Second)
				for _, pod := range []string{"ns/app-b-1", "ns/app-b-2"} {
					tr.ObserveRetiring(workload, pod, tt.retiring)
					tr.Observe(workload, pod, now, bad(20+float64(i)/2), 256*mib, 0)
				}
				est, ok := tr.Worst(workload, now)
				leaking := ok && (est.Mode == ModeReleaseRegression || est.Mode == ModeSaturation)
				if leaking != tt.want {
					t.Fatalf("sample %d after rollback: reported %v (%+v), want %v", i, leaking, est, tt.want)
				}
			}
		})
	}
}
