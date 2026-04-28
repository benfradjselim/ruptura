package analyzer

import (
	"testing"
)

// TestDissipativeFatigue_Recovery — Integration Test A
// Simulates a 2 AM backup spike lasting 30 minutes.
// Asserts that Fatigue recovers below RThreshold within 2 hours of spike end.
// Asserts no Burnout state fires during recovery.
func TestDissipativeFatigue_Recovery(t *testing.T) {
	a := NewAnalyzer()

	metrics := func(cpu, errors float64) map[string]float64 {
		return map[string]float64{
			"cpu_percent":    cpu,
			"memory_percent": 0.5,
			"load_avg_1":     0.3,
			"error_rate":     errors,
			"timeout_rate":   0.01,
			"request_rate":   0.5,
			"uptime_seconds": 86400,
		}
	}

	// Simulate 30-minute backup spike: high CPU, moderate errors
	// At 15s intervals = 120 samples for 30 min
	for i := 0; i < 120; i++ {
		snap := a.Update("host1", metrics(0.75, 0.2))
		_ = snap
	}

	snapAfterSpike := a.Update("host1", metrics(0.75, 0.2))
	fatiguePeak := snapAfterSpike.Fatigue.Value
	t.Logf("Fatigue after 30min spike: %.4f (state=%s)", fatiguePeak, snapAfterSpike.Fatigue.State)

	// Now simulate 2 hours of idle recovery (low CPU, no errors)
	// At 15s intervals = 480 samples for 2 hours
	for i := 0; i < 480; i++ {
		a.Update("host1", metrics(0.1, 0.0))
	}

	snapRecovered := a.Update("host1", metrics(0.1, 0.0))
	fatigueAfter := snapRecovered.Fatigue.Value
	t.Logf("Fatigue after 2h recovery: %.4f (state=%s)", fatigueAfter, snapRecovered.Fatigue.State)

	// Assertion: Fatigue must be below Rested threshold after 2h idle
	if fatigueAfter >= 0.3 {
		t.Errorf("fatigue should recover below 0.3 (Rested) after 2h idle; got %.4f", fatigueAfter)
	}

	// Assertion: no burnout state during recovery
	if snapRecovered.Fatigue.State == "burnout" {
		t.Error("burnout state should not fire during recovery after a backup spike")
	}
}

// TestDissipativeFatigue_LambdaReducesFalsePositives verifies that λ dissipation
// prevents a short spike from permanently elevating Fatigue.
func TestDissipativeFatigue_LambdaReducesFalsePositives(t *testing.T) {
	a := NewAnalyzer()

	metrics := func(cpu float64) map[string]float64 {
		return map[string]float64{
			"cpu_percent":    cpu,
			"memory_percent": 0.3,
			"load_avg_1":     0.2,
			"error_rate":     0.0,
			"timeout_rate":   0.0,
			"request_rate":   0.5,
			"uptime_seconds": 86400,
		}
	}

	// Feed a 5-minute high-CPU spike (20 samples)
	for i := 0; i < 20; i++ {
		a.Update("host2", metrics(0.9))
	}

	snapAfterSpike := a.Update("host2", metrics(0.9))
	t.Logf("Fatigue immediately after spike: %.4f", snapAfterSpike.Fatigue.Value)

	// Feed 1 hour of idle
	for i := 0; i < 240; i++ {
		a.Update("host2", metrics(0.05))
	}

	snapRecovered := a.Update("host2", metrics(0.05))
	t.Logf("Fatigue after 1h idle: %.4f", snapRecovered.Fatigue.Value)

	// The v5.0 λ recovery should bring it back down
	if snapRecovered.Fatigue.Value >= snapAfterSpike.Fatigue.Value {
		t.Errorf("fatigue should decrease during idle: got %.4f after recovery vs %.4f after spike",
			snapRecovered.Fatigue.Value, snapAfterSpike.Fatigue.Value)
	}
}

// TestDissipativeFatigue_HighSustainedLoad verifies burnout still fires
// for genuinely sustained high load (not a false positive).
func TestDissipativeFatigue_HighSustainedLoad(t *testing.T) {
	a := NewAnalyzer()

	metrics := func() map[string]float64 {
		return map[string]float64{
			"cpu_percent":    0.95,
			"memory_percent": 0.90,
			"load_avg_1":     0.85,
			"error_rate":     0.3,
			"timeout_rate":   0.2,
			"request_rate":   0.5,
			"uptime_seconds": 86400,
		}
	}

	// Sustained max stress for 4 hours (960 samples)
	var snap interface{ GetFatigue() float64 }
	_ = snap
	var lastFatigue float64
	for i := 0; i < 960; i++ {
		s := a.Update("host3", metrics())
		lastFatigue = s.Fatigue.Value
	}

	t.Logf("Fatigue after 4h sustained max load: %.4f", lastFatigue)

	// With S≈1.0 >> RThreshold+λ, fatigue must accumulate significantly
	if lastFatigue < 0.5 {
		t.Errorf("fatigue should be high (≥0.5) after 4h sustained max load; got %.4f", lastFatigue)
	}
}

// TestSetFatigueConfig verifies custom λ and RThreshold take effect.
func TestSetFatigueConfig(t *testing.T) {
	a := NewAnalyzer()
	// Use aggressive recovery (large λ) — should recover faster
	a.SetFatigueConfig("hostX", 0.3, 0.5)

	m := map[string]float64{
		"cpu_percent": 0.8, "memory_percent": 0.5, "load_avg_1": 0.3,
		"error_rate": 0.1, "timeout_rate": 0.0, "request_rate": 0.5,
		"uptime_seconds": 3600,
	}
	for i := 0; i < 10; i++ {
		a.Update("hostX", m)
	}

	idle := map[string]float64{
		"cpu_percent": 0.05, "memory_percent": 0.2, "load_avg_1": 0.1,
		"error_rate": 0.0, "timeout_rate": 0.0, "request_rate": 0.5,
		"uptime_seconds": 3600,
	}
	for i := 0; i < 10; i++ {
		a.Update("hostX", idle)
	}

	snap := a.Update("hostX", idle)
	// With λ=0.5, recovery should be fast
	t.Logf("Fatigue with λ=0.5 after 10 idle cycles: %.4f", snap.Fatigue.Value)
	// Should be zero or near-zero with aggressive λ
	if snap.Fatigue.Value > 0.3 {
		t.Errorf("expected quick recovery with λ=0.5; got fatigue=%.4f", snap.Fatigue.Value)
	}
}
