package ruptura

import (
	"context"
	"fmt"
)

// AlertList returns all alerts. Pass status="" for all statuses.
func (c *Client) AlertList(ctx context.Context, status string) ([]Alert, error) {
	p := "/api/v1/alerts"
	if status != "" {
		p += "?status=" + status
	}
	var out []Alert
	if err := c.get(ctx, p, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AlertGet returns a single alert by ID.
func (c *Client) AlertGet(ctx context.Context, id string) (*Alert, error) {
	var out Alert
	if err := c.get(ctx, fmt.Sprintf("/api/v1/alerts/%s", id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AlertAcknowledge marks an alert as acknowledged.
func (c *Client) AlertAcknowledge(ctx context.Context, id string) error {
	return c.post(ctx, fmt.Sprintf("/api/v1/alerts/%s/acknowledge", id), nil, nil)
}

// AlertSilence silences an alert (suppresses future notifications).
func (c *Client) AlertSilence(ctx context.Context, id string) error {
	return c.post(ctx, fmt.Sprintf("/api/v1/alerts/%s/silence", id), nil, nil)
}

// AlertDelete removes a resolved or stale alert.
func (c *Client) AlertDelete(ctx context.Context, id string) error {
	return c.del(ctx, fmt.Sprintf("/api/v1/alerts/%s", id))
}

// AlertRuleList returns all alert rules.
func (c *Client) AlertRuleList(ctx context.Context) ([]AlertRule, error) {
	var out []AlertRule
	if err := c.get(ctx, "/api/v1/alert-rules", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AlertRuleCreate creates a new alert rule.
func (c *Client) AlertRuleCreate(ctx context.Context, rule AlertRule) (*AlertRule, error) {
	var out AlertRule
	if err := c.post(ctx, "/api/v1/alert-rules", rule, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AlertRuleUpdate replaces an alert rule by name.
func (c *Client) AlertRuleUpdate(ctx context.Context, name string, rule AlertRule) (*AlertRule, error) {
	var out AlertRule
	if err := c.put(ctx, fmt.Sprintf("/api/v1/alert-rules/%s", name), rule, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AlertRuleDelete deletes an alert rule by name.
func (c *Client) AlertRuleDelete(ctx context.Context, name string) error {
	return c.del(ctx, fmt.Sprintf("/api/v1/alert-rules/%s", name))
}
