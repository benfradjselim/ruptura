package analyzer

import (
	"time"

	"github.com/benfradjselim/ruptura/pkg/models"
)

const (
	nearMissLow      = 2.0 // FusedR above this enters near-miss zone
	nearMissHigh     = 3.0 // FusedR above this is a real rupture, not a near-miss
	nearMissRecovery = 1.0 // FusedR below this clears a near-miss
	nearMissWindow   = 7 * 24 * time.Hour
)

// SetSLOConfig registers an SLO contract for a workload. The config is used to
// compute SLOBurnVelocity: the ratio of current error rate to the allowed error rate.
func (a *Analyzer) SetSLOConfig(ref models.WorkloadRef, cfg models.SLOConfig) {
	a.mu.Lock()
	defer a.mu.Unlock()
	ws := a.getOrCreate(ref)
	ws.sloConfig = &cfg
}

// UpdateFusedR notifies the analyzer of the current FusedR value for a workload.
// This drives near-miss tracking (recovery_debt business signal).
func (a *Analyzer) UpdateFusedR(ref models.WorkloadRef, fusedR float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	ws, ok := a.workloads[ref.Key()]
	if !ok {
		return
	}

	switch {
	case fusedR >= nearMissHigh:
		// Confirmed rupture — cancel any active near-miss (fingerprint engine handles recording)
		ws.inNearMiss = false
	case fusedR >= nearMissLow && !ws.inNearMiss:
		// Entered near-miss zone
		ws.inNearMiss = true
	case fusedR < nearMissRecovery && ws.inNearMiss:
		// Recovered from near-miss without rupturing — count it
		ws.nearMissTimes = append(ws.nearMissTimes, time.Now())
		ws.inNearMiss = false
	}
}

// MaybeRecordFingerprint delegates to the fingerprint engine, recording a snapshot
// when FusedR exceeds the rupture threshold and the debounce window has elapsed.
func (a *Analyzer) MaybeRecordFingerprint(snap models.KPISnapshot, fusedR float64) {
	a.fingerprints.maybeRecord(snap, fusedR)
}

// MatchFingerprint returns a PatternMatch if the snapshot resembles a past rupture.
// Returns nil when no historical match exceeds the 0.85 cosine similarity threshold.
func (a *Analyzer) MatchFingerprint(snap models.KPISnapshot, fusedR float64) *models.PatternMatch {
	return a.fingerprints.match(snap, fusedR)
}

// AllFingerprints returns all stored rupture fingerprints.
func (a *Analyzer) AllFingerprints() []models.RuptureFingerprint {
	return a.fingerprints.all()
}

// ComputeBusinessSignals returns the three P1 business-layer KPIs for a workload.
func (a *Analyzer) ComputeBusinessSignals(ref models.WorkloadRef, fusedR float64) models.BusinessSignals {
	a.mu.RLock()
	ws, ok := a.workloads[ref.Key()]
	var sloConfig *models.SLOConfig
	var nearMissTimes []time.Time
	var lastErrRate float64
	if ok {
		sloConfig = ws.sloConfig
		nearMissTimes = ws.nearMissTimes
		if er, hasErr := ws.lastMetrics["error_rate"]; hasErr {
			lastErrRate = er
		}
	}
	var topo TopologySource
	if a.topology != nil {
		topo = a.topology
	}
	a.mu.RUnlock()

	biz := models.BusinessSignals{}

	// --- SLO burn velocity ---
	if sloConfig != nil && sloConfig.TargetPercent > 0 {
		allowedErrRate := 1.0 - sloConfig.TargetPercent/100.0
		if allowedErrRate > 1e-9 {
			biz.SLOBurnVelocity = lastErrRate / allowedErrRate
		}
	}

	// --- Blast radius: count unique downstream workloads in the topology ---
	if topo != nil {
		downstream := make(map[string]struct{})
		for _, edge := range topo.Edges() {
			if edge.From == ref.Name {
				downstream[edge.To] = struct{}{}
			}
		}
		biz.BlastRadius = len(downstream)
	}

	// --- Recovery debt: near-misses in the last 7 days ---
	cutoff := time.Now().Add(-nearMissWindow)
	for _, t := range nearMissTimes {
		if t.After(cutoff) {
			biz.RecoveryDebt++
		}
	}

	return biz
}
