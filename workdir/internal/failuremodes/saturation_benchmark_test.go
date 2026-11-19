package failuremodes

import (
	"testing"
	"time"
)

const (
	benchMiB = 1024 * 1024
	// minHeld is the share of samples after the first forecast that must keep it.
	minHeld = 0.9
)

// stepLeak is the benchmark step-leak fault: 48 MiB held at once, then a pause
// of 360, 540 or 180 s in turn, from a process of baseMiB.
func stepLeak(baseMiB float64) func(min float64) float64 {
	return func(min float64) float64 {
		mib, at, i := baseMiB, 0.0, 0
		for at <= min {
			i++
			mib += 48
			at += float64(180+(i%3)*180) / 60
		}
		return mib * benchMiB
	}
}

// accelLeak is the benchmark accel-leak fault: i MiB held at minute i-1.
func accelLeak(baseMiB float64) func(min float64) float64 {
	return func(min float64) float64 {
		mib := baseMiB
		for i := 1; float64(i-1) <= min; i++ {
			mib += float64(i)
		}
		return mib * benchMiB
	}
}

// ordersLeak is the benchmark leak-orders fault: 2 MiB held every 15 s.
func ordersLeak(baseMiB float64) func(min float64) float64 {
	return func(min float64) float64 {
		return (baseMiB + 2*float64(int(min*4)+1)) * benchMiB
	}
}

// TestSaturation_BenchmarkLeaks replays the leak faults of the published
// benchmark, sampled every 30 s from container start, and requires a forecast
// before the OOM kill with at least minLead to spare.
func TestSaturation_BenchmarkLeaks(t *testing.T) {
	const limit = 256 * benchMiB
	tests := []struct {
		name    string
		curve   func(min float64) float64
		minLead float64
	}{
		{"step-leak", stepLeak(20), 5},
		{"step-leak heavier start", stepLeak(40), 3},
		{"accel-leak", accelLeak(20), 3},
		{"leak-orders", ordersLeak(20), 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewSaturation()
			first, oom := -1.0, -1.0
			after, held := 0, 0
			for i := 0; i <= 240; i++ {
				min := float64(i) / 2
				usage := tt.curve(min)
				if usage >= limit {
					oom = min
					break
				}
				_, ok := e.Observe(t0.Add(time.Duration(i)*30*time.Second), usage, limit, 0)
				if ok && first < 0 {
					first = min
				}
				if first >= 0 {
					after++
					if ok {
						held++
					}
				}
			}
			t.Logf("first forecast at minute %v, OOM at minute %v, held %d of %d samples", first, oom, held, after)
			if oom < 0 {
				t.Fatal("curve never reached the limit")
			}
			if first < 0 || oom-first < tt.minLead {
				t.Fatalf("forecast at minute %v, OOM at %v, want at least %v min of lead", first, oom, tt.minLead)
			}
			// A forecast that comes and goes would reset alert timers downstream.
			if float64(held) < minHeld*float64(after) {
				t.Fatalf("forecast held on %d of %d samples after the first, want %v", held, after, minHeld)
			}
		})
	}
}
