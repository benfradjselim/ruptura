package api

import (
	"net/http"
	"strings"

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

// scopeResult reports a result at the configured level. A result with no pod
// stays at workload level, because it cannot be narrowed further.
func scopeResult(result FailureResult, scope string) FailureResult {
	switch {
	case scope == "pod" && result.Pod != "":
		result.Level, result.Target = "pod", result.Pod
	case scope == "namespace":
		result.Level, result.Target = "namespace", strings.SplitN(result.Target, "/", 2)[0]
	}
	return result
}

// ParseSelector reads "key=value,key=value". Entries without "=" are ignored.
func ParseSelector(raw string) map[string]string {
	if raw == "" {
		return nil
	}
	selector := map[string]string{}
	for _, entry := range strings.Split(raw, ",") {
		if pair := strings.SplitN(entry, "=", 2); len(pair) == 2 {
			selector[strings.TrimSpace(pair[0])] = strings.TrimSpace(pair[1])
		}
	}
	return selector
}

// SetLevelSelector limits failure results to workloads with these labels.
func (h *Handlers) SetLevelSelector(selector map[string]string) { h.levelSelector = selector }

// selectFailures keeps results whose target has every selected label. A target
// with unknown labels is left out, because it cannot be shown to match.
func selectFailures(results []*FailureResult, selector map[string]string, labelsOf func(string) (map[string]string, bool)) []*FailureResult {
	if len(selector) == 0 {
		return results
	}
	kept := []*FailureResult{}
	for _, result := range results {
		labels, ok := labelsOf(result.Target)
		if !ok {
			continue
		}
		matches := true
		for key, value := range selector {
			matches = matches && labels[key] == value
		}
		if matches {
			kept = append(kept, result)
		}
	}
	return kept
}

func (h *Handlers) workloadLabels(target string) (map[string]string, bool) {
	parts := strings.SplitN(target, "/", 3)
	if h.discovery == nil || len(parts) != 3 {
		return nil, false
	}
	meta, ok := h.discovery.GetWorkloadMeta(parts[0], parts[1], parts[2])
	return meta.Labels, ok
}

// Failures returns the known failures selected and scoped by the configuration.
func (h *Handlers) Failures() []*FailureResult {
	results := []*FailureResult{}
	for _, host := range h.buildFleet().Hosts {
		if host.Failure != nil {
			results = append(results, host.Failure)
		}
	}
	selected := selectFailures(results, h.levelSelector, h.workloadLabels)
	scoped := make([]*FailureResult, 0, len(selected))
	for _, result := range selected {
		copied := scopeResult(*result, h.levelScope)
		scoped = append(scoped, &copied)
	}
	return scoped
}

func (h *Handlers) handleFailures(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.Failures())
}
