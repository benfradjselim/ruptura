package failuremodes

import (
	"sync"
	"time"
)

// estimateMaxAge is how long an estimate stays valid without a new reading.
const estimateMaxAge = 3 * time.Minute

// maxRestartHistory bounds the restart times kept per pod.
const maxRestartHistory = 64

type podState struct {
	workload string
	detector *Saturation
	estimate Estimate
	valid    bool
	seen     time.Time

	condition    *Condition
	condEstimate Estimate
	condValid    bool
	condSeen     time.Time
	restarts     int
	counted      bool
	history      []time.Time
}

// Tracker keeps one estimator per pod and answers per workload. It is safe for
// concurrent use.
type Tracker struct {
	mu   sync.RWMutex
	pods map[string]*podState
}

// NewTracker returns an empty tracker.
func NewTracker() *Tracker { return &Tracker{pods: make(map[string]*podState)} }

// Observe records one reading for a pod that belongs to workload.
func (t *Tracker) Observe(workload, pod string, at time.Time, usage, capacity float64, restarts int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	state := t.podLocked(workload, pod)
	if state.detector == nil {
		state.detector = NewSaturation()
	}
	if state.counted && restarts > state.restarts {
		state.history = append(state.history, at)
		if len(state.history) > maxRestartHistory {
			state.history = state.history[len(state.history)-maxRestartHistory:]
		}
	}
	state.restarts, state.counted = restarts, true
	state.estimate, state.valid = state.detector.Observe(at, usage, capacity, restarts)
	state.estimate.Pod = pod
	state.seen = at
}

func (t *Tracker) podLocked(workload, pod string) *podState {
	state, ok := t.pods[pod]
	if !ok {
		state = &podState{workload: workload}
		t.pods[pod] = state
	}
	return state
}

// ObserveCondition records one throttling ratio for a pod that belongs to
// workload.
func (t *Tracker) ObserveCondition(workload, pod string, at time.Time, ratio float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	state := t.podLocked(workload, pod)
	if state.condition == nil {
		state.condition = NewCondition()
	}
	state.condEstimate, state.condValid = state.condition.Observe(at, ratio)
	state.condEstimate.Pod = pod
	state.condSeen = at
}

// Worst returns the most urgent fresh estimate across a workload's pods and
// modes. An imminent failure outranks a condition already in progress, which
// outranks a slower projection.
func (t *Tracker) Worst(workload string, now time.Time) (Estimate, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var worst Estimate
	found := false
	consider := func(est Estimate) {
		if !found || est.urgency() < worst.urgency() {
			worst, found = est, true
		}
	}
	for _, state := range t.pods {
		if state.workload != workload {
			continue
		}
		if state.valid && now.Sub(state.seen) <= estimateMaxAge {
			consider(state.estimate)
		}
		if state.condValid && now.Sub(state.condSeen) <= estimateMaxAge {
			consider(state.condEstimate)
		}
	}
	return worst, found
}

// RecentRestarts counts restarts observed for a workload inside the window.
// Counts that predate the first observation are history and are not included.
func (t *Tracker) RecentRestarts(workload string, now time.Time, window time.Duration) int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	total := 0
	for _, state := range t.pods {
		if state.workload != workload {
			continue
		}
		for _, at := range state.history {
			if now.Sub(at) <= window {
				total++
			}
		}
	}
	return total
}

// Prune forgets pods that have not been seen since the cutoff.
func (t *Tracker) Prune(cutoff time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for pod, state := range t.pods {
		if state.seen.Before(cutoff) && state.condSeen.Before(cutoff) {
			delete(t.pods, pod)
		}
	}
}
