package metrics

import (
	"math"
	"testing"
	"time"
)

// TestCAILR_ZeroBeforeData verifies that R=0 is returned before enough data.
func TestCAILR_ZeroBeforeData(t *testing.T) {
	c := newCAILR()
	if c.RuptureIndex() != 0 {
		t.Error("RuptureIndex should be 0 before data")
	}
	if c.IsAccelerating() {
		t.Error("IsAccelerating should be false before data")
	}
}

// TestCAILR_FlatSeries verifies that a flat series produces R≈0 (no rupture).
func TestCAILR_FlatSeries(t *testing.T) {
	c := newCAILR()
	for i := 0; i < 50; i++ {
		c.Update(float64(i)*15, 0.5) // constant value
	}
	r := c.RuptureIndex()
	// Both slopes near 0 → R should be 0 (guard clause) or very small
	if math.Abs(r) > 0.1 {
		t.Errorf("flat series: RuptureIndex = %.4f; want ≈0", r)
	}
	if c.IsAccelerating() {
		t.Error("flat series should not be accelerating")
	}
}

// TestCAILR_LinearRise verifies that a steady linear rise produces R≈1.
func TestCAILR_LinearRise(t *testing.T) {
	c := newCAILR()
	for i := 0; i < 100; i++ {
		// Perfectly linear: y = 0.001 * x, slope should converge same in both models
		x := float64(i) * 15
		c.Update(x, 0.001*x)
	}
	r := c.RuptureIndex()
	// Both stable and burst see the same linear trend → R should be near 1
	if r < 0.5 || r > 2.0 {
		t.Errorf("linear rise: RuptureIndex = %.4f; want 0.5–2.0", r)
	}
	if c.IsAccelerating() {
		t.Error("steady linear rise should not trigger IsAccelerating (R<3)")
	}
}

// TestCAILR_ExponentialAcceleration verifies R>3 on a sudden acceleration.
func TestCAILR_ExponentialAcceleration(t *testing.T) {
	c := newCAILR()

	// Phase 1: stable baseline (slow rise) — feed 60 samples to warm up stable model
	for i := 0; i < 60; i++ {
		x := float64(i) * 15
		c.Update(x, 0.0001*x) // very slow drift
	}

	// Phase 2: sudden acceleration — inject steep rise into burst window
	base := float64(60) * 15
	for i := 0; i < 25; i++ {
		x := base + float64(i)*15
		// steep: 100× faster than baseline
		c.Update(x, 0.01*float64(i))
	}

	r := c.RuptureIndex()
	if r <= 3.0 {
		t.Errorf("exponential acceleration: RuptureIndex = %.2f; want > 3.0", r)
	}
	if !c.IsAccelerating() {
		t.Errorf("exponential acceleration: IsAccelerating should be true (R=%.2f)", r)
	}
}

// TestPredictor_RuptureIndex integration: Feed via Predictor and check RuptureIndex.
func TestPredictor_RuptureIndex(t *testing.T) {
	p := NewPredictor()
	now := time.Now()

	// Stable phase
	for i := 0; i < 60; i++ {
		ts := now.Add(time.Duration(i) * 15 * time.Second)
		p.Feed("host1", "memory_percent", 0.0001*float64(i), ts)
	}

	// Accelerating phase
	base := now.Add(60 * 15 * time.Second)
	for i := 0; i < 25; i++ {
		ts := base.Add(time.Duration(i) * 15 * time.Second)
		p.Feed("host1", "memory_percent", 0.01*float64(i), ts)
	}

	r := p.RuptureIndex("host1", "memory_percent")
	if r <= 0 {
		t.Errorf("expected positive rupture index, got %.4f", r)
	}
}

// TestPredictor_AcceleratingMetrics verifies accelerating metrics are returned.
func TestPredictor_AcceleratingMetrics(t *testing.T) {
	p := NewPredictor()
	now := time.Now()

	// Phase 1: long stable baseline (nearly flat) — 80 samples to anchor stable model
	for i := 0; i < 80; i++ {
		ts := now.Add(time.Duration(i) * 15 * time.Second)
		p.Feed("host1", "cpu_percent", 1e-6*float64(i), ts) // near-zero drift
	}
	// Phase 2: sudden 500× acceleration — 30 samples, very steep
	base := now.Add(80 * 15 * time.Second)
	for i := 0; i < 30; i++ {
		ts := base.Add(time.Duration(i) * 15 * time.Second)
		p.Feed("host1", "cpu_percent", 0.5*float64(i), ts) // 500× faster
	}

	events := p.AcceleratingMetrics("host1")
	if len(events) == 0 {
		// Log actual rupture index to aid debugging when the test fails
		ri := p.RuptureIndex("host1", "cpu_percent")
		t.Fatalf("expected at least one rupture event for accelerating cpu_percent (R=%.2f)", ri)
	}
	found := false
	for _, ev := range events {
		if ev.Metric == "cpu_percent" {
			found = true
			if ev.RuptureIndex <= 3.0 {
				t.Errorf("event RuptureIndex = %.2f; want > 3.0", ev.RuptureIndex)
			}
		}
	}
	if !found {
		t.Error("cpu_percent rupture event not found in AcceleratingMetrics")
	}
}

// TestCAILR_AlphaAccessors ensures exported alpha helpers work.
func TestCAILR_AlphaAccessors(t *testing.T) {
	c := newCAILR()
	for i := 0; i < 10; i++ {
		c.Update(float64(i), float64(i)*2)
	}
	if c.AlphaStable() == 0 && c.AlphaBurst() == 0 {
		t.Error("expected non-zero alphas after feeding data")
	}
}
