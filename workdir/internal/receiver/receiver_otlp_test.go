package receiver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/storage"
	"github.com/benfradjselim/ruptura/pkg/models"
)

// --- mock sinks ---

type mockMetricSink struct{ metrics []models.Metric }

func (m *mockMetricSink) IngestMetric(metric models.Metric) { m.metrics = append(m.metrics, metric) }

type mockSpanSink struct{ spans []models.Span }

func (m *mockSpanSink) IngestSpan(s models.Span) { m.spans = append(m.spans, s) }

type mockLogSink struct{ logs []models.LogEntry }

func (m *mockLogSink) IngestLog(e models.LogEntry) { m.logs = append(m.logs, e) }

// --- Bus tests ---

func openTestStore(t *testing.T) *storage.Store {
	t.Helper()
	s, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestBus_IngestMetric(t *testing.T) {
	s := openTestStore(t)
	bus := NewBus(s, nil)
	bus.IngestMetric(models.Metric{
		Name: "cpu_percent", Value: 42.5, Host: "h1", Timestamp: time.Now(),
	})
	// No panic = pass; store persistence is verified by storage tests
}

func TestBus_IngestSpan_NilTopology(t *testing.T) {
	s := openTestStore(t)
	bus := NewBus(s, nil)
	bus.IngestSpan(models.Span{
		TraceID: "t1", SpanID: "s1", Service: "svc",
		Operation: "GET /", Status: "ok", StartTime: time.Now(),
	})
}

func TestBus_IngestLog_WithService(t *testing.T) {
	s := openTestStore(t)
	bus := NewBus(s, nil)
	bus.IngestLog(models.LogEntry{
		Service: "api", Message: "started", Level: "info", Timestamp: time.Now(),
	})
}

func TestBus_IngestLog_FallbackToHost(t *testing.T) {
	s := openTestStore(t)
	bus := NewBus(s, nil)
	bus.IngestLog(models.LogEntry{Host: "h1", Message: "msg", Timestamp: time.Now()})
}

func TestBus_IngestLog_FallbackToUnknown(t *testing.T) {
	s := openTestStore(t)
	bus := NewBus(s, nil)
	bus.IngestLog(models.LogEntry{Message: "no service or host", Timestamp: time.Now()})
}

// --- OTLPReceiver handler tests ---

func newOTLPReceiver() (*OTLPReceiver, *mockMetricSink, *mockSpanSink, *mockLogSink) {
	ms := &mockMetricSink{}
	ss := &mockSpanSink{}
	ls := &mockLogSink{}
	return NewOTLPReceiver(ms, ss, ls, "testhost"), ms, ss, ls
}

func postJSON(t *testing.T, handler http.HandlerFunc, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)
	return w
}

func TestOTLPReceiver_TraceHandler(t *testing.T) {
	recv, _, ss, _ := newOTLPReceiver()
	strPtr := func(s string) *string { return &s }

	traceReq := models.OTLPTraceRequest{
		ResourceSpans: []models.OTLPResourceSpans{{
			Resource: models.OTLPResource{Attributes: []models.OTLPAttribute{
				{Key: "service.name", Value: models.OTLPAnyValue{StringValue: strPtr("frontend")}},
				{Key: "host.name", Value: models.OTLPAnyValue{StringValue: strPtr("web-01")}},
			}},
			ScopeSpans: []models.OTLPScopeSpans{{
				Spans: []models.OTLPSpan{{
					TraceID: "trace1", SpanID: "span1",
					Name:              "GET /api",
					StartTimeUnixNano: "1000000000",
					EndTimeUnixNano:   "2000000000",
					Status:            models.OTLPSpanStatus{Code: 1},
					Attributes: []models.OTLPAttribute{
						{Key: "http.method", Value: models.OTLPAnyValue{StringValue: strPtr("GET")}},
					},
				}},
			}},
		}},
	}

	w := postJSON(t, recv.TraceHandler, traceReq)
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	if len(ss.spans) != 1 {
		t.Fatalf("want 1 span, got %d", len(ss.spans))
	}
	span := ss.spans[0]
	if span.Service != "frontend" {
		t.Errorf("service: got %q", span.Service)
	}
	if span.Status != "ok" {
		t.Errorf("status: got %q", span.Status)
	}
	if span.DurationNS != 1_000_000_000 {
		t.Errorf("duration: got %d", span.DurationNS)
	}
}

func TestOTLPReceiver_TraceHandler_DefaultHost(t *testing.T) {
	recv, _, ss, _ := newOTLPReceiver()
	traceReq := models.OTLPTraceRequest{
		ResourceSpans: []models.OTLPResourceSpans{{
			ScopeSpans: []models.OTLPScopeSpans{{
				Spans: []models.OTLPSpan{{
					TraceID: "t", SpanID: "s", Name: "op",
					StartTimeUnixNano: "0", EndTimeUnixNano: "0",
				}},
			}},
		}},
	}
	w := postJSON(t, recv.TraceHandler, traceReq)
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
	if len(ss.spans) != 1 || ss.spans[0].Host != "testhost" {
		t.Errorf("expected fallback host 'testhost', got %q", ss.spans[0].Host)
	}
}

func TestOTLPReceiver_TraceHandler_BadJSON(t *testing.T) {
	recv, _, _, _ := newOTLPReceiver()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()
	recv.TraceHandler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestOTLPReceiver_TraceHandler_ErrorStatus(t *testing.T) {
	recv, _, ss, _ := newOTLPReceiver()
	traceReq := models.OTLPTraceRequest{
		ResourceSpans: []models.OTLPResourceSpans{{
			ScopeSpans: []models.OTLPScopeSpans{{
				Spans: []models.OTLPSpan{{
					TraceID: "t", SpanID: "s", Name: "op",
					StartTimeUnixNano: "500000000", EndTimeUnixNano: "400000000", // end before start
					Status: models.OTLPSpanStatus{Code: 2},
				}},
			}},
		}},
	}
	postJSON(t, recv.TraceHandler, traceReq)
	if len(ss.spans) != 1 || ss.spans[0].Status != "error" {
		t.Errorf("expected error status")
	}
	if ss.spans[0].DurationNS != 0 {
		t.Errorf("negative duration should be clamped to 0")
	}
}

func TestOTLPReceiver_MetricsHandler_Gauge(t *testing.T) {
	recv, ms, _, _ := newOTLPReceiver()
	strPtr := func(s string) *string { return &s }
	dblPtr := func(d float64) *float64 { return &d }

	metricsReq := models.OTLPMetricsRequest{
		ResourceMetrics: []models.OTLPResourceMetrics{{
			Resource: models.OTLPResource{Attributes: []models.OTLPAttribute{
				{Key: "host.name", Value: models.OTLPAnyValue{StringValue: strPtr("srv1")}},
			}},
			ScopeMetrics: []models.OTLPScopeMetrics{{
				Metrics: []models.OTLPMetric{{
					Name: "cpu.usage",
					Gauge: &models.OTLPGauge{DataPoints: []models.OTLPNumberDataPoint{
						{TimeUnixNano: "1000000000", AsDouble: dblPtr(75.5)},
					}},
				}},
			}},
		}},
	}

	w := postJSON(t, recv.MetricsHandler, metricsReq)
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
	if len(ms.metrics) != 1 {
		t.Fatalf("want 1 metric, got %d", len(ms.metrics))
	}
	m := ms.metrics[0]
	if m.Name != "cpu_usage" {
		t.Errorf("name: got %q", m.Name)
	}
	if m.Value != 75.5 {
		t.Errorf("value: got %g", m.Value)
	}
	if m.Host != "srv1" {
		t.Errorf("host: got %q", m.Host)
	}
}

func TestOTLPReceiver_MetricsHandler_SumWithIntDataPoint(t *testing.T) {
	recv, ms, _, _ := newOTLPReceiver()
	intPtr := func(i int64) *int64 { return &i }

	metricsReq := models.OTLPMetricsRequest{
		ResourceMetrics: []models.OTLPResourceMetrics{{
			ScopeMetrics: []models.OTLPScopeMetrics{{
				Metrics: []models.OTLPMetric{{
					Name: "requests.total",
					Sum: &models.OTLPSum{DataPoints: []models.OTLPNumberDataPoint{
						{TimeUnixNano: "2000000000", AsInt: intPtr(1000)},
					}},
				}},
			}},
		}},
	}

	postJSON(t, recv.MetricsHandler, metricsReq)
	if len(ms.metrics) != 1 || ms.metrics[0].Value != 1000 {
		t.Errorf("int data point: got %+v", ms.metrics)
	}
}

func TestOTLPReceiver_MetricsHandler_BadJSON(t *testing.T) {
	recv, _, _, _ := newOTLPReceiver()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("{bad}"))
	w := httptest.NewRecorder()
	recv.MetricsHandler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestOTLPReceiver_LogsHandler(t *testing.T) {
	recv, _, _, ls := newOTLPReceiver()
	strPtr := func(s string) *string { return &s }
	ts := time.Now().UnixNano()

	logsReq := models.OTLPLogsRequest{
		ResourceLogs: []models.OTLPResourceLogs{{
			Resource: models.OTLPResource{Attributes: []models.OTLPAttribute{
				{Key: "service.name", Value: models.OTLPAnyValue{StringValue: strPtr("backend")}},
				{Key: "host.name", Value: models.OTLPAnyValue{StringValue: strPtr("app-01")}},
			}},
			ScopeLogs: []models.OTLPScopeLogs{{
				LogRecords: []models.OTLPLogRecord{{
					TimeUnixNano:   fmt.Sprintf("%d", ts),
					SeverityText:   "ERROR",
					Body:           models.OTLPAnyValue{StringValue: strPtr("something failed")},
					TraceID:        "trace-abc",
					SpanID:         "span-xyz",
				}},
			}},
		}},
	}

	w := postJSON(t, recv.LogsHandler, logsReq)
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
	if len(ls.logs) != 1 {
		t.Fatalf("want 1 log, got %d", len(ls.logs))
	}
	log := ls.logs[0]
	if log.Level != "error" {
		t.Errorf("level: got %q", log.Level)
	}
	if log.Service != "backend" {
		t.Errorf("service: got %q", log.Service)
	}
	if log.Message != "something failed" {
		t.Errorf("message: got %q", log.Message)
	}
	if log.Source != "otlp" {
		t.Errorf("source: got %q", log.Source)
	}
}

func TestOTLPReceiver_LogsHandler_BadJSON(t *testing.T) {
	recv, _, _, _ := newOTLPReceiver()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("x"))
	w := httptest.NewRecorder()
	recv.LogsHandler(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestOTLPReceiver_LogsHandler_ZeroTimestamp(t *testing.T) {
	recv, _, _, ls := newOTLPReceiver()
	strPtr := func(s string) *string { return &s }

	logsReq := models.OTLPLogsRequest{
		ResourceLogs: []models.OTLPResourceLogs{{
			ScopeLogs: []models.OTLPScopeLogs{{
				LogRecords: []models.OTLPLogRecord{{
					TimeUnixNano: "0",
					Body:         models.OTLPAnyValue{StringValue: strPtr("fallback ts")},
				}},
			}},
		}},
	}
	postJSON(t, recv.LogsHandler, logsReq)
	if len(ls.logs) != 1 {
		t.Fatal("expected 1 log")
	}
	if ls.logs[0].Timestamp.IsZero() {
		t.Error("zero timestamp should fall back to time.Now()")
	}
}

// --- normalizeSeverity tests ---

func TestNormalizeSeverity_Text(t *testing.T) {
	cases := []struct{ text string; want string }{
		{"ERROR", "error"}, {"error", "error"}, {"FATAL", "error"}, {"critical", "error"},
		{"WARN", "warn"}, {"warning", "warn"},
		{"DEBUG", "debug"}, {"trace", "debug"},
		{"INFO", "info"}, {"info", "info"}, {"notice", "info"},
	}
	for _, c := range cases {
		got := normalizeSeverity(c.text, 0)
		if got != c.want {
			t.Errorf("normalizeSeverity(%q, 0) = %q, want %q", c.text, got, c.want)
		}
	}
}

func TestNormalizeSeverity_Number(t *testing.T) {
	cases := []struct{ num int; want string }{
		{0, "info"}, {1, "debug"}, {5, "debug"}, {9, "info"}, {12, "info"},
		{13, "warn"}, {16, "warn"}, {17, "error"}, {21, "error"},
	}
	for _, c := range cases {
		got := normalizeSeverity("", c.num)
		if got != c.want {
			t.Errorf("normalizeSeverity(\"\", %d) = %q, want %q", c.num, got, c.want)
		}
	}
}

// --- parseNano tests ---

func TestParseNano_Valid(t *testing.T) {
	ts := time.Now().Truncate(time.Nanosecond)
	got := parseNano(fmt.Sprintf("%d", ts.UnixNano()))
	if !got.Equal(ts) {
		t.Errorf("parseNano: got %v, want %v", got, ts)
	}
}

func TestParseNano_Empty(t *testing.T) {
	if !parseNano("").IsZero() {
		t.Error("empty string should return zero time")
	}
}

func TestParseNano_Invalid(t *testing.T) {
	if !parseNano("not-a-number").IsZero() {
		t.Error("invalid string should return zero time")
	}
}

// --- collectDataPoints tests ---

func TestCollectDataPoints_Empty(t *testing.T) {
	dps := collectDataPoints(models.OTLPMetric{Name: "x"})
	if len(dps) != 0 {
		t.Errorf("expected 0 data points for metric with no gauge/sum")
	}
}

func TestCollectDataPoints_GaugeDouble(t *testing.T) {
	v := 3.14
	m := models.OTLPMetric{
		Name:  "pi",
		Gauge: &models.OTLPGauge{DataPoints: []models.OTLPNumberDataPoint{{AsDouble: &v, TimeUnixNano: "1000000000"}}},
	}
	dps := collectDataPoints(m)
	if len(dps) != 1 || dps[0].value != 3.14 {
		t.Errorf("expected 3.14, got %+v", dps)
	}
}

func TestCollectDataPoints_SumInt(t *testing.T) {
	var i int64 = 99
	m := models.OTLPMetric{
		Name: "count",
		Sum:  &models.OTLPSum{DataPoints: []models.OTLPNumberDataPoint{{AsInt: &i, TimeUnixNano: "0"}}},
	}
	dps := collectDataPoints(m)
	if len(dps) != 1 || dps[0].value != 99 {
		t.Errorf("expected 99, got %+v", dps)
	}
}

// --- DogStatsDReceiver emit tests (via a nil-sink path) ---

func TestDogStatsDReceiver_EmitNilSink(t *testing.T) {
	s := openTestStore(t)
	r := NewDogStatsDReceiver(":0", s, nil, "h1")
	sm, err := parseStatsDLine("hits:5|c|@0.5")
	if err != nil {
		t.Fatal(err)
	}
	// Should not panic with nil MetricSink
	r.emit(sm)
}

func TestDogStatsDReceiver_EmitSampleRateCorrection(t *testing.T) {
	s := openTestStore(t)
	ms := &mockMetricSink{}
	r := NewDogStatsDReceiver(":0", s, ms, "h1")
	sm, _ := parseStatsDLine("hits:4|c|@0.25")
	r.emit(sm)
	if len(ms.metrics) != 1 || ms.metrics[0].Value != 16 {
		t.Errorf("sample rate correction: want 16, got %+v", ms.metrics)
	}
}

func TestDogStatsDReceiver_EmitGauge_NoCorrection(t *testing.T) {
	s := openTestStore(t)
	ms := &mockMetricSink{}
	r := NewDogStatsDReceiver(":0", s, ms, "h1")
	sm, _ := parseStatsDLine("cpu:75|g|@0.5")
	r.emit(sm)
	// Gauge should NOT have sample rate correction
	if len(ms.metrics) != 1 || ms.metrics[0].Value != 75 {
		t.Errorf("gauge should not apply sample rate correction: %+v", ms.metrics)
	}
}
