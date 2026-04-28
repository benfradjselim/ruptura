package ohe

import (
	"net/http"
)

// Health returns true if the OHE API is healthy
func (c *Client) Health() (bool, error) {
	req, err := c.newRequest(http.MethodGet, "/api/v1/health")
	if err != nil {
		return false, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK, nil
}
