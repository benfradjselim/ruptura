package composites

import (
	"testing"
	"time"
)

func TestEngine_Stress(t *testing.T) {
	e := NewEngine(DefaultConfig())
	e.UpdateMetrics("h1", map[string]float64{"cpu": 80}, time.Now())
	
	s, err := e.Stress("h1")
	if err != nil {
		t.Fatal(err)
	}
	if s == 0 {
		t.Error("expected stress > 0")
	}
}

func TestEngine_Fatigue(t *testing.T) {
	e := NewEngine(DefaultConfig())
	now := time.Now()
	e.UpdateMetrics("h1", map[string]float64{"cpu": 50}, now)
	f1, _ := e.Fatigue("h1")
	
	e.UpdateMetrics("h1", map[string]float64{"cpu": 80}, now.Add(time.Second))
	f2, _ := e.Fatigue("h1")
	
	if f2 <= f1 {
		t.Errorf("expected fatigue to increase, f1=%f, f2=%f", f1, f2)
	}
}

func TestEngine_Contagion(t *testing.T) {
	e := NewEngine(DefaultConfig())
	e.UpdateEdges("h1", []CompositeEdge{{From: "s1", To: "s2", Weight: 1.0}})
	e.UpdateRupture("s1", 2.0)
	e.UpdateRupture("s2", 2.0)
	
	c, err := e.Contagion("h1")
	if err != nil {
		t.Fatal(err)
	}
	if c == 0 {
		t.Error("expected contagion > 0")
	}
}

func TestEngine_Resilience(t *testing.T) {
	e := NewEngine(DefaultConfig())
	e.UpdateMetrics("h1", map[string]float64{"cpu": 90}, time.Now())
	
	r, err := e.Resilience("h1")
	if err != nil {
		t.Fatal(err)
	}
	if r >= 1.0 {
		t.Errorf("expected resilience < 1.0, got %f", r)
	}
}

func TestEngine_HealthScore(t *testing.T) {
	e := NewEngine(DefaultConfig())
	e.UpdateMetrics("h1", map[string]float64{"cpu": 10}, time.Now())
	
	h, err := e.HealthScore("h1")
	if err != nil {
		t.Fatal(err)
	}
	if h < 0 || h > 100 {
		t.Errorf("expected healthscore in [0, 100], got %f", h)
	}
}

func TestEngine_Sentiment(t *testing.T) {
	e := NewEngine(DefaultConfig())
	e.UpdateSentiment("h1", 10, 5, time.Now())
	s, err := e.Sentiment("h1")
	if err != nil {
		t.Fatal(err)
	}
	if s == 0 {
		t.Error("expected sentiment != 0")
	}
}

func TestEngine_Entropy(t *testing.T) {
	e := NewEngine(DefaultConfig())
	e.UpdateVariances("h1", []float64{1, 1}, time.Now())
	en, err := e.Entropy("h1")
	if err != nil {
		t.Fatal(err)
	}
	if en == 0 {
		t.Error("expected entropy != 0")
	}
}
