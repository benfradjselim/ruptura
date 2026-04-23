package ohe

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/health" {
			t.Errorf("expected path /api/v1/health, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "ohe_test_key")
	healthy, err := client.Health()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !healthy {
		t.Error("expected healthy, got unhealthy")
	}
}
