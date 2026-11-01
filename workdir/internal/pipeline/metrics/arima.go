package metrics

import "math"

// ARIMA implements an online AR(3)+I(1)+MA(2) model using Yule-Walker coefficient
// estimation. The differenced series is fed to the AR/MA core.
// All methods are NOT thread-safe; callers must hold the parent Predictor mutex.
type ARIMA struct {
	// AR coefficients (p=3) estimated online via Yule-Walker
	phi [3]float64

	// MA coefficients (q=2)
	theta [2]float64

	// Observation buffer for Yule-Walker (last 64 differenced values)
	obs    [64]float64
	obsPos int
	obsN   int

	// Residual/MA state
	residuals [2]float64

	// Previous raw value for differencing
	prev    float64
	hasPrev bool
	n       int

	// Rolling residual buffer for CI computation
	ciResiduals *residualBuffer
}

func newARIMA() *ARIMA {
	return &ARIMA{ciResiduals: newResidualBuffer(200)}
}

// Update ingests a new raw observation.
func (a *ARIMA) Update(y float64) {
	if !a.hasPrev {
		a.prev = y
		a.hasPrev = true
		return
	}

	// First-order differencing
	dy := y - a.prev
	a.prev = y
	a.n++

	// Store differenced observation
	a.obs[a.obsPos] = dy
	a.obsPos = (a.obsPos + 1) % 64
	if a.obsN < 64 {
		a.obsN++
	}

	// Re-estimate AR coefficients every 10 observations (Yule-Walker)
	if a.n%10 == 0 && a.obsN >= 8 {
		a.estimateYuleWalker()
	}

	// Compute AR prediction for current step
	arPred := a.arPredict(0)
	// MA correction
	maPred := a.theta[0]*a.residuals[0] + a.theta[1]*a.residuals[1]

	fitted := arPred + maPred
	residual := dy - fitted

	// Update MA residual state
	a.residuals[1] = a.residuals[0]
	a.residuals[0] = residual

	a.ciResiduals.push(residual)
}

// maxForecastSteps bounds the recursion. It is 2 hours at 15s intervals.
const maxForecastSteps = 480

// Forecast returns the predicted value m steps ahead on the original scale. Each
// step feeds the next one, around the mean change of the buffered series.
func (a *ARIMA) Forecast(steps int) float64 {
	if a.n < 10 || steps < 1 {
		return a.prev
	}
	if steps > maxForecastSteps {
		steps = maxForecastSteps
	}

	drift := a.meanChange()
	var lags [3]float64
	for lag := range lags {
		if lag < a.obsN {
			lags[lag] = a.obs[(a.obsPos-1-lag+64*2)%64] - drift
		}
	}
	total := a.prev
	r0, r1 := a.residuals[0], a.residuals[1]
	for s := 0; s < steps; s++ {
		deviation := a.theta[0]*r0 + a.theta[1]*r1
		for lag, value := range lags {
			deviation += a.phi[lag] * value
		}
		total += drift + deviation
		lags[2], lags[1], lags[0] = lags[1], lags[0], deviation
		// After step 0 the expected future innovation is zero.
		r1, r0 = r0, 0
	}
	return total
}

func (a *ARIMA) meanChange() float64 {
	if a.obsN == 0 {
		return 0
	}
	sum := 0.0
	for i := 0; i < a.obsN; i++ {
		sum += a.obs[(a.obsPos-1-i+64*2)%64]
	}
	return sum / float64(a.obsN)
}

// ResidualStdDev returns stddev of recent one-step residuals.
func (a *ARIMA) ResidualStdDev() float64 {
	return a.ciResiduals.stddev()
}

// IsTrained returns true once enough observations are collected.
func (a *ARIMA) IsTrained() bool {
	return a.n >= 10
}

// estimateYuleWalker computes AR(3) coefficients from the autocorrelation of
// the buffered differenced series. Uses the Yule-Walker normal equations solved
// via Levinson-Durbin recursion.
func (a *ARIMA) estimateYuleWalker() {
	n := a.obsN
	vals := make([]float64, n)
	start := (a.obsPos - n + 64) % 64
	for i := 0; i < n; i++ {
		vals[i] = a.obs[(start+i)%64]
	}

	p := 3
	r := make([]float64, p+1)
	for lag := 0; lag <= p; lag++ {
		var s float64
		cnt := 0
		for i := lag; i < n; i++ {
			s += vals[i] * vals[i-lag]
			cnt++
		}
		if cnt > 0 {
			r[lag] = s / float64(cnt)
		}
	}

	if math.Abs(r[0]) < 1e-12 {
		return // all-zero series
	}

	// Normalize
	for i := 1; i <= p; i++ {
		r[i] /= r[0]
	}
	r[0] = 1.0

	// Levinson-Durbin
	phi := levinsonDurbin(r[1:p+1], p)

	// Stability check: all roots of AR polynomial must be inside unit circle
	if !arStable(phi) {
		return
	}
	copy(a.phi[:], phi)
}

// arPredict predicts the differenced value using current AR state.
func (a *ARIMA) arPredict(ahead int) float64 {
	n := a.obsN
	if n == 0 {
		return 0
	}
	var pred float64
	for lag := 0; lag < 3; lag++ {
		idx := (a.obsPos - 1 - lag - ahead + 64*2) % 64
		if lag+ahead < n {
			pred += a.phi[lag] * a.obs[idx]
		}
	}
	return pred
}

// levinsonDurbin solves the Yule-Walker equations for AR(p).
func levinsonDurbin(r []float64, p int) []float64 {
	phi := make([]float64, p)
	if len(r) == 0 {
		return phi
	}
	phi[0] = r[0]
	err := 1 - r[0]*r[0]
	if err < 1e-12 {
		return phi
	}

	phiOld := []float64{phi[0]}
	for m := 1; m < p; m++ {
		var lambda float64
		for j := 0; j < m; j++ {
			lambda += phiOld[j] * r[m-1-j]
		}
		lambda = (r[m] - lambda) / err
		phiNew := make([]float64, m+1)
		phiNew[m] = lambda
		for j := 0; j < m; j++ {
			phiNew[j] = phiOld[j] - lambda*phiOld[m-1-j]
		}
		err *= 1 - lambda*lambda
		if err < 1e-12 {
			copy(phi, phiNew)
			return phi
		}
		phiOld = phiNew
		copy(phi, phiNew)
	}
	return phi
}

// arStable checks that the AR polynomial has all roots outside the unit circle.
// Uses the simple companion-matrix norm bound: coefficients sum < 1.
func arStable(phi []float64) bool {
	var sum float64
	for _, c := range phi {
		sum += math.Abs(c)
	}
	return sum < 1.0
}
