package rupture

import (
	"math"
	"time"
)

const epsilon = 1e-6

// Index computes R(t) = |alphaBurst| / max(|alphaStable|, epsilon).
// SPECS.md §3.2 (WP §6.3).
func Index(alphaBurst, alphaStable float64) float64 {
	denom := math.Max(math.Abs(alphaStable), epsilon)
	return math.Abs(alphaBurst) / denom
}

// TTF computes time-to-failure: (thetaCritical - current) / alphaBurst.
// Clamped to [0, 3600] seconds. Returns 3600s when alphaBurst <= 0.
// SPECS.md §3.3 (WP §5.3).
func TTF(current, thetaCritical, alphaBurst float64) time.Duration {
	if alphaBurst <= 0 {
		return 3600 * time.Second
	}
	seconds := (thetaCritical - current) / alphaBurst
	seconds = math.Max(0, math.Min(3600, seconds))
	return time.Duration(seconds * float64(time.Second))
}

// Classify maps a Rupture Index to its tier label.
// SPECS.md §3.2 table (WP §6.4).
func Classify(r float64) string {
	switch {
	case r >= 5.0:
		return "Emergency"
	case r >= 3.0:
		return "Critical"
	case r >= 1.5:
		return "Warning"
	case r >= 1.0:
		return "Elevated"
	default:
		return "Stable"
	}
}
