package predictor

import (
	"sync"
	"time"

	"github.com/benfradjselim/ohe/pkg/models"
)

// ILR implements Incremental Linear Regression with O(1) update complexity.
// Based on Welford's online algorithm for computing slope and intercept.
type ILR struct {
	n     int
	meanX float64
	meanY float64
	covXY float64
	varX  float64
	Alpha float64 // slope (trend rate)
	Beta  float64 // intercept (baseline)
}

// NewILR creates a fresh ILR model
func NewILR() *ILR {
	return &ILR{}
}

// Update adds a single (x, y) data point and updates the model
func (m *ILR) Update(x, y float64) {
	m.n++
	oldMeanX := m.meanX
	oldMeanY := m.meanY

	m.meanX = oldMeanX + (x-oldMeanX)/float64(m.n)
	m.meanY = oldMeanY + (y-oldMeanY)/float64(m.n)

	m.covXY += (x - oldMeanX) * (y - m.meanY)
	m.varX += (x - oldMeanX) * (x - m.meanX)

	if m.varX > 1e-12 {
		m.Alpha = m.covXY / m.varX
		m.Beta = m.meanY - m.Alpha*m.meanX
	}
}

// Predict returns predicted y for given x
func (m *ILR) Predict(x float64) float64 {
	if m.n < 3 {
		return m.meanY // return mean if not enough data
	}
	return m.Alpha*x + m.Beta
}

// IsTrained returns true if the model has sufficient data
func (m *ILR) IsTrained() bool {
	return m.n >= 3
}

// Reset clears all state
func (m *ILR) Reset() {
	*m = ILR{}
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
}

// NewBatchILR creates an incremental batch learner
func NewBatchILR(batchSize int) *BatchILR {
	return &BatchILR{
		model:     NewILR(),
		buffer:    make([]Point, 0, batchSize),
		batchSize: batchSize,
	}
}

// Update adds a point; flushes to model when batch is full.
// Caller must hold Predictor.mu (write).
func (b *BatchILR) Update(x, y float64) {
	b.buffer = append(b.buffer, Point{X: x, Y: y})
	if len(b.buffer) >= b.batchSize {
		for _, p := range b.buffer {
			b.model.Update(p.X, p.Y)
		}
		b.buffer = b.buffer[:0]
	}
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

// --- Predictor manages ILR models for all metrics/KPIs ---

// Predictor holds per-metric ILR models and generates predictions
type Predictor struct {
	mu      sync.RWMutex
	models  map[string]*BatchILR // key: "host:metric"
	startTS time.Time
}

// NewPredictor creates a predictor engine
func NewPredictor() *Predictor {
	return &Predictor{
		models:  make(map[string]*BatchILR),
		startTS: time.Now(),
	}
}

// Feed adds a new value for a metric at a given timestamp
func (p *Predictor) Feed(host, metric string, value float64, ts time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := host + ":" + metric
	if _, ok := p.models[key]; !ok {
		p.models[key] = NewBatchILR(20) // batch of 20 = ~5 minutes at 15s interval
	}
	x := ts.Sub(p.startTS).Seconds()
	p.models[key].Update(x, value)
}

// Predict returns a prediction for a metric at horizon minutes in the future
func (p *Predictor) Predict(host, metric string, horizonMinutes int) (models.Prediction, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	key := host + ":" + metric
	m, ok := p.models[key]
	if !ok {
		return models.Prediction{}, false
	}

	now := time.Now()
	x := now.Sub(p.startTS).Seconds()
	xFuture := x + float64(horizonMinutes)*60

	current := m.Predict(x)
	predicted := m.Predict(xFuture)

	return models.Prediction{
		Target:    metric,
		Current:   current,
		Predicted: predicted,
		Horizon:   horizonMinutes,
		Trend:     m.Trend(),
		Timestamp: now,
	}, true
}

// PredictAll returns predictions for all known metrics for a host
func (p *Predictor) PredictAll(host string, horizonMinutes int) []models.Prediction {
	p.mu.RLock()
	defer p.mu.RUnlock()

	now := time.Now()
	x := now.Sub(p.startTS).Seconds()
	xFuture := x + float64(horizonMinutes)*60

	var preds []models.Prediction
	prefix := host + ":"
	for key, m := range p.models {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			metric := key[len(prefix):]
			preds = append(preds, models.Prediction{
				Target:    metric,
				Current:   m.Predict(x),
				Predicted: m.Predict(xFuture),
				Horizon:   horizonMinutes,
				Trend:     m.Trend(),
				Timestamp: now,
			})
		}
	}
	return preds
}
