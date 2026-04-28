package ohe_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	ohe "github.com/benfradjselim/ohe-sdk-go"
)

// wrap returns an APIResponse envelope for testing.
func wrap(data interface{}) interface{} {
	return map[string]interface{}{
		"success":   true,
		"data":      data,
		"timestamp": time.Now(),
	}
}

func newTestServer(t *testing.T, mux *http.ServeMux) (*httptest.Server, *ohe.Client) {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, ohe.New(srv.URL, ohe.WithToken("test-token"))
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// --- Health ---

func TestHealth(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, wrap(ohe.HealthResponse{Status: "ok", Version: "5.0.0"}))
	})
	_, c := newTestServer(t, mux)

	h, err := c.Health(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if h.Status != "ok" {
		t.Errorf("got status %q, want ok", h.Status)
	}
	if h.Version != "5.0.0" {
		t.Errorf("got version %q, want 5.0.0", h.Version)
	}
}

func TestLiveness(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health/live", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, wrap(map[string]string{"status": "alive"}))
	})
	_, c := newTestServer(t, mux)
	if err := c.Liveness(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestReadiness(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health/ready", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, wrap(map[string]string{"status": "ready"}))
	})
	_, c := newTestServer(t, mux)
	if err := c.Readiness(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// --- Auth ---

func TestLogin(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]string
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req["username"] != "admin" || req["password"] != "secret" {
			writeJSON(w, 401, map[string]interface{}{"error": map[string]string{"code": "UNAUTHORIZED", "message": "bad creds"}})
			return
		}
		writeJSON(w, 200, wrap(ohe.LoginResponse{Token: "jwt-abc", User: ohe.User{Username: "admin", Role: "admin"}}))
	})
	_, c := newTestServer(t, mux)

	resp, err := c.Login(context.Background(), "admin", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Token != "jwt-abc" {
		t.Errorf("got token %q, want jwt-abc", resp.Token)
	}
}

func TestLoginBadCreds(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 401, map[string]interface{}{"error": map[string]string{"code": "UNAUTHORIZED", "message": "bad creds"}})
	})
	_, c := newTestServer(t, mux)

	_, err := c.Login(context.Background(), "x", "y")
	if err == nil {
		t.Fatal("expected error for bad credentials")
	}
	var apiErr *ohe.Error
	if !asError(err, &apiErr) {
		t.Fatalf("expected *ohe.Error, got %T", err)
	}
	if apiErr.StatusCode != 401 {
		t.Errorf("got status %d, want 401", apiErr.StatusCode)
	}
}

// asError mimics errors.As without importing errors for cleaner test code.
func asError(err error, target **ohe.Error) bool {
	if e, ok := err.(*ohe.Error); ok {
		*target = e
		return true
	}
	return false
}

// --- Metrics ---

func TestMetricsList(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/metrics", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, wrap([]string{"cpu_percent", "mem_percent"}))
	})
	_, c := newTestServer(t, mux)

	list, err := c.MetricsList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Errorf("got %d metrics, want 2", len(list))
	}
}

func TestMetricRange(t *testing.T) {
	now := time.Now()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/metrics/cpu_percent/range", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("host") != "web-01" {
			w.WriteHeader(400)
			return
		}
		pts := []ohe.DataPoint{{Timestamp: now, Value: 42.5}}
		writeJSON(w, 200, wrap(pts))
	})
	_, c := newTestServer(t, mux)

	pts, err := c.MetricRange(context.Background(), "cpu_percent", "web-01", now.Add(-time.Hour), now)
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 1 || pts[0].Value != 42.5 {
		t.Errorf("unexpected points: %v", pts)
	}
}

// --- Alerts ---

func TestAlertList(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/alerts", func(w http.ResponseWriter, _ *http.Request) {
		alerts := []ohe.Alert{
			{ID: "a1", Name: "high-cpu", Severity: "critical", Status: "active"},
		}
		writeJSON(w, 200, wrap(alerts))
	})
	_, c := newTestServer(t, mux)

	list, err := c.AlertList(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != "a1" {
		t.Errorf("unexpected alerts: %v", list)
	}
}

func TestAlertAcknowledge(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/alerts/a1/acknowledge", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(405)
			return
		}
		writeJSON(w, 200, wrap(nil))
	})
	_, c := newTestServer(t, mux)
	if err := c.AlertAcknowledge(context.Background(), "a1"); err != nil {
		t.Fatal(err)
	}
}

// --- SLOs ---

func TestSLOCreateAndGet(t *testing.T) {
	created := ohe.SLO{ID: "slo-1", Name: "uptime", Target: 99.9, Window: "30d"}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/slos", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			writeJSON(w, 200, wrap(created))
			return
		}
		writeJSON(w, 200, wrap([]ohe.SLO{created}))
	})
	mux.HandleFunc("/api/v1/slos/slo-1", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, wrap(created))
	})
	_, c := newTestServer(t, mux)

	out, err := c.SLOCreate(context.Background(), ohe.SLO{Name: "uptime", Target: 99.9, Window: "30d"})
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != "slo-1" {
		t.Errorf("got id %q, want slo-1", out.ID)
	}

	got, err := c.SLOGet(context.Background(), "slo-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Target != 99.9 {
		t.Errorf("got target %v, want 99.9", got.Target)
	}
}

// --- Ingest ---

func TestIngest(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/ingest", func(w http.ResponseWriter, r *http.Request) {
		var req ohe.IngestRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Host == "" {
			w.WriteHeader(400)
			return
		}
		writeJSON(w, 200, wrap(map[string]int{"written": len(req.Metrics)}))
	})
	_, c := newTestServer(t, mux)

	err := c.Ingest(context.Background(), ohe.IngestRequest{
		AgentID: "agent-1",
		Host:    "web-01",
		Metrics: []ohe.Metric{{Name: "cpu", Value: 55.0, Host: "web-01", Timestamp: time.Now()}},
	})
	if err != nil {
		t.Fatal(err)
	}
}

// --- API Keys ---

func TestAPIKeyCreateAndDelete(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/api-keys", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			resp := ohe.APIKeyCreateResponse{
				APIKey:       ohe.APIKey{ID: "k1", Name: "ci", Role: "operator", Active: true},
				PlaintextKey: "ohe_abc123secretkey",
			}
			writeJSON(w, 200, wrap(resp))
			return
		}
		writeJSON(w, 200, wrap([]ohe.APIKey{{ID: "k1", Name: "ci", Active: true}}))
	})
	mux.HandleFunc("/api/v1/api-keys/k1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			writeJSON(w, 200, wrap(nil))
		}
	})
	_, c := newTestServer(t, mux)

	key, err := c.APIKeyCreate(context.Background(), ohe.APIKeyCreateRequest{Name: "ci", Role: "operator"})
	if err != nil {
		t.Fatal(err)
	}
	if key.PlaintextKey == "" {
		t.Error("expected plaintext key in create response")
	}
	if err := c.APIKeyDelete(context.Background(), "k1"); err != nil {
		t.Fatal(err)
	}
}

// --- Error handling ---

func TestClientError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 503, map[string]interface{}{
			"error": map[string]string{"code": "UNAVAILABLE", "message": "storage down"},
		})
	})
	_, c := newTestServer(t, mux)

	_, err := c.Health(context.Background())
	var apiErr *ohe.Error
	if !asError(err, &apiErr) {
		t.Fatalf("expected *ohe.Error, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 503 {
		t.Errorf("got %d, want 503", apiErr.StatusCode)
	}
	if apiErr.Code != "UNAVAILABLE" {
		t.Errorf("got code %q, want UNAVAILABLE", apiErr.Code)
	}
}

// --- Client options ---

func TestWithAPIKey(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ohe_mykey" {
			w.WriteHeader(403)
			return
		}
		writeJSON(w, 200, wrap(ohe.HealthResponse{Status: "ok"}))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := ohe.New(srv.URL, ohe.WithAPIKey("ohe_mykey"))
	if _, err := c.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestWithOrgID(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Org-ID") != "acme" {
			w.WriteHeader(400)
			return
		}
		writeJSON(w, 200, wrap(ohe.HealthResponse{Status: "ok"}))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := ohe.New(srv.URL, ohe.WithToken("tok"), ohe.WithOrgID("acme"))
	if _, err := c.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
}
