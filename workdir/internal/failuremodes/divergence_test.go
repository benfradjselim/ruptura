package failuremodes

import (
	"testing"
	"time"
)

// runReplicas feeds 20 flat minutes to every pod of one version, then makes the
// pods in leaking grow at leak MiB per minute, and returns the first divergence.
func runReplicas(t *testing.T, pods []string, leaking map[string]bool, leak float64) (Estimate, float64, bool) {
	t.Helper()
	tr := NewTracker()
	start := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 200; i++ {
		now := start.Add(time.Duration(i) * 30 * time.Second)
		minutes := float64(i) / 2
		for _, pod := range pods {
			usage := 60.0
			if leaking[pod] && minutes > 20 {
				usage += leak * (minutes - 20)
			}
			tr.ObserveVersion(pod, "app-a")
			tr.Observe("ns/Deployment/app", pod, now, usage*mib, 256*mib, 0)
		}
		if est, ok := tr.Worst("ns/Deployment/app", now); ok && est.Mode == ModeReplicaDivergence {
			return est, minutes - 20, true
		}
	}
	return Estimate{}, 0, false
}

func TestReplicaDivergence(t *testing.T) {
	three := []string{"ns/app-a-1", "ns/app-a-2", "ns/app-a-3"}
	cases := []struct {
		name    string
		pods    []string
		leaking map[string]bool
		leak    float64
		want    bool
	}{
		{"one replica leaks", three, map[string]bool{"ns/app-a-2": true}, 3, true},
		{"all replicas leak together", three, map[string]bool{"ns/app-a-1": true, "ns/app-a-2": true, "ns/app-a-3": true}, 3, false},
		{"two replicas only", three[:2], map[string]bool{"ns/app-a-2": true}, 3, false},
		{"no replica leaks", three, nil, 0, false},
		{"leak too slow for the horizon", three, map[string]bool{"ns/app-a-2": true}, 0.8, false},
	}
	for _, tc := range cases {
		est, at, got := runReplicas(t, tc.pods, tc.leaking, tc.leak)
		if got != tc.want {
			t.Errorf("%s: reported = %v (%+v at %.1f min)", tc.name, got, est, at)
			continue
		}
		if !got {
			continue
		}
		if est.Pod != "ns/app-a-2" || at > 15 || est.Siblings != 2 || est.ETF <= 0 || est.ETF > releaseHorizon {
			t.Errorf("%s: estimate %+v at %.1f min", tc.name, est, at)
		}
		if est.ETFEarly <= 0 || est.ETFEarly > est.ETF || est.ETFLate < est.ETF {
			t.Errorf("%s: band %v to %v does not surround %v", tc.name, est.ETFEarly, est.ETFLate, est.ETF)
		}
	}
}
