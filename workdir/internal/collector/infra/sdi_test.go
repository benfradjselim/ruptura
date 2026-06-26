package infra

import (
	"math"
	"testing"
	"time"
)

func TestComputeSDI(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		conds []sdiCondition
		now   time.Time
		want  float64 // expected SDI, tolerance ±0.001
	}{
		{
			name:  "no conditions — zero",
			conds: []sdiCondition{},
			now:   now,
			want:  0,
		},
		{
			name: "all conditions inactive (onset zero) — zero",
			conds: []sdiCondition{
				{weight: 1.0, severity: 1.0, tRef: 60},
				{weight: 0.8, severity: 0.7, tRef: 120},
			},
			now:  now,
			want: 0,
		},
		{
			name: "single condition saturated (duration >= tRef) — full contribution",
			// weight=1, severity=1, tRef=60s, onset=120s ago → f=1.0, SDI = 1*1*1/1 = 1.0
			conds: []sdiCondition{
				{weight: 1.0, severity: 1.0, tRef: 60, onset: now.Add(-120 * time.Second)},
			},
			now:  now,
			want: 1.0,
		},
		{
			name: "single condition half-saturated",
			// weight=1, severity=1, tRef=60s, onset=30s ago → f=0.5, SDI = 0.5
			conds: []sdiCondition{
				{weight: 1.0, severity: 1.0, tRef: 60, onset: now.Add(-30 * time.Second)},
			},
			now:  now,
			want: 0.5,
		},
		{
			name: "two conditions, one active saturated, one inactive",
			// active: w=1.0, sev=1.0, tRef=60 → contrib=1.0
			// inactive: w=0.8, sev=0.8 → contrib=0
			// denom = 1*1 + 0.8*0.8 = 1.64 → SDI = 1.0/1.64 ≈ 0.610
			conds: []sdiCondition{
				{weight: 1.0, severity: 1.0, tRef: 60, onset: now.Add(-120 * time.Second)},
				{weight: 0.8, severity: 0.8, tRef: 120},
			},
			now:  now,
			want: 1.0 / (1.0 + 0.64),
		},
		{
			name: "both conditions active and saturated — SDI=1",
			conds: []sdiCondition{
				{weight: 1.0, severity: 1.0, tRef: 60, onset: now.Add(-200 * time.Second)},
				{weight: 0.8, severity: 0.8, tRef: 120, onset: now.Add(-300 * time.Second)},
			},
			now:  now,
			want: 1.0,
		},
		{
			name: "onset exactly at now — f=0, SDI=0",
			conds: []sdiCondition{
				{weight: 1.0, severity: 1.0, tRef: 60, onset: now},
			},
			now:  now,
			want: 0,
		},
		{
			name: "node-stress conditions (Ready≠True=1.0/1.0/60s, MemPressure=0.8/0.8/120s) — all saturated",
			// SDI = (1*1 + 0.8*0.8) / (1*1 + 0.8*0.8) = 1.0
			conds: []sdiCondition{
				{weight: 1.0, severity: 1.0, tRef: 60, onset: now.Add(-300 * time.Second)},
				{weight: 0.8, severity: 0.8, tRef: 120, onset: now.Add(-300 * time.Second)},
			},
			now:  now,
			want: 1.0,
		},
		{
			name: "fractional severity — warning (0.7)",
			// weight=1, severity=0.7, tRef=60, onset=60s ago → f=1.0
			// SDI = 1*1*0.7 / (1*0.7) = 1.0
			conds: []sdiCondition{
				{weight: 1.0, severity: 0.7, tRef: 60, onset: now.Add(-60 * time.Second)},
			},
			now:  now,
			want: 1.0,
		},
		{
			name: "future onset (clock skew) — treated as zero duration, f=0",
			conds: []sdiCondition{
				{weight: 1.0, severity: 1.0, tRef: 60, onset: now.Add(30 * time.Second)},
			},
			now:  now,
			want: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := computeSDI(tc.conds, tc.now)
			if math.Abs(got-tc.want) > 0.001 {
				t.Errorf("computeSDI = %.6f, want %.6f", got, tc.want)
			}
		})
	}
}

func TestSDICondition_Contribution(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		cond  sdiCondition
		want  float64
	}{
		{
			name: "inactive (zero onset)",
			cond: sdiCondition{weight: 1.0, severity: 1.0, tRef: 60},
			want: 0,
		},
		{
			name: "onset=now, f=0",
			cond: sdiCondition{weight: 1.0, severity: 1.0, tRef: 60, onset: now},
			want: 0,
		},
		{
			name: "onset 30s ago, tRef=60s → f=0.5 → contrib=0.5",
			cond: sdiCondition{weight: 1.0, severity: 1.0, tRef: 60, onset: now.Add(-30 * time.Second)},
			want: 0.5,
		},
		{
			name: "saturated at 60s → f=1.0 → contrib=0.8*1.0*0.8=0.64",
			cond: sdiCondition{weight: 0.8, severity: 0.8, tRef: 60, onset: now.Add(-60 * time.Second)},
			want: 0.64,
		},
		{
			name: "oversaturated (duration > tRef) — f capped at 1.0",
			cond: sdiCondition{weight: 1.0, severity: 1.0, tRef: 60, onset: now.Add(-1000 * time.Second)},
			want: 1.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.cond.contribution(now)
			if math.Abs(got-tc.want) > 0.001 {
				t.Errorf("contribution = %.6f, want %.6f", got, tc.want)
			}
		})
	}
}
