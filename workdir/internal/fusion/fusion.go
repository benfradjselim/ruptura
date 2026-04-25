package fusion

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/benfradjselim/kairo-core/pkg/logger"
)

type Engine struct {
	mu    sync.RWMutex
	hosts map[string]*hostData
}

type hostData struct {
	metricVal, logVal, traceVal float64
	metricTs, logTs, traceTs    time.Time
}

type FusionEngine interface {
	SetMetricR(host string, r float64, ts time.Time)
	SetLogR(host string, r float64, ts time.Time)
	SetTraceR(host string, r float64, ts time.Time)
	FusedR(host string) (float64, time.Time, error)
}

func NewEngine() *Engine {
	return &Engine{
		hosts: make(map[string]*hostData),
	}
}

func (e *Engine) getHost(host string) *hostData {
	if _, ok := e.hosts[host]; !ok {
		e.hosts[host] = &hostData{}
	}
	return e.hosts[host]
}

func (e *Engine) SetMetricR(host string, r float64, ts time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	h := e.getHost(host)
	h.metricVal = r
	h.metricTs = ts
}

func (e *Engine) SetLogR(host string, r float64, ts time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	h := e.getHost(host)
	h.logVal = r
	h.logTs = ts
}

func (e *Engine) SetTraceR(host string, r float64, ts time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	h := e.getHost(host)
	h.traceVal = r
	h.traceTs = ts
}

func (e *Engine) FusedR(host string) (float64, time.Time, error) {
	e.mu.RLock()
	h, ok := e.hosts[host]
	if !ok {
		e.mu.RUnlock()
		return 0, time.Time{}, fmt.Errorf("fusion: insufficient signals for host %s", host)
	}

	// Make a local copy of hostData to avoid race conditions
	data := *h
	e.mu.RUnlock()

	// Use local data for computations
	
	// Lag check
	latest := data.metricTs
	if data.logTs.After(latest) { latest = data.logTs }
	if data.traceTs.After(latest) { latest = data.traceTs }
	
	if !data.metricTs.IsZero() && latest.Sub(data.metricTs) > 30*time.Second {
		return 0, time.Time{}, fmt.Errorf("fusion: signal lag too large for host %s", host)
	}
	if !data.logTs.IsZero() && latest.Sub(data.logTs) > 30*time.Second {
		return 0, time.Time{}, fmt.Errorf("fusion: signal lag too large for host %s", host)
	}
	if !data.traceTs.IsZero() && latest.Sub(data.traceTs) > 30*time.Second {
		return 0, time.Time{}, fmt.Errorf("fusion: signal lag too large for host %s", host)
	}

	// Insufficient check
	count := 0
	if !data.metricTs.IsZero() { count++ }
	if !data.logTs.IsZero() { count++ }
	if !data.traceTs.IsZero() { count++ }
	
	if count < 2 {
		return 0, time.Time{}, fmt.Errorf("fusion: insufficient signals for host %s", host)
	}

	// Conflict check
	if count == 3 {
		diff1 := math.Abs(data.metricVal - data.logVal)
		diff2 := math.Abs(data.metricVal - data.traceVal)
		diff3 := math.Abs(data.logVal - data.traceVal)
		if diff1 > 2.0 || diff2 > 2.0 || diff3 > 2.0 {
			logger.Default.Warn("fusion: signal conflict detected", "host", host, "r_metric", data.metricVal, "r_log", data.logVal, "r_trace", data.traceVal)
		}
	}

	// Compute weighted average
	var val float64
	if count == 3 {
		val = 0.6*data.metricVal + 0.2*data.logVal + 0.2*data.traceVal
	} else {
		// Only 2 signals
		// Rule: metric=0.75, whichever-other=0.25
		if !data.metricTs.IsZero() {
			if !data.logTs.IsZero() {
				val = 0.75*data.metricVal + 0.25*data.logVal
			} else {
				val = 0.75*data.metricVal + 0.25*data.traceVal
			}
		} else {
			// Log and Trace are the 2 signals
			val = 0.5*data.logVal + 0.5*data.traceVal
		}
	}
	
	return val, latest, nil
}
