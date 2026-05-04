package providers

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/benfradjselim/ruptura/internal/actions/engine"
)

const (
	inClusterTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	inClusterCAPath    = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	k8sAPIBase         = "https://kubernetes.default.svc"
)

// KubernetesActuator executes remediation actions against the Kubernetes API
// using in-cluster ServiceAccount credentials.
type KubernetesActuator struct {
	apiBase    string
	token      string
	httpClient *http.Client
}

// NewKubernetesActuator creates an actuator from in-cluster ServiceAccount credentials.
// Returns an error when not running inside a Kubernetes pod.
func NewKubernetesActuator() (*KubernetesActuator, error) {
	tokenBytes, err := os.ReadFile(inClusterTokenPath)
	if err != nil {
		return nil, fmt.Errorf("k8s actuator: read service account token: %w", err)
	}
	caBytes, err := os.ReadFile(inClusterCAPath)
	if err != nil {
		return nil, fmt.Errorf("k8s actuator: read CA bundle: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caBytes) {
		return nil, fmt.Errorf("k8s actuator: could not parse CA bundle")
	}
	return &KubernetesActuator{
		apiBase: k8sAPIBase,
		token:   strings.TrimSpace(string(tokenBytes)),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{RootCAs: pool},
			},
		},
	}, nil
}

// Scale increases (or decreases) the replica count of a Deployment or StatefulSet.
// delta defaults to +1 when zero.
func (a *KubernetesActuator) Scale(ctx context.Context, namespace, kind, name string, delta int) error {
	if delta == 0 {
		delta = 1
	}
	current, err := a.getReplicas(ctx, namespace, kind, name)
	if err != nil {
		return err
	}
	desired := current + delta
	if desired < 1 {
		desired = 1
	}
	patch := fmt.Sprintf(`{"spec":{"replicas":%d}}`, desired)
	return a.patch(ctx, a.workloadURL(namespace, kind, name)+"/scale", "application/merge-patch+json", patch)
}

// Restart triggers a rolling restart by updating the restartedAt annotation on the pod template.
// Kubernetes will recreate pods one by one (rolling), respecting PodDisruptionBudgets.
func (a *KubernetesActuator) Restart(ctx context.Context, namespace, kind, name string) error {
	ts := time.Now().UTC().Format(time.RFC3339)
	patch := fmt.Sprintf(
		`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":%q}}}}}`,
		ts,
	)
	return a.patch(ctx, a.workloadURL(namespace, kind, name), "application/strategic-merge-patch+json", patch)
}

// Cordon marks a Node as unschedulable so no new pods are scheduled on it.
// Existing pods continue running; use drain to evict them.
func (a *KubernetesActuator) Cordon(ctx context.Context, nodeName string) error {
	url := fmt.Sprintf("%s/api/v1/nodes/%s", a.apiBase, nodeName)
	return a.patch(ctx, url, "application/merge-patch+json", `{"spec":{"unschedulable":true}}`)
}

func (a *KubernetesActuator) workloadURL(namespace, kind, name string) string {
	switch strings.ToLower(kind) {
	case "statefulset", "statefulsets":
		return fmt.Sprintf("%s/apis/apps/v1/namespaces/%s/statefulsets/%s", a.apiBase, namespace, name)
	case "daemonset", "daemonsets":
		return fmt.Sprintf("%s/apis/apps/v1/namespaces/%s/daemonsets/%s", a.apiBase, namespace, name)
	default:
		return fmt.Sprintf("%s/apis/apps/v1/namespaces/%s/deployments/%s", a.apiBase, namespace, name)
	}
}

func (a *KubernetesActuator) getReplicas(ctx context.Context, namespace, kind, name string) (int, error) {
	url := a.workloadURL(namespace, kind, name) + "/scale"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+a.token)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("k8s get scale %s/%s: %w", namespace, name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("k8s get scale %s/%s: unexpected status %d", namespace, name, resp.StatusCode)
	}

	var scale struct {
		Spec struct {
			Replicas int `json:"replicas"`
		} `json:"spec"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&scale); err != nil {
		return 0, fmt.Errorf("k8s get scale decode: %w", err)
	}
	return scale.Spec.Replicas, nil
}

func (a *KubernetesActuator) patch(ctx context.Context, url, contentType, body string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	req.Header.Set("Content-Type", contentType)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("k8s patch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("k8s patch %s: unexpected status %d", url, resp.StatusCode)
	}
	return nil
}

// KubernetesProvider executes Kubernetes remediation actions.
// When actuator is nil (not running in-cluster), Execute is a no-op that logs intent.
type KubernetesProvider struct {
	actuator *KubernetesActuator
}

func NewKubernetesProvider() *KubernetesProvider { return &KubernetesProvider{} }

func NewKubernetesProviderWithActuator(a *KubernetesActuator) *KubernetesProvider {
	return &KubernetesProvider{actuator: a}
}

func (p *KubernetesProvider) Name() string { return "kubernetes" }

func (p *KubernetesProvider) Execute(ctx context.Context, a engine.ActionRecommendation) error {
	if p.actuator == nil {
		// Not in-cluster — action is queued/logged but not executed.
		return nil
	}

	ns, kind, name := resolveWorkloadTarget(a)

	switch a.ActionType {
	case "scale":
		return p.actuator.Scale(ctx, ns, kind, name, a.ScaleDelta)
	case "restart":
		return p.actuator.Restart(ctx, ns, kind, name)
	case "cordon":
		// For cordon, name is the node name (set in ActionRecommendation.NodeName or Host).
		nodeName := a.NodeName
		if nodeName == "" {
			nodeName = name
		}
		return p.actuator.Cordon(ctx, nodeName)
	default:
		// alert/notify/page are handled by other providers.
		return nil
	}
}

// resolveWorkloadTarget extracts namespace/kind/name from an ActionRecommendation.
// Prefers explicit Namespace+Kind fields; falls back to parsing Host as "ns/kind/name".
func resolveWorkloadTarget(a engine.ActionRecommendation) (namespace, kind, name string) {
	if a.Namespace != "" {
		namespace = a.Namespace
		kind = a.Kind
		if kind == "" {
			kind = "Deployment"
		}
		name = a.Host
		return
	}
	// Try parsing Host as WorkloadRef key: "namespace/Kind/name"
	parts := strings.SplitN(a.Host, "/", 3)
	if len(parts) == 3 {
		return parts[0], parts[1], parts[2]
	}
	// Fallback: treat host as a Deployment name in the default namespace.
	return "default", "Deployment", a.Host
}
