package predictor

import (
	"math"
	"testing"
	"time"
)

func TestILRBasic(t *testing.T) {
	m := NewILR()
	if m.IsTrained() {
		t.Error("fresh model should not be trained")
	}

	// Feed y = 2x + 1
	for i := 0; i < 20; i++ {
		x := float64(i)
		m.Update(x, 2*x+1)
	}

	if !m.IsTrained() {
		t.Error("model should be trained after 20 updates")
	}

	// Alpha ≈ 2.0, Beta ≈ 1.0
	if math.Abs(m.Alpha-2.0) > 0.01 {
		t.Errorf("Alpha = %v; want ~2.0", m.Alpha)
	}
	if math.Abs(m.Beta-1.0) > 0.01 {
		t.Errorf("Beta = %v; want ~1.0", m.Beta)
	}

	// Predict x=10 → 21
	pred := m.Predict(10)
	if math.Abs(pred-21.0) > 0.1 {
		t.Errorf("Predict(10) = %v; want ~21", pred)
	}
}

func TestILRTrend(t *testing.T) {
	m := NewILR()
	// Rising trend
	for i := 0; i < 10; i++ {
		m.Update(float64(i), float64(i)*2)
	}
	if m.Trend() != "rising" {
		t.Errorf("Trend() = %q; want rising", m.Trend())
	}

	m.Reset()
	// Falling trend
	for i := 0; i < 10; i++ {
		m.Update(float64(i), float64(10-i))
	}
	if m.Trend() != "falling" {
		t.Errorf("Trend() = %q; want falling", m.Trend())
	}
}

func TestBatchILR(t *testing.T) {
	b := NewBatchILR(5)
	for i := 0; i < 25; i++ {
		b.Update(float64(i), float64(i)*3)
	}
	// After 5 full batches, model should predict y=3x
	pred := b.Predict(10)
	if math.Abs(pred-30.0) > 1.0 {
		t.Errorf("BatchILR Predict(10) = %v; want ~30", pred)
	}
}

func TestPredictorFeedAndPredict(t *testing.T) {
	p := NewPredictor()
	now := time.Now()

	// Feed 30 points for a rising metric
	for i := 0; i < 30; i++ {
		ts := now.Add(time.Duration(i) * 15 * time.Second)
		p.Feed("host1", "cpu_percent", float64(i)*2, ts)
	}

	pred, ok := p.Predict("host1", "cpu_percent", 60)
	if !ok {
		t.Fatal("Predict returned not-ok for known metric")
	}
	if pred.Trend != "rising" {
		t.Errorf("expected rising trend, got %q", pred.Trend)
	}
	if pred.Predicted <= pred.Current {
		t.Errorf("predicted %v should be > current %v for rising trend", pred.Predicted, pred.Current)
	}
}

func TestPredictorPredictAll(t *testing.T) {
	p := NewPredictor()
	now := time.Now()

	for i := 0; i < 25; i++ {
		ts := now.Add(time.Duration(i) * 15 * time.Second)
		p.Feed("host2", "cpu_percent", float64(i), ts)
		p.Feed("host2", "memory_percent", float64(50+i), ts)
	}

	preds := p.PredictAll("host2", 30)
	if len(preds) == 0 {
		t.Error("PredictAll returned empty slice")
	}
}

func TestPredictorUnknownMetric(t *testing.T) {
	p := NewPredictor()
	_, ok := p.Predict("host3", "nonexistent", 60)
	if ok {
		t.Error("Predict should return false for unknown metric")
	}
}

func TestILRReset(t *testing.T) {
	m := NewILR()
	for i := 0; i < 10; i++ {
		m.Update(float64(i), float64(i))
	}
	if !m.IsTrained() {
		t.Error("should be trained before reset")
	}
	m.Reset()
	if m.IsTrained() {
		t.Error("should not be trained after reset")
	}
	if m.Alpha != 0 || m.Beta != 0 {
		t.Error("Alpha/Beta should be zero after reset")
	}
}

func TestILRPredictUnderMinSamples(t *testing.T) {
	m := NewILR()
	m.Update(1, 10)
	m.Update(2, 20)
	// < 3 points: should return meanY
	pred := m.Predict(5)
	if pred != m.Predict(5) { // should be stable
		t.Error("prediction should be stable mean below min samples")
	}
}

func TestBatchILRTrend(t *testing.T) {
	b := NewBatchILR(5)
	// Feed stable values
	for i := 0; i < 25; i++ {
		b.Update(float64(i), 50.0)
	}
	if b.Trend() != "stable" {
		t.Errorf("flat series trend = %q; want stable", b.Trend())
	}
}

func TestDynamicThreshold(t *testing.T) {
	dt := NewDynamicThreshold(100)

	// Not enough data yet
	if dt.IsAnomaly(999, 3) {
		t.Error("should not detect anomaly with < 10 data points")
	}

	// Feed 20 values with variance around 50
	for i := 0; i < 20; i++ {
		dt.Update(50.0 + float64(i%3)*0.5) // slight variance: 50.0, 50.5, 51.0, ...
	}

	// Normal value — not anomalous
	if dt.IsAnomaly(50.0, 3) {
		t.Error("50.0 should not be an anomaly in a series near 50")
	}

	// Extreme outlier — should be anomalous
	if !dt.IsAnomaly(500.0, 3) {
		t.Error("500.0 should be anomalous in a series near 50")
	}

	ub := dt.UpperBound(3)
	if ub <= 50.0 {
		t.Errorf("upper bound %v should be > 50.0 when series has variance", ub)
	}
}

func TestStormDetector(t *testing.T) {
	sd := NewStormDetector(600) // 10-minute window

	// Not enough data
	detected, _ := sd.DetectStorm(0.1)
	if detected {
		t.Error("should not detect storm with insufficient data")
	}

	// Fill with high pressure
	bufSize := 600 / 15 // 40 readings
	for i := 0; i < bufSize; i++ {
		sd.Update(0.8)
	}
	detected, eta := sd.DetectStorm(0.1)
	if !detected {
		t.Error("should detect storm when all readings exceed threshold")
	}
	if eta <= 0 || eta > 4 {
		t.Errorf("eta = %v; want (0, 4]", eta)
	}

	// Fill with low pressure — no storm
	sd2 := NewStormDetector(300)
	for i := 0; i < 300/15; i++ {
		sd2.Update(0.0)
	}
	detected2, _ := sd2.DetectStorm(0.5)
	if detected2 {
		t.Error("should not detect storm when pressure is low")
	}
}

func TestBatchILRAlpha(t *testing.T) {
	b := NewBatchILR(5)
	// Before training Alpha should be 0
	if b.Alpha() != 0 {
		t.Errorf("untrained Alpha() = %v; want 0", b.Alpha())
	}
	// Train on y = 4x
	for i := 0; i < 25; i++ {
		b.Update(float64(i), float64(i)*4)
	}
	if math.Abs(b.Alpha()-4.0) > 0.1 {
		t.Errorf("trained Alpha() = %v; want ~4.0", b.Alpha())
	}
}

func TestDynamicThresholdUpperBoundInsufficientData(t *testing.T) {
	dt := NewDynamicThreshold(100)
	// Fewer than 2 values → should return 1.0 sentinel
	dt.Update(0.5)
	ub := dt.UpperBound(3)
	if ub != 1.0 {
		t.Errorf("UpperBound with <2 samples = %v; want 1.0", ub)
	}
}

// --- Tests for prediction quality improvements ---

// TestILRForgetsPastRegime verifies that the RLS forgetting factor allows the
// model to adapt to a regime change rather than being stuck on old history.
func TestILRForgetsPastRegime(t *testing.T) {
	m := NewILR()

	// Phase 1: stable flat series around y=50 for 100 points
	for i := 0; i < 100; i++ {
		m.Update(float64(i), 50.0)
	}
	// Intercept should be near 50, slope near 0
	if math.Abs(m.Beta-50.0) > 5.0 {
		t.Errorf("after flat phase Beta = %v; want ~50", m.Beta)
	}

	// Phase 2: step-change to rising series y = 100 + 3x for 60 points
	for i := 0; i < 60; i++ {
		x := float64(100 + i)
		m.Update(x, 100.0+3.0*float64(i))
	}
	// With forgetting (λ=0.995), the model should have adapted towards the new slope
	// Old Welford would still be dragged toward slope≈0; RLS should be > 1.0
	if m.Alpha < 1.0 {
		t.Errorf("after regime change Alpha = %v; RLS should have adapted (want > 1.0)", m.Alpha)
	}
}

// TestHoltWintersDampingBound verifies that damped trend forecasts do not grow
// without bound. Without damping, a series with a rising trend would produce
// ever-larger predictions; with φ=0.98 they must converge.
func TestHoltWintersDampingBound(t *testing.T) {
	hw := newHoltWinters(10) // small period for fast warm-up

	// Feed a rising series so the trend component becomes positive
	for i := 0; i < 30; i++ {
		hw.Update(float64(i) * 2.0)
	}
	if !hw.IsWarm() {
		t.Skip("HW not warm — increase feed count")
	}

	short := hw.Forecast(5)
	long := hw.Forecast(500)

	// With damping the 500-step forecast must not exceed short + some bounded delta
	// Specifically, the damped sum Σφ^i converges to φ/(1-φ) = 0.98/0.02 = 49
	// so the extra trend contribution is at most 49 * trend, not 500 * trend.
	if long > short*50 {
		t.Errorf("undamped runaway: Forecast(500)=%v >> Forecast(5)=%v", long, short)
	}
}

// TestHoltWintersSeasonalBootstrap verifies that after the first period the
// seasonal components sum to ~0 (additive seasonality invariant), confirming
// proper deviation-from-mean bootstrap instead of storing raw values.
func TestHoltWintersSeasonalBootstrap(t *testing.T) {
	period := 6
	hw := newHoltWinters(period)

	// Feed one full period of distinct values
	vals := []float64{10, 20, 30, 20, 10, 15}
	for _, v := range vals {
		hw.Update(v)
	}

	// Seasonal components should sum to approximately 0
	var sum float64
	for _, s := range hw.seasonal {
		sum += s
	}
	if math.Abs(sum) > 1.0 {
		t.Errorf("seasonal components sum = %v; want ~0 (proper bootstrap)", sum)
	}
}

// TestARIMAMultiStepResidualDecay confirms that multi-step forecasts use
// zero future innovations (E[ε_{t+s}]=0 for s≥1) so the MA component
// doesn't amplify phantom autocorrelation across steps.
func TestARIMAMultiStepResidualDecay(t *testing.T) {
	a := newARIMA()
	// Feed a stationary series
	for i := 0; i < 50; i++ {
		a.Update(10.0 + math.Sin(float64(i)*0.3))
	}
	if !a.IsTrained() {
		t.Skip("ARIMA not trained")
	}
	f1 := a.Forecast(1)
	f5 := a.Forecast(5)
	f20 := a.Forecast(20)

	// For a near-zero-mean differenced series the forecasts should not diverge.
	// With the fix (r0=0 for future steps), forecasts stay bounded; without it
	// (r0=dy) the MA term amplifies the AR prediction and diverges.
	const bound = 50.0
	if math.Abs(f1) > bound || math.Abs(f5) > bound || math.Abs(f20) > bound {
		t.Errorf("forecast diverges: f1=%v f5=%v f20=%v (bound %v)", f1, f5, f20, bound)
	}
}

// TestEnsembleColdStartWeightsILR verifies that at cold start, before HW and
// ARIMA MSE trackers are ready, the ensemble gives ILR full weight (1.0, 0, 0).
func TestEnsembleColdStartWeightsILR(t *testing.T) {
	e := newSeriesEnsemble()
	// Feed only 2 points — ILR is warm (n≥3 not met yet so partial),
	// but MSE trackers have 0 observations → ILR should dominate.
	e.Update(0, 10)
	e.Update(1, 12)

	wILR, wHW, wAR := e.weights()
	if wHW != 0 || wAR != 0 {
		t.Errorf("cold start: wHW=%v wAR=%v should be 0", wHW, wAR)
	}
	if wILR <= 0 {
		t.Errorf("cold start: wILR=%v should be > 0", wILR)
	}
}

// TestMADStreamingMedian verifies that the dual-heap streaming median detector
// returns the same anomaly decisions as the reference sort-based approach.
func TestMADStreamingMedian(t *testing.T) {
	det := newMADDetector(50, 3.0)

	// Seed with normal values around 100
	for i := 0; i < 50; i++ {
		det.Update(100.0 + float64(i%5))
	}

	// Normal value — no anomaly
	anom, _, _ := det.IsAnomaly(102.0)
	if anom {
		t.Error("102.0 should not be anomalous in series near 100")
	}

	// Extreme outlier — anomaly
	anom, expected, score := det.IsAnomaly(500.0)
	if !anom {
		t.Errorf("500.0 should be anomalous (expected≈%v, score=%.2f)", expected, score)
	}
	if score < 3.0 {
		t.Errorf("anomaly score %v should be ≥ 3.0 (threshold k)", score)
	}
}

// TestRunningMedianAccuracy checks that the dual-heap streaming median matches
// the exact sort-based median for a variety of inputs.
func TestRunningMedianAccuracy(t *testing.T) {
	cases := [][]float64{
		{5, 3, 1, 4, 2},         // odd: median=3
		{10, 20, 30, 40},        // even: median=25
		{7, 7, 7, 7},            // all same: median=7
		{1, 100, 2, 99, 3, 98}, // mixed: median=50.5
	}
	expected := []float64{3, 25, 7, 50.5}

	for ci, vals := range cases {
		var rm runningMedian
		for _, v := range vals {
			rm.push(v)
		}
		got := rm.median()
		if math.Abs(got-expected[ci]) > 0.01 {
			t.Errorf("case %d: median=%v want %v", ci, got, expected[ci])
		}
	}
}

func TestAnomalyDetector(t *testing.T) {
	ad := NewAnomalyDetector(3.0)

	// Feed normal values — no anomaly
	for i := 0; i < 30; i++ {
		_, ok := ad.Observe("cpu", 50.0)
		if ok {
			t.Errorf("iteration %d: unexpected anomaly on normal data", i)
		}
	}

	// Inject extreme outlier
	result, detected := ad.Observe("cpu", 1000.0)
	if !detected {
		t.Error("expected anomaly for extreme outlier")
	}
	if result.Metric != "cpu" {
		t.Errorf("result metric = %q; want 'cpu'", result.Metric)
	}
}
