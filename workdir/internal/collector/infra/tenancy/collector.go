// Package tenancy provides the tenancy-layer infra collector.
// It watches ResourceQuotas, LimitRanges, and Namespaces,
// emitting GroupTenancy signals.
package tenancy

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/benfradjselim/ruptura/internal/collector/infra"
	"github.com/benfradjselim/ruptura/internal/discovery"
)

type k8sMeta struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
}

// Collector watches ResourceQuota, LimitRange, and Namespace objects,
// emitting GroupTenancy signals.
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

func (c *Collector) Name() string { return "tenancy" }

// Probe verifies ResourceQuota API is reachable (always present in k8s).
func (c *Collector) Probe(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/v1/resourcequotas?limit=1", c.apiBase)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("tenancy: probe: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tenancy: probe: status %d", resp.StatusCode)
	}
	return nil
}

// Start launches watch goroutines for ResourceQuota, LimitRange, and Namespace.
func (c *Collector) Start(ctx context.Context) error {
	go c.watchLoop(ctx, "api/v1/resourcequotas", "ResourceQuota")
	go c.watchLoop(ctx, "api/v1/limitranges", "LimitRange")
	go c.watchLoop(ctx, "api/v1/namespaces", "Namespace")
	<-ctx.Done()
	return nil
}

// Signals returns a concurrent-safe snapshot of all current tenancy signals.
func (c *Collector) Signals() []infra.InfraSignal {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]infra.InfraSignal, 0, len(c.signals))
	for _, s := range c.signals {
		out = append(out, s)
	}
	return out
}

func (c *Collector) watchLoop(ctx context.Context, path, kind string) {
	backoff := time.Second
	const maxBackoff = 30 * time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		rv, err := c.listAll(ctx, path, kind)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff = minDur(backoff*2, maxBackoff)
			continue
		}
		backoff = time.Second
		_, _ = c.watchStream(ctx, path, kind, rv)
		if ctx.Err() != nil {
			return
		}
	}
}

func (c *Collector) listAll(ctx context.Context, path, kind string) (string, error) {
	url := fmt.Sprintf("%s/%s?limit=500", c.apiBase, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("tenancy: list %s: %w", kind, err)
	}
	var lr struct {
		Metadata struct {
			ResourceVersion string `json:"resourceVersion"`
		} `json:"metadata"`
		Items []json.RawMessage `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		resp.Body.Close()
		return "", fmt.Errorf("tenancy: list %s decode: %w", kind, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("tenancy: list %s: status %d", kind, resp.StatusCode)
	}
	now := time.Now()
	for _, raw := range lr.Items {
		var meta k8sMeta
		if err := json.Unmarshal(raw, &meta); err != nil || meta.Metadata.Name == "" {
			continue
		}
		c.upsert(kind, meta.Metadata.Name, meta.Metadata.Namespace, raw, now)
	}
	return lr.Metadata.ResourceVersion, nil
}

func (c *Collector) watchStream(ctx context.Context, path, kind, rv string) (bool, error) {
	url := fmt.Sprintf("%s/%s?watch=true&allowWatchBookmarks=true&resourceVersion=%s&timeoutSeconds=300",
		c.apiBase, path, rv)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("tenancy: watch %s: %w", kind, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusGone {
		return true, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("tenancy: watch %s: status %d", kind, resp.StatusCode)
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
		now := time.Now()
		switch ev.Type {
		case "ADDED", "MODIFIED":
			var meta k8sMeta
			if err := json.Unmarshal(ev.Object, &meta); err == nil && meta.Metadata.Name != "" {
				c.upsert(kind, meta.Metadata.Name, meta.Metadata.Namespace, ev.Object, now)
			}
		case "DELETED":
			var meta k8sMeta
			if err := json.Unmarshal(ev.Object, &meta); err == nil {
				c.remove(kind, meta.Metadata.Name, meta.Metadata.Namespace)
			}
		case "ERROR":
			var errObj struct{ Code int `json:"code"` }
			if err := json.Unmarshal(ev.Object, &errObj); err == nil && errObj.Code == http.StatusGone {
				return true, nil
			}
		}
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		return false, fmt.Errorf("tenancy: watch %s stream: %w", kind, err)
	}
	return false, nil
}

func (c *Collector) upsert(kind, name, namespace string, rawObj json.RawMessage, now time.Time) {
	// Namespace objects are cluster-scoped but describe a namespace; use their
	// name as the Namespace field so CGPM can scope them correctly.
	ns := namespace
	scope := infra.ScopeNamespace
	if kind == "Namespace" {
		ns = name // the object IS the namespace
	}
	if ns == "" {
		scope = infra.ScopeCluster
	}
	obj := infra.ObjectID{
		Group:     infra.GroupTenancy,
		Scope:     scope,
		Namespace: ns,
		Kind:      kind,
		Name:      name,
	}
	c.mu.Lock()
	c.signals[obj.Key()] = tenancySignal(obj, kind, rawObj, now)
	c.mu.Unlock()
}

func (c *Collector) remove(kind, name, namespace string) {
	ns := namespace
	scope := infra.ScopeNamespace
	if kind == "Namespace" {
		ns = name
	}
	if ns == "" {
		scope = infra.ScopeCluster
	}
	obj := infra.ObjectID{
		Group:     infra.GroupTenancy,
		Scope:     scope,
		Namespace: ns,
		Kind:      kind,
		Name:      name,
	}
	c.mu.Lock()
	delete(c.signals, obj.Key())
	c.mu.Unlock()
}

func tenancySignal(obj infra.ObjectID, kind string, rawObj json.RawMessage, now time.Time) infra.InfraSignal {
	switch kind {
	case "ResourceQuota":
		return resourceQuotaSignal(obj, rawObj, now)
	case "Namespace":
		return namespaceSignal(obj, rawObj, now)
	default:
		// LimitRange — presence is healthy governance.
		return infra.InfraSignal{
			Object: obj, Signal: "tenancyPresence",
			Value: 0.0, Severity: infra.SeverityStable, Timestamp: now,
		}
	}
}

// resourceQuotaSignal computes a signal based on the maximum utilisation ratio
// across all quota resources that have parseable numeric values (pods, counts).
// Resources with units (Gi, m) are silently skipped.
func resourceQuotaSignal(obj infra.ObjectID, rawObj json.RawMessage, now time.Time) infra.InfraSignal {
	var rq struct {
		Status struct {
			Hard map[string]string `json:"hard"`
			Used map[string]string `json:"used"`
		} `json:"status"`
	}
	_ = json.Unmarshal(rawObj, &rq)

	maxRatio := 0.0
	for res, hardStr := range rq.Status.Hard {
		usedStr, ok := rq.Status.Used[res]
		if !ok {
			continue
		}
		hard, err1 := strconv.ParseFloat(hardStr, 64)
		used, err2 := strconv.ParseFloat(usedStr, 64)
		if err1 != nil || err2 != nil || hard <= 0 {
			continue
		}
		if r := used / hard; r > maxRatio {
			maxRatio = r
		}
	}

	switch {
	case maxRatio >= 0.95:
		return infra.InfraSignal{
			Object: obj, Signal: "quotaUtilization",
			Value: 0.8, Severity: infra.SeverityWarning,
			Message: fmt.Sprintf("quota at %.0f%% capacity", maxRatio*100),
			Timestamp: now,
		}
	case maxRatio >= 0.85:
		return infra.InfraSignal{
			Object: obj, Signal: "quotaUtilization",
			Value: 0.5, Severity: infra.SeverityElevated,
			Message: fmt.Sprintf("quota at %.0f%% capacity", maxRatio*100),
			Timestamp: now,
		}
	default:
		return infra.InfraSignal{
			Object: obj, Signal: "quotaUtilization",
			Value: 0.0, Severity: infra.SeverityStable, Timestamp: now,
		}
	}
}

func namespaceSignal(obj infra.ObjectID, rawObj json.RawMessage, now time.Time) infra.InfraSignal {
	var ns struct {
		Status struct {
			Phase string `json:"phase"`
		} `json:"status"`
	}
	_ = json.Unmarshal(rawObj, &ns)

	if ns.Status.Phase == "Terminating" {
		return infra.InfraSignal{
			Object: obj, Signal: "namespacePhase",
			Value: 0.6, Severity: infra.SeverityWarning,
			Message: "namespace terminating", Timestamp: now,
		}
	}
	return infra.InfraSignal{
		Object: obj, Signal: "namespacePhase",
		Value: 0.0, Severity: infra.SeverityStable, Timestamp: now,
	}
}

func minDur(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
