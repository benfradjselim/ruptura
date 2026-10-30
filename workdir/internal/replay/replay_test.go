package replay

import (
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/analyzer"
	"github.com/benfradjselim/ruptura/pkg/models"
)

const recording = "testdata/oom-2026-10-30.csv"

var defaultOptions = Options{
	Target:    "leak-orders",
	Controls:  []string{"stable-web", "cpu-batch"},
	MinLead:   10 * time.Minute,
	Warmup:    0,
	Eligibles: 20 * time.Minute,
}

// analyzerDetector mirrors the current production decision: unhealthy score.
type analyzerDetector struct {
	engine *analyzer.Analyzer
}

func (d *analyzerDetector) Observe(s Sample) bool {
	ref := models.WorkloadRef{Namespace: "demo-shop", Kind: "Deployment", Name: s.Workload}
	metrics := map[string]float64{"cpu_percent": 0}
	if s.LimitBytes > 0 {
		metrics["memory_percent"] = s.MemoryBytes / s.LimitBytes
	}
	snapshot := d.engine.Update(ref, metrics)
	return snapshot.CalibrationProgress >= 100 && snapshot.HealthScore.Value < 0.70
}

func TestReplay_RecordingHasAnEligibleFailure(t *testing.T) {
	samples, err := Load(recording)
	if err != nil {
		t.Fatal(err)
	}
	result := Score(samples, detectorFunc(func(Sample) bool { return false }), defaultOptions)
	if result.Failures != 2 || result.Eligible != 1 {
		t.Fatalf("failures = %d, eligible = %d, want 2 and 1", result.Failures, result.Eligible)
	}
}

func TestReplay_CurrentAnalyzerBaseline(t *testing.T) {
	samples, err := Load(recording)
	if err != nil {
		t.Fatal(err)
	}
	result := Score(samples, &analyzerDetector{engine: analyzer.NewAnalyzer()}, defaultOptions)
	t.Logf("baseline: %+v", result)
	if result.Predicted != 0 {
		t.Fatalf("baseline changed: predicted = %d, update the recorded score", result.Predicted)
	}
}

func TestScore_CountsLeadTime(t *testing.T) {
	start := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	var samples []Sample
	for minute := 0; minute <= 30; minute++ {
		restarts := 0
		if minute == 30 {
			restarts = 1
		}
		samples = append(samples, Sample{
			At:       start.Add(time.Duration(minute) * time.Minute),
			Workload: "leak-orders",
			Restarts: restarts,
		})
	}
	warnFrom := start.Add(12 * time.Minute)
	result := Score(samples, detectorFunc(func(s Sample) bool { return !s.At.Before(warnFrom) }), defaultOptions)
	if result.Predicted != 1 || result.LeadMinutes[0] != 18 {
		t.Fatalf("result = %+v, want one prediction with 18 minute lead", result)
	}
}

type detectorFunc func(Sample) bool

func (f detectorFunc) Observe(s Sample) bool { return f(s) }
