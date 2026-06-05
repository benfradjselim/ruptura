package scraper

import "strings"

// metricToKPI maps raw Prometheus metric names to Ruptura KPI names.
// The mapping tries common naming conventions from kube-state-metrics,
// cAdvisor, and application-level metrics.
var metricToKPI = map[string]string{
	// CPU → stress
	"cpu_usage_percent":            "stress",
	"cpu_percent":                  "stress",
	"container_cpu_usage_ratio":    "stress",
	"process_cpu_percent":          "stress",

	// Memory → pressure
	"memory_usage_percent":         "pressure",
	"mem_percent":                  "pressure",
	"container_memory_usage_ratio": "pressure",
	"process_memory_percent":       "pressure",

	// Error rate → error_rate (0-1 fraction; feeds mood formula in analyzer)
	"http_error_rate": "error_rate",
	"grpc_error_rate": "error_rate",

	// Restarts / failures → fatigue
	"restart_count":    "fatigue",
	"container_restarts": "fatigue",
	"pod_restarts":     "fatigue",
	"process_restarts": "fatigue",

	// HTTP request rate → request_rate (feeds mood numerator)
	"http_rps":            "request_rate",
	"rps":                 "request_rate",
	"throughput_rps":      "throughput",
	"requests_per_second": "request_rate",

	// Throughput (bytes/operations)
	"bytes_processed": "throughput",
	"ops_per_second":  "throughput",

	// Humidity / queue depth / saturation
	"queue_depth":    "humidity",
	"queue_size":     "humidity",
	"saturation":     "humidity",
	"goroutine_count": "entropy",
	"thread_count":   "entropy",
	"open_fds":       "entropy",

	// Resilience / recovery
	"circuit_breaker_state": "resilience",
	"retry_rate":            "resilience",

	// Process uptime → feeds mood numerator
	"process_uptime_seconds": "uptime_seconds",

	// Health score pass-through
	"health_score":        "health_score",
	"ruptura_health_score": "health_score",

	// Direct KPI names pass through
	"stress":          "stress",
	"fatigue":         "fatigue",
	"mood":            "mood",
	"pressure":        "pressure",
	"humidity":        "humidity",
	"contagion":       "contagion",
	"resilience":      "resilience",
	"entropy":         "entropy",
	"velocity":        "velocity",
	"throughput":      "throughput",
	"error_rate":      "error_rate",
	"request_rate":    "request_rate",
	"uptime_seconds":  "uptime_seconds",
}

// knownKPIs is the full set of KPI names accepted by the pipeline.
var knownKPIs = map[string]bool{
	"stress": true, "fatigue": true, "mood": true, "pressure": true,
	"humidity": true, "contagion": true, "resilience": true,
	"entropy": true, "velocity": true, "throughput": true, "health_score": true,
	"error_rate": true, "request_rate": true, "uptime_seconds": true,
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
		return "error_rate"
	case strings.Contains(lower, "goroutine") || strings.Contains(lower, "thread"):
		return "entropy"
	case strings.Contains(lower, "queue") || strings.Contains(lower, "pending"):
		return "humidity"
	case strings.Contains(lower, "request") && strings.Contains(lower, "rate"):
		return "request_rate"
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

// NormalizeValue converts a raw metric value for a given KPI before it enters
// the analysis pipeline. Most values pass through unchanged.
func NormalizeValue(kpi string, raw float64) float64 {
	switch kpi {
	case "error_rate":
		// Prometheus query returns 0-1 fraction; external sources may send 0-100 percentage.
		if raw > 1.0 {
			return raw / 100.0
		}
		return raw
	default:
		return raw
	}
}
