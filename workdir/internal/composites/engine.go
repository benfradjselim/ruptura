package composites

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/benfradjselim/kairo-core/pkg/composites"
)

type CompositeEdge struct {
	From, To string
	Weight   float64
}

type Engine struct {
	mu     sync.RWMutex
	hosts  map[string]*hostData
	ruptures map[string]float64
	cfg    EngineConfig
}

type hostData struct {
	metrics          map[string]float64
	variances        []float64
	nPos, nNeg       int
	edges            []CompositeEdge
	
	// Stateful for Fatigue
	prevFatigue float64
	prevStress  float64
	
	// Stateful for Pressure (EWMA)
	muLat, sigma2Lat   float64
	muErr, sigma2Err   float64
	
	// Stateful for Resilience (Ring buffer)
	stressHistory []stressRecord
}

type stressRecord struct {
	ts    time.Time
	value float64
}

type EngineConfig struct {
	StressThresholds map[string]float64
	FatigueLambda    float64
	ResilienceWindow time.Duration
	ContagionTheta   float64
	HealthWeights    map[string]float64
}

func DefaultConfig() EngineConfig {
	return EngineConfig{
		StressThresholds: composites.DefaultThresholds(),
		FatigueLambda:    0.05,
		ResilienceWindow: 30 * time.Minute,
		ContagionTheta:   1.5,
		HealthWeights:    composites.DefaultHealthWeights(),
	}
}

func NewEngine(cfg EngineConfig) *Engine {
	return &Engine{
		hosts:    make(map[string]*hostData),
		ruptures: make(map[string]float64),
		cfg:      cfg,
	}
}

func (e *Engine) getHost(host string) *hostData {
	if _, ok := e.hosts[host]; !ok {
		e.hosts[host] = &hostData{
			metrics: make(map[string]float64),
		}
	}
	return e.hosts[host]
}

func (e *Engine) UpdateMetrics(host string, factors map[string]float64, ts time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	h := e.getHost(host)
	
	for k, v := range factors {
		h.metrics[k] = v
	}
	
	curStress := composites.Stress(h.metrics, e.cfg.StressThresholds)
	h.prevFatigue = composites.Fatigue(h.prevFatigue, h.prevStress, curStress, e.cfg.FatigueLambda)
	h.prevStress = curStress
	
	h.stressHistory = append(h.stressHistory, stressRecord{ts: ts, value: curStress})
	cutoff := ts.Add(-e.cfg.ResilienceWindow)
	for len(h.stressHistory) > 0 && h.stressHistory[0].ts.Before(cutoff) {
		h.stressHistory = h.stressHistory[1:]
	}
	
	lat := factors["latency"]
	err := factors["error_rate"]
	
	h.muLat = 0.9*h.muLat + 0.1*lat
	h.sigma2Lat = 0.9*h.sigma2Lat + 0.1*math.Pow(lat-h.muLat, 2)
	
	h.muErr = 0.9*h.muErr + 0.1*err
	h.sigma2Err = 0.9*h.sigma2Err + 0.1*math.Pow(err-h.muErr, 2)
}

func (e *Engine) UpdateSentiment(host string, nPos, nNeg int, ts time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	h := e.getHost(host)
	h.nPos, h.nNeg = nPos, nNeg
}

func (e *Engine) UpdateEdges(host string, edges []CompositeEdge) {
	e.mu.Lock()
	defer e.mu.Unlock()
	h := e.getHost(host)
	h.edges = edges
}

func (e *Engine) UpdateVariances(host string, variances []float64, ts time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	h := e.getHost(host)
	h.variances = variances
}

func (e *Engine) UpdateRupture(service string, r float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ruptures[service] = r
}

func (e *Engine) Stress(host string) (float64, error) {
	e.mu.RLock()
	h, ok := e.hosts[host]
	e.mu.RUnlock()
	if !ok || len(h.metrics) == 0 {
		return 0, fmt.Errorf("no metrics for host %s", host)
	}
	return composites.Stress(h.metrics, e.cfg.StressThresholds), nil
}

func (e *Engine) Fatigue(host string) (float64, error) {
	e.mu.RLock()
	h, ok := e.hosts[host]
	e.mu.RUnlock()
	if !ok || len(h.metrics) == 0 {
		return 0, fmt.Errorf("no metrics for host %s", host)
	}
	return h.prevFatigue, nil
}

func (e *Engine) Pressure(host string) (float64, error) {
	e.mu.RLock()
	h, ok := e.hosts[host]
	e.mu.RUnlock()
	if !ok || len(h.metrics) == 0 {
		return 0, fmt.Errorf("no metrics for host %s", host)
	}
	
	sigmaLat := math.Sqrt(h.sigma2Lat)
	if sigmaLat < 1e-6 { sigmaLat = 1.0 }
	latencyZ := (h.metrics["latency"] - h.muLat) / sigmaLat
	
	sigmaErr := math.Sqrt(h.sigma2Err)
	if sigmaErr < 1e-6 { sigmaErr = 1.0 }
	errorZ := (h.metrics["error_rate"] - h.muErr) / sigmaErr
	
	return composites.Pressure(latencyZ, errorZ, 0.5, 0.5), nil
}

func (e *Engine) Contagion(host string) (float64, error) {
	e.mu.RLock()
	h, ok := e.hosts[host]
	ruptures := e.ruptures
	e.mu.RUnlock()
	
	if !ok || len(h.edges) == 0 {
		return 0.0, nil
	}
	
	var sum float64
	for _, edge := range h.edges {
		ri := ruptures[edge.From]
		rj := ruptures[edge.To]
		if ri > e.cfg.ContagionTheta && rj > e.cfg.ContagionTheta {
			sum += edge.Weight
		}
	}
	
	return sum / float64(len(h.edges)), nil
}

func (e *Engine) Resilience(host string) (float64, error) {
	e.mu.RLock()
	h, ok := e.hosts[host]
	e.mu.RUnlock()
	
	if !ok || len(h.stressHistory) == 0 {
		return 1.0, nil
	}
	
	var sum float64
	for _, rec := range h.stressHistory {
		sum += rec.value
	}
	mean := sum / float64(len(h.stressHistory))
	
	return 1.0 / (1.0 + mean), nil
}

func (e *Engine) Entropy(host string) (float64, error) {
	e.mu.RLock()
	h, ok := e.hosts[host]
	e.mu.RUnlock()
	
	if !ok || len(h.variances) == 0 {
		return 0.0, nil
	}
	
	return composites.Entropy(h.variances), nil
}

func (e *Engine) Sentiment(host string) (float64, error) {
	e.mu.RLock()
	h, ok := e.hosts[host]
	e.mu.RUnlock()
	
	if !ok {
		return 0.0, nil
	}
	
	return composites.Sentiment(h.nPos, h.nNeg), nil
}

func (e *Engine) HealthScore(host string) (float64, error) {
	signals := make(map[string]float64)
	
	s, err := e.Stress(host)
	if err == nil { signals["stress"] = s }
	
	f, err := e.Fatigue(host)
	if err == nil { signals["fatigue"] = f }
	
	p, err := e.Pressure(host)
	if err == nil { signals["pressure"] = p }
	
	c, err := e.Contagion(host)
	if err == nil { signals["contagion"] = c }
	
	return composites.HealthScore(signals, e.cfg.HealthWeights), nil
}
