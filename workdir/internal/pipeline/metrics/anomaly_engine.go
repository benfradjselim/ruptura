package metrics

import (
	"math"
	"sync"
	"time"

	"github.com/benfradjselim/ruptura/pkg/models"
	"github.com/benfradjselim/ruptura/pkg/utils"
)

// AnomalyEngine runs all three anomaly detectors per metric and
// reports consensus anomalies (≥2 methods agree = HighConfidence).
type AnomalyEngine struct {
	zscore   *AnomalyDetector
	mad      *MADAnomalyDetector
	seasonal *SeasonalAnomalyDetector
	counters *counterRates
}

// NewAnomalyEngine creates an engine with sane defaults.
func NewAnomalyEngine() *AnomalyEngine {
	return &AnomalyEngine{
		zscore:   NewAnomalyDetector(3.0),
		mad:      NewMADAnomalyDetector(300, 3.0),
		seasonal: NewSeasonalAnomalyDetector(240, 3.0), // 1h season at 15s
		counters: newCounterRates(),
	}
}

// Observe runs all three detectors and returns all triggered anomalies.
// Anomalies from ≥2 methods are tagged severity "critical"; single-method ones "warning".
func (e *AnomalyEngine) Observe(host, metric string, value float64, ts time.Time) []models.AnomalyEvent {
	fullMetric := host + ":" + metric

	if cumulativeMetrics[metric] {
		rate, ok := e.counters.rate(fullMetric, value, ts)
		if !ok {
			return nil
		}
		value = rate
	}

	var results []models.AnomalyEvent

	// Z-score detector
	if res, ok := e.zscore.Observe(fullMetric, value); ok {
		results = append(results, models.AnomalyEvent{
			Host:      host,
			Metric:    metric,
			Value:     value,
			Expected:  res.Expected,
			Score:     res.Deviation,
			Method:    "zscore",
			Severity:  models.SeverityWarning,
			Timestamp: ts,
		})
	}

	// MAD detector
	if res, ok := e.mad.Observe(fullMetric, value, ts); ok {
		results = append(results, models.AnomalyEvent{
			Host:      host,
			Metric:    metric,
			Value:     value,
			Expected:  res.Expected,
			Score:     res.Score,
			Method:    "mad",
			Severity:  models.SeverityWarning,
			Timestamp: ts,
		})
	}

	// Seasonal detector
	if res, ok := e.seasonal.Observe(fullMetric, value, ts); ok {
		results = append(results, models.AnomalyEvent{
			Host:      host,
			Metric:    metric,
			Value:     value,
			Expected:  res.Expected,
			Score:     res.Score,
			Method:    "seasonal",
			Severity:  models.SeverityWarning,
			Timestamp: ts,
		})
	}

	results = dropNegligible(results)

	// Escalate to critical if ≥2 methods agree
	if len(results) >= 2 {
		for i := range results {
			results[i].Severity = models.SeverityCritical
		}
	}

	return results
}

// cumulativeMetrics lists monotonic counters. Their raw value always drifts away
// from any baseline, so anomaly detection runs on their per-second rate.
var cumulativeMetrics = map[string]bool{
	"k8s.pod.network.io":     true,
	"k8s.pod.cpu.time":       true,
	"k8s.node.network.io":    true,
	"k8s.node.cpu.time":      true,
	"container.cpu.time":     true,
	"k8s.container.cpu.time": true,
}

type counterSample struct {
	value float64
	ts    time.Time
}

// counterRates converts cumulative samples into per-second rates.
type counterRates struct {
	mu   sync.Mutex
	last map[string]counterSample
}

func newCounterRates() *counterRates {
	return &counterRates{last: make(map[string]counterSample)}
}

// rate returns the per-second rate since the previous sample of key. It returns
// false for the first sample, a repeated timestamp and a counter reset.
func (c *counterRates) rate(key string, value float64, ts time.Time) (float64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	prev, ok := c.last[key]
	if ok && !ts.After(prev.ts) {
		return 0, false
	}
	c.last[key] = counterSample{value: value, ts: ts}
	if !ok || value < prev.value {
		return 0, false
	}
	return (value - prev.value) / ts.Sub(prev.ts).Seconds(), true
}

// dropNegligible removes events whose absolute deviation is too small to act
// on, whichever method raised them.
func dropNegligible(events []models.AnomalyEvent) []models.AnomalyEvent {
	kept := events[:0]
	for _, ev := range events {
		if !negligibleDeviation(ev.Value, ev.Expected) {
			kept = append(kept, ev)
		}
	}
	return kept
}

// negligibleDeviation reports whether value is too close to expected to alert.
// Values within the 0 to 1 unit range use the bounded floor. A seasonal
// expected value may dip slightly below 0, so the check uses magnitudes. Other
// values use a fraction of the expected value.
func negligibleDeviation(value, expected float64) bool {
	dev := math.Abs(value - expected)
	if math.Abs(value) <= 1 && math.Abs(expected) <= 1 {
		return dev < madBoundedFraction
	}
	return dev < madMedianFraction*math.Abs(expected)
}

// AnomalyStore is an in-memory ring buffer of recent anomaly events.
// It is safe for concurrent use.
type AnomalyStore struct {
	mu     sync.RWMutex
	events []models.AnomalyEvent
	cap    int
	pos    int
	n      int
}

// NewAnomalyStore creates a store that retains the last cap events.
func NewAnomalyStore(cap int) *AnomalyStore {
	return &AnomalyStore{events: make([]models.AnomalyEvent, cap), cap: cap}
}

// Push adds an anomaly event to the ring buffer.
func (s *AnomalyStore) Push(ev models.AnomalyEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events[s.pos] = ev
	s.pos = (s.pos + 1) % s.cap
	if s.n < s.cap {
		s.n++
	}
}

// Query returns events since 'since', optionally filtered by host/metric/methods.
func (s *AnomalyStore) Query(host, metric string, methods []string, since time.Time) []models.AnomalyEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	methodSet := make(map[string]bool, len(methods))
	for _, m := range methods {
		methodSet[m] = true
	}

	var out []models.AnomalyEvent
	start := (s.pos - s.n + s.cap*2) % s.cap
	for i := 0; i < s.n; i++ {
		ev := s.events[(start+i)%s.cap]
		if ev.Timestamp.Before(since) {
			continue
		}
		if host != "" && ev.Host != host {
			continue
		}
		if metric != "" && ev.Metric != metric {
			continue
		}
		if len(methodSet) > 0 && !methodSet[ev.Method] {
			continue
		}
		out = append(out, ev)
	}
	return out
}

// GenerateID delegates to utils for consistency.
func anomalyID() string {
	return utils.GenerateID(8)
}

var _ = anomalyID // suppress unused warning
