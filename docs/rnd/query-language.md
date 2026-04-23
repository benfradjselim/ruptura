# OHE Query Language (QQL)

Currently, "QQL" refers to the API endpoint (`POST /api/v1/query`) used for querying metric time-series data. It is **not** a complex domain-specific language at this stage, but rather a structured JSON request format for fetching metric ranges from the OHE storage engine.

## API Usage
The endpoint accepts a `QueryRequest` JSON body:

```json
{
  "query": "metric_name",
  "from": "2026-04-13T10:00:00Z",
  "to": "2026-04-13T11:00:00Z",
  "step_seconds": 60
}
```

- **query:** The name of the metric to fetch.
- **from/to:** ISO8601 timestamps for the time range.
- **step_seconds:** Optional; if provided, OHE will downsample the returned data points to this step.

*Future development of QQL is planned to include arithmetic operations, metric transformations, and cross-host correlation capabilities.*
