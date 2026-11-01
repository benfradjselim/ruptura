// Package alertmanager sends failure results to a Prometheus Alertmanager.
package alertmanager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const requestTimeout = 10 * time.Second

// Alert is one failure result to send.
type Alert struct {
	Name     string
	Severity string
	Level    string
	Target   string
	Kind     string
	Summary  string
}

type postableAlert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// Client posts alerts to one Alertmanager.
type Client struct {
	url  string
	http *http.Client
}

// NewClient builds a client for an Alertmanager base URL.
func NewClient(url string, client *http.Client) *Client {
	if client == nil {
		client = &http.Client{Timeout: requestTimeout}
	}
	return &Client{url: strings.TrimRight(url, "/"), http: client}
}

func alertName(mode string) string {
	var name strings.Builder
	name.WriteString("Ruptura")
	for _, word := range strings.Split(mode, "_") {
		if word != "" {
			name.WriteString(strings.ToUpper(word[:1]) + word[1:])
		}
	}
	return name.String()
}

// Send posts alerts. Alertmanager expires an alert that is not sent again, so
// callers repeat active alerts and resolved failures clear on their own.
func (c *Client) Send(ctx context.Context, alerts []Alert) error {
	if len(alerts) == 0 {
		return nil
	}
	payload := make([]postableAlert, 0, len(alerts))
	for _, alert := range alerts {
		payload = append(payload, postableAlert{
			Labels: map[string]string{
				"alertname": alertName(alert.Name),
				"severity":  alert.Severity,
				"level":     alert.Level,
				"target":    alert.Target,
				"kind":      alert.Kind,
			},
			Annotations: map[string]string{"summary": alert.Summary},
		})
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("alertmanager: encode: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+"/api/v2/alerts", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("alertmanager: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("alertmanager: send: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("alertmanager: status %d", resp.StatusCode)
	}
	return nil
}
