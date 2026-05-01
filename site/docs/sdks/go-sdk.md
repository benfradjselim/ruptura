# Go SDK

The Go SDK is part of the `ruptura` module at `sdk/go`.

## Install

```bash
go get github.com/benfradjselim/ruptura/sdk/go@v6.2.2
```

## Create a client

```go
import ruptura "github.com/benfradjselim/ruptura/sdk/go"

// API key auth (recommended)
c := ruptura.New("http://ruptura:8080", ruptura.WithAPIKey("your-api-key"))

// Custom timeout
c := ruptura.New("http://ruptura:8080",
    ruptura.WithAPIKey("your-api-key"),
    ruptura.WithTimeout(10*time.Second),
)
```

## Health check

```go
ctx := context.Background()

health, err := c.Health(ctx)
if err != nil {
    log.Fatal(err)
}
fmt.Println(health.Status)  // "ok"
```

## Rupture Index (WorkloadRef — primary)

```go
// Kubernetes workload: namespace + name
rupture, err := c.RuptureIndex(ctx, "default", "payment-api")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("FusedR=%.2f  state=%s  health=%d\n",
    rupture.FusedRuptureIndex, rupture.State, rupture.HealthScore)
```

## All ruptures

```go
ruptures, err := c.Ruptures(ctx)
for _, r := range ruptures {
    fmt.Printf("%s/%s: FusedR=%.2f  state=%s\n",
        r.Workload.Namespace, r.Workload.Name,
        r.FusedRuptureIndex, r.State)
}
```

## KPI signals

```go
// Any of: stress, fatigue, mood, pressure, humidity, contagion,
//         resilience, entropy, velocity, health_score
kpi, err := c.KPI(ctx, "fatigue", "default", "payment-api")
fmt.Printf("fatigue=%.2f  state=%s\n", kpi.Value, kpi.State)

// Health score (0–100)
hs, err := c.KPI(ctx, "health_score", "default", "payment-api")
fmt.Printf("health_score=%.1f\n", hs.Value)
```

## Narrative explain

```go
narrative, err := c.ExplainNarrative(ctx, "r_abc123")
if err != nil {
    log.Fatal(err)
}
fmt.Println(narrative.Narrative)
// "payment-api has been accumulating fatigue for 72h..."
fmt.Printf("severity=%s  top_factor=%s  ttf=%ds\n",
    narrative.Severity, narrative.TopFactor, narrative.TTFSeconds)
```

## Anomalies

```go
// All anomalies
anomalies, err := c.Anomalies(ctx, "")

// For a specific workload
anomalies, err := c.Anomalies(ctx, "payment-api")
for _, a := range anomalies {
    fmt.Printf("host=%s  method=%s  severity=%s  consensus=%v\n",
        a.Host, a.Method, a.Severity, a.Consensus)
}
```

## Actions

```go
// List pending actions
actions, err := c.ListActions(ctx)

// Approve a suggested action
err = c.ApproveAction(ctx, "act_abc")

// Reject
err = c.RejectAction(ctx, "act_abc")

// Emergency stop all Tier-1 auto-actions
err = c.EmergencyStop(ctx)
```

## Maintenance windows

```go
err := c.CreateSuppression(ctx, ruptura.Suppression{
    Workload: "default/Deployment/order-processor",
    Start:    time.Now(),
    End:      time.Now().Add(30 * time.Minute),
    Reason:   "rolling deploy v2.4.1",
})
```

## Error handling

```go
rupture, err := c.RuptureIndex(ctx, "default", "unknown-svc")
if err != nil {
    var apiErr *ruptura.Error
    if errors.As(err, &apiErr) {
        fmt.Printf("HTTP %d: %s\n", apiErr.StatusCode, apiErr.Message)
    }
}
```

## Options reference

| Option | Description |
|--------|-------------|
| `WithAPIKey(key string)` | Set API key |
| `WithTimeout(d time.Duration)` | HTTP request timeout (default 30s) |
| `WithHTTPClient(hc *http.Client)` | Replace the default HTTP client |
