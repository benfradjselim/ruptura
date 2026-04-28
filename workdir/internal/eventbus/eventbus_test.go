package eventbus_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/eventbus"
)

func TestMemBusPublishSubscribe(t *testing.T) {
	b := eventbus.NewMemBus()
	defer b.Close()

	var received []eventbus.Event
	var mu sync.Mutex
	cancel := b.Subscribe("ohe.metric", func(ctx context.Context, e eventbus.Event) {
		mu.Lock()
		received = append(received, e)
		mu.Unlock()
	})
	defer cancel()

	if err := b.Publish(context.Background(), "ohe.metric.ingest", "org1", map[string]interface{}{"count": 5}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	mu.Lock()
	if len(received) != 1 {
		t.Errorf("expected 1 event, got %d", len(received))
	}
	mu.Unlock()
}

func TestMemBusPrefixFiltering(t *testing.T) {
	b := eventbus.NewMemBus()
	defer b.Close()

	var alertCount, sloCount int
	b.Subscribe("ohe.alert", func(ctx context.Context, e eventbus.Event) { alertCount++ })
	b.Subscribe("ohe.slo", func(ctx context.Context, e eventbus.Event) { sloCount++ })

	b.Publish(context.Background(), "ohe.alert.fire", "org1", nil)
	b.Publish(context.Background(), "ohe.alert.resolve", "org1", nil)
	b.Publish(context.Background(), "ohe.slo.breach", "org1", nil)

	if alertCount != 2 {
		t.Errorf("alertCount = %d; want 2", alertCount)
	}
	if sloCount != 1 {
		t.Errorf("sloCount = %d; want 1", sloCount)
	}
}

func TestMemBusUnsubscribe(t *testing.T) {
	b := eventbus.NewMemBus()
	defer b.Close()

	count := 0
	cancel := b.Subscribe("ohe.", func(ctx context.Context, e eventbus.Event) { count++ })

	b.Publish(context.Background(), "ohe.metric.ingest", "org1", nil)
	cancel() // unsubscribe
	b.Publish(context.Background(), "ohe.metric.ingest", "org1", nil)

	if count != 1 {
		t.Errorf("expected 1 event after unsubscribe, got %d", count)
	}
}

func TestMemBusMultipleSubscribers(t *testing.T) {
	b := eventbus.NewMemBus()
	defer b.Close()

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		b.Subscribe("ohe.", func(ctx context.Context, e eventbus.Event) { wg.Done() })
	}

	b.Publish(context.Background(), "ohe.kpi.anomaly", "org2", map[string]interface{}{"value": 99.9})
	wg.Wait()
}

func TestMemBusEventFields(t *testing.T) {
	b := eventbus.NewMemBus()
	defer b.Close()

	var got eventbus.Event
	b.Subscribe("ohe.", func(ctx context.Context, e eventbus.Event) { got = e })
	b.Publish(context.Background(), "ohe.alert.fire", "my-org", map[string]string{"id": "a1"})

	if got.Topic != "ohe.alert.fire" {
		t.Errorf("Topic = %q; want ohe.alert.fire", got.Topic)
	}
	if got.OrgID != "my-org" {
		t.Errorf("OrgID = %q; want my-org", got.OrgID)
	}
	if got.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
	if len(got.Payload) == 0 {
		t.Error("Payload should not be empty")
	}
}

func TestNATSBusLocalDelivery(t *testing.T) {
	// NATSBus with an unreachable NATS URL — local delivery must still work.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	b := eventbus.NewNATSBus(ctx, eventbus.NATSConfig{
		URL:           "http://127.0.0.1:1", // nothing listening
		SubjectPrefix: "ohe",
		FlushInterval: 50 * time.Millisecond,
	})
	defer b.Close()

	received := make(chan eventbus.Event, 1)
	b.Subscribe("ohe.", func(ctx context.Context, e eventbus.Event) {
		received <- e
	})

	if err := b.Publish(context.Background(), "ohe.metric.ingest", "org1", map[string]int{"count": 3}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case e := <-received:
		if e.Topic != "ohe.metric.ingest" {
			t.Errorf("Topic = %q", e.Topic)
		}
	case <-time.After(2 * time.Second):
		t.Error("timed out waiting for event")
	}
}

func TestNATSBusClose(t *testing.T) {
	ctx := context.Background()
	b := eventbus.NewNATSBus(ctx, eventbus.NATSConfig{
		URL:           "http://127.0.0.1:1",
		FlushInterval: 10 * time.Millisecond,
	})
	b.Publish(context.Background(), "ohe.test", "org1", nil)
	if err := b.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestFactory(t *testing.T) {
	ctx := context.Background()
	// Empty URL → MemBus
	b := eventbus.New(ctx, "", "ohe")
	if b == nil {
		t.Fatal("expected non-nil Bus from factory")
	}
	defer b.Close()
	if err := b.Publish(ctx, "ohe.test", "org1", nil); err != nil {
		t.Errorf("Publish on MemBus: %v", err)
	}

	// Non-empty URL → NATSBus (no real NATS needed for factory test)
	b2 := eventbus.New(ctx, "http://127.0.0.1:1", "ohe")
	if b2 == nil {
		t.Fatal("expected non-nil Bus from factory with nats URL")
	}
	defer b2.Close()
}
