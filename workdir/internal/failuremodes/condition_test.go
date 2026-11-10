package failuremodes

import (
	"math"
	"testing"
	"time"
)

// ratioRun feeds one ratio per 30 seconds for the given number of minutes and
// returns the last result.
func ratioRun(c *Condition, startMin float64, minutes float64, ratio float64) (Estimate, bool) {
	var est Estimate
	var ok bool
	for m := startMin; m <= startMin+minutes+1e-9; m += 0.5 {
		est, ok = c.Observe(t0.Add(time.Duration(m*float64(time.Minute))), ratio)
	}
	return est, ok
}

func TestCondition_Table(t *testing.T) {
	tests := []struct {
		name string
		run  func(c *Condition) (Estimate, bool)
		want bool
	}{
		{"sustained for 10 minutes raises", func(c *Condition) (Estimate, bool) { return ratioRun(c, 0, 10, 0.4) }, true},
		{"sustained for 9 minutes does not", func(c *Condition) (Estimate, bool) { return ratioRun(c, 0, 9, 0.4) }, false},
		{"exactly at threshold counts", func(c *Condition) (Estimate, bool) { return ratioRun(c, 0, 10, 0.25) }, true},
		{"just below threshold never raises", func(c *Condition) (Estimate, bool) { return ratioRun(c, 0, 30, 0.249) }, false},
		{"brief spike does not raise", func(c *Condition) (Estimate, bool) {
			ratioRun(c, 0, 3, 0.9)
			return ratioRun(c, 3.5, 20, 0.05)
		}, false},
		{"flapping never raises", func(c *Condition) (Estimate, bool) {
			var est Estimate
			var ok bool
			for i := 0; i < 40; i++ {
				r := 0.05
				if i%16 < 15 { // 7.5 minutes high, 0.5 low
					r = 0.5
				}
				est, ok = c.Observe(t0.Add(time.Duration(i)*30*time.Second), r)
			}
			return est, ok
		}, false},
		{"short dip does not clear", func(c *Condition) (Estimate, bool) {
			ratioRun(c, 0, 10, 0.4)
			return ratioRun(c, 10.5, 4, 0.05)
		}, true},
		{"clears after 5 minutes below", func(c *Condition) (Estimate, bool) {
			ratioRun(c, 0, 10, 0.4)
			return ratioRun(c, 10.5, 5, 0.05)
		}, false},
		{"dip then recovery resets the clear timer", func(c *Condition) (Estimate, bool) {
			ratioRun(c, 0, 10, 0.4)
			ratioRun(c, 10.5, 4, 0.05)
			ratioRun(c, 14.5, 0.5, 0.4)
			return ratioRun(c, 15, 4, 0.05)
		}, true},
		{"data gap does not count as sustained", func(c *Condition) (Estimate, bool) {
			ratioRun(c, 0, 4, 0.4)
			return ratioRun(c, 11, 4, 0.4)
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			est, ok := tt.run(NewCondition())
			if ok != tt.want {
				t.Fatalf("raised = %v, want %v (%+v)", ok, tt.want, est)
			}
			if ok && (est.Mode != ModeCPUThrottling || est.ETF != 0) {
				t.Fatalf("estimate = %+v, want cpu_throttling with zero ETF", est)
			}
		})
	}
}

func TestCondition_ReportsCurrentRatio(t *testing.T) {
	c := NewCondition()
	est, ok := ratioRun(c, 0, 10, 0.4)
	if !ok || est.Usage != 0.4 {
		t.Fatalf("est = %+v ok = %v", est, ok)
	}
}

func TestTracker_Condition(t *testing.T) {
	const w = "ns/Deployment/app"
	tr := NewTracker()
	var last time.Time
	for i := 0; i <= 20; i++ {
		last = t0.Add(time.Duration(i) * 30 * time.Second)
		tr.ObserveCondition(w, "p1", last, 0.5)
	}
	est, ok := tr.Worst(w, last)
	if !ok || est.Mode != ModeCPUThrottling || est.Pod != "p1" {
		t.Fatalf("worst = %+v ok = %v", est, ok)
	}
	if _, ok := tr.Worst(w, last.Add(time.Hour)); ok {
		t.Fatal("stale condition must expire")
	}
}

func TestTracker_MostUrgentMode(t *testing.T) {
	const w = "ns/Deployment/app"
	tests := []struct {
		name      string
		leakStep  float64 // usage added per minute, of 1000
		wantMode  Mode
		wantAlive bool
	}{
		{"imminent saturation outranks throttling", 60, ModeSaturation, true},
		{"slow saturation yields to throttling", 10, ModeCPUThrottling, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewTracker()
			var last time.Time
			for i := 0; i <= 20; i++ {
				last = t0.Add(time.Duration(i) * 30 * time.Second)
				tr.Observe(w, "p1", last, 100+tt.leakStep*float64(i)/2, 1000, 0)
				tr.ObserveCondition(w, "p1", last, 0.5)
			}
			est, ok := tr.Worst(w, last)
			if ok != tt.wantAlive || est.Mode != tt.wantMode {
				t.Fatalf("worst = %+v ok = %v, want mode %s", est, ok, tt.wantMode)
			}
		})
	}
}

func TestCondition_NonFiniteRatioIsNotThrottling(t *testing.T) {
	tests := []struct {
		name  string
		ratio float64
		want  bool
	}{
		{"positive infinity", math.Inf(1), false},
		{"negative infinity", math.Inf(-1), false},
		{"not a number", math.NaN(), false},
		{"negative", -0.5, false},
		{"above one", 3, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			est, ok := ratioRun(NewCondition(), 0, 10, tt.ratio)
			if ok != tt.want {
				t.Fatalf("active = %v, want %v", ok, tt.want)
			}
			if ok && est.Usage != 1 {
				t.Fatalf("usage = %v, want 1", est.Usage)
			}
		})
	}
}
