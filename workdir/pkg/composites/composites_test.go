package composites

import (
	"math"
	"testing"
)

func TestStress_pure(t *testing.T) {
	factors := map[string]float64{
		"cpu":        80,
		"memory":     60,
		"io":         100,
		"network":    500,
		"error_rate": 0.1,
	}
	
	// Default thresholds: cpu=100, mem=100, io=200, net=1000, err=1.0
	// g(cpu) = 80/100 = 0.8
	// g(mem) = (60 - 0.5*100) / (0.5*100) = (60-50)/50 = 10/50 = 0.2
	// g(io) = 1 - exp(-100/200) = 1 - exp(-0.5) = 1 - 0.6065 = 0.3935
	// g(net) = 500/1000 = 0.5
	// g(err) = 1 - exp(-3*0.1/1.0) = 1 - exp(-0.3) = 1 - 0.7408 = 0.2592
	//
	// weights: cpu=0.25, mem=0.25, io=0.20, net=0.15, err=0.15
	// sum = 0.25*0.8 + 0.25*0.2 + 0.20*0.3935 + 0.15*0.5 + 0.15*0.2592
	// sum = 0.2 + 0.05 + 0.0787 + 0.075 + 0.03888 = 0.44258
	
	got := Stress(factors, nil)
	expected := 0.44258
	if math.Abs(got-expected) > 1e-3 {
		t.Errorf("expected approx %f, got %f", expected, got)
	}
}

func TestFatigue_pure(t *testing.T) {
	// Fatigue(0,0,0.5,0.05) = 0 + (0.5-0) - 0.05*0 = 0.5
	got := Fatigue(0, 0, 0.5, 0.05)
	if got != 0.5 {
		t.Errorf("expected 0.5, got %f", got)
	}
}

func TestFatigueHalfLife(t *testing.T) {
	// lambda=0.05 → result ≈ 13.86 (tolerance 1e-3)
	got := FatigueHalfLife(0.05)
	expected := 13.8629
	if math.Abs(got-expected) > 1e-3 {
		t.Errorf("expected approx %f, got %f", expected, got)
	}
}

func TestPressure_pure(t *testing.T) {
	// latencyZ=1.0, errorZ=0.5, w=0.5 → (0.5*1.0) + (0.5*0.5) = 0.75
	got := Pressure(1.0, 0.5, 0.5, 0.5)
	if got != 0.75 {
		t.Errorf("expected 0.75, got %f", got)
	}
}

func TestHealthScore_perfect(t *testing.T) {
	// all signals=0 → HealthScore=100
	signals := map[string]float64{
		"stress":     0,
		"fatigue":    0,
		"pressure":   0,
		"contagion":  0,
	}
	got := HealthScore(signals, nil)
	if got != 100.0 {
		t.Errorf("expected 100.0, got %f", got)
	}
}

func TestHealthScore_worst(t *testing.T) {
	// all signals=1 → HealthScore=100 * prod(1-w_i)
	// sum weights = 0.35+0.25+0.25+0.15 = 1.0
	// 100 * (1-0.35)*(1-0.25)*(1-0.25)*(1-0.15)
	// 100 * 0.65 * 0.75 * 0.75 * 0.85 = 100 * 0.31078 = 31.078
	signals := map[string]float64{
		"stress":     1,
		"fatigue":    1,
		"pressure":   1,
		"contagion":  1,
	}
	got := HealthScore(signals, nil)
	expected := 31.078125
	if math.Abs(got-expected) > 1e-3 {
		t.Errorf("expected approx %f, got %f", expected, got)
	}
}

func TestEntropy_pure(t *testing.T) {
	// [1,1,1] → log(3)
	vars := []float64{1, 1, 1}
	got := Entropy(vars)
	expected := math.Log(3)
	if math.Abs(got-expected) > 1e-3 {
		t.Errorf("expected approx %f, got %f", expected, got)
	}
}

func TestSentiment_pure(t *testing.T) {
	// nPos=10,nNeg=5 → log(11)-log(6) ≈ 2.3979 - 1.7917 = 0.6062
	got := Sentiment(10, 5)
	expected := 0.60613
	if math.Abs(got-expected) > 1e-3 {
		t.Errorf("expected approx %f, got %f", expected, got)
	}
}

func TestStress_empty(t *testing.T) {
	got := Stress(map[string]float64{}, nil)
	if got != 0 {
		t.Errorf("expected 0, got %f", got)
	}
}

func TestEntropy_zero(t *testing.T) {
	got := Entropy([]float64{0, 0, 0})
	if got != 0 {
		t.Errorf("expected 0, got %f", got)
	}
}
