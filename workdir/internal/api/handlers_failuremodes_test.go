package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/failuremodes"
	"github.com/benfradjselim/ruptura/internal/telemetry"
)

type fixedEstimates map[string]failuremodes.Estimate

func (f fixedEstimates) Worst(workload string, _ time.Time) (failuremodes.Estimate, bool) {
	est, ok := f[workload]
	return est, ok
}

func TestApplyEstimate(t *testing.T) {
	tests := []struct {
		name      string
		state     string
		etf       time.Duration
		wantState string
		wantRisk  bool
	}{
		{"distant failure keeps state", "healthy", 3 * time.Hour, "healthy", false},
		{"near failure degrades", "healthy", 20 * time.Minute, "degraded", true},
		{"imminent failure is critical", "degraded", 5 * time.Minute, "critical", true},
		{"calibrating workload still warns", "calibrating", 20 * time.Minute, "degraded", true},
		{"critical is never lowered", "critical", 20 * time.Minute, "critical", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state, risk := applyEstimate(tc.state, 0, failuremodes.Estimate{ETF: tc.etf})
			if state != tc.wantState {
				t.Errorf("state = %q, want %q", state, tc.wantState)
			}
			if (risk >= 1.5) != tc.wantRisk {
				t.Errorf("risk = %v, want alerting %v", risk, tc.wantRisk)
			}
		})
	}
}

func TestSummary_AnnouncesTimeToFailure(t *testing.T) {
	h := New(nil, nil, nil, nil, nil, nil, nil, nil, telemetry.NewRegistry("test"), telemetry.NewHealthChecker(), "key")
	h.SetFailureModes(fixedEstimates{
		"demo/Deployment/orders": {Mode: failuremodes.ModeSaturation, ETF: 18 * time.Minute, Confidence: 0.9, Usage: 0.7},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v2/workloads/demo/Deployment/orders/summary", nil)
	req.Header.Set("Authorization", "Bearer key")
	rec := httptest.NewRecorder()
	h.NewRouter().ServeHTTP(rec, req)
	var body WorkloadSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v, body %s", err, rec.Body.String())
	}
	want := "orders is projected to reach its memory limit in 18 minutes."
	if body.Headline != want {
		t.Fatalf("headline = %q, want %q", body.Headline, want)
	}
}
