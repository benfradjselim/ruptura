package predictor

import "testing"

func TestMADIsAnomaly_MinimumAbsoluteDeviation(t *testing.T) {
	tests := []struct {
		name   string
		window []float64
		value  float64
		want   bool
	}{
		{"bounded negligible change", []float64{0.0002, 0.00021, 0.00019, 0.0002, 0.00022, 0.00018, 0.0002, 0.00021, 0.00019, 0.0002}, 0.0004, false},
		{"bounded real change", []float64{0.0002, 0.00021, 0.00019, 0.0002, 0.00022, 0.00018, 0.0002, 0.00021, 0.00019, 0.0002}, 0.5, true},
		{"unbounded small relative change", []float64{100, 101, 99, 100, 102, 98, 100, 101, 99, 100}, 110, false},
		{"unbounded real change", []float64{100, 101, 99, 100, 102, 98, 100, 101, 99, 100}, 150, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := newMADDetector(100, 3.0)
			for _, v := range tc.window {
				d.Update(v)
			}
			if got, _, _ := d.IsAnomaly(tc.value); got != tc.want {
				t.Errorf("IsAnomaly(%v) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}
