package scraper

import "strings"

// metricToKPI maps raw Prometheus metric names to Ruptura KPI names.
// The mapping tries common naming conventions from kube-state-metrics,
// cAdvisor, and application-level metrics.
var metricToKPI = map[string]string{
	// CPU → stress
	"cpu_usage_percent":                 "stress",
	"cpu_percent":                       "stress",
	"container_cpu_usage_ratio":         "stress",
	"process_cpu_percent":               "stress",

	// Memory → pressure
	"memory_usage_percent":              "pressure",
	"mem_percent":                       "pressure",
	"container_memory_usage_ratio":      "pressure",
	"process_memory_percent":            "pressure",

	// Error rate → mood (inverted: high errors = stress on mood)
	"http_error_rate":                   "mood",
	"error_rate":                        "mood",
	"grpc_error_rate":                   "mood",

	// Restarts / failures → fatigue
	"restart_count":                     "fatigue",
	"container_restarts":                "fatigue",
	"pod_restarts":                      "fatigue",
	"process_restarts":                  "fatigue",

	// Request rate / velocity
	"request_rate":                      "velocity",
	"http_rps":                          "velocity",
	"rps":                               "velocity",
	"throughput_rps":                    "throughput",
	"requests_per_second":               "velocity",

	// Throughput (bytes/operations)
	"bytes_processed":                   "throughput",
	"ops_per_second":                    "throughput",

	// Humidity / queue depth / saturation
	"queue_depth":                       "humidity",
	"queue_size":                        "humidity",
	"saturation":                        "humidity",
	"goroutine_count":                   "entropy",
	"thread_count":                      "entropy",
	"open_fds":                          "entropy",

	// Resilience / recovery
	"circuit_breaker_state":             "resilience",
	"retry_rate":                        "resilience",

	// Health score pass-through
	"health_score":                      "health_score",
	"ruptura_health_score":              "health_score",

	// Direct KPI names pass through
	"stress":     "stress",
	"fatigue":    "fatigue",
	"mood":       "mood",
	"pressure":   "pressure",
	"humidity":   "humidity",
	"contagion":  "contagion",
	"resilience": "resilience",
	"entropy":    "entropy",
	"velocity":   "velocity",
	"throughput": "throughput",
}

// knownKPIs is the full set of KPI names accepted by the pipeline.
var knownKPIs = map[string]bool{
	"stress": true, "fatigue": true, "mood": true, "pressure": true,
	"humidity": true, "contagion": true, "resilience": true,
	"entropy": true, "velocity": true, "throughput": true, "health_score": true,
}

// MapMetric returns the KPI name for a raw metric name, or "" if unmapped.
// It tries exact match first, then a case-insensitive prefix/suffix scan.
func MapMetric(name string) string {
	if kpi, ok := metricToKPI[name]; ok {
		return kpi
	}
	lower := strings.ToLower(name)
	// Substring match for common patterns
	switch {
	case strings.Contains(lower, "cpu"):
		return "stress"
	case strings.Contains(lower, "mem") || strings.Contains(lower, "memory"):
		return "pressure"
	case strings.Contains(lower, "restart"):
		return "fatigue"
	case strings.Contains(lower, "error") && (strings.Contains(lower, "rate") || strings.HasSuffix(lower, "errors")):
		return "mood"
	case strings.Contains(lower, "goroutine") || strings.Contains(lower, "thread"):
		return "entropy"
	case strings.Contains(lower, "queue") || strings.Contains(lower, "pending"):
		return "humidity"
	case strings.Contains(lower, "request") && strings.Contains(lower, "rate"):
		return "velocity"
	case strings.Contains(lower, "throughput") || strings.Contains(lower, "bytes_out"):
		return "throughput"
	case strings.Contains(lower, "health"):
		return "health_score"
	}
	// If the metric name is itself a known KPI, pass it through
	if knownKPIs[lower] {
		return lower
	}
	return ""
}

// NormalizeValue converts a raw metric value to a 0–100 range where needed.
// Most values are passed through as-is; the pipeline handles normalization.
func NormalizeValue(kpi string, raw float64) float64 {
	switch kpi {
	case "mood":
		// mood = 1 - error_rate (error_rate is 0–1, mood is 0–100)
		if raw <= 1.0 {
			return (1.0 - raw) * 100
		}
		// error_rate expressed as percentage (0–100) → convert
		return 100 - raw
	default:
		return raw
	}
}
