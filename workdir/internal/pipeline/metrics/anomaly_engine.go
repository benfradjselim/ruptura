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
	throttle *alertThrottle
}

// NewAnomalyEngine creates an engine with sane defaults.
func NewAnomalyEngine() *AnomalyEngine {
	return newAnomalyEngineWithSigma(defaultAnomalySigma)
}

const defaultAnomalySigma = 3.0

// SensitivitySigma converts a sensitivity name to a detection threshold.
// Unknown names use the default.
func SensitivitySigma(sensitivity string) float64 {
	switch sensitivity {
	case "conservative":
		return 4
	case "aggressive":
		return 2.5
	}
	return defaultAnomalySigma
}

func newAnomalyEngineWithSigma(sigma float64) *AnomalyEngine {
	return &AnomalyEngine{
		zscore:   NewAnomalyDetector(sigma),
		mad:      NewMADAnomalyDetector(300, sigma),
		seasonal: NewSeasonalAnomalyDetector(240, sigma), // 1h season at 15s
		counters: newCounterRates(),
		throttle: newAlertThrottle(),
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

	results = dropUnconfirmedSeasonal(dropNegligible(results))

	// Escalate to critical if ≥2 methods agree
	if len(results) >= 2 {
		for i := range results {
			results[i].Severity = models.SeverityCritical
		}
	}

	if len(results) == 0 || !e.throttle.allow(fullMetric, results[0].Severity, ts) {
		return nil
	}
	return results[:1]
}

// alertCooldown is the reminder interval of an anomaly that keeps firing.
const alertCooldown = 5 * time.Minute

type alertState struct {
	severity string
	ts       time.Time
}

// alertThrottle keeps one alert per anomaly episode. A series alerts once, then
// again after alertCooldown or as soon as it escalates to critical.
type alertThrottle struct {
	mu   sync.Mutex
	last map[string]alertState
}

func newAlertThrottle() *alertThrottle {
	return &alertThrottle{last: make(map[string]alertState)}
}

// allow reports whether an anomaly of key at ts must be published.
func (t *alertThrottle) allow(key string, severity string, ts time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	prev, ok := t.last[key]
	escalated := severity == models.SeverityCritical && prev.severity != models.SeverityCritical
	if ok && !escalated && ts.Sub(prev.ts) < alertCooldown {
		return false
	}
	t.last[key] = alertState{severity: severity, ts: ts}
	return true
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

// counterRateWindow is the span of a counter rate. Idle pods send short bursts
// that are normal for them, so a 15 second rate is too heavy tailed to score.
const counterRateWindow = 5 * time.Minute

type counterSample struct {
	value float64
	ts    time.Time
}

// counterRates converts cumulative samples into per-second rates over
// counterRateWindow.
type counterRates struct {
	mu      sync.Mutex
	history map[string][]counterSample
}

func newCounterRates() *counterRates {
	return &counterRates{history: make(map[string][]counterSample)}
}

// rate returns the per-second rate of key over counterRateWindow. It returns
// false for a repeated timestamp, a counter reset and until the window is
// covered.
func (c *counterRates) rate(key string, value float64, ts time.Time) (float64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	history := c.history[key]
	if n := len(history); n > 0 {
		last := history[n-1]
		if !ts.After(last.ts) {
			return 0, false
		}
		if value < last.value {
			history = nil
		}
	}
	history = append(history, counterSample{value: value, ts: ts})

	oldest := 0
	for oldest+1 < len(history) && !history[oldest+1].ts.After(ts.Add(-counterRateWindow)) {
		oldest++
	}
	history = history[oldest:]
	c.history[key] = history

	base := history[0]
	if ts.Sub(base.ts) < counterRateWindow {
		return 0, false
	}
	return (value - base.value) / ts.Sub(base.ts).Seconds(), true
}

// dropUnconfirmedSeasonal removes a seasonal event that no other method raised.
// One hour of history cannot establish an hourly cycle, so the seasonal method
// only confirms the other methods.
func dropUnconfirmedSeasonal(events []models.AnomalyEvent) []models.AnomalyEvent {
	if len(events) == 1 && events[0].Method == "seasonal" {
		return nil
	}
	return events
}

// dropNegligible removes events whose absolute deviation is too small to act
// on, whichever method raised them.
func dropNegligible(events []models.AnomalyEvent) []models.AnomalyEvent {
	kept := events[:0]
	for _, ev := range events {
		if !negligibleDeviation(ev.Metric, ev.Value, ev.Expected) {
			kept = append(kept, ev)
		}
	}
	return kept
}

const (
	// minRateRatio is the change a rate must show, up or down, to be unusual.
	// On the lab, half of the anomalies on low traffic rates were below it.
	minRateRatio = 2.0
	// minCoreChange is the smallest CPU change, in cores, worth reporting. It
	// matches the 100m granularity of CPU requests.
	minCoreChange = 0.1
)

// metricScale says how a metric is compared with its usual value.
type metricScale int

const (
	scaleDefault metricScale = iota
	scaleRate
	scaleCores
)

var metricScales = map[string]metricScale{
	"k8s.pod.network.io":            scaleRate,
	"k8s.node.network.io":           scaleRate,
	"k8s.pod.cpu.time":              scaleCores,
	"k8s.node.cpu.time":             scaleCores,
	"container.cpu.time":            scaleCores,
	"k8s.container.cpu.time":        scaleCores,
	"k8s.pod.cpu.utilization":       scaleCores,
	"k8s.container.cpu.utilization": scaleCores,
}

// changeRatio returns how many times value differs from expected, in either
// direction. A zero on one side counts as a large change.
func changeRatio(value, expected float64) float64 {
	low, high := math.Min(math.Abs(value), math.Abs(expected)), math.Max(math.Abs(value), math.Abs(expected))
	if low == 0 {
		return math.Inf(1)
	}
	return high / low
}

// negligibleDeviation reports whether value is too close to expected to alert.
// Rates and CPU cores need a large relative change, and cores also an absolute
// one. Other values within the 0 to 1 range use the bounded floor, and the rest a
// fraction of the expected value. A seasonal expected value may dip slightly
// below 0, so checks use magnitudes.
func negligibleDeviation(metric string, value, expected float64) bool {
	dev := math.Abs(value - expected)
	switch metricScales[metric] {
	case scaleRate:
		return changeRatio(value, expected) < minRateRatio
	case scaleCores:
		return changeRatio(value, expected) < minRateRatio || dev < minCoreChange
	}
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
