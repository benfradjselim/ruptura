package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNotify_TierFiltering(t *testing.T) {
	var received int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&received, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tests := []struct {
		name     string
		minTier  TierLevel
		evTier   TierLevel
		wantSent bool
	}{
		{"below min tier is suppressed", Tier2, Tier1, false},
		{"at min tier fires", Tier2, Tier2, true},
		{"above min tier fires", Tier2, Tier3, true},
		{"default min tier (zero Config) suppresses tier1", 0, Tier1, false},
		{"default min tier (zero Config) fires tier2", 0, Tier2, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			atomic.StoreInt32(&received, 0)
			n := New(Config{GenericWebhookURL: srv.URL, MinTier: tt.minTier})
			err := n.Notify(context.Background(), Event{ID: "e1", Host: "ns/Deployment/api", Tier: tt.evTier})
			if err != nil {
				t.Fatalf("Notify: %v", err)
			}
			got := atomic.LoadInt32(&received) == 1
			if got != tt.wantSent {
				t.Errorf("sent = %v, want %v", got, tt.wantSent)
			}
		})
	}
}

func TestNotify_SlackAndGeneric_BothFire(t *testing.T) {
	var slackHit, genericHit int32
	slack := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.StoreInt32(&slackHit, 1)
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("slack payload not valid JSON: %v", err)
		}
		if _, ok := body["text"]; !ok {
			t.Error("slack payload missing text field")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer slack.Close()
	generic := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.StoreInt32(&genericHit, 1)
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("generic payload not valid JSON: %v", err)
		}
		if body["workload"] != "ns/Deployment/api" {
			t.Errorf("generic payload workload = %v, want ns/Deployment/api", body["workload"])
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer generic.Close()

	n := New(Config{SlackWebhookURL: slack.URL, GenericWebhookURL: generic.URL, MinTier: Tier2})
	err := n.Notify(context.Background(), Event{ID: "e1", Host: "ns/Deployment/api", ActionType: "alert", Tier: Tier2, Reason: "FRI breach"})
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if atomic.LoadInt32(&slackHit) != 1 {
		t.Error("slack webhook was not called")
	}
	if atomic.LoadInt32(&genericHit) != 1 {
		t.Error("generic webhook was not called")
	}
}

func TestNotify_PartialFailureAggregatesErrors(t *testing.T) {
	badSlack := "http://127.0.0.1:1" // guaranteed connection refused
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer good.Close()

	n := New(Config{SlackWebhookURL: badSlack, GenericWebhookURL: good.URL, MinTier: Tier2})
	err := n.Notify(context.Background(), Event{ID: "e1", Host: "h", Tier: Tier2})
	if err == nil {
		t.Fatal("expected an error from the failing slack destination")
	}
	if !strings.Contains(err.Error(), "slack") {
		t.Errorf("error = %q, want it to mention slack", err.Error())
	}
}

func TestNotify_ServerErrorStatusIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	n := New(Config{GenericWebhookURL: srv.URL, MinTier: Tier2})
	err := n.Notify(context.Background(), Event{ID: "e1", Host: "h", Tier: Tier2})
	if err == nil {
		t.Fatal("expected an error when the destination returns 500")
	}
}

func TestNotify_NoDestinationsConfigured_NoOp(t *testing.T) {
	n := New(Config{MinTier: Tier2})
	if err := n.Notify(context.Background(), Event{ID: "e1", Host: "h", Tier: Tier3}); err != nil {
		t.Fatalf("Notify with no destinations should be a no-op, got: %v", err)
	}
	if n.Configured() {
		t.Error("Configured() should be false with no destinations set")
	}
}

func TestNotify_ErrorNeverLeaksDestinationURL(t *testing.T) {
	secretURL := "http://127.0.0.1:1/T00/SUPER/SECRETTOKEN123"
	n := New(Config{GenericWebhookURL: secretURL, MinTier: Tier2})
	err := n.Notify(context.Background(), Event{ID: "e1", Host: "h", Tier: Tier2})
	if err == nil {
		t.Fatal("expected a connection error")
	}
	if strings.Contains(err.Error(), "SECRETTOKEN123") {
		t.Errorf("error leaked the destination URL/token: %v", err)
	}
}

func TestNotifyChannel_UnknownChannel(t *testing.T) {
	n := New(Config{})
	err := n.NotifyChannel(context.Background(), "carrier-pigeon", Event{ID: "e1"})
	if err == nil {
		t.Fatal("expected an error for an unknown channel")
	}
}

func TestNotifyChannel_NotConfigured(t *testing.T) {
	n := New(Config{})
	if err := n.NotifyChannel(context.Background(), "slack", Event{ID: "e1"}); err == nil {
		t.Error("expected an error when slack is not configured")
	}
	if err := n.NotifyChannel(context.Background(), "webhook", Event{ID: "e1"}); err == nil {
		t.Error("expected an error when webhook is not configured")
	}
}

func TestNotifyChannel_SendsRegardlessOfEventTier(t *testing.T) {
	var hit int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.StoreInt32(&hit, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// NotifyChannel is used for "send a test notification" — it must bypass
	// the MinTier gate, unlike Notify().
	n := New(Config{GenericWebhookURL: srv.URL, MinTier: Tier3})
	if err := n.NotifyChannel(context.Background(), "webhook", Event{ID: "test", Tier: Tier1}); err != nil {
		t.Fatalf("NotifyChannel: %v", err)
	}
	if atomic.LoadInt32(&hit) != 1 {
		t.Error("webhook was not called despite Tier1 < MinTier(Tier3) — NotifyChannel must bypass the gate")
	}
}

func TestConfig_RoundTrips(t *testing.T) {
	cfg := Config{SlackWebhookURL: "https://hooks.slack.com/x", GenericWebhookURL: "https://example.com/hook", MinTier: Tier3}
	n := New(cfg)
	got := n.Config()
	if got.SlackWebhookURL != cfg.SlackWebhookURL || got.GenericWebhookURL != cfg.GenericWebhookURL || got.MinTier != cfg.MinTier {
		t.Errorf("Config() = %+v, want %+v", got, cfg)
	}
}

func TestNotify_TimestampDefaultsToNow(t *testing.T) {
	var gotTS string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		gotTS, _ = body["timestamp"].(string)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := New(Config{GenericWebhookURL: srv.URL, MinTier: Tier2})
	before := time.Now().Add(-time.Second)
	if err := n.Notify(context.Background(), Event{ID: "e1", Host: "h", Tier: Tier2}); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	parsed, err := time.Parse(time.RFC3339, gotTS)
	if err != nil {
		t.Fatalf("timestamp not RFC3339: %v", err)
	}
	if parsed.Before(before) {
		t.Errorf("timestamp %v looks stale (before %v)", parsed, before)
	}
}
