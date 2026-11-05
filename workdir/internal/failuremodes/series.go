package failuremodes

import (
	"sort"
	"time"
)

// SeriesPoint is one memory reading as a share of the pod limit.
type SeriesPoint struct {
	At    time.Time
	Usage float64
}

// PodSeries is the memory window a pod's saturation estimate is computed from.
type PodSeries struct {
	Pod    string
	Points []SeriesPoint
}

// MemorySeries returns, per pod of workload, the readings the saturation
// estimator holds, sorted by pod name. Pods without a memory limit have none
// and are left out.
func (t *Tracker) MemorySeries(workload string) []PodSeries {
	t.mu.RLock()
	defer t.mu.RUnlock()
	series := []PodSeries{}
	for pod, state := range t.pods {
		if state.workload != workload || state.detector == nil || len(state.detector.points) == 0 {
			continue
		}
		points := make([]SeriesPoint, len(state.detector.points))
		for i, p := range state.detector.points {
			points[i] = SeriesPoint{At: p.at, Usage: p.usage}
		}
		series = append(series, PodSeries{Pod: pod, Points: points})
	}
	sort.Slice(series, func(i, j int) bool { return series[i].Pod < series[j].Pod })
	return series
}
