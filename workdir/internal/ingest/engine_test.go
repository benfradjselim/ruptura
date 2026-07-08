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

// TestOTLPMetrics_PodResolvesToOwningDeployment is FBL-V2's core regression
// test: a pod-scoped OTLP resource (k8s.pod.name, no explicit deployment
// attribute — the shape a cAdvisor/eBPF-style exporter actually sends) must
// register under its owning Deployment, not the pod's own ReplicaSet-hash
// name, once an owner resolver is wired.
func TestOTLPMetrics_PodResolvesToOwningDeployment(t *testing.T) {
	mp := &mockPipeline{}
	e := New(mp, nil, nil, nil, nil)
	e.SetOwnerResolver(func(ns, podName string) (kind, name string, ok bool) {
		if ns == "prod" && podName == "web-stable-58cdf6849d-n5l2d" {
			return "Deployment", "web-stable", true
		}
		return "", "", false
	})

	body := `{"resourceMetrics":[{"resource":{"attributes":[
		{"key":"k8s.namespace.name","value":{"stringValue":"prod"}},
		{"key":"k8s.pod.name","value":{"stringValue":"web-stable-58cdf6849d-n5l2d"}}
	]},"scopeMetrics":[{"metrics":[{"name":"cpu","gauge":{"dataPoints":[{"asDouble":0.5,"timeUnixNano":"1000000000"}]}}]}]}]}`
	req := httptest.NewRequest("POST", "/otlp/v1/metrics", strings.NewReader(body))
	w := httptest.NewRecorder()

	e.handleOTLPMetrics(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if len(mp.ingested) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(mp.ingested))
	}
	if got, want := mp.ingested[0].host, "prod/Deployment/web-stable"; got != want {
		t.Errorf("host = %q, want %q (owning Deployment, not the pod name)", got, want)
	}
}

// TestOTLPMetrics_PodOwnerUnresolved_FallsBackToPodAsHost covers the
// transient case (no resolver wired, or the informer hasn't seen this pod's
// owner yet): telemetry must still be ingested, degraded to a pod-keyed
// "host" identity, rather than silently dropped.
func TestOTLPMetrics_PodOwnerUnresolved_FallsBackToPodAsHost(t *testing.T) {
	mp := &mockPipeline{}
	e := New(mp, nil, nil, nil, nil) // no SetOwnerResolver call

	body := `{"resourceMetrics":[{"resource":{"attributes":[
		{"key":"k8s.namespace.name","value":{"stringValue":"prod"}},
		{"key":"k8s.pod.name","value":{"stringValue":"web-stable-58cdf6849d-n5l2d"}}
	]},"scopeMetrics":[{"metrics":[{"name":"cpu","gauge":{"dataPoints":[{"asDouble":0.5,"timeUnixNano":"1000000000"}]}}]}]}]}`
	req := httptest.NewRequest("POST", "/otlp/v1/metrics", strings.NewReader(body))
	w := httptest.NewRecorder()

	e.handleOTLPMetrics(w, req)

	if len(mp.ingested) != 1 {
		t.Fatalf("expected 1 metric (degraded, not dropped), got %d", len(mp.ingested))
	}
	if got, want := mp.ingested[0].host, "prod/host/web-stable-58cdf6849d-n5l2d"; got != want {
		t.Errorf("host = %q, want %q", got, want)
	}
}

// TestOTLPMetrics_NodeScopedTelemetry_NeverRegisteredAsWorkload is FBL-V2's
// other AC half: a resource carrying only k8s.node.name (real node-level
// metrics, no pod/service/workload attribute at all) must never become a
// fleet entry — it's dropped, not registered under the node's name.
func TestOTLPMetrics_NodeScopedTelemetry_NeverRegisteredAsWorkload(t *testing.T) {
	mp := &mockPipeline{}
	e := New(mp, nil, nil, nil, nil)

	body := `{"resourceMetrics":[{"resource":{"attributes":[
		{"key":"k8s.node.name","value":{"stringValue":"k3s-lab-ruptura-3a97-node-pool-1"}}
	]},"scopeMetrics":[{"metrics":[{"name":"node_cpu_percent","gauge":{"dataPoints":[{"asDouble":0.3,"timeUnixNano":"1000000000"}]}}]}]}]}`
	req := httptest.NewRequest("POST", "/otlp/v1/metrics", strings.NewReader(body))
	w := httptest.NewRecorder()

	e.handleOTLPMetrics(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if len(mp.ingested) != 0 {
		t.Errorf("expected 0 metrics (node telemetry must never become a fleet workload), got %d: %+v", len(mp.ingested), mp.ingested)
	}
}

// TestOTLPMetrics_BareHostTelemetry_StillRegistersAsHost is the negative
// case for the node-exclusion fix: a plain host.name attribute with no
// k8s.node.name at all is the documented non-K8s bare-metal/VM path
// (WorkloadRef's own doc comment) and must keep working exactly as before —
// this guards against over-broadly treating any "node"-like identity as
// excluded.
func TestOTLPMetrics_BareHostTelemetry_StillRegistersAsHost(t *testing.T) {
	mp := &mockPipeline{}
	e := New(mp, nil, nil, nil, nil)

	body := `{"resourceMetrics":[{"resource":{"attributes":[{"key":"host.name","value":{"stringValue":"h1"}}]},"scopeMetrics":[{"metrics":[{"name":"cpu","gauge":{"dataPoints":[{"asDouble":1.0,"timeUnixNano":"1000000000"}]}}]}]}]}`
	req := httptest.NewRequest("POST", "/otlp/v1/metrics", strings.NewReader(body))
	w := httptest.NewRecorder()

	e.handleOTLPMetrics(w, req)

	if len(mp.ingested) != 1 {
		t.Fatalf("expected 1 metric (bare-metal host telemetry), got %d", len(mp.ingested))
	}
	if got, want := mp.ingested[0].host, "default/host/h1"; got != want {
		t.Errorf("host = %q, want %q", got, want)
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

func TestOTLPLogs_LogPipeline(t *testing.T) {
	burstSink := &mockLogs{}
	pipelineSink := &mockLogs{}
	e := New(nil, burstSink, nil, nil, nil)
	e.SetLogPipeline(pipelineSink)

	// Two log records for the same service — both sinks should receive both lines.
	body := `{"resourceLogs":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"svc1"}}]},"scopeLogs":[{"logRecords":[{"body":{"stringValue":"error: disk full"},"timeUnixNano":"1000000000"},{"body":{"stringValue":"warn: high memory"},"timeUnixNano":"2000000000"}]}]}]}`
	req := httptest.NewRequest("POST", "/otlp/v1/logs", strings.NewReader(body))
	w := httptest.NewRecorder()

	e.handleOTLPLogs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if burstSink.lines != 2 {
		t.Errorf("burst sink: expected 2 lines, got %d", burstSink.lines)
	}
	if pipelineSink.lines != 2 {
		t.Errorf("log pipeline sink: expected 2 lines, got %d", pipelineSink.lines)
	}
}

func TestOTLPLogs_LogPipeline_WorkloadKey(t *testing.T) {
	type call struct{ service string; line string }
	var calls []call
	captureSink := &capturingLogSink{fn: func(svc string, line []byte, _ time.Time) {
		calls = append(calls, call{svc, string(line)})
	}}

	e := New(nil, nil, nil, nil, nil)
	e.SetLogPipeline(captureSink)

	// Resource has k8s.deployment.name and k8s.namespace.name → workload key used
	body := `{"resourceLogs":[{"resource":{"attributes":[{"key":"k8s.namespace.name","value":{"stringValue":"prod"}},{"key":"k8s.deployment.name","value":{"stringValue":"frontend"}},{"key":"service.name","value":{"stringValue":"frontend-svc"}}]},"scopeLogs":[{"logRecords":[{"body":{"stringValue":"started"},"timeUnixNano":"1000000000"}]}]}]}`
	req := httptest.NewRequest("POST", "/otlp/v1/logs", strings.NewReader(body))
	e.handleOTLPLogs(w(req), req)

	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	// Pipeline should receive workload key, not raw service name
	if calls[0].service != "prod/Deployment/frontend" {
		t.Errorf("expected workload key 'prod/Deployment/frontend', got %q", calls[0].service)
	}
}

type capturingLogSink struct {
	fn func(service string, line []byte, ts time.Time)
}

func (c *capturingLogSink) IngestLine(service string, line []byte, ts time.Time) {
	c.fn(service, line, ts)
}

func w(_ *http.Request) *httptest.ResponseRecorder { return httptest.NewRecorder() }

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
