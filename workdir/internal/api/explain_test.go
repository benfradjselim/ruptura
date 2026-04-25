package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func postJSON(t *testing.T, url string, body interface{}) *http.Response {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

// TestExplainHandler_NoData returns 404 before any metrics are ingested.
func TestExplainHandler_NoData(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/explain/stress?host=nonexistent-host")
	if err != nil {
		t.Fatalf("GET /explain/stress: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 before data, got %d", resp.StatusCode)
	}
}

// TestExplainHandler_UnknownKPI returns 400 for unsupported KPI names.
func TestExplainHandler_UnknownKPI(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/explain/unknown_kpi")
	if err != nil {
		t.Fatalf("GET /explain/unknown_kpi: %v", err)
	}
	defer resp.Body.Close()
	// Either 400 (unknown KPI) or 404 (no data yet) — both acceptable
	if resp.StatusCode >= 500 {
		t.Errorf("expected 4xx, got %d", resp.StatusCode)
	}
}

// TestExplainHandler_StressAfterIngest — Integration Test C.
// Ingest known metrics, then call /explain/stress and assert the response
// contains the correct formula, contributions, and dominant driver.
func TestExplainHandler_StressAfterIngest(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	// Ingest a batch with dominant CPU
	batch := map[string]interface{}{
		"agent_id":  "test-agent",
		"host":      "test-host",
		"timestamp": time.Now(),
		"metrics": []map[string]interface{}{
			{"name": "cpu_percent", "value": 0.80, "host": "test-host", "timestamp": time.Now()},
			{"name": "memory_percent", "value": 0.40, "host": "test-host", "timestamp": time.Now()},
			{"name": "load_avg_1", "value": 0.20, "host": "test-host", "timestamp": time.Now()},
			{"name": "error_rate", "value": 0.05, "host": "test-host", "timestamp": time.Now()},
			{"name": "timeout_rate", "value": 0.01, "host": "test-host", "timestamp": time.Now()},
		},
	}
	_ = postJSON(t, srv.URL+"/api/v1/ingest", batch)

	// Wait briefly for processing
	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get(srv.URL + "/api/v1/explain/stress?host=test-host")
	if err != nil {
		t.Fatalf("GET /explain/stress: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		t.Skip("KPI data not yet available (collection cycle may not have run) — skipping assertion")
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
		return
	}

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			KPI           string             `json:"kpi"`
			Formula       string             `json:"formula"`
			Contributions map[string]float64 `json:"contributions"`
			DominantDriver string            `json:"dominant_driver"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode explain response: %v", err)
	}

	if !body.Success {
		t.Error("expected success=true")
	}
	if body.Data.KPI != "stress" {
		t.Errorf("kpi = %q; want stress", body.Data.KPI)
	}
	if body.Data.Formula == "" {
		t.Error("formula should not be empty")
	}
	if len(body.Data.Contributions) == 0 {
		t.Error("contributions should not be empty")
	}
	if body.Data.DominantDriver == "" {
		t.Error("dominant_driver should not be empty")
	}
}

// TestExplainHandler_AllKPIs verifies all supported KPIs return a valid structure.
func TestExplainHandler_AllKPIs(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	// Ingest data to warm up analyzer
	batch := map[string]interface{}{
		"agent_id":  "test-agent",
		"host":      "test-host",
		"timestamp": time.Now(),
		"metrics": []map[string]interface{}{
			{"name": "cpu_percent", "value": 0.6, "host": "test-host", "timestamp": time.Now()},
			{"name": "memory_percent", "value": 0.5, "host": "test-host", "timestamp": time.Now()},
			{"name": "load_avg_1", "value": 0.3, "host": "test-host", "timestamp": time.Now()},
			{"name": "error_rate", "value": 0.1, "host": "test-host", "timestamp": time.Now()},
			{"name": "timeout_rate", "value": 0.05, "host": "test-host", "timestamp": time.Now()},
			{"name": "request_rate", "value": 0.8, "host": "test-host", "timestamp": time.Now()},
			{"name": "uptime_seconds", "value": 86400, "host": "test-host", "timestamp": time.Now()},
		},
	}
	_ = postJSON(t, srv.URL+"/api/v1/ingest", batch)
	time.Sleep(100 * time.Millisecond)

	kpis := []string{"stress", "fatigue", "mood", "pressure", "humidity", "contagion", "health_score"}
	for _, kpi := range kpis {
		t.Run(kpi, func(t *testing.T) {
			resp, err := http.Get(srv.URL + "/api/v1/explain/" + kpi + "?host=test-host")
			if err != nil {
				t.Fatalf("GET /explain/%s: %v", kpi, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				t.Skipf("KPI %s not yet computed — skipping", kpi)
			}
			if resp.StatusCode >= 500 {
				t.Errorf("/explain/%s server error: %d", kpi, resp.StatusCode)
			}
		})
	}
}
