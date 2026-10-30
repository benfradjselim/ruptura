package failuremodes

import (
	"testing"
	"time"
)

func TestTracker_WorstReturnsShortestFreshETF(t *testing.T) {
	start := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		query   time.Time
		wantOK  bool
		wantPod string
	}{
		{"fresh estimates pick the fastest pod", start.Add(10 * time.Minute), true, "fast"},
		{"stale estimates are dropped", start.Add(30 * time.Minute), false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tracker := NewTracker()
			for i := 0; i <= 10; i++ {
				at := start.Add(time.Duration(i) * time.Minute)
				tracker.Observe("ns/Deployment/app", "slow", at, 100+float64(i), 1000, 0)
				tracker.Observe("ns/Deployment/app", "fast", at, 100+float64(i)*50, 1000, 0)
			}
			est, ok := tracker.Worst("ns/Deployment/app", tc.query)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && est.Pod != tc.wantPod {
				t.Fatalf("pod = %q, want %q", est.Pod, tc.wantPod)
			}
		})
	}
}
