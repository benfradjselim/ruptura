package analyzer

import (
	"math"
	"reflect"
	"testing"

	"github.com/benfradjselim/ruptura/pkg/models"
)

func TestWeightedMean(t *testing.T) {
	present := func(v float64) reading { return reading{value: v, available: true} }
	cases := []struct {
		name   string
		inputs []weightedInput
		want   reading
	}{
		{"no input", nil, reading{}},
		{"all absent", []weightedInput{{0.5, reading{}}}, reading{}},
		{"zero weight is ignored", []weightedInput{{0, present(1)}}, reading{}},
		{"absent input leaves the mean unchanged", []weightedInput{{0.3, present(1)}, {0.2, present(0)}, {0.5, reading{}}}, present(0.6)},
		{"measured zero counts", []weightedInput{{0.5, present(1)}, {0.5, present(0)}}, present(0.5)},
	}
	for _, c := range cases {
		got := weightedMean(c.inputs)
		if got.available != c.want.available || math.Abs(got.value-c.want.value) > 1e-9 {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestUpdate_MissingInputsAreExplicit(t *testing.T) {
	ref := models.WorkloadRef{Namespace: "n", Kind: "Deployment", Name: "w"}
	full := map[string]float64{
		"cpu_percent": 0, "memory_percent": 0, "latency": 0, "error_rate": 0,
		"timeout_rate": 0, "request_rate": 1, "uptime_seconds": 100,
	}
	cases := []struct {
		name    string
		metrics map[string]float64
		want    []string
	}{
		{"cpu and memory only", map[string]float64{"cpu_percent": 0.5, "memory_percent": 0.5},
			[]string{"contagion", "humidity", "mood", "pressure", "resilience", "throughput"}},
		{"measured zeros are available", full, nil},
		{"nothing measured", map[string]float64{"other": 1},
			[]string{"contagion", "entropy", "fatigue", "health_score", "humidity", "mood", "pressure", "resilience", "stress", "throughput", "velocity"}},
	}
	for _, c := range cases {
		snap := NewAnalyzer().Update(ref, c.metrics)
		if !reflect.DeepEqual(snap.UnavailableSignals, c.want) {
			t.Errorf("%s: unavailable = %v, want %v", c.name, snap.UnavailableSignals, c.want)
		}
	}
	snap := NewAnalyzer().Update(ref, map[string]float64{"cpu_percent": 0.5})
	if !snap.Mood.Unavailable || snap.Mood.State != stateUnavailable {
		t.Errorf("mood = %+v, want unavailable", snap.Mood)
	}
}

func TestUpdate_AbsentInputDoesNotChangeHealth(t *testing.T) {
	ref := models.WorkloadRef{Namespace: "n", Kind: "Deployment", Name: "w"}

	snap := NewAnalyzer().Update(ref, map[string]float64{"cpu_percent": 0, "memory_percent": 0})
	if snap.HealthScore.Value != 1 {
		t.Errorf("idle workload health = %v, want 1 when other inputs are absent", snap.HealthScore.Value)
	}

	snap = NewAnalyzer().Update(ref, map[string]float64{"cpu_percent": 0.5, "memory_percent": 0.5})
	if snap.Stress.Value != 0.5 {
		t.Errorf("stress = %v, want 0.5 from the measured inputs only", snap.Stress.Value)
	}
}

func TestUpdate_HealthUnavailableWithoutInputs(t *testing.T) {
	ref := models.WorkloadRef{Namespace: "n", Kind: "Deployment", Name: "w"}
	snap := NewAnalyzer().Update(ref, map[string]float64{"other": 1})
	if !snap.HealthScore.Unavailable {
		t.Errorf("health = %+v, want unavailable", snap.HealthScore)
	}
}

func TestBaseline_SignalAppearingLateUsesItsOwnCount(t *testing.T) {
	ws := &workloadState{}
	for i := 0; i < 10; i++ {
		ws.updateBaseline(map[string]float64{"stress": 0.5})
	}
	ws.updateBaseline(map[string]float64{"stress": 0.5, "humidity": 0.4})
	if got := ws.baselineMeans["humidity"]; got != 0.4 {
		t.Errorf("humidity mean = %v, want 0.4 after its first reading", got)
	}
}

func TestSignalVector_UnavailableIsNotTheWorstValue(t *testing.T) {
	cases := []struct {
		name string
		kpi  models.KPI
		want float64
	}{
		{"unavailable", models.KPI{Unavailable: true}, 0},
		{"measured worst", models.KPI{Value: 0}, 1},
		{"measured best", models.KPI{Value: 1}, 0},
	}
	for _, c := range cases {
		got := signalVector(models.KPISnapshot{Mood: c.kpi, Resilience: c.kpi}, 0)
		if got[2] != c.want || got[6] != c.want {
			t.Errorf("%s: mood %v, resilience %v, want %v", c.name, got[2], got[6], c.want)
		}
	}
}
