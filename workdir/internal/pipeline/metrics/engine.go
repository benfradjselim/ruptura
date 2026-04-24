package metrics

import (
	"fmt"
	"sync"
	"time"

	"github.com/benfradjselim/kairo-core/pkg/rupture"
)

// Engine implements MetricPipeline backed by dual-scale CAILR + ensemble.
type Engine struct {
	mu       sync.RWMutex
	cailr    *cailrStore
	ensemble map[string]*seriesEnsemble // key: "host:metric"
	start    time.Time
}

// NewEngine constructs a ready-to-use MetricPipeline engine.
func NewEngine() *Engine {
	return &Engine{
		cailr:    newCAILRStore(),
		ensemble: make(map[string]*seriesEnsemble),
		start:    time.Now(),
	}
}

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
	result := ens.Forecast(host, metric, 5, e.start)
	return result.Confidence, nil
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
