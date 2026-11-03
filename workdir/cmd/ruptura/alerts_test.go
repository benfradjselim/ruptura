package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/benfradjselim/ruptura/internal/api"
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
