package predictor

import "math"

// HoltWinters implements damped triple-exponential smoothing with additive seasonality.
// Key improvements over a naive HW:
//   - Proper seasonal bootstrap: seasonal table is initialised as deviations from the
//     first-period mean, not raw values. This prevents the first season from injecting
//     a systematic bias into all subsequent forecasts.
//   - Trend damping (φ=0.98): long-horizon forecasts converge to level + trend/(1-φ)
//     instead of growing without bound. This is critical for infrastructure metrics
//     where unbounded linear extrapolation is almost always wrong beyond ~30 minutes.
//
// All methods are NOT thread-safe; callers must hold the parent Predictor mutex.
type HoltWinters struct {
	alpha  float64 // level smoothing [0.01, 0.99]
	beta   float64 // trend smoothing [0.01, 0.99]
	gamma  float64 // seasonal smoothing [0.01, 0.99]
	phi    float64 // damping factor [0.8, 1.0]; 1.0 = no damping
	period int     // season length in samples

	level    float64
	trend    float64
	seasonal []float64

	// bootstrap accumulator: holds raw values for the first period
	bootBuf []float64
	n       int

	residuals *residualBuffer // last 200 residuals for CI
}

// newHoltWinters creates a HW model with default smoothing parameters.
// phi=0.98 damps trend growth so 60-step forecasts don't diverge.
func newHoltWinters(period int) *HoltWinters {
	if period < 2 {
		period = 2
	}
	return &HoltWinters{
		alpha:     0.3,
		beta:      0.05,
		gamma:     0.2,
		phi:       0.98,
		period:    period,
		seasonal:  make([]float64, period),
		bootBuf:   make([]float64, 0, period),
		residuals: newResidualBuffer(200),
	}
}

// Update incorporates a new observation.
func (h *HoltWinters) Update(y float64) {
	h.n++

	// Bootstrap phase: collect one full period then initialise properly.
	if h.n <= h.period {
		h.bootBuf = append(h.bootBuf, y)
		if h.n == h.period {
			h.initFromBoot()
		}
		return
	}

	prevLevel := h.level
	prevTrend := h.trend
	idx := (h.n - 1) % h.period
	s := h.seasonal[idx]

	alpha := clamp(h.alpha, 0.01, 0.99)
	beta := clamp(h.beta, 0.01, 0.99)
	gamma := clamp(h.gamma, 0.01, 0.99)

	h.level = alpha*(y-s) + (1-alpha)*(prevLevel+h.phi*prevTrend)
	h.trend = beta*(h.level-prevLevel) + (1-beta)*h.phi*prevTrend
	h.seasonal[idx] = gamma*(y-h.level) + (1-gamma)*s

	fitted := prevLevel + h.phi*prevTrend + s
	h.residuals.push(y - fitted)
}

// initFromBoot initialises level, trend, and seasonal components from the
// first-period buffer using the standard decomposition approach:
//   - level = mean of first period
//   - trend = 0 (no trend evidence from a single period)
//   - seasonal[i] = bootBuf[i] - level  (additive deviation from mean)
func (h *HoltWinters) initFromBoot() {
	var sum float64
	for _, v := range h.bootBuf {
		sum += v
	}
	mean := sum / float64(len(h.bootBuf))
	h.level = mean
	h.trend = 0
	for i, v := range h.bootBuf {
		h.seasonal[i] = v - mean
	}
	h.bootBuf = nil // release bootstrap buffer
}

// Forecast returns the predicted value m steps ahead using damped trend.
func (h *HoltWinters) Forecast(steps int) float64 {
	if h.n < h.period {
		if len(h.bootBuf) > 0 {
			return h.bootBuf[len(h.bootBuf)-1]
		}
		return 0
	}
	if steps < 1 {
		steps = 1
	}
	// Damped trend sum: Σ_{i=1}^{steps} φ^i
	dampSum := h.phi * (1 - math.Pow(h.phi, float64(steps))) / (1 - h.phi)
	sIdx := (h.n + steps - 1) % h.period
	return h.level + dampSum*h.trend + h.seasonal[sIdx]
}

// IsWarm returns true once a full season has been observed.
func (h *HoltWinters) IsWarm() bool {
	return h.n > h.period
}

// ResidualStdDev returns the standard deviation of recent residuals.
func (h *HoltWinters) ResidualStdDev() float64 {
	return h.residuals.stddev()
}

// SeasonalComponent returns the current seasonal index for the observation at offset steps ahead.
func (h *HoltWinters) SeasonalComponent(steps int) float64 {
	if len(h.seasonal) == 0 {
		return 0
	}
	sIdx := (h.n + steps - 1) % h.period
	return h.seasonal[sIdx]
}

// --- residualBuffer: lightweight rolling residual tracker ---

type residualBuffer struct {
	data []float64
	size int
	pos  int
	n    int
}

func newResidualBuffer(size int) *residualBuffer {
	return &residualBuffer{data: make([]float64, size), size: size}
}

func (r *residualBuffer) push(v float64) {
	r.data[r.pos] = v
	r.pos = (r.pos + 1) % r.size
	if r.n < r.size {
		r.n++
	}
}

func (r *residualBuffer) values() []float64 {
	if r.n == 0 {
		return nil
	}
	out := make([]float64, r.n)
	if r.n < r.size {
		copy(out, r.data[:r.n])
	} else {
		copy(out, r.data[r.pos:])
		copy(out[r.size-r.pos:], r.data[:r.pos])
	}
	return out
}

func (r *residualBuffer) stddev() float64 {
	vals := r.values()
	if len(vals) < 2 {
		return 0
	}
	var sum, sumSq float64
	for _, v := range vals {
		sum += v
		sumSq += v * v
	}
	n := float64(len(vals))
	variance := sumSq/n - (sum/n)*(sum/n)
	if variance < 0 {
		variance = 0
	}
	return math.Sqrt(variance)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
