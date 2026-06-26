// Package operator provides the OLM operator-layer infra collector.
// It watches OLM Subscriptions, ClusterServiceVersions, InstallPlans, and
// CustomResourceDefinitions, emitting GroupOperators signals.
//
// OLM (operators.coreos.com/v1alpha1) is probed at startup. If absent the
// collector is silently skipped — CRDs are only monitored alongside OLM.
package operator

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

const olmProbePath = "apis/operators.coreos.com/v1alpha1/subscriptions"

type k8sMeta struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
}

// Collector watches OLM resources and CRDs, emitting GroupOperators signals.
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

func (c *Collector) Name() string { return "operator" }

// Probe verifies OLM is present. Returns an error on clusters without OLM,
// causing the Registry to silently skip this collector.
func (c *Collector) Probe(ctx context.Context) error {
	url := fmt.Sprintf("%s/%s?limit=1", c.apiBase, olmProbePath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		probeErr := fmt.Errorf("operator: probe: %w", err)
		logger.Default.Info("infra collector skipped", "collector", c.Name(), "reason", probeErr.Error())
		return probeErr
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		probeErr := fmt.Errorf("operator: OLM (operators.coreos.com/v1alpha1) not available: status %d", resp.StatusCode)
		logger.Default.Info("infra collector skipped", "collector", c.Name(), "reason", probeErr.Error())
		return probeErr
	}
	logger.Default.Info("infra collector probed", "collector", c.Name())
	return nil
}

// Start watches OLM resources and CRDs until ctx is cancelled.
func (c *Collector) Start(ctx context.Context) error {
	logger.Default.Info("infra collector started", "collector", c.Name())
	go c.watchLoop(ctx, "apis/operators.coreos.com/v1alpha1/subscriptions", "Subscription")
	go c.watchLoop(ctx, "apis/operators.coreos.com/v1alpha1/clusterserviceversions", "ClusterServiceVersion")
	go c.watchLoop(ctx, "apis/operators.coreos.com/v1alpha1/installplans", "InstallPlan")
	go c.watchLoop(ctx, "apis/apiextensions.k8s.io/v1/customresourcedefinitions", "CustomResourceDefinition")
	<-ctx.Done()
	return nil
}

// Signals returns a concurrent-safe snapshot of all current operator signals.
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
		return "", fmt.Errorf("operator: list %s: %w", kind, err)
	}
	var lr struct {
		Metadata struct {
			ResourceVersion string `json:"resourceVersion"`
		} `json:"metadata"`
		Items []json.RawMessage `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		resp.Body.Close()
		return "", fmt.Errorf("operator: list %s decode: %w", kind, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("operator: list %s: status %d", kind, resp.StatusCode)
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
		return false, fmt.Errorf("operator: watch %s: %w", kind, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusGone {
		return true, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("operator: watch %s: status %d", kind, resp.StatusCode)
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
		return false, fmt.Errorf("operator: watch %s stream: %w", kind, err)
	}
	return false, nil
}

func (c *Collector) upsert(kind, name, namespace string, rawObj json.RawMessage, now time.Time) {
	scope := infra.ScopeNamespace
	if namespace == "" {
		scope = infra.ScopeCluster
	}
	obj := infra.ObjectID{
		Group:     infra.GroupOperators,
		Scope:     scope,
		Namespace: namespace,
		Kind:      kind,
		Name:      name,
	}
	c.mu.Lock()
	c.signals[obj.Key()] = operatorSignal(obj, kind, rawObj, now)
	c.mu.Unlock()
}

func (c *Collector) remove(kind, name, namespace string) {
	scope := infra.ScopeNamespace
	if namespace == "" {
		scope = infra.ScopeCluster
	}
	obj := infra.ObjectID{
		Group:     infra.GroupOperators,
		Scope:     scope,
		Namespace: namespace,
		Kind:      kind,
		Name:      name,
	}
	c.mu.Lock()
	delete(c.signals, obj.Key())
	c.mu.Unlock()
}

func operatorSignal(obj infra.ObjectID, kind string, rawObj json.RawMessage, now time.Time) infra.InfraSignal {
	switch kind {
	case "Subscription":
		return subscriptionSignal(obj, rawObj, now)
	case "ClusterServiceVersion":
		return csvSignal(obj, rawObj, now)
	case "InstallPlan":
		return installPlanSignal(obj, rawObj, now)
	case "CustomResourceDefinition":
		return crdSignal(obj, rawObj, now)
	default:
		return infra.InfraSignal{
			Object: obj, Signal: "operatorPresence",
			Value: 0.0, Severity: infra.SeverityStable, Timestamp: now,
		}
	}
}

func subscriptionSignal(obj infra.ObjectID, rawObj json.RawMessage, now time.Time) infra.InfraSignal {
	var sub struct {
		Status struct {
			State string `json:"state"`
		} `json:"status"`
	}
	_ = json.Unmarshal(rawObj, &sub)

	switch sub.Status.State {
	case "AtLatestKnown":
		return infra.InfraSignal{
			Object: obj, Signal: "subscriptionState",
			Value: 0.0, Severity: infra.SeverityStable, Timestamp: now,
		}
	case "UpgradePending":
		return infra.InfraSignal{
			Object: obj, Signal: "subscriptionState",
			Value: 0.2, Severity: infra.SeverityElevated,
			Message: "upgrade pending", Timestamp: now,
		}
	default:
		return infra.InfraSignal{
			Object: obj, Signal: "subscriptionState",
			Value: 0.5, Severity: infra.SeverityWarning,
			Message: fmt.Sprintf("subscription state: %s", sub.Status.State),
			Timestamp: now,
		}
	}
}

func csvSignal(obj infra.ObjectID, rawObj json.RawMessage, now time.Time) infra.InfraSignal {
	var csv struct {
		Status struct {
			Phase string `json:"phase"`
		} `json:"status"`
	}
	_ = json.Unmarshal(rawObj, &csv)

	switch csv.Status.Phase {
	case "Succeeded":
		return infra.InfraSignal{
			Object: obj, Signal: "csvPhase",
			Value: 0.0, Severity: infra.SeverityStable, Timestamp: now,
		}
	case "Failed":
		return infra.InfraSignal{
			Object: obj, Signal: "csvPhase",
			Value: 1.0, Severity: infra.SeverityCritical,
			Message: "CSV failed", Timestamp: now,
		}
	case "Installing", "InstallReady":
		return infra.InfraSignal{
			Object: obj, Signal: "csvPhase",
			Value: 0.2, Severity: infra.SeverityElevated,
			Message: fmt.Sprintf("CSV phase: %s", csv.Status.Phase), Timestamp: now,
		}
	case "Pending":
		return infra.InfraSignal{
			Object: obj, Signal: "csvPhase",
			Value: 0.3, Severity: infra.SeverityElevated,
			Message: "CSV pending", Timestamp: now,
		}
	default:
		return infra.InfraSignal{
			Object: obj, Signal: "csvPhase",
			Value: 0.5, Severity: infra.SeverityWarning,
			Message: fmt.Sprintf("CSV phase: %s", csv.Status.Phase), Timestamp: now,
		}
	}
}

func installPlanSignal(obj infra.ObjectID, rawObj json.RawMessage, now time.Time) infra.InfraSignal {
	var ip struct {
		Status struct {
			Phase string `json:"phase"`
		} `json:"status"`
	}
	_ = json.Unmarshal(rawObj, &ip)

	switch ip.Status.Phase {
	case "Complete":
		return infra.InfraSignal{
			Object: obj, Signal: "installPlanPhase",
			Value: 0.0, Severity: infra.SeverityStable, Timestamp: now,
		}
	case "Failed":
		return infra.InfraSignal{
			Object: obj, Signal: "installPlanPhase",
			Value: 0.8, Severity: infra.SeverityCritical,
			Message: "install plan failed", Timestamp: now,
		}
	default:
		return infra.InfraSignal{
			Object: obj, Signal: "installPlanPhase",
			Value: 0.2, Severity: infra.SeverityElevated,
			Message: fmt.Sprintf("install plan phase: %s", ip.Status.Phase), Timestamp: now,
		}
	}
}

func crdSignal(obj infra.ObjectID, rawObj json.RawMessage, now time.Time) infra.InfraSignal {
	var crd struct {
		Status struct {
			Conditions []struct {
				Type   string `json:"type"`
				Status string `json:"status"`
			} `json:"conditions"`
		} `json:"status"`
	}
	_ = json.Unmarshal(rawObj, &crd)

	for _, c := range crd.Status.Conditions {
		if c.Type == "Established" && c.Status != "True" {
			return infra.InfraSignal{
				Object: obj, Signal: "crdEstablished",
				Value: 0.7, Severity: infra.SeverityWarning,
				Message: "CRD not established", Timestamp: now,
			}
		}
	}
	return infra.InfraSignal{
		Object: obj, Signal: "crdEstablished",
		Value: 0.0, Severity: infra.SeverityStable, Timestamp: now,
	}
}

func minDur(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
