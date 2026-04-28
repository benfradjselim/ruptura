package ruptura

import (
	"context"
	"fmt"
)

// DashboardList returns all dashboards in the current org.
func (c *Client) DashboardList(ctx context.Context) ([]Dashboard, error) {
	var out []Dashboard
	if err := c.get(ctx, "/api/v1/dashboards", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DashboardGet returns a single dashboard by ID.
func (c *Client) DashboardGet(ctx context.Context, id string) (*Dashboard, error) {
	var out Dashboard
	if err := c.get(ctx, fmt.Sprintf("/api/v1/dashboards/%s", id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DashboardCreate creates a new dashboard.
func (c *Client) DashboardCreate(ctx context.Context, d Dashboard) (*Dashboard, error) {
	var out Dashboard
	if err := c.post(ctx, "/api/v1/dashboards", d, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DashboardUpdate replaces a dashboard by ID.
func (c *Client) DashboardUpdate(ctx context.Context, id string, d Dashboard) (*Dashboard, error) {
	var out Dashboard
	if err := c.put(ctx, fmt.Sprintf("/api/v1/dashboards/%s", id), d, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DashboardDelete removes a dashboard by ID.
func (c *Client) DashboardDelete(ctx context.Context, id string) error {
	return c.del(ctx, fmt.Sprintf("/api/v1/dashboards/%s", id))
}
