package ohe

import (
	"context"
	"fmt"
)

// APIKeyList lists all API keys in the current org.
func (c *Client) APIKeyList(ctx context.Context) ([]APIKey, error) {
	var out []APIKey
	if err := c.get(ctx, "/api/v1/api-keys", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// APIKeyCreate creates a new API key. The plaintext key is only returned once;
// store it immediately.
func (c *Client) APIKeyCreate(ctx context.Context, req APIKeyCreateRequest) (*APIKeyCreateResponse, error) {
	var out APIKeyCreateResponse
	if err := c.post(ctx, "/api/v1/api-keys", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// APIKeyDelete revokes an API key by ID.
func (c *Client) APIKeyDelete(ctx context.Context, id string) error {
	return c.del(ctx, fmt.Sprintf("/api/v1/api-keys/%s", id))
}
