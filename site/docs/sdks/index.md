# SDKs

Official Kairo Core client libraries.

| SDK | Language | Package | Status |
|-----|----------|---------|--------|
| [Go SDK](go-sdk.md) | Go 1.18+ | `github.com/benfradjselim/kairo-core/sdk/go` | Stable (v6.1) |
| [Python SDK](python-sdk.md) | Python 3.9+ | `kairo-client` (PyPI) | Stable (v6.0) |

Both SDKs wrap the REST API v2 and handle auth, error decoding, and JSON unmarshalling automatically.

## Quick comparison

=== "Go"

    ```go
    import ohe "github.com/benfradjselim/kairo-core/sdk/go"

    c := ohe.New("http://kairo-core:8080", ohe.WithAPIKey("ohe_abc123"))
    health, err := c.Health(ctx)
    rupture, err := c.RuptureIndex(ctx, "web-01")
    weights, err := c.EnsembleWeights(ctx, "web-01")  // v6.1
    ```

=== "Python"

    ```python
    from kairo import KairoClient

    c = KairoClient("http://kairo-core:8080", api_key="ohe_abc123")
    health = c.health()
    rupture = c.rupture_index("web-01")
    ```
