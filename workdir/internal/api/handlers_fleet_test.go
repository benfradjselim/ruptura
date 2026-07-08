package api

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"

	"github.com/benfradjselim/ruptura/internal/analyzer"
	"github.com/benfradjselim/ruptura/pkg/models"
)

// TestSnapshotState is FBL-V1's core regression test: a workload with a
// realistic, healthy HealthScore (the [0,1] scale it's actually stored on)
// must never be classified "critical" just because calibration finished.
// Before the fix, snapshotState compared hs against 70/40 as if it were a
// [0,100] value, so every real health score (typically 0.7-0.9) fell
// straight through to "critical" — the exact "37/37 critical, health
// 75-90, risk 0.0" contradiction observed on the lab cluster.
func TestSnapshotState(t *testing.T) {
	tests := []struct {
		name                string
		calibrationProgress int
		healthScore         float64
		want                string
	}{
		{"still calibrating, no health score yet", 40, 0, "calibrating"},
		{"still calibrating despite a high-looking score", 99, 0.9, "calibrating"},
		{"calibrated, healthy — the regression case: 0.8 must never be critical", 100, 0.8, "healthy"},
		{"calibrated, healthy at the boundary", 100, 0.70, "healthy"},
		{"calibrated, degraded", 100, 0.55, "degraded"},
		{"calibrated, degraded at the boundary", 100, 0.40, "degraded"},
		{"calibrated, genuinely critical", 100, 0.10, "critical"},
		{"calibrated, health score never computed (NaN) must not read as critical", 100, math.NaN(), "calibrating"},
		{"calibrated, exactly zero health is genuinely critical, not NaN", 100, 0, "critical"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap := models.KPISnapshot{
				CalibrationProgress: tt.calibrationProgress,
				HealthScore:         models.KPI{Value: tt.healthScore},
			}
			if got := snapshotState(snap); got != tt.want {
				t.Errorf("snapshotState(calibration=%d, health=%v) = %q, want %q",
					tt.calibrationProgress, tt.healthScore, got, tt.want)
			}
		})
	}
}

// TestHandleFleet_CalibratingWorkloadsNeverCountAsCritical is the AC's
// literal requirement in fleet-aggregate form: a fleet that's still warming
// up must never show those workloads as critical. Before the fix,
// handleFleet's count switch used "default: CriticalHosts++", so every
// "calibrating" state (a real, distinct case — see snapshotState) fell into
// that default and was tallied as critical alongside genuinely critical
// hosts, matching the "37/37 critical" contradiction observed on the lab
// cluster. Uses a real *analyzer.Analyzer (not a hand-built snapshot) so the
// test goes through handleFleet's actual enrichSnapshot->CalibrationInfo
// path exactly as production does — enrichSnapshot overwrites
// CalibrationProgress from the analyzer's real state, so a hand-set field
// on the stored snapshot wouldn't survive to snapshotState untouched.
func TestHandleFleet_CalibratingWorkloadsNeverCountAsCritical(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	a := analyzer.NewAnalyzer()

	// One Update() call is far short of the 96-observation baseline —
	// analyzer.CalibrationInfo reports "calibrating" for this ref.
	freshRef := models.WorkloadRef{Namespace: "prod", Kind: "Deployment", Name: "fresh-svc"}
	store.StoreSnapshot(a.Update(freshRef, map[string]float64{"cpu_percent": 0.2}))

	// 100 idle-metric cycles crosses the baseline (matches
	// TestAdaptiveBaseline_IdleWorkload in the analyzer package, which
	// asserts this settles the health score above 0.80 — i.e. "healthy").
	idleRef := models.WorkloadRef{Namespace: "prod", Kind: "Deployment", Name: "web-stable"}
	var lastSnap models.KPISnapshot
	for i := 0; i < 100; i++ {
		lastSnap = a.Update(idleRef, map[string]float64{})
	}
	store.StoreSnapshot(lastSnap)

	h := &Handlers{store: store, analyzer: a}
	r := mux.NewRouter()
	r.HandleFunc("/api/v2/fleet", h.handleFleet).Methods("GET")

	req := httptest.NewRequest(http.MethodGet, "/api/v2/fleet", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp fleetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.TotalHosts != 2 {
		t.Errorf("TotalHosts = %d, want 2", resp.TotalHosts)
	}
	if resp.HealthyHosts != 1 {
		t.Errorf("HealthyHosts = %d, want 1 (the idle, fully-calibrated workload)", resp.HealthyHosts)
	}
	if resp.CriticalHosts != 0 {
		t.Errorf("CriticalHosts = %d, want 0 (the still-calibrating workload must not be counted as critical)", resp.CriticalHosts)
	}
	if resp.DegradedHosts != 0 {
		t.Errorf("DegradedHosts = %d, want 0", resp.DegradedHosts)
	}

	var freshState, idleState string
	for _, host := range resp.Hosts {
		switch host.Host {
		case "prod/Deployment/fresh-svc":
			freshState = host.State
		case "prod/Deployment/web-stable":
			idleState = host.State
		}
	}
	if freshState != "calibrating" {
		t.Errorf("fresh-svc per-host state = %q, want calibrating", freshState)
	}
	if idleState != "healthy" {
		t.Errorf("web-stable per-host state = %q, want healthy", idleState)
	}
}
