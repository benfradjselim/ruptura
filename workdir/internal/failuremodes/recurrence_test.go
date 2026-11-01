package failuremodes

import (
	"math/rand"
	"testing"
	"time"
)

func failuresEvery(start time.Time, gaps ...time.Duration) []time.Time {
	times := []time.Time{start}
	for _, gap := range gaps {
		start = start.Add(gap)
		times = append(times, start)
	}
	return times
}

func TestNextFailure(t *testing.T) {
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	m := time.Minute
	regular := failuresEvery(start, 30*m, 31*m, 30*m, 29*m, 30*m, 31*m)
	last := regular[len(regular)-1]
	cases := []struct {
		name    string
		times   []time.Time
		now     time.Time
		want    bool
		wantETA time.Duration
	}{
		{"too few failures", regular[:5], last, false, 0},
		{"regular failures", regular, last.Add(5 * m), true, 25 * m},
		{"overdue within tolerance", regular, last.Add(33 * m), true, 0},
		{"long overdue stops", regular, last.Add(50 * m), false, 0},
		{"irregular failures", failuresEvery(start, 5*m, 40*m, 12*m, 70*m, 3*m, 25*m), start.Add(160 * m), false, 0},
	}
	for _, c := range cases {
		eta, confidence, ok := nextFailure(c.times, c.now)
		if ok != c.want {
			t.Errorf("%s: ok = %v, want %v", c.name, ok, c.want)
			continue
		}
		if ok && (eta != c.wantETA || confidence <= 0 || confidence > 1) {
			t.Errorf("%s: eta = %v, confidence = %v, want %v", c.name, eta, confidence, c.wantETA)
		}
	}
}

func TestNextFailure_RandomGapsStaySilent(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	warned, total := 0, 0
	for round := 0; round < 200; round++ {
		times := []time.Time{start}
		for i := 0; i < 12; i++ {
			gap := time.Duration(rng.ExpFloat64()*30*60) * time.Second
			times = append(times, times[len(times)-1].Add(gap))
			if len(times) >= recurrenceMinFailures {
				total++
				if _, _, ok := nextFailure(times, times[len(times)-1].Add(time.Minute)); ok {
					warned++
				}
			}
		}
	}
	if warned*20 > total {
		t.Errorf("warned %d of %d on random failures, want at most 5 percent", warned, total)
	}
}

func TestTracker_RecurrentFailure(t *testing.T) {
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	tr := NewTracker()
	for i := 0; i <= 7; i++ {
		tr.ObserveRestarts("w", "n/p", start.Add(time.Duration(i)*30*time.Minute), i)
	}
	est, ok := tr.Worst("w", start.Add(215*time.Minute))
	if !ok || est.Mode != ModeRecurrentFailure || est.ETF != 25*time.Minute {
		t.Errorf("estimate = %+v, %v", est, ok)
	}
}

func TestTracker_RecurrenceSpansReplacedPods(t *testing.T) {
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	tr := NewTracker()
	for i := 0; i <= 4; i++ {
		tr.ObserveRestarts("w", "n/old", start.Add(time.Duration(i)*30*time.Minute), i)
	}
	for i := 0; i <= 4; i++ {
		tr.ObserveRestarts("w", "n/new", start.Add(time.Duration(i+4)*30*time.Minute), i)
	}
	if est, ok := tr.Worst("w", start.Add(245*time.Minute)); !ok || est.Mode != ModeRecurrentFailure {
		t.Errorf("estimate = %+v, %v", est, ok)
	}
}
