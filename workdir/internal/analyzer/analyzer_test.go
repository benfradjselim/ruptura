package analyzer

import (
	"testing"

	"github.com/benfradjselim/ruptura/pkg/models"
)

func TestStressStates(t *testing.T) {
	tests := []struct {
		s    float64
		want string
	}{
		{0.1, "calm"},
		{0.4, "nervous"},
		{0.7, "stressed"},
		{0.9, "panic"},
	}
	for _, tc := range tests {
		if got := stressState(tc.s); got != tc.want {
			t.Errorf("stressState(%v) = %q; want %q", tc.s, got, tc.want)
		}
	}
}

func TestFatigueStates(t *testing.T) {
	tests := []struct {
		f    float64
		want string
	}{
		{0.2, "rested"},
		{0.4, "tired"},
		{0.7, "exhausted"},
		{0.9, "burnout"},
	}
	for _, tc := range tests {
		if got := fatigueState(tc.f); got != tc.want {
			t.Errorf("fatigueState(%v) = %q; want %q", tc.f, got, tc.want)
		}
	}
}

func TestAnalyzerUpdate(t *testing.T) {
	a := NewAnalyzer()

	metrics := map[string]float64{
		"cpu_percent":    0.5,
		"memory_percent": 0.4,
		"load_avg_1":     0.3,
		"error_rate":     0.0,
		"timeout_rate":   0.0,
		"request_rate":   100.0,
		"uptime_seconds": 86400.0,
	}

	snap := a.UpdateHost("host1", metrics)

	if snap.Host != "host1" {
		t.Errorf("snapshot host = %q; want host1", snap.Host)
	}
	if snap.Stress.Value < 0 || snap.Stress.Value > 1 {
		t.Errorf("stress out of [0,1]: %v", snap.Stress.Value)
	}
	if snap.Fatigue.Value < 0 || snap.Fatigue.Value > 1 {
		t.Errorf("fatigue out of [0,1]: %v", snap.Fatigue.Value)
	}
	if snap.Pressure.Value < 0 || snap.Pressure.Value > 1 {
		t.Errorf("pressure out of [0,1]: %v", snap.Pressure.Value)
	}
	if snap.Humidity.Value < 0 || snap.Humidity.Value > 1 {
		t.Errorf("humidity out of [0,1]: %v", snap.Humidity.Value)
	}
	if snap.Contagion.Value < 0 || snap.Contagion.Value > 1 {
		t.Errorf("contagion out of [0,1]: %v", snap.Contagion.Value)
	}
}

func TestStressFormula(t *testing.T) {
	a := NewAnalyzer()

	// Known values: cpu=1.0, rest=0
	// S = 0.30*1 + 0.20*0 + 0.20*0 + 0.20*0 + 0.10*0 = 0.30
	metrics := map[string]float64{
		"cpu_percent":    1.0,
		"memory_percent": 0.0,
		"load_avg_1":     0.0,
		"error_rate":     0.0,
		"timeout_rate":   0.0,
	}
	snap := a.UpdateHost("test", metrics)
	if snap.Stress.Value < 0.28 || snap.Stress.Value > 0.32 {
		t.Errorf("expected stress ~0.30, got %v", snap.Stress.Value)
	}
}

func TestFatigueAccumulation(t *testing.T) {
	a := NewAnalyzer()

	// High stress → fatigue should increase over repeated updates
	highStress := map[string]float64{
		"cpu_percent":    0.9,
		"memory_percent": 0.9,
		"load_avg_1":     0.9,
		"error_rate":     0.1,
		"timeout_rate":   0.1,
	}

	var prev float64
	for i := 0; i < 10; i++ {
		snap := a.UpdateHost("fatigue-host", highStress)
		if i > 0 && snap.Fatigue.Value < prev {
			t.Errorf("fatigue should increase under high stress, got %v < %v", snap.Fatigue.Value, prev)
		}
		prev = snap.Fatigue.Value
	}
}

func TestHumidityStates(t *testing.T) {
	tests := []struct {
		h    float64
		want string
	}{
		{0.05, "dry"},
		{0.2, "humid"},
		{0.4, "very_humid"},
		{0.6, "storm"},
	}
	for _, tc := range tests {
		if got := humidityState(tc.h); got != tc.want {
			t.Errorf("humidityState(%v) = %q; want %q", tc.h, got, tc.want)
		}
	}
}

func TestContagionStates(t *testing.T) {
	tests := []struct {
		c    float64
		want string
	}{
		{0.1, "low"},
		{0.4, "moderate"},
		{0.7, "epidemic"},
		{0.9, "pandemic"},
	}
	for _, tc := range tests {
		if got := contagionState(tc.c); got != tc.want {
			t.Errorf("contagionState(%v) = %q; want %q", tc.c, got, tc.want)
		}
	}
}

func TestMoodStates(t *testing.T) {
	tests := []struct {
		m    float64
		want string
	}{
		{0.9, "happy"},
		{0.6, "content"},
		{0.4, "neutral"},
		{0.2, "sad"},
		{0.05, "depressed"},
	}
	for _, tc := range tests {
		if got := moodState(tc.m); got != tc.want {
			t.Errorf("moodState(%v) = %q; want %q", tc.m, got, tc.want)
		}
	}
}

func TestPressureStates(t *testing.T) {
	tests := []struct {
		p    float64
		want string
	}{
		{0.8, "storm_approaching"},
		{0.6, "rising"},
		{0.5, "stable"},
		{0.3, "improving"},
	}
	for _, tc := range tests {
		if got := pressureState(tc.p); got != tc.want {
			t.Errorf("pressureState(%v) = %q; want %q", tc.p, got, tc.want)
		}
	}
}

func TestAnalyzerSnapshot(t *testing.T) {
	a := NewAnalyzer()

	// Before any update, Snapshot should return false
	if _, ok := a.Snapshot("ghost"); ok {
		t.Error("Snapshot should return false for unknown host")
	}

	// After an update the snapshot should be retrievable
	metrics := map[string]float64{"cpu_percent": 0.5, "memory_percent": 0.3}
	a.UpdateHost("snap-host", metrics)

	snap, ok := a.Snapshot("snap-host")
	if !ok {
		t.Fatal("Snapshot returned false after Update")
	}
	if snap.Host != "snap-host" {
		t.Errorf("snapshot host = %q; want snap-host", snap.Host)
	}
}

func TestAnalyzerRecordRestartAndResetFatigue(t *testing.T) {
	a := NewAnalyzer()

	// Accumulate some fatigue first
	high := map[string]float64{
		"cpu_percent": 0.95, "memory_percent": 0.95, "load_avg_1": 0.95,
	}
	for i := 0; i < 5; i++ {
		a.UpdateHost("rr-host", high)
	}

	snap, _ := a.Snapshot("rr-host")
	fatigueBeforeReset := snap.Fatigue.Value

	// RecordRestart should not panic
	a.RecordRestart("rr-host")

	// ResetFatigue should bring fatigue back to 0
	a.ResetFatigue("rr-host")

	// Next update should reflect near-zero fatigue
	snap2 := a.UpdateHost("rr-host", map[string]float64{"cpu_percent": 0.0})
	if snap2.Fatigue.Value >= fatigueBeforeReset {
		t.Errorf("fatigue after reset (%v) should be < pre-reset (%v)", snap2.Fatigue.Value, fatigueBeforeReset)
	}
}

func TestNormaliseWeights(t *testing.T) {
	w := normaliseWeights(models.SignalWeights{
		Selector:  "payments/*",
		Stress:    0.35,
		Fatigue:   0.15,
		Mood:      0.20,
		Pressure:  0.20,
		Humidity:  0.05,
		Contagion: 0.05,
	})
	total := w.Stress + w.Fatigue + w.Mood + w.Pressure + w.Humidity + w.Contagion
	if total < 0.999 || total > 1.001 {
		t.Errorf("normalised weights sum = %v; want 1.0", total)
	}
}

func TestNormaliseWeights_AllZero(t *testing.T) {
	w := normaliseWeights(models.SignalWeights{Selector: "*"})
	// All-zero input should be returned as-is without dividing by zero.
	total := w.Stress + w.Fatigue + w.Mood + w.Pressure + w.Humidity + w.Contagion
	if total != 0 {
		t.Errorf("all-zero weights should stay zero, got sum %v", total)
	}
}

func TestResolveWeights_MatchesSelector(t *testing.T) {
	a := NewAnalyzer()
	a.SetWeightConfigs([]models.SignalWeights{
		{Selector: "payments/*", Stress: 0.50, Fatigue: 0.10, Mood: 0.10, Pressure: 0.10, Humidity: 0.10, Contagion: 0.10},
		{Selector: "*", Stress: 0.25, Fatigue: 0.20, Mood: 0.20, Pressure: 0.15, Humidity: 0.10, Contagion: 0.10},
	})

	// Should match "payments/*"
	w := a.resolveWeights("payments/Deployment/checkout")
	if w.Stress < 0.49 || w.Stress > 0.51 {
		t.Errorf("expected payments selector stress ~0.50 (after normalise), got %v", w.Stress)
	}

	// Should fall through to "*"
	wDefault := a.resolveWeights("orders/Deployment/api")
	if wDefault.Stress < 0.24 || wDefault.Stress > 0.26 {
		t.Errorf("expected default stress ~0.25, got %v", wDefault.Stress)
	}
}

func TestResolveWeights_NoConfigs(t *testing.T) {
	a := NewAnalyzer()
	w := a.resolveWeights("any/workload")
	def := models.DefaultSignalWeights()
	if w.Stress != def.Stress || w.Fatigue != def.Fatigue {
		t.Errorf("expected default weights when no configs set, got %+v", w)
	}
}

func TestWeightConfigRoundtrip(t *testing.T) {
	a := NewAnalyzer()
	cfgs := []models.SignalWeights{
		{Selector: "batch/*", Stress: 0.10, Fatigue: 0.30, Mood: 0.10, Pressure: 0.10, Humidity: 0.20, Contagion: 0.20},
	}
	a.SetWeightConfigs(cfgs)
	got := a.WeightConfigs()
	if len(got) != 1 || got[0].Selector != "batch/*" {
		t.Errorf("roundtrip failed: %+v", got)
	}
}

// TestAdaptiveBaseline_IdleWorkload verifies that an idle workload (no CPU/memory/request
// metrics) does not get its health_score pinned at 0.80 once the adaptive baseline is
// established.
//
// Root cause of the pin: the baseline stored mood=0 (raw signal) but adaptiveScore was
// called with 1-mood=1.0 (penalty direction). For an idle pod where baseline["mood"]=0
// and sigma→0.05 guard, z=(1.0-0)/0.05=20 → clamped to 1.0 → full 0.20 mood penalty →
// healthScore=1-0.20=0.80 every tick regardless of activity. Fixed by storing 1-mood in
// the baseline so z=(1.0-1.0)/0.05=0 → no penalty for a normally-idle workload.
func TestAdaptiveBaseline_IdleWorkload(t *testing.T) {
	a := NewAnalyzer()
	ref := models.WorkloadRef{Namespace: "test", Kind: "host", Name: "idle-pod"}
	// Feed 100 cycles of idle metrics (no cpu, no memory, no requests — pure zeros).
	// 96 cycles is the threshold for baselineReady; 100 ensures it trips.
	idleMetrics := map[string]float64{}
	var lastSnap models.KPISnapshot
	for i := 0; i < 100; i++ {
		lastSnap = a.Update(ref, idleMetrics)
	}
	hs := lastSnap.HealthScore.Value
	// An idle workload with all-zero signals should settle well above 0.80 after
	// the adaptive baseline correctly learns that mood=0 is normal for this workload.
	// Threshold of 0.85 gives headroom for the pressure term (pressureNorm=0.5 before
	// adaptive kicks in, then ≈0 once baseline converges).
	if hs <= 0.80 {
		t.Errorf("idle workload health_score = %.4f, want > 0.80 (adaptive baseline stuck at 0.80 pin)", hs)
	}
}
