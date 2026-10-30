package failuremodes

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

const (
	realisticStep      = 30 * time.Second
	realisticLimit     = 1e9
	realisticSeeds     = 20
	realisticWarnETF   = 25 * time.Minute
	realisticMinLeadIn = 10 * time.Minute
)

// realisticSeries returns usage as a fraction of the limit at minute m.
type realisticSeries func(rng *rand.Rand, minute float64) float64

// noisy applies relative uniform noise of +/- frac to v.
func noisy(rng *rand.Rand, v, frac float64) float64 {
	return v * (1 + frac*(2*rng.Float64()-1))
}

// sawtooth ramps from floor to floor+amp over period minutes, then drops.
func sawtooth(minute, floor, amp, period float64) float64 {
	phase := minute - period*float64(int(minute/period))
	return floor + amp*phase/period
}

// runScenario feeds a series and returns the first warning time (minutes) after the grace period and
// the first minute at which usage reaches the limit (-1 if never).
func runScenario(seed int64, minutes, grace float64, gen realisticSeries) (warnAt, limitAt float64) {
	rng := rand.New(rand.NewSource(seed))
	s := NewSaturation()
	start := time.Unix(1700000000, 0)
	warnAt, limitAt = -1, -1
	for i := 0; float64(i)*realisticStep.Minutes() <= minutes; i++ {
		m := float64(i) * realisticStep.Minutes()
		u := gen(rng, m)
		if u >= 1 {
			if limitAt < 0 {
				limitAt = m
			}
			u = 1
		}
		est, ok := s.Observe(start.Add(time.Duration(i)*realisticStep), u*realisticLimit, realisticLimit, 0)
		if ok && m >= grace && est.ETF <= realisticWarnETF && warnAt < 0 {
			warnAt = m
		}
		if limitAt >= 0 {
			break
		}
	}
	return warnAt, limitAt
}

func TestSaturationRealisticScenarios(t *testing.T) {
	tests := []struct {
		name    string
		minutes float64
		warn    bool
		gen     realisticSeries
		// grace stays zero: the detector enforces its own cold-start rule.
		grace float64
	}{
		{"gc sawtooth flat trend", 60, false, func(r *rand.Rand, m float64) float64 {
			return noisy(r, sawtooth(m, 0.50, 0.20, 8), 0.01)
		}, 10},
		{"five minute spike returns to baseline", 60, false, func(r *rand.Rand, m float64) float64 {
			v := 0.50
			if m >= 20 && m < 25 {
				v = 0.90
			}
			return noisy(r, v, 0.01)
		}, 0},
		{"stable at 95 percent", 60, false, func(r *rand.Rand, m float64) float64 {
			return noisy(r, 0.95, 0.005)
		}, 0},
		{"step change to new flat level", 60, false, func(r *rand.Rand, m float64) float64 {
			v := 0.40
			if m >= 15 {
				v = 0.80
			}
			return noisy(r, v, 0.01)
		}, 0},
		{"linear leak with 5 percent noise", 120, true, func(r *rand.Rand, m float64) float64 {
			return noisy(r, 0.20+0.01*m, 0.05)
		}, 0},
		{"sawtooth with rising floor", 120, true, func(r *rand.Rand, m float64) float64 {
			return noisy(r, sawtooth(m, 0.30+0.007*m, 0.15, 8), 0.01)
		}, 0},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			for seed := int64(1); seed <= realisticSeeds; seed++ {
				seed := seed
				t.Run(fmt.Sprintf("seed%d", seed), func(t *testing.T) {
					warnAt, limitAt := runScenario(seed, tc.minutes, tc.grace, tc.gen)
					if !tc.warn {
						if warnAt >= 0 {
							t.Fatalf("false warning at minute %.1f", warnAt)
						}
						return
					}
					if limitAt < 0 {
						t.Fatal("scenario never reached the limit")
					}
					if warnAt < 0 {
						t.Fatalf("no warning before limit at minute %.1f", limitAt)
					}
					lead := time.Duration((limitAt - warnAt) * float64(time.Minute))
					if lead < realisticMinLeadIn {
						t.Fatalf("warning lead %v, want at least %v", lead, realisticMinLeadIn)
					}
				})
			}
		})
	}
}
