package metrics_test

import (
	"testing"
	"time"

	"github.com/benfradjselim/kairo-core/internal/pipeline/metrics"
	"github.com/benfradjselim/kairo-core/pkg/models"
)

// --- Processor ---

func TestProcessor_IngestAndGet(t *testing.T) {
	p := metrics.NewProcessor(100)
	p.Ingest([]models.Metric{
		{Host: "h1", Name: "cpu_percent", Value: 50.0},
		{Host: "h1", Name: "memory_percent", Value: 80.0},
	})
	v, ok := p.GetNormalized("h1", "cpu_percent")
	if !ok {
		t.Fatal("expected value for cpu_percent")
	}
	if v < 0 || v > 1 {
		t.Fatalf("normalized value out of [0,1]: %f", v)
	}
}

func TestProcessor_GetNormalized_missing(t *testing.T) {
	p := metrics.NewProcessor(100)
	_, ok := p.GetNormalized("ghost", "cpu")
	if ok {
		t.Fatal("expected false for unknown metric")
	}
}

func TestProcessor_GetHistory(t *testing.T) {
	p := metrics.NewProcessor(100)
	p.Ingest([]models.Metric{{Host: "h1", Name: "cpu_percent", Value: 30.0}})
	if h := p.GetHistory("h1", "cpu_percent"); len(h) == 0 {
		t.Fatal("expected non-empty history")
	}
}

func TestProcessor_GetHistory_missing(t *testing.T) {
	p := metrics.NewProcessor(100)
	if h := p.GetHistory("ghost", "cpu"); h != nil {
		t.Fatal("expected nil for unknown metric")
	}
}

func TestProcessor_Aggregate(t *testing.T) {
	p := metrics.NewProcessor(100)
	for i := 0; i < 10; i++ {
		p.Ingest([]models.Metric{{Host: "h1", Name: "cpu_percent", Value: float64(i * 10)}})
	}
	agg, ok := p.Aggregate("h1", "cpu_percent")
	if !ok {
		t.Fatal("expected aggregate result")
	}
	if agg.Min > agg.Max {
		t.Fatalf("min %f > max %f", agg.Min, agg.Max)
	}
}

func TestProcessor_Aggregate_missing(t *testing.T) {
	p := metrics.NewProcessor(100)
	_, ok := p.Aggregate("ghost", "cpu")
	if ok {
		t.Fatal("expected false for unknown metric")
	}
}

func TestDownsample(t *testing.T) {
	base := time.Now()
	pts := []models.DataPoint{
		{Timestamp: base, Value: 1.0},
		{Timestamp: base.Add(5 * time.Second), Value: 2.0},
		{Timestamp: base.Add(65 * time.Second), Value: 3.0},
	}
	out := metrics.Downsample(pts, time.Minute)
	if len(out) == 0 {
		t.Fatal("expected downsampled output")
	}
}

func TestDownsample_empty(t *testing.T) {
	if out := metrics.Downsample(nil, time.Minute); out != nil {
		t.Fatal("expected nil for empty input")
	}
}

// --- AnomalyEngine ---

func TestAnomalyEngine_Observe(t *testing.T) {
	ae := metrics.NewAnomalyEngine()
	for i := 0; i < 20; i++ {
		ae.Observe("h1", "cpu", float64(i), time.Now())
	}
	ae.Observe("h1", "cpu", 1000.0, time.Now())
}

func TestAnomalyStore_PushQuery(t *testing.T) {
	store := metrics.NewAnomalyStore(100)
	store.Push(models.AnomalyEvent{
		Host:      "h1",
		Metric:    "cpu",
		Score:     5.0,
		Timestamp: time.Now(),
	})
	events := store.Query("h1", "cpu", nil, time.Now().Add(-time.Minute))
	if len(events) == 0 {
		t.Fatal("expected at least one anomaly event")
	}
}

// --- Predictor (legacy CAILR wrapper) threshold ---

func TestPredictor_SetRuptureThreshold(t *testing.T) {
	p := metrics.NewPredictor()
	base := time.Now()
	for i := 0; i < 30; i++ {
		p.Feed("h1", "cpu", float64(i)*2, base.Add(time.Duration(i)*15*time.Second))
	}
	p.SetRuptureThreshold(1.0)
	_ = p.AcceleratingMetrics("h1")
}
