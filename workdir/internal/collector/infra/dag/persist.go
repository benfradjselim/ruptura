package dag

import "time"

// Persister is the subset of *storage.Store the registry needs to persist
// infra signal history (FBL-A3-1). Defined on the consumer side (not
// imported from internal/storage) to keep this package free of a storage
// dependency when persistence isn't wired up — SetPersister is optional.
type Persister interface {
	PutInfraSignal(group, scope, ns, kind, name, signal string, ts time.Time, value float64) error
	PutGroupHealth(group, ns string, ts time.Time, health float64) error
	PutGroupNoise(group string, ts time.Time, gni float64) error
	PutPropagationSnapshot(ts time.Time, payload []byte) error
}

// SetPersister wires a Persister into the registry. Must be called before
// Run() for the first tick to be persisted; nil (the default) means infra
// signal history is not written — the registry behaves exactly as before
// FBL-A3-1.
func (r *Registry) SetPersister(p Persister) {
	r.mu.Lock()
	r.persister = p
	r.mu.Unlock()
}
