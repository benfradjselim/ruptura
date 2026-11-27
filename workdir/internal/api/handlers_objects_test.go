package api

import (
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/failuremodes"
)

type fakeObjects []failuremodes.ObjectFinding

func (f fakeObjects) Findings(time.Time) []failuremodes.ObjectFinding { return f }

func TestObjectResult(t *testing.T) {
	tests := []struct {
		name      string
		finding   failuremodes.ObjectFinding
		info      CardInfo
		wantKind  string
		wantState string
		wantETA   int
		wantWork  string
	}{
		{"node forecast far", failuremodes.ObjectFinding{Level: "node", Target: "n1//dev/sda4", Predicted: true, Unit: "bytes", Value: 1 << 30, Capacity: 2 << 30,
			Estimate: failuremodes.Estimate{Mode: "node_filesystem_filling", ETF: time.Hour}}, CardInfo{Severity: "critical"}, "predicted", "degraded", 3600, ""},
		{"node forecast close", failuremodes.ObjectFinding{Level: "node", Target: "n1//dev/sda4", Predicted: true, Unit: "bytes", Value: 1, Capacity: 2,
			Estimate: failuremodes.Estimate{Mode: "node_filesystem_filling", ETF: 5 * time.Minute}}, CardInfo{}, "predicted", "critical", 300, ""},
		{"detected warning", failuremodes.ObjectFinding{Level: "workload", Target: "shop/orders",
			Estimate: failuremodes.Estimate{Mode: "pdb_blocks_drain"}}, CardInfo{Severity: "warning", Action: "Add replicas"}, "detected", "degraded", 0, "shop/orders"},
		{"detected critical", failuremodes.ObjectFinding{Level: "node", Target: "n1/Ready",
			Estimate: failuremodes.Estimate{Mode: "node_not_ready"}}, CardInfo{Severity: "critical"}, "detected", "critical", 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := objectResult(tt.finding, tt.info)
			if got.Kind != tt.wantKind || got.State != tt.wantState || got.ETASeconds != tt.wantETA || got.Workload != tt.wantWork {
				t.Fatalf("result = %+v", got)
			}
			if got.Level != tt.finding.Level || got.Target != tt.finding.Target || got.FailureMode != string(tt.finding.Estimate.Mode) || len(got.Evidence) != 1 {
				t.Fatalf("result = %+v", got)
			}
			if got.RecommendedAction != tt.info.Action {
				t.Fatalf("action = %q", got.RecommendedAction)
			}
		})
	}
}

func TestObjectEvidence(t *testing.T) {
	f := failuremodes.ObjectFinding{Predicted: true, Unit: "count", Value: 45, Capacity: 60}
	if got, want := objectEvidence(f, CardInfo{}), "45 used of 60 (75%) and growing"; got != want {
		t.Fatalf("evidence = %q, want %q", got, want)
	}
	f = failuremodes.ObjectFinding{Predicted: true, Unit: "bytes", Value: 256 << 20, Capacity: 512 << 20}
	if got, want := objectEvidence(f, CardInfo{}), "256 MiB used of 512 MiB (50%) and growing"; got != want {
		t.Fatalf("evidence = %q, want %q", got, want)
	}
	detected := failuremodes.ObjectFinding{Value: 1}
	if got := objectEvidence(detected, CardInfo{}); got != "Reported by kube-state-metrics at the last reading" {
		t.Fatalf("default detected evidence = %q", got)
	}
	if got, want := objectEvidence(detected, CardInfo{Evidence: "Clock offset over 50 ms"}), "Clock offset over 50 ms"; got != want {
		t.Fatalf("card evidence = %q, want %q", got, want)
	}
}

type labelSource map[string]map[string]string

func (l labelSource) Of(ns string) (map[string]string, bool) { v, ok := l[ns]; return v, ok }

func TestObjectFailuresScope(t *testing.T) {
	findings := fakeObjects{
		{Level: "node", Target: "n1/Ready", Estimate: failuremodes.Estimate{Mode: "node_not_ready"}},
		{Level: "namespace", Target: "shop/q/pods", Predicted: true, Estimate: failuremodes.Estimate{Mode: "quota_exhaustion", ETF: time.Hour}},
		{Level: "workload", Target: "other/pdb", Estimate: failuremodes.Estimate{Mode: "pdb_blocks_drain"}},
	}
	h := &Handlers{}
	if got := h.objectFailures(time.Now()); got != nil {
		t.Fatalf("without findings source = %v", got)
	}
	h.SetObjectFindings(findings, nil)
	if got := len(h.objectFailures(time.Now())); got != 3 {
		t.Fatalf("cluster-wide instance sees %d, want 3", got)
	}
	h.SetNamespaceSelector(map[string]string{"team": "shop"}, labelSource{"shop": {"team": "shop"}, "other": {"team": "x"}})
	got := h.objectFailures(time.Now())
	if len(got) != 1 || got[0].Target != "shop/q/pods" {
		t.Fatalf("namespace-limited instance sees %+v, want only the shop quota", got)
	}
}
