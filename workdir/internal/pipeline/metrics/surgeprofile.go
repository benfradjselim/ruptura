package metrics

// SurgeProfile classifies the current behaviour of a metric stream.
// SPECS.md §5.5 (WP §5.5).
func SurgeProfile(alphaBurst, alphaStable, ruptureIndex float64) string {
	const nearZero = 0.01
	switch {
	case ruptureIndex >= 5.0:
		return "Cascading"
	case alphaBurst < -nearZero:
		return "Recovering"
	case alphaBurst > nearZero && ruptureIndex >= 1.5 && alphaBurst < alphaStable*0.5:
		return "Spiking"
	case alphaBurst > nearZero:
		return "Ramping"
	default:
		return "Flat"
	}
}
