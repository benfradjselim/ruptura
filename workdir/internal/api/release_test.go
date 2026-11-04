package api

import (
	"strings"
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/failuremodes"
)

func TestReleaseRegressionReporting(t *testing.T) {
	est := failuremodes.Estimate{Mode: failuremodes.ModeReleaseRegression, ETF: 50 * time.Minute, Usage: 0.4,
		Version: "app-b", PreviousVersion: "app-a", Rate: 3 * 1024 * 1024 / 60.0, PreviousRate: 0}
	cases := []struct {
		state, want string
		etf         time.Duration
	}{
		{"healthy", "degraded", 50 * time.Minute},
		{"critical", "critical", 50 * time.Minute},
		{"healthy", "critical", time.Minute},
	}
	for _, tc := range cases {
		est.ETF = tc.etf
		if got, _ := applyEstimate(tc.state, 0, est, 30*time.Minute); got != tc.want {
			t.Errorf("state %s, ETF %v: got %s, want %s", tc.state, tc.etf, got, tc.want)
		}
	}
	est.ETF = 50 * time.Minute
	if reportedEstimate(est, 30*time.Minute) == nil {
		t.Error("a regression within its horizon must be reported")
	}
	evidence := strings.Join(failureEvidence(est), " | ")
	if !strings.Contains(evidence, "Release app-b grows 3.0 MiB per minute, release app-a grew 0.0") {
		t.Errorf("evidence = %q", evidence)
	}
	if action := estimateAction(est); !strings.Contains(action, "Roll back to app-a") {
		t.Errorf("action = %q", action)
	}
	if headline := estimateHeadline("app", est); !strings.Contains(headline, "release app-b") {
		t.Errorf("headline = %q", headline)
	}
}
