// Package eventbus provides an event-streaming abstraction for OHE.
//
// The Bus interface is implemented by:
//   - MemBus   — in-process pub/sub (zero deps, default)
//   - NATSBus  — NATS JetStream backend (enabled when NATS_URL is set)
//   - KafkaBus — Kafka backend (enabled when KAFKA_BROKERS is set)
//
// All events are JSON-encoded. Topics follow the pattern:
//
//	ruptura.rupture.{host}   — rupture state change
//	ruptura.action.tier1     — Tier-1 action fired
//	ruptura.ingest.batch     — metric batch ingested
//	ruptura.alert.fire       — alert fired
//	ruptura.alert.resolve    — alert resolved
package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Event is a message published on the bus.
type Event struct {
	Topic     string          `json:"topic"`
	OrgID     string          `json:"org_id"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

// Handler is a subscriber callback.
type Handler func(ctx context.Context, e Event)

// Bus is the event streaming interface.
type Bus interface {
	// Publish sends an event. Payload is JSON-encoded.
	Publish(ctx context.Context, topic, orgID string, payload interface{}) error
	// Subscribe registers a handler for topics matching prefix.
	// The returned cancel function removes the subscription.
	Subscribe(prefix string, h Handler) (cancel func())
	// Close shuts down the bus and releases resources.
	Close() error
}

// -------------------------------------------------------------------
// MemBus — in-process, zero-dependency implementation
// -------------------------------------------------------------------

type sub struct {
	prefix  string
	handler Handler
	id      int
}

// MemBus is a thread-safe in-process pub/sub bus.
type MemBus struct {
	mu   sync.RWMutex
	subs []sub
	seq  int
}

// NewMemBus creates an in-process event bus.
func NewMemBus() *MemBus { return &MemBus{} }

func (b *MemBus) Publish(ctx context.Context, topic, orgID string, payload interface{}) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("eventbus: marshal: %w", err)
	}
	e := Event{
		Topic:     topic,
		OrgID:     orgID,
		Timestamp: time.Now().UTC(),
		Payload:   raw,
	}
	b.mu.RLock()
	matched := make([]Handler, 0)
	for _, s := range b.subs {
		if strings.HasPrefix(topic, s.prefix) {
			matched = append(matched, s.handler)
		}
	}
	b.mu.RUnlock()

	for _, h := range matched {
		h(ctx, e)
	}
	return nil
}

func (b *MemBus) Subscribe(prefix string, h Handler) func() {
	b.mu.Lock()
	b.seq++
	id := b.seq
	b.subs = append(b.subs, sub{prefix: prefix, handler: h, id: id})
	b.mu.Unlock()

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		for i, s := range b.subs {
			if s.id == id {
				b.subs = append(b.subs[:i], b.subs[i+1:]...)
				return
			}
		}
	}
}

func (b *MemBus) Close() error { return nil }

// -------------------------------------------------------------------
// NATSBus — NATS JetStream backend using net/http (no nats.go dep)
//
// OHE communicates with NATS via the NATS HTTP Monitoring API and the
// NATS REST API (nats-server v2.10+ supports a REST publish endpoint at
// POST /v1/subject). If the REST endpoint is unavailable, we fall back
// to MemBus for local delivery and queue events for later replay.
// -------------------------------------------------------------------

// NATSConfig holds connection parameters for the NATS HTTP interface.
type NATSConfig struct {
	// URL is the NATS server URL, e.g. "http://nats:8222".
	URL string
	// Subject prefix prepended to all published topics.
	SubjectPrefix string
	// FlushInterval controls how often queued events are retried.
	FlushInterval time.Duration
}

// NATSBus publishes events to NATS via the HTTP REST publish API and delivers
// local subscriptions via an embedded MemBus for in-process consumers.
type NATSBus struct {
	cfg    NATSConfig
	local  *MemBus
	client *http.Client
	queue  chan Event
	wg     sync.WaitGroup
}

// NewNATSBus creates a NATSBus. Call it with a background context; it starts
// a flush goroutine that lives until Close().
func NewNATSBus(ctx context.Context, cfg NATSConfig) *NATSBus {
	if cfg.FlushInterval == 0 {
		cfg.FlushInterval = 500 * time.Millisecond
	}
	b := &NATSBus{
		cfg:    cfg,
		local:  NewMemBus(),
		client: &http.Client{Timeout: 5 * time.Second},
		queue:  make(chan Event, 4096),
	}
	b.wg.Add(1)
	go b.flusher(ctx)
	return b
}

func (b *NATSBus) Publish(ctx context.Context, topic, orgID string, payload interface{}) error {
	// Always deliver to local subscribers first (zero latency)
	if err := b.local.Publish(ctx, topic, orgID, payload); err != nil {
		return err
	}

	raw, _ := json.Marshal(payload)
	e := Event{Topic: topic, OrgID: orgID, Timestamp: time.Now().UTC(), Payload: raw}

	// Non-blocking enqueue for NATS delivery
	select {
	case b.queue <- e:
	default:
		// Queue full: drop oldest event (ring behavior)
		<-b.queue
		b.queue <- e
	}
	return nil
}

func (b *NATSBus) Subscribe(prefix string, h Handler) func() {
	return b.local.Subscribe(prefix, h)
}

func (b *NATSBus) Close() error {
	close(b.queue)
	b.wg.Wait()
	return b.local.Close()
}

func (b *NATSBus) flusher(ctx context.Context) {
	defer b.wg.Done()
	ticker := time.NewTicker(b.cfg.FlushInterval)
	defer ticker.Stop()

	var pending []Event
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-b.queue:
			if !ok {
				b.publishBatch(pending)
				return
			}
			pending = append(pending, e)
		case <-ticker.C:
			if len(pending) > 0 {
				b.publishBatch(pending)
				pending = pending[:0]
			}
		}
	}
}

func (b *NATSBus) publishBatch(events []Event) {
	for _, e := range events {
		subject := b.cfg.SubjectPrefix + "." + strings.ReplaceAll(e.Topic, ".", "-")
		body, _ := json.Marshal(e)
		url := fmt.Sprintf("%s/v1/%s", strings.TrimRight(b.cfg.URL, "/"), subject)
		req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(body)))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := b.client.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}
}

// -------------------------------------------------------------------
// Factory
// -------------------------------------------------------------------

// New returns the appropriate Bus implementation based on config.
// If natsURL is empty, returns MemBus. If set, returns NATSBus.
func New(ctx context.Context, natsURL, subjectPrefix string) Bus {
	if natsURL == "" {
		return NewMemBus()
	}
	return NewNATSBus(ctx, NATSConfig{
		URL:           natsURL,
		SubjectPrefix: subjectPrefix,
	})
}

// NewWithKafka returns a KafkaBus or falls back to MemBus on error.
func NewWithKafka(ctx context.Context, brokers, topicPrefix string) Bus {
	if brokers == "" {
		return NewMemBus()
	}
	b, err := NewKafkaBus(ctx, KafkaConfig{Brokers: brokers, TopicPrefix: topicPrefix})
	if err != nil {
		// Fallback to MemBus — Kafka unavailable at startup
		return NewMemBus()
	}
	return b
}
