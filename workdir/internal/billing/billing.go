// Package billing provides lightweight usage metering for SaaS billing integration.
// Events are appended to an in-memory ring buffer and flushed to a webhook endpoint
// (e.g. Stripe Billing, Lago, or a custom aggregator) on a configurable interval.
package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/benfradjselim/kairo-core/pkg/logger"
)

// EventType classifies what was metered.
type EventType string

const (
	EventIngestBytes EventType = "ingest_bytes"
	EventAPICall     EventType = "api_call"
	EventAlertEval   EventType = "alert_eval"
	EventPrediction  EventType = "prediction"
)

// UsageEvent is a single billable event.
type UsageEvent struct {
	OrgID     string    `json:"org_id"`
	EventType EventType `json:"event_type"`
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

// Meter collects UsageEvents in a ring buffer and flushes them to a webhook.
type Meter struct {
	mu          sync.Mutex
	buf         []UsageEvent
	maxBuf      int
	webhookURL  string
	flushPeriod time.Duration
	client      *http.Client
}

// New creates a Meter. webhookURL may be empty to disable flushing (events are
// still collected so they can be read by GetAndReset).
func New(webhookURL string, bufSize int, flushPeriod time.Duration) *Meter {
	if bufSize <= 0 {
		bufSize = 10000
	}
	if flushPeriod <= 0 {
		flushPeriod = time.Minute
	}
	return &Meter{
		buf:         make([]UsageEvent, 0, bufSize),
		maxBuf:      bufSize,
		webhookURL:  webhookURL,
		flushPeriod: flushPeriod,
		client:      &http.Client{Timeout: 10 * time.Second},
	}
}

// Record appends a usage event. When the buffer is full the oldest event is dropped
// (ring-buffer semantics) to prevent unbounded memory growth.
func (m *Meter) Record(orgID string, ev EventType, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.buf) >= m.maxBuf {
		// Drop oldest
		m.buf = m.buf[1:]
	}
	m.buf = append(m.buf, UsageEvent{
		OrgID:     orgID,
		EventType: ev,
		Value:     value,
		Timestamp: time.Now().UTC(),
	})
}

// GetAndReset returns all buffered events and clears the buffer.
func (m *Meter) GetAndReset() []UsageEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.buf) == 0 {
		return nil
	}
	out := make([]UsageEvent, len(m.buf))
	copy(out, m.buf)
	m.buf = m.buf[:0]
	return out
}

// Run starts the periodic flush loop. Blocks until ctx is cancelled.
func (m *Meter) Run(ctx context.Context) {
	if m.webhookURL == "" {
		return // no-op when webhook is not configured
	}
	ticker := time.NewTicker(m.flushPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			m.flush() // drain on shutdown
			return
		case <-ticker.C:
			m.flush()
		}
	}
}

func (m *Meter) flush() {
	events := m.GetAndReset()
	if len(events) == 0 {
		return
	}
	body, err := json.Marshal(map[string]interface{}{
		"events":     events,
		"flushed_at": time.Now().UTC(),
	})
	if err != nil {
		logger.Default.Error("billing flush marshal error", "err", err)
		return
	}
	resp, err := m.client.Post(m.webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		logger.Default.Error("billing flush webhook error", "err", err, "events", len(events))
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		logger.Default.Warn("billing webhook returned error", "status", resp.StatusCode, "events", len(events))
		return
	}
	logger.Default.Info("billing flushed", "events", len(events))
}
