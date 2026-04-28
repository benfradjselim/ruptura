package predictor

import (
	"math"
	"sync"
	"time"
)

// SeasonalAnomalyDetector flags observations whose deviation from the
// Holt-Winters seasonal fitted value exceeds 3σ of historical residuals.
// It requires at least one full seasonal period to warm up.
type SeasonalAnomalyDetector struct {
	mu       sync.RWMutex
	models   map[string]*HoltWinters
	period   int
	sigmaK   float64
}

// NewSeasonalAnomalyDetector creates a seasonal anomaly detector.
// period is the season length in samples (240 = 1h at 15s intervals).
func NewSeasonalAnomalyDetector(period int, sigmaK float64) *SeasonalAnomalyDetector {
	if sigmaK <= 0 {
		sigmaK = 3.0
	}
	return &SeasonalAnomalyDetector{
		models: make(map[string]*HoltWinters),
		period: period,
		sigmaK: sigmaK,
	}
}

// Observe updates the HW model and returns an anomaly if the residual exceeds k*σ.
// Returns SeasonalResult{}, false if the model is still warming up.
func (d *SeasonalAnomalyDetector) Observe(metric string, value float64, ts time.Time) (SeasonalResult, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	hw, ok := d.models[metric]
	if !ok {
		hw = newHoltWinters(d.period)
		d.models[metric] = hw
	}

	// Compute expected before updating so we measure out-of-sample residual
	var expected float64
	var isWarm bool
	if hw.IsWarm() {
		expected = hw.Forecast(1)
		isWarm = true
	}

	hw.Update(value)

	if !isWarm {
		return SeasonalResult{WarmingUp: true}, false
	}

	residual := value - expected
	sigma := hw.ResidualStdDev()
	if sigma < 1e-10 {
		return SeasonalResult{}, false
	}

	score := math.Abs(residual) / sigma
	if score <= d.sigmaK {
		return SeasonalResult{}, false
	}

	return SeasonalResult{
		Metric:    metric,
		Value:     value,
		Expected:  expected,
		Residual:  residual,
		Score:     score,
		Timestamp: ts,
	}, true
}

// SeasonalResult is the output of seasonal anomaly detection.
type SeasonalResult struct {
	Metric    string
	Value     float64
	Expected  float64
	Residual  float64
	Score     float64
	WarmingUp bool
	Timestamp time.Time
}
