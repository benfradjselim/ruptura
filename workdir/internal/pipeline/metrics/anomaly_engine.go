package metrics

import (
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
}

// NewAnomalyEngine creates an engine with sane defaults.
func NewAnomalyEngine() *AnomalyEngine {
	return &AnomalyEngine{
		zscore:   NewAnomalyDetector(3.0),
		mad:      NewMADAnomalyDetector(300, 3.0),
		seasonal: NewSeasonalAnomalyDetector(240, 3.0), // 1h season at 15s
	}
}

// Observe runs all three detectors and returns all triggered anomalies.
// Anomalies from ≥2 methods are tagged severity "critical"; single-method ones "warning".
func (e *AnomalyEngine) Observe(host, metric string, value float64, ts time.Time) []models.AnomalyEvent {
	fullMetric := host + ":" + metric

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

	// Escalate to critical if ≥2 methods agree
	if len(results) >= 2 {
		for i := range results {
			results[i].Severity = models.SeverityCritical
		}
	}

	return results
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
