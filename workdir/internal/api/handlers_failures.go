package api

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/benfradjselim/ruptura/internal/discovery"
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
	Level             string   `json:"level"`
	Target            string   `json:"target"`
	Pod               string   `json:"pod,omitempty"`
	FailureMode       string   `json:"failureMode"`
	Kind              string   `json:"kind"`
	State             string   `json:"state"`
	ETASeconds        int      `json:"etaSeconds,omitempty"`
	Confidence        float64  `json:"confidence,omitempty"`
	RecommendedAction string   `json:"recommendedAction"`
	Evidence          []string `json:"evidence,omitempty"`
}

// TrackRecorder reports how early failures were announced.
type TrackRecorder interface {
	NoteForecast(workload string, at time.Time)
	TrackRecord(from, to time.Time) failuremodes.TrackRecord
}

const (
	trackRecordWindow = 24 * time.Hour
	// timelineWindow is the past side of the horizon.
	timelineWindow = 30 * time.Minute
	// changeWindow is how long a change stays relevant to a failure.
	changeWindow = 2 * time.Hour
)

// TimelineEvent is a past event on the horizon.
type TimelineEvent struct {
	Target     string `json:"target"`
	Type       string `json:"type"`
	Detail     string `json:"detail,omitempty"`
	MinutesAgo int    `json:"minutesAgo"`
}

// timelineEvents merges failures and changes, oldest first.
func timelineEvents(failures map[string][]time.Time, changes []discovery.Change, now time.Time) []TimelineEvent {
	events := []TimelineEvent{}
	for target, times := range failures {
		for _, at := range times {
			events = append(events, TimelineEvent{Target: target, Type: "failure", MinutesAgo: int(now.Sub(at).Minutes())})
		}
	}
	for _, change := range changes {
		events = append(events, TimelineEvent{Target: change.Workload, Type: "change", Detail: change.Image, MinutesAgo: int(now.Sub(change.At).Minutes())})
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].MinutesAgo > events[j].MinutesAgo })
	return events
}

// changeEvidence names the latest change of a workload.
func changeEvidence(target string, changes []discovery.Change, now time.Time) string {
	for i := len(changes) - 1; i >= 0; i-- {
		if changes[i].Workload == target {
			return fmt.Sprintf("Changed %d min ago: %s", int(now.Sub(changes[i].At).Minutes()), changes[i].Image)
		}
	}
	return ""
}

// FailureHistory reports recent failures per workload.
type FailureHistory interface {
	RecentFailures(since time.Time) map[string][]time.Time
}

func (h *Handlers) handleEventHistory(w http.ResponseWriter, _ *http.Request) {
	now := time.Now()
	since := now.Add(-timelineWindow)
	var failures map[string][]time.Time
	if history, ok := h.failureModes.(FailureHistory); ok {
		failures = history.RecentFailures(since)
	}
	writeJSON(w, http.StatusOK, timelineEvents(failures, h.discovery.RecentChanges(since), now))
}

func replicaCount(n int) string {
	if n == 1 {
		return "1 replica"
	}
	return fmt.Sprintf("%d replicas", n)
}

// failureEvidence states the measurement behind a result, in plain words.
func failureEvidence(est failuremodes.Estimate) []string {
	switch est.Mode {
	case failuremodes.ModeSaturation:
		return []string{fmt.Sprintf("Memory at %d%% of its limit", int(math.Round(est.Usage*100)))}
	case failuremodes.ModeCPUThrottling:
		return []string{fmt.Sprintf("CPU throttled %d%% of the time", int(math.Round(est.Usage*100)))}
	case failuremodes.ModeStartupStuck:
		return []string{"Kubernetes reports " + est.Reason}
	case failuremodes.ModeReplicasUnavailable:
		return []string{fmt.Sprintf("%s unavailable for over 5 minutes", replicaCount(int(est.Usage)))}
	case failuremodes.ModeRecurrentFailure:
		return []string{fmt.Sprintf("%d failures, about every %d min", est.Failures, int(math.Round(est.Interval.Minutes())))}
	case failuremodes.ModeRolloutStuck:
		return []string{"Rollout passed its progress deadline"}
	case failuremodes.ModeReplicaDivergence:
		mib := func(bps float64) float64 { return math.Round(bps*60/1024/1024*10) / 10 }
		return []string{
			fmt.Sprintf("Pod grows %.1f MiB per minute, its %d replicas %.1f", mib(est.Rate), est.Siblings, mib(est.PreviousRate)),
			fmt.Sprintf("Memory at %d%% of its limit", int(math.Round(est.Usage*100))),
		}
	case failuremodes.ModeReleaseRegression:
		mib := func(bps float64) float64 { return math.Round(bps*60/1024/1024*10) / 10 }
		return []string{
			fmt.Sprintf("Release %s grows %.1f MiB per minute, release %s grew %.1f", est.Version, mib(est.Rate), est.PreviousVersion, mib(est.PreviousRate)),
			fmt.Sprintf("Memory at %d%% of its limit", int(math.Round(est.Usage*100))),
		}
	case failuremodes.ModeOOMKilled:
		return []string{"Last exit was OOMKilled"}
	}
	return nil
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
		result.Evidence = []string{"Repeated restarts in the last 30 minutes"}
	case est == nil:
		return nil
	default:
		result.FailureMode = string(est.Mode)
		result.Pod = est.Pod
		result.Confidence = est.Confidence
		result.RecommendedAction = estimateAction(*est)
		result.Evidence = failureEvidence(*est)
		result.Kind = failureKindDetected
		switch est.Mode {
		case failuremodes.ModeSaturation, failuremodes.ModeRecurrentFailure, failuremodes.ModeReleaseRegression, failuremodes.ModeReplicaDivergence:
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
	now := time.Now()
	changes := h.discovery.RecentChanges(now.Add(-changeWindow))
	for _, result := range selected {
		copied := *result
		if evidence := changeEvidence(result.Target, changes, now); evidence != "" {
			copied.Evidence = append(append([]string{}, result.Evidence...), evidence)
		}
		copied = scopeResult(copied, h.levelScope)
		scoped = append(scoped, &copied)
	}
	return scoped
}

func (h *Handlers) handleFailures(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.Failures())
}

type trackRecordResponse struct {
	WindowHours       int `json:"windowHours"`
	Failures          int `json:"failures"`
	Warned            int `json:"warned"`
	MedianLeadSeconds int `json:"medianLeadSeconds"`
}

func (h *Handlers) handleTrackRecord(w http.ResponseWriter, _ *http.Request) {
	response := trackRecordResponse{WindowHours: int(trackRecordWindow.Hours())}
	if recorder, ok := h.failureModes.(TrackRecorder); ok {
		now := time.Now()
		record := recorder.TrackRecord(now.Add(-trackRecordWindow), now)
		response.Failures, response.Warned = record.Failures, record.Warned
		response.MedianLeadSeconds = int(record.MedianLead.Seconds())
	}
	writeJSON(w, http.StatusOK, response)
}
