package replay

import (
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/analyzer"
	"github.com/benfradjselim/ruptura/internal/failuremodes"
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

// saturationDetector warns when the failure-mode estimator predicts saturation
// within the warning horizon.
type saturationDetector struct {
	byPod map[string]*failuremodes.Saturation
}

const warningETF = 25 * time.Minute

func (d *saturationDetector) Observe(s Sample) bool {
	key := s.Workload + "/" + s.Pod
	est, ok := d.byPod[key]
	if !ok {
		est = failuremodes.NewSaturation()
		d.byPod[key] = est
	}
	e, has := est.Observe(s.At, s.MemoryBytes, s.LimitBytes, s.Restarts)
	return has && e.ETF <= warningETF
}

func TestReplay_SaturationDetector(t *testing.T) {
	samples, err := Load(recording)
	if err != nil {
		t.Fatal(err)
	}
	detector := &saturationDetector{byPod: map[string]*failuremodes.Saturation{}}
	result := Score(samples, detector, defaultOptions)
	t.Logf("saturation: %+v", result)
	if result.Predicted != 1 || result.Eligible != 1 {
		t.Fatalf("predicted = %d of eligible %d, want 1 of 1", result.Predicted, result.Eligible)
	}
	if result.FalseAlarms != 0 {
		t.Fatalf("false alarms = %d, want 0", result.FalseAlarms)
	}
	if len(result.LeadMinutes) == 0 || result.LeadMinutes[0] < 10 {
		t.Fatalf("lead = %v, want at least 10 minutes", result.LeadMinutes)
	}
}
