package failuremodes

import "time"

// ModeRolloutStuck is a rollout that Kubernetes reports as no longer progressing.
const ModeRolloutStuck Mode = "rollout_stuck"

// ModeOOMKilled is a container killed for memory, detected after the fact.
const ModeOOMKilled Mode = "oom_killed"

const (
	rolloutUrgency = 3 * time.Minute
	// oomUrgency ranks a past OOM behind any forecast, so that a leak which
	// restarts keeps showing the next failure.
	oomUrgency = 24 * time.Hour
	// oomRecentWindow keeps an OOM visible. The metric stays set after the
	// incident, so only a restart inside this window counts.
	oomRecentWindow = 30 * time.Minute
)

// ObserveRollout records whether a workload rollout has stalled.
func (t *Tracker) ObserveRollout(workload string, stalled bool, at time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !stalled {
		delete(t.rollouts, workload)
		return
	}
	t.rollouts[workload] = at
}

func (t *Tracker) rolloutEstimate(workload string, now time.Time) (Estimate, bool) {
	seen, ok := t.rollouts[workload]
	if !ok || now.Sub(seen) > estimateMaxAge {
		return Estimate{}, false
	}
	return Estimate{Mode: ModeRolloutStuck, Confidence: 1}, true
}

// ObserveOOM records whether the last termination of a pod was an OOM kill.
func (t *Tracker) ObserveOOM(workload, pod string, oom bool, at time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.podLocked(workload, pod).lastOOM = oom
}

func (s *podState) oomEstimate(pod string, now time.Time) (Estimate, bool) {
	if !s.lastOOM || len(s.history) == 0 {
		return Estimate{}, false
	}
	if now.Sub(s.history[len(s.history)-1]) > oomRecentWindow {
		return Estimate{}, false
	}
	return Estimate{Mode: ModeOOMKilled, Pod: pod, Confidence: 1}, true
}

// OOMReason marks a recurrent failure whose last exits were OOM kills.
const OOMReason = "OOMKilled"

// lastExitOOM reports whether a pod of workload last exited on an OOM kill. The
// caller holds the lock.
func (t *Tracker) lastExitOOM(workload string) bool {
	for _, state := range t.pods {
		if state.workload == workload && state.lastOOM {
			return true
		}
	}
	return false
}
