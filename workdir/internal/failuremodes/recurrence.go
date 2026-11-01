package failuremodes

import (
	"sort"
	"time"
)

// ModeRecurrentFailure is a failure expected from the rhythm of earlier failures.
const ModeRecurrentFailure Mode = "recurrent_failure"

const (
	// recurrenceMinFailures gives 5 intervals to judge regularity. With 3 intervals,
	// 8 percent of random failure times looked like a rhythm.
	recurrenceMinFailures = 6
	// recurrenceMaxSpread is the highest spread of intervals, relative to their
	// median, that still counts as a rhythm.
	recurrenceMaxSpread = 0.2
	// recurrenceOverdue is how far past due a failure can be, in intervals,
	// before the rhythm counts as broken.
	recurrenceOverdue = 1.5
)

func medianDuration(values []time.Duration) time.Duration {
	sorted := append([]time.Duration(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle]
	}
	return (sorted[middle-1] + sorted[middle]) / 2
}

// nextFailure predicts the next failure from past failure times, sorted oldest
// first. It returns the time left, the regularity of the rhythm, and whether a
// rhythm exists. An overdue failure within tolerance has zero time left.
func nextFailure(times []time.Time, now time.Time) (time.Duration, float64, bool) {
	if len(times) < recurrenceMinFailures {
		return 0, 0, false
	}
	intervals := make([]time.Duration, 0, len(times)-1)
	for i := 1; i < len(times); i++ {
		intervals = append(intervals, times[i].Sub(times[i-1]))
	}
	interval := medianDuration(intervals)
	if interval <= 0 {
		return 0, 0, false
	}
	deviations := make([]time.Duration, 0, len(intervals))
	for _, value := range intervals {
		deviation := value - interval
		if deviation < 0 {
			deviation = -deviation
		}
		deviations = append(deviations, deviation)
	}
	spread := float64(medianDuration(deviations)) / float64(interval)
	if spread >= recurrenceMaxSpread {
		return 0, 0, false
	}
	elapsed := now.Sub(times[len(times)-1])
	if float64(elapsed) > recurrenceOverdue*float64(interval) {
		return 0, 0, false
	}
	remaining := interval - elapsed
	if remaining < 0 {
		remaining = 0
	}
	return remaining, 1 - spread, true
}

// recurrenceEstimate reads the failure times of a workload.
func (t *Tracker) recurrenceEstimate(workload string, now time.Time) (Estimate, bool) {
	remaining, confidence, ok := nextFailure(t.failures[workload], now)
	if !ok {
		return Estimate{}, false
	}
	return Estimate{Mode: ModeRecurrentFailure, ETF: remaining, Confidence: confidence}, true
}
