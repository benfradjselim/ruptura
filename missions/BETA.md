# Mission BETA — Fusion Wiring + OTLP Route Fix + Topology Contagion

## Context
Ruptura has a fusion engine (`internal/fusion/`) that is designed to combine
three signal pipelines — metric R, log R, and trace R — into a single FusedR
per workload. This engine exists and the math is correct, but nothing feeds it.

Additionally:
- The OTLP routes in the main API router (`/api/v2/v1/*`) are dead stubs that
  accept payloads and return 204 without processing anything.
- The BurstDetector in `internal/correlator/` fires burst events on log error/warn
  surges, but its output channel is never consumed.
- Trace spans are ingested but span latency/error status is never converted to
  a trace rupture signal.
- Contagion in the analyzer uses `errors × cpu` as a proxy. A topology-based
  approach (service call edges from traces) must replace it.

## Files you OWN (only touch these)
- `workdir/internal/fusion/fusion.go`
- `workdir/internal/correlator/correlator.go`
- `workdir/internal/correlator/bursts.go` (read only to understand BurstEvent)
- `workdir/internal/receiver/otlp.go` (enhance span/log parsing)
- `workdir/internal/api/router.go` (remove dead OTLP stubs)
- `workdir/internal/api/handlers_extra.go` (remove/fix handleOTLP stub)
- `workdir/pkg/models/trace.go` (read only — do not modify)

## Files you must NOT touch
- internal/analyzer/* (Agent ALPHA owns this)
- internal/ingest/* (Agent ALPHA owns this)
- internal/actions/* (Agent GAMMA owns this)
- internal/alerter/* (Agent GAMMA owns this)
- internal/explain/* (Agent GAMMA owns this)
- pkg/models/models.go or workload.go (Agent ALPHA owns these)

---

## Task 1 — Wire BurstDetector output → logR → fusion

The BurstDetector is in `internal/correlator/correlator.go`.
It has a channel `Events() <-chan models.BurstEvent` that fires when
a service's error/warn log rate exceeds mean + 3σ.

In `internal/fusion/fusion.go`, add a method to consume from this channel:

```go
// StartLogWatcher consumes BurstEvents from the correlator and updates logR.
// Call this once at startup in a goroutine.
func (e *Engine) StartLogWatcher(ctx context.Context, events <-chan models.BurstEvent) {
    go func() {
        for {
            select {
            case <-ctx.Done():
                return
            case ev, ok := <-events:
                if !ok {
                    return
                }
                // Normalize burst score to a rupture contribution [0, ∞)
                // BurstEvent.Score is the number of σ above baseline
                // R=1.0 means 1σ above normal; R=3.0 means critical
                logR := ev.Score / 3.0
                e.SetLogR(ev.Service, logR, ev.Timestamp)
            }
        }
    }()
}
```

Check what fields `models.BurstEvent` has in `pkg/models/` and adapt accordingly.
If `BurstEvent` has a `Rate` and `BaselineRate`, use `logR = Rate / BaselineRate - 1.0`.

---

## Task 2 — Wire trace spans → traceR → fusion

In `internal/receiver/otlp.go`, the `TraceHandler` processes spans per service.
Add a per-service span accumulator that computes traceR:

Add a `traceAggregator` struct inside the OTLPReceiver (or as a separate helper):

```go
type spanWindow struct {
    mu          sync.Mutex
    total       int
    errors      int
    durationSum int64 // nanoseconds
    lastFlush   time.Time
}

// update adds a span to the window. Returns (errorRate, avgLatencyMS, flush) when
// enough spans have accumulated (≥10) or 15 seconds have passed.
func (w *spanWindow) update(durationNS int64, isError bool, ts time.Time) (float64, float64, bool) {
    w.mu.Lock()
    defer w.mu.Unlock()
    w.total++
    w.durationSum += durationNS
    if isError {
        w.errors++
    }
    shouldFlush := w.total >= 10 || ts.Sub(w.lastFlush) >= 15*time.Second
    if !shouldFlush {
        return 0, 0, false
    }
    errorRate := float64(w.errors) / float64(w.total)
    avgLatMS := float64(w.durationSum) / float64(w.total) / 1e6
    w.total, w.errors, w.durationSum = 0, 0, 0
    w.lastFlush = ts
    return errorRate, avgLatMS, true
}
```

Add `spanWindows sync.Map` to `OTLPReceiver` (key = service name).
Add a `FusionSink` interface field to `OTLPReceiver`:

```go
type TraceFusionSink interface {
    SetTraceR(host string, r float64, ts time.Time)
}
```

In `TraceHandler`, after parsing each span:
```go
isError := span.Status.Code == 2
window := r.getOrCreateWindow(service)
if errRate, avgLatMS, ok := window.update(span.DurationNS, isError, start); ok {
    // Normalize: errRate [0,1] + latency normalized against 200ms baseline
    latScore := math.Min(avgLatMS/200.0, 3.0) / 3.0 // 0=fast, 1=very slow
    traceR := 0.6*errRate + 0.4*latScore
    if r.fusion != nil {
        r.fusion.SetTraceR(service, traceR, start)
    }
}
```

Make `fusion TraceFusionSink` an optional field on `OTLPReceiver` (nil = disabled).
Update `NewOTLPReceiver` to accept it as a parameter (can be nil).

---

## Task 3 — Build service topology from trace spans

In `internal/correlator/correlator.go`, add a `TopologyBuilder` alongside `BurstDetector`:

```go
// TopologyBuilder builds a live service dependency graph from trace spans.
// It tracks which services call which by correlating parent-child span relationships.
type TopologyBuilder struct {
    mu    sync.RWMutex
    edges map[string]*edgeStats // key: "from→to"
}

type edgeStats struct {
    From   string
    To     string
    Calls  int64
    Errors int64
    TotalLatencyNS int64
}

func NewTopologyBuilder() *TopologyBuilder {
    return &TopologyBuilder{edges: make(map[string]*edgeStats)}
}

// ObserveSpan records a span. If it has a parent, it records the From→To edge.
// service is the current span's service; parentService is the caller's service
// (derived from the parent span if available, empty otherwise).
func (t *TopologyBuilder) ObserveSpan(service, parentService string, durationNS int64, isError bool) {
    if parentService == "" || parentService == service {
        return
    }
    key := parentService + "→" + service
    t.mu.Lock()
    e, ok := t.edges[key]
    if !ok {
        e = &edgeStats{From: parentService, To: service}
        t.edges[key] = e
    }
    e.Calls++
    e.TotalLatencyNS += durationNS
    if isError {
        e.Errors++
    }
    t.mu.Unlock()
}

// Edges returns a snapshot of current service edges.
func (t *TopologyBuilder) Edges() []models.ServiceEdge {
    t.mu.RLock()
    defer t.mu.RUnlock()
    out := make([]models.ServiceEdge, 0, len(t.edges))
    for _, e := range t.edges {
        avgLat := 0.0
        if e.Calls > 0 {
            avgLat = float64(e.TotalLatencyNS) / float64(e.Calls) / 1e6
        }
        out = append(out, models.ServiceEdge{
            From:     e.From,
            To:       e.To,
            Calls:    e.Calls,
            Errors:   e.Errors,
            AvgLatMS: avgLat,
        })
    }
    return out
}
```

In `internal/receiver/otlp.go` `TraceHandler`, after parsing each span:
```go
if r.topology != nil {
    r.topology.ObserveSpan(service, parentService, durationNS, isError)
}
```

Where `parentService` is resolved by looking up the parent span's service in a local
span cache (keyed by SpanID → Service). Add a small `sync.Map` for this: `spanCache`.
Cache entry: `spanID → serviceName`. Evict entries older than 5 minutes.

Add `topology *correlator.TopologyBuilder` as optional field on `OTLPReceiver`.

---

## Task 4 — Fix the dead OTLP stubs in API router

In `workdir/internal/api/router.go`, remove or replace these three routes:
```go
r.HandleFunc("/api/v2/v1/metrics", h.handleOTLP).Methods("POST")
r.HandleFunc("/api/v2/v1/logs", h.handleOTLP).Methods("POST")
r.HandleFunc("/api/v2/v1/traces", h.handleOTLP).Methods("POST")
```

Replace with a single informational route that returns a 301 redirect or a
JSON error explaining the correct port:

```go
r.HandleFunc("/api/v2/v1/{signal:metrics|logs|traces}", func(w http.ResponseWriter, r *http.Request) {
    writeJSON(w, http.StatusMisdirectedRequest, map[string]string{
        "error": "OTLP ingestion runs on a separate port. Send to :4318/otlp/v1/{metrics,logs,traces}",
        "docs":  "https://benfradjselim.github.io/ruptura/",
    })
}).Methods("POST")
```

In `workdir/internal/api/handlers_extra.go`, remove the `handleOTLP` method entirely
(it is now handled by the inline handler above).

---

## Task 5 — Expose FusedR from fusion engine

In `internal/fusion/fusion.go`, the existing `FusedR(host string)` method returns
the fused value. Add a method that returns all hosts' fused values as a snapshot:

```go
// Snapshot returns a map of workload key → FusedR value for all known workloads.
func (e *Engine) Snapshot() map[string]float64 {
    e.mu.RLock()
    defer e.mu.RUnlock()
    out := make(map[string]float64, len(e.hosts))
    for host, h := range e.hosts {
        r, _ := e.fusedR(h)
        out[host] = r
    }
    return out
}
```

Also look at the existing `FusedR` formula (it calls an internal method).
If there is a staleness guard (e.g. ignore signals older than 5 minutes), keep it.
If not, add one:
```go
const staleThreshold = 5 * time.Minute
// in FusedR computation: if signal timestamp is older than staleThreshold, weight it 0
```

---

## Verification

Run: `cd /root/ruptura/workdir && go build ./... && go test -race ./...`
All existing tests must pass.
Write at least one test in `internal/fusion/fusion_test.go` covering:
- `StartLogWatcher` receives a BurstEvent and updates logR
- `FusedR` returns non-zero when any of the three R values is non-zero
