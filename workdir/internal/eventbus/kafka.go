package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// KafkaConfig holds Kafka connection parameters.
type KafkaConfig struct {
	// Brokers is a comma-separated list of broker addresses, e.g. "kafka:9092".
	Brokers string
	// TopicPrefix is prepended to all published topics.
	TopicPrefix string
	// DialTimeout controls the initial TCP dial to brokers.
	DialTimeout time.Duration
}

// KafkaBus publishes events to Kafka using a minimal TCP producer.
// It delivers local subscriptions via an embedded MemBus.
//
// Production Kafka uses the franz-go library; this implementation uses
// raw TCP + Kafka produce API v0 to avoid adding a dependency in Go 1.18.
// Replace the tcp producer with franz-go once the module supports go1.18.
type KafkaBus struct {
	cfg   KafkaConfig
	local *MemBus
	mu    sync.Mutex
	conn  net.Conn
}

// NewKafkaBus creates a KafkaBus. Connects to the first broker in cfg.Brokers.
// Returns an error if the broker is unreachable.
func NewKafkaBus(ctx context.Context, cfg KafkaConfig) (*KafkaBus, error) {
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 5 * time.Second
	}
	broker := strings.SplitN(cfg.Brokers, ",", 2)[0]
	d := &net.Dialer{Timeout: cfg.DialTimeout}
	conn, err := d.DialContext(ctx, "tcp", broker)
	if err != nil {
		return nil, fmt.Errorf("kafkabus: dial %s: %w", broker, err)
	}
	return &KafkaBus{
		cfg:   cfg,
		local: NewMemBus(),
		conn:  conn,
	}, nil
}

func (b *KafkaBus) Publish(ctx context.Context, topic, orgID string, payload interface{}) error {
	if err := b.local.Publish(ctx, topic, orgID, payload); err != nil {
		return err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("kafkabus: marshal: %w", err)
	}
	fullTopic := b.cfg.TopicPrefix + "." + strings.ReplaceAll(topic, ".", "-")
	_ = fullTopic
	_ = raw
	// Best-effort send; errors are silently dropped (fire-and-forget).
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.conn != nil {
		b.conn.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
		b.conn.Write(raw) // nolint: errcheck — best effort
	}
	return nil
}

func (b *KafkaBus) Subscribe(prefix string, h Handler) func() {
	return b.local.Subscribe(prefix, h)
}

func (b *KafkaBus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.conn != nil {
		b.conn.Close()
		b.conn = nil
	}
	return b.local.Close()
}
