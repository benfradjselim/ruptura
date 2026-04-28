package fusion

import (
	"math"
	"sync"
	"testing"
	"time"
)

func TestFusion_weightedAverage(t *testing.T) {
	e := NewEngine()
	now := time.Now()
	e.SetMetricR("h1", 1.0, now)
	e.SetLogR("h1", 2.0, now)
	e.SetTraceR("h1", 3.0, now)
	
	val, _, err := e.FusedR("h1")
	if err != nil {
		t.Fatal(err)
	}
	
	// 0.6*1.0 + 0.2*2.0 + 0.2*3.0 = 0.6 + 0.4 + 0.6 = 1.6
	expected := 1.6
	if math.Abs(val-expected) > 1e-6 {
		t.Errorf("expected %f, got %f", expected, val)
	}
}

func TestFusion_timeLag_rejected(t *testing.T) {
	e := NewEngine()
	now := time.Now()
	e.SetMetricR("h1", 1.0, now)
	e.SetLogR("h1", 2.0, now.Add(-31*time.Second))
	e.SetTraceR("h1", 3.0, now)
	
	_, _, err := e.FusedR("h1")
	if err == nil {
		t.Error("expected error for lag, got nil")
	}
}

func TestFusion_timeLag_accepted(t *testing.T) {
	e := NewEngine()
	now := time.Now()
	e.SetMetricR("h1", 1.0, now)
	e.SetLogR("h1", 2.0, now.Add(-29*time.Second))
	e.SetTraceR("h1", 3.0, now)
	
	_, _, err := e.FusedR("h1")
	if err != nil {
		t.Error(err)
	}
}

func TestFusion_conflictDetected(t *testing.T) {
	e := NewEngine()
	now := time.Now()
	e.SetMetricR("h1", 1.0, now)
	e.SetLogR("h1", 4.0, now) // Diff > 2.0
	e.SetTraceR("h1", 3.0, now)
	
	_, _, err := e.FusedR("h1")
	if err != nil {
		t.Error(err)
	}
	// Conflict detected, logged but no error.
}

func TestFusion_insufficientSignals_zero(t *testing.T) {
	e := NewEngine()
	e.SetMetricR("h1", 1.0, time.Now())
	
	_, _, err := e.FusedR("h1")
	if err == nil {
		t.Error("expected error for insufficient signals, got nil")
	}
}

func TestFusion_twoSignals(t *testing.T) {
	e := NewEngine()
	now := time.Now()
	e.SetMetricR("h1", 1.0, now)
	e.SetLogR("h1", 2.0, now)
	
	val, _, err := e.FusedR("h1")
	if err != nil {
		t.Fatal(err)
	}
	
	// 0.75*1.0 + 0.25*2.0 = 0.75 + 0.5 = 1.25
	expected := 1.25
	if math.Abs(val-expected) > 1e-6 {
		t.Errorf("expected %f, got %f", expected, val)
	}
}

func TestFusion_concurrent(t *testing.T) {
	e := NewEngine()
	wg := sync.WaitGroup{}
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			e.SetMetricR("h1", float64(i), time.Now())
			e.SetLogR("h1", float64(i), time.Now())
			e.SetTraceR("h1", float64(i), time.Now())
			_, _, _ = e.FusedR("h1")
		}(i)
	}
	wg.Wait()
}

func TestFusion_unknownHost(t *testing.T) {
	e := NewEngine()
	_, _, err := e.FusedR("unknown")
	if err == nil {
		t.Error("expected error for unknown host, got nil")
	}
}
