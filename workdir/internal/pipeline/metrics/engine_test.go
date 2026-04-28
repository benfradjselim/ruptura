package metrics_test

import (
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/pipeline/metrics"
)

func seedEngine(t *testing.T) *metrics.Engine {
	t.Helper()
	e := metrics.NewEngine()
	base := time.Now()
	for i := 0; i < 30; i++ {
		v := float64(i) * 1.5
		e.Ingest("host1", "cpu", v, base.Add(time.Duration(i)*15*time.Second))
	}
	return e
}

func TestEngine_RuptureIndex(t *testing.T) {
	e := seedEngine(t)
	r, err := e.RuptureIndex("host1", "cpu")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r < 0 {
		t.Fatalf("rupture index must be non-negative, got %f", r)
	}
}

func TestEngine_RuptureIndex_noData(t *testing.T) {
	e := metrics.NewEngine()
	_, err := e.RuptureIndex("ghost", "cpu")
	if err == nil {
		t.Fatal("expected error for unknown host:metric")
	}
}

func TestEngine_TTF(t *testing.T) {
	e := seedEngine(t)
	d, err := e.TTF("host1", "cpu")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d < 0 {
		t.Fatalf("TTF must be non-negative, got %v", d)
	}
}

func TestEngine_TTF_noData(t *testing.T) {
	e := metrics.NewEngine()
	_, err := e.TTF("ghost", "cpu")
	if err == nil {
		t.Fatal("expected error for unknown host:metric")
	}
}

func TestEngine_Confidence(t *testing.T) {
	e := seedEngine(t)
	_, err := e.Confidence("host1", "cpu")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEngine_Confidence_noData(t *testing.T) {
	e := metrics.NewEngine()
	_, err := e.Confidence("ghost", "cpu")
	if err == nil {
		t.Fatal("expected error for unknown host:metric")
	}
}

func TestEngine_SurgeProfile(t *testing.T) {
	e := seedEngine(t)
	profile, err := e.SurgeProfile("host1", "cpu")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	valid := map[string]bool{"Flat": true, "Ramping": true, "Spiking": true, "Recovering": true, "Cascading": true}
	if !valid[profile] {
		t.Fatalf("unexpected surge profile: %q", profile)
	}
}

func TestEngine_SurgeProfile_noData(t *testing.T) {
	e := metrics.NewEngine()
	_, err := e.SurgeProfile("ghost", "cpu")
	if err == nil {
		t.Fatal("expected error for unknown host:metric")
	}
}

func TestSurgeProfile_allBranches(t *testing.T) {
	cases := []struct {
		burst, stable, ri float64
		want              string
	}{
		{0.5, 2.0, 6.0, "Cascading"},
		{-0.5, 0.5, 1.0, "Recovering"},
		{0.5, 2.0, 1.5, "Spiking"},
		{0.5, 0.3, 0.5, "Ramping"},
		{0.0, 0.0, 0.0, "Flat"},
	}
	for _, tc := range cases {
		got := metrics.SurgeProfile(tc.burst, tc.stable, tc.ri)
		if got != tc.want {
			t.Errorf("SurgeProfile(%f,%f,%f): want %q got %q", tc.burst, tc.stable, tc.ri, tc.want, got)
		}
	}
}
