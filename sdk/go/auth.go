package ohe

import "context"

// Login authenticates with username/password and stores the returned JWT token
// on the client so subsequent calls are automatically authenticated.
func (c *Client) Login(ctx context.Context, username, password string) (*LoginResponse, error) {
	var out LoginResponse
	if err := c.post(ctx, "/api/v1/auth/login", LoginRequest{Username: username, Password: password}, &out); err != nil {
		return nil, err
	}
	c.SetToken(out.Token)
	return &out, nil
}

// Logout invalidates the current JWT token server-side.
func (c *Client) Logout(ctx context.Context) error {
	return c.post(ctx, "/api/v1/auth/logout", nil, nil)
}

// Refresh exchanges the current token for a fresh one.
func (c *Client) Refresh(ctx context.Context) (*LoginResponse, error) {
	var out LoginResponse
	if err := c.post(ctx, "/api/v1/auth/refresh", nil, &out); err != nil {
		return nil, err
	}
	c.SetToken(out.Token)
	return &out, nil
}
