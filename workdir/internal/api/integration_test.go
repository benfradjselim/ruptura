package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/alerter"
	"github.com/benfradjselim/ruptura/internal/analyzer"
	"github.com/benfradjselim/ruptura/internal/explain"
	"github.com/benfradjselim/ruptura/internal/fusion"
	"github.com/benfradjselim/ruptura/internal/storage"
	"github.com/benfradjselim/ruptura/internal/telemetry"
	"github.com/benfradjselim/ruptura/pkg/models"
)

func TestIntegration_RuptureWorkload(t *testing.T) {
	dir, err := os.MkdirTemp("", "ruptura-integration-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	store, err := storage.Open(dir)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("open store: %v", err)
	}
	defer func() { store.Close(); os.RemoveAll(dir) }()

	ana := analyzer.NewAnalyzer()

	ref := models.WorkloadRef{Namespace: "default", Kind: "Deployment", Name: "test-workload"}
	rawMetrics := map[string]float64{
		"cpu_percent":    55.0,
		"memory_percent": 70.0,
		"error_rate":     0.02,
	}

	// Simulate the production ticker: set MetricR + LogR in fusion BEFORE storing snapshot.
	// FusedR requires >=2 signals; logR=0.5 simulates a modest burst event.
	now := time.Now()
	fusionEng := fusion.NewEngine()
	fusionEng.SetMetricR(ref.Key(), 2.5, now)
	fusionEng.SetLogR(ref.Key(), 0.5, now)

	snap := ana.Update(ref, rawMetrics)
	if r, _, err := fusionEng.FusedR(ref.Key()); err == nil {
		snap.FusedRuptureIndex = r
	}
	store.StoreSnapshot(snap)

	met := telemetry.NewRegistry("test")
	hc := telemetry.NewHealthChecker()
	al := alerter.NewAlerter(10)
	exp := explain.NewEngine()
	h := New(store, nil, exp, al, nil, nil, nil, nil, met, hc, "test-key")
	h.SetReady(true)
	router := h.NewRouter()

	t.Run("GET /api/v2/rupture/default/test-workload", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v2/rupture/default/test-workload", nil)
		req.Header.Set("Authorization", "Bearer test-key")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d — body: %s", w.Code, w.Body.String())
		}
		var got models.KPISnapshot
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.Workload.Name != "test-workload" {
			t.Errorf("expected workload=test-workload, got %q", got.Workload.Name)
		}
		if got.HealthScore.Value <= 0 {
			t.Errorf("expected positive HealthScore, got %f", got.HealthScore.Value)
		}
		if got.FusedRuptureIndex <= 0 {
			t.Errorf("expected positive FusedRuptureIndex, got %f", got.FusedRuptureIndex)
		}
	})

	t.Run("GET /api/v2/ruptures returns >=1", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v2/ruptures", nil)
		req.Header.Set("Authorization", "Bearer test-key")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var snaps []models.KPISnapshot
		if err := json.NewDecoder(w.Body).Decode(&snaps); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(snaps) < 1 {
			t.Errorf("expected >=1 snapshot, got %d", len(snaps))
		}
	})
}
