package failuremodes

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

// cycleRun feeds minutes of a curve sampled every 30 s. The curve returns usage
// as a share of the limit and the container's restart count. It returns the
// minutes at which a saturation forecast was given.
func cycleRun(seed int64, minutes float64, curve func(rng *rand.Rand, m float64) (float64, int)) []float64 {
	rng := rand.New(rand.NewSource(seed))
	s := NewSaturation()
	var out []float64
	for i := 0; float64(i)/2 <= minutes; i++ {
		m := float64(i) / 2
		u, restarts := curve(rng, m)
		if _, ok := s.Observe(t0.Add(time.Duration(i)*30*time.Second), u*1e9, 1e9, restarts); ok {
			out = append(out, m)
		}
	}
	return out
}

// batch rises from floor to peak over ramp minutes, then falls back over 3
// minutes and rests, once per period, without restarting.
func batch(m, floor, peak, ramp, period float64) float64 {
	phase := m - period*float64(int(m/period))
	switch {
	case phase < ramp:
		return floor + (peak-floor)*phase/ramp
	case phase < ramp+3:
		return peak - (peak-floor)*(phase-ramp)/3
	}
	return floor
}

func TestSaturation_CycleGuard(t *testing.T) {
	tests := []struct {
		name    string
		minutes float64
		curve   func(rng *rand.Rand, m float64) (float64, int)
		// quietAfter is the minute after which no forecast is allowed; zero
		// means the check is skipped.
		quietAfter float64
		// cycles is the period of failures that must each be forecast before
		// they happen; zero means the check is skipped.
		cycles float64
		// warnAfter requires a forecast after this minute; zero skips it.
		warnAfter float64
	}{
		{name: "hourly batch, 40 minute ramp to 80 percent", minutes: 360, quietAfter: 150, curve: func(r *rand.Rand, m float64) (float64, int) {
			return noisy(r, batch(m, 0.3, 0.8, 40, 60), 0.01), 0
		}},
		{name: "two hour batch, 70 minute ramp to 90 percent", minutes: 600, quietAfter: 290, curve: func(r *rand.Rand, m float64) (float64, int) {
			return noisy(r, batch(m, 0.2, 0.9, 70, 120), 0.01), 0
		}},
		{name: "leak killed by OOM every 30 minutes", minutes: 300, cycles: 30, curve: func(r *rand.Rand, m float64) (float64, int) {
			phase := m - 30*float64(int(m/30))
			return noisy(r, 0.08+0.03*phase, 0.01), int(m / 30)
		}},
		{name: "hourly batch whose peak starts to leak", minutes: 420, warnAfter: 240, curve: func(r *rand.Rand, m float64) (float64, int) {
			peak := 0.6
			if m > 240 {
				peak += 0.004 * (m - 240)
			}
			return noisy(r, batch(m, 0.3, peak, 40, 60), 0.01), 0
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for seed := int64(1); seed <= 10; seed++ {
				seed := seed
				t.Run(fmt.Sprintf("seed%d", seed), func(t *testing.T) {
					got := cycleRun(seed, tt.minutes, tt.curve)
					if tt.quietAfter > 0 {
						for _, m := range got {
							if m > tt.quietAfter {
								t.Fatalf("forecast at minute %v on a repeating load", m)
							}
						}
					}
					if tt.cycles > 0 {
						for start := 0.0; start+tt.cycles <= tt.minutes; start += tt.cycles {
							warned := false
							for _, m := range got {
								if m >= start && m < start+tt.cycles {
									warned = true
								}
							}
							if !warned {
								t.Fatalf("no forecast in the cycle starting at minute %v", start)
							}
						}
					}
					if tt.warnAfter > 0 {
						warned := false
						for _, m := range got {
							if m > tt.warnAfter {
								warned = true
							}
						}
						if !warned {
							t.Fatalf("no forecast after minute %v although the peak grows", tt.warnAfter)
						}
					}
				})
			}
		})
	}
}

func TestWithinCycle(t *testing.T) {
	start := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	var cyclic []bucket
	for i := 0; i < 24; i++ {
		v := 0.3 + 0.1*float64(i%6)
		cyclic = append(cyclic, bucket{at: start.Add(time.Duration(i) * cycleBucket), low: v, high: v})
	}
	window := start.Add(24 * cycleBucket)
	tests := []struct {
		name    string
		history []bucket
		usage   float64
		want    bool
	}{
		{"inside earlier peaks", cyclic, 0.7, true},
		{"above earlier peaks", cyclic, 0.9, false},
		{"at earlier peaks with noise", cyclic, 0.83, true},
		{"one cycle only", cyclic[:9], 0.5, false},
		{"no history", nil, 0.5, false},
	}
	for _, tt := range tests {
		if got := withinCycle(tt.history, window, tt.usage); got != tt.want {
			t.Errorf("%s: withinCycle = %v, want %v", tt.name, got, tt.want)
		}
	}
}
