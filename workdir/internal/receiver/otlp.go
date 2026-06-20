// Package receiver provides protocol-compatible ingestion endpoints for
// OpenTelemetry (OTLP), DogStatsD, Loki, and Elasticsearch — allowing Ruptura to
// act as a drop-in replacement for Grafana Cloud, Datadog, and ELK.
package receiver

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/benfradjselim/ruptura/internal/correlator"
	"github.com/benfradjselim/ruptura/pkg/models"
	"github.com/benfradjselim/ruptura/pkg/logger"
)

// TraceFusionSink accepts trace-derived rupture signals.
type TraceFusionSink interface {
	SetTraceR(host string, r float64, ts time.Time)
}

// spanWindow accumulates spans for a service and flushes when enough have
// arrived or the flush interval has elapsed.
type spanWindow struct {
	mu          sync.Mutex
	total       int
	errors      int
	durationSum int64 // nanoseconds
	lastFlush   time.Time
}

// update adds a span to the window. Returns (errorRate, avgLatencyMS, flush) when
// enough spans have accumulated (≥10) or 15 seconds have passed.
func (w *spanWindow) update(durationNS int64, isError bool, ts time.Time) (float64, float64, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.total++
	w.durationSum += durationNS
	if isError {
		w.errors++
	}
	shouldFlush := w.total >= 10 || ts.Sub(w.lastFlush) >= 15*time.Second
	if !shouldFlush {
		return 0, 0, false
	}
	errorRate := float64(w.errors) / float64(w.total)
	avgLatMS := float64(w.durationSum) / float64(w.total) / 1e6
	w.total, w.errors, w.durationSum = 0, 0, 0
	w.lastFlush = ts
	return errorRate, avgLatMS, true
}

// spanCacheEntry stores span-to-service mapping with an eviction timestamp.
type spanCacheEntry struct {
	service string
	addedAt time.Time
}

// MetricSink accepts parsed metrics from any receiver
type MetricSink interface {
	IngestMetric(m models.Metric)
}

// SpanSink accepts parsed spans from any receiver
type SpanSink interface {
	IngestSpan(s models.Span)
}

// LogSink accepts parsed log entries from any receiver
type LogSink interface {
	IngestLog(e models.LogEntry)
}

// OTLPReceiver handles OTLP/HTTP requests (traces, metrics, logs).
// It is a pure HTTP handler — mount it into the main router.
// Persistence is delegated to the sink interfaces to avoid double-writes.
type OTLPReceiver struct {
	metrics     MetricSink
	spans       SpanSink
	logs        LogSink
	hostname    string
	fusion      TraceFusionSink               // optional; nil = disabled
	topology    *correlator.TopologyBuilder   // optional; nil = disabled
	spanWindows sync.Map                      // key: service name → *spanWindow
	spanCache   sync.Map                      // key: spanID → spanCacheEntry
}

// NewOTLPReceiver creates a new OTLP HTTP receiver.
// fusion and topology are optional (may be nil).
func NewOTLPReceiver(metrics MetricSink, spans SpanSink, logs LogSink, hostname string, fusion TraceFusionSink, topology *correlator.TopologyBuilder) *OTLPReceiver {
	return &OTLPReceiver{
		metrics:  metrics,
		spans:    spans,
		logs:     logs,
		hostname: hostname,
		fusion:   fusion,
		topology: topology,
	}
}

// getOrCreateWindow returns the spanWindow for the given service, creating it if needed.
func (r *OTLPReceiver) getOrCreateWindow(service string) *spanWindow {
	if v, ok := r.spanWindows.Load(service); ok {
		return v.(*spanWindow)
	}
	w := &spanWindow{lastFlush: time.Now()}
	actual, _ := r.spanWindows.LoadOrStore(service, w)
	return actual.(*spanWindow)
}

const spanCacheTTL = 5 * time.Minute

// cacheSpan stores spanID → service in the span cache.
func (r *OTLPReceiver) cacheSpan(spanID, service string, ts time.Time) {
	r.spanCache.Store(spanID, spanCacheEntry{service: service, addedAt: ts})
}

// lookupSpanService returns the service for a spanID (from cache), or "".
func (r *OTLPReceiver) lookupSpanService(spanID string) string {
	if v, ok := r.spanCache.Load(spanID); ok {
		entry := v.(spanCacheEntry)
		if time.Since(entry.addedAt) < spanCacheTTL {
			return entry.service
		}
		r.spanCache.Delete(spanID)
	}
	return ""
}

// TraceHandler handles POST /otlp/v1/traces
// Note: persistence is handled exclusively by r.spans (SpanSink) to avoid double-writes.
func (r *OTLPReceiver) TraceHandler(w http.ResponseWriter, req *http.Request) {
	decoded, err := DecodeTracesRequest(req)
	if err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	otlpReq := *decoded

	count := 0
	for _, rs := range otlpReq.ResourceSpans {
		service := rs.Resource.GetAttr("service.name")
		host := rs.Resource.GetAttr("host.name")
		if host == "" {
			host = r.hostname
		}
		if service == "" {
			service = host
		}

		for _, ss := range rs.ScopeSpans {
			for _, s := range ss.Spans {
				span := parseOTLPSpan(s, service, host)
				if r.spans != nil {
					r.spans.IngestSpan(span) // Bus handles persistence
				}

				start := span.StartTime
				isError := s.Status.Code == 2

				// Cache spanID → service for parent lookup
				if span.SpanID != "" {
					r.cacheSpan(span.SpanID, service, start)
				}

				// Topology: resolve parent service and record edge
				if r.topology != nil {
					parentService := r.lookupSpanService(span.ParentID)
					r.topology.ObserveSpan(service, parentService, span.DurationNS, isError)
				}

				// Fusion: update per-service traceR
				if r.fusion != nil {
					window := r.getOrCreateWindow(service)
					if errRate, avgLatMS, ok := window.update(span.DurationNS, isError, start); ok {
						// Normalize: errRate [0,1] + latency normalized against 200ms baseline
						latScore := math.Min(avgLatMS/200.0, 3.0) / 3.0 // 0=fast, 1=very slow
						traceR := 0.6*errRate + 0.4*latScore
						r.fusion.SetTraceR(service, traceR, start)
					}
				}

				count++
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"partialSuccess":{}}`)
	logger.Default.Info("otlp traces ingested", "count", count)
}

// MetricsHandler handles POST /otlp/v1/metrics
// Note: persistence is handled exclusively by r.metrics (MetricSink) to avoid double-writes.
func (r *OTLPReceiver) MetricsHandler(w http.ResponseWriter, req *http.Request) {
	decoded, err := DecodeMetricsRequest(req)
	if err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	otlpReq := *decoded

	count := 0
	for _, rm := range otlpReq.ResourceMetrics {
		host := rm.Resource.GetAttr("host.name")
		if host == "" {
			host = r.hostname
		}

		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				dataPoints := collectDataPoints(m)
				for _, dp := range dataPoints {
					metric := models.Metric{
						Name:      sanitizeName(m.Name),
						Value:     dp.value,
						Timestamp: dp.ts,
						Host:      host,
						Labels:    dp.attrs,
					}
					if r.metrics != nil {
						r.metrics.IngestMetric(metric) // Bus handles persistence
					}
					count++
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"partialSuccess":{}}`)
	logger.Default.Info("otlp metrics ingested", "count", count)
}

// LogsHandler handles POST /otlp/v1/logs
// Note: persistence is handled exclusively by r.logs (LogSink) to avoid double-writes.
func (r *OTLPReceiver) LogsHandler(w http.ResponseWriter, req *http.Request) {
	decoded, err := DecodeLogsRequest(req)
	if err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	otlpReq := *decoded

	count := 0
	for _, rl := range otlpReq.ResourceLogs {
		service := rl.Resource.GetAttr("service.name")
		host := rl.Resource.GetAttr("host.name")
		if host == "" {
			host = r.hostname
		}
		if service == "" {
			service = host
		}

		for _, sl := range rl.ScopeLogs {
			for _, rec := range sl.LogRecords {
				entry := parseOTLPLog(rec, service, host)
				if r.logs != nil {
					r.logs.IngestLog(entry) // Bus handles persistence
				}
				count++
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"partialSuccess":{}}`)
	logger.Default.Info("otlp logs ingested", "count", count)
}

// --- helpers ---

func parseOTLPSpan(s models.OTLPSpan, service, host string) models.Span {
	start := parseNano(s.StartTimeUnixNano)
	end := parseNano(s.EndTimeUnixNano)
	dur := end.Sub(start).Nanoseconds()
	if dur < 0 {
		dur = 0
	}

	status := "unset"
	switch s.Status.Code {
	case 1:
		status = "ok"
	case 2:
		status = "error"
	}

	attrs := make(map[string]string, len(s.Attributes))
	for _, a := range s.Attributes {
		attrs[a.Key] = a.Value.GetString()
	}

	return models.Span{
		TraceID:    s.TraceID,
		SpanID:     s.SpanID,
		ParentID:   s.ParentSpanID,
		Service:    service,
		Operation:  s.Name,
		StartTime:  start,
		DurationNS: dur,
		Status:     status,
		Attributes: attrs,
		Host:       host,
	}
}

func parseOTLPLog(rec models.OTLPLogRecord, service, host string) models.LogEntry {
	ts := parseNano(rec.TimeUnixNano)
	if ts.IsZero() {
		ts = time.Now()
	}

	level := normalizeSeverity(rec.SeverityText, rec.SeverityNumber)
	msg := rec.Body.GetString()

	labels := make(map[string]string, len(rec.Attributes))
	for _, a := range rec.Attributes {
		labels[a.Key] = a.Value.GetString()
	}

	return models.LogEntry{
		Timestamp: ts,
		Level:     level,
		Message:   msg,
		Service:   service,
		Host:      host,
		Labels:    labels,
		TraceID:   rec.TraceID,
		SpanID:    rec.SpanID,
		Source:    "otlp",
	}
}

type dataPoint struct {
	value float64
	ts    time.Time
	attrs map[string]string
}

func collectDataPoints(m models.OTLPMetric) []dataPoint {
	var dps []dataPoint
	var rawDps []models.OTLPNumberDataPoint

	if m.Gauge != nil {
		rawDps = m.Gauge.DataPoints
	} else if m.Sum != nil {
		rawDps = m.Sum.DataPoints
	}

	for _, dp := range rawDps {
		val := 0.0
		if dp.AsDouble != nil {
			val = *dp.AsDouble
		} else if dp.AsInt != nil {
			val = float64(*dp.AsInt)
		}
		ts := parseNano(dp.TimeUnixNano)
		if ts.IsZero() {
			ts = time.Now()
		}
		attrs := make(map[string]string, len(dp.Attributes))
		for _, a := range dp.Attributes {
			attrs[a.Key] = a.Value.GetString()
		}
		dps = append(dps, dataPoint{value: val, ts: ts, attrs: attrs})
	}
	return dps
}

// parseNano parses a Unix nanosecond string (OTLP format)
func parseNano(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	ns, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(0, ns)
}

// normalizeSeverity converts OTLP severity to a short level string
func normalizeSeverity(text string, number int) string {
	if text != "" {
		t := strings.ToLower(text)
		switch {
		case strings.HasPrefix(t, "err") || strings.HasPrefix(t, "fatal") || strings.HasPrefix(t, "crit"):
			return "error"
		case strings.HasPrefix(t, "warn"):
			return "warn"
		case strings.HasPrefix(t, "debug") || strings.HasPrefix(t, "trace"):
			return "debug"
		default:
			return "info"
		}
	}
	// OTLP severity numbers: 1-4=trace,5-8=debug,9-12=info,13-16=warn,17-20=error,21-24=fatal
	switch {
	case number >= 17:
		return "error"
	case number >= 13:
		return "warn"
	case number >= 9:
		return "info"
	case number > 0:
		return "debug"
	default:
		return "info"
	}
}

// sanitizeName replaces illegal metric name characters
func sanitizeName(name string) string {
	return strings.NewReplacer(".", "_", "-", "_", "/", "_", " ", "_").Replace(name)
}
