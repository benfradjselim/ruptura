package ruptura

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

// LogQueryOptions controls the log query.
type LogQueryOptions struct {
	Service string
	From    time.Time
	To      time.Time
	Limit   int
}

// LogQuery fetches log entries matching the options.
func (c *Client) LogQuery(ctx context.Context, opts LogQueryOptions) ([]LogEntry, error) {
	q := url.Values{}
	if opts.Service != "" {
		q.Set("service", opts.Service)
	}
	if !opts.From.IsZero() {
		q.Set("from", opts.From.UTC().Format(time.RFC3339))
	}
	if !opts.To.IsZero() {
		q.Set("to", opts.To.UTC().Format(time.RFC3339))
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	var out []LogEntry
	if err := c.get(ctx, "/api/v1/logs", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// TraceSearch lists traces, optionally filtered by service.
func (c *Client) TraceSearch(ctx context.Context, service string, limit int) ([]Span, error) {
	q := url.Values{}
	if service != "" {
		q.Set("service", service)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var out []Span
	if err := c.get(ctx, "/api/v1/traces", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// TraceGet returns all spans for a trace ID.
func (c *Client) TraceGet(ctx context.Context, traceID string) ([]Span, error) {
	var out []Span
	if err := c.get(ctx, "/api/v1/traces/"+traceID, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
