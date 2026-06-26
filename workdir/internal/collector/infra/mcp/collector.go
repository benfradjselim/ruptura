// Package mcp provides the OpenShift MachineConfigPool infra collector.
// It watches cluster-scoped MachineConfigPool objects and emits GroupControlPlane signals.
// Probe returns an error on non-OpenShift clusters where the API is absent.
package mcp

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
)

const listPath = "apis/machineconfiguration.openshift.io/v1/machineconfigpools"

type mcpItem struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Status struct {
		MachineCount            int `json:"machineCount"`
		DegradedMachineCount    int `json:"degradedMachineCount"`
		UnavailableMachineCount int `json:"unavailableMachineCount"`
		UpdatedMachineCount     int `json:"updatedMachineCount"`
	} `json:"status"`
}

// Collector watches OpenShift MachineConfigPool objects and emits GroupControlPlane signals.
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
func (c *Collector) Name() string { return "machineconfigpool" }

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
		return fmt.Errorf("mcp: probe: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("mcp: probe: status %d (not OpenShift?)", resp.StatusCode)
	}
	return nil
}

// Start runs the perpetual LIST+WATCH loop. Blocks until ctx is cancelled.
func (c *Collector) Start(ctx context.Context) error {
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

// Signals returns a concurrent-safe snapshot of all current MachineConfigPool signals.
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
		return "", fmt.Errorf("mcp: list: %w", err)
	}
	var lr struct {
		Metadata struct {
			ResourceVersion string `json:"resourceVersion"`
		} `json:"metadata"`
		Items []mcpItem `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		resp.Body.Close()
		return "", fmt.Errorf("mcp: list decode: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("mcp: list: status %d", resp.StatusCode)
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
		return false, fmt.Errorf("mcp: watch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusGone {
		return true, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("mcp: watch: status %d", resp.StatusCode)
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
			var item mcpItem
			if err := json.Unmarshal(ev.Object, &item); err == nil && item.Metadata.Name != "" {
				c.upsert(&item)
			}
		case "DELETED":
			var item mcpItem
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
		return false, fmt.Errorf("mcp: watch stream: %w", err)
	}
	return false, nil
}

func (c *Collector) upsert(item *mcpItem) {
	obj := infra.ObjectID{
		Group: infra.GroupControlPlane,
		Scope: infra.ScopeCluster,
		Kind:  "MachineConfigPool",
		Name:  item.Metadata.Name,
	}
	sig := mcpSignal(obj, item)
	c.mu.Lock()
	c.signals[obj.Key()] = sig
	c.mu.Unlock()
}

func (c *Collector) remove(name string) {
	obj := infra.ObjectID{
		Group: infra.GroupControlPlane,
		Scope: infra.ScopeCluster,
		Kind:  "MachineConfigPool",
		Name:  name,
	}
	c.mu.Lock()
	delete(c.signals, obj.Key())
	c.mu.Unlock()
}

// mcpSignal computes a single InfraSignal from a MachineConfigPool's status.
// Degraded machines → critical; unavailable → warning; updating → elevated.
func mcpSignal(obj infra.ObjectID, item *mcpItem) infra.InfraSignal {
	total := item.Status.MachineCount
	if total == 0 {
		total = 1 // avoid division by zero for empty pools
	}

	val := 0.0
	severity := infra.SeverityStable
	msg := ""

	if item.Status.DegradedMachineCount > 0 {
		ratio := float64(item.Status.DegradedMachineCount) / float64(total)
		if ratio < 0.4 {
			ratio = 0.4
		}
		val, severity, msg = ratio, infra.SeverityCritical,
			fmt.Sprintf("%d/%d machines degraded", item.Status.DegradedMachineCount, item.Status.MachineCount)
	} else if item.Status.UnavailableMachineCount > 0 {
		ratio := float64(item.Status.UnavailableMachineCount) / float64(total)
		if ratio < 0.3 {
			ratio = 0.3
		}
		if ratio > 0.8 {
			ratio = 0.8
		}
		val, severity, msg = ratio, infra.SeverityWarning,
			fmt.Sprintf("%d/%d machines unavailable", item.Status.UnavailableMachineCount, item.Status.MachineCount)
	} else if item.Status.UpdatedMachineCount < item.Status.MachineCount {
		val, severity, msg = 0.3, infra.SeverityElevated,
			fmt.Sprintf("%d/%d machines updating", item.Status.MachineCount-item.Status.UpdatedMachineCount, item.Status.MachineCount)
	}

	return infra.InfraSignal{
		Object:    obj,
		Signal:    "mcpHealth",
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
