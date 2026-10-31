package metrics

import (
	"testing"
	"time"
)

func TestAnomalyEngine_IgnoresNegligibleDeviations(t *testing.T) {
	tests := []struct {
		name      string
		base      float64
		jitter    float64
		value     float64
		wantAlert bool
	}{
		{"bounded negligible change", 0.0002, 0.00001, 0.0006, false},
		{"bounded real change", 0.1, 0.002, 0.8, true},
		{"unbounded negligible change", 1000, 2, 1020, false},
		{"unbounded real change", 1000, 2, 2000, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := NewAnomalyEngine()
			ts := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
			for i := 0; i < 400; i++ {
				v := tc.base + tc.jitter*float64(i%3-1)
				ts = ts.Add(15 * time.Second)
				if got := e.Observe("h", "m", v, ts); len(got) != 0 && i > 300 {
					t.Fatalf("training sample %d alerted: %+v", i, got)
				}
			}
			got := e.Observe("h", "m", tc.value, ts.Add(15*time.Second))
			if (len(got) > 0) != tc.wantAlert {
				t.Errorf("alerts = %+v, want alert %v", got, tc.wantAlert)
			}
		})
	}
}

func TestNegligibleDeviation_NegativeExpectedStaysBounded(t *testing.T) {
	if !negligibleDeviation(0.001134, -0.015593) {
		t.Error("a 0.017 change on a 0 to 1 metric must be negligible")
	}
	if negligibleDeviation(2.97, 2.30) {
		t.Error("a 29 percent change on an unbounded metric must alert")
	}
}
