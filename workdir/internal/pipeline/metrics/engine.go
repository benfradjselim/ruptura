package metrics

import (
	"fmt"
	"sync"
	"time"

	"github.com/benfradjselim/ruptura/pkg/models"
	"github.com/benfradjselim/ruptura/pkg/rupture"
)

// Engine implements MetricPipeline backed by dual-scale CAILR + ensemble.
type Engine struct {
	mu           sync.RWMutex
	cailr        *cailrStore
	ensemble     map[string]*seriesEnsemble // key: "host:metric"
	start        time.Time
	cfg          EngineConfig
	anomalyEng   *AnomalyEngine
	anomalyStore *AnomalyStore
}

// NewEngineWithConfig constructs a ready-to-use MetricPipeline engine with custom config.
func NewEngineWithConfig(cfg EngineConfig) *Engine {
	cap := cfg.AnomalyStoreCapacity
	if cap <= 0 {
		cap = 1000
	}
	return &Engine{
		cailr:        newCAILRStore(),
		ensemble:     make(map[string]*seriesEnsemble),
		start:        time.Now(),
		cfg:          cfg,
		anomalyEng:   NewAnomalyEngine(),
		anomalyStore: NewAnomalyStore(cap),
	}
}

// NewEngine constructs a ready-to-use MetricPipeline engine.
func NewEngine() *Engine {
	return NewEngineWithConfig(DefaultEngineConfig())
}

// EnsembleMode returns the configured ensemble mode.
func (e *Engine) EnsembleMode() EnsembleMode { return e.cfg.EnsembleMode }

func (e *Engine) key(host, metric string) string { return host + ":" + metric }

// Ingest feeds a new sample into both the CAILR store and the ensemble.
func (e *Engine) Ingest(host, metric string, value float64, ts time.Time) {
	k := e.key(host, metric)
	x := ts.Sub(e.start).Seconds()
	e.cailr.update(k, x, value)

	e.mu.Lock()
	if _, ok := e.ensemble[k]; !ok {
		e.ensemble[k] = newSeriesEnsemble()
	}
	e.ensemble[k].Update(x, value)
	e.mu.Unlock()

	for _, ev := range e.anomalyEng.Observe(host, metric, value, ts) {
		e.anomalyStore.Push(ev)
	}
}

// RecentAnomalies returns anomaly events for the given host since the given time.
func (e *Engine) RecentAnomalies(host string, since time.Time) []models.AnomalyEvent {
	return e.anomalyStore.Query(host, "", nil, since)
}

// RuptureIndex returns R(t) for the given host:metric.
func (e *Engine) RuptureIndex(host, metric string) (float64, error) {
	k := e.key(host, metric)
	e.cailr.mu.RLock()
	c, ok := e.cailr.models[k]
	e.cailr.mu.RUnlock()
	if !ok {
		return 0, fmt.Errorf("metrics: no data for %s", k)
	}
	return rupture.Index(c.AlphaBurst(), c.AlphaStable()), nil
}

// TTF returns the time-to-failure estimate for the given host:metric.
func (e *Engine) TTF(host, metric string) (time.Duration, error) {
	k := e.key(host, metric)
	e.cailr.mu.RLock()
	c, ok := e.cailr.models[k]
	e.cailr.mu.RUnlock()
	if !ok {
		return 0, fmt.Errorf("metrics: no data for %s", k)
	}
	e.mu.RLock()
	ens, hasEns := e.ensemble[k]
	e.mu.RUnlock()
	var current float64
	if hasEns && ens.hasValue {
		current = ens.lastValue
	}
	return rupture.TTF(current, 3600, c.AlphaBurst()), nil
}

// Confidence returns the ensemble confidence score C(t).
func (e *Engine) Confidence(host, metric string) (float64, error) {
	k := e.key(host, metric)
	e.mu.RLock()
	ens, ok := e.ensemble[k]
	e.mu.RUnlock()
	if !ok {
		return 0, fmt.Errorf("metrics: no ensemble for %s", k)
	}
	result := ens.Forecast(host, metric, 5, e.start, e.cfg.EnsembleMode)
	return result.Confidence, nil
}

// AllHosts returns the distinct host names that have ingested at least one metric.
func (e *Engine) AllHosts() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	seen := make(map[string]struct{})
	for k := range e.ensemble {
		if idx := len(k) - 1; idx >= 0 {
			for i, c := range k {
				if c == ':' {
					seen[k[:i]] = struct{}{}
					break
				}
			}
		}
	}
	hosts := make([]string, 0, len(seen))
	for h := range seen {
		hosts = append(hosts, h)
	}
	return hosts
}

// LatestByHost returns the most recent observed value for every metric known for host.
func (e *Engine) LatestByHost(host string) map[string]float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	prefix := host + ":"
	result := make(map[string]float64)
	for k, ens := range e.ensemble {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			if ens.hasValue {
				result[k[len(prefix):]] = ens.lastValue
			}
		}
	}
	return result
}

// SurgeProfile returns the surge classification for the given host:metric.
func (e *Engine) SurgeProfile(host, metric string) (string, error) {
	k := e.key(host, metric)
	e.cailr.mu.RLock()
	c, ok := e.cailr.models[k]
	e.cailr.mu.RUnlock()
	if !ok {
		return "Flat", fmt.Errorf("metrics: no data for %s", k)
	}
	ri := rupture.Index(c.AlphaBurst(), c.AlphaStable())
	return SurgeProfile(c.AlphaBurst(), c.AlphaStable(), ri), nil
}
