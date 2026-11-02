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

	eventSeen   time.Time
	eventReason string
	lastOOM     bool
}

// Tracker keeps one estimator per pod and answers per workload. It is safe for
// concurrent use.
type Tracker struct {
	mu        sync.RWMutex
	pods      map[string]*podState
	replicas  map[string]replicaState
	rollouts  map[string]time.Time
	failures  map[string][]time.Time
	forecasts map[string]time.Time
	outcomes  []failureOutcome
	recorder  func(workload string, at time.Time)
}

// NewTracker returns an empty tracker.
func NewTracker() *Tracker {
	return &Tracker{pods: make(map[string]*podState), replicas: make(map[string]replicaState), rollouts: make(map[string]time.Time), failures: make(map[string][]time.Time), forecasts: make(map[string]time.Time)}
}

func (s *podState) recordRestarts(at time.Time, restarts int) bool {
	grew := s.counted && restarts > s.restarts
	if grew {
		s.history = append(s.history, at)
		if len(s.history) > maxRestartHistory {
			s.history = s.history[len(s.history)-maxRestartHistory:]
		}
	}
	s.restarts, s.counted = restarts, true
	return grew
}

// recordFailure keeps failure times per workload, because pods are replaced.
func (t *Tracker) recordFailure(workload string, at time.Time) {
	history := append(t.failures[workload], at)
	if len(history) > maxRestartHistory {
		history = history[len(history)-maxRestartHistory:]
	}
	t.failures[workload] = history
	t.recordOutcome(workload, at)
	if t.recorder != nil {
		t.recorder(workload, at)
	}
}

// SetFailureRecorder sets a callback that receives each new failure.
func (t *Tracker) SetFailureRecorder(recorder func(workload string, at time.Time)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.recorder = recorder
}

// RestoreFailures loads failure times saved by an earlier run, oldest first.
func (t *Tracker) RestoreFailures(workload string, times []time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.failures[workload] = append([]time.Time(nil), times...)
}

// ObserveRestarts records the restart count of a pod with no usage reading,
// which is the normal case for a pod in CrashLoopBackOff.
func (t *Tracker) ObserveRestarts(workload, pod string, at time.Time, restarts int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	state := t.podLocked(workload, pod)
	if state.recordRestarts(at, restarts) {
		t.recordFailure(workload, at)
	}
	state.seen = at
}

// Observe records one reading for a pod that belongs to workload.
func (t *Tracker) Observe(workload, pod string, at time.Time, usage, capacity float64, restarts int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	state := t.podLocked(workload, pod)
	if state.detector == nil {
		state.detector = NewSaturation()
	}
	if state.recordRestarts(at, restarts) {
		t.recordFailure(workload, at)
	}
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
	for pod, state := range t.pods {
		if state.workload != workload {
			continue
		}
		if state.valid && now.Sub(state.seen) <= estimateMaxAge {
			consider(state.estimate)
		}
		if state.condValid && now.Sub(state.condSeen) <= estimateMaxAge {
			consider(state.condEstimate)
		}
		if est, ok := state.oomEstimate(pod, now); ok {
			consider(est)
		}
		if !state.eventSeen.IsZero() && now.Sub(state.eventSeen) <= startupEventMaxAge {
			consider(Estimate{Mode: ModeStartupStuck, Pod: pod, Confidence: 1, Reason: state.eventReason})
		}
	}
	if est, ok := t.replicaEstimate(workload, now); ok {
		consider(est)
	}
	if est, ok := t.rolloutEstimate(workload, now); ok {
		consider(est)
	}
	if est, ok := t.recurrenceEstimate(workload, now); ok {
		consider(est)
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
		if state.seen.Before(cutoff) && state.condSeen.Before(cutoff) && state.eventSeen.Before(cutoff) {
			delete(t.pods, pod)
		}
	}
}
