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
	restarts int
	counted  bool
	history  []time.Time
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
	state, ok := t.pods[pod]
	if !ok {
		state = &podState{workload: workload, detector: NewSaturation()}
		t.pods[pod] = state
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

// Worst returns the shortest fresh time to failure across a workload's pods.
func (t *Tracker) Worst(workload string, now time.Time) (Estimate, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var worst Estimate
	found := false
	for _, state := range t.pods {
		if state.workload != workload || !state.valid || now.Sub(state.seen) > estimateMaxAge {
			continue
		}
		if !found || state.estimate.ETF < worst.ETF {
			worst, found = state.estimate, true
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
		if state.seen.Before(cutoff) {
			delete(t.pods, pod)
		}
	}
}
