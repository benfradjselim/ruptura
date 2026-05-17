package scraper

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// scrapeDirect fetches a Prometheus text-format /metrics endpoint and returns samples.
// The workload key is taken from cfg.WorkloadKey. If empty, it defaults to
// "default/Deployment/<hostname-from-url>".
func scrapeDirect(cfg *DatasourceConfig, client *http.Client) ([]promSample, error) {
	resp, err := client.Get(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", cfg.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, cfg.URL)
	}

	wl := cfg.WorkloadKey
	if wl == "" {
		wl = workloadFromURL(cfg.URL)
	}

	return parsePrometheusText(resp.Body, wl)
}

// parsePrometheusText parses Prometheus exposition format (text/plain) line by line.
// It ignores HELP and TYPE lines. For each metric line, it maps the name to a KPI.
func parsePrometheusText(r io.Reader, workload string) ([]promSample, error) {
	var out []promSample
	now := time.Now()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, value, ok := parseLine(line)
		if !ok {
			continue
		}
		kpi := MapMetric(name)
		if kpi == "" {
			continue
		}
		out = append(out, promSample{
			workload: workload,
			metric:   kpi,
			value:    NormalizeValue(kpi, value),
			ts:       now,
		})
	}
	return out, scanner.Err()
}

// parseLine parses a single Prometheus exposition format metric line.
// Format: metric_name[{labels}] value [timestamp]
// Returns (name, value, ok).
func parseLine(line string) (string, float64, bool) {
	// strip trailing timestamp if present
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", 0, false
	}
	nameWithLabels := fields[0]
	valueStr := fields[1]

	// strip labels: "metric_name{...}" → "metric_name"
	name := nameWithLabels
	if idx := strings.IndexByte(nameWithLabels, '{'); idx >= 0 {
		name = nameWithLabels[:idx]
	}

	// skip histogram/summary suffixes
	for _, suffix := range []string{"_bucket", "_count", "_sum", "_created", "_total"} {
		if strings.HasSuffix(name, suffix) {
			name = strings.TrimSuffix(name, suffix)
			break
		}
	}

	val, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return "", 0, false
	}
	return name, val, true
}

// workloadFromURL extracts a workload key from a scrape URL.
// "http://payment-api:8080/metrics" → "default/Deployment/payment-api"
func workloadFromURL(rawURL string) string {
	// strip scheme
	s := rawURL
	if idx := strings.Index(s, "://"); idx >= 0 {
		s = s[idx+3:]
	}
	// strip path and port
	if idx := strings.IndexByte(s, '/'); idx >= 0 {
		s = s[:idx]
	}
	if idx := strings.LastIndexByte(s, ':'); idx >= 0 {
		s = s[:idx]
	}
	if s == "" {
		s = "unknown"
	}
	return "default/Deployment/" + s
}
