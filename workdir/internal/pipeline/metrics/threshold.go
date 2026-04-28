package metrics

import (
	"sync"
	"time"

	"github.com/benfradjselim/kairo-core/pkg/utils"
)

// DynamicThreshold computes adaptive thresholds using mean ± 3σ
type DynamicThreshold struct {
	mu     sync.RWMutex
	buffer *utils.CircularBuffer
}

// NewDynamicThreshold creates a threshold tracker with given history size
func NewDynamicThreshold(historySize int) *DynamicThreshold {
	return &DynamicThreshold{
		buffer: utils.NewCircularBuffer(historySize),
	}
}

// Update adds a new observation
func (dt *DynamicThreshold) Update(value float64) {
	dt.mu.Lock()
	defer dt.mu.Unlock()
	dt.buffer.Push(value)
}

// IsAnomaly returns true if value exceeds mean ± sigma*stddev
func (dt *DynamicThreshold) IsAnomaly(value, sigma float64) bool {
	dt.mu.RLock()
	defer dt.mu.RUnlock()

	values := dt.buffer.Values()
	if len(values) < 10 {
		return false
	}
	mean := utils.Mean(values)
	std := utils.StdDev(values)
	return value > mean+sigma*std || value < mean-sigma*std
}

// UpperBound returns mean + sigma*stddev
func (dt *DynamicThreshold) UpperBound(sigma float64) float64 {
	dt.mu.RLock()
	defer dt.mu.RUnlock()

	values := dt.buffer.Values()
	if len(values) < 2 {
		return 1.0
	}
	return utils.Mean(values) + sigma*utils.StdDev(values)
}

// --- StormDetector watches atmospheric pressure ---

// StormDetector triggers storm warnings when pressure is elevated
type StormDetector struct {
	mu            sync.RWMutex
	pressureBuffer *utils.CircularBuffer
	windowSeconds int
}

// NewStormDetector watches pressure over the given rolling window in seconds
func NewStormDetector(windowSeconds int) *StormDetector {
	return &StormDetector{
		pressureBuffer: utils.NewCircularBuffer(windowSeconds / 15), // assume 15s intervals
		windowSeconds:  windowSeconds,
	}
}

// Update adds a new pressure reading
func (sd *StormDetector) Update(pressure float64) {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	sd.pressureBuffer.Push(pressure)
}

// DetectStorm returns true and ETA if pressure has been above threshold
// for the full window. Threshold 0.1 over 10 minutes → storm in 2 hours.
func (sd *StormDetector) DetectStorm(threshold float64) (detected bool, etaHours float64) {
	sd.mu.RLock()
	defer sd.mu.RUnlock()

	values := sd.pressureBuffer.Values()
	if len(values) < sd.pressureBuffer.Len() {
		return false, 0 // not enough data
	}

	allAbove := true
	for _, v := range values {
		if v < threshold {
			allAbove = false
			break
		}
	}

	if allAbove && len(values) > 0 {
		avgPressure := utils.Mean(values)
		// Scale ETA: higher pressure → sooner storm
		etaHours = utils.Clamp(2.0-(avgPressure-threshold)*10.0, 0.25, 4.0)
		return true, etaHours
	}
	return false, 0
}

// --- AnomalyDetector combines dynamic thresholds ---

// AnomalyResult describes a detected anomaly
type AnomalyResult struct {
	Metric    string
	Value     float64
	Expected  float64
	Deviation float64
	Timestamp time.Time
}

// AnomalyDetector manages per-metric dynamic thresholds
type AnomalyDetector struct {
	mu         sync.RWMutex
	thresholds map[string]*DynamicThreshold
	sigma      float64
}

// NewAnomalyDetector creates a detector with 3σ anomaly detection
func NewAnomalyDetector(sigma float64) *AnomalyDetector {
	return &AnomalyDetector{
		thresholds: make(map[string]*DynamicThreshold),
		sigma:      sigma,
	}
}

// Observe updates the threshold model and returns an anomaly if detected
func (ad *AnomalyDetector) Observe(metric string, value float64) (AnomalyResult, bool) {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	if _, ok := ad.thresholds[metric]; !ok {
		ad.thresholds[metric] = NewDynamicThreshold(300) // 5 min history at 1s
	}
	dt := ad.thresholds[metric]
	dt.Update(value)

	if dt.IsAnomaly(value, ad.sigma) {
		values := dt.buffer.Values()
		expected := utils.Mean(values)
		deviation := (value - expected) / (utils.StdDev(values) + 1e-10)
		return AnomalyResult{
			Metric:    metric,
			Value:     value,
			Expected:  expected,
			Deviation: deviation,
			Timestamp: time.Now(),
		}, true
	}
	return AnomalyResult{}, false
}
