package api

import (
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/failuremodes"
	"github.com/benfradjselim/ruptura/internal/telemetry"
)

type fixedEstimates map[string]failuremodes.Estimate

func (f fixedEstimates) Worst(workload string, _ time.Time) (failuremodes.Estimate, bool) {
	est, ok := f[workload]
	return est, ok
}

func TestApplyEstimate(t *testing.T) {
	tests := []struct {
		name      string
		state     string
		etf       time.Duration
		wantState string
		wantRisk  bool
	}{
		{"distant failure keeps state", "healthy", 3 * time.Hour, "healthy", false},
		{"near failure degrades", "healthy", 20 * time.Minute, "degraded", true},
		{"imminent failure is critical", "degraded", 5 * time.Minute, "critical", true},
		{"calibrating workload still warns", "calibrating", 20 * time.Minute, "degraded", true},
		{"critical is never lowered", "critical", 20 * time.Minute, "critical", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state, risk := applyEstimate(tc.state, 0, failuremodes.Estimate{ETF: tc.etf}, degradedETF)
			if state != tc.wantState {
				t.Errorf("state = %q, want %q", state, tc.wantState)
			}
			if (risk >= 1.5) != tc.wantRisk {
				t.Errorf("risk = %v, want alerting %v", risk, tc.wantRisk)
			}
		})
	}
}

func (f fixedEstimates) RecentRestarts(workload string, _ time.Time, _ time.Duration) int {
	if workload == "demo/Deployment/looping" {
		return 3
	}
	return 1
}

func TestStateRestarts_PrefersRecentCount(t *testing.T) {
	h := New(nil, nil, telemetry.NewRegistry("test"), telemetry.NewHealthChecker(), "key")
	if got := h.stateRestarts("demo/Deployment/orders", 9); got != 9 {
		t.Fatalf("without tracking = %d, want the lifetime count", got)
	}
	h.SetFailureModes(fixedEstimates{})
	if got := h.stateRestarts("demo/Deployment/orders", 9); got != 0 {
		t.Fatalf("one recent restart = %d, want 0", got)
	}
	if got := h.stateRestarts("demo/Deployment/looping", 0); got < crashLoopRestartThreshold {
		t.Fatalf("three recent restarts = %d, want a crash loop", got)
	}
}

func TestApplyEstimate_ThrottlingIsDegradedNeverCritical(t *testing.T) {
	tests := []struct {
		name      string
		state     string
		wantState string
	}{
		{"healthy becomes degraded", "healthy", "degraded"},
		{"calibrating becomes degraded", "calibrating", "degraded"},
		{"critical is kept", "critical", "critical"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state, risk := applyEstimate(tc.state, 0, failuremodes.Estimate{Mode: failuremodes.ModeCPUThrottling, Usage: 0.4}, degradedETF)
			if state != tc.wantState {
				t.Errorf("state = %q, want %q", state, tc.wantState)
			}
			if risk < 1.5 {
				t.Errorf("risk = %v, want alerting", risk)
			}
		})
	}
}

func TestApplyCrashLoop_ReportsRiskNine(t *testing.T) {
	tests := []struct {
		name      string
		state     string
		risk      float64
		restarts  int
		wantState string
		wantRisk  float64
	}{
		{"detected loop gets risk 9", "critical", 0, crashLoopRestartThreshold, "critical", 9},
		{"higher risk is kept", "critical", 9.5, crashLoopRestartThreshold, "critical", 9.5},
		{"no loop leaves risk", "healthy", 0, 0, "healthy", 0},
		{"below threshold leaves risk", "degraded", 2, crashLoopRestartThreshold - 1, "degraded", 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state, risk := applyCrashLoop(tc.state, tc.risk, tc.restarts)
			if state != tc.wantState || risk != tc.wantRisk {
				t.Errorf("got (%q, %v), want (%q, %v)", state, risk, tc.wantState, tc.wantRisk)
			}
		})
	}
}
