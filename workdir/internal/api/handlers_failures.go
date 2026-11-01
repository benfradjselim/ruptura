package api

import (
	"net/http"

	"github.com/benfradjselim/ruptura/internal/failuremodes"
)

const (
	failureKindPredicted = "predicted"
	failureKindDetected  = "detected"
	failureModeCrashLoop = "crash_loop"
	failureLevelWorkload = "workload"
)

// FailureResult is the structured conclusion of the engine for one target.
type FailureResult struct {
	Level             string  `json:"level"`
	Target            string  `json:"target"`
	Pod               string  `json:"pod,omitempty"`
	FailureMode       string  `json:"failureMode"`
	Kind              string  `json:"kind"`
	State             string  `json:"state"`
	ETASeconds        int     `json:"etaSeconds,omitempty"`
	Confidence        float64 `json:"confidence,omitempty"`
	RecommendedAction string  `json:"recommendedAction"`
}

// failureResult builds the result for a workload. It returns nil when no failure
// is known, so an anomaly is never presented as a diagnosed failure.
func failureResult(workload, state string, est *failuremodes.Estimate, crashLoop bool) *FailureResult {
	result := FailureResult{Level: failureLevelWorkload, Target: workload, State: state}
	switch {
	case crashLoop:
		result.FailureMode = failureModeCrashLoop
		result.Kind = failureKindDetected
		result.RecommendedAction = "Read the logs of the previous container and fix the startup failure"
	case est == nil:
		return nil
	default:
		result.FailureMode = string(est.Mode)
		result.Pod = est.Pod
		result.Confidence = est.Confidence
		result.RecommendedAction = estimateAction(*est)
		result.Kind = failureKindDetected
		if est.Mode == failuremodes.ModeSaturation {
			result.Kind = failureKindPredicted
			result.ETASeconds = int(est.ETF.Seconds())
		}
	}
	return &result
}

func (h *Handlers) handleFailures(w http.ResponseWriter, _ *http.Request) {
	results := []*FailureResult{}
	for _, host := range h.buildFleet().Hosts {
		if host.Failure != nil {
			results = append(results, host.Failure)
		}
	}
	writeJSON(w, http.StatusOK, results)
}
