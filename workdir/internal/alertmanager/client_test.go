package alertmanager

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
