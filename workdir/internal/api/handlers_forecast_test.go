package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/predictor"
	"github.com/benfradjselim/ruptura/internal/telemetry"
)

func TestForecastRoute_ResolvesKubernetesWorkloads(t *testing.T) {
	const workload = "demo/Deployment/orders"
	pred := predictor.NewPredictor()
	start := time.Now().Add(-10 * time.Minute)
	for i := 0; i < 40; i++ {
		pred.Feed(workload, "memory_percent", 0.2+float64(i)*0.01, start.Add(time.Duration(i)*15*time.Second))
	}
	h := New(nil, nil, nil, nil, nil, nil, nil, nil, telemetry.NewRegistry("test"), telemetry.NewHealthChecker(), "key")
	h.predictor = pred

	tests := []struct {
		name string
		path string
	}{
		{"full key", "/api/v2/forecast/memory_percent/demo/Deployment/orders"},
		{"encoded key as sent by the UI", "/api/v2/forecast/memory_percent/" + url.PathEscape(workload)},
		{"namespace and name", "/api/v2/forecast/memory_percent/demo/orders"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Header.Set("Authorization", "Bearer key")
			rec := httptest.NewRecorder()
			h.NewRouter().ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			var body struct {
				Host string `json:"host"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if body.Host != workload {
				t.Fatalf("host = %q, want %q", body.Host, workload)
			}
		})
	}
}
