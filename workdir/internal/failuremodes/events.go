package failuremodes

import (
	"strings"
	"time"
)

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
	"InvalidImageName":       true,
	"FailedCreate":           true,
}

// imagePullReason names image failures, which Kubernetes reports with the generic
// reason Failed and details in the message.
const imagePullReason = "ImagePull"

// startupFailure returns the cause of an event that blocks a start.
func startupFailure(reason, message string) (string, bool) {
	if startupFailureReasons[reason] {
		return reason, true
	}
	if reason == "Failed" && (strings.Contains(message, "ErrImagePull") || strings.Contains(message, "ImagePullBackOff")) {
		return imagePullReason, true
	}
	return "", false
}

// ObserveEvent records a Kubernetes warning event for a pod of workload.
// Reasons that do not block a start are ignored.
func (t *Tracker) ObserveEvent(workload, pod, reason, message string, at time.Time) {
	cause, ok := startupFailure(reason, message)
	if !ok {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	state := t.podLocked(workload, pod)
	if at.After(state.eventSeen) {
		state.eventSeen = at
		state.eventReason = cause
	}
}
