package api

import (
    "encoding/json"
    "net/http"
    "sync/atomic"
    "time"

    "github.com/benfradjselim/ruptura/internal/actions/engine"
    "github.com/benfradjselim/ruptura/internal/alerter"
    "github.com/benfradjselim/ruptura/internal/analyzer"
    apicontext "github.com/benfradjselim/ruptura/internal/context"
    "github.com/benfradjselim/ruptura/internal/explain"
    pipelinemetrics "github.com/benfradjselim/ruptura/internal/pipeline/metrics"
    "github.com/benfradjselim/ruptura/internal/predictor"
    "github.com/benfradjselim/ruptura/internal/storage"
    "github.com/benfradjselim/ruptura/internal/telemetry"
)

type Handlers struct {
    store      *storage.Store
    engine     *engine.Engine
    explainer  *explain.Engine
    alerter    *alerter.Alerter
    predictor  *predictor.Predictor
    pipeline   pipelinemetrics.MetricPipeline
    ctxStore   *apicontext.ManualContextStore
    detector   *apicontext.DeploymentDetector
    metrics    *telemetry.Registry
    health     *telemetry.HealthChecker
    startTime  time.Time
    ready      int32  // atomic: 1=ready
    apiKey     string // expected bearer token; "" disables auth
    // v6.3: calibration + forecast enrichment
    analyzer   *analyzer.Analyzer
}

// SetAnalyzer wires the analyzer for calibration status and HealthScore forecasting.
func (h *Handlers) SetAnalyzer(a *analyzer.Analyzer) { h.analyzer = a }

func NewHandlers(
    store *storage.Store,
    eng *engine.Engine,
    exp *explain.Engine,
    al  *alerter.Alerter,
    pred *predictor.Predictor,
    pipe pipelinemetrics.MetricPipeline,
    ctx *apicontext.ManualContextStore,
    det *apicontext.DeploymentDetector,
    met *telemetry.Registry,
    hc  *telemetry.HealthChecker,
    apiKey string,
) *Handlers {
    return &Handlers{
        store: store, engine: eng, explainer: exp, alerter: al,
        predictor: pred, pipeline: pipe, ctxStore: ctx, detector: det,
        metrics: met, health: hc, startTime: time.Now(), apiKey: apiKey,
    }
}

// New is an alias for NewHandlers.
func New(
	store *storage.Store,
	eng *engine.Engine,
	exp *explain.Engine,
	al  *alerter.Alerter,
	pred *predictor.Predictor,
	pipe pipelinemetrics.MetricPipeline,
	ctx *apicontext.ManualContextStore,
	det *apicontext.DeploymentDetector,
	met *telemetry.Registry,
	hc *telemetry.HealthChecker,
	apiKey string,
) *Handlers {
	return NewHandlers(store, eng, exp, al, pred, pipe, ctx, det, met, hc, apiKey)
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
