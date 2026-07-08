package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/telemetry"
)

func TestHandleInfraGroupHistory_ReturnsPersistedPoints(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	now := time.Now()
	if err := store.PutGroupHealth("grp.network", "prod", now.Add(-time.Hour), 0.8); err != nil {
		t.Fatalf("PutGroupHealth: %v", err)
	}
	if err := store.PutGroupHealth("grp.network", "prod", now, 0.9); err != nil {
		t.Fatalf("PutGroupHealth: %v", err)
	}

	met := telemetry.NewRegistry("test")
	hc := telemetry.NewHealthChecker()
	h := New(store, nil, nil, nil, nil, nil, nil, nil, met, hc, "test-key")
	router := h.NewRouter()

	req, _ := http.NewRequest("GET", "/api/v2/infra/groups/grp.network/history?ns=prod", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Group     string `json:"group"`
		Namespace string `json:"namespace"`
		Points    []struct {
			Health float64 `json:"health"`
		} `json:"points"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Group != "grp.network" || resp.Namespace != "prod" {
		t.Errorf("group/namespace = %q/%q, want grp.network/prod", resp.Group, resp.Namespace)
	}
	if len(resp.Points) != 2 {
		t.Fatalf("expected 2 points, got %d", len(resp.Points))
	}
}

func TestHandleInfraGroupHistory_EmptyGroup_ReturnsEmptyTimeline(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	met := telemetry.NewRegistry("test")
	hc := telemetry.NewHealthChecker()
	h := New(store, nil, nil, nil, nil, nil, nil, nil, met, hc, "test-key")
	router := h.NewRouter()

	req, _ := http.NewRequest("GET", "/api/v2/infra/groups/grp.storage/history?ns=staging", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	points, ok := resp["points"].([]interface{})
	if !ok || len(points) != 0 {
		t.Errorf("expected an empty points array, got %v", resp["points"])
	}
}

func TestHandleInfraGroupHistory_RespectsFromToWindow(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	now := time.Now()
	old := now.Add(-48 * time.Hour)
	if err := store.PutGroupHealth("grp.tenancy", "", old, 0.5); err != nil {
		t.Fatalf("PutGroupHealth: %v", err)
	}
	if err := store.PutGroupHealth("grp.tenancy", "", now, 0.95); err != nil {
		t.Fatalf("PutGroupHealth: %v", err)
	}

	met := telemetry.NewRegistry("test")
	hc := telemetry.NewHealthChecker()
	h := New(store, nil, nil, nil, nil, nil, nil, nil, met, hc, "test-key")
	router := h.NewRouter()

	// Default window is last 24h — the 48h-old point must be excluded.
	req, _ := http.NewRequest("GET", "/api/v2/infra/groups/grp.tenancy/history", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var resp struct {
		Points []struct {
			Health float64 `json:"health"`
		} `json:"points"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Points) != 1 || resp.Points[0].Health != 0.95 {
		t.Errorf("expected exactly the recent point (0.95) within the default 24h window, got %+v", resp.Points)
	}
}

func TestHandleEngineStorage_IncludesRetentionStats(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	now := time.Now()
	if err := store.PutInfraSignal("grp.network", "namespace", "prod", "Route", "console", "nodeStress", now, 0.3); err != nil {
		t.Fatalf("PutInfraSignal: %v", err)
	}
	if err := store.PutPropagationSnapshot(now, []byte(`{"namespace":"prod"}`)); err != nil {
		t.Fatalf("PutPropagationSnapshot: %v", err)
	}

	met := telemetry.NewRegistry("test")
	hc := telemetry.NewHealthChecker()
	h := New(store, nil, nil, nil, nil, nil, nil, nil, met, hc, "test-key")
	router := h.NewRouter()

	req, _ := http.NewRequest("GET", "/api/v2/engine/storage", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Retention map[string]int64 `json:"retention"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// FBL-A3-1 AC: non-zero counts for is: and prop: specifically.
	if resp.Retention["is:"] == 0 {
		t.Error("expected non-zero is: count in /api/v2/engine/storage retention stats")
	}
	if resp.Retention["prop:"] == 0 {
		t.Error("expected non-zero prop: count in /api/v2/engine/storage retention stats")
	}
}
