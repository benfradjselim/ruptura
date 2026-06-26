package fusion

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/benfradjselim/ruptura/pkg/logger"
	"github.com/benfradjselim/ruptura/pkg/models"
)

const staleThreshold = 5 * time.Minute

type Engine struct {
	mu    sync.RWMutex
	hosts map[string]*hostData
}

type hostData struct {
	metricVal, logVal, traceVal, infraVal float64
	metricTs, logTs, traceTs, infraTs     time.Time
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

// SetInfraR records the infrastructure rupture index for a host.
// When infraR has never been set (or goes stale), FusedR renormalises to the
// v7 0.6/0.2/0.2 split — the 0.30 infra weight is never left dangling.
func (e *Engine) SetInfraR(host string, r float64, ts time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	h := e.getHost(host)
	h.infraVal = r
	h.infraTs = ts
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

	// Staleness check for all signals. Infra updates every 30s by design so it is
	// excluded from the lag check above but still subject to the 5-minute stale window.
	now := time.Now()
	if !data.metricTs.IsZero() && now.Sub(data.metricTs) > staleThreshold {
		data.metricTs = time.Time{}
		data.metricVal = 0
	}
	if !data.logTs.IsZero() && now.Sub(data.logTs) > staleThreshold {
		data.logTs = time.Time{}
		data.logVal = 0
	}
	if !data.traceTs.IsZero() && now.Sub(data.traceTs) > staleThreshold {
		data.traceTs = time.Time{}
		data.traceVal = 0
	}
	if !data.infraTs.IsZero() && now.Sub(data.infraTs) > staleThreshold {
		data.infraTs = time.Time{}
		data.infraVal = 0
	}

	// Count active signals across all four pipelines.
	count := 0
	if !data.metricTs.IsZero() { count++ }
	if !data.logTs.IsZero() { count++ }
	if !data.traceTs.IsZero() { count++ }
	if !data.infraTs.IsZero() { count++ }

	if count < 2 {
		return 0, time.Time{}, fmt.Errorf("fusion: insufficient signals for host %s", host)
	}

	// Conflict check among metric/log/trace when all three are active.
	if !data.metricTs.IsZero() && !data.logTs.IsZero() && !data.traceTs.IsZero() {
		diff1 := math.Abs(data.metricVal - data.logVal)
		diff2 := math.Abs(data.metricVal - data.traceVal)
		diff3 := math.Abs(data.logVal - data.traceVal)
		if diff1 > 2.0 || diff2 > 2.0 || diff3 > 2.0 {
			logger.Default.Warn("fusion: signal conflict detected", "host", host, "r_metric", data.metricVal, "r_log", data.logVal, "r_trace", data.traceVal)
		}
	}

	val, _ := e.fusedR(&data)
	return val, latest, nil
}

// fusedR computes the fused R for a hostData snapshot (caller must not hold e.mu).
//
// When infraTs is zero (never set or went stale), weights renormalise to the v7
// 0.6/0.2/0.2 metric/log/trace split — the 0.30 infra weight is never left dangling.
// When infraTs is valid, proportional 4-signal weights are used (base: metric=0.42,
// log=0.14, trace=0.14, infra=0.30), renormalised over the active subset.
func (e *Engine) fusedR(data *hostData) (float64, bool) {
	d := *data

	now := time.Now()
	if !d.metricTs.IsZero() && now.Sub(d.metricTs) > staleThreshold {
		d.metricTs = time.Time{}
		d.metricVal = 0
	}
	if !d.logTs.IsZero() && now.Sub(d.logTs) > staleThreshold {
		d.logTs = time.Time{}
		d.logVal = 0
	}
	if !d.traceTs.IsZero() && now.Sub(d.traceTs) > staleThreshold {
		d.traceTs = time.Time{}
		d.traceVal = 0
	}
	if !d.infraTs.IsZero() && now.Sub(d.infraTs) > staleThreshold {
		d.infraTs = time.Time{}
		d.infraVal = 0
	}

	// When infraR has never been set (or went stale), fall back to the v7 3-signal logic
	// so the 0.30 base weight is never left dangling over an absent signal.
	if d.infraTs.IsZero() {
		count := 0
		if !d.metricTs.IsZero() { count++ }
		if !d.logTs.IsZero() { count++ }
		if !d.traceTs.IsZero() { count++ }
		if count < 2 {
			return 0, false
		}
		if count == 3 {
			return 0.6*d.metricVal + 0.2*d.logVal + 0.2*d.traceVal, true
		}
		// Two signals: metric=0.75, other=0.25.
		if !d.metricTs.IsZero() {
			if !d.logTs.IsZero() {
				return 0.75*d.metricVal + 0.25*d.logVal, true
			}
			return 0.75*d.metricVal + 0.25*d.traceVal, true
		}
		return 0.5*d.logVal + 0.5*d.traceVal, true
	}

	// infraR is participating: proportional 4-signal weighting.
	// Base weights: metric=0.42, log=0.14, trace=0.14, infra=0.30.
	activeW := 0.30 // infra is active
	activeCount := 1
	if !d.metricTs.IsZero() { activeW += 0.42; activeCount++ }
	if !d.logTs.IsZero()    { activeW += 0.14; activeCount++ }
	if !d.traceTs.IsZero()  { activeW += 0.14; activeCount++ }
	if activeCount < 2 {
		return 0, false
	}
	val := (0.30 / activeW) * d.infraVal
	if !d.metricTs.IsZero() { val += (0.42 / activeW) * d.metricVal }
	if !d.logTs.IsZero()    { val += (0.14 / activeW) * d.logVal }
	if !d.traceTs.IsZero()  { val += (0.14 / activeW) * d.traceVal }
	return val, true
}

// WorkloadState holds the per-signal breakdown for a single workload.
type WorkloadState struct {
	Workload         string    `json:"workload"`
	MetricR          float64   `json:"metric_r"`
	LogR             float64   `json:"log_r"`
	TraceR           float64   `json:"trace_r"`
	InfraR           float64   `json:"infra_r"`
	FusedR           float64   `json:"fused_r"`
	DominantPipeline string    `json:"dominant_pipeline"`
	LastUpdated      time.Time `json:"last_updated"`
}

// dominantPipeline returns the pipeline name with the highest R value among active signals.
func dominantPipeline(d hostData) string {
	best := ""
	var bestVal float64
	if !d.metricTs.IsZero() && d.metricVal > bestVal { bestVal = d.metricVal; best = "metrics" }
	if !d.logTs.IsZero() && d.logVal > bestVal        { bestVal = d.logVal;    best = "logs"    }
	if !d.traceTs.IsZero() && d.traceVal > bestVal    { bestVal = d.traceVal;  best = "traces"  }
	if !d.infraTs.IsZero() && d.infraVal > bestVal    {                         best = "infra"   }
	return best
}

// StateByWorkload returns the full fusion state for a workload key.
// Returns an error if the workload is unknown or has insufficient signals.
func (e *Engine) StateByWorkload(key string) (WorkloadState, error) {
	e.mu.RLock()
	h, ok := e.hosts[key]
	if !ok {
		e.mu.RUnlock()
		return WorkloadState{}, fmt.Errorf("fusion: unknown workload %q", key)
	}
	d := *h
	e.mu.RUnlock()

	fused, ok := e.fusedR(&d)
	if !ok {
		fused = 0
	}

	latest := d.metricTs
	if d.logTs.After(latest)   { latest = d.logTs   }
	if d.traceTs.After(latest) { latest = d.traceTs }

	return WorkloadState{
		Workload:         key,
		MetricR:          d.metricVal,
		LogR:             d.logVal,
		TraceR:           d.traceVal,
		InfraR:           d.infraVal,
		FusedR:           math.Round(fused*1000) / 1000,
		DominantPipeline: dominantPipeline(d),
		LastUpdated:      latest,
	}, nil
}

// Snapshot returns a map of workload key → FusedR value for all known workloads.
func (e *Engine) Snapshot() map[string]float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make(map[string]float64, len(e.hosts))
	for host, h := range e.hosts {
		r, ok := e.fusedR(h)
		if ok {
			out[host] = r
		}
	}
	return out
}

// StartLogWatcher consumes BurstEvents from the correlator and updates logR.
// Call this once at startup in a goroutine.
func (e *Engine) StartLogWatcher(ctx context.Context, events <-chan models.BurstEvent) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-events:
				if !ok {
					return
				}
				// Normalize: Count / BaselineRate - 1.0 gives σ-like distance above baseline
				var logR float64
				if ev.BaselineRate > 0 {
					logR = float64(ev.Count)/ev.BaselineRate - 1.0
				}
				if logR < 0 {
					logR = 0
				}
				e.SetLogR(ev.Service, logR, ev.StartTS)
			}
		}
	}()
}
