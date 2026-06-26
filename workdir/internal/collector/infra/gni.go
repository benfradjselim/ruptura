package infra

import (
	"math"
	"sync"
	"time"
)

const (
	gniWindow          = 5 * time.Minute // sliding window for transition counting
	gniAgitatedGNI     = 0.4             // GNI threshold above which a group is considered agitated
	gniAgitatedHealth  = 0.8             // health threshold: agitated only when still "green"
)

// transitionEntry records one state transition observed in a group.
type transitionEntry struct {
	kind string    // e.g. "Ready:True->False", "Phase:Pending->Bound"
	ts   time.Time
}

// groupNoise maintains the state needed to compute the Group Noise Index (GNI).
//
//	GNI(g) = 0.5 * StateChurn(g) + 0.5 * EventBurst(g)
//
// StateChurn uses Shannon entropy of the transition-type distribution over the
// last 5 min, normalized by log2(N_types). EventBurst is the max EBI across
// the group's objects (passed in from the ebiTracker at call time).
// GNI is used both as a surfaced signal and as the noise amplifier in CGPM.
type groupNoise struct {
	mu          sync.Mutex
	transitions []transitionEntry
}

// newGroupNoise creates an initialized groupNoise tracker.
func newGroupNoise() *groupNoise {
	return &groupNoise{}
}

// RecordTransition records one state/condition transition for GNI StateChurn.
// kind should be a string like "Ready:True->False" or "Phase:Pending->Running".
func (n *groupNoise) RecordTransition(kind string, ts time.Time) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.transitions = append(n.transitions, transitionEntry{kind: kind, ts: ts})
}

// GNI computes the Group Noise Index at time now.
//
//	GNI = 0.5*StateChurn + 0.5*eventBurst
//
// eventBurst is the max EBI norm across the group's objects, provided by the
// caller (typically ebiTracker.MaxNorm). GNI ∈ [0,1].
func (n *groupNoise) GNI(now time.Time, eventBurst float64) float64 {
	churn := n.stateChurn(now)
	gni := 0.5*churn + 0.5*eventBurst
	return math.Max(0, math.Min(1, gni))
}

// stateChurn returns the Shannon entropy of the transition distribution over
// the last 5 minutes, normalized by log2(N_types). Returns 0 when N_types ≤ 1.
func (n *groupNoise) stateChurn(now time.Time) float64 {
	n.mu.Lock()
	defer n.mu.Unlock()

	cutoff := now.Add(-gniWindow)

	// Evict stale transitions and count by kind.
	counts := make(map[string]int)
	var fresh []transitionEntry
	for _, tr := range n.transitions {
		if tr.ts.Before(cutoff) {
			continue
		}
		fresh = append(fresh, tr)
		counts[tr.kind]++
	}
	n.transitions = fresh

	return shannonChurn(counts)
}

// shannonChurn computes H/log2(N) for the given transition type counts.
//
//	H = -Σ p_i * log2(p_i)    (Shannon entropy of the distribution)
//	N = number of distinct transition types
//
// Returns 0 when N ≤ 1 (single repeated transition = no disorder).
// Returns 1 when all types are equally likely (maximum disorder for N types).
// Result is in [0,1].
func shannonChurn(counts map[string]int) float64 {
	n := len(counts)
	if n <= 1 {
		return 0
	}

	total := 0
	for _, c := range counts {
		total += c
	}
	if total == 0 {
		return 0
	}

	var h float64
	ft := float64(total)
	for _, c := range counts {
		if c == 0 {
			continue
		}
		p := float64(c) / ft
		h -= p * math.Log2(p)
	}

	// Normalize: H / log2(N) so that uniform distribution over N types → 1.
	return math.Max(0, math.Min(1, h/math.Log2(float64(n))))
}

// IsAgitated returns true when GNI exceeds the agitation threshold while
// group health is still in the healthy range — the pre-rupture warning state.
func IsAgitated(gni, health float64) bool {
	return gni >= gniAgitatedGNI && health >= gniAgitatedHealth
}
