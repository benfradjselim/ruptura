// Package k8smetrics polls the Kubernetes metrics-server API (metrics.k8s.io/v1beta1)
// and injects pod CPU/memory signals into the Ruptura pipeline.
//
// This path is active only when metrics-server is installed in the cluster.
// It complements the Prometheus scraper by providing direct CPU/memory readings
// without depending on cAdvisor or kube-state-metrics.
package k8smetrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	pipelinemetrics "github.com/benfradjselim/ruptura/internal/pipeline/metrics"
	"github.com/benfradjselim/ruptura/pkg/logger"
)

// MetricPipeline is the subset of pipelinemetrics.Engine needed by Poller.
type MetricPipeline interface {
	Ingest(host, metric string, value float64, ts time.Time)
}

// Poller polls the k8s metrics API and injects stress/pressure into the pipeline.
type Poller struct {
	apiBase    string
	token      string
	client     *http.Client
	pipeline   MetricPipeline
	interval   time.Duration
	namespaces []string // empty = all namespaces
}

// New creates a Poller using the pod's in-cluster ServiceAccount credentials.
// Returns a non-nil error only when not running inside a k8s pod.
// If metrics-server is unavailable, the poller starts anyway and logs errors
// per cycle at Debug level — it will pick up metrics as soon as metrics-server
// becomes ready.
func New(pipeline MetricPipeline, interval time.Duration, namespaces ...string) (*Poller, error) {
	apiBase, token, client, err := inClusterCreds()
	if err != nil {
		return nil, err
	}
	return &Poller{
		apiBase:    apiBase,
		token:      token,
		client:     client,
		pipeline:   pipeline,
		interval:   interval,
		namespaces: namespaces,
	}, nil
}

// Run starts the polling loop. Blocks until ctx is cancelled.
func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.poll(); err != nil {
				logger.Default.Debug("k8smetrics: poll error", "err", err)
			}
		}
	}
}

func (p *Poller) poll() error {
	namespaces := p.namespaces
	if len(namespaces) == 0 {
		all, err := p.listNamespaces()
		if err != nil {
			return err
		}
		namespaces = all
	}
	now := time.Now()
	for _, ns := range namespaces {
		pods, err := p.listPodMetrics(ns)
		if err != nil {
			continue
		}
		for _, pod := range pods.Items {
			workload := workloadKey(pod.Metadata.Namespace, pod.Metadata.Name)
			if workload == "" {
				continue
			}
			var totalCPUm, totalMemBytes int64
			for _, c := range pod.Containers {
				totalCPUm += parseCPU(c.Usage.CPU)
				totalMemBytes += parseMem(c.Usage.Memory)
			}
			// stress: CPU millicores as a 0–1 fraction (1 core = 1.0)
			stress := float64(totalCPUm) / 1000.0
			// pressure: memory in MiB (raw, pipeline normalises)
			pressureMiB := float64(totalMemBytes) / (1024 * 1024)
			p.pipeline.Ingest(workload, "stress", stress, now)
			p.pipeline.Ingest(workload, "pressure", pressureMiB, now)
		}
	}
	return nil
}

// podMetricsList is the deserialized metrics.k8s.io/v1beta1 PodMetricsList.
type podMetricsList struct {
	Items []struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Containers []struct {
			Name  string `json:"name"`
			Usage struct {
				CPU    string `json:"cpu"`
				Memory string `json:"memory"`
			} `json:"usage"`
		} `json:"containers"`
	} `json:"items"`
}

func (p *Poller) listPodMetrics(ns string) (*podMetricsList, error) {
	url := fmt.Sprintf("%s/apis/metrics.k8s.io/v1beta1/namespaces/%s/pods", p.apiBase, ns)
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+p.token)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("metrics API HTTP %d: %s", resp.StatusCode, body)
	}
	var out podMetricsList
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

type nsList struct {
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
	} `json:"items"`
}

func (p *Poller) listNamespaces() ([]string, error) {
	req, _ := http.NewRequest(http.MethodGet, p.apiBase+"/api/v1/namespaces", nil)
	req.Header.Set("Authorization", "Bearer "+p.token)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out nsList
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(out.Items))
	for _, item := range out.Items {
		names = append(names, item.Metadata.Name)
	}
	return names, nil
}

// workloadKey derives "namespace/Deployment/name" from a pod name.
// Strips the last two dash-separated segments (replicaset hash + pod ID).
func workloadKey(ns, pod string) string {
	parts := strings.Split(pod, "-")
	var name string
	switch {
	case len(parts) >= 3:
		name = strings.Join(parts[:len(parts)-2], "-")
	case len(parts) >= 2:
		name = strings.Join(parts[:len(parts)-1], "-")
	default:
		name = pod
	}
	if name == "" || ns == "" {
		return ""
	}
	return ns + "/Deployment/" + name
}

// parseCPU converts a Kubernetes CPU quantity string to millicores.
// Handles "250m" (millicores) and "1" (cores).
func parseCPU(s string) int64 {
	if strings.HasSuffix(s, "m") {
		var m int64
		fmt.Sscanf(s[:len(s)-1], "%d", &m)
		return m
	}
	var cores float64
	fmt.Sscanf(s, "%f", &cores)
	return int64(cores * 1000)
}

// parseMem converts a Kubernetes memory quantity string to bytes.
// Handles Ki, Mi, Gi suffixes.
func parseMem(s string) int64 {
	multipliers := map[string]int64{
		"Ki": 1024,
		"Mi": 1024 * 1024,
		"Gi": 1024 * 1024 * 1024,
		"K":  1000,
		"M":  1000 * 1000,
		"G":  1000 * 1000 * 1000,
	}
	for suffix, mult := range multipliers {
		if strings.HasSuffix(s, suffix) {
			var n int64
			fmt.Sscanf(s[:len(s)-len(suffix)], "%d", &n)
			return n * mult
		}
	}
	var n int64
	fmt.Sscanf(s, "%d", &n)
	return n
}

// Verify MetricPipeline is satisfied by the real engine at compile time.
var _ MetricPipeline = (*pipelinemetrics.Engine)(nil)
