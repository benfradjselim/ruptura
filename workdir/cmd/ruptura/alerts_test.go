package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/api"
	"github.com/benfradjselim/ruptura/internal/notify"
)

func TestAlertClients_TokenOnlyToMarkedTargets(t *testing.T) {
	var mu sync.Mutex
	auth := map[string]string{}
	handler := func(name string) http.Handler {
		return http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			mu.Lock()
			auth[name] = r.Header.Get("Authorization")
			mu.Unlock()
		})
	}
	plain := httptest.NewServer(handler("plain"))
	defer plain.Close()
	marked := httptest.NewServer(handler("marked"))
	defer marked.Close()

	tokenFile := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenFile, []byte("sa-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	previous := serviceAccountToken
	serviceAccountToken = tokenFile
	defer func() { serviceAccountToken = previous }()
	t.Setenv(alertTargetsEnv, plain.URL+" , ")
	t.Setenv(alertTokenTargetsEnv, marked.URL)
	t.Setenv(alertCAFileEnv, "")

	clients := alertClients()
	if len(clients) != 2 {
		t.Fatalf("clients = %d, want 2", len(clients))
	}
	sendAlerts(context.Background(), clients, []*api.FailureResult{{FailureMode: "crash_loop", State: "critical"}})
	if got, ok := auth["plain"]; !ok || got != "" {
		t.Errorf("plain target: Authorization = %q, received = %v", got, ok)
	}
	if auth["marked"] != "Bearer sa-token" {
		t.Errorf("marked target: Authorization = %q", auth["marked"])
	}
}

func TestAlertClients_BadCAKeepsSending(t *testing.T) {
	t.Setenv(alertTargetsEnv, "http://127.0.0.1:1")
	t.Setenv(alertTokenTargetsEnv, "")
	t.Setenv(alertCAFileEnv, filepath.Join(t.TempDir(), "absent"))
	if got := len(alertClients()); got != 1 {
		t.Errorf("clients = %d, want 1 with system roots", got)
	}
}

type recordingSender struct{ events []notify.Event }

func (r *recordingSender) Notify(_ context.Context, ev notify.Event) error {
	r.events = append(r.events, ev)
	return nil
}

func TestNotifyNew_OncePerAppearance(t *testing.T) {
	sender := &recordingSender{}
	now := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	leak := &api.FailureResult{Target: "shop/Deployment/api", FailureMode: "saturation", Kind: "predicted", State: "critical", ETASeconds: 300, RecommendedAction: "Raise the memory limit."}
	notified := notifyNew(context.Background(), sender, nil, []*api.FailureResult{leak}, now)
	notified = notifyNew(context.Background(), sender, notified, []*api.FailureResult{leak}, now)
	if len(sender.events) != 1 {
		t.Fatalf("sent %d events while the failure lasted, want 1", len(sender.events))
	}
	if ev := sender.events[0]; ev.Tier != notify.Tier1 || ev.Host != leak.Target || !strings.Contains(ev.Reason, "Raise the memory limit") {
		t.Fatalf("event = %+v", ev)
	}
	notified = notifyNew(context.Background(), sender, notified, nil, now)
	notifyNew(context.Background(), sender, notified, []*api.FailureResult{leak}, now)
	if len(sender.events) != 2 {
		t.Fatalf("a failure that came back was not notified again: %d events", len(sender.events))
	}
}
