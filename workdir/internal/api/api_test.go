package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/benfradjselim/ohe/internal/alerter"
	"github.com/benfradjselim/ohe/internal/analyzer"
	"github.com/benfradjselim/ohe/internal/api"
	"github.com/benfradjselim/ohe/internal/predictor"
	"github.com/benfradjselim/ohe/internal/processor"
	"github.com/benfradjselim/ohe/internal/storage"
	"github.com/benfradjselim/ohe/pkg/models"
)

func setupServer(t *testing.T) *httptest.Server {
	t.Helper()
	dir, err := os.MkdirTemp("", "ohe-api-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	store, err := storage.Open(dir)
	if err != nil {
		t.Fatalf("Open storage: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	proc := processor.NewProcessor(1000)
	ana := analyzer.NewAnalyzer()
	pred := predictor.NewPredictor()
	alrt := alerter.NewAlerter(100)

	handlers := api.NewHandlers(store, proc, ana, pred, alrt, "test-secret", false)
	router := api.NewRouter(handlers, "test-secret", false, nil) // nil = wildcard CORS for tests
	return httptest.NewServer(router)
}

func TestHealthEndpoint(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d; want 200", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["success"] != true {
		t.Error("success should be true")
	}
}

func TestIngestAndKPIs(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	batch := models.MetricBatch{
		AgentID:   "test-agent",
		Host:      "testhost",
		Timestamp: time.Now(),
		Metrics: []models.Metric{
			{Name: "cpu_percent", Value: 60, Host: "testhost", Timestamp: time.Now()},
			{Name: "memory_percent", Value: 70, Host: "testhost", Timestamp: time.Now()},
			{Name: "load_avg_1", Value: 1.5, Host: "testhost", Timestamp: time.Now()},
		},
	}
	body, _ := json.Marshal(batch)
	resp, err := http.Post(srv.URL+"/api/v1/ingest", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /ingest: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("ingest status = %d; want 200", resp.StatusCode)
	}

	// Check KPIs are computed and accessible
	resp2, err := http.Get(srv.URL + "/api/v1/kpis?host=testhost")
	if err != nil {
		t.Fatalf("GET /kpis: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("kpis status = %d; want 200", resp2.StatusCode)
	}
}

func TestMetricsListEndpoint(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/metrics?host=localhost")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d; want 200", resp.StatusCode)
	}
}

func TestDashboardCRUD(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	// Create
	d := models.Dashboard{Name: "Test Dashboard", Refresh: 30}
	body, _ := json.Marshal(d)
	resp, err := http.Post(srv.URL+"/api/v1/dashboards", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /dashboards: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("create status = %d; want 201", resp.StatusCode)
	}

	var created map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&created)
	data := created["data"].(map[string]interface{})
	id := data["id"].(string)

	// Get
	resp2, err := http.Get(srv.URL + "/api/v1/dashboards/" + id)
	if err != nil {
		t.Fatalf("GET /dashboards/%s: %v", id, err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("get status = %d; want 200", resp2.StatusCode)
	}

	// List
	resp3, err := http.Get(srv.URL + "/api/v1/dashboards")
	if err != nil {
		t.Fatalf("GET /dashboards: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Errorf("list status = %d; want 200", resp3.StatusCode)
	}

	// Delete
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/dashboards/"+id, nil)
	resp4, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /dashboards/%s: %v", id, err)
	}
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusNoContent {
		t.Errorf("delete status = %d; want 204", resp4.StatusCode)
	}
}

func TestAlertsEndpoint(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/alerts")
	if err != nil {
		t.Fatalf("GET /alerts: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d; want 200", resp.StatusCode)
	}
}

func TestQueryEndpoint(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	qr := models.QueryRequest{
		Query: "cpu_percent",
		From:  time.Now().Add(-time.Hour),
		To:    time.Now(),
	}
	body, _ := json.Marshal(qr)
	resp, err := http.Post(srv.URL+"/api/v1/query?host=localhost", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /query: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("query status = %d; want 200", resp.StatusCode)
	}
}

func TestPredictEndpoint(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/predict?host=localhost&horizon=60")
	if err != nil {
		t.Fatalf("GET /predict: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d; want 200", resp.StatusCode)
	}
}

func TestSetupEndpoint(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	// First setup should succeed
	payload := map[string]string{"username": "testadmin", "password": "securepassword1"}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(srv.URL+"/api/v1/auth/setup", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /auth/setup: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("setup status = %d; want 201", resp.StatusCode)
	}

	// Second setup should fail with 409
	body2, _ := json.Marshal(payload)
	resp2, err := http.Post(srv.URL+"/api/v1/auth/setup", "application/json", bytes.NewReader(body2))
	if err != nil {
		t.Fatalf("POST /auth/setup (second): %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusConflict {
		t.Errorf("second setup status = %d; want 409", resp2.StatusCode)
	}
}

func TestTemplatesEndpoints(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	// List templates
	resp, err := http.Get(srv.URL + "/api/v1/templates")
	if err != nil {
		t.Fatalf("GET /templates: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("list templates status = %d; want 200", resp.StatusCode)
	}

	var listResult map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&listResult)
	data := listResult["data"].([]interface{})
	if len(data) == 0 {
		t.Error("expected at least one template")
	}

	// Get specific template
	resp2, err := http.Get(srv.URL + "/api/v1/templates/kpi-holistic")
	if err != nil {
		t.Fatalf("GET /templates/kpi-holistic: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("get template status = %d; want 200", resp2.StatusCode)
	}

	// Get non-existent template
	resp3, err := http.Get(srv.URL + "/api/v1/templates/nonexistent")
	if err != nil {
		t.Fatalf("GET /templates/nonexistent: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusNotFound {
		t.Errorf("nonexistent template status = %d; want 404", resp3.StatusCode)
	}

	// Apply a template
	applyBody, _ := json.Marshal(map[string]string{"name": "My KPI Board"})
	resp4, err := http.Post(srv.URL+"/api/v1/templates/system-overview/apply", "application/json", bytes.NewReader(applyBody))
	if err != nil {
		t.Fatalf("POST /templates/system-overview/apply: %v", err)
	}
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusCreated {
		t.Errorf("apply template status = %d; want 201", resp4.StatusCode)
	}
}

func TestCORSHeaders(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodOptions, srv.URL+"/api/v1/health", nil)
	req.Header.Set("Origin", "https://example.com")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS: %v", err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("Access-Control-Allow-Origin") == "" {
		t.Error("missing CORS header")
	}
}
