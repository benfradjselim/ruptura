package ohe_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/benfradjselim/ohe/sdk/go/ohe"
)

// stubServer starts an httptest.Server that routes stub responses.
func stubServer(t *testing.T, mux *http.ServeMux) (*ohe.Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := ohe.New(srv.URL, "test-token")
	return client, srv
}

func jsonResp(w http.ResponseWriter, code int, data interface{}) {
	env := map[string]interface{}{"data": data}
	b, _ := json.Marshal(env)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(b)
}

func TestHealth(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		jsonResp(w, 200, map[string]string{"status": "ok"})
	})
	c, _ := stubServer(t, mux)
	if !c.Health(context.Background()) {
		t.Error("Health() should return true for 200 response")
	}
}

func TestHealthFail(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	})
	c, _ := stubServer(t, mux)
	if c.Health(context.Background()) {
		t.Error("Health() should return false for 503 response")
	}
}

func TestIngest(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/ingest", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("want POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("missing auth header")
		}
		w.WriteHeader(204)
	})
	c, _ := stubServer(t, mux)
	err := c.Ingest(context.Background(), []ohe.Metric{
		{Host: "web-01", Name: "cpu_percent", Value: 72.5, Timestamp: time.Now()},
	})
	if err != nil {
		t.Errorf("Ingest: %v", err)
	}
}

func TestIngestAPIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/ingest", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(402)
		w.Write([]byte(`{"error":"quota_exceeded"}`))
	})
	c, _ := stubServer(t, mux)
	err := c.Ingest(context.Background(), []ohe.Metric{{Host: "h", Name: "cpu", Value: 1}})
	if err == nil {
		t.Fatal("expected error from 402 response")
	}
	apiErr, ok := err.(*ohe.APIError)
	if !ok {
		t.Fatalf("want *APIError, got %T", err)
	}
	if apiErr.StatusCode != 402 {
		t.Errorf("StatusCode = %d; want 402", apiErr.StatusCode)
	}
}

func TestMetricRange(t *testing.T) {
	points := []map[string]interface{}{
		{"timestamp": "2024-01-01T00:00:00Z", "value": 55.0},
		{"timestamp": "2024-01-01T01:00:00Z", "value": 60.0},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/metrics/cpu_percent/range", func(w http.ResponseWriter, r *http.Request) {
		jsonResp(w, 200, points)
	})
	c, _ := stubServer(t, mux)
	got, err := c.MetricRange(context.Background(), "cpu_percent", "web-01",
		time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatalf("MetricRange: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 points, got %d", len(got))
	}
}

func TestListAlerts(t *testing.T) {
	alerts := []map[string]interface{}{
		{"id": "a1", "host": "db-01", "severity": "critical"},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/alerts", func(w http.ResponseWriter, r *http.Request) {
		jsonResp(w, 200, alerts)
	})
	c, _ := stubServer(t, mux)
	got, err := c.ListAlerts(context.Background())
	if err != nil {
		t.Fatalf("ListAlerts: %v", err)
	}
	if len(got) != 1 || got[0].ID != "a1" {
		t.Errorf("unexpected alerts: %+v", got)
	}
}

func TestAcknowledgeAlert(t *testing.T) {
	called := false
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/alerts/a1/acknowledge", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(200)
	})
	c, _ := stubServer(t, mux)
	if err := c.AcknowledgeAlert(context.Background(), "a1"); err != nil {
		t.Errorf("AcknowledgeAlert: %v", err)
	}
	if !called {
		t.Error("acknowledge endpoint not called")
	}
}

func TestAlertRuleCRUD(t *testing.T) {
	rules := []ohe.AlertRule{{Name: "cpu-high", Metric: "cpu_percent", Threshold: 90, Severity: "critical"}}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/alert-rules", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			jsonResp(w, 200, rules)
		case http.MethodPost:
			var rule ohe.AlertRule
			json.NewDecoder(r.Body).Decode(&rule)
			jsonResp(w, 201, rule)
		}
	})
	mux.HandleFunc("/api/v1/alert-rules/cpu-high", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			w.WriteHeader(200)
		case http.MethodDelete:
			w.WriteHeader(200)
		}
	})
	c, _ := stubServer(t, mux)

	listed, err := c.ListAlertRules(context.Background())
	if err != nil || len(listed) != 1 {
		t.Fatalf("ListAlertRules: err=%v len=%d", err, len(listed))
	}

	created, err := c.CreateAlertRule(context.Background(), ohe.AlertRule{Name: "mem-high", Metric: "mem_percent", Threshold: 85, Severity: "warning"})
	if err != nil {
		t.Fatalf("CreateAlertRule: %v", err)
	}
	if created.Name != "mem-high" {
		t.Errorf("created name = %q", created.Name)
	}

	if err := c.UpdateAlertRule(context.Background(), "cpu-high", rules[0]); err != nil {
		t.Errorf("UpdateAlertRule: %v", err)
	}
	if err := c.DeleteAlertRule(context.Background(), "cpu-high"); err != nil {
		t.Errorf("DeleteAlertRule: %v", err)
	}
}

func TestDashboardCRUD(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/dashboards", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			jsonResp(w, 200, []ohe.Dashboard{{ID: "d1", Name: "Overview"}})
		case http.MethodPost:
			var d ohe.Dashboard
			json.NewDecoder(r.Body).Decode(&d)
			d.ID = "d2"
			jsonResp(w, 201, d)
		}
	})
	mux.HandleFunc("/api/v1/dashboards/d1", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			jsonResp(w, 200, ohe.Dashboard{ID: "d1", Name: "Overview"})
		case http.MethodDelete:
			w.WriteHeader(200)
		}
	})
	c, _ := stubServer(t, mux)

	listed, err := c.ListDashboards(context.Background())
	if err != nil || len(listed) != 1 {
		t.Fatalf("ListDashboards: %v / %d", err, len(listed))
	}
	got, err := c.GetDashboard(context.Background(), "d1")
	if err != nil || got.ID != "d1" {
		t.Fatalf("GetDashboard: %v", err)
	}
	created, err := c.CreateDashboard(context.Background(), ohe.Dashboard{Name: "New"})
	if err != nil || created.ID != "d2" {
		t.Fatalf("CreateDashboard: %v / %+v", err, created)
	}
	if err := c.DeleteDashboard(context.Background(), "d1"); err != nil {
		t.Errorf("DeleteDashboard: %v", err)
	}
}

func TestSLOCRUD(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/slos", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			jsonResp(w, 200, []ohe.SLO{{ID: "s1", Name: "uptime", Target: 99.9}})
		case http.MethodPost:
			var s ohe.SLO
			json.NewDecoder(r.Body).Decode(&s)
			s.ID = "s2"
			jsonResp(w, 201, s)
		}
	})
	mux.HandleFunc("/api/v1/slos/s1", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			jsonResp(w, 200, ohe.SLO{ID: "s1", Name: "uptime", Target: 99.9})
		case http.MethodDelete:
			w.WriteHeader(200)
		}
	})
	mux.HandleFunc("/api/v1/slos/s1/status", func(w http.ResponseWriter, r *http.Request) {
		jsonResp(w, 200, ohe.SLOStatus{SLOID: "s1", Current: 99.95, Target: 99.9, Compliant: true})
	})
	c, _ := stubServer(t, mux)

	listed, err := c.ListSLOs(context.Background())
	if err != nil || len(listed) != 1 {
		t.Fatalf("ListSLOs: %v", err)
	}
	got, err := c.GetSLO(context.Background(), "s1")
	if err != nil || got.ID != "s1" {
		t.Fatalf("GetSLO: %v", err)
	}
	created, err := c.CreateSLO(context.Background(), ohe.SLO{Name: "latency", Target: 99.0})
	if err != nil || created.ID != "s2" {
		t.Fatalf("CreateSLO: %v", err)
	}
	st, err := c.SLOStatus(context.Background(), "s1")
	if err != nil || !st.Compliant {
		t.Fatalf("SLOStatus: %v / %+v", err, st)
	}
	if err := c.DeleteSLO(context.Background(), "s1"); err != nil {
		t.Errorf("DeleteSLO: %v", err)
	}
}

func TestWithTimeout(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	c, _ := stubServer(t, mux)
	_ = c // WithTimeout is exercised at construction time
	c2 := ohe.New("http://localhost:1", "tok", ohe.WithTimeout(10*time.Millisecond))
	if c2.Health(context.Background()) {
		t.Error("expected Health to fail with no server")
	}
}
