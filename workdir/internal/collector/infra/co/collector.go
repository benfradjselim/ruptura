// Package co provides the OpenShift ClusterOperator infra collector.
// It watches cluster-scoped ClusterOperator objects and emits GroupControlPlane signals.
// Probe returns an error on non-OpenShift clusters where the API is absent.
package co

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/benfradjselim/ruptura/internal/collector/infra"
	"github.com/benfradjselim/ruptura/internal/discovery"
	"github.com/benfradjselim/ruptura/pkg/logger"
)

const listPath = "apis/config.openshift.io/v1/clusteroperators"

type coItem struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Status struct {
		Conditions []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"conditions"`
	} `json:"status"`
}

// Collector watches OpenShift ClusterOperator objects and emits GroupControlPlane signals.
type Collector struct {
	apiBase    string
	token      string
	httpClient *http.Client
	mu         sync.RWMutex
	signals    map[string]infra.InfraSignal
}

// New creates a Collector using in-cluster ServiceAccount credentials.
func New() (*Collector, error) {
	apiBase, token, client, err := discovery.InClusterCreds()
	if err != nil {
		return nil, err
	}
	return &Collector{
		apiBase:    apiBase,
		token:      token,
		httpClient: client,
		signals:    make(map[string]infra.InfraSignal),
	}, nil
}

// Name returns the collector identifier.
func (c *Collector) Name() string { return "clusteroperator" }

// Probe performs a single LIST with limit=1. Returns non-nil error on non-OpenShift clusters.
func (c *Collector) Probe(ctx context.Context) error {
	url := fmt.Sprintf("%s/%s?limit=1", c.apiBase, listPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		probeErr := fmt.Errorf("co: probe: %w", err)
		logger.Default.Info("infra collector skipped", "collector", c.Name(), "reason", probeErr.Error())
		return probeErr
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		probeErr := fmt.Errorf("co: probe: status %d (not OpenShift?)", resp.StatusCode)
		logger.Default.Info("infra collector skipped", "collector", c.Name(), "reason", probeErr.Error())
		return probeErr
	}
	logger.Default.Info("infra collector probed", "collector", c.Name())
	return nil
}

// Start runs the perpetual LIST+WATCH loop. Blocks until ctx is cancelled.
func (c *Collector) Start(ctx context.Context) error {
	logger.Default.Info("infra collector started", "collector", c.Name())
	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		if ctx.Err() != nil {
			return nil
		}
		rv, err := c.listAll(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(backoff):
			}
			backoff = minDur(backoff*2, maxBackoff)
			continue
		}
		backoff = time.Second

		_, _ = c.watchStream(ctx, rv)
		if ctx.Err() != nil {
			return nil
		}
	}
}

// Signals returns a concurrent-safe snapshot of all current ClusterOperator signals.
func (c *Collector) Signals() []infra.InfraSignal {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]infra.InfraSignal, 0, len(c.signals))
	for _, s := range c.signals {
		out = append(out, s)
	}
	return out
}

func (c *Collector) listAll(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/%s?limit=500", c.apiBase, listPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("co: list: %w", err)
	}
	var lr struct {
		Metadata struct {
			ResourceVersion string `json:"resourceVersion"`
		} `json:"metadata"`
		Items []coItem `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		resp.Body.Close()
		return "", fmt.Errorf("co: list decode: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("co: list: status %d", resp.StatusCode)
	}
	for i := range lr.Items {
		c.upsert(&lr.Items[i])
	}
	return lr.Metadata.ResourceVersion, nil
}

func (c *Collector) watchStream(ctx context.Context, rv string) (bool, error) {
	url := fmt.Sprintf("%s/%s?watch=true&allowWatchBookmarks=true&resourceVersion=%s&timeoutSeconds=300",
		c.apiBase, listPath, rv)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("co: watch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusGone {
		return true, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("co: watch: status %d", resp.StatusCode)
	}

	type watchEvent struct {
		Type   string          `json:"type"`
		Object json.RawMessage `json:"object"`
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)

	for scanner.Scan() {
		if ctx.Err() != nil {
			return false, nil
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev watchEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		switch ev.Type {
		case "ADDED", "MODIFIED":
			var item coItem
			if err := json.Unmarshal(ev.Object, &item); err == nil && item.Metadata.Name != "" {
				c.upsert(&item)
			}
		case "DELETED":
			var item coItem
			if err := json.Unmarshal(ev.Object, &item); err == nil {
				c.remove(item.Metadata.Name)
			}
		case "ERROR":
			var errObj struct {
				Code int `json:"code"`
			}
			if err := json.Unmarshal(ev.Object, &errObj); err == nil && errObj.Code == http.StatusGone {
				return true, nil
			}
		}
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		return false, fmt.Errorf("co: watch stream: %w", err)
	}
	return false, nil
}

func (c *Collector) upsert(item *coItem) {
	obj := infra.ObjectID{
		Group: infra.GroupControlPlane,
		Scope: infra.ScopeCluster,
		Kind:  "ClusterOperator",
		Name:  item.Metadata.Name,
	}
	sig := coSignal(obj, item)
	c.mu.Lock()
	c.signals[obj.Key()] = sig
	c.mu.Unlock()
}

func (c *Collector) remove(name string) {
	obj := infra.ObjectID{
		Group: infra.GroupControlPlane,
		Scope: infra.ScopeCluster,
		Kind:  "ClusterOperator",
		Name:  name,
	}
	c.mu.Lock()
	delete(c.signals, obj.Key())
	c.mu.Unlock()
}

// coSignal computes a single InfraSignal from a ClusterOperator's condition set.
// Available=False → critical; Degraded=True → critical; Progressing=True → elevated.
func coSignal(obj infra.ObjectID, item *coItem) infra.InfraSignal {
	cond := make(map[string]string, len(item.Status.Conditions))
	for _, c := range item.Status.Conditions {
		cond[c.Type] = c.Status
	}

	val := 0.0
	severity := infra.SeverityStable
	msg := ""

	if cond["Available"] == "False" {
		val, severity, msg = 1.0, infra.SeverityCritical, "cluster operator unavailable"
	}
	if cond["Degraded"] == "True" && 0.8 > val {
		val, severity, msg = 0.8, infra.SeverityCritical, "cluster operator degraded"
	}
	if cond["Progressing"] == "True" && 0.3 > val {
		val, severity, msg = 0.3, infra.SeverityElevated, "cluster operator progressing"
	}

	return infra.InfraSignal{
		Object:    obj,
		Signal:    "operatorHealth",
		Value:     val,
		Severity:  severity,
		Message:   msg,
		Timestamp: time.Now(),
	}
}

func minDur(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
