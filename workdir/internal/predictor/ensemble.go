package predictor

import (
	"math"
	"time"

	"github.com/benfradjselim/ruptura/pkg/models"
)

// z-score constants for confidence intervals
const (
	z80 = 1.2816
	z95 = 1.9600
)

// seriesEnsemble holds all three forecasters for a single host:metric series.
// Not thread-safe; callers must hold Predictor.mu.
type seriesEnsemble struct {
	ilr  *BatchILR
	hw1h *HoltWinters // 1-hour season (240 samples @ 15s)
	hw24 *HoltWinters // 24-hour season (5760 samples @ 15s)
	ar   *ARIMA

	// running MSE tracker per model for inverse-MSE ensemble weighting
	ilrMSE  *mseTracker
	hwMSE   *mseTracker
	arMSE   *mseTracker

	lastValue float64
	hasValue  bool
}

func newSeriesEnsemble() *seriesEnsemble {
	return &seriesEnsemble{
		ilr:  NewBatchILR(20),
		hw1h: newHoltWinters(240),
		hw24: newHoltWinters(5760),
		ar:   newARIMA(),

		ilrMSE: newMSETracker(),
		hwMSE:  newMSETracker(),
		arMSE:  newMSETracker(),
	}
}

// Update feeds a new (x, y) point. x is elapsed seconds from engine start;
// y is the observed metric value.
func (e *seriesEnsemble) Update(x, y float64) {
	// Track each model's prediction error before updating
	if e.hasValue {
		e.ilrMSE.observe(e.ilr.Predict(x) - y)
		if e.hw1h.IsWarm() {
			e.hwMSE.observe(e.hw1h.Forecast(1) - y)
		}
		if e.ar.IsTrained() {
			e.arMSE.observe(e.ar.Forecast(1) - y)
		}
	}

	e.ilr.Update(x, y)
	e.hw1h.Update(y)
	e.hw24.Update(y)
	e.ar.Update(y)

	e.lastValue = y
	e.hasValue = true
}

// Forecast returns a full ForecastResult for the given horizon in minutes.
// horizonSteps is computed as horizonMin*60/15 (one step = 15s).
func (e *seriesEnsemble) Forecast(host, metric string, horizonMin int, startTS time.Time) models.ForecastResult {
	now := time.Now()
	x := now.Sub(startTS).Seconds()
	steps := (horizonMin * 60) / 15
	if steps < 1 {
		steps = 1
	}

	wILR, wHW, wAR := e.weights()

	// Build forecast points at 1, 5, 10, 30, horizonMin offsets
	offsets := forecastOffsets(horizonMin)
	points := make([]models.ForecastPoint, len(offsets))
	for i, off := range offsets {
		s := (off * 60) / 15
		if s < 1 {
			s = 1
		}
		mean, stddev := e.ensembleMeanStddev(x, s, startTS, wILR, wHW, wAR)
		points[i] = models.ForecastPoint{
			OffsetMinutes: off,
			Mean:          mean,
			Lower80:       mean - z80*stddev,
			Upper80:       mean + z80*stddev,
			Lower95:       mean - z95*stddev,
			Upper95:       mean + z95*stddev,
		}
	}

	// Main prediction (at horizonMin)
	_ = steps

	contribs := []models.ModelContribution{
		{Name: "ilr", Weight: wILR, Mean: e.ilr.Predict(x + float64(horizonMin)*60)},
		{Name: "holt_winters", Weight: wHW, Mean: e.hwForecast(horizonMin)},
		{Name: "arima", Weight: wAR, Mean: e.ar.Forecast(horizonMin * 4)},
	}

	confidence := e.confidence()
	warmingUp := !e.hw1h.IsWarm()

	return models.ForecastResult{
		Host:       host,
		Metric:     metric,
		Current:    e.lastValue,
		Trend:      e.ilr.Trend(),
		Confidence: confidence,
		Models:     contribs,
		Points:     points,
		Timestamp:  now,
		WarmingUp:  warmingUp,
	}
}

// ForecastSingle returns the ensemble mean for horizonMin ahead (for backwards compat).
func (e *seriesEnsemble) ForecastSingle(x float64, horizonMin int, startTS time.Time) (mean, low80, up80, low95, up95 float64) {
	steps := horizonMin * 4
	wILR, wHW, wAR := e.weights()
	m, sd := e.ensembleMeanStddev(x, steps, startTS, wILR, wHW, wAR)
	return m, m - z80*sd, m + z80*sd, m - z95*sd, m + z95*sd
}

func (e *seriesEnsemble) ensembleMeanStddev(x float64, steps int, _ time.Time, wILR, wHW, wAR float64) (float64, float64) {
	xFut := x + float64(steps)*15

	ilrMean := e.ilr.Predict(xFut)
	hwMean := e.hwForecast(steps / 4)
	arMean := e.ar.Forecast(steps)

	mean := wILR*ilrMean + wHW*hwMean + wAR*arMean

	// Pooled variance (weighted)
	ilrSD := e.ilr.ResidualStdDev() * math.Sqrt(float64(steps)/4.0+1)
	hwSD := e.hw1h.ResidualStdDev() * math.Sqrt(float64(steps)/4.0+1)
	arSD := e.ar.ResidualStdDev() * math.Sqrt(float64(steps)/4.0+1)
	stddev := math.Sqrt(wILR*ilrSD*ilrSD + wHW*hwSD*hwSD + wAR*arSD*arSD)

	return mean, stddev
}

func (e *seriesEnsemble) hwForecast(steps int) float64 {
	if e.hw1h.IsWarm() {
		return e.hw1h.Forecast(steps)
	}
	return e.lastValue
}

// weights returns exponentially-decayed inverse-MSE weights for the three models.
// Cold-start logic: a model that hasn't seen enough data yet is excluded from the
// ensemble (weight=0) so the remaining warm models carry full weight. ILR is
// always at least partially weighted once it has 3 points, since it is the
// only model that is available from the very first samples.
func (e *seriesEnsemble) weights() (wILR, wHW, wAR float64) {
	iILR := 0.0
	if e.ilrMSE.ready() {
		iILR = e.ilrMSE.inverseMSE()
	} else if e.ilr.model.IsTrained() {
		// ILR is warm but MSE tracker isn't — give it a modest prior weight
		iILR = 0.5
	}

	iHW := 0.0
	if e.hwMSE.ready() && e.hw1h.IsWarm() {
		iHW = e.hwMSE.inverseMSE()
	}

	iAR := 0.0
	if e.arMSE.ready() && e.ar.IsTrained() {
		iAR = e.arMSE.inverseMSE()
	}

	total := iILR + iHW + iAR
	if total < 1e-12 {
		// Nothing ready yet — ILR gets full weight as the fastest-warming model
		return 1.0, 0.0, 0.0
	}
	return iILR / total, iHW / total, iAR / total
}

// confidence returns the model confidence as 1 / (1 + mean_mse).
func (e *seriesEnsemble) confidence() float64 {
	mse := (e.ilrMSE.meanMSE() + e.hwMSE.meanMSE() + e.arMSE.meanMSE()) / 3
	return 1.0 / (1.0 + mse)
}

// forecastOffsets returns an ascending set of minute offsets up to horizonMin.
func forecastOffsets(horizonMin int) []int {
	bases := []int{1, 5, 10, 30, 60, 120, 360}
	var out []int
	for _, b := range bases {
		if b < horizonMin {
			out = append(out, b)
		}
	}
	out = append(out, horizonMin)
	return out
}

// --- mseTracker: exponentially-decayed mean-squared-error accumulator ---
//
// Uses an EWMA (exponential weighted moving average) of squared residuals so
// recent prediction errors matter more than old ones. This lets the ensemble
// react quickly when a model degrades or improves after a regime change.
//
// decay=0.97 gives an effective window of 1/(1-0.97)≈33 samples (~8 min at 15s).
// obs counts actual observations so cold-start guards can distinguish "no data"
// from "perfect model".
type mseTracker struct {
	ewma  float64 // exponentially weighted MSE estimate
	decay float64 // smoothing factor (0 < decay < 1); default 0.97
	obs   int     // number of observations seen
}

func newMSETracker() *mseTracker {
	return &mseTracker{decay: 0.97, ewma: 1.0} // start at 1.0 (pessimistic prior)
}

func (m *mseTracker) observe(residual float64) {
	sq := residual * residual
	if m.obs == 0 {
		m.ewma = sq
	} else {
		m.ewma = m.decay*m.ewma + (1-m.decay)*sq
	}
	m.obs++
}

func (m *mseTracker) meanMSE() float64 {
	if m.obs == 0 {
		return 1.0 // no data → high MSE (pessimistic)
	}
	return m.ewma
}

func (m *mseTracker) inverseMSE() float64 {
	mse := m.meanMSE()
	if mse < 1e-12 {
		return 1e12
	}
	return 1.0 / mse
}

// ready returns true once the tracker has seen enough observations to be trusted.
func (m *mseTracker) ready() bool { return m.obs >= 5 }
