// Package dag provides the runtime topology graph and CGPM tick runner for the
// infra collector. It builds per-namespace activation maps from GroupSnapshots
// and delegates the propagation math to the parent infra package.
package dag

import (
	"github.com/benfradjselim/ruptura/internal/collector/infra"
)

// NamespaceInput holds the activation and GNI maps for one namespace, ready
// to be passed into the CGPM propagation functions.
//
// Cluster-scoped group snapshots (GroupSnapshot.Namespace == "") always apply
// to every namespace and are automatically merged in by BuildNamespaceInput.
// When both a cluster-scoped and a namespace-scoped snapshot exist for the same
// group, the higher activation wins (conservative: take the worst signal).
type NamespaceInput struct {
	// Namespace identifies the Kubernetes namespace this input represents.
	Namespace string
	// Activation maps each group to its current activation A(g) = 1 - GroupHealth ∈ [0,1].
	Activation map[string]float64
	// GNI maps each group to its Group Noise Index ∈ [0,1].
	GNI map[string]float64
}

// BuildNamespaceInput constructs activation and GNI maps for a specific namespace
// from a full slice of GroupSnapshots. Rules:
//   - Cluster-scoped snapshots (Namespace="") are included for every namespace.
//   - Namespace-scoped snapshots are included only when Namespace == ns.
//   - When multiple snapshots contribute to the same group, the higher activation
//     and higher GNI are taken (conservative: surface the worst signal).
func BuildNamespaceInput(snapshots []infra.GroupSnapshot, ns string) NamespaceInput {
	activation := make(map[string]float64)
	gni := make(map[string]float64)

	for _, snap := range snapshots {
		if snap.Namespace != "" && snap.Namespace != ns {
			continue
		}
		a := 1.0 - snap.Health
		if a < 0 {
			a = 0
		}
		if existing, ok := activation[snap.Group]; !ok || a > existing {
			activation[snap.Group] = a
		}
		if existingGNI, ok := gni[snap.Group]; !ok || snap.GNI > existingGNI {
			gni[snap.Group] = snap.GNI
		}
	}

	return NamespaceInput{
		Namespace:  ns,
		Activation: activation,
		GNI:        gni,
	}
}
