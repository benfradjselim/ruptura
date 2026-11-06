package failuremodes

import (
	"testing"
	"time"
)

func TestCrashBackOff(t *testing.T) {
	at := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name, reason, message string
		now                   time.Time
		want                  bool
	}{
		{"back-off restarting", "BackOff", "Back-off restarting failed container app in pod w-1", at.Add(time.Minute), true},
		{"expired", "BackOff", "Back-off restarting failed container app in pod w-1", at.Add(11 * time.Minute), false},
		{"image pull back-off is not a crash loop", "BackOff", "Back-off pulling image \"x\"", at.Add(time.Minute), false},
		{"other warning", "Unhealthy", "Readiness probe failed", at.Add(time.Minute), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := NewTracker()
			tr.ObserveEvent("n/Deployment/w", "n/w-1", tc.reason, tc.message, at)
			if got := tr.CrashBackOff("n/Deployment/w", tc.now); got != tc.want {
				t.Errorf("CrashBackOff = %v, want %v", got, tc.want)
			}
			if tr.CrashBackOff("n/Deployment/other", tc.now) {
				t.Error("another workload must not be in back-off")
			}
		})
	}
}

func TestCrashBackOffDoesNotMakeStartupStuck(t *testing.T) {
	at := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	tr := NewTracker()
	tr.ObserveEvent("n/Deployment/w", "n/w-1", "BackOff", "Back-off restarting failed container app", at)
	if est, ok := tr.Worst("n/Deployment/w", at.Add(time.Minute)); ok && est.Mode == ModeStartupStuck {
		t.Errorf("a crash back-off is not a startup failure: %+v", est)
	}
}

func TestRecurrentFailureCarriesOOMCause(t *testing.T) {
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		oom  bool
		want string
	}{
		{"OOM kills", true, OOMReason},
		{"other exits", false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := NewTracker()
			for i := 0; i <= 7; i++ {
				tr.ObserveRestarts("w", "n/p", start.Add(time.Duration(i)*11*time.Minute), i)
			}
			tr.ObserveOOM("w", "n/p", tc.oom, start.Add(77*time.Minute))
			est, ok := tr.Worst("w", start.Add(83*time.Minute))
			if !ok || est.Mode != ModeRecurrentFailure || est.Reason != tc.want {
				t.Errorf("estimate %+v, %v; want recurrent with reason %q", est, ok, tc.want)
			}
		})
	}
}
