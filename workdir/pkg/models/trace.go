package models

import (
	"fmt"
	"time"
)

// LogEntry is a single structured log record (Loki/ELK/OTEL compatible)
type LogEntry struct {
	Timestamp time.Time         `json:"timestamp"`
	Level     string            `json:"level"`              // info, warn, error, debug
	Message   string            `json:"message"`
	Service   string            `json:"service,omitempty"`
	Host      string            `json:"host,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	TraceID   string            `json:"trace_id,omitempty"`
	SpanID    string            `json:"span_id,omitempty"`
	Source    string            `json:"source,omitempty"` // "loki", "otlp", "elasticsearch", "ohe"
}

// Span is a distributed trace span (OTLP/APM compatible)
type Span struct {
	TraceID    string            `json:"trace_id"`
	SpanID     string            `json:"span_id"`
	ParentID   string            `json:"parent_id,omitempty"`
	Service    string            `json:"service"`
	Operation  string            `json:"operation"`
	StartTime  time.Time         `json:"start_time"`
	DurationNS int64             `json:"duration_ns"`
	Status     string            `json:"status"` // "ok", "error", "unset"
	Attributes map[string]string `json:"attributes,omitempty"`
	Host       string            `json:"host,omitempty"`
}

// ServiceEdge represents a directional call between two services
type ServiceEdge struct {
	From     string  `json:"from"`
	To       string  `json:"to"`
	Calls    int64   `json:"calls"`
	Errors   int64   `json:"errors"`
	AvgLatMS float64 `json:"avg_lat_ms"`
}

// TopologyGraph is the live service dependency map derived from traces
type TopologyGraph struct {
	Timestamp time.Time     `json:"timestamp"`
	Nodes     []string      `json:"nodes"`
	Edges     []ServiceEdge `json:"edges"`
}

// StatsDMetric is a parsed DogStatsD / StatsD metric
type StatsDMetric struct {
	Name      string            `json:"name"`
	Value     float64           `json:"value"`
	Type      string            `json:"type"` // "c" counter, "g" gauge, "ms" timer, "h" histogram, "s" set
	SampleRate float64          `json:"sample_rate"`
	Tags      map[string]string `json:"tags,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// OTLPAttribute is a key-value attribute in OTLP JSON format
type OTLPAttribute struct {
	Key   string         `json:"key"`
	Value OTLPAnyValue   `json:"value"`
}

// OTLPAnyValue holds OTLP scalar values
type OTLPAnyValue struct {
	StringValue *string  `json:"stringValue,omitempty"`
	IntValue    *int64   `json:"intValue,omitempty"`
	DoubleValue *float64 `json:"doubleValue,omitempty"`
	BoolValue   *bool    `json:"boolValue,omitempty"`
}

// GetString returns the string representation of any OTLP value
func (v OTLPAnyValue) GetString() string {
	if v.StringValue != nil {
		return *v.StringValue
	}
	if v.IntValue != nil {
		return fmt.Sprintf("%d", *v.IntValue)
	}
	if v.DoubleValue != nil {
		return fmt.Sprintf("%g", *v.DoubleValue)
	}
	if v.BoolValue != nil {
		if *v.BoolValue {
			return "true"
		}
		return "false"
	}
	return ""
}

// OTLPResource holds resource attributes
type OTLPResource struct {
	Attributes []OTLPAttribute `json:"attributes"`
}

// GetAttr returns a named attribute value from a resource
func (r OTLPResource) GetAttr(key string) string {
	for _, a := range r.Attributes {
		if a.Key == key {
			return a.Value.GetString()
		}
	}
	return ""
}

// --- OTLP Trace JSON structures ---

type OTLPTraceRequest struct {
	ResourceSpans []OTLPResourceSpans `json:"resourceSpans"`
}

type OTLPResourceSpans struct {
	Resource   OTLPResource    `json:"resource"`
	ScopeSpans []OTLPScopeSpans `json:"scopeSpans"`
}

type OTLPScopeSpans struct {
	Spans []OTLPSpan `json:"spans"`
}

type OTLPSpan struct {
	TraceID           string          `json:"traceId"`
	SpanID            string          `json:"spanId"`
	ParentSpanID      string          `json:"parentSpanId,omitempty"`
	Name              string          `json:"name"`
	StartTimeUnixNano string          `json:"startTimeUnixNano"`
	EndTimeUnixNano   string          `json:"endTimeUnixNano"`
	Status            OTLPSpanStatus  `json:"status"`
	Attributes        []OTLPAttribute `json:"attributes,omitempty"`
}

type OTLPSpanStatus struct {
	Code    int    `json:"code"`    // 0=unset, 1=ok, 2=error
	Message string `json:"message,omitempty"`
}

// --- OTLP Metrics JSON structures ---

type OTLPMetricsRequest struct {
	ResourceMetrics []OTLPResourceMetrics `json:"resourceMetrics"`
}

type OTLPResourceMetrics struct {
	Resource     OTLPResource      `json:"resource"`
	ScopeMetrics []OTLPScopeMetrics `json:"scopeMetrics"`
}

type OTLPScopeMetrics struct {
	Metrics []OTLPMetric `json:"metrics"`
}

type OTLPMetric struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Gauge       *OTLPGauge      `json:"gauge,omitempty"`
	Sum         *OTLPSum        `json:"sum,omitempty"`
}

type OTLPGauge struct {
	DataPoints []OTLPNumberDataPoint `json:"dataPoints"`
}

type OTLPSum struct {
	DataPoints []OTLPNumberDataPoint `json:"dataPoints"`
}

type OTLPNumberDataPoint struct {
	Attributes     []OTLPAttribute `json:"attributes,omitempty"`
	TimeUnixNano   string          `json:"timeUnixNano"`
	AsDouble       *float64        `json:"asDouble,omitempty"`
	AsInt          *int64          `json:"asInt,omitempty"`
}

// --- OTLP Logs JSON structures ---

type OTLPLogsRequest struct {
	ResourceLogs []OTLPResourceLogs `json:"resourceLogs"`
}

type OTLPResourceLogs struct {
	Resource  OTLPResource    `json:"resource"`
	ScopeLogs []OTLPScopeLogs `json:"scopeLogs"`
}

type OTLPScopeLogs struct {
	LogRecords []OTLPLogRecord `json:"logRecords"`
}

type OTLPLogRecord struct {
	TimeUnixNano         string          `json:"timeUnixNano"`
	ObservedTimeUnixNano string          `json:"observedTimeUnixNano,omitempty"`
	SeverityNumber       int             `json:"severityNumber,omitempty"`
	SeverityText         string          `json:"severityText,omitempty"`
	Body                 OTLPAnyValue    `json:"body"`
	Attributes           []OTLPAttribute `json:"attributes,omitempty"`
	TraceID              string          `json:"traceId,omitempty"`
	SpanID               string          `json:"spanId,omitempty"`
}

// --- Loki push format ---

type LokiPushRequest struct {
	Streams []LokiStream `json:"streams"`
}

type LokiStream struct {
	Stream map[string]string `json:"stream"` // label set
	Values [][]string        `json:"values"` // [["timestamp_ns", "log_line"], ...]
}

// --- Elasticsearch bulk format ---

type ESBulkAction struct {
	Index  *ESBulkIndex  `json:"index,omitempty"`
	Create *ESBulkIndex  `json:"create,omitempty"`
}

type ESBulkIndex struct {
	Index string `json:"_index,omitempty"`
	ID    string `json:"_id,omitempty"`
}
