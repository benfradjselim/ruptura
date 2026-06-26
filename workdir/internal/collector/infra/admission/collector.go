// Package admission provides the admission-layer infra collector.
// It watches Kyverno PolicyReports and ValidatingWebhookConfigurations,
// emitting GroupAdmission signals.
//
// Kyverno ships two possible API groups depending on version:
//   - wgpolicyk8s.io/v1alpha2 (newer)
//   - kyverno.io/v1 (older)
//
// Probe tries them in that order. If neither is present the collector is
// silently skipped; there is no signal loss on non-Kyverno clusters.
package admission

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

// kyvernoPaths lists the two Kyverno policy-report API group paths in preference order.
var kyvernoPaths = []string{
	"apis/wgpolicyk8s.io/v1alpha2/policyreports",
	"apis/kyverno.io/v1/policyreports",
}

const webhookPath = "apis/admissionregistration.k8s.io/v1/validatingwebhookconfigurations"

type k8sMeta struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
}

// Collector watches PolicyReports and ValidatingWebhookConfigurations,
// emitting GroupAdmission signals.
type Collector struct {
	apiBase          string
	token            string
	httpClient       *http.Client
	policyReportPath string // resolved during Probe; read-only after that
	mu               sync.RWMutex
	signals          map[string]infra.InfraSignal
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

func (c *Collector) Name() string { return "admission" }

// Probe tries the newer then older Kyverno API group. Returns an error if
// neither is present — the Registry will silently skip this collector.
func (c *Collector) Probe(ctx context.Context) error {
	for _, path := range kyvernoPaths {
		if err := c.probeAPI(ctx, path); err == nil {
			c.policyReportPath = path
			logger.Default.Info("infra collector probed", "collector", c.Name())
			return nil
		}
	}
	probeErr := fmt.Errorf("admission: kyverno policy-reports API not available (tried wgpolicyk8s.io/v1alpha2, kyverno.io/v1)")
	logger.Default.Info("infra collector skipped", "collector", c.Name(), "reason", probeErr.Error())
	return probeErr
}

// Start watches PolicyReports and ValidatingWebhookConfigurations until ctx is
// cancelled. The policyReportPath is guaranteed to be set when Start is called.
func (c *Collector) Start(ctx context.Context) error {
	logger.Default.Info("infra collector started", "collector", c.Name())
	go c.watchLoop(ctx, c.policyReportPath, "PolicyReport")
	go c.watchLoop(ctx, webhookPath, "ValidatingWebhookConfiguration")
	<-ctx.Done()
	return nil
}

// Signals returns a concurrent-safe snapshot of all current admission signals.
func (c *Collector) Signals() []infra.InfraSignal {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]infra.InfraSignal, 0, len(c.signals))
	for _, s := range c.signals {
		out = append(out, s)
	}
	return out
}

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
		return "", fmt.Errorf("admission: list %s: %w", kind, err)
	}
	var lr struct {
		Metadata struct {
			ResourceVersion string `json:"resourceVersion"`
		} `json:"metadata"`
		Items []json.RawMessage `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		resp.Body.Close()
		return "", fmt.Errorf("admission: list %s decode: %w", kind, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("admission: list %s: status %d", kind, resp.StatusCode)
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
		return false, fmt.Errorf("admission: watch %s: %w", kind, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusGone {
		return true, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("admission: watch %s: status %d", kind, resp.StatusCode)
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
		return false, fmt.Errorf("admission: watch %s stream: %w", kind, err)
	}
	return false, nil
}

func (c *Collector) upsert(kind, name, namespace string, rawObj json.RawMessage, now time.Time) {
	scope := infra.ScopeNamespace
	if namespace == "" {
		scope = infra.ScopeCluster
	}
	obj := infra.ObjectID{
		Group:     infra.GroupAdmission,
		Scope:     scope,
		Namespace: namespace,
		Kind:      kind,
		Name:      name,
	}
	c.mu.Lock()
	c.signals[obj.Key()] = admissionSignal(obj, kind, rawObj, now)
	c.mu.Unlock()
}

func (c *Collector) remove(kind, name, namespace string) {
	scope := infra.ScopeNamespace
	if namespace == "" {
		scope = infra.ScopeCluster
	}
	obj := infra.ObjectID{
		Group:     infra.GroupAdmission,
		Scope:     scope,
		Namespace: namespace,
		Kind:      kind,
		Name:      name,
	}
	c.mu.Lock()
	delete(c.signals, obj.Key())
	c.mu.Unlock()
}

func admissionSignal(obj infra.ObjectID, kind string, rawObj json.RawMessage, now time.Time) infra.InfraSignal {
	if kind == "PolicyReport" {
		return policyReportSignal(obj, rawObj, now)
	}
	// ValidatingWebhookConfiguration — presence is healthy governance.
	return infra.InfraSignal{
		Object: obj, Signal: "webhookPresence",
		Value: 0.0, Severity: infra.SeverityStable, Timestamp: now,
	}
}

func policyReportSignal(obj infra.ObjectID, rawObj json.RawMessage, now time.Time) infra.InfraSignal {
	var pr struct {
		Summary struct {
			Pass  int `json:"pass"`
			Fail  int `json:"fail"`
			Warn  int `json:"warn"`
			Error int `json:"error"`
		} `json:"summary"`
	}
	_ = json.Unmarshal(rawObj, &pr)

	total := pr.Summary.Pass + pr.Summary.Fail + pr.Summary.Warn + pr.Summary.Error
	if total == 0 || (pr.Summary.Fail == 0 && pr.Summary.Error == 0) {
		return infra.InfraSignal{
			Object: obj, Signal: "policyViolation",
			Value: 0.0, Severity: infra.SeverityStable, Timestamp: now,
		}
	}

	if pr.Summary.Error > 0 && pr.Summary.Fail == 0 {
		return infra.InfraSignal{
			Object: obj, Signal: "policyViolation",
			Value: 0.5, Severity: infra.SeverityWarning,
			Message: fmt.Sprintf("%d policy errors", pr.Summary.Error),
			Timestamp: now,
		}
	}

	ratio := float64(pr.Summary.Fail) / float64(total)
	if ratio > 1.0 {
		ratio = 1.0
	}
	severity := infra.SeverityElevated
	if ratio > 0.3 {
		severity = infra.SeverityWarning
	}
	if ratio > 0.7 {
		severity = infra.SeverityCritical
	}
	return infra.InfraSignal{
		Object: obj, Signal: "policyViolation",
		Value:     ratio,
		Severity:  severity,
		Message:   fmt.Sprintf("%d policy violations", pr.Summary.Fail),
		Timestamp: now,
	}
}

func minDur(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
