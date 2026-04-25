// Package receiver provides protocol-compatible ingestion endpoints for
// OpenTelemetry (OTLP), DogStatsD, Loki, and Elasticsearch — allowing OHE to
// act as a drop-in replacement for Grafana Cloud, Datadog, and ELK.
package receiver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/benfradjselim/kairo-core/pkg/models"
	"github.com/benfradjselim/kairo-core/pkg/logger"
)

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
	metrics  MetricSink
	spans    SpanSink
	logs     LogSink
	hostname string
}

// NewOTLPReceiver creates a new OTLP HTTP receiver
func NewOTLPReceiver(metrics MetricSink, spans SpanSink, logs LogSink, hostname string) *OTLPReceiver {
	return &OTLPReceiver{metrics: metrics, spans: spans, logs: logs, hostname: hostname}
}

// TraceHandler handles POST /otlp/v1/traces
// Note: persistence is handled exclusively by r.spans (SpanSink) to avoid double-writes.
func (r *OTLPReceiver) TraceHandler(w http.ResponseWriter, req *http.Request) {
	req.Body = http.MaxBytesReader(w, req.Body, 32<<20)
	var otlpReq models.OTLPTraceRequest
	if err := json.NewDecoder(req.Body).Decode(&otlpReq); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

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
	req.Body = http.MaxBytesReader(w, req.Body, 32<<20)
	var otlpReq models.OTLPMetricsRequest
	if err := json.NewDecoder(req.Body).Decode(&otlpReq); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

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
	req.Body = http.MaxBytesReader(w, req.Body, 32<<20)
	var otlpReq models.OTLPLogsRequest
	if err := json.NewDecoder(req.Body).Decode(&otlpReq); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

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
