package alertmanager

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSend(t *testing.T) {
	var got []postableAlert
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	alert := Alert{Name: "saturation", Severity: "critical", Level: "workload", Target: "n/Deployment/w", Kind: "predicted", Summary: "act"}
	if err := NewClient(srv.URL, nil).Send(context.Background(), []Alert{alert}); err != nil {
		t.Fatal(err)
	}
	if path != "/api/v2/alerts" || len(got) != 1 {
		t.Fatalf("path = %s, alerts = %+v", path, got)
	}
	labels := got[0].Labels
	if labels["alertname"] != "RupturaSaturation" || labels["severity"] != "critical" || labels["target"] != "n/Deployment/w" || labels["kind"] != "predicted" {
		t.Errorf("labels = %v", labels)
	}
	if got[0].Annotations["summary"] != "act" {
		t.Errorf("annotations = %v", got[0].Annotations)
	}
}

func TestSend_ReportsFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	if err := NewClient(srv.URL, nil).Send(context.Background(), []Alert{{Name: "x"}}); err == nil {
		t.Error("a refused alert must return an error")
	}
}

func TestSend_NothingToSend(t *testing.T) {
	if err := NewClient("http://127.0.0.1:1", nil).Send(context.Background(), nil); err != nil {
		t.Errorf("no alert must not call the server: %v", err)
	}
}

func TestAlertName(t *testing.T) {
	cases := map[string]string{"saturation": "RupturaSaturation", "crash_loop": "RupturaCrashLoop", "startup_stuck": "RupturaStartupStuck"}
	for mode, want := range cases {
		if got := alertName(mode); got != want {
			t.Errorf("%s: got %s, want %s", mode, got, want)
		}
	}
}

func TestSend_SeverityStateAndEnd(t *testing.T) {
	cases := []struct{ state, severity string }{
		{"critical", "critical"},
		{"degraded", "warning"},
		{"", "warning"},
	}
	for _, tc := range cases {
		var got []postableAlert
		srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&got)
		}))
		client := NewClient(srv.URL, nil)
		client.now = func() time.Time { return time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC) }
		if err := client.Send(context.Background(), []Alert{{Name: "crash_loop", Severity: tc.state}}); err != nil {
			t.Fatal(err)
		}
		srv.Close()
		if got[0].Labels["severity"] != tc.severity || got[0].Annotations["state"] != tc.state {
			t.Errorf("state %q: labels = %v", tc.state, got[0].Labels)
		}
		if got[0].EndsAt != "2026-11-03T12:03:00Z" {
			t.Errorf("endsAt = %s, want 3 min after now", got[0].EndsAt)
		}
	}
}

func TestSend_BearerToken(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenFile, []byte("abc\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		opts []Option
		want string
	}{
		{"without token", nil, ""},
		{"with token", []Option{WithTokenFile(tokenFile)}, "Bearer abc"},
	}
	for _, tc := range cases {
		var auth string
		srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			auth = r.Header.Get("Authorization")
		}))
		if err := NewClient(srv.URL, nil, tc.opts...).Send(context.Background(), []Alert{{Name: "x"}}); err != nil {
			t.Fatal(err)
		}
		srv.Close()
		if auth != tc.want {
			t.Errorf("%s: Authorization = %q, want %q", tc.name, auth, tc.want)
		}
	}
}

func TestSend_MissingTokenFails(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", nil, WithTokenFile(filepath.Join(t.TempDir(), "absent")))
	if err := client.Send(context.Background(), []Alert{{Name: "x"}}); err == nil {
		t.Error("a missing token file must return an error")
	}
}

func TestHTTPClientWithCA(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	caFile := filepath.Join(t.TempDir(), "ca.crt")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	if err := os.WriteFile(caFile, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := NewClient(srv.URL, nil).Send(context.Background(), []Alert{{Name: "x"}}); err == nil {
		t.Error("an unknown CA must be refused without the CA file")
	}
	client, err := HTTPClientWithCA(caFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := NewClient(srv.URL, client).Send(context.Background(), []Alert{{Name: "x"}}); err != nil {
		t.Errorf("CA file must be trusted: %v", err)
	}
	if _, err := HTTPClientWithCA(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Error("a missing CA file must return an error")
	}
}

func TestAlertLabels(t *testing.T) {
	cases := []struct {
		target, namespace string
	}{
		{"demo-shop/Deployment/step-leak", "demo-shop"},
		{"node-1", ""},
		{"/Deployment/x", ""},
	}
	for _, tc := range cases {
		labels := alertLabels(Alert{Name: "saturation", Severity: "degraded", Target: tc.target})
		if labels["namespace"] != tc.namespace {
			t.Errorf("%s: namespace = %q, want %q", tc.target, labels["namespace"], tc.namespace)
		}
		if _, ok := labels["state"]; ok {
			t.Errorf("%s: state must not be a label", tc.target)
		}
	}
}

func TestAnnotationsCarryImpact(t *testing.T) {
	tests := []struct {
		impact string
		want   int
	}{
		{"", 2},
		{"Can hit its callers: api", 3},
	}
	for _, tc := range tests {
		notes := annotations(Alert{Summary: "act", Severity: "critical", Impact: tc.impact})
		if len(notes) != tc.want || (tc.impact != "" && notes["impact"] != tc.impact) {
			t.Errorf("impact %q: got %v", tc.impact, notes)
		}
	}
}
