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
	"time"

	"github.com/benfradjselim/ruptura/pkg/models"
)

// Informer watches k8s workload resources and pre-registers them in Ruptura.
type Informer struct {
	apiBase    string
	token      string
	httpClient *http.Client
	logMu      sync.Mutex
	logFn      func(msg string, args ...interface{})
	cache      *MetadataCache
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
		cache:      newMetadataCache(),
	}, nil
}

// newInformerForTest creates an Informer with a custom API base and HTTP client.
// Used by tests to inject an httptest.Server without touching SA paths.
func newInformerForTest(apiBase string, client *http.Client) *Informer {
	return &Informer{
		apiBase:    apiBase,
		token:      "test-token",
		httpClient: client,
		cache:      newMetadataCache(),
	}
}

// GetWorkloadMeta returns the cached k8s metadata for a workload.
// ok is false when the workload is not in the cache (informer not running, or
// workload not yet discovered).
func (inf *Informer) GetWorkloadMeta(ns, kind, name string) (WorkloadMeta, bool) {
	if inf.cache == nil {
		return WorkloadMeta{}, false
	}
	return inf.cache.Get(ns, kind, name)
}

// Run starts goroutines for workload + pod watching and blocks until ctx is cancelled.
// onAdd is called for every ADDED/MODIFIED workload event; onDelete for DELETED.
// The metadata cache is populated in parallel.
func (inf *Informer) Run(ctx context.Context, onAdd func(models.WorkloadRef), onDelete func(models.WorkloadRef)) {
	resources := []resource{
		{path: "apis/apps/v1/deployments", kind: "Deployment"},
		{path: "apis/apps/v1/statefulsets", kind: "StatefulSet"},
		{path: "apis/apps/v1/daemonsets", kind: "DaemonSet"},
	}

	var wg sync.WaitGroup

	// Existing: WorkloadRef registration goroutines (one per resource type).
	for _, res := range resources {
		res := res
		wg.Add(1)
		go func() {
			defer wg.Done()
			inf.watchResource(ctx, res, onAdd, onDelete)
		}()
	}

	// New: metadata cache goroutines — one per workload type + one for pods.
	for _, res := range resources {
		res := res
		wg.Add(1)
		go func() {
			defer wg.Done()
			inf.runMetadataLoop(ctx, res.path, res.kind)
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		inf.runPodLoop(ctx)
	}()

	wg.Wait()
}

// runMetadataLoop runs a perpetual LIST+WATCH loop populating the metadata cache.
func (inf *Informer) runMetadataLoop(ctx context.Context, path, kind string) {
	backoff := time.Second
	const maxBackoff = 30 * time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		rv, err := inf.listWorkloads(ctx, path, kind)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			inf.logf("discovery: metadata list %s failed: %v — retrying in %s", path, err, backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff = minDuration(backoff*2, maxBackoff)
			continue
		}
		backoff = time.Second
		if err := inf.watchWorkloads(ctx, path, kind, rv); err != nil && ctx.Err() == nil {
			inf.logf("discovery: metadata watch %s error: %v — re-listing", path, err)
		}
	}
}

// runPodLoop runs a perpetual LIST+WATCH loop for pods, populating the pod cache.
func (inf *Informer) runPodLoop(ctx context.Context) {
	backoff := time.Second
	const maxBackoff = 30 * time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		rv, err := inf.listPods(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			inf.logf("discovery: pod list failed: %v — retrying in %s", err, backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff = minDuration(backoff*2, maxBackoff)
			continue
		}
		backoff = time.Second
		if err := inf.watchPods(ctx, rv); err != nil && ctx.Err() == nil {
			inf.logf("discovery: pod watch error: %v — re-listing", err)
		}
	}
}

func (inf *Informer) logf(format string, args ...interface{}) {
	if inf.logFn != nil {
		inf.logFn(format, args...)
		return
	}
	fmt.Printf(format+"\n", args...)
}
