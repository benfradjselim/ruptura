package notifier_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/benfradjselim/kairo-core/internal/notifier"
	"github.com/benfradjselim/kairo-core/pkg/models"
)

func makeAlert(name, severity string) models.Alert {
	return models.Alert{
		ID:          "test-id",
		Name:        name,
		Host:        "host1",
		Severity:    severity,
		Description: "test message",
		Status:      "firing",
		CreatedAt:   time.Now(),
	}
}

func TestDispatchWebhook(t *testing.T) {
	var received int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&received, 1)
		var p map[string]interface{}
		json.NewDecoder(r.Body).Decode(&p)
		if p["alert_name"] != "cpu_high" {
			t.Errorf("unexpected alert_name: %v", p["alert_name"])
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := notifier.New()
	d.SetChannels([]notifier.Channel{
		{ID: "1", Name: "test", Type: "webhook", URL: srv.URL, Enabled: true},
	})

	d.Dispatch(makeAlert("cpu_high", "warning"))
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&received) != 1 {
		t.Errorf("webhook called %d times; want 1", received)
	}
}

func TestDispatchSkipsDisabled(t *testing.T) {
	var received int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&received, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := notifier.New()
	d.SetChannels([]notifier.Channel{
		{ID: "1", Name: "disabled", Type: "webhook", URL: srv.URL, Enabled: false},
	})

	d.Dispatch(makeAlert("stress_panic", "critical"))
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&received) != 0 {
		t.Errorf("disabled channel was called %d times; want 0", received)
	}
}

func TestDispatchMultipleChannels(t *testing.T) {
	var hits int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
	})
	srv1 := httptest.NewServer(handler)
	srv2 := httptest.NewServer(handler)
	defer srv1.Close()
	defer srv2.Close()

	d := notifier.New()
	d.SetChannels([]notifier.Channel{
		{ID: "1", Name: "ch1", Type: "webhook", URL: srv1.URL, Enabled: true},
		{ID: "2", Name: "ch2", Type: "slack", URL: srv2.URL, Enabled: true},
	})

	d.Dispatch(makeAlert("memory_high", "warning"))
	time.Sleep(150 * time.Millisecond)

	if atomic.LoadInt32(&hits) != 2 {
		t.Errorf("expected 2 channel hits; got %d", hits)
	}
}

func TestDispatchHTTPError(t *testing.T) {
	// Setup a server that returns 500
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	d := notifier.New()
	d.SetChannels([]notifier.Channel{
		{ID: "1", Name: "bad", Type: "webhook", URL: srv.URL, Enabled: true},
	})
	
	// Dispatch
	d.Dispatch(makeAlert("test", "info"))
	time.Sleep(200 * time.Millisecond)
}

func TestDispatchNetworkError(t *testing.T) {
	d := notifier.New()
	// Use an invalid URL to force Post to return an error
	d.SetChannels([]notifier.Channel{
		{ID: "1", Name: "invalid", Type: "webhook", URL: "http://invalid-url-that-does-not-exist", Enabled: true},
	})
	
	d.Dispatch(makeAlert("test", "info"))
	time.Sleep(200 * time.Millisecond)
}
