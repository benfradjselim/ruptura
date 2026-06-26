// Package networking provides the network-layer infra collector.
// It watches Services, Endpoints, Routes, NetworkPolicies, and Ingresses,
// emitting GroupNetwork signals per namespace.
package networking

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

// Core network resources — always available in any k8s cluster.
var coreResources = []struct{ path, kind string }{
	{"api/v1/services", "Service"},
	{"api/v1/endpoints", "Endpoints"},
	{"apis/networking.k8s.io/v1/networkpolicies", "NetworkPolicy"},
}

// Optional resources — probed at startup; skipped if the API is absent.
var optionalResources = []struct{ path, kind string }{
	{"apis/route.openshift.io/v1/routes", "Route"},
	{"apis/networking.k8s.io/v1/ingresses", "Ingress"},
}

// k8sMeta is the minimal metadata common to all k8s objects.
type k8sMeta struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
}

// Collector watches multiple network-layer k8s resource types and emits
// GroupNetwork signals per namespace. Optional APIs (Routes, Ingresses) are
// probed at startup and silently skipped when absent.
type Collector struct {
	apiBase    string
	token      string
	httpClient *http.Client
	mu         sync.RWMutex
	signals    map[string]infra.InfraSignal // key: ObjectID.Key()
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
func (c *Collector) Name() string { return "networking" }

// Probe checks that core network APIs are reachable by listing Services.
func (c *Collector) Probe(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/v1/services?limit=1", c.apiBase)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("networking: probe: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("networking: probe: status %d", resp.StatusCode)
	}
	return nil
}

// Start launches one watch goroutine per resource type, then blocks until ctx
// is cancelled. Optional resource types (Routes, Ingresses) are probed first
// and silently skipped when their API group is absent.
func (c *Collector) Start(ctx context.Context) error {
	for _, res := range coreResources {
		go c.watchLoop(ctx, res.path, res.kind)
	}
	for _, res := range optionalResources {
		if err := c.probeAPI(ctx, res.path); err == nil {
			go c.watchLoop(ctx, res.path, res.kind)
		}
	}
	<-ctx.Done()
	return nil
}

// Signals returns a concurrent-safe snapshot of all current network signals.
func (c *Collector) Signals() []infra.InfraSignal {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]infra.InfraSignal, 0, len(c.signals))
	for _, s := range c.signals {
		out = append(out, s)
	}
	return out
}

// probeAPI checks a single API path with limit=1 to detect availability.
func (c *Collector) probeAPI(ctx context.Context, path string) error {
	url := fmt.Sprintf("%s/%s?limit=1", c.apiBase, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

// watchLoop runs the LIST+WATCH loop for a single resource type.
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
		return "", fmt.Errorf("networking: list %s: %w", kind, err)
	}
	var lr struct {
		Metadata struct {
			ResourceVersion string `json:"resourceVersion"`
		} `json:"metadata"`
		Items []json.RawMessage `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		resp.Body.Close()
		return "", fmt.Errorf("networking: list %s decode: %w", kind, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("networking: list %s: status %d", kind, resp.StatusCode)
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
		return false, fmt.Errorf("networking: watch %s: %w", kind, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusGone {
		return true, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("networking: watch %s: status %d", kind, resp.StatusCode)
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
			var errObj struct {
				Code int `json:"code"`
			}
			if err := json.Unmarshal(ev.Object, &errObj); err == nil && errObj.Code == http.StatusGone {
				return true, nil
			}
		}
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		return false, fmt.Errorf("networking: watch %s stream: %w", kind, err)
	}
	return false, nil
}

func (c *Collector) upsert(kind, name, namespace string, rawObj json.RawMessage, now time.Time) {
	obj := infra.ObjectID{
		Group:     infra.GroupNetwork,
		Scope:     infra.ScopeNamespace,
		Namespace: namespace,
		Kind:      kind,
		Name:      name,
	}
	sig := netSignal(obj, kind, rawObj, now)
	c.mu.Lock()
	c.signals[obj.Key()] = sig
	c.mu.Unlock()
}

func (c *Collector) remove(kind, name, namespace string) {
	obj := infra.ObjectID{
		Group:     infra.GroupNetwork,
		Scope:     infra.ScopeNamespace,
		Namespace: namespace,
		Kind:      kind,
		Name:      name,
	}
	c.mu.Lock()
	delete(c.signals, obj.Key())
	c.mu.Unlock()
}

// netSignal dispatches to the correct per-kind signal extractor.
func netSignal(obj infra.ObjectID, kind string, rawObj json.RawMessage, now time.Time) infra.InfraSignal {
	switch kind {
	case "Endpoints":
		return endpointsSignal(obj, rawObj, now)
	case "Route":
		return routeSignal(obj, rawObj, now)
	case "Ingress":
		return ingressSignal(obj, rawObj, now)
	default:
		// Service, NetworkPolicy — track presence only; signal is healthy by default.
		return infra.InfraSignal{
			Object:    obj,
			Signal:    "netPresence",
			Value:     0.0,
			Severity:  infra.SeverityStable,
			Timestamp: now,
		}
	}
}

func endpointsSignal(obj infra.ObjectID, rawObj json.RawMessage, now time.Time) infra.InfraSignal {
	var ep struct {
		Subsets []struct {
			Addresses         []struct{ IP string `json:"ip"` } `json:"addresses"`
			NotReadyAddresses []struct{ IP string `json:"ip"` } `json:"notReadyAddresses"`
		} `json:"subsets"`
	}
	_ = json.Unmarshal(rawObj, &ep)

	ready, notReady := 0, 0
	for _, s := range ep.Subsets {
		ready += len(s.Addresses)
		notReady += len(s.NotReadyAddresses)
	}

	if ready == 0 && notReady > 0 {
		return infra.InfraSignal{
			Object:    obj,
			Signal:    "endpointReady",
			Value:     0.7,
			Severity:  infra.SeverityWarning,
			Message:   "all endpoints not ready",
			Timestamp: now,
		}
	}
	if notReady > ready {
		return infra.InfraSignal{
			Object:    obj,
			Signal:    "endpointReady",
			Value:     0.4,
			Severity:  infra.SeverityElevated,
			Message:   "majority endpoints not ready",
			Timestamp: now,
		}
	}
	return infra.InfraSignal{
		Object: obj, Signal: "endpointReady",
		Value: 0.0, Severity: infra.SeverityStable, Timestamp: now,
	}
}

func routeSignal(obj infra.ObjectID, rawObj json.RawMessage, now time.Time) infra.InfraSignal {
	var route struct {
		Status struct {
			Ingress []struct {
				Conditions []struct {
					Type   string `json:"type"`
					Status string `json:"status"`
				} `json:"conditions"`
			} `json:"ingress"`
		} `json:"status"`
	}
	_ = json.Unmarshal(rawObj, &route)

	if len(route.Status.Ingress) == 0 {
		return infra.InfraSignal{
			Object: obj, Signal: "routeAdmitted",
			Value: 0.0, Severity: infra.SeverityStable, Timestamp: now,
		}
	}
	for _, ing := range route.Status.Ingress {
		for _, cond := range ing.Conditions {
			if cond.Type == "Admitted" && cond.Status == "True" {
				return infra.InfraSignal{
					Object: obj, Signal: "routeAdmitted",
					Value: 0.0, Severity: infra.SeverityStable, Timestamp: now,
				}
			}
		}
	}
	return infra.InfraSignal{
		Object:    obj,
		Signal:    "routeAdmitted",
		Value:     0.7,
		Severity:  infra.SeverityWarning,
		Message:   "route not admitted",
		Timestamp: now,
	}
}

func ingressSignal(obj infra.ObjectID, rawObj json.RawMessage, now time.Time) infra.InfraSignal {
	var ing struct {
		Status struct {
			LoadBalancer struct {
				Ingress []struct {
					IP       string `json:"ip"`
					Hostname string `json:"hostname"`
				} `json:"ingress"`
			} `json:"loadBalancer"`
		} `json:"status"`
	}
	_ = json.Unmarshal(rawObj, &ing)

	if len(ing.Status.LoadBalancer.Ingress) == 0 {
		return infra.InfraSignal{
			Object:    obj,
			Signal:    "ingressLB",
			Value:     0.4,
			Severity:  infra.SeverityElevated,
			Message:   "load balancer not provisioned",
			Timestamp: now,
		}
	}
	return infra.InfraSignal{
		Object: obj, Signal: "ingressLB",
		Value: 0.0, Severity: infra.SeverityStable, Timestamp: now,
	}
}

func minDur(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
