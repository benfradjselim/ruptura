# OHE v5.1.0 — SDK Guide

OHE ships two official client SDKs: a **Go SDK** and a **Python SDK**. Both wrap the full REST API surface, support multi-tenancy, and are designed for CI/CD integration.

---

## Go SDK

**Module:** `github.com/benfradjselim/ohe-sdk-go`
**Location in repo:** `sdk/go/`
**Go version:** 1.22+

### Install

```bash
go get github.com/benfradjselim/ohe-sdk-go
```

### Quick start

```go
package main

import (
    "context"
    "fmt"
    "log"

    ohe "github.com/benfradjselim/ohe-sdk-go"
)

func main() {
    c := ohe.New("http://ohe.example.com:8080",
        ohe.WithAPIKey("ohe_<your-key>"),
        ohe.WithOrgID("my-org"),
    )

    ctx := context.Background()

    h, err := c.Health(ctx)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("version:", h.Version)

    kpi, err := c.KPIGet(ctx, "stress", "web-01")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("stress=%.3f\n", kpi.Value)
}
```

### Authentication

```go
// JWT login (token auto-stored after Login)
c := ohe.New("http://ohe.example.com:8080")
_, err := c.Login(ctx, "admin", "secret")

// API key (for CI/CD)
c := ohe.New("http://ohe.example.com:8080", ohe.WithAPIKey("ohe_xxx"))
```

### Client options

| Option | Description |
|--------|-------------|
| `WithToken(t)` | Pre-set a JWT |
| `WithAPIKey(k)` | Use API key auth |
| `WithOrgID(id)` | Set `X-Org-ID` header on all requests |
| `WithHTTPClient(hc)` | Inject custom `*http.Client` |
| `WithTimeout(d)` | Override default 30s timeout |

### Key methods

```go
// Auth
c.Login(ctx, username, password)    // → LoginResponse
c.Logout(ctx)
c.Refresh(ctx)

// Metrics
c.MetricsList(ctx)                                        // → []string
c.MetricGet(ctx, name, host)                              // → MetricValue
c.MetricRange(ctx, name, host, start, end, step)          // → []TimeValue

// KPIs
c.KPIList(ctx)                                            // → []string
c.KPIGet(ctx, name, host)                                 // → KPIValue
c.KPIMulti(ctx, names []string, host)                     // → map[string]float64
c.KPIPredict(ctx, name, host)                             // → PredictResponse

// Alerts
c.AlertList(ctx, status, host)                            // → []Alert
c.AlertAcknowledge(ctx, id)
c.AlertSilence(ctx, id, duration, reason)

// SLOs
c.SLOCreate(ctx, req SLOCreateRequest)                    // → SLO
c.SLOList(ctx)                                            // → []SLO
c.SLOStatus(ctx, id)                                      // → SLOStatus

// Ingest
c.Ingest(ctx, host string, points []MetricPoint)

// Dashboards, Datasources, Orgs, Notifications, APIKeys
// follow the same Create / List / Get / Update / Delete pattern
```

### Error handling

```go
kpi, err := c.KPIGet(ctx, "stress", "web-01")
if err != nil {
    var oheErr *ohe.Error
    if errors.As(err, &oheErr) {
        fmt.Println("status:", oheErr.StatusCode)
        fmt.Println("code:",   oheErr.Code)
    }
    log.Fatal(err)
}
```

### Testing with httptest

```go
srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    json.NewEncoder(w).Encode(map[string]any{"status": "ok", "version": "5.1.0"})
}))
defer srv.Close()

c := ohe.New(srv.URL)
h, _ := c.Health(context.Background())
// assert h.Version == "5.1.0"
```

---

## Python SDK

**Package:** `ohe-sdk`
**Location in repo:** `sdk/python/`
**Python version:** 3.9+

### Install

```bash
pip install ohe-sdk
```

Or from source:

```bash
cd sdk/python
pip install -e .
```

### Quick start

```python
from ohe import OHEClient

c = OHEClient("http://ohe.example.com:8080", api_key="ohe_<your-key>", org_id="my-org")

h = c.health()
print("version:", h["version"])

kpi = c.kpi_get("stress", host="web-01")
print(f"stress={kpi['value']:.3f}")
```

### Authentication

```python
# JWT login
c = OHEClient("http://ohe.example.com:8080")
c.login("admin", "secret")          # token stored automatically

# API key
c = OHEClient("http://ohe.example.com:8080", api_key="ohe_xxx")
```

### Constructor

```python
OHEClient(
    base_url: str,
    token:    str | None = None,
    api_key:  str | None = None,
    org_id:   str | None = None,
    timeout:  float = 30.0,
)
```

### Key methods

```python
# Auth
c.login(username, password)         # → dict with token
c.logout()
c.refresh()

# Metrics
c.metrics_list()                    # → list[str]
c.metric_get(name, host=None)       # → dict
c.metric_range(name, host, start, end, step="1m")

# KPIs
c.kpi_list()                        # → list[str]
c.kpi_get(name, host=None)          # → dict
c.kpi_multi(names, host=None)       # → dict[str, float]
c.kpi_predict(name, host=None)      # → dict

# Alerts
c.alert_list(status=None, host=None)
c.alert_acknowledge(alert_id)
c.alert_silence(alert_id, duration, reason="")

# SLOs
c.slo_create(name, kpi, target, window, comparator, threshold)
c.slo_list()
c.slo_status(slo_id)

# Ingest
c.ingest(host, points: list[dict])  # point: {"name": str, "value": float, "ts": datetime}

# Logs
c.log_query(service=None, level=None, start=None, end=None, q=None, limit=100)
```

### Error handling

```python
from ohe import OHEClient, OHEError

try:
    kpi = c.kpi_get("stress", host="web-01")
except OHEError as e:
    print(f"HTTP {e.status_code}: [{e.code}] {e.message}")
```

### Testing with `responses`

```python
import responses
from ohe import OHEClient

@responses.activate
def test_health():
    responses.add(responses.GET, "http://ohe/api/v1/health",
                  json={"status": "ok", "version": "5.1.0"})
    c = OHEClient("http://ohe")
    h = c.health()
    assert h["version"] == "5.1.0"
```

---

## Multi-Tenancy

Both SDKs support `org_id` / `WithOrgID` — every request adds `X-Org-ID: <orgID>`.

**Go**
```go
c.SetOrgID("tenant-a")
```

**Python**
```python
c.org_id = "tenant-a"
```

---

## CI/CD Integration

### GitHub Actions

```yaml
- name: Check OHE health
  env:
    OHE_URL: ${{ secrets.OHE_URL }}
    OHE_API_KEY: ${{ secrets.OHE_API_KEY }}
  run: |
    pip install ohe-sdk
    python -c "
    from ohe import OHEClient
    c = OHEClient('$OHE_URL', api_key='$OHE_API_KEY')
    h = c.health()
    assert h['status'] == 'ok', f'OHE unhealthy: {h}'
    print('OHE OK:', h['version'])
    "
```

### Go test helper

```go
func oheClient(t *testing.T) *ohe.Client {
    t.Helper()
    url := os.Getenv("OHE_URL")
    key := os.Getenv("OHE_API_KEY")
    if url == "" {
        t.Skip("OHE_URL not set")
    }
    return ohe.New(url, ohe.WithAPIKey(key))
}
```

---

## Changelog

| Version | Notes |
|---------|-------|
| v5.1.0 | Initial SDK release — full REST API coverage, multi-tenancy, httptest compatible |
