package ruptura

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// MetricsList returns all metric names known to the server.
func (c *Client) MetricsList(ctx context.Context) ([]string, error) {
	var out []string
	if err := c.get(ctx, "/api/v1/metrics", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// MetricGet returns the latest value for a metric on host.
func (c *Client) MetricGet(ctx context.Context, name, host string) (*Metric, error) {
	q := url.Values{}
	if host != "" {
		q.Set("host", host)
	}
	var out Metric
	if err := c.get(ctx, fmt.Sprintf("/api/v1/metrics/%s", name), q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// MetricRange returns time-series data points for a metric between from and to.
func (c *Client) MetricRange(ctx context.Context, name, host string, from, to time.Time) ([]DataPoint, error) {
	q := url.Values{
		"from": {from.UTC().Format(time.RFC3339)},
		"to":   {to.UTC().Format(time.RFC3339)},
	}
	if host != "" {
		q.Set("host", host)
	}
	var out []DataPoint
	if err := c.get(ctx, fmt.Sprintf("/api/v1/metrics/%s/range", name), q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// MetricAggregate returns an aggregated value (avg, min, max, p95, p99) for a metric.
func (c *Client) MetricAggregate(ctx context.Context, name, host, agg string, from, to time.Time) (float64, error) {
	q := url.Values{
		"from": {from.UTC().Format(time.RFC3339)},
		"to":   {to.UTC().Format(time.RFC3339)},
		"agg":  {agg},
	}
	if host != "" {
		q.Set("host", host)
	}
	var out float64
	if err := c.get(ctx, fmt.Sprintf("/api/v1/metrics/%s/aggregate", name), q, &out); err != nil {
		return 0, err
	}
	return out, nil
}
