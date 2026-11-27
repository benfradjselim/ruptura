package api

import (
	"strings"
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/failuremodes"
)

func TestMemoryBeyondUsual_NeverCriticalNorOOM(t *testing.T) {
	est := failuremodes.Estimate{Mode: failuremodes.ModeMemoryBeyondUsual, Pod: "shop/cache-1", ETF: 3 * time.Minute, Usage: 0.95,
		Ceiling: 600 * 1024 * 1024, CeilingSource: "7-day P99 + 20%"}
	tests := []struct {
		name, state, want string
		etf               time.Duration
	}{
		{"imminent stays degraded", "healthy", "degraded", 3 * time.Minute},
		{"critical is kept", "critical", "critical", 3 * time.Minute},
		{"far beyond the horizon is ignored", "healthy", "healthy", 48 * time.Hour},
	}
	for _, tt := range tests {
		e := est
		e.ETF = tt.etf
		if state, _ := applyEstimate(tt.state, 0, e, degradedETF); state != tt.want {
			t.Errorf("%s: state = %q, want %q", tt.name, state, tt.want)
		}
	}
	if reportedEstimate(failuremodes.Estimate{Mode: failuremodes.ModeMemoryBeyondUsual, ETF: 48 * time.Hour}, degradedETF) != nil {
		t.Error("a ceiling two days away is reported")
	}

	result := failureResult("shop/Deployment/cache", "degraded", &est, false)
	if result.FailureMode != "memory_beyond_usual" || result.Kind != failureKindPredicted || result.ETASeconds != 180 {
		t.Fatalf("result = %+v", result)
	}
	text := strings.Join(append(result.Evidence, result.RecommendedAction), " | ")
	for _, want := range []string{"No memory limit set", "600 MiB", "7-day P99 + 20%", "Set a memory limit"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in %q", want, text)
		}
	}
	if strings.Contains(strings.ToLower(text), "oom") || strings.Contains(text, "killed") {
		t.Errorf("text announces a kill: %q", text)
	}
}

func TestReplicasUnavailable_Unschedulable(t *testing.T) {
	est := failuremodes.Estimate{Mode: failuremodes.ModeReplicasUnavailable, Pod: "shop/det-replicas-2", Reason: "FailedScheduling", Usage: 1, Confidence: 1}
	result := failureResult("shop/Deployment/det-replicas", "degraded", &est, false)
	text := strings.Join(append(result.Evidence, result.RecommendedAction), " | ")
	for _, want := range []string{"1 replica unavailable", "FailedScheduling for pod det-replicas-2", "scheduling constraints"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in %q", want, text)
		}
	}
	oom := failuremodes.Estimate{Mode: failuremodes.ModeOOMKilled, Pod: "shop/det-oom-1", Confidence: 1}
	if got := failureResult("shop/Deployment/det-oom", "critical", &oom, true); got.FailureMode != "oom_killed" {
		t.Errorf("crash loop caused by OOM reported as %q", got.FailureMode)
	}
}
