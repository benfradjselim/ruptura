package failuremodes

import "time"

// ModeCPUThrottling is a condition that is already happening: the container is
// being throttled by its CPU limit.
const ModeCPUThrottling Mode = "cpu_throttling"

const (
	// ThrottleThreshold is the throttled-periods ratio at or above which a
	// workload counts as throttled.
	ThrottleThreshold = 0.25
	// conditionRaiseAfter is how long the ratio must stay at or above the
	// threshold before the condition is raised.
	conditionRaiseAfter = 10 * time.Minute
	// conditionClearAfter is how long the ratio must stay below the threshold
	// before a raised condition clears.
	conditionClearAfter = 5 * time.Minute
	// conditionMaxGap is the longest silence between samples that still counts
	// as continuous observation. A longer gap restarts the pending run.
	conditionMaxGap = 2 * time.Minute

	// conditionUrgencyETF ranks a condition against time-to-failure modes. It
	// sits just above the critical horizon: a condition already in progress
	// outranks a slow projection but never an imminent failure.
	conditionUrgencyETF = 10*time.Minute + time.Nanosecond
)

// Condition raises an estimate when a ratio stays at or above ThrottleThreshold
// for ten minutes and clears it after five minutes below. It is not safe for
// concurrent use.
type Condition struct {
	runStart   time.Time // start of the current run at or above the threshold
	belowSince time.Time // start of the current run below the threshold
	last       time.Time
	active     bool
}

// NewCondition returns an idle detector.
func NewCondition() *Condition { return &Condition{} }

// Observe records one ratio sample and reports whether the condition is active.
// The estimate has a zero ETF because the condition is already happening, and
// Usage carries the current ratio.
func (c *Condition) Observe(at time.Time, ratio float64) (Estimate, bool) {
	if !c.last.IsZero() && at.Sub(c.last) > conditionMaxGap {
		c.runStart, c.belowSince = time.Time{}, time.Time{}
	}
	c.last = at
	if ratio >= ThrottleThreshold {
		c.belowSince = time.Time{}
		if c.runStart.IsZero() {
			c.runStart = at
		}
		if at.Sub(c.runStart) >= conditionRaiseAfter {
			c.active = true
		}
	} else {
		c.runStart = time.Time{}
		if c.belowSince.IsZero() {
			c.belowSince = at
		}
		if at.Sub(c.belowSince) >= conditionClearAfter {
			c.active = false
		}
	}
	if !c.active {
		return Estimate{}, false
	}
	return Estimate{Mode: ModeCPUThrottling, Confidence: 1, Usage: ratio}, true
}

// urgency orders estimates: the smallest value is the most urgent.
func (e Estimate) urgency() time.Duration {
	switch e.Mode {
	case ModeCPUThrottling:
		return conditionUrgencyETF
	case ModeStartupStuck:
		return startupStuckUrgency
	}
	return e.ETF
}
