package api

// otlp_handlers.go — OTLP HTTP protocol handlers.
// These delegate to the receiver.OTLPReceiver for parsing and routing.
// Mounted at /otlp/v1/{traces,metrics,logs}.

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/benfradjselim/kairo-core/pkg/models"
	"github.com/benfradjselim/kairo-core/pkg/logger"
)

const otlpMaxBodyBytes = 32 << 20 // 32 MB

// OTLPTraceHandler handles POST /otlp/v1/traces
func (h *Handlers) OTLPTraceHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, otlpMaxBodyBytes)
	var req models.OTLPTraceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	count := 0
	for _, rs := range req.ResourceSpans {
		service := rs.Resource.GetAttr("service.name")
		host := rs.Resource.GetAttr("host.name")
		if host == "" {
			host = h.hostname
		}
		if service == "" {
			service = host
		}

		for _, ss := range rs.ScopeSpans {
			for _, s := range ss.Spans {
				span := otlpToSpan(s, service, host)
				h.topology.IngestSpan(span)
				if err := h.store.SaveSpan(span, span.TraceID, span.SpanID); err != nil {
					logger.Default.ErrorCtx(r.Context(), "otlp traces save error", "err", err)
				}
				count++
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"partialSuccess":{}}`)) //nolint:errcheck
	logger.Default.InfoCtx(r.Context(), "otlp traces ingested", "count", count)
}

// OTLPMetricsHandler handles POST /otlp/v1/metrics
func (h *Handlers) OTLPMetricsHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, otlpMaxBodyBytes)
	var req models.OTLPMetricsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	count := 0
	for _, rm := range req.ResourceMetrics {
		host := rm.Resource.GetAttr("host.name")
		if host == "" {
			host = h.hostname
		}

		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				dps := otlpDataPoints(m)
				for _, dp := range dps {
					name := otlpSanitize(m.Name)
					if err := h.store.SaveMetric(host, name, dp.value, dp.ts); err != nil {
						logger.Default.ErrorCtx(r.Context(), "otlp metrics save error", "err", err)
					}
					count++
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"partialSuccess":{}}`)) //nolint:errcheck
	logger.Default.InfoCtx(r.Context(), "otlp metrics ingested", "count", count)
}

// OTLPLogsHandler handles POST /otlp/v1/logs
func (h *Handlers) OTLPLogsHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, otlpMaxBodyBytes)
	var req models.OTLPLogsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	count := 0
	for _, rl := range req.ResourceLogs {
		service := rl.Resource.GetAttr("service.name")
		host := rl.Resource.GetAttr("host.name")
		if host == "" {
			host = h.hostname
		}
		if service == "" {
			service = host
		}

		for _, sl := range rl.ScopeLogs {
			for _, rec := range sl.LogRecords {
				entry := otlpToLogEntry(rec, service, host)
				if err := h.store.SaveLog(service, entry, entry.Timestamp); err != nil {
					logger.Default.ErrorCtx(r.Context(), "otlp logs save error", "err", err)
				}
				count++
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"partialSuccess":{}}`)) //nolint:errcheck
	logger.Default.InfoCtx(r.Context(), "otlp logs ingested", "count", count)
}

// --- conversion helpers ---

func otlpToSpan(s models.OTLPSpan, service, host string) models.Span {
	start := otlpParseNano(s.StartTimeUnixNano)
	end := otlpParseNano(s.EndTimeUnixNano)
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

func otlpToLogEntry(rec models.OTLPLogRecord, service, host string) models.LogEntry {
	ts := otlpParseNano(rec.TimeUnixNano)
	if ts.IsZero() {
		ts = time.Now()
	}

	labels := make(map[string]string, len(rec.Attributes))
	for _, a := range rec.Attributes {
		labels[a.Key] = a.Value.GetString()
	}

	return models.LogEntry{
		Timestamp: ts,
		Level:     otlpSeverityToLevel(rec.SeverityText, rec.SeverityNumber),
		Message:   rec.Body.GetString(),
		Service:   service,
		Host:      host,
		Labels:    labels,
		TraceID:   rec.TraceID,
		SpanID:    rec.SpanID,
		Source:    "otlp",
	}
}

type otlpDP struct {
	value float64
	ts    time.Time
}

func otlpDataPoints(m models.OTLPMetric) []otlpDP {
	var raw []models.OTLPNumberDataPoint
	if m.Gauge != nil {
		raw = m.Gauge.DataPoints
	} else if m.Sum != nil {
		raw = m.Sum.DataPoints
	}

	dps := make([]otlpDP, 0, len(raw))
	for _, dp := range raw {
		val := 0.0
		if dp.AsDouble != nil {
			val = *dp.AsDouble
		} else if dp.AsInt != nil {
			val = float64(*dp.AsInt)
		}
		ts := otlpParseNano(dp.TimeUnixNano)
		if ts.IsZero() {
			ts = time.Now()
		}
		dps = append(dps, otlpDP{value: val, ts: ts})
	}
	return dps
}

func otlpParseNano(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	ns, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(0, ns)
}

func otlpSeverityToLevel(text string, number int) string {
	if text != "" {
		switch {
		case len(text) >= 3 && (text[:3] == "ERR" || text[:3] == "err" || text[:3] == "FAT" || text[:3] == "fat" || text[:3] == "CRI" || text[:3] == "cri"):
			return "error"
		case len(text) >= 4 && (text[:4] == "WARN" || text[:4] == "warn"):
			return "warn"
		case len(text) >= 5 && (text[:5] == "DEBUG" || text[:5] == "debug" || text[:5] == "TRACE" || text[:5] == "trace"):
			return "debug"
		default:
			return "info"
		}
	}
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

var otlpReplacer = strings.NewReplacer(".", "_", "-", "_", "/", "_", " ", "_")

func otlpSanitize(name string) string {
	return otlpReplacer.Replace(name)
}
