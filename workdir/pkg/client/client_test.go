package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNew_DefaultTimeout(t *testing.T) {
	c := New(Config{BaseURL: "http://localhost"})
	if c.httpClient.Timeout != 15*time.Second {
		t.Errorf("want 15s, got %v", c.httpClient.Timeout)
	}
}

func TestHealth_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ready","version":"6.6.0"}`)
	}))
	defer ts.Close()

	c := New(Config{BaseURL: ts.URL})
	res, err := c.Health(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "ready" {
		t.Errorf("want ready, got %s", res.Status)
	}
	if res.Version != "6.6.0" {
		t.Errorf("want 6.6.0, got %s", res.Version)
	}
}

func TestHealth_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := New(Config{BaseURL: ts.URL})
	_, err := c.Health(context.Background())
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("expected 500 error, got %v", err)
	}
}

func TestActions_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"id":"act_1","type":"scale","tier":2}]`)
	}))
	defer ts.Close()

	c := New(Config{BaseURL: ts.URL})
	res, err := c.Actions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].ID != "act_1" {
		t.Errorf("unexpected response: %+v", res)
	}
}

func TestApproveAction_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := New(Config{BaseURL: ts.URL})
	if err := c.ApproveAction(context.Background(), "act_1"); err != nil {
		t.Fatal(err)
	}
}

func TestEmergencyStop_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := New(Config{BaseURL: ts.URL})
	if err := c.EmergencyStop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestMetrics_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "rpt_uptime_seconds 120")
	}))
	defer ts.Close()

	c := New(Config{BaseURL: ts.URL})
	res, err := c.Metrics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res, "rpt_uptime_seconds") {
		t.Errorf("unexpected metrics: %s", res)
	}
}

func TestAuth_HeaderSent(t *testing.T) {
	token := "secret-token"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+token {
			t.Errorf("want Bearer %s, got %s", token, auth)
		}
		fmt.Fprint(w, `{"status":"ok"}`)
	}))
	defer ts.Close()

	c := New(Config{BaseURL: ts.URL, APIKey: token})
	c.Health(context.Background()) //nolint:errcheck
}

func TestClient_NetworkError(t *testing.T) {
	c := New(Config{BaseURL: "http://127.0.0.1:19999"})
	_, err := c.Health(context.Background())
	if err == nil {
		t.Error("expected error for unreachable server")
	}
}

func TestFailuresAndFleet_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/failures":
			_, _ = w.Write([]byte(`[{"target":"shop/Deployment/api","failureMode":"saturation","kind":"predicted","etaSeconds":600}]`))
		case "/api/v2/fleet":
			_, _ = w.Write([]byte(`{"total_hosts":1,"critical_hosts":1,"hosts":[{"host":"shop/Deployment/api","state":"critical","risk":8.5}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := New(Config{BaseURL: srv.URL})
	failures, err := c.Failures(context.Background())
	if err != nil || len(failures) != 1 || failures[0].ETASeconds != 600 {
		t.Fatalf("Failures = %+v, %v", failures, err)
	}
	fleet, err := c.Fleet(context.Background())
	if err != nil || fleet.CriticalHosts != 1 || fleet.Hosts[0].State != "critical" {
		t.Fatalf("Fleet = %+v, %v", fleet, err)
	}
}
