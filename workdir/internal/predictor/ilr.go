package predictor

import (
	"sync"
	"time"

	"github.com/benfradjselim/ruptura/pkg/models"
)

// ILR implements Incremental Linear Regression using Recursive Least Squares
// with an exponential forgetting factor (λ). Unlike Welford's online algorithm,
// old observations decay so the model tracks non-stationary series (e.g. after
// a deploy or restart) without being dragged by stale history.
//
// λ=1.0 is pure cumulative RLS (equivalent to Welford). λ=0.99 gives ~100-sample
// effective memory. Default λ=0.995 gives ~200-sample window — enough to smooth
// noise while still adapting within ~50 minutes at 15-second intervals.
type ILR struct {
	n     int
	lam   float64 // forgetting factor [0.9, 1.0]
	// RLS state: P is 2×2 covariance matrix (stored as [P00,P01,P10,P11])
	// θ = [slope, intercept]
	theta [2]float64
	p     [4]float64 // 2×2 inverse covariance
	Alpha float64    // slope (trend rate) — alias for theta[0]
	Beta  float64    // intercept (baseline) — alias for theta[1]
}

// NewILR creates a fresh ILR model with default forgetting factor (λ=0.995).
func NewILR() *ILR {
	return NewILRWithLambda(0.995)
}

// NewILRWithLambda creates an ILR model with the given forgetting factor.
// λ closer to 1.0 = longer memory (stable trend); λ closer to 0.9 = short memory (burst trend).
// Effective window size N_eff ≈ 1/(1−λ): λ=0.995→200 samples, λ=0.95→20 samples.
func NewILRWithLambda(lambda float64) *ILR {
	m := &ILR{lam: lambda}
	m.p[0] = 1e6
	m.p[3] = 1e6
	return m
}

// Update adds a single (x, y) data point using the RLS update equations.
func (m *ILR) Update(x, y float64) {
	m.n++
	// Regressor vector φ = [x, 1]
	phi0, phi1 := x, 1.0

	// k = P·φ / (λ + φᵀ·P·φ)
	Pp0 := m.p[0]*phi0 + m.p[1]*phi1
	Pp1 := m.p[2]*phi0 + m.p[3]*phi1
	denom := m.lam + phi0*Pp0 + phi1*Pp1
	if denom < 1e-12 {
		return
	}
	k0 := Pp0 / denom
	k1 := Pp1 / denom

	// Innovation: e = y - φᵀ·θ
	e := y - (m.theta[0]*phi0 + m.theta[1]*phi1)

	// θ ← θ + k·e
	m.theta[0] += k0 * e
	m.theta[1] += k1 * e

	// P ← (P - k·φᵀ·P) / λ
	m.p[0] = (m.p[0] - k0*Pp0) / m.lam
	m.p[1] = (m.p[1] - k0*Pp1) / m.lam
	m.p[2] = (m.p[2] - k1*Pp0) / m.lam
	m.p[3] = (m.p[3] - k1*Pp1) / m.lam

	m.Alpha = m.theta[0]
	m.Beta = m.theta[1]
}

// Predict returns predicted y for given x
func (m *ILR) Predict(x float64) float64 {
	if m.n < 3 {
		return m.Beta // intercept is the current level estimate
	}
	return m.Alpha*x + m.Beta
}

// IsTrained returns true if the model has sufficient data
func (m *ILR) IsTrained() bool {
	return m.n >= 3
}

// Reset clears all state and re-initialises the RLS covariance matrix.
func (m *ILR) Reset() {
	*m = ILR{}
	m.lam = 0.995
	m.p[0] = 1e6
	m.p[3] = 1e6
}

// Trend returns "rising", "stable", or "falling" based on slope
func (m *ILR) Trend() string {
	const threshold = 0.001
	switch {
	case m.Alpha > threshold:
		return "rising"
	case m.Alpha < -threshold:
		return "falling"
	default:
		return "stable"
	}
}

// --- BatchILR wraps ILR with a buffer for incremental batch learning ---

// Point is a (X, Y) pair
type Point struct {
	X float64
	Y float64
}

// BatchILR buffers samples and updates the model every batchSize points.
// All exported methods are NOT independently thread-safe; callers must hold
// the parent Predictor mutex (p.mu) before calling any method. This eliminates
// a double-lock pattern and removes the lock-inversion risk.
type BatchILR struct {
	model     *ILR
	buffer    []Point
	batchSize int
	residuals *residualBuffer // tracks prediction residuals for CI
}

// NewBatchILR creates an incremental batch learner
func NewBatchILR(batchSize int) *BatchILR {
	return &BatchILR{
		model:     NewILR(),
		buffer:    make([]Point, 0, batchSize),
		batchSize: batchSize,
		residuals: newResidualBuffer(200),
	}
}

// Update adds a point; flushes to model when batch is full.
// Caller must hold Predictor.mu (write).
func (b *BatchILR) Update(x, y float64) {
	// Track residual before updating so we measure actual prediction error
	if b.model.IsTrained() {
		b.residuals.push(b.model.Predict(x) - y)
	}
	b.buffer = append(b.buffer, Point{X: x, Y: y})
	if len(b.buffer) >= b.batchSize {
		for _, p := range b.buffer {
			b.model.Update(p.X, p.Y)
		}
		b.buffer = b.buffer[:0]
	}
}

// ResidualStdDev returns the standard deviation of recent ILR prediction errors.
// Caller must hold Predictor.mu.
func (b *BatchILR) ResidualStdDev() float64 {
	return b.residuals.stddev()
}

// Predict returns y for x.
// Caller must hold Predictor.mu (read or write).
func (b *BatchILR) Predict(x float64) float64 {
	return b.model.Predict(x)
}

// Trend returns the current trend direction.
// Caller must hold Predictor.mu (read or write).
func (b *BatchILR) Trend() string {
	return b.model.Trend()
}

// Alpha returns the slope.
// Caller must hold Predictor.mu (read or write).
func (b *BatchILR) Alpha() float64 {
	return b.model.Alpha
}

// ILRResidualStdDev returns the stddev of the ILR model's recent residuals.
// Caller must hold Predictor.mu.
func (b *BatchILR) ILRResidualStdDev() float64 {
	return b.residuals.stddev()
}

// --- Predictor manages ensemble models for all metrics/KPIs ---

// Predictor holds per-metric ensemble models and generates predictions.
// It retains the original ILR-based API for backwards compatibility while
// internally using the full three-model ensemble.
type Predictor struct {
	mu        sync.RWMutex
	ensembles map[string]*seriesEnsemble // key: "host:metric"
	startTS   time.Time
	cailr     *cailrStore // v5.0 dual-scale rupture detector
}

// NewPredictor creates a predictor engine
func NewPredictor() *Predictor {
	return &Predictor{
		ensembles: make(map[string]*seriesEnsemble),
		startTS:   time.Now(),
		cailr:     newCAILRStore(),
	}
}

// Feed adds a new value for a metric at a given timestamp
func (p *Predictor) Feed(host, metric string, value float64, ts time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := host + ":" + metric
	if _, ok := p.ensembles[key]; !ok {
		p.ensembles[key] = newSeriesEnsemble()
	}
	x := ts.Sub(p.startTS).Seconds()
	p.ensembles[key].Update(x, value)

	// v5.0: also feed dual-scale CAILR (uses x=elapsed seconds, y=value)
	p.cailr.update(key, x, value)
}

// Predict returns a prediction for a metric at horizon minutes in the future.
// The response includes CI bands when the ensemble is sufficiently warm.
func (p *Predictor) Predict(host, metric string, horizonMinutes int) (models.Prediction, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	key := host + ":" + metric
	ens, ok := p.ensembles[key]
	if !ok {
		return models.Prediction{}, false
	}

	now := time.Now()
	x := now.Sub(p.startTS).Seconds()
	mean, low80, up80, low95, up95 := ens.ForecastSingle(x, horizonMinutes, p.startTS)

	wILR, wHW, wAR := ens.weights()
	contribs := []models.ModelContribution{
		{Name: "ilr", Weight: wILR, Mean: ens.ilr.Predict(x + float64(horizonMinutes)*60)},
		{Name: "holt_winters", Weight: wHW, Mean: ens.hwForecast(horizonMinutes * 4)},
		{Name: "arima", Weight: wAR, Mean: ens.ar.Forecast(horizonMinutes * 4)},
	}

	return models.Prediction{
		Target:     metric,
		Current:    ens.lastValue,
		Predicted:  mean,
		Horizon:    horizonMinutes,
		Trend:      ens.ilr.Trend(),
		Timestamp:  now,
		Lower80:    low80,
		Upper80:    up80,
		Lower95:    low95,
		Upper95:    up95,
		Confidence: ens.confidence(),
		Method:     "ensemble",
		Models:     contribs,
	}, true
}

// Forecast returns the full ForecastResult (multi-step with confidence bands).
func (p *Predictor) Forecast(host, metric string, horizonMinutes int) (models.ForecastResult, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	key := host + ":" + metric
	ens, ok := p.ensembles[key]
	if !ok {
		return models.ForecastResult{}, false
	}
	return ens.Forecast(host, metric, horizonMinutes, p.startTS), true
}

// PredictAll returns predictions for all known metrics for a host
func (p *Predictor) PredictAll(host string, horizonMinutes int) []models.Prediction {
	p.mu.RLock()
	defer p.mu.RUnlock()

	now := time.Now()
	x := now.Sub(p.startTS).Seconds()

	var preds []models.Prediction
	prefix := host + ":"
	for key, ens := range p.ensembles {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			metric := key[len(prefix):]
			mean, low80, up80, low95, up95 := ens.ForecastSingle(x, horizonMinutes, p.startTS)
			preds = append(preds, models.Prediction{
				Target:     metric,
				Current:    ens.lastValue,
				Predicted:  mean,
				Horizon:    horizonMinutes,
				Trend:      ens.ilr.Trend(),
				Timestamp:  now,
				Lower80:    low80,
				Upper80:    up80,
				Lower95:    low95,
				Upper95:    up95,
				Confidence: ens.confidence(),
				Method:     "ensemble",
			})
		}
	}
	return preds
}
