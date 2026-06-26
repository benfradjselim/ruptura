package dag

import (
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/collector/infra"
)

func snap(group, ns string, health, gni float64) infra.GroupSnapshot {
	return infra.GroupSnapshot{
		Group:     group,
		Namespace: ns,
		Health:    health,
		GNI:       gni,
		Timestamp: time.Now(),
	}
}

func TestBuildNamespaceInput(t *testing.T) {
	now := time.Now()
	_ = now

	tests := []struct {
		name       string
		snapshots  []infra.GroupSnapshot
		ns         string
		wantAct    map[string]float64 // expected activation (±0.001)
		wantGNI    map[string]float64 // expected GNI (±0.001)
		wantAbsent []string           // groups that must NOT appear in activation
	}{
		{
			name:       "no snapshots — empty maps",
			snapshots:  nil,
			ns:         "default",
			wantAct:    map[string]float64{},
			wantGNI:    map[string]float64{},
		},
		{
			name: "cluster-scoped snapshot applies to every namespace",
			snapshots: []infra.GroupSnapshot{
				snap(infra.GroupControlPlane, "", 0.6, 0.2), // Namespace=""
			},
			ns:      "kube-system",
			wantAct: map[string]float64{infra.GroupControlPlane: 0.4}, // 1-0.6
			wantGNI: map[string]float64{infra.GroupControlPlane: 0.2},
		},
		{
			name: "namespace-scoped snapshot for correct namespace is included",
			snapshots: []infra.GroupSnapshot{
				snap(infra.GroupNetwork, "app-ns", 0.8, 0.1),
			},
			ns:      "app-ns",
			wantAct: map[string]float64{infra.GroupNetwork: 0.2},
			wantGNI: map[string]float64{infra.GroupNetwork: 0.1},
		},
		{
			name: "namespace-scoped snapshot for wrong namespace is excluded",
			snapshots: []infra.GroupSnapshot{
				snap(infra.GroupNetwork, "other-ns", 0.5, 0.3),
			},
			ns:         "app-ns",
			wantAct:    map[string]float64{},
			wantAbsent: []string{infra.GroupNetwork},
		},
		{
			name: "cluster-scoped and namespace-scoped for same group — higher activation wins",
			snapshots: []infra.GroupSnapshot{
				snap(infra.GroupControlPlane, "", 0.9, 0.1),   // activation=0.1
				snap(infra.GroupControlPlane, "ns1", 0.5, 0.4), // activation=0.5 (worse)
			},
			ns:      "ns1",
			wantAct: map[string]float64{infra.GroupControlPlane: 0.5}, // max(0.1, 0.5)
			wantGNI: map[string]float64{infra.GroupControlPlane: 0.4}, // max(0.1, 0.4)
		},
		{
			name: "multiple groups mixed scope",
			snapshots: []infra.GroupSnapshot{
				snap(infra.GroupControlPlane, "", 0.7, 0.0),     // cluster-scoped: activation=0.3
				snap(infra.GroupNetwork, "production", 0.8, 0.2), // ns-scoped match
				snap(infra.GroupStorage, "staging", 0.6, 0.5),    // ns-scoped mismatch
			},
			ns: "production",
			wantAct: map[string]float64{
				infra.GroupControlPlane: 0.3,
				infra.GroupNetwork:      0.2,
			},
			wantGNI: map[string]float64{
				infra.GroupControlPlane: 0.0,
				infra.GroupNetwork:      0.2,
			},
			wantAbsent: []string{infra.GroupStorage},
		},
		{
			name: "fully healthy group — activation=0",
			snapshots: []infra.GroupSnapshot{
				snap(infra.GroupNetwork, "ns", 1.0, 0.0),
			},
			ns:      "ns",
			wantAct: map[string]float64{infra.GroupNetwork: 0.0},
			wantGNI: map[string]float64{infra.GroupNetwork: 0.0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildNamespaceInput(tc.snapshots, tc.ns)
			if got.Namespace != tc.ns {
				t.Errorf("Namespace = %q, want %q", got.Namespace, tc.ns)
			}
			for g, want := range tc.wantAct {
				if diff := got.Activation[g] - want; diff < -0.001 || diff > 0.001 {
					t.Errorf("Activation[%s] = %.4f, want %.4f", g, got.Activation[g], want)
				}
			}
			for g, want := range tc.wantGNI {
				if diff := got.GNI[g] - want; diff < -0.001 || diff > 0.001 {
					t.Errorf("GNI[%s] = %.4f, want %.4f", g, got.GNI[g], want)
				}
			}
			for _, g := range tc.wantAbsent {
				if v, ok := got.Activation[g]; ok && v > 0 {
					t.Errorf("Activation[%s] = %.4f, want absent/zero", g, v)
				}
			}
		})
	}
}
