package api

import (
	"strings"
	"testing"
)

func TestFailureMetrics(t *testing.T) {
	got := failureMetrics([]*FailureResult{
		{Workload: "boutique/Deployment/catalog", Pod: "boutique/catalog-1", FailureMode: "saturation", Kind: "predicted", State: "degraded", ETASeconds: 840, ETAEarlySeconds: 660, ETALateSeconds: 1140, RecommendedAction: `Raise the "memory" limit`},
		{Workload: "shop/Deployment/pay", FailureMode: "crash_loop", Kind: "detected", State: "critical"},
		{Workload: "bad"},
	})
	for _, want := range []string{
		"# TYPE ruptura_failure_eta_seconds gauge",
		`ruptura_failure_eta_seconds{namespace="boutique",workload_kind="Deployment",workload="catalog",pod="catalog-1",failure_mode="saturation",failure_kind="predicted",severity="warning",action="Raise the \"memory\" limit"} 840`,
		`ruptura_failure_eta_early_seconds{namespace="boutique",workload_kind="Deployment",workload="catalog",pod="catalog-1",failure_mode="saturation",failure_kind="predicted",severity="warning",action="Raise the \"memory\" limit"} 660`,
		`ruptura_failure_eta_late_seconds{namespace="boutique"`,
		`ruptura_failure_eta_seconds{namespace="shop",workload_kind="Deployment",workload="pay",pod="",failure_mode="crash_loop",failure_kind="detected",severity="critical",action=""} 0`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	if strings.Contains(got, `ruptura_failure_eta_early_seconds{namespace="shop"`) {
		t.Error("a failure without a range must not export early or late series")
	}
	if strings.Contains(got, `workload="bad"`) {
		t.Error("a malformed workload key must be skipped")
	}
}
