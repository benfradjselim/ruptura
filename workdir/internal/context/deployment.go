package context

import (
    "sync"
    "time"
)

// DeploymentEvent records a detected or manually-registered deployment.
type DeploymentEvent struct {
    ID        string
    Service   string
    StartedAt time.Time
    // Pre-suppression window: 60s before StartedAt
    // Post-suppression window: 300s after StartedAt
}

// DeploymentDetector tracks deployments and determines if anomaly detection
// should be suppressed for a given time.
type DeploymentDetector struct {
    mu      sync.RWMutex
    events  []DeploymentEvent
    preSup  time.Duration  // default 60s
    postSup time.Duration  // default 300s
}

func NewDeploymentDetector() *DeploymentDetector {
    return &DeploymentDetector{
        preSup:  60 * time.Second,
        postSup: 300 * time.Second,
    }
}
func (d *DeploymentDetector) Register(e DeploymentEvent) {
    d.mu.Lock()
    defer d.mu.Unlock()
    d.events = append(d.events, e)
}
// IsSuppressed returns true if t falls within the suppression window of any deployment.
func (d *DeploymentDetector) IsSuppressed(t time.Time) bool {
    d.mu.RLock()
    defer d.mu.RUnlock()
    for _, e := range d.events {
        if t.After(e.StartedAt.Add(-d.preSup)) && t.Before(e.StartedAt.Add(d.postSup)) {
            return true
        }
    }
    return false
}
// Active returns events whose suppression window overlaps with now.
func (d *DeploymentDetector) Active(now time.Time) []DeploymentEvent {
    d.mu.RLock()
    defer d.mu.RUnlock()
    var res []DeploymentEvent
    for _, e := range d.events {
        if now.After(e.StartedAt.Add(-d.preSup)) && now.Before(e.StartedAt.Add(d.postSup)) {
            res = append(res, e)
        }
    }
    return res
}
