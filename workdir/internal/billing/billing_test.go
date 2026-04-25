package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRecord(t *testing.T) {
	m := New("", 100, time.Minute)
	m.Record("org1", EventIngestBytes, 1024)
	m.Record("org1", EventAPICall, 1)
	m.Record("org2", EventPrediction, 5)

	events := m.GetAndReset()
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	if events[0].OrgID != "org1" || events[0].EventType != EventIngestBytes {
		t.Errorf("unexpected first event: %+v", events[0])
	}
}

func TestGetAndReset(t *testing.T) {
	m := New("", 100, time.Minute)
	m.Record("org1", EventAPICall, 1)

	first := m.GetAndReset()
	if len(first) != 1 {
		t.Fatal("expected 1 event")
	}
	second := m.GetAndReset()
	if len(second) != 0 {
		t.Fatal("expected empty after reset")
	}
}

func TestRingBufferDropsOldest(t *testing.T) {
	m := New("", 3, time.Minute)
	m.Record("a", EventAPICall, 1)
	m.Record("b", EventAPICall, 2)
	m.Record("c", EventAPICall, 3)
	m.Record("d", EventAPICall, 4) // should drop "a"

	events := m.GetAndReset()
	if len(events) != 3 {
		t.Fatalf("expected 3, got %d", len(events))
	}
	if events[0].OrgID != "b" {
		t.Errorf("oldest not dropped, first is %s", events[0].OrgID)
	}
}

func TestFlushToWebhook(t *testing.T) {
	var received []UsageEvent
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Events []UsageEvent `json:"events"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode error: %v", err)
		}
		received = body.Events
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	m := New(srv.URL, 100, 10*time.Millisecond)
	m.Record("org1", EventIngestBytes, 512)
	m.Record("org1", EventAPICall, 3)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	m.Run(ctx)

	if len(received) != 2 {
		t.Fatalf("expected 2 events flushed, got %d", len(received))
	}
}

func TestRunNoWebhook(t *testing.T) {
	m := New("", 100, 10*time.Millisecond)
	m.Record("org1", EventAlertEval, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	m.Run(ctx) // should return immediately when no webhook is set

	// Events should still be in the buffer (not flushed)
	events := m.GetAndReset()
	if len(events) != 1 {
		t.Errorf("expected 1 buffered event, got %d", len(events))
	}
}
