# Go SDK

The Go SDK is part of the `kairo-core` module at `sdk/go`.

## Install

```bash
go get github.com/benfradjselim/kairo-core/sdk/go@v6.1.1
```

## Import

The package name is `ohe`:

```go
import ohe "github.com/benfradjselim/kairo-core/sdk/go"
```

## Create a client

```go
// API key auth (recommended for services)
c := ohe.New("http://kairo-core:8080", ohe.WithAPIKey("ohe_abc123"))

// JWT auth (for interactive / user sessions)
c := ohe.New("http://kairo-core:8080", ohe.WithToken("eyJ..."))

// Custom timeout
c := ohe.New("http://kairo-core:8080",
    ohe.WithAPIKey("ohe_abc123"),
    ohe.WithTimeout(10*time.Second),
)

// Multi-tenant
c := ohe.New("http://kairo-core:8080",
    ohe.WithAPIKey("ohe_abc123"),
    ohe.WithOrgID("org_xyz"),
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

## Rupture Index

```go
rupture, err := c.RuptureIndex(ctx, "web-01")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("R=%.2f  state=%s\n", rupture.RuptureIndex, rupture.State)
```

## Composite signals

```go
// Single signal
kpi, err := c.KPI(ctx, "stress", "web-01")
fmt.Printf("stress=%.2f  state=%s\n", kpi.Value, kpi.State)

// Health score (0–100)
hs, err := c.KPI(ctx, "healthscore", "web-01")
fmt.Printf("healthscore=%.1f\n", hs.Value)
```

## Adaptive ensemble weights (v6.1)

```go
weights, err := c.EnsembleWeights(ctx, "web-01")
for model, w := range weights.Weights {
    fmt.Printf("  %s: %.2f\n", model, w)
}
```

## Ingest metrics

```go
err := c.IngestMetrics(ctx, []ohe.Metric{
    {Name: "cpu_usage", Value: 0.72, Host: "web-01", Timestamp: time.Now()},
    {Name: "mem_usage", Value: 0.45, Host: "web-01", Timestamp: time.Now()},
})
```

## Actions

```go
// List pending actions
actions, err := c.ListActions(ctx)

// Approve a suggested action
err = c.ApproveAction(ctx, "act_abc")

// Emergency stop
err = c.EmergencyStop(ctx)
```

## Error handling

The SDK returns `*ohe.Error` for non-2xx responses:

```go
rupture, err := c.RuptureIndex(ctx, "unknown-host")
if err != nil {
    var apiErr *ohe.Error
    if errors.As(err, &apiErr) {
        fmt.Printf("HTTP %d: %s\n", apiErr.StatusCode, apiErr.Message)
    }
}
```

## Options reference

| Option | Description |
|--------|-------------|
| `WithAPIKey(key string)` | Set API key (`ohe_*` format) |
| `WithToken(token string)` | Set JWT bearer token |
| `WithOrgID(id string)` | Set `X-Org-ID` header for multi-tenancy |
| `WithTimeout(d time.Duration)` | HTTP request timeout (default 30s) |
| `WithHTTPClient(hc *http.Client)` | Replace the default HTTP client |
