package infra

import (
	"math"
	"testing"
	"time"
)

func TestEBITracker_Norm_NoEvents(t *testing.T) {
	tr := newEBITracker()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	got := tr.Norm("FailedCreate", now)
	if got != 0 {
		t.Errorf("want 0 for no events, got %.4f", got)
	}
}

func TestEBITracker_Norm(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		events      []struct{ reason string; offset time.Duration }
		queryReason string
		wantMin     float64 // norm must be >= wantMin
		wantMax     float64 // norm must be <= wantMax
	}{
		{
			name:        "no events for reason — norm=0",
			events:      nil,
			queryReason: "FailedCreate",
			wantMin:     0, wantMax: 0,
		},
		{
			// Fresh tracker has no prior baseline; epsilon=0.1 is the floor.
			// EBI = 1/0.1 = 10 → norm = clamp((10-1)/9, 0, 1) = 1.0.
			// Spec intent: ε ensures even the first event in a quiet tracker is detected.
			name: "single event in fresh tracker — baseline=epsilon, norm=1.0",
			events: []struct{ reason string; offset time.Duration }{
				{"FailedCreate", -1 * time.Minute},
			},
			queryReason: "FailedCreate",
			wantMin:     0.9, wantMax: 1.0,
		},
		{
			name: "burst: 10 events in window, baseline=1 → EBI=10 → norm=1",
			events: func() []struct{ reason string; offset time.Duration } {
				out := make([]struct{ reason string; offset time.Duration }, 10)
				for i := range out {
					out[i] = struct{ reason string; offset time.Duration }{"Burst", time.Duration(-i-1) * time.Minute}
				}
				return out
			}(),
			queryReason: "Burst",
			wantMin:     0.9, wantMax: 1.0,
		},
		{
			name: "stale events outside window — not counted",
			events: []struct{ reason string; offset time.Duration }{
				{"Old", -10 * time.Minute},
				{"Old", -15 * time.Minute},
			},
			queryReason: "Old",
			wantMin:     0, wantMax: 0.05,
		},
		{
			name: "events for different reason not counted",
			events: []struct{ reason string; offset time.Duration }{
				{"OtherReason", -1 * time.Minute},
				{"OtherReason", -2 * time.Minute},
			},
			queryReason: "TargetReason",
			wantMin:     0, wantMax: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := newEBITracker()
			for _, ev := range tc.events {
				tr.Observe(ev.reason, now.Add(ev.offset))
			}
			got := tr.Norm(tc.queryReason, now)
			if got < tc.wantMin || got > tc.wantMax {
				t.Errorf("Norm = %.4f, want [%.4f, %.4f]", got, tc.wantMin, tc.wantMax)
			}
		})
	}
}

func TestEBITracker_MaxNorm(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	tr := newEBITracker()

	// Add a small burst for reason A and a large burst for reason B.
	tr.Observe("A", now.Add(-1*time.Minute))
	for i := 0; i < 10; i++ {
		tr.Observe("B", now.Add(time.Duration(-i-1)*time.Minute))
	}

	got := tr.MaxNorm(now)
	if got < 0.9 {
		t.Errorf("MaxNorm = %.4f, want >= 0.9 (reason B has burst)", got)
	}
	if got > 1.0 {
		t.Errorf("MaxNorm = %.4f, must be <= 1.0", got)
	}
}

func TestEBITracker_NormClamped(t *testing.T) {
	// Extreme burst: 100 events, baseline=1 → EBI=100, norm must clamp to 1.
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	tr := newEBITracker()
	for i := 0; i < 100; i++ {
		tr.Observe("extreme", now.Add(time.Duration(-i)*time.Second))
	}
	got := tr.Norm("extreme", now)
	if math.Abs(got-1.0) > 0.001 {
		t.Errorf("Norm extreme burst = %.4f, want 1.0 (clamped)", got)
	}
}

func TestEBITracker_ZeroNormFloor(t *testing.T) {
	// Even one event should never produce negative norm.
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	tr := newEBITracker()
	tr.Observe("single", now.Add(-1*time.Minute))
	got := tr.Norm("single", now)
	if got < 0 {
		t.Errorf("Norm = %.4f, must be >= 0", got)
	}
}
