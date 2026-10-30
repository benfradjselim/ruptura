package failuremodes

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)

func feed(e *Saturation, from, n int, step time.Duration, usageAt func(i int) float64, restarts int) (Estimate, bool) {
	var est Estimate
	var ok bool
	for i := from; i < from+n; i++ {
		est, ok = e.Observe(t0.Add(time.Duration(i)*step), usageAt(i)*1000, 1000, restarts)
	}
	return est, ok
}

func TestSaturation_Estimate(t *testing.T) {
	tests := []struct {
		name    string
		limit   float64
		usage   func(i int) float64
		wantOK  bool
		wantETF time.Duration
	}{
		{"linear leak", 1000, func(i int) float64 { return 0.5 + 0.01*float64(i) }, true, 30 * time.Minute},
		{"flat", 1000, func(int) float64 { return 0.5 }, false, 0},
		{"unknown capacity", 0, func(i int) float64 { return 0.5 + 0.01*float64(i) }, false, 0},
		{"falling", 1000, func(i int) float64 { return 0.9 - 0.01*float64(i) }, false, 0},
		{"noisy flat", 1000, func(i int) float64 {
			if i%2 == 0 {
				return 0.5
			}
			return 0.55
		}, false, 0},
		{"outlier spike on leak", 1000, func(i int) float64 {
			if i == 7 {
				return 0.99
			}
			return 0.5 + 0.01*float64(i)
		}, true, 30 * time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewSaturation()
			var est Estimate
			var ok bool
			for i := 0; i <= 10; i++ {
				est, ok = e.Observe(t0.Add(time.Duration(i)*time.Minute), tt.usage(i)*tt.limit, tt.limit, 0)
			}
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v (%+v)", ok, tt.wantOK, est)
			}
			if !ok {
				return
			}
			if est.Mode != ModeSaturation {
				t.Fatalf("mode = %q", est.Mode)
			}
			diff := est.ETF - tt.wantETF
			if diff < 0 {
				diff = -diff
			}
			// usage at i=9 is 0.59, slope 0.01/min -> 41 minutes left.
			want := 41 * time.Minute
			if d := est.ETF - want; d < -2*time.Minute || d > 2*time.Minute {
				t.Fatalf("ETF = %v, want about %v", est.ETF, want)
			}
			if est.Confidence <= 0 || est.Confidence > 1 {
				t.Fatalf("confidence = %v", est.Confidence)
			}
		})
	}
}

func TestSaturation_ResetsAfterRestart(t *testing.T) {
	e := NewSaturation()
	if _, ok := feed(e, 0, 11, time.Minute, func(i int) float64 { return 0.5 + 0.01*float64(i) }, 0); !ok {
		t.Fatal("expected estimate before restart")
	}
	if _, ok := feed(e, 10, 1, time.Minute, func(int) float64 { return 0.1 }, 1); ok {
		t.Fatal("expected no estimate right after restart")
	}
}

func TestSaturation_TooFewSamples(t *testing.T) {
	e := NewSaturation()
	if _, ok := feed(e, 0, 2, time.Minute, func(i int) float64 { return 0.5 + 0.1*float64(i) }, 0); ok {
		t.Fatal("expected no estimate with two samples")
	}
}
