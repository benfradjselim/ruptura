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

// fakeSource is a failure source with a fixed list of workloads.
type fakeSource struct {
	workloads []string
	worst     map[string]failuremodes.Estimate
	restarts  map[string]int
}

func (f fakeSource) Workloads(time.Time) []string { return f.workloads }
func (f fakeSource) Worst(w string, _ time.Time) (failuremodes.Estimate, bool) {
	est, ok := f.worst[w]
	return est, ok
}
func (f fakeSource) RecentRestarts(w string, _ time.Time, _ time.Duration) int { return f.restarts[w] }

func fleetOf(t *testing.T, h *Handlers) fleetResponse {
	t.Helper()
	rec := httptest.NewRecorder()
	h.handleFleet(rec, httptest.NewRequest(http.MethodGet, "/api/v2/fleet", nil))
	var out fleetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestFleet_StatesFromFailures(t *testing.T) {
	h := New(nil, nil, telemetry.NewRegistry("test"), telemetry.NewHealthChecker(), "key")
	h.SetFailureModes(fakeSource{
		workloads: []string{"shop/Deployment/api", "shop/Deployment/db", "shop/Deployment/web", "shop/Deployment/cache"},
		worst: map[string]failuremodes.Estimate{
			"shop/Deployment/api":   {Mode: failuremodes.ModeSaturation, ETF: 3 * time.Minute},
			"shop/Deployment/cache": {Mode: failuremodes.ModeCPUThrottling},
		},
		restarts: map[string]int{"shop/Deployment/db": 10},
	})
	got := fleetOf(t, h)
	states := map[string]string{}
	for _, w := range got.Hosts {
		states[w.Host] = w.State
	}
	want := map[string]string{"shop/Deployment/api": "critical", "shop/Deployment/db": "critical", "shop/Deployment/cache": "degraded", "shop/Deployment/web": "healthy"}
	for w, s := range want {
		if states[w] != s {
			t.Errorf("%s = %q, want %q", w, states[w], s)
		}
	}
	if got.TotalHosts != 4 || got.CriticalHosts != 2 || got.DegradedHosts != 1 || got.HealthyHosts != 1 {
		t.Fatalf("counts = %+v", got)
	}
	if got.Hosts[len(got.Hosts)-1].State != "healthy" {
		t.Fatal("critical and degraded workloads must sort before healthy ones")
	}
}

func TestFleet_EmptyWithoutSources(t *testing.T) {
	h := New(nil, nil, telemetry.NewRegistry("test"), telemetry.NewHealthChecker(), "key")
	if got := fleetOf(t, h); got.TotalHosts != 0 || len(got.Hosts) != 0 {
		t.Fatalf("fleet = %+v, want empty", got)
	}
}

func TestWorkloadRestartCount_NoDiscovery(t *testing.T) {
	h := New(nil, nil, telemetry.NewRegistry("test"), telemetry.NewHealthChecker(), "key")
	if got := h.workloadRestartCount("shop/Deployment/api"); got != 0 {
		t.Fatalf("restarts = %d, want 0 without an informer", got)
	}
}
