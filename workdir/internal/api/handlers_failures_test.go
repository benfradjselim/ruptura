package api

import (
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/analyzer"
	"github.com/benfradjselim/ruptura/internal/discovery"
	"github.com/benfradjselim/ruptura/internal/failuremodes"
	"github.com/benfradjselim/ruptura/pkg/models"
)

func TestFailureResult(t *testing.T) {
	saturation := failuremodes.Estimate{Mode: failuremodes.ModeSaturation, Pod: "n/p", ETF: 12 * time.Minute, Confidence: 0.8, Usage: 900}
	throttled := failuremodes.Estimate{Mode: failuremodes.ModeCPUThrottling, Pod: "n/p", Confidence: 1, Usage: 0.4}
	cases := []struct {
		name      string
		est       *failuremodes.Estimate
		crashLoop bool
		wantMode  string
		wantKind  string
		wantETA   int
	}{
		{"no finding", nil, false, "", "", 0},
		{"saturation is predicted", &saturation, false, "saturation", failureKindPredicted, 720},
		{"throttling is detected", &throttled, false, "cpu_throttling", failureKindDetected, 0},
		{"crash loop is detected", nil, true, failureModeCrashLoop, failureKindDetected, 0},
		{"crash loop outranks a forecast", &saturation, true, failureModeCrashLoop, failureKindDetected, 0},
	}
	for _, c := range cases {
		got := failureResult("n/Deployment/w", "critical", c.est, c.crashLoop)
		if c.wantMode == "" {
			if got != nil {
				t.Errorf("%s: got %+v, want none", c.name, got)
			}
			continue
		}
		if got == nil {
			t.Fatalf("%s: got none", c.name)
		}
		if got.FailureMode != c.wantMode || got.Kind != c.wantKind || got.ETASeconds != c.wantETA {
			t.Errorf("%s: got %+v", c.name, got)
		}
		if got.Level != "workload" || got.Target != "n/Deployment/w" || got.State != "critical" || got.RecommendedAction == "" {
			t.Errorf("%s: incomplete result %+v", c.name, got)
		}
	}
}

func TestApplyEstimate_UsesHorizon(t *testing.T) {
	est := failuremodes.Estimate{Mode: failuremodes.ModeSaturation, ETF: 40 * time.Minute}
	cases := []struct {
		name    string
		horizon time.Duration
		want    string
	}{
		{"inside a long horizon", time.Hour, "degraded"},
		{"outside the default horizon", 30 * time.Minute, "healthy"},
		{"unset uses the default", 0, "healthy"},
	}
	for _, c := range cases {
		h := &Handlers{}
		h.SetHorizon(c.horizon)
		if got, _ := applyEstimate("healthy", 0, est, h.warningHorizon()); got != c.want {
			t.Errorf("%s: state = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestScopeResult(t *testing.T) {
	base := FailureResult{Level: failureLevelWorkload, Target: "n/Deployment/w", Pod: "n/p"}
	cases := []struct {
		scope      string
		wantLevel  string
		wantTarget string
	}{
		{"", "workload", "n/Deployment/w"},
		{"workload", "workload", "n/Deployment/w"},
		{"pod", "pod", "n/p"},
		{"namespace", "namespace", "n"},
	}
	for _, c := range cases {
		got := scopeResult(base, c.scope)
		if got.Level != c.wantLevel || got.Target != c.wantTarget {
			t.Errorf("scope %q: got %+v", c.scope, got)
		}
	}
	noPod := FailureResult{Level: failureLevelWorkload, Target: "n/Deployment/w"}
	if got := scopeResult(noPod, "pod"); got.Level != "workload" {
		t.Errorf("a result without a pod must stay at workload level: %+v", got)
	}
}

func TestParseSelector(t *testing.T) {
	cases := []struct {
		raw  string
		want map[string]string
	}{
		{"", nil},
		{"team=shop", map[string]string{"team": "shop"}},
		{"team=shop,tier=db", map[string]string{"team": "shop", "tier": "db"}},
		{"broken", map[string]string{}},
	}
	for _, c := range cases {
		got := ParseSelector(c.raw)
		if len(got) != len(c.want) {
			t.Errorf("%q: got %v", c.raw, got)
		}
		for k, v := range c.want {
			if got[k] != v {
				t.Errorf("%q: got %v", c.raw, got)
			}
		}
	}
}

func TestSelectFailures(t *testing.T) {
	results := []*FailureResult{{Target: "n/Deployment/a"}, {Target: "n/Deployment/b"}, {Target: "n/Deployment/unknown"}}
	labels := map[string]map[string]string{
		"n/Deployment/a": {"team": "shop", "tier": "web"},
		"n/Deployment/b": {"team": "pay"},
	}
	lookup := func(target string) (map[string]string, bool) {
		l, ok := labels[target]
		return l, ok
	}
	cases := []struct {
		name     string
		selector map[string]string
		want     int
	}{
		{"no selector keeps all", nil, 3},
		{"match", map[string]string{"team": "shop"}, 1},
		{"all labels must match", map[string]string{"team": "shop", "tier": "db"}, 0},
	}
	for _, c := range cases {
		if got := selectFailures(results, c.selector, lookup); len(got) != c.want {
			t.Errorf("%s: kept %d, want %d", c.name, len(got), c.want)
		}
	}
}

func TestFailures_WorkloadWithoutTelemetry(t *testing.T) {
	a := analyzer.NewAnalyzer()
	ref := models.WorkloadRef{Namespace: "n", Kind: "Deployment", Name: "w"}
	a.RegisterWorkload(ref)
	store, cleanup := newTestStore(t)
	defer cleanup()
	h := &Handlers{store: store, analyzer: a}
	h.SetFailureModes(fixedEstimates{ref.Key(): {Mode: failuremodes.ModeSaturation, Pod: "n/p", ETF: 5 * time.Minute, Confidence: 0.9}})

	results := h.Failures()
	if len(results) != 1 {
		t.Fatalf("results = %d, want the failure of a workload with no telemetry", len(results))
	}
	if results[0].FailureMode != "saturation" || results[0].State != "critical" {
		t.Errorf("result = %+v", results[0])
	}
	if fleet := h.buildFleet(); fleet.CriticalHosts != 1 {
		t.Errorf("critical hosts = %d, want 1", fleet.CriticalHosts)
	}
}

func TestFailureEvidence(t *testing.T) {
	cases := []struct {
		name string
		est  failuremodes.Estimate
		want string
	}{
		{"saturation", failuremodes.Estimate{Mode: failuremodes.ModeSaturation, Usage: 0.91}, "Memory at 91% of its limit"},
		{"throttling", failuremodes.Estimate{Mode: failuremodes.ModeCPUThrottling, Usage: 0.4}, "CPU throttled 40% of the time"},
		{"startup", failuremodes.Estimate{Mode: failuremodes.ModeStartupStuck, Reason: "FailedMount"}, "Kubernetes reports FailedMount"},
		{"replicas", failuremodes.Estimate{Mode: failuremodes.ModeReplicasUnavailable, Usage: 2}, "2 replicas unavailable for over 5 minutes"},
		{"one replica", failuremodes.Estimate{Mode: failuremodes.ModeReplicasUnavailable, Usage: 1}, "1 replica unavailable for over 5 minutes"},
		{"rhythm", failuremodes.Estimate{Mode: failuremodes.ModeRecurrentFailure, Interval: 31 * time.Minute, Failures: 7}, "7 failures, about every 31 min"},
		{"rollout", failuremodes.Estimate{Mode: failuremodes.ModeRolloutStuck}, "Rollout passed its progress deadline"},
		{"oom", failuremodes.Estimate{Mode: failuremodes.ModeOOMKilled}, "Last exit was OOMKilled"},
	}
	for _, c := range cases {
		got := failureEvidence(c.est)
		if len(got) != 1 || got[0] != c.want {
			t.Errorf("%s: evidence = %v, want %q", c.name, got, c.want)
		}
	}
}

func TestEstimateAction_RecurrentFailure(t *testing.T) {
	est := failuremodes.Estimate{Mode: failuremodes.ModeRecurrentFailure, Failures: 7, Interval: 31 * time.Minute}
	if got := estimateAction(est); got != "Find what restarts this workload before the next failure" {
		t.Errorf("action = %q", got)
	}
	if got := estimateHeadline("w", est); got != "w fails about every 31 minutes." {
		t.Errorf("headline = %q", got)
	}
}

func TestTimelineEvents(t *testing.T) {
	now := time.Date(2026, 11, 2, 1, 0, 0, 0, time.UTC)
	failures := map[string][]time.Time{"n/Deployment/a": {now.Add(-5 * time.Minute)}}
	changes := []discovery.Change{{Workload: "n/Deployment/a", Image: "app:2", At: now.Add(-20 * time.Minute)}}
	got := timelineEvents(failures, changes, now)
	if len(got) != 2 {
		t.Fatalf("events = %+v", got)
	}
	if got[0].Type != "change" || got[0].MinutesAgo != 20 || got[0].Detail != "app:2" {
		t.Errorf("first = %+v", got[0])
	}
	if got[1].Type != "failure" || got[1].MinutesAgo != 5 {
		t.Errorf("second = %+v", got[1])
	}
}

func TestChangeEvidence(t *testing.T) {
	now := time.Date(2026, 11, 2, 1, 0, 0, 0, time.UTC)
	changes := []discovery.Change{
		{Workload: "w", Image: "app:1", At: now.Add(-3 * time.Hour)},
		{Workload: "w", Image: "app:2", At: now.Add(-12 * time.Minute)},
		{Workload: "x", Image: "z", At: now.Add(-time.Minute)},
	}
	if got := changeEvidence("w", changes, now); got != "Changed 12 min ago: app:2" {
		t.Errorf("evidence = %q", got)
	}
	if got := changeEvidence("none", changes, now); got != "" {
		t.Errorf("evidence = %q", got)
	}
}

func TestReportedEstimate_DropsDistantSaturation(t *testing.T) {
	cases := []struct {
		name    string
		est     failuremodes.Estimate
		horizon time.Duration
		want    bool
	}{
		{"saturation inside the risk horizon", failuremodes.Estimate{Mode: failuremodes.ModeSaturation, ETF: 40 * time.Minute}, 30 * time.Minute, true},
		{"saturation ten days away", failuremodes.Estimate{Mode: failuremodes.ModeSaturation, ETF: 240 * time.Hour}, 30 * time.Minute, false},
		{"saturation inside a long configured horizon", failuremodes.Estimate{Mode: failuremodes.ModeSaturation, ETF: 90 * time.Minute}, 2 * time.Hour, true},
		{"a detected condition has no time to failure", failuremodes.Estimate{Mode: failuremodes.ModeCPUThrottling}, 30 * time.Minute, true},
	}
	for _, c := range cases {
		if got := reportedEstimate(c.est, c.horizon) != nil; got != c.want {
			t.Errorf("%s: reported = %v, want %v", c.name, got, c.want)
		}
	}
}
