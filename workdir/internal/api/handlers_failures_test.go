package api

import (
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/failuremodes"
)

func TestFailureResult(t *testing.T) {
	saturation := failuremodes.Estimate{Mode: failuremodes.ModeSaturation, Pod: "n/p", ETF: 12 * time.Minute, Confidence: 0.8, Usage: 900}
	throttled := failuremodes.Estimate{Mode: failuremodes.ModeCPUThrottling, Pod: "n/p", Confidence: 1, Usage: 0.4}
	cases := []struct {
		name      string
		est       *failuremodes.Estimate
		crashLoop bool
		wantMode  string
		wantKind  string
		wantETA   int
	}{
		{"no finding", nil, false, "", "", 0},
		{"saturation is predicted", &saturation, false, "saturation", failureKindPredicted, 720},
		{"throttling is detected", &throttled, false, "cpu_throttling", failureKindDetected, 0},
		{"crash loop is detected", nil, true, failureModeCrashLoop, failureKindDetected, 0},
		{"crash loop outranks a forecast", &saturation, true, failureModeCrashLoop, failureKindDetected, 0},
	}
	for _, c := range cases {
		got := failureResult("n/Deployment/w", "critical", c.est, c.crashLoop)
		if c.wantMode == "" {
			if got != nil {
				t.Errorf("%s: got %+v, want none", c.name, got)
			}
			continue
		}
		if got == nil {
			t.Fatalf("%s: got none", c.name)
		}
		if got.FailureMode != c.wantMode || got.Kind != c.wantKind || got.ETASeconds != c.wantETA {
			t.Errorf("%s: got %+v", c.name, got)
		}
		if got.Level != "workload" || got.Target != "n/Deployment/w" || got.State != "critical" || got.RecommendedAction == "" {
			t.Errorf("%s: incomplete result %+v", c.name, got)
		}
	}
}

func TestApplyEstimate_UsesHorizon(t *testing.T) {
	est := failuremodes.Estimate{Mode: failuremodes.ModeSaturation, ETF: 40 * time.Minute}
	cases := []struct {
		name    string
		horizon time.Duration
		want    string
	}{
		{"inside a long horizon", time.Hour, "degraded"},
		{"outside the default horizon", 30 * time.Minute, "healthy"},
		{"unset uses the default", 0, "healthy"},
	}
	for _, c := range cases {
		h := &Handlers{}
		h.SetHorizon(c.horizon)
		if got, _ := applyEstimate("healthy", 0, est, h.warningHorizon()); got != c.want {
			t.Errorf("%s: state = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestScopeResult(t *testing.T) {
	base := FailureResult{Level: failureLevelWorkload, Target: "n/Deployment/w", Pod: "n/p"}
	cases := []struct {
		scope      string
		wantLevel  string
		wantTarget string
	}{
		{"", "workload", "n/Deployment/w"},
		{"workload", "workload", "n/Deployment/w"},
		{"pod", "pod", "n/p"},
		{"namespace", "namespace", "n"},
	}
	for _, c := range cases {
		got := scopeResult(base, c.scope)
		if got.Level != c.wantLevel || got.Target != c.wantTarget {
			t.Errorf("scope %q: got %+v", c.scope, got)
		}
	}
	noPod := FailureResult{Level: failureLevelWorkload, Target: "n/Deployment/w"}
	if got := scopeResult(noPod, "pod"); got.Level != "workload" {
		t.Errorf("a result without a pod must stay at workload level: %+v", got)
	}
}
