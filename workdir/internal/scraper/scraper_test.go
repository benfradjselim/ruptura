package scraper

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ── mapper tests ──────────────────────────────────────────────────────────────

func TestMapMetric_ExactMatch(t *testing.T) {
	cases := []struct{ in, want string }{
		{"stress", "stress"},
		{"cpu_percent", "stress"},
		{"memory_usage_percent", "pressure"},
		{"restart_count", "fatigue"},
		{"request_rate", "request_rate"},
		{"goroutine_count", "entropy"},
		{"queue_depth", "humidity"},
		{"health_score", "health_score"},
		{"http_error_rate", "error_rate"},
	}
	for _, c := range cases {
		got := MapMetric(c.in)
		if got != c.want {
			t.Errorf("MapMetric(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMapMetric_SubstringFallback(t *testing.T) {
	cases := []struct{ in, want string }{
		{"container_cpu_usage_total", "stress"},
		{"container_memory_bytes", "pressure"},
		{"pod_restarts_total", "fatigue"},
		{"http_error_rate_total", "error_rate"},
		{"go_goroutines_total", "entropy"},
		{"queue_pending_jobs", "humidity"},
		{"request_rate_p99", "request_rate"},
		{"data_throughput_bytes", "throughput"},
	}
	for _, c := range cases {
		got := MapMetric(c.in)
		if got != c.want {
			t.Errorf("MapMetric(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMapMetric_Unknown(t *testing.T) {
	if got := MapMetric("completely_unknown_xyzzy"); got != "" {
		t.Errorf("MapMetric(unknown) = %q, want empty", got)
	}
}

func TestNormalizeValue_ErrorRateFraction(t *testing.T) {
	// 0-1 fraction passes through unchanged
	if got := NormalizeValue("error_rate", 0.05); got != 0.05 {
		t.Errorf("NormalizeValue(error_rate, 0.05) = %v, want 0.05", got)
	}
	// 0-100 percentage is converted to fraction
	if got := NormalizeValue("error_rate", 5.0); got != 0.05 {
		t.Errorf("NormalizeValue(error_rate, 5.0) = %v, want 0.05", got)
	}
}

func TestNormalizeValue_Passthrough(t *testing.T) {
	if got := NormalizeValue("stress", 42.5); got != 42.5 {
		t.Errorf("NormalizeValue(stress, 42.5) = %v, want 42.5", got)
	}
}

// ── direct scraper tests ──────────────────────────────────────────────────────

func TestScrapeDirect_ParsesPrometheusText(t *testing.T) {
	body := `# HELP cpu_percent CPU usage percent
# TYPE cpu_percent gauge
cpu_percent 42.5
# HELP memory_usage_percent Memory usage percent
# TYPE memory_usage_percent gauge
memory_usage_percent 70.1
# HELP unknown_xyzzy Unknown metric
unknown_xyzzy 1.0
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	cfg := &DatasourceConfig{
		Type:        TypeDirect,
		URL:         srv.URL + "/metrics",
		WorkloadKey: "production/Deployment/api",
	}

	samples, err := scrapeDirect(cfg, &http.Client{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("scrapeDirect failed: %v", err)
	}
	if len(samples) < 2 {
		t.Fatalf("expected >=2 samples, got %d", len(samples))
	}
	for _, s := range samples {
		if s.workload != "production/Deployment/api" {
			t.Errorf("expected workload=production/Deployment/api, got %q", s.workload)
		}
		if s.metric == "" {
			t.Error("empty metric name")
		}
	}
}

func TestScrapeDirect_ConnectionError(t *testing.T) {
	cfg := &DatasourceConfig{
		Type: TypeDirect,
		URL:  "http://127.0.0.1:19999/metrics", // nothing listening
	}
	_, err := scrapeDirect(cfg, &http.Client{Timeout: 100 * time.Millisecond})
	if err == nil {
		t.Error("expected error for unreachable endpoint")
	}
}

func TestScrapeDirect_HTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := &DatasourceConfig{Type: TypeDirect, URL: srv.URL}
	_, err := scrapeDirect(cfg, &http.Client{Timeout: 5 * time.Second})
	if err == nil {
		t.Error("expected error for HTTP 500")
	}
}

// ── parsePrometheusText tests ─────────────────────────────────────────────────

func TestParsePrometheusText_SkipsComments(t *testing.T) {
	body := `# HELP test ignored
# TYPE test gauge
stress 50
`
	samples, err := parsePrometheusText(strings.NewReader(body), "ns/Deployment/svc")
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 1 {
		t.Fatalf("expected 1 sample, got %d", len(samples))
	}
	if samples[0].value != 50 {
		t.Errorf("expected value=50, got %v", samples[0].value)
	}
}

func TestParsePrometheusText_StripsTotalSuffix(t *testing.T) {
	body := `restart_count_total 3
`
	samples, err := parsePrometheusText(strings.NewReader(body), "wl")
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 1 {
		t.Fatalf("expected 1 sample, got %d: %v", len(samples), samples)
	}
	if samples[0].metric != "fatigue" {
		t.Errorf("expected metric=fatigue (from restart_count), got %q", samples[0].metric)
	}
}

func TestParsePrometheusText_HandlesLabels(t *testing.T) {
	body := `cpu_percent{job="api",pod="api-123"} 55.2
`
	samples, err := parsePrometheusText(strings.NewReader(body), "wl")
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 1 {
		t.Fatalf("expected 1 sample, got %d", len(samples))
	}
	if samples[0].metric != "stress" {
		t.Errorf("expected metric=stress, got %q", samples[0].metric)
	}
	if samples[0].value != 55.2 {
		t.Errorf("expected value=55.2, got %v", samples[0].value)
	}
}

// ── workloadFromURL tests ─────────────────────────────────────────────────────

func TestWorkloadFromURL(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"http://payment-api:8080/metrics", "default/Deployment/payment-api"},
		{"http://svc:9090", "default/Deployment/svc"},
		{"https://api.internal:443/metrics", "default/Deployment/api.internal"},
	}
	for _, c := range cases {
		got := workloadFromURL(c.url)
		if got != c.want {
			t.Errorf("workloadFromURL(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

// ── stripPodSuffix tests ──────────────────────────────────────────────────────

func TestStripPodSuffix(t *testing.T) {
	cases := []struct {
		pod  string
		want string
	}{
		{"payment-api-7d6b9f4c8-xk2pq", "payment-api"},
		{"frontend-abc12-xyz99", "frontend"},
		{"simple-app-abc12-xyz99", "simple-app"},
		{"single", "single"},
	}
	for _, c := range cases {
		got := stripPodSuffix(c.pod)
		if got != c.want {
			t.Errorf("stripPodSuffix(%q) = %q, want %q", c.pod, got, c.want)
		}
	}
}

// ── configIntervalDefault tests ───────────────────────────────────────────────

func TestDatasourceConfig_DefaultInterval(t *testing.T) {
	cfg := &DatasourceConfig{}
	if cfg.scrapeInterval() != 30*time.Second {
		t.Errorf("expected 30s default interval, got %v", cfg.scrapeInterval())
	}
}

func TestDatasourceConfig_CustomInterval(t *testing.T) {
	cfg := &DatasourceConfig{ScrapeIntervalSec: 60}
	if cfg.scrapeInterval() != 60*time.Second {
		t.Errorf("expected 60s, got %v", cfg.scrapeInterval())
	}
}

// ── OTLP datasource tests ─────────────────────────────────────────────────────

// TestManager_OTLPNoScrapeLoop verifies that an OTLP datasource does not start
// a scrape goroutine (it is push-based and has no polling loop).
func TestManager_OTLPNoScrapeLoop(t *testing.T) {
	m := New(nil, nil)
	cfg := DatasourceConfig{
		ID:      "otlp-test",
		Type:    TypeOTLP,
		URL:     "http://127.0.0.1:31470",
		Enabled: true,
	}
	m.startDS(&cfg)

	m.mu.RLock()
	state, ok := m.ds["otlp-test"]
	m.mu.RUnlock()

	if !ok {
		t.Fatal("OTLP datasource not registered")
	}
	if state.status != "push-only" {
		t.Errorf("expected status push-only, got %q", state.status)
	}
}

// TestManager_OTLPTest_Unreachable verifies that Test() for an OTLP datasource
// returns an error string when the endpoint is unreachable.
func TestManager_OTLPTest_Unreachable(t *testing.T) {
	m := New(nil, nil)
	cfg := DatasourceConfig{
		Type: TypeOTLP,
		// Use a port that is almost certainly not bound.
		URL: "http://127.0.0.1:19999",
	}
	_, errMsg := m.Test(cfg)
	if errMsg == "" {
		t.Error("expected error for unreachable OTLP endpoint, got none")
	}
}

// TestManager_OTLPTest_Reachable verifies that Test() succeeds when the endpoint accepts TCP.
func TestManager_OTLPTest_Reachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	m := New(nil, nil)
	cfg := DatasourceConfig{
		Type: TypeOTLP,
		URL:  "http://" + srv.Listener.Addr().String(),
	}
	count, errMsg := m.Test(cfg)
	if errMsg != "" {
		t.Errorf("unexpected error: %s", errMsg)
	}
	if count != 0 {
		t.Errorf("expected 0 scraped metrics for OTLP, got %d", count)
	}
}
