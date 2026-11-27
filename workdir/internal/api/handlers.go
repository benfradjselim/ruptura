package api

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/benfradjselim/ruptura/internal/actions/engine"
	"github.com/benfradjselim/ruptura/internal/collector/infra/dag"
	"github.com/benfradjselim/ruptura/internal/discovery"
	"github.com/benfradjselim/ruptura/internal/storage"
	"github.com/benfradjselim/ruptura/internal/telemetry"
)

// IngestCounter exposes cumulative ingest counts from the ingest engine.
type IngestCounter interface {
	IngestCounts() (metrics, logs, traces int64)
}

// Injector reports a synthetic failure on a workload, in demo mode.
type Injector interface {
	Inject(workload, pattern string, now time.Time) error
}

// Handlers serves the Ruptura REST API.
type Handlers struct {
	store             *storage.Store
	engine            *engine.Engine
	metrics           *telemetry.Registry
	health            *telemetry.HealthChecker
	startTime         time.Time
	ready             int32  // atomic: 1=ready
	apiKey            string // expected bearer token; "" disables auth
	edition           string // "community" (default) or "autopilot"
	version           string
	ingest            IngestCounter
	discovery         *discovery.Informer
	infraRegistry     *dag.Registry
	demoMode          bool
	injector          Injector
	failureModes      FailureEstimates
	callGraph         CallGraphSource
	namespaceSelector map[string]string
	objectFindings    ObjectFindings
	cardInfo          map[string]CardInfo
	namespaceLabels   NamespaceLabels
	inputs            InputCoverage
	horizon           time.Duration
	levelScope        string
	levelSelector     map[string]string
}

// SetDemoMode marks the server as running on synthetic failures
// (--demo / RUPTURA_DEMO_MODE=true), reported in /api/v2/health so the console
// shows its demo banner.
func (h *Handlers) SetDemoMode(v bool) { h.demoMode = v }

// SetInjector wires the demo failure source behind POST /api/v2/sim/inject.
func (h *Handlers) SetInjector(i Injector) { h.injector = i }

// SetIngest wires the ingest counters.
func (h *Handlers) SetIngest(c IngestCounter) { h.ingest = c }

// SetDiscovery wires the Kubernetes informer.
func (h *Handlers) SetDiscovery(d *discovery.Informer) { h.discovery = d }

// SetEdition sets the product edition: "community" (default) or "autopilot".
func (h *Handlers) SetEdition(e string) { h.edition = e }

// SetVersion sets the server version string returned in /api/v2/health.
func (h *Handlers) SetVersion(v string) { h.version = v }

// New returns API handlers over the store and the action engine.
func New(store *storage.Store, eng *engine.Engine, met *telemetry.Registry, hc *telemetry.HealthChecker, apiKey string) *Handlers {
	return &Handlers{store: store, engine: eng, metrics: met, health: hc, startTime: time.Now(), apiKey: apiKey}
}

func (h *Handlers) SetReady(v bool) {
	if v {
		atomic.StoreInt32(&h.ready, 1)
	} else {
		atomic.StoreInt32(&h.ready, 0)
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
