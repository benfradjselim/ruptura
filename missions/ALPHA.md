# Mission ALPHA — Core Model + Engine Consolidation

## Context
Ruptura is a workload health monitoring engine for Kubernetes clusters.
It currently uses `host` (a node/machine name) as the atomic unit of observation.
This is wrong for K8s: a single Deployment has pods across many nodes.
The treatment unit must become `WorkloadRef` (namespace + kind + name).

There are also two parallel composite engines computing overlapping signals:
- `internal/composites/engine.go` — 7 signals, graph-based Contagion, EWMA Pressure
- `internal/analyzer/analyzer.go` — 10 KPIs, canonical engine going forward

The composites engine must be retired. Its superior Contagion (graph-based) and
Pressure (EWMA z-score) formulas must be ported into the analyzer.

The analyzer also needs two new capabilities:
- Adaptive per-workload baselines (signals relative to each workload's normal)
- Throughput collapse signal (captures when request_rate drops suddenly)

## Files you OWN (only touch these)
- `workdir/pkg/models/workload.go` — CREATE THIS FILE
- `workdir/pkg/models/models.go` — add WorkloadRef fields
- `workdir/internal/analyzer/analyzer.go` — WorkloadRef keying + new signals + baselines
- `workdir/internal/ingest/engine.go` — extract k8s OTLP attributes into WorkloadRef
- `workdir/internal/composites/engine.go` — add deprecation notice, do NOT delete (tests reference it)

## Files you must NOT touch
- internal/api/* (Agent GAMMA owns this)
- internal/fusion/* (Agent BETA owns this)
- internal/correlator/* (Agent BETA owns this)
- internal/actions/* (Agent GAMMA owns this)
- internal/alerter/* (Agent GAMMA owns this)
- internal/explain/* (Agent GAMMA owns this)

---

## Task 1 — Create `workdir/pkg/models/workload.go`

```go
package models

// WorkloadRef identifies the primary treatment unit in Ruptura.
// In Kubernetes: namespace + kind + name identifies a workload uniquely.
// For non-K8s (bare-metal, VMs): Namespace="default", Kind="host", Name=hostname.
type WorkloadRef struct {
    Cluster   string `json:"cluster,omitempty"`   // optional, defaults to "default"
    Namespace string `json:"namespace"`
    Kind      string `json:"kind"`   // Deployment|StatefulSet|DaemonSet|Job|host
    Name      string `json:"name"`   // workload name
    Node      string `json:"node,omitempty"` // infra node — secondary dimension
}

// Key returns the canonical string key used as map key throughout the engine.
func (w WorkloadRef) Key() string {
    if w.Namespace == "" {
        return "default/host/" + w.Name
    }
    return w.Namespace + "/" + w.Kind + "/" + w.Name
}

// IsEmpty returns true when the WorkloadRef carries no meaningful identity.
func (w WorkloadRef) IsEmpty() bool {
    return w.Name == ""
}

// WorkloadRefFromHost creates a degraded WorkloadRef for non-K8s sources.
// This preserves backward compatibility for bare-metal and VM deployments.
func WorkloadRefFromHost(host string) WorkloadRef {
    return WorkloadRef{
        Namespace: "default",
        Kind:      "host",
        Name:      host,
        Node:      host,
    }
}

// firstNonEmpty returns the first non-empty string from the list.
func FirstNonEmpty(vals ...string) string {
    for _, v := range vals {
        if v != "" {
            return v
        }
    }
    return ""
}
```

---

## Task 2 — Update `workdir/pkg/models/models.go`

Add `Workload WorkloadRef` to `Metric`, `KPI`, and `KPISnapshot`.
Keep `Host string` field on all structs — do not remove it, other code references it.

```go
type Metric struct {
    Name      string            `json:"name"`
    Value     float64           `json:"value"`
    Timestamp time.Time         `json:"timestamp"`
    Labels    map[string]string `json:"labels,omitempty"`
    Host      string            `json:"host"`
    Workload  WorkloadRef       `json:"workload,omitempty"`  // ADD THIS
}

type KPI struct {
    Name      string      `json:"name"`
    Value     float64     `json:"value"`
    State     string      `json:"state"`
    Timestamp time.Time   `json:"timestamp"`
    Host      string      `json:"host"`
    Workload  WorkloadRef `json:"workload,omitempty"`  // ADD THIS
}

type KPISnapshot struct {
    Host        string      `json:"host"`
    Workload    WorkloadRef `json:"workload,omitempty"`  // ADD THIS
    Timestamp   time.Time   `json:"timestamp"`
    // ... all existing signal fields unchanged
}
```

---

## Task 3 — Update `workdir/internal/ingest/engine.go`

Add a helper function `extractWorkloadRef` and use it in all three OTLP handlers
(`handleOTLPMetrics`, `handleOTLPLogs`, `handleOTLPTraces`) and in `handleRemoteWrite`.

```go
func extractWorkloadRef(r models.OTLPResource) models.WorkloadRef {
    ns := r.GetAttr("k8s.namespace.name")
    node := models.FirstNonEmpty(r.GetAttr("k8s.node.name"), r.GetAttr("host.name"))
    name := models.FirstNonEmpty(
        r.GetAttr("k8s.deployment.name"),
        r.GetAttr("k8s.statefulset.name"),
        r.GetAttr("k8s.daemonset.name"),
        r.GetAttr("k8s.job.name"),
        r.GetAttr("service.name"),
        node, // final fallback: use node as identity (non-K8s)
    )
    kind := inferWorkloadKind(r)
    if ns == "" {
        ns = "default"
    }
    return models.WorkloadRef{Namespace: ns, Kind: kind, Name: name, Node: node}
}

func inferWorkloadKind(r models.OTLPResource) string {
    switch {
    case r.GetAttr("k8s.deployment.name") != "":
        return "Deployment"
    case r.GetAttr("k8s.statefulset.name") != "":
        return "StatefulSet"
    case r.GetAttr("k8s.daemonset.name") != "":
        return "DaemonSet"
    case r.GetAttr("k8s.job.name") != "":
        return "Job"
    default:
        return "host"
    }
}
```

In `handleOTLPMetrics`, attach the WorkloadRef to the metric:
```go
ref := extractWorkloadRef(rm.Resource)
// when calling pipeline.Ingest, also store ref
metric := models.Metric{
    Name:      sanitizeName(m.Name),
    Value:     value,
    Timestamp: ts,
    Host:      ref.Node,
    Workload:  ref,
}
```

In `handleRemoteWrite`, derive WorkloadRef from labels:
```go
host := "unknown"
workload := models.WorkloadRef{}
for _, lbl := range ts.Labels {
    switch lbl.Name {
    case "__name__":
        name = lbl.Value
    case "host", "instance":
        host = lbl.Value
    case "namespace":
        workload.Namespace = lbl.Value
    case "deployment":
        workload.Name = lbl.Value
        workload.Kind = "Deployment"
    }
}
if workload.IsEmpty() {
    workload = models.WorkloadRefFromHost(host)
}
```

---

## Task 4 — Update `workdir/internal/analyzer/analyzer.go`

### 4a — Key by WorkloadRef instead of host string

Change the state map key:
```go
type Analyzer struct {
    mu        sync.RWMutex
    workloads map[string]*workloadState   // key = WorkloadRef.Key()
    snapshots map[string]models.KPISnapshot
    // ... existing config fields unchanged
}
```

Update `getOrCreate(host string)` to `getOrCreate(ref models.WorkloadRef)`.
Use `ref.Key()` as the map key.
Store `ref` on `workloadState` so snapshots can carry it.

Update `Update(host string, metrics map[string]float64)` signature to:
```go
func (a *Analyzer) Update(ref models.WorkloadRef, metrics map[string]float64) models.KPISnapshot
```

Keep a backward-compat wrapper:
```go
func (a *Analyzer) UpdateHost(host string, metrics map[string]float64) models.KPISnapshot {
    return a.Update(models.WorkloadRefFromHost(host), metrics)
}
```

In `KPISnapshot` construction, set both `Host` and `Workload`:
```go
snap := models.KPISnapshot{
    Host:      ref.Node,
    Workload:  ref,
    Timestamp: now,
    // ... signals
}
```

### 4b — Port EWMA Pressure from composites engine

Replace the current Pressure formula (derivative-based) with the EWMA z-score approach from `internal/composites/engine.go`:

Add to `workloadState`:
```go
muLat, sigma2Lat float64  // EWMA mean and variance for latency
muErr, sigma2Err float64  // EWMA mean and variance for error_rate
```

In `Update()`, compute pressure as:
```go
lat := getMetric(metrics, "latency")
if lat == 0 { lat = getMetric(metrics, "load_avg_1") }
errRate := getMetric(metrics, "error_rate")

hs.muLat   = 0.9*hs.muLat   + 0.1*lat
hs.sigma2Lat = 0.9*hs.sigma2Lat + 0.1*math.Pow(lat-hs.muLat, 2)
hs.muErr   = 0.9*hs.muErr   + 0.1*errRate
hs.sigma2Err = 0.9*hs.sigma2Err + 0.1*math.Pow(errRate-hs.muErr, 2)

sigmaLat := math.Sqrt(hs.sigma2Lat)
if sigmaLat < 1e-6 { sigmaLat = 1.0 }
sigmaErr := math.Sqrt(hs.sigma2Err)
if sigmaErr < 1e-6 { sigmaErr = 1.0 }

latencyZ := (lat - hs.muLat) / sigmaLat
errorZ   := (errRate - hs.muErr) / sigmaErr
rawPressure := 0.5*latencyZ + 0.5*errorZ
pressureNorm := utils.Clamp((rawPressure+3)/6.0, 0, 1) // map [-3,+3] z-score to [0,1]
```

### 4c — Add Throughput signal

Add `request_rate` history to `workloadState`:
```go
prevRequestRate float64
```

In `Update()`, compute throughput degradation:
```go
reqRate := getMetric(metrics, "request_rate")
throughputDrop := 0.0
if hs.prevRequestRate > 0.01 && reqRate < hs.prevRequestRate {
    drop := (hs.prevRequestRate - reqRate) / hs.prevRequestRate
    throughputDrop = utils.Clamp(drop, 0, 1)
}
hs.prevRequestRate = reqRate
```

Add `Throughput KPI` field to `models.KPISnapshot`:
```go
Throughput KPI `json:"throughput"`
```

State labels: `collapsing` (>0.5 drop), `declining` (>0.2), `stable`.

Include in HealthScore with weight 0.10 (reduce other weights proportionally).

### 4d — Adaptive baselines

Add to `workloadState`:
```go
observationCount int
baselineReady    bool
baselineMeans    map[string]float64  // per-signal rolling mean
baselineM2       map[string]float64  // Welford M2 for stddev
```

In `Update()`, increment `observationCount`. After each update, call:
```go
func (hs *workloadState) updateBaseline(signals map[string]float64) {
    hs.observationCount++
    if hs.baselineMeans == nil {
        hs.baselineMeans = make(map[string]float64)
        hs.baselineM2 = make(map[string]float64)
    }
    for k, v := range signals {
        n := float64(hs.observationCount)
        delta := v - hs.baselineMeans[k]
        hs.baselineMeans[k] += delta / n
        hs.baselineM2[k] += delta * (v - hs.baselineMeans[k])
    }
    if hs.observationCount >= 96 { // 24h at 15s intervals
        hs.baselineReady = true
    }
}
```

Expose `BaselineReady() bool` and `BaselineSigma(signal string) float64` on Analyzer.
Do NOT yet change the threshold comparisons — just establish the baseline data.
The thresholds will be switched to relative mode in a follow-up once baselines are validated.

---

## Task 5 — Deprecate composites engine

In `workdir/internal/composites/engine.go`, add at the top:
```go
// Deprecated: Use internal/analyzer.Analyzer instead.
// This engine is kept for test compatibility only and will be removed in v6.3.
// The superior Contagion (graph-based) and Pressure (EWMA) formulas have been
// ported into the Analyzer.
```

Do NOT delete the file. Do NOT change any logic. The tests in engine_test.go must still pass.

---

## Verification

Run: `cd /root/ruptura/workdir && go build ./... && go test -race ./...`
All existing tests must pass. Fix any compilation errors from the signature changes.
The `UpdateHost` wrapper ensures callers that use the old `host string` API still compile.
