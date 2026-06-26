package dag

import (
	"sync"
	"time"

	"github.com/benfradjselim/ruptura/internal/collector/infra"
)

// PropagationResult holds the CGPM output for one namespace at one tick.
type PropagationResult struct {
	// Namespace identifies the Kubernetes namespace this result covers.
	Namespace string
	// PropPressure maps each group to the propagated pressure it received ∈ [0,1].
	PropPressure map[string]float64
	// BlastRadius maps each activated source group to its downstream blast info.
	BlastRadius map[string]infra.BlastInfo
	// Timestamp is when this result was computed.
	Timestamp time.Time
}

// WorkloadPressure returns the CGPM pressure propagating into grp.workload for
// this namespace. This is the value consumed by the extended contagion signal
// in the analyzer and by fusion.SetInfraR.
func (r PropagationResult) WorkloadPressure() float64 {
	if r.PropPressure == nil {
		return 0
	}
	return r.PropPressure[infra.GroupWorkload]
}

// Propagator runs the CGPM tick for each namespace and caches the latest result.
// It is safe for concurrent use.
type Propagator struct {
	mu      sync.RWMutex
	results map[string]PropagationResult
}

// NewPropagator creates an initialized Propagator with an empty result cache.
func NewPropagator() *Propagator {
	return &Propagator{
		results: make(map[string]PropagationResult),
	}
}

// Tick computes PropPressure and BlastRadius for a namespace from the provided
// GroupSnapshots, caches the result, and returns it. It is safe to call
// concurrently from multiple goroutines (one per namespace).
//
// Cluster-scoped GroupSnapshots (Namespace="") are automatically merged into
// every namespace's computation via BuildNamespaceInput.
func (p *Propagator) Tick(ns string, snapshots []infra.GroupSnapshot) PropagationResult {
	input := BuildNamespaceInput(snapshots, ns)
	pp := infra.ComputePropPressure(input.Activation, input.GNI)
	blast := infra.ComputeBlastRadius(input.Activation, input.GNI)

	result := PropagationResult{
		Namespace:    ns,
		PropPressure: pp,
		BlastRadius:  blast,
		Timestamp:    time.Now(),
	}

	p.mu.Lock()
	p.results[ns] = result
	p.mu.Unlock()

	return result
}

// LastResult returns the most recent PropagationResult for a namespace.
// Returns a zero-value result (WorkloadPressure=0) when Tick has never been
// called for this namespace — the safe healthy default.
func (p *Propagator) LastResult(ns string) PropagationResult {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.results[ns]
}

// AllResults returns a snapshot of all cached results keyed by namespace.
// Safe to call concurrently; returns a shallow copy.
func (p *Propagator) AllResults() map[string]PropagationResult {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make(map[string]PropagationResult, len(p.results))
	for k, v := range p.results {
		out[k] = v
	}
	return out
}
