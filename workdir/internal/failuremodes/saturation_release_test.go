package failuremodes

import (
	"testing"
	"time"
)

// releaseCurve is the memory of a pod of the F2 release test: warm MiB reached
// linearly over ramp minutes, then leak MiB per minute.
func releaseCurve(warm, ramp, leak float64) func(min float64) float64 {
	return func(min float64) float64 {
		mib := warm
		if min < ramp {
			mib = warm * min / ramp
		} else {
			mib += leak * (min - ramp)
		}
		return mib * 1024 * 1024
	}
}

func TestSaturation_ReleaseCurves(t *testing.T) {
	const limit = 256 * 1024 * 1024
	tests := []struct {
		name  string
		curve func(min float64) float64
		// wantWithin is the latest minute of the first estimate; zero means
		// the curve must never produce one during 25 minutes.
		wantWithin float64
	}{
		{"healthy 60 MiB in 3 min", releaseCurve(60, 3, 0), 0},
		{"heavy 120 MiB in 8 min", releaseCurve(120, 8, 0), 0},
		{"bad leak 3 MiB/min", releaseCurve(60, 3, 3), 15},
		{"bad leak 5 MiB/min", releaseCurve(60, 3, 5), 15},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewSaturation()
			first := -1.0
			for i := 0; i <= 50; i++ {
				min := float64(i) / 2
				_, ok := e.Observe(t0.Add(time.Duration(i)*30*time.Second), tt.curve(min), limit, 0)
				if ok && first < 0 {
					first = min
				}
			}
			t.Logf("first estimate at minute %v", first)
			if tt.wantWithin == 0 && first >= 0 {
				t.Fatalf("estimate at minute %v on a curve that stabilises", first)
			}
			if tt.wantWithin > 0 && (first < 0 || first > tt.wantWithin) {
				t.Fatalf("first estimate at minute %v, want within %v", first, tt.wantWithin)
			}
		})
	}
}
