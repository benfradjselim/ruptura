package composites

import (
	"math"
)

// Stress computes the weighted stress score from a set of metric factors.
func Stress(factors map[string]float64, thresholds map[string]float64) float64 {
	if thresholds == nil {
		thresholds = DefaultThresholds()
	}

	weights := map[string]float64{
		"cpu":        0.25,
		"memory":     0.25,
		"io":         0.20,
		"network":    0.15,
		"error_rate": 0.15,
	}

	var totalScore float64
	for metric, weight := range weights {
		val, ok := factors[metric]
		if !ok {
			continue
		}
		theta := thresholds[metric]
		if theta == 0 {
			// fallback to default thresholds if not provided
			theta = DefaultThresholds()[metric]
		}

		var g float64
		switch metric {
		case "cpu":
			g = val / theta
		case "memory":
			g = (val - 0.5*theta) / (0.5 * theta)
			if g < 0 {
				g = 0
			}
		case "io":
			g = 1 - math.Exp(-val/theta)
		case "network":
			g = val / theta
		case "error_rate":
			g = 1 - math.Exp(-3*val/theta)
		}
		
		// Clamp to [0, 1]
		if g < 0 { g = 0 }
		if g > 1 { g = 1 }

		totalScore += weight * g
	}

	return totalScore
}

// DefaultThresholds returns the canonical thresholds from SPECS.md §11.3.
func DefaultThresholds() map[string]float64 {
	return map[string]float64{
		"cpu":        100.0,
		"memory":     100.0,
		"io":         200.0,
		"network":    1000.0,
		"error_rate": 1.0,
	}
}

// Fatigue computes the next fatigue value from previous state.
func Fatigue(prevFatigue, prevStress, curStress, lambda float64) float64 {
	deltaStress := curStress - prevStress
	if deltaStress < 0 {
		deltaStress = 0
	}
	
	newFatigue := prevFatigue + deltaStress - (lambda * prevFatigue)
	if newFatigue < 0 {
		return 0
	}
	return newFatigue
}

// FatigueHalfLife returns t_half = ln(2) / lambda.
func FatigueHalfLife(lambda float64) float64 {
	if lambda == 0 {
		return 0
	}
	return math.Ln2 / lambda
}

// Pressure computes the pressure composite from z-scored signals.
func Pressure(latencyZ, errorZ, wLat, wErr float64) float64 {
	return (wLat * latencyZ) + (wErr * errorZ)
}

// HealthScore computes the multiplicative health score [0,100].
func HealthScore(signals map[string]float64, weights map[string]float64) float64 {
	if weights == nil {
		weights = DefaultHealthWeights()
	}

	score := 100.0
	for sig, w := range weights {
		val := signals[sig]
		// Formula: min(1, max(0, 1 - w_k * s_k(t)))
		term := 1 - (w * val)
		if term < 0 { term = 0 }
		if term > 1 { term = 1 }
		score *= term
	}
	return score
}

// DefaultHealthWeights returns canonical health weights from SPECS.md §11.10.
func DefaultHealthWeights() map[string]float64 {
	return map[string]float64{
		"stress":     0.35,
		"fatigue":    0.25,
		"pressure":   0.25,
		"contagion":  0.15,
	}
}

// Entropy computes Shannon entropy from a slice of variance values.
func Entropy(variances []float64) float64 {
	var sum float64
	for _, v := range variances {
		sum += v
	}
	
	if sum == 0 {
		return 0
	}
	
	var entropy float64
	for _, v := range variances {
		p := v / sum
		if p > 0 {
			entropy -= p * math.Log(p)
		}
	}
	return entropy
}

// Sentiment computes log(nPos+1) - log(nNeg+1).
func Sentiment(nPos, nNeg int) float64 {
	return math.Log(float64(nPos)+1) - math.Log(float64(nNeg)+1)
}
