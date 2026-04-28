package context

import "time"

// BaselineAdapter adjusts the ELS lambda based on context.
type BaselineAdapter struct {
    detector *DeploymentDetector
    store    *ManualContextStore
}

func NewBaselineAdapter(d *DeploymentDetector, s *ManualContextStore) *BaselineAdapter {
    return &BaselineAdapter{detector: d, store: s}
}
// Lambda returns the appropriate ELS lambda for time t.
func (a *BaselineAdapter) Lambda(t time.Time) float64 {
    if a.detector.IsSuppressed(t) {
        return 0.90
    }
    for _, e := range a.store.List() {
        if e.Type == ContextAbnormalTraffic {
            return 0.80
        }
    }
    return 0.99
}
