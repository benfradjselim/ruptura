package sources

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	requestTimeout = 15 * time.Second
	saDir          = "/var/run/secrets/kubernetes.io/serviceaccount"
	maxBody        = 32 << 20
)

// Reading is one pod observation joined across the signal queries.
type Reading struct {
	Namespace string
	Pod       string
	Usage     float64
	Capacity  float64
	Restarts  int
	At        time.Time
}

// Thanos queries a Prometheus-compatible HTTP API.
type Thanos struct {
	baseURL string
	token   string
	client  *http.Client
}

// NewThanos builds a Thanos source from a base URL, bearer token and client.
// The client timeout is capped at 15 seconds.
func NewThanos(baseURL, token string, client *http.Client) *Thanos {
	if client == nil {
		client = &http.Client{}
	}
	c := *client
	if c.Timeout == 0 || c.Timeout > requestTimeout {
		c.Timeout = requestTimeout
	}
	return &Thanos{baseURL: strings.TrimRight(baseURL, "/"), token: token, client: &c}
}

// NewInClusterThanos builds a source using the pod service account token and
// trusting ca.crt plus service-ca.crt (when present).
func NewInClusterThanos(baseURL string) (*Thanos, error) {
	return newInClusterThanos(baseURL, saDir)
}

func newInClusterThanos(baseURL, dir string) (*Thanos, error) {
	tok, err := os.ReadFile(filepath.Join(dir, "token"))
	if err != nil {
		return nil, fmt.Errorf("sources: read token: %w", err)
	}
	pool := x509.NewCertPool()
	for _, f := range []struct {
		name     string
		optional bool
	}{{"ca.crt", false}, {"service-ca.crt", true}} {
		pem, err := os.ReadFile(filepath.Join(dir, f.name))
		if err != nil {
			if f.optional && errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("sources: read %s: %w", f.name, err)
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("sources: no valid certificates in %s", f.name)
		}
	}
	client := &http.Client{
		Timeout:   requestTimeout,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}},
	}
	return NewThanos(baseURL, strings.TrimSpace(string(tok)), client), nil
}

type promResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Data   struct {
		Result []struct {
			Metric map[string]string `json:"metric"`
			Value  [2]interface{}    `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

type sample struct {
	v  float64
	at time.Time
}

func (t *Thanos) query(ctx context.Context, q string) (map[string]sample, map[string][2]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		t.baseURL+"/api/v1/query?query="+url.QueryEscape(q), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("sources: build request: %w", err)
	}
	if t.token != "" {
		req.Header.Set("Authorization", "Bearer "+t.token)
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("sources: query failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("sources: unexpected HTTP status %d", resp.StatusCode)
	}
	var pr promResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&pr); err != nil {
		return nil, nil, fmt.Errorf("sources: decode response: %w", err)
	}
	if pr.Status != "success" {
		return nil, nil, fmt.Errorf("sources: prometheus error: %s", pr.Error)
	}
	out := make(map[string]sample, len(pr.Data.Result))
	ids := make(map[string][2]string, len(pr.Data.Result))
	for _, r := range pr.Data.Result {
		ts, ok1 := r.Value[0].(float64)
		s, ok2 := r.Value[1].(string)
		if !ok1 || !ok2 {
			return nil, nil, errors.New("sources: malformed sample")
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, nil, fmt.Errorf("sources: parse value: %w", err)
		}
		ns, pod := r.Metric["namespace"], r.Metric["pod"]
		key := ns + "/" + pod
		out[key] = sample{v: v, at: time.Unix(int64(ts), 0)}
		ids[key] = [2]string{ns, pod}
	}
	return out, ids, nil
}

// optionalQuery runs q, or returns no samples when q is empty (condition
// signals have only a usage query).
func (t *Thanos) optionalQuery(ctx context.Context, q string) (map[string]sample, error) {
	if q == "" {
		return nil, nil
	}
	out, _, err := t.query(ctx, q)
	return out, err
}

// Collect runs the usage, capacity and restarts queries and joins them on
// namespace and pod. Pods without usage are dropped; missing capacity or
// restarts yield zero.
func (t *Thanos) Collect(ctx context.Context, sig Signal) ([]Reading, error) {
	usage, ids, err := t.query(ctx, sig.Usage)
	if err != nil {
		return nil, err
	}
	capacity, err := t.optionalQuery(ctx, sig.Capacity)
	if err != nil {
		return nil, err
	}
	restarts, err := t.optionalQuery(ctx, sig.Restarts)
	if err != nil {
		return nil, err
	}
	out := make([]Reading, 0, len(usage))
	for key, u := range usage {
		out = append(out, Reading{
			Namespace: ids[key][0],
			Pod:       ids[key][1],
			Usage:     u.v,
			Capacity:  capacity[key].v,
			Restarts:  int(restarts[key].v),
			At:        u.at,
		})
	}
	return out, nil
}
