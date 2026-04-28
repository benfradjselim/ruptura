package telemetry

import (
    "encoding/json"
    "time"
)

type TrackerStatus struct {
    Ready          bool `json:"ready"`
    MetricsTracked int  `json:"metrics_tracked,omitempty"`
}

type HealthResponse struct {
    Status           string                   `json:"status"` // "warming"|"ready"|"degraded"
    Trackers         map[string]TrackerStatus `json:"trackers"`
    RuptureDetection string                   `json:"rupture_detection"` // "suppressed"|"degraded"|"active"
    Message          string                   `json:"message"`
}

// HealthChecker computes health state based on uptime.
type HealthChecker struct {
    startTime time.Time
}

func NewHealthChecker() *HealthChecker {
    return &HealthChecker{startTime: time.Now()}
}

func (h *HealthChecker) Check(now time.Time) HealthResponse {
    uptime := now.Sub(h.startTime)
    res := HealthResponse{Trackers: make(map[string]TrackerStatus)}
    if uptime < 5*time.Minute {
        res.Status = "warming"
        res.RuptureDetection = "suppressed"
        res.Message = "warming up"
    } else if uptime < 60*time.Minute {
        res.Status = "degraded"
        res.RuptureDetection = "degraded"
        res.Message = "burst ready, stable warming"
    } else {
        res.Status = "ready"
        res.RuptureDetection = "active"
        res.Message = "full operation"
    }
    return res
}
// CheckJSON returns the health response as JSON bytes for time t.
func (h *HealthChecker) CheckJSON(t time.Time) ([]byte, error) {
    return json.Marshal(h.Check(t))
}
