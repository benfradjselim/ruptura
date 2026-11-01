package failuremodes

import "time"

// ModeReplicasUnavailable is a workload missing replicas for a sustained time.
// It is detected, not predicted.
const ModeReplicasUnavailable Mode = "replicas_unavailable"

// replicasSustained separates a failure from a normal rollout gap.
const replicasSustained = 5 * time.Minute

// replicasUrgency ranks missing replicas above a throttling condition.
const replicasUrgency = 2 * time.Minute

type replicaState struct {
	missing int
	since   time.Time
	seen    time.Time
}

// ObserveReplicas records the unavailable replicas of a workload.
func (t *Tracker) ObserveReplicas(workload string, missing int, at time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	state := t.replicas[workload]
	if missing <= 0 {
		delete(t.replicas, workload)
		return
	}
	if state.since.IsZero() {
		state.since = at
	}
	state.missing, state.seen = missing, at
	t.replicas[workload] = state
}

func (t *Tracker) replicaEstimate(workload string, now time.Time) (Estimate, bool) {
	state, ok := t.replicas[workload]
	if !ok || now.Sub(state.seen) > estimateMaxAge || state.seen.Sub(state.since) < replicasSustained {
		return Estimate{}, false
	}
	return Estimate{Mode: ModeReplicasUnavailable, Confidence: 1, Usage: float64(state.missing)}, true
}
