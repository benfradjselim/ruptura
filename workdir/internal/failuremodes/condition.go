package failuremodes

import (
	"math"
	"time"
)

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

// Condition raises an estimate when a ratio stays at or above its threshold
// for raiseAfter and clears it after clearAfter below. NewCondition gives the
// CPU throttling settings. It is not safe for concurrent use.
type Condition struct {
	mode       Mode
	threshold  float64
	raiseAfter time.Duration
	clearAfter time.Duration
	runStart   time.Time // start of the current run at or above the threshold
	belowSince time.Time // start of the current run below the threshold
	last       time.Time
	active     bool
	maxGap     time.Duration
	// midpoint dates a change halfway between the last sample before it and
	// the first sample after it, for sources scraped minutes apart.
	midpoint bool
}

// NewCondition returns an idle CPU throttling detector.
func NewCondition() *Condition {
	return NewConditionFor(ModeCPUThrottling, ThrottleThreshold, conditionRaiseAfter, conditionClearAfter)
}

// NewConditionFor returns an idle detector that reports mode once the ratio
// stays at or above threshold for raiseAfter, and clears after clearAfter below.
func NewConditionFor(mode Mode, threshold float64, raiseAfter, clearAfter time.Duration) *Condition {
	return &Condition{mode: mode, threshold: threshold, raiseAfter: raiseAfter, clearAfter: clearAfter, maxGap: conditionMaxGap}
}

// NewSampledCondition returns a detector fed with the time of each source
// sample rather than the time it was read. A sample read twice counts once,
// and a change is dated halfway between samples. prior is the time of the
// source's previous sample, or zero when unknown; maxGap is the longest
// silence between samples that still counts as continuous observation.
func NewSampledCondition(mode Mode, threshold float64, raiseAfter, clearAfter, maxGap time.Duration, prior time.Time) *Condition {
	return &Condition{mode: mode, threshold: threshold, raiseAfter: raiseAfter, clearAfter: clearAfter,
		maxGap: maxGap, midpoint: true, last: prior}
}

// Observe records one ratio sample and reports whether the condition is active.
// The estimate has a zero ETF because the condition is already happening, and
// Usage carries the current ratio.
func (c *Condition) Observe(at time.Time, ratio float64) (Estimate, bool) {
	ratio = clampRatio(ratio)
	if c.midpoint && !c.last.IsZero() && !at.After(c.last) {
		return c.estimate(ratio)
	}
	start := at
	if !c.last.IsZero() && at.Sub(c.last) > c.maxGap {
		c.runStart, c.belowSince = time.Time{}, time.Time{}
	} else if c.midpoint && !c.last.IsZero() {
		start = c.last.Add(at.Sub(c.last) / 2)
	}
	c.last = at
	if ratio >= c.threshold {
		c.belowSince = time.Time{}
		if c.runStart.IsZero() {
			c.runStart = start
		}
		if at.Sub(c.runStart) >= c.raiseAfter {
			c.active = true
		}
	} else {
		c.runStart = time.Time{}
		if c.belowSince.IsZero() {
			c.belowSince = start
		}
		if at.Sub(c.belowSince) >= c.clearAfter {
			c.active = false
		}
	}
	return c.estimate(ratio)
}

func (c *Condition) estimate(ratio float64) (Estimate, bool) {
	if !c.active {
		return Estimate{}, false
	}
	return Estimate{Mode: c.mode, Confidence: 1, Usage: ratio}, true
}

// clampRatio bounds a throttled-periods ratio to [0, 1]. A query dividing by a
// zero periods rate returns NaN or +Inf, which counts as no throttling.
func clampRatio(ratio float64) float64 {
	if math.IsNaN(ratio) || math.IsInf(ratio, 0) || ratio < 0 {
		return 0
	}
	return math.Min(ratio, 1)
}

// urgency orders estimates: the smallest value is the most urgent.
func (e Estimate) urgency() time.Duration {
	switch e.Mode {
	case ModeCPUThrottling:
		return conditionUrgencyETF
	case ModeStartupStuck:
		return startupStuckUrgency
	case ModeReplicasUnavailable:
		return replicasUrgency
	case ModeRolloutStuck:
		return rolloutUrgency
	case ModeOOMKilled:
		return oomUrgency
	}
	return e.ETF
}
