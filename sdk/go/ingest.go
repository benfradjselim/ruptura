package ohe

import "context"

// Ingest pushes a batch of metrics from an agent to OHE (operator+).
func (c *Client) Ingest(ctx context.Context, req IngestRequest) error {
	return c.post(ctx, "/api/v1/ingest", req, nil)
}
