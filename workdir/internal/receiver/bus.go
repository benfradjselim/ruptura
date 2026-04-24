package receiver

import (

	"github.com/benfradjselim/kairo-core/internal/analyzer"
	"github.com/benfradjselim/kairo-core/internal/storage"
	"github.com/benfradjselim/kairo-core/pkg/models"
	"github.com/benfradjselim/kairo-core/pkg/logger"
)

// Bus wires ingested data from all receivers into the OHE pipeline.
// It implements MetricSink, SpanSink, and LogSink.
type Bus struct {
	store    *storage.Store
	topology *analyzer.TopologyAnalyzer
}

// NewBus creates a receiver bus wired to the given store and topology analyzer
func NewBus(store *storage.Store, topology *analyzer.TopologyAnalyzer) *Bus {
	return &Bus{store: store, topology: topology}
}

// IngestMetric satisfies MetricSink — forwards metric to the store
func (b *Bus) IngestMetric(m models.Metric) {
	if err := b.store.SaveMetric(m.Host, m.Name, m.Value, m.Timestamp); err != nil {
		logger.Default.Error("bus metric save error", "host", m.Host, "metric", m.Name, "err", err)
	}
}

// IngestSpan satisfies SpanSink — forwards span to topology analyzer
func (b *Bus) IngestSpan(s models.Span) {
	if b.topology != nil {
		b.topology.IngestSpan(s)
	}
	if err := b.store.SaveSpan(s, s.TraceID, s.SpanID); err != nil {
		logger.Default.Error("bus span save error", "trace_id", s.TraceID, "span_id", s.SpanID, "err", err)
	}
}

// IngestLog satisfies LogSink — stores log entry
func (b *Bus) IngestLog(e models.LogEntry) {
	service := e.Service
	if service == "" {
		service = e.Host
	}
	if service == "" {
		service = "unknown"
	}
	if err := b.store.SaveLog(service, e, e.Timestamp); err != nil {
		logger.Default.Error("bus log save error", "service", service, "err", err)
	}
}
