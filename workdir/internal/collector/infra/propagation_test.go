package infra

import (
	"math"
	"testing"
)

func TestComputePropPressure(t *testing.T) {
	tests := []struct {
		name       string
		activation map[string]float64
		gni        map[string]float64
		// wantExact: key -> expected value (tolerance ±0.001)
		wantExact map[string]float64
		// wantMin/Max: key -> inclusive bounds (for computed paths)
		wantBounds map[string][2]float64
		// wantZero: groups that must be 0
		wantZero []string
	}{
		{
			name:       "all healthy — all pressures zero",
			activation: map[string]float64{},
			gni:        map[string]float64{},
			wantZero:   []string{GroupControlPlane, GroupNetwork, GroupStorage, GroupWorkload, GroupAdmission, GroupOperators, GroupTenancy},
		},
		{
			// Single source activated, no noise: pressure = A·ω exactly.
			// controlplane→workload (ω=1.0): 0.6·1.0·1 = 0.6
			// controlplane→network  (ω=0.9): 0.6·0.9·1 = 0.54
			// network→workload: effective(network)=0.54, 0.54·0.9·1 = 0.486
			// workload = max(0.6, 0.486) = 0.6 (direct edge dominates)
			name:       "controlplane active no noise — direct pressure equals A·ω",
			activation: map[string]float64{GroupControlPlane: 0.6},
			gni:        map[string]float64{},
			wantExact: map[string]float64{
				GroupNetwork:  0.6 * 0.9,       // 0.54
				GroupWorkload: 0.6 * 1.0,       // 0.6 (direct controlplane→workload)
			},
			wantZero: []string{GroupStorage, GroupAdmission, GroupOperators, GroupTenancy},
		},
		{
			// Noisy source: GNI=1 → amp=1.5 → pressure = A·ω·1.5.
			// controlplane(A=0.8, GNI=1.0):
			//   →workload: 0.8·1.0·1.5 = 1.2 → clamped to 1.0
			//   →network:  0.8·0.9·1.5 = 1.08 → clamped to 1.0
			name:       "noisy source GNI=1 — downstream pressure = A·ω·1.5 clamped",
			activation: map[string]float64{GroupControlPlane: 0.8},
			gni:        map[string]float64{GroupControlPlane: 1.0},
			wantExact: map[string]float64{
				GroupWorkload: 1.0, // clamped from 1.2
				GroupNetwork:  1.0, // clamped from 1.08
			},
			wantZero: []string{GroupStorage, GroupAdmission, GroupOperators, GroupTenancy},
		},
		{
			// Two-hop: operators(A=1.0) → network (ω=0.6) → workload (ω=0.9)
			// network effective = max(own=0, pp=0.6) = 0.6
			// network→workload two-hop: 0.6·0.9 = 0.54
			// operators→workload direct (ω=0.5): 0.5
			// workload = max(0.54, 0.5) = 0.54 (two-hop via network exceeds direct)
			// operators→storage (ω=0.6): storage effective = 0.6
			// storage→workload: 0.6·0.8 = 0.48 (less than 0.54)
			name:       "operators active — two-hop via network exceeds direct to workload",
			activation: map[string]float64{GroupOperators: 1.0},
			gni:        map[string]float64{},
			wantExact: map[string]float64{
				GroupStorage:  0.6,  // operators→storage
				GroupNetwork:  0.6,  // operators→network
				GroupWorkload: 0.54, // max(direct=0.5, via-network=0.54, via-storage=0.48)
			},
			wantZero: []string{GroupControlPlane, GroupAdmission, GroupTenancy},
		},
		{
			// Multi-hop: controlplane fully active propagates through network to workload.
			// The two-hop contribution (via network) is attenuated vs direct.
			// direct: 1.0·1.0 = 1.0 → workload
			// via network: 1.0·0.9=0.9 (network) → 0.9·0.9=0.81 (workload)
			// workload = max(1.0, 0.81) = 1.0 (direct dominates; two-hop is attenuated)
			name:       "controlplane full — two-hop via network is attenuated vs direct",
			activation: map[string]float64{GroupControlPlane: 1.0},
			gni:        map[string]float64{},
			wantExact: map[string]float64{
				GroupNetwork:  0.9, // direct controlplane→network
				GroupWorkload: 1.0, // direct dominates; two-hop (0.81) is attenuated
			},
			wantZero: []string{GroupStorage, GroupAdmission, GroupOperators, GroupTenancy},
		},
		{
			// Admission active: propagates to both network and workload.
			// admission→workload (ω=0.7): 0.5·0.7 = 0.35
			// admission→network  (ω=0.6): 0.5·0.6 = 0.30
			// network→workload: effective(network)=0.30, 0.30·0.9 = 0.27
			// workload = max(0.35, 0.27) = 0.35 (direct wins)
			name:       "admission active — direct to workload beats two-hop via network",
			activation: map[string]float64{GroupAdmission: 0.5},
			gni:        map[string]float64{},
			wantExact: map[string]float64{
				GroupNetwork:  0.5 * 0.6,               // 0.30
				GroupWorkload: 0.5 * 0.7,               // 0.35
			},
			wantZero: []string{GroupControlPlane, GroupStorage, GroupOperators, GroupTenancy},
		},
		{
			// Storage active: propagates only to workload.
			// storage→workload (ω=0.8): 0.4·0.8 = 0.32
			name:       "storage active — pressure into workload only",
			activation: map[string]float64{GroupStorage: 0.4},
			gni:        map[string]float64{},
			wantExact: map[string]float64{
				GroupWorkload: 0.4 * 0.8, // 0.32
			},
			wantZero: []string{GroupControlPlane, GroupNetwork, GroupAdmission, GroupOperators, GroupTenancy},
		},
		{
			// Network active as intermediate: direct network→workload only.
			name:       "network active — pressure into workload",
			activation: map[string]float64{GroupNetwork: 0.7},
			gni:        map[string]float64{},
			wantExact: map[string]float64{
				GroupWorkload: 0.7 * 0.9, // 0.63
			},
			wantZero: []string{GroupControlPlane, GroupStorage, GroupAdmission, GroupOperators, GroupTenancy},
		},
		{
			// Tenancy has no outgoing edges — activating it produces no downstream pressure.
			name:       "tenancy active — no outgoing edges, no downstream pressure",
			activation: map[string]float64{GroupTenancy: 1.0},
			gni:        map[string]float64{},
			wantZero:   []string{GroupNetwork, GroupStorage, GroupWorkload, GroupAdmission, GroupControlPlane, GroupOperators},
		},
		{
			// All sources active, no noise: workload should receive maximum possible pressure (1.0).
			name: "all sources active — workload receives max pressure",
			activation: map[string]float64{
				GroupControlPlane: 1.0,
				GroupOperators:    1.0,
				GroupAdmission:    1.0,
				GroupStorage:      1.0,
				GroupNetwork:      1.0,
			},
			gni: map[string]float64{},
			wantBounds: map[string][2]float64{
				GroupWorkload: {1.0, 1.0}, // must be clamped to 1.0
				GroupNetwork:  {0.9, 1.0}, // at least controlplane→network
				GroupStorage:  {0.6, 1.0}, // at least operators→storage
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputePropPressure(tc.activation, tc.gni)

			for g, want := range tc.wantExact {
				if math.Abs(got[g]-want) > 0.001 {
					t.Errorf("PropPressure[%s] = %.6f, want %.6f", g, got[g], want)
				}
			}
			for g, bounds := range tc.wantBounds {
				v := got[g]
				if v < bounds[0] || v > bounds[1] {
					t.Errorf("PropPressure[%s] = %.6f, want in [%.4f, %.4f]", g, v, bounds[0], bounds[1])
				}
			}
			for _, g := range tc.wantZero {
				if got[g] != 0 {
					t.Errorf("PropPressure[%s] = %.6f, want 0", g, got[g])
				}
			}
			// Invariant: all pressures in [0,1].
			for g, p := range got {
				if p < 0 || p > 1.0+1e-9 {
					t.Errorf("PropPressure[%s] = %.6f out of [0,1]", g, p)
				}
			}
		})
	}
}

func TestComputeBlastRadius(t *testing.T) {
	tests := []struct {
		name          string
		activation    map[string]float64
		gni           map[string]float64
		wantSourceKey string        // which source to inspect
		wantMinGroups int           // minimum GroupsReached for that source
		wantPresence  []string      // groups that must appear in Downstream
		wantAbsence   []string      // sources below thetaBlast — not in result
	}{
		{
			name:          "all healthy — no blast radius",
			activation:    map[string]float64{},
			gni:           map[string]float64{},
			wantAbsence:   []string{GroupControlPlane, GroupOperators, GroupAdmission, GroupStorage, GroupNetwork},
		},
		{
			name:          "low activation below thetaBlast — not reported",
			activation:    map[string]float64{GroupControlPlane: 0.1},
			gni:           map[string]float64{},
			wantAbsence:   []string{GroupControlPlane},
		},
		{
			name:          "controlplane fully active — reaches workload and network",
			activation:    map[string]float64{GroupControlPlane: 1.0},
			gni:           map[string]float64{},
			wantSourceKey: GroupControlPlane,
			wantMinGroups: 2, // network and workload
			wantPresence:  []string{GroupNetwork, GroupWorkload},
		},
		{
			name:          "operators active — reaches storage, network, workload",
			activation:    map[string]float64{GroupOperators: 1.0},
			gni:           map[string]float64{},
			wantSourceKey: GroupOperators,
			wantMinGroups: 3, // storage, network, workload
			wantPresence:  []string{GroupStorage, GroupNetwork, GroupWorkload},
		},
		{
			name:          "storage active — reaches workload only",
			activation:    map[string]float64{GroupStorage: 0.5},
			gni:           map[string]float64{},
			wantSourceKey: GroupStorage,
			wantMinGroups: 1,
			wantPresence:  []string{GroupWorkload},
		},
		{
			name:          "tenancy active — no downstream (no outgoing edges)",
			activation:    map[string]float64{GroupTenancy: 1.0},
			gni:           map[string]float64{},
			wantSourceKey: GroupTenancy,
			wantMinGroups: 0,
		},
		{
			// Noisy controlplane: GNI=1 amplifies pressure.
			// Even at A=0.25: 0.25·1.0·1.5 = 0.375 > thetaBlast → workload reached.
			name:          "noisy source reaches further than quiet one at same activation",
			activation:    map[string]float64{GroupControlPlane: 0.25},
			gni:           map[string]float64{GroupControlPlane: 1.0},
			wantSourceKey: GroupControlPlane,
			wantMinGroups: 1,
			wantPresence:  []string{GroupWorkload},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeBlastRadius(tc.activation, tc.gni)

			// Check absent sources.
			for _, src := range tc.wantAbsence {
				if _, ok := got[src]; ok {
					t.Errorf("blast radius for %s should be absent (activation below thetaBlast or no edges), but was reported", src)
				}
			}

			if tc.wantSourceKey == "" {
				return
			}
			info, ok := got[tc.wantSourceKey]
			if !ok {
				t.Fatalf("blast radius for %s not found in result", tc.wantSourceKey)
			}
			if info.GroupsReached < tc.wantMinGroups {
				t.Errorf("GroupsReached = %d, want >= %d", info.GroupsReached, tc.wantMinGroups)
			}
			for _, g := range tc.wantPresence {
				if _, present := info.Downstream[g]; !present {
					t.Errorf("group %s missing from Downstream of %s (pressure >= %.2f required)", g, tc.wantSourceKey, thetaBlast)
				}
			}
		})
	}
}
