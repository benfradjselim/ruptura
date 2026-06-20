package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/alerter"
	"github.com/benfradjselim/ruptura/internal/explain"
	"github.com/benfradjselim/ruptura/internal/storage"
	"github.com/benfradjselim/ruptura/internal/telemetry"
	"github.com/benfradjselim/ruptura/pkg/models"
)

func newTestStore(t *testing.T) (*storage.Store, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "ruptura-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	s, err := storage.Open(dir)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("open store: %v", err)
	}
	return s, func() {
		s.Close()
		os.RemoveAll(dir)
	}
}

func TestHandleRupture_404_unknownHost(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	met := telemetry.NewRegistry("test")
	hc := telemetry.NewHealthChecker()
	al := alerter.NewAlerter(10)
	exp := explain.NewEngine()
	h := New(store, nil, exp, al, nil, nil, nil, nil, met, hc, "test-key")
	h.SetReady(true)
	router := h.NewRouter()

	req, _ := http.NewRequest("GET", "/api/v2/rupture/nonexistent-host", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown host, got %d", w.Code)
	}
}

func TestHandleRupture_200_knownHost(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	met := telemetry.NewRegistry("test")
	hc := telemetry.NewHealthChecker()
	al := alerter.NewAlerter(10)
	exp := explain.NewEngine()
	h := New(store, nil, exp, al, nil, nil, nil, nil, met, hc, "test-key")
	h.SetReady(true)
	router := h.NewRouter()

	// Insert a snapshot for a known host
	snap := models.KPISnapshot{
		Host:      "web-01",
		Timestamp: time.Now(),
		Stress:    models.KPI{Name: "stress", Value: 0.42, Host: "web-01"},
	}
	store.StoreSnapshot(snap)

	req, _ := http.NewRequest("GET", "/api/v2/rupture/web-01", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for known host, got %d", w.Code)
	}

	var got models.KPISnapshot
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Host != "web-01" {
		t.Errorf("expected host=web-01, got %q", got.Host)
	}
	if got.Stress.Value != 0.42 {
		t.Errorf("expected stress=0.42, got %f", got.Stress.Value)
	}
}

func TestHandleRuptures_200_allHosts(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	met := telemetry.NewRegistry("test")
	hc := telemetry.NewHealthChecker()
	al := alerter.NewAlerter(10)
	exp := explain.NewEngine()
	h := New(store, nil, exp, al, nil, nil, nil, nil, met, hc, "test-key")
	h.SetReady(true)
	router := h.NewRouter()

	// Insert snapshots for two hosts
	for _, host := range []string{"host-a", "host-b"} {
		store.StoreSnapshot(models.KPISnapshot{Host: host, Timestamp: time.Now()})
	}

	req, _ := http.NewRequest("GET", "/api/v2/ruptures", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var snaps []models.KPISnapshot
	if err := json.NewDecoder(w.Body).Decode(&snaps); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(snaps) != 2 {
		t.Errorf("expected 2 snapshots, got %d", len(snaps))
	}
}
