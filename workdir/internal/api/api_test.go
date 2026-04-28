package api

import (
    "net/http"
    "net/http/httptest"
    "testing"
    "github.com/benfradjselim/kairo-core/internal/telemetry"
)

func TestAPI(t *testing.T) {
    met := telemetry.NewRegistry("6.0.0")
    hc := telemetry.NewHealthChecker()
    h := New(nil, nil, nil, nil, nil, met, hc, "token")
    h.SetReady(true)
    router := h.NewRouter()

    t.Run("Health", func(t *testing.T) {
        req, _ := http.NewRequest("GET", "/api/v2/health", nil)
        req.Header.Set("Authorization", "Bearer token")
        w := httptest.NewRecorder()
        router.ServeHTTP(w, req)
        if w.Code != http.StatusOK {
            t.Errorf("expected 200, got %d", w.Code)
        }
    })
    
    t.Run("Metrics", func(t *testing.T) {
        req, _ := http.NewRequest("GET", "/api/v2/metrics", nil)
        req.Header.Set("Authorization", "Bearer token")
        w := httptest.NewRecorder()
        router.ServeHTTP(w, req)
        if w.Code != http.StatusOK {
            t.Errorf("expected 200, got %d", w.Code)
        }
    })

    t.Run("EmergencyStop", func(t *testing.T) {
        req, _ := http.NewRequest("POST", "/api/v2/actions/emergency-stop", nil)
        req.Header.Set("Authorization", "Bearer token")
        w := httptest.NewRecorder()
        router.ServeHTTP(w, req)
        if w.Code != http.StatusOK {
            t.Errorf("expected 200, got %d", w.Code)
        }
    })

    t.Run("Explain", func(t *testing.T) {
        req, _ := http.NewRequest("GET", "/api/v2/explain/1", nil)
        req.Header.Set("Authorization", "Bearer token")
        w := httptest.NewRecorder()
        router.ServeHTTP(w, req)
        if w.Code != http.StatusNotFound {
            t.Errorf("expected 404, got %d", w.Code)
        }
    })

    t.Run("Rupture", func(t *testing.T) {
        req, _ := http.NewRequest("GET", "/api/v2/ruptures", nil)
        req.Header.Set("Authorization", "Bearer token")
        w := httptest.NewRecorder()
        router.ServeHTTP(w, req)
        if w.Code != http.StatusOK {
            t.Errorf("expected 200, got %d", w.Code)
        }
    })
    t.Run("Actions", func(t *testing.T) {
        req, _ := http.NewRequest("GET", "/api/v2/actions", nil)
        req.Header.Set("Authorization", "Bearer token")
        w := httptest.NewRecorder()
        router.ServeHTTP(w, req)
        if w.Code != http.StatusOK {
            t.Errorf("expected 200, got %d", w.Code)
        }
    })
    t.Run("Ready", func(t *testing.T) {
        req, _ := http.NewRequest("GET", "/api/v2/ready", nil)
        req.Header.Set("Authorization", "Bearer token")
        w := httptest.NewRecorder()
        router.ServeHTTP(w, req)
        if w.Code != http.StatusOK {
            t.Errorf("expected 200, got %d", w.Code)
        }
    })
    t.Run("KPI", func(t *testing.T) {
        req, _ := http.NewRequest("GET", "/api/v2/kpi/stress/h1", nil)
        req.Header.Set("Authorization", "Bearer token")
        w := httptest.NewRecorder()
        router.ServeHTTP(w, req)
        if w.Code != http.StatusOK {
            t.Errorf("expected 200, got %d", w.Code)
        }
    })
    t.Run("Write", func(t *testing.T) {
        req, _ := http.NewRequest("POST", "/api/v2/write", nil)
        req.Header.Set("Authorization", "Bearer token")
        w := httptest.NewRecorder()
        router.ServeHTTP(w, req)
        if w.Code != http.StatusNoContent {
            t.Errorf("expected 204, got %d", w.Code)
        }
    })
}
