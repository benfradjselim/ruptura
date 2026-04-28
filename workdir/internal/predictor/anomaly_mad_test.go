package predictor

import (
	"testing"
)

func TestAnomalyMAD(t *testing.T) {
	// Need at least 10 observations for MAD to trigger
	detector := newMADDetector(100, 3.0)
	values := []float64{1, 2, 3, 4, 1, 2, 3, 4, 1, 2}
	for _, v := range values {
		detector.Update(v)
	}
	
	// Test if 100 is detected as anomaly
	isAnomaly, _, _ := detector.IsAnomaly(100)
	if !isAnomaly {
		t.Error("expected 100 to be anomaly")
	}

	// Test if 2 is not anomaly
	isAnomaly, _, _ = detector.IsAnomaly(2)
	if isAnomaly {
		t.Error("expected 2 not to be anomaly")
	}
}
