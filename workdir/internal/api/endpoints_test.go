package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/benfradjselim/ruptura/internal/alerter"
	"github.com/benfradjselim/ruptura/internal/explain"
	"github.com/benfradjselim/ruptura/internal/storage"
	"github.com/benfradjselim/ruptura/internal/telemetry"
)

// newTestRouter builds a minimal Handlers+Router for endpoint tests.
func newTestRouter(t *testing.T) (http.Handler, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "ruptura-ep-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	store, err := storage.Open(dir)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("open store: %v", err)
	}
	met := telemetry.NewRegistry("test")
	hc := telemetry.NewHealthChecker()
	al := alerter.NewAlerter(10)
	exp := explain.NewEngine()
	h := New(store, nil, exp, al, nil, nil, nil, nil, met, hc, "")
	h.SetReady(true)
	cleanup := func() { store.Close(); os.RemoveAll(dir) }
	return h.NewRouter(), cleanup
}

func getJSON(t *testing.T, router http.Handler, path string) (int, []byte) {
	t.Helper()
	req, _ := http.NewRequest("GET", path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w.Code, w.Body.Bytes()
}

func TestEndpoint_Health(t *testing.T) {
	router, cleanup := newTestRouter(t)
	defer cleanup()

	code, body := getJSON(t, router, "/api/v2/health")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", code, body)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if m["status"] == nil {
		t.Error("health response missing 'status' field")
	}
}

func TestEndpoint_Fleet_ReturnsObject(t *testing.T) {
	router, cleanup := newTestRouter(t)
	defer cleanup()

	code, body := getJSON(t, router, "/api/v2/fleet")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", code, body)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := m["hosts"]; !ok {
		t.Error("fleet response missing 'hosts' field")
	}
	if _, ok := m["total_hosts"]; !ok {
		t.Error("fleet response missing 'total_hosts' field")
	}
}

func TestEndpoint_Ruptures_ReturnsArray(t *testing.T) {
	router, cleanup := newTestRouter(t)
	defer cleanup()

	code, body := getJSON(t, router, "/api/v2/ruptures")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", code, body)
	}
	var arr []interface{}
	if err := json.Unmarshal(body, &arr); err != nil {
		t.Fatalf("decode (expected array): %v — body: %s", err, body)
	}
}

func TestEndpoint_Alerts_ReturnsArray(t *testing.T) {
	router, cleanup := newTestRouter(t)
	defer cleanup()

	code, body := getJSON(t, router, "/api/v2/alerts")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", code, body)
	}
	var arr []interface{}
	if err := json.Unmarshal(body, &arr); err != nil {
		t.Fatalf("decode (expected array): %v — body: %s", err, body)
	}
}

func TestEndpoint_Nodes_ReturnsArray(t *testing.T) {
	router, cleanup := newTestRouter(t)
	defer cleanup()

	code, body := getJSON(t, router, "/api/v2/nodes")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", code, body)
	}
	var arr []interface{}
	if err := json.Unmarshal(body, &arr); err != nil {
		t.Fatalf("decode (expected array): %v — body: %s", err, body)
	}
}

func TestEndpoint_Topology_ReturnsGraph(t *testing.T) {
	router, cleanup := newTestRouter(t)
	defer cleanup()

	code, body := getJSON(t, router, "/api/v2/topology")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", code, body)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := m["nodes"]; !ok {
		t.Error("topology response missing 'nodes' field")
	}
	if _, ok := m["edges"]; !ok {
		t.Error("topology response missing 'edges' field")
	}
}

func TestEndpoint_EngineStatus_ReturnsObject(t *testing.T) {
	router, cleanup := newTestRouter(t)
	defer cleanup()

	code, body := getJSON(t, router, "/api/v2/engine/status")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", code, body)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := m["analyzer"]; !ok {
		t.Error("engine status missing 'analyzer' field")
	}
	if _, ok := m["ingest"]; !ok {
		t.Error("engine status missing 'ingest' field")
	}
}

func TestEndpoint_Suppressions_ReturnsArray(t *testing.T) {
	router, cleanup := newTestRouter(t)
	defer cleanup()

	code, body := getJSON(t, router, "/api/v2/suppressions")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", code, body)
	}
	var arr []interface{}
	if err := json.Unmarshal(body, &arr); err != nil {
		t.Fatalf("decode (expected array): %v — body: %s", err, body)
	}
}

func TestEndpoint_Dataflow_ReturnsObject(t *testing.T) {
	router, cleanup := newTestRouter(t)
	defer cleanup()

	code, body := getJSON(t, router, "/api/v2/dataflow")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d — %s", code, body)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := m["metrics"]; !ok {
		t.Error("dataflow response missing 'metrics' field")
	}
}

func TestEndpoint_UnknownPath_Returns404(t *testing.T) {
	router, cleanup := newTestRouter(t)
	defer cleanup()

	code, _ := getJSON(t, router, "/api/v2/nonexistent-endpoint-xyz")
	if code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown path, got %d", code)
	}
}
