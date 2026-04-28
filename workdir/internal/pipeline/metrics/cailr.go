package metrics

import (
	"math"
	"sync"
	"time"

	"github.com/benfradjselim/kairo-core/pkg/models"
)

// CAILR is the Context-Aware Incremental Linear Regression dual-scale detector (v5.0).
//
// It runs two ILR models in parallel on the same data stream:
//   - stable: λ=0.995, N_eff≈200 samples (~50 min @ 15s) — long-term baseline trend
//   - burst:  λ=0.95,  N_eff≈20 samples  (~5 min @ 15s)  — micro-acceleration, crisis onset
//
// Rupture Index R = α_burst / α_stable.
// When R > 3, the metric is accelerating exponentially faster than its baseline —
// a leading indicator of memory leaks, runaway latency, or cascade failures.
type CAILR struct {
	stable *ILR // long-term trend (λ=0.995)
	burst  *ILR // short-term burst (λ=0.95)
}

// newCAILR returns a fresh dual-scale ILR detector.
// stable λ=0.995 → N_eff≈200 samples (~50 min @ 15s) — long-term baseline
// burst  λ=0.80  → N_eff≈5 samples  (~75s @ 15s)    — ultra-short burst window
// The wide λ gap is intentional: it maximises the rupture index on sudden acceleration.
func newCAILR() *CAILR {
	return &CAILR{
		stable: NewILRWithLambda(0.995),
		burst:  NewILRWithLambda(0.80),
	}
}

// Update feeds a new (x, y) point to both ILR models.
func (c *CAILR) Update(x, y float64) {
	c.stable.Update(x, y)
	c.burst.Update(x, y)
}

// RuptureIndex returns R = α_burst / α_stable.
// Returns 0 when the stable model has insufficient data or near-zero slope
// (avoids division by zero and spurious triggers on flat series).
func (c *CAILR) RuptureIndex() float64 {
	if c.stable.n < 3 || c.burst.n < 3 {
		return 0
	}
	if math.Abs(c.stable.Alpha) < 1e-9 {
		return 0
	}
	return c.burst.Alpha / c.stable.Alpha
}

// IsAccelerating returns true when the rupture index exceeds the canonical threshold (R > 3).
func (c *CAILR) IsAccelerating() bool {
	return c.RuptureIndex() > 3.0
}

// AlphaStable returns the stable model's slope.
func (c *CAILR) AlphaStable() float64 { return c.stable.Alpha }

// AlphaBurst returns the burst model's slope.
func (c *CAILR) AlphaBurst() float64 { return c.burst.Alpha }

// --- Integration into Predictor ---

// cailrState extends Predictor with per-metric dual-scale tracking.
// Stored separately from the ensemble map to avoid coupling.
type cailrStore struct {
	mu               sync.RWMutex
	models           map[string]*CAILR // key: "host:metric"
	ruptureThreshold float64           // default 3.0; configurable via SetRuptureThreshold
}

func newCAILRStore() *cailrStore {
	return &cailrStore{models: make(map[string]*CAILR), ruptureThreshold: 3.0}
}

func (s *cailrStore) update(key string, x, y float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.models[key]; !ok {
		s.models[key] = newCAILR()
	}
	s.models[key].Update(x, y)
}

func (s *cailrStore) ruptureIndex(key string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if c, ok := s.models[key]; ok {
		return c.RuptureIndex()
	}
	return 0
}

// SetRuptureThreshold overrides the R threshold used by AcceleratingMetrics.
// The default is 3.0. Lower values produce earlier (more sensitive) alerts.
func (p *Predictor) SetRuptureThreshold(threshold float64) {
	p.cailr.mu.Lock()
	p.cailr.ruptureThreshold = threshold
	p.cailr.mu.Unlock()
}

// AcceleratingMetrics returns RuptureEvent records for all metrics of a host
// where R > ruptureThreshold (default 3.0).
func (p *Predictor) AcceleratingMetrics(host string) []models.RuptureEvent {
	p.cailr.mu.RLock()
	defer p.cailr.mu.RUnlock()

	threshold := p.cailr.ruptureThreshold
	prefix := host + ":"
	now := time.Now()
	var events []models.RuptureEvent
	for key, c := range p.cailr.models {
		if len(key) <= len(prefix) || key[:len(prefix)] != prefix {
			continue
		}
		if c.RuptureIndex() <= threshold {
			continue
		}
		events = append(events, models.RuptureEvent{
			Host:         host,
			Metric:       key[len(prefix):],
			RuptureIndex: c.RuptureIndex(),
			AlphaStable:  c.AlphaStable(),
			AlphaBurst:   c.AlphaBurst(),
			Timestamp:    now,
		})
	}
	return events
}

// RuptureIndex returns the rupture index for a specific host:metric pair.
func (p *Predictor) RuptureIndex(host, metric string) float64 {
	return p.cailr.ruptureIndex(host + ":" + metric)
}
