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
