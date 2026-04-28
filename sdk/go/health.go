package ruptura

import "context"

// Health returns the full health status including version and component checks.
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	var out HealthResponse
	if err := c.get(ctx, "/api/v1/health", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Liveness returns true when the process is alive (for k8s livenessProbe).
func (c *Client) Liveness(ctx context.Context) error {
	return c.get(ctx, "/api/v1/health/live", nil, nil)
}

// Readiness returns nil when OHE is ready to serve traffic (for k8s readinessProbe).
func (c *Client) Readiness(ctx context.Context) error {
	return c.get(ctx, "/api/v1/health/ready", nil, nil)
}

// Fleet returns the aggregated health summary for all known hosts.
func (c *Client) Fleet(ctx context.Context) (*FleetStatus, error) {
	var out FleetStatus
	if err := c.get(ctx, "/api/v1/fleet", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
