package discovery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/pkg/models"
)

// TestRun_FirstSyncPrunesStaleWorkloads_23vs17Regression reproduces the
// historical fleet-count drift (REFERENCE.md A8): a workload gets registered
// by telemetry (Prometheus/OTLP) after the informer already knows it's gone
// — e.g. a renamed or deleted Deployment whose old Pod metrics keep arriving
// for a few more scrape intervals. Before this fix, that stale entry lived
// forever because nothing ever reconciled the analyzer's registered set
// against what k8s actually reports. The fix: once the informer's first LIST
// sync completes, prune anything not confirmed by k8s.
func TestRun_FirstSyncPrunesStaleWorkloads_23vs17Regression(t *testing.T) {
	// k8s reports exactly one live Deployment ("api") in the LIST response —
	// simulating a cluster where 17 workloads truly exist (collapsed to 1
	// here for test brevity; the mechanism doesn't depend on the count).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("watch") == "true" {
			<-r.Context().Done()
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/apis/apps/v1/deployments":
			w.Write(listFixture("prod", []string{"api"}, "1"))
		default:
			// statefulsets, daemonsets, pods: nothing else live.
			w.Write(listFixture("prod", nil, "1"))
		}
	}))
	defer srv.Close()

	inf := newInformerForTest(srv.URL, srv.Client())

	// Simulate the drift: a "registry" (standing in for the analyzer) that
	// already has a stale workload registered from a telemetry scrape,
	// BEFORE the informer's first sync ever runs — exactly the historical
	// bug's precondition (Prometheus poller re-registers a workload the
	// informer already removed, or never confirmed to begin with).
	var mu sync.Mutex
	registered := map[string]models.WorkloadRef{
		"prod/Deployment/stale-worker": {Namespace: "prod", Kind: "Deployment", Name: "stale-worker"},
	}
	onAdd := func(ref models.WorkloadRef) {
		mu.Lock()
		registered[ref.Key()] = ref
		mu.Unlock()
	}
	onDelete := func(ref models.WorkloadRef) {
		mu.Lock()
		delete(registered, ref.Key())
		mu.Unlock()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	firstSyncDone := make(chan struct{})
	onFirstSync := func() {
		known := make(map[string]bool)
		for _, ref := range inf.KnownRefs() {
			known[ref.Key()] = true
		}
		mu.Lock()
		for key, ref := range registered {
			if ref.Kind != "Deployment" && ref.Kind != "StatefulSet" && ref.Kind != "DaemonSet" {
				continue
			}
			if !known[key] {
				delete(registered, key)
			}
		}
		mu.Unlock()
		close(firstSyncDone)
	}

	go inf.Run(ctx, onAdd, onDelete, onFirstSync)

	select {
	case <-firstSyncDone:
	case <-time.After(4 * time.Second):
		t.Fatal("onFirstSync never fired")
	}
	cancel()

	mu.Lock()
	defer mu.Unlock()
	if _, stillThere := registered["prod/Deployment/stale-worker"]; stillThere {
		t.Error("stale-worker should have been pruned after first sync — it was never confirmed by k8s (the historical drift bug)")
	}
	if _, ok := registered["prod/Deployment/api"]; !ok {
		t.Error("api should remain registered — it IS confirmed by k8s's LIST response")
	}
	if len(registered) != 1 {
		t.Errorf("expected exactly 1 registered workload after prune, got %d: %v", len(registered), registered)
	}
}

// TestRun_FirstSync_NonK8sHostsNeverPruned proves telemetry-only hosts
// (Kind="host", bare-metal/VM/demo mode) are never touched by the prune —
// they were never k8s-known to begin with, and REFERENCE.md A8's fix must
// not break non-k8s deployments.
func TestRun_FirstSync_NonK8sHostsNeverPruned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("watch") == "true" {
			<-r.Context().Done()
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(listFixture("prod", nil, "1"))
	}))
	defer srv.Close()

	inf := newInformerForTest(srv.URL, srv.Client())

	var mu sync.Mutex
	registered := map[string]models.WorkloadRef{
		"default/host/bare-metal-1": models.WorkloadRefFromHost("bare-metal-1"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	firstSyncDone := make(chan struct{})
	onFirstSync := func() {
		known := make(map[string]bool)
		for _, ref := range inf.KnownRefs() {
			known[ref.Key()] = true
		}
		mu.Lock()
		for key, ref := range registered {
			if ref.Kind != "Deployment" && ref.Kind != "StatefulSet" && ref.Kind != "DaemonSet" {
				continue
			}
			if !known[key] {
				delete(registered, key)
			}
		}
		mu.Unlock()
		close(firstSyncDone)
	}

	go inf.Run(ctx, func(models.WorkloadRef) {}, func(models.WorkloadRef) {}, onFirstSync)

	select {
	case <-firstSyncDone:
	case <-time.After(4 * time.Second):
		t.Fatal("onFirstSync never fired")
	}
	cancel()

	mu.Lock()
	defer mu.Unlock()
	if _, ok := registered["default/host/bare-metal-1"]; !ok {
		t.Error("non-k8s host workload must never be pruned by the informer's first-sync reconciliation")
	}
}
