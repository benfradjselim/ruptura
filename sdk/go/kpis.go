package ruptura

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// KPIGet returns the current KPI snapshot for host.
func (c *Client) KPIGet(ctx context.Context, host string) (*KPISnapshot, error) {
	q := url.Values{}
	if host != "" {
		q.Set("host", host)
	}
	var out KPISnapshot
	if err := c.get(ctx, "/api/v1/kpis", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// KPIPredict returns an ensemble forecast for kpiName on host.
// horizon is in minutes (e.g. 60 for a 1-hour ahead forecast).
func (c *Client) KPIPredict(ctx context.Context, kpiName, host string, horizon int) (*Prediction, error) {
	q := url.Values{}
	if host != "" {
		q.Set("host", host)
	}
	if horizon > 0 {
		q.Set("horizon", strconv.Itoa(horizon))
	}
	var out Prediction
	if err := c.get(ctx, fmt.Sprintf("/api/v1/kpis/%s/predict", kpiName), q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// KPIMulti returns KPI snapshots for multiple hosts simultaneously.
func (c *Client) KPIMulti(ctx context.Context, hosts []string) ([]KPISnapshot, error) {
	q := url.Values{}
	for _, h := range hosts {
		q.Add("host", h)
	}
	var out []KPISnapshot
	if err := c.get(ctx, "/api/v1/kpis/multi", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Explain returns the XAI explanation for why a KPI is in its current state.
func (c *Client) Explain(ctx context.Context, kpi, host string) (*ExplainResult, error) {
	q := url.Values{}
	if host != "" {
		q.Set("host", host)
	}
	var out ExplainResult
	if err := c.get(ctx, fmt.Sprintf("/api/v1/explain/%s", kpi), q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Query executes a QQL (OHE Query Language) query.
func (c *Client) Query(ctx context.Context, req QueryRequest) ([]QueryResult, error) {
	var out []QueryResult
	if err := c.post(ctx, "/api/v1/query", req, &out); err != nil {
		return nil, err
	}
	return out, nil
}
