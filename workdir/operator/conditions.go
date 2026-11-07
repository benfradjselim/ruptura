package main

import "time"

// Condition follows the shape of metav1.Condition, which OpenShift console,
// Argo CD and kstatus read.
type Condition struct {
	Type               string `json:"type"`
	Status             string `json:"status"`
	Reason             string `json:"reason"`
	Message            string `json:"message"`
	LastTransitionTime string `json:"lastTransitionTime"`
	ObservedGeneration int64  `json:"observedGeneration,omitempty"`
}

const (
	conditionAvailable   = "Available"
	conditionProgressing = "Progressing"
	conditionDegraded    = "Degraded"
)

// conditions derives the standard conditions from the phase. A condition keeps
// its lastTransitionTime while its status does not change.
func conditions(previous []Condition, phase, message string, ready int32, generation int64, now time.Time) []Condition {
	type want struct{ kind, status, reason, message string }
	var wants []want
	switch phase {
	case "Running":
		wants = []want{
			{conditionAvailable, "True", "EngineReady", "The engine is ready"},
			{conditionProgressing, "False", "Reconciled", "The instance matches its spec"},
			{conditionDegraded, "False", "Reconciled", ""},
		}
	case "Pending":
		wants = []want{
			{conditionAvailable, "False", "EngineNotReady", message},
			{conditionProgressing, "True", "Deploying", message},
			{conditionDegraded, "False", "Deploying", ""},
		}
	case "Degraded":
		available, reason := "False", "EngineNotReady"
		if ready > 0 {
			available, reason = "True", "PreviousEngineServing"
		}
		wants = []want{
			{conditionAvailable, available, reason, message},
			{conditionProgressing, "False", "ChangeNotApplied", message},
			{conditionDegraded, "True", "ChangeNotApplied", message},
		}
	default:
		wants = []want{
			{conditionAvailable, "False", "InvalidSpec", message},
			{conditionProgressing, "False", "InvalidSpec", message},
			{conditionDegraded, "True", "InvalidSpec", message},
		}
	}
	stamp := now.UTC().Format(time.RFC3339)
	out := make([]Condition, 0, len(wants))
	for _, w := range wants {
		since := stamp
		for _, p := range previous {
			if p.Type == w.kind && p.Status == w.status && p.LastTransitionTime != "" {
				since = p.LastTransitionTime
			}
		}
		out = append(out, Condition{Type: w.kind, Status: w.status, Reason: w.reason, Message: w.message, LastTransitionTime: since, ObservedGeneration: generation})
	}
	return out
}
