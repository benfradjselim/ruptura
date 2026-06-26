package infra

import (
	"math"
	"testing"
	"time"
)

// TestShannonChurn covers the core normalization invariants.
func TestShannonChurn(t *testing.T) {
	tests := []struct {
		name    string
		counts  map[string]int
		want    float64
		epsilon float64
	}{
		{
			name:    "empty — zero",
			counts:  map[string]int{},
			want:    0, epsilon: 0,
		},
		{
			name:    "single type repeated — zero (no disorder)",
			counts:  map[string]int{"Ready:True->False": 10},
			want:    0, epsilon: 0,
		},
		{
			name: "two types, uniform distribution — 1.0",
			// H = 1 bit, log2(2)=1 → H/log2(N)=1.0
			counts:  map[string]int{"A": 5, "B": 5},
			want:    1.0, epsilon: 0.001,
		},
		{
			name: "four types, uniform — 1.0",
			counts:  map[string]int{"A": 4, "B": 4, "C": 4, "D": 4},
			want:    1.0, epsilon: 0.001,
		},
		{
			name: "three types, skewed — between 0 and 1",
			// One type dominates: lower entropy than uniform.
			counts:  map[string]int{"A": 10, "B": 1, "C": 1},
			want:    0.5, epsilon: 0.2, // rough bounds; exact: ~0.476
		},
		{
			name: "two types, heavily skewed — near 0",
			// 99:1 split → nearly single type → churn near 0.
			counts:  map[string]int{"A": 99, "B": 1},
			want:    0, epsilon: 0.1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shannonChurn(tc.counts)
			if math.Abs(got-tc.want) > tc.epsilon {
				t.Errorf("shannonChurn = %.6f, want %.6f ±%.3f", got, tc.want, tc.epsilon)
			}
			if got < 0 || got > 1 {
				t.Errorf("shannonChurn = %.6f out of [0,1]", got)
			}
		})
	}
}

func TestGroupNoise_GNI(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		transitions []struct{ kind string; offset time.Duration }
		eventBurst  float64
		wantMin     float64
		wantMax     float64
	}{
		{
			name:       "no transitions, no burst — GNI=0",
			eventBurst: 0,
			wantMin:    0, wantMax: 0,
		},
		{
			name: "single repeated transition — churn=0, burst=0 → GNI=0",
			transitions: []struct{ kind string; offset time.Duration }{
				{"Ready:True->False", -1 * time.Minute},
				{"Ready:True->False", -2 * time.Minute},
			},
			eventBurst: 0,
			wantMin:    0, wantMax: 0.01,
		},
		{
			name: "two uniform transition types — churn=1.0, burst=0 → GNI=0.5",
			transitions: []struct{ kind string; offset time.Duration }{
				{"Ready:True->False", -1 * time.Minute},
				{"Phase:Pending->Running", -2 * time.Minute},
			},
			eventBurst: 0,
			wantMin:    0.49, wantMax: 0.51,
		},
		{
			name:       "no transitions, full burst — GNI=0.5",
			eventBurst: 1.0,
			wantMin:    0.49, wantMax: 0.51,
		},
		{
			name: "uniform transitions + full burst — GNI=1.0",
			transitions: []struct{ kind string; offset time.Duration }{
				{"A", -1 * time.Minute},
				{"B", -2 * time.Minute},
			},
			eventBurst: 1.0,
			wantMin:    0.99, wantMax: 1.0,
		},
		{
			name: "stale transitions outside window not counted",
			transitions: []struct{ kind string; offset time.Duration }{
				{"A", -10 * time.Minute}, // outside 5m window
				{"B", -15 * time.Minute},
			},
			eventBurst: 0,
			wantMin:    0, wantMax: 0.01,
		},
		{
			name: "GNI always in [0,1]",
			transitions: []struct{ kind string; offset time.Duration }{
				{"A", -1 * time.Minute},
				{"B", -1 * time.Minute},
				{"C", -2 * time.Minute},
				{"D", -3 * time.Minute},
			},
			eventBurst: 0.8,
			wantMin:    0, wantMax: 1.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gn := newGroupNoise()
			for _, tr := range tc.transitions {
				gn.RecordTransition(tr.kind, now.Add(tr.offset))
			}
			got := gn.GNI(now, tc.eventBurst)
			if got < 0 || got > 1 {
				t.Errorf("GNI = %.6f out of [0,1]", got)
			}
			if got < tc.wantMin || got > tc.wantMax {
				t.Errorf("GNI = %.6f, want [%.4f, %.4f]", got, tc.wantMin, tc.wantMax)
			}
		})
	}
}

func TestIsAgitated(t *testing.T) {
	tests := []struct {
		name   string
		gni    float64
		health float64
		want   bool
	}{
		{"healthy and quiet — not agitated", 0.1, 0.95, false},
		{"high GNI but degraded health — not agitated (already breaking)", 0.8, 0.5, false},
		{"high GNI and green health — agitated (pre-rupture)", 0.5, 0.9, true},
		{"GNI exactly at threshold, health green — agitated", gniAgitatedGNI, 0.9, true},
		{"GNI just below threshold — not agitated", gniAgitatedGNI - 0.001, 0.9, false},
		{"health exactly at threshold, high GNI — agitated", 0.8, gniAgitatedHealth, true},
		{"health just below threshold, high GNI — not agitated", 0.8, gniAgitatedHealth - 0.001, false},
		{"zero GNI — never agitated", 0, 1.0, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsAgitated(tc.gni, tc.health)
			if got != tc.want {
				t.Errorf("IsAgitated(gni=%.3f, health=%.3f) = %v, want %v", tc.gni, tc.health, got, tc.want)
			}
		})
	}
}
