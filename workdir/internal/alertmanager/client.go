// Package alertmanager sends failure results to a Prometheus Alertmanager.
package alertmanager

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	requestTimeout = 10 * time.Second
	// alertTTL is longer than the send interval, so an active alert never expires
	// and a cleared failure resolves within this time.
	alertTTL = 3 * time.Minute
)

// Alert is one failure result to send.
type Alert struct {
	Name     string
	Severity string
	Level    string
	Target   string
	Kind     string
	Summary  string
	// Impact names the callers a failure can hit, empty when none is known.
	Impact string
}

type postableAlert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	EndsAt      string            `json:"endsAt"`
}

// Client posts alerts to one Alertmanager.
type Client struct {
	url       string
	http      *http.Client
	tokenFile string
	now       func() time.Time
}

// Option configures a Client.
type Option func(*Client)

// WithTokenFile sends the token read from path as a bearer token. The file is
// read at each send, so a rotated service account token is picked up.
func WithTokenFile(path string) Option {
	return func(c *Client) { c.tokenFile = path }
}

// NewClient builds a client for an Alertmanager base URL.
func NewClient(url string, client *http.Client, opts ...Option) *Client {
	if client == nil {
		client = &http.Client{Timeout: requestTimeout}
	}
	c := &Client{url: strings.TrimRight(url, "/"), http: client, now: time.Now}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// HTTPClientWithCA returns an HTTP client that trusts the system roots and the
// certificates in caFile, such as the OpenShift service CA.
func HTTPClientWithCA(caFile string) (*http.Client, error) {
	pem, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("alertmanager: read CA: %w", err)
	}
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("alertmanager: no certificate in %s", caFile)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	return &http.Client{Timeout: requestTimeout, Transport: transport}, nil
}

// alertLabels identify an alert. The state stays out of them, so a change of
// state updates the alert instead of opening a second one. The namespace lets
// inhibition rules and per project views match only alerts of that namespace.
func alertLabels(alert Alert) map[string]string {
	labels := map[string]string{
		"alertname": alertName(alert.Name),
		"severity":  severityLabel(alert.Severity),
		"level":     alert.Level,
		"target":    alert.Target,
		"kind":      alert.Kind,
	}
	if parts := strings.Split(alert.Target, "/"); len(parts) == 3 && parts[0] != "" {
		labels["namespace"] = parts[0]
	}
	return labels
}

// severityLabel maps a failure state to the severities Alertmanager routes use.
func severityLabel(state string) string {
	if state == "critical" {
		return "critical"
	}
	return "warning"
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
	endsAt := c.now().Add(alertTTL).UTC().Format(time.RFC3339)
	payload := make([]postableAlert, 0, len(alerts))
	for _, alert := range alerts {
		payload = append(payload, postableAlert{
			Labels:      alertLabels(alert),
			Annotations: annotations(alert),
			EndsAt:      endsAt,
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
	if c.tokenFile != "" {
		token, err := os.ReadFile(c.tokenFile)
		if err != nil {
			return fmt.Errorf("alertmanager: read token: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(token)))
	}
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

func annotations(alert Alert) map[string]string {
	notes := map[string]string{"summary": alert.Summary, "state": alert.Severity}
	if alert.Impact != "" {
		notes["impact"] = alert.Impact
	}
	return notes
}
