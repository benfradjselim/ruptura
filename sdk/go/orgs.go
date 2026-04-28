package ruptura

import (
	"context"
	"fmt"
)

// OrgList returns all orgs (admin only).
func (c *Client) OrgList(ctx context.Context) ([]Org, error) {
	var out []Org
	if err := c.get(ctx, "/api/v1/orgs", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// OrgGet returns a single org by ID.
func (c *Client) OrgGet(ctx context.Context, id string) (*Org, error) {
	var out Org
	if err := c.get(ctx, fmt.Sprintf("/api/v1/orgs/%s", id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// OrgCreate creates a new org (operator+).
func (c *Client) OrgCreate(ctx context.Context, o Org) (*Org, error) {
	var out Org
	if err := c.post(ctx, "/api/v1/orgs", o, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// OrgUpdate replaces an org by ID (operator+).
func (c *Client) OrgUpdate(ctx context.Context, id string, o Org) (*Org, error) {
	var out Org
	if err := c.put(ctx, fmt.Sprintf("/api/v1/orgs/%s", id), o, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// OrgDelete removes an org by ID (operator+).
func (c *Client) OrgDelete(ctx context.Context, id string) error {
	return c.del(ctx, fmt.Sprintf("/api/v1/orgs/%s", id))
}

// OrgMemberList lists members of an org.
func (c *Client) OrgMemberList(ctx context.Context, orgID string) ([]OrgMember, error) {
	var out []OrgMember
	if err := c.get(ctx, fmt.Sprintf("/api/v1/orgs/%s/members", orgID), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// OrgInvite adds a user to an org with the given role (operator+).
func (c *Client) OrgInvite(ctx context.Context, orgID, username, role string) error {
	body := map[string]string{"username": username, "role": role}
	return c.post(ctx, fmt.Sprintf("/api/v1/orgs/%s/members", orgID), body, nil)
}
