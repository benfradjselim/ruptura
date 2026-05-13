// Package discovery provides Kubernetes workload auto-discovery for Ruptura.
// It LIST+WATCHes Deployments, StatefulSets, and DaemonSets using the pod's
// in-cluster ServiceAccount credentials (raw HTTP, no client-go dependency).
// When not running inside a cluster, NewInformer returns an error and the caller
// should skip the informer — Ruptura continues to work via telemetry-driven discovery.
package discovery

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/benfradjselim/ruptura/pkg/models"
)

// Informer watches k8s workload resources and pre-registers them in Ruptura.
type Informer struct {
	apiBase    string
	token      string
	httpClient *http.Client
	logMu      sync.Mutex
	logFn      func(msg string, args ...interface{})
}

// NewInformer creates an Informer from in-cluster ServiceAccount credentials.
// Returns an error if the token or CA cert cannot be read (i.e., not in-cluster).
func NewInformer() (*Informer, error) {
	apiBase, token, client, err := inClusterCreds()
	if err != nil {
		return nil, err
	}
	return &Informer{
		apiBase:    apiBase,
		token:      token,
		httpClient: client,
	}, nil
}

// newInformerForTest creates an Informer with a custom API base and HTTP client.
// Used by tests to inject an httptest.Server without touching SA paths.
func newInformerForTest(apiBase string, client *http.Client) *Informer {
	return &Informer{
		apiBase:    apiBase,
		token:      "test-token",
		httpClient: client,
	}
}

// Run starts three goroutines — one per resource type — and blocks until ctx is cancelled.
// onAdd is called for every ADDED or MODIFIED event; onDelete for DELETED.
// Both callbacks must be safe to call concurrently from multiple goroutines.
func (inf *Informer) Run(ctx context.Context, onAdd func(models.WorkloadRef), onDelete func(models.WorkloadRef)) {
	resources := []resource{
		{path: "apis/apps/v1/deployments", kind: "Deployment"},
		{path: "apis/apps/v1/statefulsets", kind: "StatefulSet"},
		{path: "apis/apps/v1/daemonsets", kind: "DaemonSet"},
	}

	var wg sync.WaitGroup
	for _, res := range resources {
		res := res // capture
		wg.Add(1)
		go func() {
			defer wg.Done()
			inf.watchResource(ctx, res, onAdd, onDelete)
		}()
	}
	wg.Wait()
}

func (inf *Informer) logf(format string, args ...interface{}) {
	if inf.logFn != nil {
		inf.logFn(format, args...)
		return
	}
	// Default: fmt.Printf so tests can see output without importing the logger.
	fmt.Printf(format+"\n", args...)
}
