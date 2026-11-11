package failuremodes

import (
	"sort"
	"time"
)

// Watched is one pod read against its memory limit: how close it is and how
// fast it moves. It shows what the engine watches when nothing is forecast.
type Watched struct {
	Workload string
	Pod      string
	// Usage is the latest working set as a share of the limit.
	Usage float64
	// TrendPerHour is the Theil-Sen slope of that share over the saturation
	// window, per hour; zero when too few readings.
	TrendPerHour float64
	// Readings is the number of readings in the window.
	Readings int
}

// latest returns the latest share of the limit and its slope per second.
func (s *Saturation) latest() (usage, slope float64, readings int, ok bool) {
	if len(s.points) == 0 {
		return 0, 0, 0, false
	}
	usage = s.points[len(s.points)-1].usage
	if len(s.points) >= saturationMinSamples {
		slope, _ = theilSen(pairSlopes(s.points))
	}
	return usage, slope, len(s.points), true
}

// Closest returns up to n pods with a memory limit, read within estimateMaxAge,
// ordered by their share of the limit, highest first.
func (t *Tracker) Closest(now time.Time, n int) []Watched {
	t.mu.RLock()
	type candidate struct {
		pod   string
		state *podState
		usage float64
	}
	var all []candidate
	for pod, state := range t.pods {
		if state.detector == nil || now.Sub(state.seen) > estimateMaxAge || len(state.detector.points) == 0 {
			continue
		}
		points := state.detector.points
		all = append(all, candidate{pod, state, points[len(points)-1].usage})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].usage != all[j].usage {
			return all[i].usage > all[j].usage
		}
		return all[i].pod < all[j].pod
	})
	if len(all) > n {
		all = all[:n]
	}
	out := make([]Watched, 0, len(all))
	for _, c := range all {
		usage, slope, readings, _ := c.state.detector.latest()
		out = append(out, Watched{Workload: c.state.workload, Pod: c.pod, Usage: usage, TrendPerHour: slope * 3600, Readings: readings})
	}
	t.mu.RUnlock()
	return out
}
