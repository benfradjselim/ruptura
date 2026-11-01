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
