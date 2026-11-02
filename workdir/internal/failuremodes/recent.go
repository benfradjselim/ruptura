package failuremodes

import "time"

// RecentFailures returns the failures of each workload since a time.
func (t *Tracker) RecentFailures(since time.Time) map[string][]time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := map[string][]time.Time{}
	for workload, times := range t.failures {
		for _, at := range times {
			if !at.Before(since) {
				out[workload] = append(out[workload], at)
			}
		}
	}
	return out
}
