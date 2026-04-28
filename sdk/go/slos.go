package ruptura

import (
	"context"
	"fmt"
)

// SLOList returns all SLOs in the current org.
func (c *Client) SLOList(ctx context.Context) ([]SLO, error) {
	var out []SLO
	if err := c.get(ctx, "/api/v1/slos", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SLOGet returns a single SLO by ID.
func (c *Client) SLOGet(ctx context.Context, id string) (*SLO, error) {
	var out SLO
	if err := c.get(ctx, fmt.Sprintf("/api/v1/slos/%s", id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SLOCreate creates a new SLO.
func (c *Client) SLOCreate(ctx context.Context, s SLO) (*SLO, error) {
	var out SLO
	if err := c.post(ctx, "/api/v1/slos", s, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SLOUpdate replaces an SLO by ID.
func (c *Client) SLOUpdate(ctx context.Context, id string, s SLO) (*SLO, error) {
	var out SLO
	if err := c.put(ctx, fmt.Sprintf("/api/v1/slos/%s", id), s, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SLODelete removes an SLO by ID.
func (c *Client) SLODelete(ctx context.Context, id string) error {
	return c.del(ctx, fmt.Sprintf("/api/v1/slos/%s", id))
}

// SLOStatus returns the live compliance state of an SLO.
func (c *Client) SLOStatus(ctx context.Context, id string) (*SLOStatus, error) {
	var out SLOStatus
	if err := c.get(ctx, fmt.Sprintf("/api/v1/slos/%s/status", id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SLOAllStatus returns the live status for every SLO in the org.
func (c *Client) SLOAllStatus(ctx context.Context) ([]SLOStatus, error) {
	var out []SLOStatus
	if err := c.get(ctx, "/api/v1/slos/status", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
