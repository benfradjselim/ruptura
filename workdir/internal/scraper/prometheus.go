package scraper

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// promSample is a single metric sample from the Prometheus HTTP API.
type promSample struct {
	workload string
	metric   string
	value    float64
	ts       time.Time
}

// scrapePrometheus queries a Prometheus server's HTTP API and returns
// metric samples mapped to Ruptura workload keys.
//
// It queries a standard set of PromQL expressions that cover the 10 KPI signals:
//   - rate(container_cpu_usage_seconds_total[2m]) / limits → stress
//   - container_memory_working_set_bytes / limits          → pressure
//   - kube_pod_container_status_restarts_total             → fatigue
//   - rate(http_requests_total{status=~"5.."}[2m])         → mood
//   - rate(http_requests_total[2m])                        → velocity
//   - process_open_fds / limits                            → entropy
func scrapePrometheus(cfg *DatasourceConfig, client *http.Client) ([]promSample, error) {
	base := strings.TrimRight(cfg.URL, "/")
	ns := cfg.Namespace

	queries := promQueries(ns)
	var all []promSample

	for kpi, expr := range queries {
		samples, err := runInstantQuery(base, expr, kpi, client)
		if err != nil {
			continue // partial failure: skip bad query, keep going
		}
		all = append(all, samples...)
	}
	return all, nil
}

// promQueries returns a map of kpi→PromQL expression.
func promQueries(namespace string) map[string]string {
	nsFilter := ""
	if namespace != "" {
		nsFilter = fmt.Sprintf(`namespace="%s",`, namespace)
	}

	return map[string]string{
		// CPU utilisation as % of 1 core — prefer "% of request" when kube-state-metrics
		// resource requests are available; fall back to raw core-seconds/s × 100 otherwise.
		// The `or` clause ensures non-zero values even on clusters without kube-state-metrics.
		"stress": fmt.Sprintf(
			`100 * sum(rate(container_cpu_usage_seconds_total{%scontainer!="",container!="POD"}[2m])) by (namespace,pod) / on(namespace,pod) group_left() sum(kube_pod_container_resource_requests{%sresource="cpu",container!=""}) by (namespace,pod) `+
				`or 100 * sum(rate(container_cpu_usage_seconds_total{%scontainer!="",container!="POD"}[2m])) by (namespace,pod)`,
			nsFilter, nsFilter, nsFilter,
		),
		// Memory utilisation — prefer "% of request"; fall back to working-set MiB × 100.
		"pressure": fmt.Sprintf(
			`100 * sum(container_memory_working_set_bytes{%scontainer!="",container!="POD"}) by (namespace,pod) / on(namespace,pod) group_left() sum(kube_pod_container_resource_requests{%sresource="memory",container!=""}) by (namespace,pod) `+
				`or sum(container_memory_working_set_bytes{%scontainer!="",container!="POD"}) by (namespace,pod) / 1048576`,
			nsFilter, nsFilter, nsFilter,
		),
		// Container restart count (monotonic — pipeline tracks delta)
		"fatigue": fmt.Sprintf(
			`sum(kube_pod_container_status_restarts_total{%scontainer!=""}) by (namespace,pod)`,
			nsFilter,
		),
		// HTTP 5xx error rate as percentage
		"mood": fmt.Sprintf(
			`100 * sum(rate(http_requests_total{%scode=~"5.."}[2m])) by (namespace,pod) / (sum(rate(http_requests_total{%s}[2m])) by (namespace,pod) + 1)`,
			nsFilter, nsFilter,
		),
		// HTTP request rate
		"velocity": fmt.Sprintf(
			`sum(rate(http_requests_total{%s}[2m])) by (namespace,pod)`,
			nsFilter,
		),
		// Goroutine count → entropy
		"entropy": fmt.Sprintf(
			`sum(go_goroutines{%s}) by (namespace,pod)`,
			nsFilter,
		),
		// Active connections → humidity
		"humidity": fmt.Sprintf(
			`sum(process_open_fds{%s}) by (namespace,pod)`,
			nsFilter,
		),
	}
}

// promQueryResponse is the envelope from GET /api/v1/query.
type promQueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  [2]interface{}    `json:"value"` // [timestamp, "valueStr"]
		} `json:"result"`
	} `json:"data"`
}

func runInstantQuery(base, expr, kpi string, client *http.Client) ([]promSample, error) {
	u := fmt.Sprintf("%s/api/v1/query?query=%s&time=%d",
		base,
		url.QueryEscape(expr),
		time.Now().Unix(),
	)
	resp, err := client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB max
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prometheus query HTTP %d: %s", resp.StatusCode, string(body)[:min(200, len(body))])
	}

	var qr promQueryResponse
	if err := json.Unmarshal(body, &qr); err != nil {
		return nil, err
	}
	if qr.Status != "success" {
		return nil, fmt.Errorf("prometheus query status: %s", qr.Status)
	}

	var out []promSample
	for _, row := range qr.Data.Result {
		wl := workloadFromLabels(row.Metric)
		if wl == "" {
			continue
		}
		valStr, ok := row.Value[1].(string)
		if !ok {
			continue
		}
		var val float64
		if _, err := fmt.Sscanf(valStr, "%f", &val); err != nil {
			continue
		}
		out = append(out, promSample{
			workload: wl,
			metric:   kpi,
			value:    NormalizeValue(kpi, val),
			ts:       time.Now(),
		})
	}
	return out, nil
}

// workloadFromLabels builds a "namespace/kind/name" key from Prometheus labels.
// Tries pod → deployment → job → service name in order.
func workloadFromLabels(labels map[string]string) string {
	ns := labels["namespace"]
	if ns == "" {
		ns = labels["exported_namespace"]
	}
	if ns == "" {
		ns = "default"
	}

	// Try to infer workload name: pod name often encodes the deployment
	pod := labels["pod"]
	name := labels["deployment"]
	kind := "Deployment"

	if name == "" && pod != "" {
		// Strip the last two segments of a pod name (replicaset hash + pod hash)
		// e.g. "payment-api-7d6b9f4c8-xk2pq" → "payment-api"
		name = stripPodSuffix(pod)
	}
	if name == "" {
		name = labels["job"]
		kind = "Job"
	}
	if name == "" {
		name = labels["service"]
		kind = "Service"
	}
	if name == "" {
		return ""
	}
	return ns + "/" + kind + "/" + name
}

// stripPodSuffix removes the replicaset hash and pod-id suffix from a pod name.
// "payment-api-7d6b9f4c8-xk2pq" → "payment-api"
func stripPodSuffix(pod string) string {
	parts := strings.Split(pod, "-")
	// Last part is random alphanumeric pod ID (5 chars)
	// Second-to-last is replicaset hash (typically 9-10 hex chars)
	// Everything before is the deployment name
	if len(parts) >= 3 {
		return strings.Join(parts[:len(parts)-2], "-")
	}
	if len(parts) >= 2 {
		return strings.Join(parts[:len(parts)-1], "-")
	}
	return pod
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
