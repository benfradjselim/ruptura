package failuremodes

import (
	"strings"
	"time"
)

// ModeStartupStuck is a pod that cannot start. It is detected, not predicted.
const ModeStartupStuck Mode = "startup_stuck"

// startupEventMaxAge is how long a startup failure event stays valid. Kubernetes
// repeats these events every few minutes, up to its 5 minute back-off.
const startupEventMaxAge = 10 * time.Minute

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
	failedPull := reason == "Failed" && (strings.Contains(message, "ErrImagePull") || strings.Contains(message, "ImagePullBackOff"))
	waitingPull := reason == "BackOff" && strings.Contains(message, "pulling image")
	if failedPull || waitingPull {
		return imagePullReason, true
	}
	return "", false
}

// crashBackOffMaxAge is how long a crash back-off event keeps a workload in a
// crash loop. Kubernetes repeats it at each back-off, up to every 5 minutes.
const crashBackOffMaxAge = 10 * time.Minute

// crashBackOff reports the event Kubernetes sends when it delays restarting a
// container that keeps failing, the CrashLoopBackOff state.
func crashBackOff(reason, message string) bool {
	return reason == "BackOff" && strings.Contains(message, "restarting failed container")
}

// ObserveEvent records a Kubernetes warning event for a pod of workload.
// Reasons that neither block a start nor signal a crash back-off are ignored.
func (t *Tracker) ObserveEvent(workload, pod, reason, message string, at time.Time) {
	if crashBackOff(reason, message) {
		t.mu.Lock()
		defer t.mu.Unlock()
		state := t.podLocked(workload, pod)
		if at.After(state.backOffSeen) {
			state.backOffSeen = at
		}
		return
	}
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

// CrashBackOff reports whether a pod of workload is in a crash back-off now.
func (t *Tracker) CrashBackOff(workload string, now time.Time) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, state := range t.pods {
		if state.workload == workload && !state.backOffSeen.IsZero() && now.Sub(state.backOffSeen) <= crashBackOffMaxAge {
			return true
		}
	}
	return false
}
