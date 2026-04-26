package eventbus

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestKafkaBus_DialFail(t *testing.T) {
	ctx := context.Background()
	_, err := NewKafkaBus(ctx, KafkaConfig{Brokers: "127.0.0.1:1", DialTimeout: 100 * time.Millisecond})
	if err == nil {
		t.Error("expected error for closed port, got nil")
	}
}

func TestKafkaBus_LocalDelivery(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	ctx := context.Background()
	bus, err := NewKafkaBus(ctx, KafkaConfig{Brokers: ln.Addr().String()})
	if err != nil {
		t.Fatal(err)
	}
	defer bus.Close()

	received := make(chan struct{})
	bus.Subscribe("test", func(ctx context.Context, e Event) {
		close(received)
	})

	err = bus.Publish(ctx, "test", "org1", "payload")
	if err != nil {
		t.Error(err)
	}

	select {
	case <-received:
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for local delivery")
	}
}

func TestKafkaBus_Close(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	bus, err := NewKafkaBus(context.Background(), KafkaConfig{Brokers: ln.Addr().String()})
	if err != nil {
		t.Fatal(err)
	}

	if err := bus.Close(); err != nil {
		t.Error(err)
	}
	if err := bus.Close(); err != nil {
		t.Error("second close should not error")
	}
}

func TestKafkaBus_PublishAfterClose(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	bus, err := NewKafkaBus(context.Background(), KafkaConfig{Brokers: ln.Addr().String()})
	if err != nil {
		t.Fatal(err)
	}
	bus.Close()

	// Should not panic
	err = bus.Publish(context.Background(), "test", "org1", "payload")
	if err != nil {
		t.Error("Publish after Close should not return error for MemBus")
	}
}

func TestNewWithKafka_EmptyBrokers(t *testing.T) {
	b := NewWithKafka(context.Background(), "", "prefix")
	if _, ok := b.(*MemBus); !ok {
		t.Error("expected MemBus for empty brokers")
	}
}

func TestNewWithKafka_BadBrokers(t *testing.T) {
	b := NewWithKafka(context.Background(), "127.0.0.1:1", "prefix")
	if _, ok := b.(*MemBus); !ok {
		t.Error("expected MemBus as fallback for bad brokers")
	}
}
