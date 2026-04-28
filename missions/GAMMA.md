# Mission GAMMA — Actions Wiring + Maintenance Windows + API Stubs + Narrative Explain

## Context
Ruptura has:
- An action engine with T1/T2/T3 tiers that is never triggered by anomalies
- An alerter with suppression/maintenance-window support that is a stub
- REST API handlers for rupture/KPI/forecast/suppress/explain that return empty arrays
- An explain engine that stores rupture records but the API handler returns 404

All of this must be wired and implemented tonight.

## Files you OWN (only touch these)
- `workdir/internal/alerter/alerter.go`
- `workdir/internal/actions/engine/engine.go`
- `workdir/internal/actions/arbitration/arbitrator.go` (read only unless bug fix needed)
- `workdir/internal/explain/explain.go`
- `workdir/internal/api/handlers_extra.go`
- `workdir/internal/api/handlers.go` (read only — understand Handlers struct)
- `workdir/internal/api/router.go` (add new workload routes, do not remove existing ones)
- `workdir/internal/notifier/notifier.go` (update payload struct only)

## Files you must NOT touch
- internal/analyzer/* (Agent ALPHA owns this)
- internal/ingest/* (Agent ALPHA owns this)
- internal/fusion/* (Agent BETA owns this)
- internal/correlator/* (Agent BETA owns this)
- pkg/models/models.go or workload.go (Agent ALPHA owns these)

---

## Task 1 — Implement Maintenance Windows in Alerter

In `workdir/internal/alerter/alerter.go`, add:

```go
// MaintenanceWindow suppresses rupture alarms for a specific workload during
// planned maintenance (deploys, restarts, migrations).
type MaintenanceWindow struct {
    ID          string    `json:"id"`
    WorkloadKey string    `json:"workload_key"` // WorkloadRef.Key() or "*" for cluster-wide
    From        time.Time `json:"from"`
    Until       time.Time `json:"until"`
    Reason      string    `json:"reason,omitempty"`
}
```

Add to `Alerter` struct:
```go
windows   []MaintenanceWindow
windowsMu sync.RWMutex
```

Add methods:
```go
// AddMaintenanceWindow registers a suppression window.
func (a *Alerter) AddMaintenanceWindow(w MaintenanceWindow) string {
    if w.ID == "" {
        w.ID = utils.GenerateID(8)
    }
    a.windowsMu.Lock()
    a.windows = append(a.windows, w)
    a.windowsMu.Unlock()
    return w.ID
}

// RemoveMaintenanceWindow removes a suppression window by ID.
func (a *Alerter) RemoveMaintenanceWindow(id string) {
    a.windowsMu.Lock()
    defer a.windowsMu.Unlock()
    filtered := a.windows[:0]
    for _, w := range a.windows {
        if w.ID != id {
            filtered = append(filtered, w)
        }
    }
    a.windows = filtered
}

// ListMaintenanceWindows returns all currently active windows.
func (a *Alerter) ListMaintenanceWindows() []MaintenanceWindow {
    a.windowsMu.RLock()
    defer a.windowsMu.RUnlock()
    now := time.Now()
    var active []MaintenanceWindow
    for _, w := range a.windows {
        if w.Until.After(now) {
            active = append(active, w)
        }
    }
    return active
}

// isSuppressed returns true if the given workload key is currently in a maintenance window.
func (a *Alerter) isSuppressed(workloadKey string, now time.Time) bool {
    a.windowsMu.RLock()
    defer a.windowsMu.RUnlock()
    for _, w := range a.windows {
        if w.Until.Before(now) {
            continue
        }
        if w.From.After(now) {
            continue
        }
        if w.WorkloadKey == "*" || w.WorkloadKey == workloadKey {
            return true
        }
    }
    return false
}
```

In the `Evaluate` (or equivalent fire method) of the Alerter, before dispatching:
```go
// Look up workload key from the host in the alert (use WorkloadRefFromHost as fallback)
workloadKey := "default/host/" + host
if a.isSuppressed(workloadKey, time.Now()) {
    return // silently skip during maintenance
}
```

---

## Task 2 — Wire Anomaly Engine to Action Engine

In `workdir/internal/actions/engine/engine.go`, add a method that accepts
an anomaly event and creates a RuptureEvent from it:

```go
// RecommendFromAnomaly translates a critical anomaly event into a RuptureEvent
// and returns the recommended actions. Only processes SeverityCritical anomalies.
func (e *Engine) RecommendFromAnomaly(ev models.AnomalyEvent) ([]ActionRecommendation, error) {
    if ev.Severity != models.SeverityCritical {
        return nil, nil // only act on consensus anomalies (≥2 methods)
    }
    profile := "spike"
    if ev.Score > 5.0 {
        profile = "spike"
    } else {
        profile = "plateau"
    }
    rupture := RuptureEvent{
        ID:         utils.GenerateID(8),
        Host:       ev.Host,
        Metric:     ev.Metric,
        R:          ev.Score,
        Confidence: 0.75, // anomaly consensus = moderate confidence
        Profile:    profile,
        Timestamp:  ev.Timestamp,
    }
    return e.Recommend(rupture)
}
```

Also look at how `Recommend` in `engine.go` is currently implemented (read the full file).
If `Recommend` is a stub, implement it properly:

```go
func (e *Engine) Recommend(event RuptureEvent) ([]ActionRecommendation, error) {
    if atomic.LoadInt32(&e.emergencyStopped) == 1 {
        return nil, fmt.Errorf("emergency stop active")
    }
    var recs []ActionRecommendation
    for _, rule := range e.rules {
        if rule.MinR > event.R {
            continue
        }
        if rule.Profile != "" && rule.Profile != event.Profile {
            continue
        }
        tier := Tier3
        conf := event.Confidence
        if conf >= 0.85 {
            tier = Tier1
        } else if conf >= 0.60 {
            tier = Tier2
        }
        rec := ActionRecommendation{
            ID:         utils.GenerateID(8),
            EventID:    event.ID,
            Host:       event.Host,
            ActionType: rule.ActionType,
            Tier:       tier,
            Confidence: conf,
            Timestamp:  time.Now(),
        }
        recs = append(recs, rec)
        if e.bus != nil {
            _ = e.bus.Publish("action.recommended", rec)
        }
    }
    return recs, nil
}
```

---

## Task 3 — Implement Narrative Explain

In `workdir/internal/explain/explain.go`, add `NarrativeExplain`:

```go
// NarrativeExplain returns a human-readable explanation of a rupture event.
// It is a structured template filled from the rupture record — no LLM required.
func (e *Engine) NarrativeExplain(id string) (string, error) {
    val, ok := e.records.Load(id)
    if !ok {
        return "", fmt.Errorf("explain: rupture %s not found", id)
    }
    rec := val.(RuptureRecord)

    // Determine primary pipeline
    primaryPipeline := "metric"
    primaryR := rec.MetricR
    if rec.LogR > primaryR {
        primaryPipeline = "log"
        primaryR = rec.LogR
    }
    if rec.TraceR > primaryR {
        primaryPipeline = "trace"
    }

    // Severity label
    severity := "warning"
    if rec.R >= 5.0 {
        severity = "critical"
    } else if rec.R >= 3.0 {
        severity = "elevated"
    }

    // Find the top contributing metric
    topMetric := "unknown"
    topWeight := 0.0
    for _, m := range rec.Metrics {
        if m.Weight > topWeight {
            topMetric = m.Metric
            topWeight = m.Weight
        }
    }

    // Build TTF description
    ttfDesc := ""
    if rec.TTFSeconds > 0 {
        mins := int(rec.TTFSeconds / 60)
        if mins < 1 {
            ttfDesc = fmt.Sprintf(" TTF was %ds.", int(rec.TTFSeconds))
        } else {
            ttfDesc = fmt.Sprintf(" TTF was %d minutes.", mins)
        }
    }

    // Contagion note
    contagionNote := ""
    if rec.LogR > 1.0 {
        contagionNote = " Log burst signals indicate contagion may have spread from a dependency."
    }
    if rec.TraceR > 1.0 {
        contagionNote = " Trace error propagation detected — check service dependency graph."
    }

    narrative := fmt.Sprintf(
        "[%s] Rupture %s on %s — R=%.2f (%s). "+
            "Primary signal: %s pipeline (R=%.2f). "+
            "Top contributing factor: %s (weight=%.0f%%)."+
            "%s%s",
        rec.Timestamp.UTC().Format("2006-01-02 15:04:05 UTC"),
        rec.ID,
        rec.Host,
        rec.R, severity,
        primaryPipeline, primaryR,
        topMetric, topWeight*100,
        ttfDesc, contagionNote,
    )
    return narrative, nil
}
```

---

## Task 4 — Implement API Handlers (handleRupture, handleKPI, handleForecast, handleSuppressions, handleExplain)

Read `workdir/internal/api/handlers.go` first to understand the `Handlers` struct and what
fields are available (store, engine, explainer, etc.).

Then implement in `workdir/internal/api/handlers_extra.go`:

### handleRupture

```go
func (h *Handlers) handleRupture(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    host := vars["host"]
    // Try to get KPI snapshot from the analyzer via the store or engine
    // Use h.store to query KPI snapshot for this host
    snap, ok := h.store.LatestSnapshot(host)
    if !ok {
        writeError(w, http.StatusNotFound, "no data for host: "+host)
        return
    }
    writeJSON(w, http.StatusOK, snap)
}
```

For the `GET /api/v2/ruptures` (all hosts) route:
```go
func (h *Handlers) handleRuptures(w http.ResponseWriter, r *http.Request) {
    snapshots := h.store.AllSnapshots()
    writeJSON(w, http.StatusOK, snapshots)
}
```

Check what methods `storage.Store` exposes in `internal/storage/storage.go`.
If `LatestSnapshot` and `AllSnapshots` don't exist, add them to `storage.go`.
If the store doesn't hold KPI snapshots at all, look at what it does hold and adapt.
The goal is: handleRupture returns a real KPISnapshot, not an empty array.

### handleKPI

```go
func (h *Handlers) handleKPI(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    name := vars["name"]
    host := vars["host"]

    snap, ok := h.store.LatestSnapshot(host)
    if !ok {
        writeError(w, http.StatusNotFound, "no data for host: "+host)
        return
    }

    var kpi models.KPI
    switch name {
    case "stress":     kpi = snap.Stress
    case "fatigue":    kpi = snap.Fatigue
    case "mood":       kpi = snap.Mood
    case "pressure":   kpi = snap.Pressure
    case "humidity":   kpi = snap.Humidity
    case "contagion":  kpi = snap.Contagion
    case "resilience": kpi = snap.Resilience
    case "entropy":    kpi = snap.Entropy
    case "velocity":   kpi = snap.Velocity
    case "health_score": kpi = snap.HealthScore
    default:
        writeError(w, http.StatusBadRequest, "unknown KPI: "+name)
        return
    }
    writeJSON(w, http.StatusOK, kpi)
}
```

### handleForecast

```go
func (h *Handlers) handleForecast(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    metric := vars["metric"]
    host   := vars["host"]
    horizon := 60 // default 60 minutes
    if hStr := r.URL.Query().Get("horizon"); hStr != "" {
        if v, err := strconv.Atoi(hStr); err == nil && v > 0 {
            horizon = v
        }
    }
    // h.engine is *actions/engine.Engine — the predictor is separate
    // Look at the main cmd/ruptura/main.go to understand how predictor is wired
    // For now, return a stub with a TODO note that predictor needs to be injected
    writeJSON(w, http.StatusOK, map[string]interface{}{
        "host":    host,
        "metric":  metric,
        "horizon": horizon,
        "note":    "predictor not yet injected into Handlers — wire in cmd/ruptura/main.go",
    })
}
```

Note: if the predictor IS accessible via h.store or h.engine, wire it properly.
Check `cmd/ruptura/main.go` to understand how things are wired. If predictor
is available, use it. If not, leave the stub with the note above.

### handleSuppressions (implement, not stub)

```go
func (h *Handlers) handleSuppressions(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodPost:
        var req struct {
            WorkloadKey string `json:"workload_key"`
            From        string `json:"from"`    // RFC3339 or empty = now
            Until       string `json:"until"`   // RFC3339, required
            Reason      string `json:"reason"`
        }
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
            return
        }
        until, err := time.Parse(time.RFC3339, req.Until)
        if err != nil {
            writeError(w, http.StatusBadRequest, "invalid until: "+err.Error())
            return
        }
        from := time.Now()
        if req.From != "" {
            if f, err := time.Parse(time.RFC3339, req.From); err == nil {
                from = f
            }
        }
        // h.alerter must be added to Handlers struct — see below
        id := h.alerter.AddMaintenanceWindow(alerter.MaintenanceWindow{
            WorkloadKey: req.WorkloadKey,
            From:        from,
            Until:       until,
            Reason:      req.Reason,
        })
        writeJSON(w, http.StatusCreated, map[string]string{"id": id})

    case http.MethodGet:
        writeJSON(w, http.StatusOK, h.alerter.ListMaintenanceWindows())
    }
}
```

You will need to add `alerter *alerter.Alerter` field to `Handlers` in `handlers.go`.
Update `NewHandlers` and `New` to accept it. Update `cmd/ruptura/main.go` to pass it.

### handleExplain (implement, not 404)

```go
func (h *Handlers) handleExplain(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    id := vars["rupture_id"]

    switch {
    case strings.HasSuffix(r.URL.Path, "/formula"):
        audit, err := h.explainer.FormulaAudit(id)
        if err != nil {
            writeError(w, http.StatusNotFound, err.Error())
            return
        }
        writeJSON(w, http.StatusOK, audit)

    case strings.HasSuffix(r.URL.Path, "/pipeline"):
        dbg, err := h.explainer.PipelineDebug(id)
        if err != nil {
            writeError(w, http.StatusNotFound, err.Error())
            return
        }
        writeJSON(w, http.StatusOK, dbg)

    default:
        exp, err := h.explainer.Explain(id)
        if err != nil {
            writeError(w, http.StatusNotFound, err.Error())
            return
        }
        // Also try narrative
        narrative, _ := h.explainer.NarrativeExplain(id)
        writeJSON(w, http.StatusOK, map[string]interface{}{
            "explain":   exp,
            "narrative": narrative,
        })
    }
}
```

---

## Task 5 — Update API Router with workload-aware routes

In `workdir/internal/api/router.go`, add alongside existing routes:

```go
// Workload-centric routes (primary K8s user-facing API)
r.HandleFunc("/api/v2/ruptures", h.handleRuptures).Methods("GET")
r.HandleFunc("/api/v2/rupture/{namespace}/{workload}", h.handleRuptureByWorkload).Methods("GET")
r.HandleFunc("/api/v2/kpi/{name}/{namespace}/{workload}", h.handleKPIByWorkload).Methods("GET")
```

Keep the existing `{host}` routes as backward-compat aliases.

Add `handleRuptureByWorkload` and `handleKPIByWorkload` in `handlers_extra.go`
that extract namespace+workload from vars and call the same logic as the host-based handlers,
but using `namespace/kind/name` as the lookup key.

---

## Verification

Run: `cd /root/ruptura/workdir && go build ./... && go test -race ./...`
All existing tests must pass.
Add tests for:
- `alerter.isSuppressed` returns true when inside a window, false outside
- `engine.RecommendFromAnomaly` returns non-empty recommendations for SeverityCritical
- `explain.NarrativeExplain` returns a non-empty string containing the host name
- `handleRupture` returns 404 for unknown host, 200 with KPISnapshot for known host
