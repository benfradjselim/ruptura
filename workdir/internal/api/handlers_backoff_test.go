package api

import (
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/failuremodes"
)

func TestFailureResult_OOMLoop(t *testing.T) {
	rhythmOOM := failuremodes.Estimate{Mode: failuremodes.ModeRecurrentFailure, ETF: 6 * time.Minute, Confidence: 0.9, Failures: 8, Interval: 11 * time.Minute, Reason: failuremodes.OOMReason}
	rhythm := failuremodes.Estimate{Mode: failuremodes.ModeRecurrentFailure, ETF: 6 * time.Minute, Confidence: 0.9, Failures: 8, Interval: 11 * time.Minute}
	oom := failuremodes.Estimate{Mode: failuremodes.ModeOOMKilled, Pod: "n/p", Confidence: 1}
	tests := []struct {
		name     string
		est      *failuremodes.Estimate
		wantMode string
		wantKind string
		wantETA  int
	}{
		{"OOM rhythm forecasts the next kill", &rhythmOOM, "recurrent_failure", failureKindPredicted, 360},
		{"OOM without rhythm is diagnosed as OOM", &oom, "oom_killed", failureKindDetected, 0},
		{"other crash loop stays a crash loop", &rhythm, failureModeCrashLoop, failureKindDetected, 0},
	}
	for _, tc := range tests {
		got := failureResult("n/Deployment/w", "critical", tc.est, true)
		if got == nil || got.FailureMode != tc.wantMode || got.Kind != tc.wantKind || got.ETASeconds != tc.wantETA {
			t.Errorf("%s: got %+v", tc.name, got)
		}
	}
	got := failureResult("n/Deployment/w", "critical", &rhythmOOM, true)
	if len(got.Evidence) != 2 || got.Evidence[1] != "Last exit was OOMKilled" || got.RecommendedAction != "Raise the memory limit or find the memory spike before the next OOM kill" {
		t.Errorf("OOM rhythm evidence %v, action %q", got.Evidence, got.RecommendedAction)
	}
}

type backOffModes struct{ backOff bool }

func (b backOffModes) Worst(string, time.Time) (failuremodes.Estimate, bool) {
	return failuremodes.Estimate{}, false
}
func (b backOffModes) RecentRestarts(string, time.Time, time.Duration) int { return 0 }
func (b backOffModes) CrashBackOff(string, time.Time) bool                 { return b.backOff }

func TestStateRestarts_CrashBackOff(t *testing.T) {
	tests := []struct {
		backOff bool
		want    int
	}{
		{true, crashLoopRestartThreshold},
		{false, 0},
	}
	for _, tc := range tests {
		h := &Handlers{failureModes: backOffModes{tc.backOff}}
		if got := h.stateRestarts("n/Deployment/w", 0); got != tc.want {
			t.Errorf("back-off %v: got %d, want %d", tc.backOff, got, tc.want)
		}
	}
}
