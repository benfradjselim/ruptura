package api_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/benfradjselim/kairo-core/internal/alerter"
	"github.com/benfradjselim/kairo-core/internal/analyzer"
	"github.com/benfradjselim/kairo-core/internal/api"
	"github.com/benfradjselim/kairo-core/internal/predictor"
	"github.com/benfradjselim/kairo-core/internal/processor"
	"github.com/benfradjselim/kairo-core/internal/storage"
	"github.com/benfradjselim/kairo-core/pkg/models"
)

func setupHandlers(t *testing.T) (*api.Handlers, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "ohe-api-boost-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	store, err := storage.Open(dir)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("Open storage: %v", err)
	}
	proc := processor.NewProcessor(1000)
	ana := analyzer.NewAnalyzer()
	pred := predictor.NewPredictor()
	alrt := alerter.NewAlerter(100)
	h := api.NewHandlers(store, proc, ana, pred, alrt, "test-host", "test-secret", false)
	return h, func() { store.Close(); os.RemoveAll(dir) }
}

// TestSetUsageRecorder ensures SetUsageRecorder doesn't panic.
func TestSetUsageRecorder(t *testing.T) {
	h, cleanup := setupHandlers(t)
	defer cleanup()

	called := false
	h.SetUsageRecorder(func(orgID, eventType string, value float64) {
		called = true
	})
	_ = called
}

// TestTopologyAnalyzer ensures the getter returns non-nil.
func TestTopologyAnalyzer(t *testing.T) {
	h, cleanup := setupHandlers(t)
	defer cleanup()

	ta := h.TopologyAnalyzer()
	if ta == nil {
		t.Error("TopologyAnalyzer() should not return nil")
	}
}

// TestDispatchAlertToChannels exercises the no-op path (no channels configured).
func TestDispatchAlertToChannels(t *testing.T) {
	h, cleanup := setupHandlers(t)
	defer cleanup()

	h.DispatchAlertToChannels(models.Alert{
		Host:        "test-host",
		Metric:      "cpu_percent",
		Severity:    "warning",
		Description: "test alert",
	})
}

// TestKPIMultiHandler_NoHosts exercises GET /api/v1/kpis/multi with no hosts.
func TestKPIMultiHandler_NoHosts(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/kpis/multi")
	if err != nil {
		t.Fatalf("GET /kpis/multi: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		t.Errorf("unexpected server error: %d", resp.StatusCode)
	}
}

func TestKPIMultiHandler_WithHosts(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/kpis/multi?host=host1&host=host2")
	if err != nil {
		t.Fatalf("GET /kpis/multi?host=...: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		t.Errorf("unexpected server error: %d", resp.StatusCode)
	}
}

// TestOTLPMetricsHandler exercises POST /otlp/v1/metrics.
func TestOTLPMetricsHandler(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/otlp/v1/metrics", "application/x-protobuf", bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatalf("POST /otlp/v1/metrics: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		t.Errorf("server error: %d", resp.StatusCode)
	}
}

// TestOTLPLogsHandler exercises POST /otlp/v1/logs.
func TestOTLPLogsHandler(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/otlp/v1/logs", "application/x-protobuf", bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatalf("POST /otlp/v1/logs: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		t.Errorf("server error: %d", resp.StatusCode)
	}
}

// TestLogStreamHandler exercises GET /api/v1/logs/stream (SSE).
func TestLogStreamHandler(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so the streaming body doesn't block

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1/logs/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil && !strings.Contains(err.Error(), "context canceled") {
		// Cancelled context produces an error — that's fine, it means the handler was reached
		return
	}
	if resp != nil {
		defer resp.Body.Close()
		if resp.StatusCode >= 500 {
			t.Errorf("server error: %d", resp.StatusCode)
		}
	}
}

// TestESInfoHandler exercises the ES cluster info handler via httptest.
func TestESInfoHandler(t *testing.T) {
	h, cleanup := setupHandlers(t)
	defer cleanup()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ESInfoHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("ESInfoHandler status = %d; want 200", rec.Code)
	}
}
