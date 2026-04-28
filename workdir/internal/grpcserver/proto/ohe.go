// Package proto defines the wire types for the OHE agent gRPC protocol.
// Messages are serialized with JSON (using the ohe-json gRPC codec) to avoid
// protoc code-generation while preserving binary HTTP/2 framing.
package proto

// MetricSample is a single metric observation from an agent.
type MetricSample struct {
	Host        string  `json:"host"`
	Name        string  `json:"name"`
	Value       float64 `json:"value"`
	TimestampMs int64   `json:"timestamp_ms"`
}

// LogEntry is a structured log line from an agent.
type LogEntry struct {
	Host        string `json:"host"`
	Service     string `json:"service"`
	Level       string `json:"level"`
	Body        string `json:"body"`
	TimestampMs int64  `json:"timestamp_ms"`
}

// IngestRequest carries metrics and/or logs in a single agent push.
type IngestRequest struct {
	Metrics []*MetricSample `json:"metrics,omitempty"`
	Logs    []*LogEntry     `json:"logs,omitempty"`
}

// IngestResponse is the server acknowledgement for an IngestRequest.
type IngestResponse struct {
	MetricsWritten int32  `json:"metrics_written"`
	LogsWritten    int32  `json:"logs_written"`
	Error          string `json:"error,omitempty"`
}
