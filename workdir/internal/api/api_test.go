package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"encoding/json"

	"github.com/benfradjselim/ruptura/internal/telemetry"
)

func TestAPI(t *testing.T) {
	met := telemetry.NewRegistry("6.0.0")
	hc := telemetry.NewHealthChecker()
	h := New(nil, nil, met, hc, "token")
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
		hAutopilot := New(nil, nil, met, hc, "token")
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

func TestWorkloadK8sEndpoint(t *testing.T) {
	met := telemetry.NewRegistry("test")
	hc := telemetry.NewHealthChecker()
	h := New(nil, nil, met, hc, "test-key")
	h.SetReady(true)
	router := h.NewRouter()

	t.Run("returns 503 when discovery is nil", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v2/workloads/prod/Deployment/api/k8s", nil)
		req.Header.Set("Authorization", "Bearer test-key")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d: %s", w.Code, w.Body.String())
		}
		var resp map[string]string
		_ = json.NewDecoder(w.Body).Decode(&resp)
		if resp["error"] == "" {
			t.Error("expected error message in response")
		}
	})
}

func TestNodesEndpoint(t *testing.T) {
	met := telemetry.NewRegistry("test")
	hc := telemetry.NewHealthChecker()
	h := New(nil, nil, met, hc, "test-key")
	h.SetReady(true)
	router := h.NewRouter()

	t.Run("returns empty array when store is nil", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v2/nodes", nil)
		req.Header.Set("Authorization", "Bearer test-key")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var resp []interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("expected JSON array: %v", err)
		}
		if len(resp) != 0 {
			t.Errorf("expected empty array, got %d items", len(resp))
		}
	})

	t.Run("node detail returns 404 when store is nil", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v2/nodes/node-1", nil)
		req.Header.Set("Authorization", "Bearer test-key")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestEngineStorageEndpoint(t *testing.T) {
	met := telemetry.NewRegistry("test")
	hc := telemetry.NewHealthChecker()
	h := New(nil, nil, met, hc, "test-key")
	h.SetReady(true)
	router := h.NewRouter()

	t.Run("returns 200 with badger section when store is nil", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v2/engine/storage", nil)
		req.Header.Set("Authorization", "Bearer test-key")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		if _, ok := resp["badger"]; !ok {
			t.Error("response missing badger section")
		}
	})
}
