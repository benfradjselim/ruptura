package infra

import (
	"math"
	"time"
)

// sdiCondition represents one named condition tracked by the State Drift Index.
// weight and severity are fixed per condition definition; onset is updated at runtime.
// When onset is zero the condition is normal and contributes 0 to SDI.
type sdiCondition struct {
	// weight is the relative importance of this condition (0,1].
	weight float64
	// severity is the per-condition severity scaling factor (0,1].
	// {normal 0.0, elevated 0.3, warning 0.7, critical 1.0}
	severity float64
	// tRef is the saturation duration in seconds: f(d) reaches 1.0 at d=tRef.
	tRef float64
	// onset is when the condition became active; zero means not active.
	onset time.Time
}

// contribution returns this condition's normalized contribution at time now.
// contribution = weight * f(duration) * severity
// f(d) = min(1, d/tRef) — linear ramp that saturates at tRef.
// Returns 0 when onset is zero (condition is normal).
func (c sdiCondition) contribution(now time.Time) float64 {
	if c.onset.IsZero() {
		return 0
	}
	d := now.Sub(c.onset).Seconds()
	if d < 0 {
		d = 0
	}
	f := math.Min(1.0, d/c.tRef)
	return c.weight * f * c.severity
}

// computeSDI calculates the State Drift Index for a slice of conditions at time now.
//
//	SDI = Σ contribution_i / Σ (weight_i · severity_i)
//
// SDI ∈ [0,1]. Returns 0 when there are no conditions or the denominator is zero.
// The denominator is the theoretical maximum (all active, all saturated), so SDI=1
// means every condition is active and fully saturated.
func computeSDI(conds []sdiCondition, now time.Time) float64 {
	var num, denom float64
	for _, c := range conds {
		num += c.contribution(now)
		denom += c.weight * c.severity
	}
	if denom < 1e-12 {
		return 0
	}
	return math.Min(1.0, num/denom)
}
