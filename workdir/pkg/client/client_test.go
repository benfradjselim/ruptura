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
	if c.httpClient.Timeout != 10*time.Second {
		t.Errorf("want 10s, got %v", c.httpClient.Timeout)
	}
}

func TestHealth_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ready"}`)
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
}

func TestHealth_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := New(Config{BaseURL: ts.URL})
	_, err := c.Health(context.Background())
	if err == nil || !strings.Contains(err.Error(), "status 500") {
		t.Errorf("expected 500 error, got %v", err)
	}
}

func TestRuptures_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	}))
	defer ts.Close()

	c := New(Config{BaseURL: ts.URL})
	res, err := c.Ruptures(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || len(res) != 0 {
		t.Errorf("want empty slice, got %v", res)
	}
}

func TestRuptureForHost_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"host":"h1","metric":"m1","rupture_index":0.5}`)
	}))
	defer ts.Close()

	c := New(Config{BaseURL: ts.URL})
	res, err := c.RuptureForHost(context.Background(), "h1")
	if err != nil {
		t.Fatal(err)
	}
	if res.Host != "h1" || res.RuptureIndex != 0.5 {
		t.Errorf("unexpected response: %+v", res)
	}
}

func TestKPI_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"name":"stress","host":"h1","value":0.5}`)
	}))
	defer ts.Close()

	c := New(Config{BaseURL: ts.URL})
	res, err := c.KPI(context.Background(), "stress", "h1")
	if err != nil {
		t.Fatal(err)
	}
	if res.Name != "stress" || res.Value != 0.5 {
		t.Errorf("unexpected response: %+v", res)
	}
}

func TestAddContext_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"1"}`)
	}))
	defer ts.Close()

	c := New(Config{BaseURL: ts.URL})
	res, err := c.AddContext(context.Background(), ContextEntry{ID: "1"})
	if err != nil {
		t.Fatal(err)
	}
	if res.ID != "1" {
		t.Errorf("want 1, got %s", res.ID)
	}
}

func TestDeleteContext_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := New(Config{BaseURL: ts.URL})
	err := c.DeleteContext(context.Background(), "1")
	if err != nil {
		t.Fatal(err)
	}
}

func TestListContexts_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"id":"1"}]`)
	}))
	defer ts.Close()

	c := New(Config{BaseURL: ts.URL})
	res, err := c.ListContexts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].ID != "1" {
		t.Errorf("unexpected response: %+v", res)
	}
}

func TestEmergencyStop_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"emergency_stop":true}`)
	}))
	defer ts.Close()

	c := New(Config{BaseURL: ts.URL})
	res, err := c.EmergencyStop(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !res.EmergencyStop {
		t.Errorf("want true")
	}
}

func TestMetrics_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "metric1 1.0")
	}))
	defer ts.Close()

	c := New(Config{BaseURL: ts.URL})
	res, err := c.Metrics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res != "metric1 1.0" {
		t.Errorf("want metric1 1.0, got %s", res)
	}
}

func TestAuth_HeaderSent(t *testing.T) {
	token := "secret-token"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+token {
			t.Errorf("want Bearer %s, got %s", token, auth)
		}
	}))
	defer ts.Close()

	c := New(Config{BaseURL: ts.URL, APIKey: token})
	c.Health(context.Background())
}

func TestClient_NetworkError(t *testing.T) {
	c := New(Config{BaseURL: "http://invalid-url-that-does-not-exist"})
	_, err := c.Health(context.Background())
	if err == nil {
		t.Error("expected error")
	}
}
