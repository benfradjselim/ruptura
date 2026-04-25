package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestLokiPush verifies the Loki-compatible push endpoint.
func TestLokiPush(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	payload := `{
		"streams": [
			{
				"stream": {"service": "web", "level": "error"},
				"values": [
					["1700000000000000000", "connection refused"]
				]
			}
		]
	}`

	resp, err := http.Post(srv.URL+"/loki/api/v1/push", "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("POST loki push: %v", err)
	}
	defer resp.Body.Close()
	// 204 No Content or 200
	if resp.StatusCode >= 500 {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("unexpected server error %d: %s", resp.StatusCode, body)
	}
}

// TestLokiQueryRange verifies the Loki-compatible query_range endpoint.
func TestLokiQueryRange(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/loki/api/v1/query_range?query={service=%22web%22}&limit=10&start=0&end=9999999999")
	if err != nil {
		t.Fatalf("GET loki query_range: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("unexpected server error %d: %s", resp.StatusCode, body)
	}
}

// TestLokiLabels verifies the labels endpoint.
func TestLokiLabels(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/loki/api/v1/labels")
	if err != nil {
		t.Fatalf("GET loki labels: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d; want 200", resp.StatusCode)
	}
}

// TestESBulkIngest verifies the Elasticsearch bulk endpoint.
func TestESBulkIngest(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	// Elasticsearch bulk format: alternating action + document lines
	body := `{"index":{"_index":"logs"}}
{"service":"web","level":"error","message":"timeout","@timestamp":"2024-01-01T00:00:00Z"}
{"index":{"_index":"logs"}}
{"service":"api","level":"info","message":"ok","@timestamp":"2024-01-01T00:00:01Z"}
`
	resp, err := http.Post(srv.URL+"/_bulk", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST _bulk: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		data, _ := io.ReadAll(resp.Body)
		t.Errorf("unexpected server error %d: %s", resp.StatusCode, data)
	}
}

// TestESSearch verifies the Elasticsearch search endpoint.
func TestESSearch(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"query": map[string]interface{}{
			"match_all": map[string]interface{}{},
		},
		"size": 10,
	})
	resp, err := http.Post(srv.URL+"/_search", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST _search: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		data, _ := io.ReadAll(resp.Body)
		t.Errorf("unexpected server error %d: %s", resp.StatusCode, data)
	}
}

// TestEsCatIndices verifies the cat indices endpoint.
func TestESCatIndices(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/_cat/indices")
	if err != nil {
		t.Fatalf("GET _cat/indices: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d; want 200", resp.StatusCode)
	}
}

// TestDatadogMetrics verifies the Datadog v1 metrics endpoint.
func TestDatadogMetrics(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"series": []map[string]interface{}{
			{
				"metric": "web.requests",
				"points": [][]float64{{1700000000, 42}},
				"type":   "gauge",
				"host":   "web-01",
			},
		},
	})
	resp, err := http.Post(srv.URL+"/api/v1/series", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST dd metrics: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		data, _ := io.ReadAll(resp.Body)
		t.Errorf("unexpected server error %d: %s", resp.StatusCode, data)
	}
}

// TestDatadogLogs verifies the Datadog v2 logs endpoint.
func TestDatadogLogs(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	body, _ := json.Marshal([]map[string]interface{}{
		{
			"ddsource": "web",
			"service":  "web",
			"message":  "request handled",
			"status":   "info",
		},
	})
	resp, err := http.Post(srv.URL+"/api/v2/logs", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST dd logs: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		data, _ := io.ReadAll(resp.Body)
		t.Errorf("unexpected server error %d: %s", resp.StatusCode, data)
	}
}

// TestOTLPTraces verifies the OTLP traces endpoint.
func TestOTLPTraces(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	body, _ := json.Marshal(map[string]interface{}{
		"resourceSpans": []map[string]interface{}{
			{
				"resource": map[string]interface{}{
					"attributes": []map[string]interface{}{
						{"key": "service.name", "value": map[string]string{"stringValue": "web"}},
					},
				},
				"scopeSpans": []map[string]interface{}{
					{
						"spans": []map[string]interface{}{
							{
								"traceId":           "deadbeefdeadbeef",
								"spanId":            "cafebabecafebabe",
								"parentSpanId":      "",
								"name":              "GET /api",
								"startTimeUnixNano": "1700000000000000000",
								"endTimeUnixNano":   "1700000001000000000",
								"status":            map[string]interface{}{"code": 1},
							},
						},
					},
				},
			},
		},
	})
	resp, err := http.Post(srv.URL+"/otlp/v1/traces", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST otlp traces: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		data, _ := io.ReadAll(resp.Body)
		t.Errorf("unexpected server error %d: %s", resp.StatusCode, data)
	}
}

// TestPrometheusMetricsEndpoint verifies the /metrics scrape endpoint.
func TestPrometheusMetricsEndpoint(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d; want 200", resp.StatusCode)
	}
}
