# API

Kairo Core exposes a REST API v2 with 44 endpoints. All endpoints are under `/api/v2/`.

## Authentication

Two auth methods are supported:

=== "JWT Bearer"

    ```bash
    # Login to get a token
    curl -X POST http://localhost:8080/api/v2/auth/login \
      -H "Content-Type: application/json" \
      -d '{"username":"admin","password":"<jwt_secret>"}'

    # Use the token
    curl -H "Authorization: Bearer <token>" http://localhost:8080/api/v2/health
    ```

=== "API Key"

    ```bash
    # Create a key (requires JWT auth)
    curl -X POST http://localhost:8080/api/v2/apikeys \
      -H "Authorization: Bearer <token>" \
      -d '{"name":"ci-pipeline","scopes":["read","write"]}'

    # Use the key (ohe_* format)
    curl -H "Authorization: Bearer ohe_abc123" http://localhost:8080/api/v2/health
    ```

## Response envelope

All responses follow a consistent JSON envelope:

```json
{
  "success": true,
  "data": { ... },
  "timestamp": "2026-04-28T10:00:00Z"
}
```

Errors:

```json
{
  "success": false,
  "error": {
    "code": "UNAUTHORIZED",
    "message": "API key expired"
  },
  "timestamp": "2026-04-28T10:00:00Z"
}
```

## Quick reference

| Category | Endpoints |
|----------|----------|
| Health | `GET /health`, `GET /ready` |
| Ingest | `POST /write`, `POST /v1/metrics`, `POST /v1/logs`, `POST /v1/traces` |
| Rupture | `GET /rupture/{host}`, `GET /ruptures` |
| KPIs | `GET /kpi/{signal}/{host}` (8 signals) |
| Actions | `GET /actions`, `POST /actions/{id}/approve`, `POST /actions/emergency-stop` |
| Explainability | `GET /explain/{id}`, `GET /explain/{id}/formula` |
| Ensemble | `GET /ensemble/{host}` |
| Auth | `POST /auth/login`, `POST /auth/refresh` |
| API keys | `GET /apikeys`, `POST /apikeys`, `DELETE /apikeys/{id}` |
| Self-metrics | `GET /metrics` (Prometheus format) |

[Full API Reference →](reference.md)
