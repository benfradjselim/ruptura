package fusion

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/pkg/models"
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

func TestFusion_singleSignal_returnsMetricR(t *testing.T) {
	e := NewEngine()
	e.SetMetricR("h1", 2.5, time.Now())

	val, _, err := e.FusedR("h1")
	if err != nil {
		t.Fatalf("unexpected error with single metricR signal: %v", err)
	}
	if math.Abs(val-2.5) > 1e-6 {
		t.Errorf("expected FusedR=2.5 with single signal, got %.3f", val)
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

func TestFusion_StartLogWatcher_UpdatesLogR(t *testing.T) {
	e := NewEngine()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan models.BurstEvent, 1)
	e.StartLogWatcher(ctx, ch)

	now := time.Now()
	// Also set a metricR so we have 2 signals and can call FusedR
	e.SetMetricR("svc-a", 1.0, now)

	// Send a burst event: Count=30, BaselineRate=10 → logR = 30/10 - 1.0 = 2.0
	ch <- models.BurstEvent{
		Service:      "svc-a",
		StartTS:      now,
		EndTS:        now.Add(5 * time.Second),
		Count:        30,
		BaselineRate: 10.0,
	}

	// Wait for watcher goroutine to process
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		val, _, err := e.FusedR("svc-a")
		if err == nil && val > 0 {
			return // success
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("expected FusedR to return non-zero after StartLogWatcher received BurstEvent")
}

func TestFusion_FusedR_NonZero_WhenLogRSet(t *testing.T) {
	e := NewEngine()
	now := time.Now()
	e.SetLogR("svc-b", 1.5, now)
	e.SetTraceR("svc-b", 0.5, now)

	val, _, err := e.FusedR("svc-b")
	if err != nil {
		t.Fatal(err)
	}
	if val == 0 {
		t.Error("expected FusedR to return non-zero, got 0")
	}
}

func TestFusion_Snapshot(t *testing.T) {
	e := NewEngine()
	now := time.Now()
	e.SetMetricR("host1", 1.0, now)
	e.SetLogR("host1", 1.0, now)

	snap := e.Snapshot()
	if _, ok := snap["host1"]; !ok {
		t.Error("expected host1 in snapshot")
	}
	if snap["host1"] == 0 {
		t.Error("expected non-zero snapshot value for host1")
	}
}

func TestFusion_StateByWorkload_AllSignals(t *testing.T) {
	e := NewEngine()
	now := time.Now()
	e.SetMetricR("prod/Deployment/api", 1.2, now)
	e.SetLogR("prod/Deployment/api", 3.8, now)
	e.SetTraceR("prod/Deployment/api", 0.4, now)

	st, err := e.StateByWorkload("prod/Deployment/api")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.Workload != "prod/Deployment/api" {
		t.Errorf("wrong workload: %q", st.Workload)
	}
	if st.MetricR != 1.2 || st.LogR != 3.8 || st.TraceR != 0.4 {
		t.Errorf("wrong signal values: metric=%.2f log=%.2f trace=%.2f", st.MetricR, st.LogR, st.TraceR)
	}
	if st.FusedR == 0 {
		t.Error("expected non-zero FusedR")
	}
	if st.DominantPipeline != "logs" {
		t.Errorf("expected dominant=logs, got %q", st.DominantPipeline)
	}
	if st.LastUpdated.IsZero() {
		t.Error("expected non-zero LastUpdated")
	}
}

func TestFusion_StateByWorkload_Unknown(t *testing.T) {
	e := NewEngine()
	_, err := e.StateByWorkload("ns/Deployment/missing")
	if err == nil {
		t.Error("expected error for unknown workload")
	}
}

func TestFusion_StateByWorkload_SingleSignal(t *testing.T) {
	e := NewEngine()
	now := time.Now()
	e.SetMetricR("ns/Deployment/lonely", 2.0, now)

	st, err := e.StateByWorkload("ns/Deployment/lonely")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Single-signal path: FusedR == metricR directly.
	if math.Abs(st.FusedR-2.0) > 1e-6 {
		t.Errorf("expected FusedR=2.0 with single metricR signal, got %.3f", st.FusedR)
	}
}
