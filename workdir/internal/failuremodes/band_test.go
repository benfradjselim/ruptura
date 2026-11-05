package failuremodes

import (
	"math"
	"testing"
	"time"
)

func TestSlopeBand(t *testing.T) {
	tests := []struct {
		name      string
		sorted    []float64
		low, high float64
	}{
		{"empty", nil, 0, 0},
		{"one slope", []float64{2}, 2, 2},
		{"quartiles", []float64{1, 2, 3, 4, 5, 6, 7, 8}, 3, 7},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			low, high := slopeBand(tc.sorted)
			if low != tc.low || high != tc.high {
				t.Errorf("got %v %v, want %v %v", low, high, tc.low, tc.high)
			}
		})
	}
}

func TestFailureBand(t *testing.T) {
	tests := []struct {
		name                 string
		remaining, low, high float64
		early, late          time.Duration
	}{
		{"both grow", 0.6, 0.001, 0.002, 300 * time.Second, 600 * time.Second},
		{"low slope flat leaves latest unknown", 0.6, 0, 0.002, 300 * time.Second, 0},
		{"nothing grows", 0.6, -0.001, 0, 0, 0},
		{"over the limit", -0.1, 0.001, 0.002, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			early, late := failureBand(tc.remaining, tc.low, tc.high)
			if early != tc.early || late != tc.late {
				t.Errorf("got %v %v, want %v %v", early, late, tc.early, tc.late)
			}
		})
	}
}

func TestSaturationBandSurroundsETF(t *testing.T) {
	s := NewSaturation()
	start := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	var est Estimate
	var ok bool
	for i := 0; i <= 40; i++ {
		noise := 3 * math.Sin(float64(i)*1.7)
		est, ok = s.Observe(start.Add(time.Duration(i)*30*time.Second), 100+float64(i)*2+noise, 400, 0)
	}
	if !ok {
		t.Fatal("no estimate on a noisy leak")
	}
	if est.ETFEarly <= 0 || est.ETFLate <= 0 || est.ETFEarly > est.ETF || est.ETF > est.ETFLate {
		t.Errorf("band %v to %v does not surround %v", est.ETFEarly, est.ETFLate, est.ETF)
	}
}
