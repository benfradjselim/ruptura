package api

import (
    "encoding/json"
    "net/http"
    "sync/atomic"
    "time"

    "github.com/benfradjselim/kairo-core/internal/actions/engine"
    apicontext "github.com/benfradjselim/kairo-core/internal/context"
    "github.com/benfradjselim/kairo-core/internal/explain"
    "github.com/benfradjselim/kairo-core/internal/storage"
    "github.com/benfradjselim/kairo-core/internal/telemetry"
)

type Handlers struct {
    store      *storage.Store
    engine     *engine.Engine
    explainer  *explain.Engine
    ctxStore   *apicontext.ManualContextStore
    detector   *apicontext.DeploymentDetector
    metrics    *telemetry.Registry
    health     *telemetry.HealthChecker
    startTime  time.Time
    ready      int32  // atomic: 1=ready
    apiKey     string // expected bearer token; "" disables auth
}

func NewHandlers(
    store *storage.Store,
    eng *engine.Engine,
    exp *explain.Engine,
    ctx *apicontext.ManualContextStore,
    det *apicontext.DeploymentDetector,
    met *telemetry.Registry,
    hc  *telemetry.HealthChecker,
    apiKey string,
) *Handlers {
    return &Handlers{
        store: store, engine: eng, explainer: exp, ctxStore: ctx,
        detector: det, metrics: met, health: hc,
        startTime: time.Now(), apiKey: apiKey,
    }
}

// New is an alias for NewHandlers.
func New(
	store *storage.Store,
	eng *engine.Engine,
	exp *explain.Engine,
	ctx *apicontext.ManualContextStore,
	det *apicontext.DeploymentDetector,
	met *telemetry.Registry,
	hc *telemetry.HealthChecker,
	apiKey string,
) *Handlers {
	return NewHandlers(store, eng, exp, ctx, det, met, hc, apiKey)
}

func (h *Handlers) SetReady(v bool) {
    if v { atomic.StoreInt32(&h.ready, 1) } else { atomic.StoreInt32(&h.ready, 0) }
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, msg string) {
    writeJSON(w, status, map[string]string{"error": msg})
}
