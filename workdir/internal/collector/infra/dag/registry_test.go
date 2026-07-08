package dag

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/collector/infra"
)

// fakeCollector is a minimal infra.InfraCollector for whitebox registry tests.
type fakeCollector struct {
	name    string
	signals []infra.InfraSignal
}

func (f *fakeCollector) Name() string                    { return f.name }
func (f *fakeCollector) Probe(ctx context.Context) error { return nil }
func (f *fakeCollector) Start(ctx context.Context) error { <-ctx.Done(); return nil }
func (f *fakeCollector) Signals() []infra.InfraSignal    { return f.signals }

// fakePersister captures every Put* call for assertions.
type fakePersister struct {
	mu           sync.Mutex
	signals      []string // "group|scope|ns|kind|name|signal"
	groupHealth  []string // "group|ns"
	groupNoise   []string // "group"
	propagations int
	lastPropJSON []byte
}

func (f *fakePersister) PutInfraSignal(group, scope, ns, kind, name, signal string, ts time.Time, value float64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.signals = append(f.signals, group+"|"+scope+"|"+ns+"|"+kind+"|"+name+"|"+signal)
	return nil
}

func (f *fakePersister) PutGroupHealth(group, ns string, ts time.Time, health float64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.groupHealth = append(f.groupHealth, group+"|"+ns)
	return nil
}

func (f *fakePersister) PutGroupNoise(group string, ts time.Time, gni float64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.groupNoise = append(f.groupNoise, group)
	return nil
}

func (f *fakePersister) PutPropagationSnapshot(ts time.Time, payload []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.propagations++
	f.lastPropJSON = payload
	return nil
}

func TestRegistry_Tick_PersistsSignalsAndGroupHealth(t *testing.T) {
	r := NewRegistry()
	r.active = []infra.InfraCollector{
		&fakeCollector{name: "fake-network", signals: []infra.InfraSignal{
			{
				Object: infra.ObjectID{Group: infra.GroupNetwork, Scope: infra.ScopeNamespace, Namespace: "prod", Kind: "Route", Name: "console"},
				Signal: "nodeStress", Value: 0.4, Timestamp: time.Now(),
			},
		}},
	}
	fp := &fakePersister{}
	r.SetPersister(fp)

	r.tick()

	fp.mu.Lock()
	defer fp.mu.Unlock()
	if len(fp.signals) != 1 {
		t.Fatalf("expected 1 persisted signal, got %d: %v", len(fp.signals), fp.signals)
	}
	want := infra.GroupNetwork + "|" + infra.ScopeNamespace + "|prod|Route|console|nodeStress"
	if fp.signals[0] != want {
		t.Errorf("persisted signal = %q, want %q", fp.signals[0], want)
	}
	if len(fp.groupHealth) == 0 {
		t.Error("expected at least one PutGroupHealth call")
	}
}

func TestRegistry_Tick_NilPersister_NoPanic(t *testing.T) {
	r := NewRegistry()
	r.active = []infra.InfraCollector{
		&fakeCollector{name: "fake", signals: []infra.InfraSignal{
			{Object: infra.ObjectID{Group: infra.GroupStorage, Scope: infra.ScopeNamespace, Namespace: "prod", Kind: "PersistentVolumeClaim", Name: "data"}, Signal: "pvcStall", Value: 0.2, Timestamp: time.Now()},
		}},
	}
	// No SetPersister call — must not panic, matches pre-FBL-A3-1 behavior.
	r.tick()

	if _, ok := r.GroupSnapshot(infra.GroupStorage, "prod"); !ok {
		t.Error("in-memory aggregation should still work with no persister set")
	}
}

func TestRegistry_Tick_GroupNoiseOnlyFromClusterScopedSnapshot(t *testing.T) {
	r := NewRegistry()
	r.active = []infra.InfraCollector{
		&fakeCollector{name: "fake-ns", signals: []infra.InfraSignal{
			{Object: infra.ObjectID{Group: infra.GroupNetwork, Scope: infra.ScopeNamespace, Namespace: "prod", Kind: "Route", Name: "a"}, Signal: "x", Value: 0.1, Timestamp: time.Now()},
		}},
		&fakeCollector{name: "fake-cluster", signals: []infra.InfraSignal{
			{Object: infra.ObjectID{Group: infra.GroupNetwork, Scope: infra.ScopeCluster, Namespace: "", Kind: "ClusterOperator", Name: "b"}, Signal: "y", Value: 0.2, Timestamp: time.Now()},
		}},
	}
	fp := &fakePersister{}
	r.SetPersister(fp)

	r.tick()

	fp.mu.Lock()
	defer fp.mu.Unlock()
	if len(fp.groupNoise) != 1 {
		t.Fatalf("expected exactly 1 PutGroupNoise call (cluster-scoped only), got %d: %v", len(fp.groupNoise), fp.groupNoise)
	}
	if len(fp.groupHealth) != 2 {
		t.Errorf("expected 2 PutGroupHealth calls (one per namespace scope), got %d", len(fp.groupHealth))
	}
}

func TestRegistry_Tick_PersistsPropagationSnapshotAsValidJSON(t *testing.T) {
	r := NewRegistry()
	r.active = []infra.InfraCollector{
		&fakeCollector{name: "fake", signals: []infra.InfraSignal{
			{Object: infra.ObjectID{Group: infra.GroupWorkload, Scope: infra.ScopeNamespace, Namespace: "prod", Kind: "Deployment", Name: "api"}, Signal: "x", Value: 0.5, Timestamp: time.Now()},
		}},
	}
	fp := &fakePersister{}
	r.SetPersister(fp)

	r.tick()

	fp.mu.Lock()
	defer fp.mu.Unlock()
	if fp.propagations != 1 {
		t.Fatalf("expected 1 propagation snapshot write, got %d", fp.propagations)
	}
	var results []PropagationResult
	if err := json.Unmarshal(fp.lastPropJSON, &results); err != nil {
		t.Fatalf("propagation snapshot payload is not valid JSON: %v", err)
	}
	if len(results) != 1 || results[0].Namespace != "prod" {
		t.Errorf("propagation snapshot = %+v, want one result for namespace prod", results)
	}
}

func TestRegistry_Tick_NoSignals_PersisterNotCalled(t *testing.T) {
	r := NewRegistry()
	fp := &fakePersister{}
	r.SetPersister(fp)

	r.tick() // no active collectors — should return early, before persistence

	fp.mu.Lock()
	defer fp.mu.Unlock()
	if len(fp.signals) != 0 || fp.propagations != 0 {
		t.Errorf("expected no persistence calls with zero signals, got signals=%d propagations=%d", len(fp.signals), fp.propagations)
	}
}
