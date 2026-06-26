package dag

import (
	"context"
	"sync"
	"time"

	"github.com/benfradjselim/ruptura/internal/collector/infra"
)

// Registry probes, starts, and aggregates all registered InfraCollectors.
// It computes GroupSnapshots from live signals every 30 seconds, then ticks
// the CGPM Propagator for each observed namespace. Safe for concurrent use.
type Registry struct {
	mu             sync.RWMutex
	collectors     []infra.InfraCollector
	active         []infra.InfraCollector
	groupSnapshots map[string]infra.GroupSnapshot // key: group+"|"+namespace
	propagator     *Propagator
}

// NewRegistry creates an empty Registry with an initialized Propagator.
func NewRegistry() *Registry {
	return &Registry{
		groupSnapshots: make(map[string]infra.GroupSnapshot),
		propagator:     NewPropagator(),
	}
}

// Add registers a collector. Must be called before Run.
func (r *Registry) Add(c infra.InfraCollector) {
	r.mu.Lock()
	r.collectors = append(r.collectors, c)
	r.mu.Unlock()
}

// Run probes all registered collectors concurrently, starts the ones that pass,
// then ticks the aggregation+propagation loop every 30 seconds. Blocks until
// ctx is cancelled.
func (r *Registry) Run(ctx context.Context) {
	r.mu.RLock()
	collectors := make([]infra.InfraCollector, len(r.collectors))
	copy(collectors, r.collectors)
	r.mu.RUnlock()

	var (
		probeMu sync.Mutex
		wg      sync.WaitGroup
		active  []infra.InfraCollector
	)
	for _, c := range collectors {
		wg.Add(1)
		go func(c infra.InfraCollector) {
			defer wg.Done()
			if err := c.Probe(ctx); err != nil {
				return
			}
			probeMu.Lock()
			active = append(active, c)
			probeMu.Unlock()
		}(c)
	}
	wg.Wait()

	r.mu.Lock()
	r.active = active
	r.mu.Unlock()

	for _, c := range active {
		go func(c infra.InfraCollector) { _ = c.Start(ctx) }(c)
	}

	r.tick()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.tick()
		}
	}
}

// GroupSnapshot returns the cached snapshot for the given group and namespace.
// Namespace="" means cluster-scoped. Returns (zero, false) when no data.
func (r *Registry) GroupSnapshot(group, ns string) (infra.GroupSnapshot, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.groupSnapshots[group+"|"+ns]
	return s, ok
}

// AllGroupSnapshots returns all cached GroupSnapshots as a slice.
func (r *Registry) AllGroupSnapshots() []infra.GroupSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]infra.GroupSnapshot, 0, len(r.groupSnapshots))
	for _, s := range r.groupSnapshots {
		out = append(out, s)
	}
	return out
}

// GetPropagator returns the underlying Propagator for querying CGPM results.
func (r *Registry) GetPropagator() *Propagator {
	return r.propagator
}

// NamespaceSnapshot builds an infra.NamespaceSnapshot for the given namespace
// by combining cached GroupSnapshots with CGPM propagation results.
// Returns the healthy default when the registry has no data for ns.
func (r *Registry) NamespaceSnapshot(ns string) infra.NamespaceSnapshot {
	r.mu.RLock()

	networkHealth := 1.0
	if s, ok := r.groupSnapshots[infra.GroupNetwork+"|"+ns]; ok {
		networkHealth = s.Health
	}

	storageRisk := 0.0
	if s, ok := r.groupSnapshots[infra.GroupStorage+"|"+ns]; ok {
		storageRisk = 1.0 - s.Health
	}

	admissionPressure := 0.0
	if s, ok := r.groupSnapshots[infra.GroupAdmission+"|"+ns]; ok {
		admissionPressure = 1.0 - s.Health
	}

	infraStress := 0.0
	for _, s := range r.groupSnapshots {
		if s.Namespace == "" || s.Namespace == ns {
			if stress := 1.0 - s.Health; stress > infraStress {
				infraStress = stress
			}
		}
	}

	r.mu.RUnlock()

	return infra.NamespaceSnapshot{
		Namespace:         ns,
		InfraStress:       infraStress,
		NetworkHealth:     networkHealth,
		StorageRisk:       storageRisk,
		AdmissionPressure: admissionPressure,
		PropPressure:      r.propagator.LastResult(ns).WorkloadPressure(),
		Timestamp:         time.Now(),
	}
}

// AllSignals collects and returns raw InfraSignals from all active collectors.
// Callers can filter by sig.Object.Kind for per-resource-type views.
func (r *Registry) AllSignals() []infra.InfraSignal {
	r.mu.RLock()
	active := make([]infra.InfraCollector, len(r.active))
	copy(active, r.active)
	r.mu.RUnlock()
	var all []infra.InfraSignal
	for _, c := range active {
		all = append(all, c.Signals()...)
	}
	return all
}

// ActiveCollectors returns the names of collectors that passed Probe.
func (r *Registry) ActiveCollectors() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, len(r.active))
	for i, c := range r.active {
		names[i] = c.Name()
	}
	return names
}

// tick aggregates signals from all active collectors into GroupSnapshots and
// ticks the CGPM propagator for every observed namespace.
func (r *Registry) tick() {
	r.mu.RLock()
	active := make([]infra.InfraCollector, len(r.active))
	copy(active, r.active)
	r.mu.RUnlock()

	var allSignals []infra.InfraSignal
	for _, c := range active {
		allSignals = append(allSignals, c.Signals()...)
	}
	if len(allSignals) == 0 {
		return
	}

	type groupKey struct{ group, ns string }
	type groupAcc struct {
		maxVal     float64
		sumVal     float64
		count      int
		objectKeys map[string]struct{}
	}

	acc := make(map[groupKey]*groupAcc)
	for _, sig := range allSignals {
		k := groupKey{sig.Object.Group, sig.Object.Namespace}
		a := acc[k]
		if a == nil {
			a = &groupAcc{objectKeys: make(map[string]struct{})}
			acc[k] = a
		}
		if sig.Value > a.maxVal {
			a.maxVal = sig.Value
		}
		a.sumVal += sig.Value
		a.count++
		a.objectKeys[sig.Object.Key()] = struct{}{}
	}

	now := time.Now()
	snapshots := make([]infra.GroupSnapshot, 0, len(acc))
	for k, a := range acc {
		spread := 0.0
		if a.count > 0 {
			spread = a.sumVal / float64(a.count)
		}
		snapshots = append(snapshots, infra.GroupSnapshot{
			Group:       k.group,
			Namespace:   k.ns,
			Health:      1.0 - a.maxVal,
			Spread:      spread,
			GNI:         0.0,
			Agitated:    false,
			ObjectCount: len(a.objectKeys),
			Timestamp:   now,
		})
	}

	r.mu.Lock()
	r.groupSnapshots = make(map[string]infra.GroupSnapshot, len(snapshots))
	for _, s := range snapshots {
		r.groupSnapshots[s.Group+"|"+s.Namespace] = s
	}
	r.mu.Unlock()

	namespaces := make(map[string]struct{})
	for _, s := range snapshots {
		if s.Namespace != "" {
			namespaces[s.Namespace] = struct{}{}
		}
	}
	for ns := range namespaces {
		r.propagator.Tick(ns, snapshots)
	}
}
