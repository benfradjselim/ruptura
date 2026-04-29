package ingest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/pipeline/metrics"
	"github.com/benfradjselim/ruptura/pkg/models"
)

type mockPipeline struct {
	metrics.MetricPipeline
	ingested []struct {
		host, name string
		value      float64
	}
}

func (m *mockPipeline) Ingest(host, metric string, value float64, ts time.Time) {
	m.ingested = append(m.ingested, struct {
		host, name string
		value      float64
	}{host, metric, value})
}

type mockLogs struct {
	lines int
}

func (m *mockLogs) IngestLine(service string, line []byte, ts time.Time) {
	m.lines++
}

type mockSpans struct {
	spans []models.Span
}

func (m *mockSpans) IngestSpan(span models.Span) error {
	m.spans = append(m.spans, span)
	return nil
}

func TestRemoteWrite_valid(t *testing.T) {
	mp := &mockPipeline{}
	e := New(mp, nil, nil, nil, nil)
	
	body := `{"timeseries": [{"labels": [{"name":"__name__","value":"cpu"}, {"name":"host","value":"h1"}], "samples": [{"value":1.0,"timestamp":1700000000000}]}]}`
	req := httptest.NewRequest("POST", "/api/v2/write", strings.NewReader(body))
	w := httptest.NewRecorder()
	
	e.handleRemoteWrite(w, req)
	
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
	if len(mp.ingested) != 1 {
		t.Errorf("expected 1 metric, got %d", len(mp.ingested))
	}
}

func TestRemoteWrite_wrongMethod(t *testing.T) {
	e := New(nil, nil, nil, nil, nil)
	req := httptest.NewRequest("GET", "/api/v2/write", nil)
	w := httptest.NewRecorder()
	
	e.handleRemoteWrite(w, req)
	
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestRemoteWrite_badJSON(t *testing.T) {
	e := New(nil, nil, nil, nil, nil)
	req := httptest.NewRequest("POST", "/api/v2/write", strings.NewReader("{invalid"))
	w := httptest.NewRecorder()
	
	e.handleRemoteWrite(w, req)
	
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestRemoteWrite_missingName(t *testing.T) {
	mp := &mockPipeline{}
	e := New(mp, nil, nil, nil, nil)
	
	body := `{"timeseries": [{"labels": [{"name":"host","value":"h1"}], "samples": [{"value":1.0,"timestamp":1700000000000}]}]}`
	req := httptest.NewRequest("POST", "/api/v2/write", strings.NewReader(body))
	w := httptest.NewRecorder()
	
	e.handleRemoteWrite(w, req)
	
	if len(mp.ingested) != 0 {
		t.Errorf("expected 0 metrics, got %d", len(mp.ingested))
	}
}

func TestOTLPMetrics_gauge(t *testing.T) {
	mp := &mockPipeline{}
	e := New(mp, nil, nil, nil, nil)
	
	body := `{"resourceMetrics":[{"resource":{"attributes":[{"key":"host.name","value":{"stringValue":"h1"}}]},"scopeMetrics":[{"metrics":[{"name":"cpu","gauge":{"dataPoints":[{"asDouble":1.0,"timeUnixNano":"1000000000"}]}}]}]}]}`
	req := httptest.NewRequest("POST", "/otlp/v1/metrics", strings.NewReader(body))
	w := httptest.NewRecorder()
	
	e.handleOTLPMetrics(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if len(mp.ingested) != 1 {
		t.Errorf("expected 1 metric, got %d", len(mp.ingested))
	}
}

func TestOTLPMetrics_sum(t *testing.T) {
	mp := &mockPipeline{}
	e := New(mp, nil, nil, nil, nil)
	
	body := `{"resourceMetrics":[{"resource":{"attributes":[{"key":"host.name","value":{"stringValue":"h1"}}]},"scopeMetrics":[{"metrics":[{"name":"mem","sum":{"dataPoints":[{"asInt":10,"timeUnixNano":"1000000000"}]}}]}]}]}`
	req := httptest.NewRequest("POST", "/otlp/v1/metrics", strings.NewReader(body))
	w := httptest.NewRecorder()
	
	e.handleOTLPMetrics(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if len(mp.ingested) != 1 {
		t.Errorf("expected 1 metric, got %d", len(mp.ingested))
	}
}

func TestOTLPLogs(t *testing.T) {
	logs := &mockLogs{}
	e := New(nil, logs, nil, nil, nil)
	
	body := `{"resourceLogs":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"svc1"}}]},"scopeLogs":[{"logRecords":[{"body":{"stringValue":"err"},"timeUnixNano":"1000000000"}]}]}]}`
	req := httptest.NewRequest("POST", "/otlp/v1/logs", strings.NewReader(body))
	w := httptest.NewRecorder()
	
	e.handleOTLPLogs(w, req)
	
	if logs.lines != 1 {
		t.Errorf("expected 1 log, got %d", logs.lines)
	}
}

func TestOTLPTraces_ok(t *testing.T) {
	spans := &mockSpans{}
	e := New(nil, nil, spans, nil, nil)
	
	body := `{"resourceSpans":[{"scopeSpans":[{"spans":[{"traceId":"t1","spanId":"s1","name":"span1","status":{"code":1}}]}]}]}`
	req := httptest.NewRequest("POST", "/otlp/v1/traces", strings.NewReader(body))
	w := httptest.NewRecorder()
	
	e.handleOTLPTraces(w, req)
	
	if len(spans.spans) != 1 || spans.spans[0].Status != "ok" {
		t.Errorf("expected ok status, got %v", spans.spans)
	}
}

func TestOTLPTraces_error(t *testing.T) {
	spans := &mockSpans{}
	e := New(nil, nil, spans, nil, nil)
	
	body := `{"resourceSpans":[{"scopeSpans":[{"spans":[{"traceId":"t1","spanId":"s1","name":"span1","status":{"code":2}}]}]}]}`
	req := httptest.NewRequest("POST", "/otlp/v1/traces", strings.NewReader(body))
	w := httptest.NewRecorder()
	
	e.handleOTLPTraces(w, req)
	
	if len(spans.spans) != 1 || spans.spans[0].Status != "error" {
		t.Errorf("expected error status, got %v", spans.spans)
	}
}

func TestDogStatsD_gauge(t *testing.T) {
	mp := &mockPipeline{}
	e := New(mp, nil, nil, nil, nil)
	
	e.SendDogStatsDPacket([]byte("cpu:88|g|#host:db-01"))
	
	if len(mp.ingested) != 1 || mp.ingested[0].host != "db-01" || mp.ingested[0].value != 88 {
		t.Errorf("expected host db-01 val 88, got %v", mp.ingested)
	}
}

func TestDogStatsD_counter(t *testing.T) {
	mp := &mockPipeline{}
	e := New(mp, nil, nil, nil, nil)
	
	e.SendDogStatsDPacket([]byte("req:10|c"))
	
	if len(mp.ingested) != 1 || mp.ingested[0].host != "unknown" {
		t.Errorf("expected host unknown, got %v", mp.ingested)
	}
}

func TestDogStatsD_multiline(t *testing.T) {
	mp := &mockPipeline{}
	e := New(mp, nil, nil, nil, nil)
	
	e.SendDogStatsDPacket([]byte("m1:1|c\nm2:2|c"))
	
	if len(mp.ingested) != 2 {
		t.Errorf("expected 2 metrics, got %d", len(mp.ingested))
	}
}

func TestDogStatsD_invalid(t *testing.T) {
	mp := &mockPipeline{}
	e := New(mp, nil, nil, nil, nil)
	
	e.SendDogStatsDPacket([]byte("invalid"))
	
	if len(mp.ingested) != 0 {
		t.Errorf("expected 0 metrics, got %d", len(mp.ingested))
	}
}


func TestStartHTTP(t *testing.T) {
	e := New(nil, nil, nil, nil, nil)
	if err := e.StartHTTP(":0"); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	e.Stop(context.Background())
}

func TestStartGRPC(t *testing.T) {
	e := New(nil, nil, nil, nil, nil)
	if err := e.StartGRPC(":0"); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestStartDogStatsD(t *testing.T) {
	e := New(nil, nil, nil, nil, nil)
	// Use a random port
	if err := e.StartDogStatsD("127.0.0.1:0"); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	e.Stop(context.Background())
}

func TestRegisterHandlers(t *testing.T) {
	mux := http.NewServeMux()
	e := New(nil, nil, nil, nil, nil)
	RegisterHandlers(mux, e)
}

func TestStop_NilServers(t *testing.T) {
	e := New(nil, nil, nil, nil, nil)
	if err := e.Stop(context.Background()); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestStop_WithServers(t *testing.T) {
	e := New(nil, nil, nil, nil, nil)
	e.StartHTTP(":0")
	e.StartDogStatsD("127.0.0.1:0")
	if err := e.Stop(context.Background()); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestCardinality_limit(t *testing.T) {
	mp := &mockPipeline{}
	e := New(mp, nil, nil, nil, nil)
	
	for i := 0; i < 50005; i++ {
		e.checkCardinality("h1", "m"+fmt.Sprint(i))
	}
	
	// Should be 50000
	if atomic.LoadInt32(&e.seriesCount) != 50000 {
		t.Errorf("expected 50000, got %d", e.seriesCount)
	}
}
