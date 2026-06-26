package infra

import (
	"math"
	"sync"
	"time"
)

const (
	ebiWindow      = 5 * time.Minute // observation window for event counting
	ebiLambda      = 0.99            // EWMA decay factor for baseline per window
	ebiEpsilon     = 0.1             // minimum baseline rate to avoid division by zero
	ebiNormDivisor = 9.0             // EBI=10 → norm=1.0; EBI=1 → norm=0
)

// ebiEntry holds one event observation.
type ebiEntry struct {
	reason string
	ts     time.Time
}

// ebiTracker tracks Event Burst Index per reason string.
// It maintains a sliding window of observations and an EWMA baseline rate.
//
//	EBI(reason, t)  = count(reason, t−Δt, t) / max(baselineEWMA, ε)
//	ebiNorm         = clamp((EBI−1)/9, 0, 1)   — EBI 1→0, 10→1
//	baseline updated per completed window via EWMA with λ=0.99.
type ebiTracker struct {
	mu       sync.Mutex
	entries  []ebiEntry            // ring of recent observations (across all reasons)
	baseline map[string]float64    // EWMA baseline rate per reason (events/window)
	lastFlush map[string]time.Time // last window boundary per reason
}

// newEBITracker creates an initialized ebiTracker.
func newEBITracker() *ebiTracker {
	return &ebiTracker{
		baseline:  make(map[string]float64),
		lastFlush: make(map[string]time.Time),
	}
}

// Observe records one event for reason at time ts.
func (e *ebiTracker) Observe(reason string, ts time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.entries = append(e.entries, ebiEntry{reason: reason, ts: ts})
}

// Norm returns the normalized EBI for a reason at time now: clamp((EBI-1)/9, 0, 1).
// EBI = count(reason, now-5m, now) / max(baseline, 0.1).
// Norm is 0 when count equals or is below baseline; 1 when count is ≥10× baseline.
func (e *ebiTracker) Norm(reason string, now time.Time) float64 {
	e.mu.Lock()
	defer e.mu.Unlock()

	cutoff := now.Add(-ebiWindow)

	// Count events for this reason in the window and evict stale entries.
	var fresh []ebiEntry
	var count float64
	for _, en := range e.entries {
		if en.ts.Before(cutoff) {
			continue // discard old entries
		}
		fresh = append(fresh, en)
		if en.reason == reason {
			count++
		}
	}
	e.entries = fresh

	// Update EWMA baseline if we have crossed a window boundary.
	e.maybeFlushBaseline(reason, count, now)

	baseline := math.Max(e.baseline[reason], ebiEpsilon)
	ebi := count / baseline
	norm := (ebi - 1) / ebiNormDivisor
	return math.Max(0, math.Min(1, norm))
}

// maybeFlushBaseline updates the per-reason EWMA baseline when a window has elapsed.
// Called with e.mu held.
// On cold start the baseline is seeded at ebiEpsilon (not at the observed count)
// so that an initial burst is correctly detected rather than absorbed into the baseline.
func (e *ebiTracker) maybeFlushBaseline(reason string, count float64, now time.Time) {
	last, ok := e.lastFlush[reason]
	if !ok {
		// Cold start: assume quiet baseline so a burst in the first window is detected.
		e.baseline[reason] = ebiEpsilon
		e.lastFlush[reason] = now
		return
	}
	if now.Sub(last) < ebiWindow {
		return // window not yet complete
	}
	// EWMA update: baseline = λ*baseline + (1-λ)*count
	prev := math.Max(e.baseline[reason], ebiEpsilon)
	e.baseline[reason] = ebiLambda*prev + (1-ebiLambda)*count
	e.lastFlush[reason] = now
}

// MaxNorm returns the maximum normalized EBI across all known reasons at time now.
// Used by GNI to incorporate the event burst signal across an entire group.
func (e *ebiTracker) MaxNorm(now time.Time) float64 {
	e.mu.Lock()
	reasons := make(map[string]struct{})
	for _, en := range e.entries {
		reasons[en.reason] = struct{}{}
	}
	e.mu.Unlock()

	var max float64
	for reason := range reasons {
		if n := e.Norm(reason, now); n > max {
			max = n
		}
	}
	return max
}
