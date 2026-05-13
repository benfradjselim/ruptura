package api

import (
    "bytes"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/benfradjselim/ruptura/internal/analyzer"
    "github.com/benfradjselim/ruptura/internal/fusion"
    "github.com/benfradjselim/ruptura/internal/telemetry"
)

func TestAPI(t *testing.T) {
    met := telemetry.NewRegistry("6.0.0")
    hc := telemetry.NewHealthChecker()
    h := New(nil, nil, nil, nil, nil, nil, nil, nil, met, hc, "token")
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
    t.Run("ApproveBlockedInCommunityEdition", func(t *testing.T) {
        // edition defaults to "" which is treated as community — approve must return 402.
        req, _ := http.NewRequest("POST", "/api/v2/actions/abc123/approve", nil)
        req.Header.Set("Authorization", "Bearer token")
        w := httptest.NewRecorder()
        router.ServeHTTP(w, req)
        if w.Code != http.StatusPaymentRequired {
            t.Errorf("expected 402 in community edition, got %d", w.Code)
        }
    })
    t.Run("ApproveAllowedInAutopilotEdition", func(t *testing.T) {
        hAutopilot := New(nil, nil, nil, nil, nil, nil, nil, nil, met, hc, "token")
        hAutopilot.SetReady(true)
        hAutopilot.SetEdition("autopilot")
        rAutopilot := hAutopilot.NewRouter()
        req, _ := http.NewRequest("POST", "/api/v2/actions/abc123/approve", nil)
        req.Header.Set("Authorization", "Bearer token")
        w := httptest.NewRecorder()
        rAutopilot.ServeHTTP(w, req)
        // engine is nil so Approve() won't fire — expect 404 (action not found), not 402.
        if w.Code == http.StatusPaymentRequired {
            t.Errorf("autopilot edition should not return 402, got %d", w.Code)
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

func TestFusionStateEndpoint(t *testing.T) {
	met := telemetry.NewRegistry("test")
	hc := telemetry.NewHealthChecker()
	h := New(nil, nil, nil, nil, nil, nil, nil, nil, met, hc, "")
	fe := fusion.NewEngine()
	h.SetFusion(fe)
	h.SetReady(true)
	router := h.NewRouter()

	t.Run("unknown workload returns 404", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v2/engine/fusion/ns/Deployment/missing", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})

	t.Run("known workload returns 200 with state", func(t *testing.T) {
		now := time.Now()
		fe.SetMetricR("prod/Deployment/api", 1.2, now)
		fe.SetLogR("prod/Deployment/api", 3.8, now)

		req, _ := http.NewRequest("GET", "/api/v2/engine/fusion/prod/Deployment/api", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		body := w.Body.String()
		if !contains(body, "prod/Deployment/api") {
			t.Errorf("response missing workload key: %s", body)
		}
		if !contains(body, "fused_r") {
			t.Errorf("response missing fused_r: %s", body)
		}
	})

	t.Run("no fusion engine returns 503", func(t *testing.T) {
		h2 := New(nil, nil, nil, nil, nil, nil, nil, nil, met, hc, "")
		h2.SetReady(true)
		r2 := h2.NewRouter()
		req, _ := http.NewRequest("GET", "/api/v2/engine/fusion/ns/Deployment/foo", nil)
		w := httptest.NewRecorder()
		r2.ServeHTTP(w, req)
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503, got %d", w.Code)
		}
	})
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}

func TestConfigWeights(t *testing.T) {
    met := telemetry.NewRegistry("test")
    hc := telemetry.NewHealthChecker()
    h := New(nil, nil, nil, nil, nil, nil, nil, nil, met, hc, "")
    h.SetAnalyzer(analyzer.NewAnalyzer())
    h.SetReady(true)
    router := h.NewRouter()

    t.Run("GET returns empty list initially", func(t *testing.T) {
        req, _ := http.NewRequest("GET", "/api/v2/config/weights", nil)
        w := httptest.NewRecorder()
        router.ServeHTTP(w, req)
        if w.Code != http.StatusOK {
            t.Errorf("expected 200, got %d", w.Code)
        }
    })

    t.Run("POST sets weight configs", func(t *testing.T) {
        body := bytes.NewBufferString(`[{"selector":"payments/*","stress":0.5,"fatigue":0.1,"mood":0.1,"pressure":0.1,"humidity":0.1,"contagion":0.1}]`)
        req, _ := http.NewRequest("POST", "/api/v2/config/weights", body)
        req.Header.Set("Content-Type", "application/json")
        w := httptest.NewRecorder()
        router.ServeHTTP(w, req)
        if w.Code != http.StatusOK {
            t.Errorf("expected 200, got %d", w.Code)
        }
    })

    t.Run("GET returns set configs", func(t *testing.T) {
        req, _ := http.NewRequest("GET", "/api/v2/config/weights", nil)
        w := httptest.NewRecorder()
        router.ServeHTTP(w, req)
        if w.Code != http.StatusOK {
            t.Errorf("expected 200, got %d", w.Code)
        }
    })

    t.Run("POST with bad JSON returns 400", func(t *testing.T) {
        req, _ := http.NewRequest("POST", "/api/v2/config/weights", bytes.NewBufferString(`not-json`))
        w := httptest.NewRecorder()
        router.ServeHTTP(w, req)
        if w.Code != http.StatusBadRequest {
            t.Errorf("expected 400, got %d", w.Code)
        }
    })
}
