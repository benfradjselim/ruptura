package failuremodes

import "time"

// ModeStartupStuck is a pod that cannot start. It is detected, not predicted.
const ModeStartupStuck Mode = "startup_stuck"

// startupEventMaxAge is how long a startup failure event stays valid.
const startupEventMaxAge = 5 * time.Minute

// startupStuckUrgency ranks a pod that cannot start as already failing.
const startupStuckUrgency = time.Minute

var startupFailureReasons = map[string]bool{
	"FailedMount":            true,
	"FailedAttachVolume":     true,
	"FailedScheduling":       true,
	"FailedCreatePodSandBox": true,
	"ErrImagePull":           true,
	"ImagePullBackOff":       true,
	"InvalidImageName":       true,
	"FailedCreate":           true,
}

// ObserveEvent records a Kubernetes warning event for a pod of workload.
// Reasons that do not block a start are ignored.
func (t *Tracker) ObserveEvent(workload, pod, reason string, at time.Time) {
	if !startupFailureReasons[reason] {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	state := t.podLocked(workload, pod)
	if at.After(state.eventSeen) {
		state.eventSeen = at
		state.eventReason = reason
	}
}
