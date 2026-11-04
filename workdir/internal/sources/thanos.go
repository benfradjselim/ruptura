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
	// RestartsOnly marks a pod with no usage reading, such as one in
	// CrashLoopBackOff. Only Restarts is meaningful.
	RestartsOnly bool
}

// Thanos queries a Prometheus-compatible HTTP API.
type Thanos struct {
	baseURL   string
	authorize func(*http.Request) error
	client    *http.Client
}

// NewThanos builds a Thanos source from a base URL, bearer token and client.
// The client timeout is capped at 15 seconds.
func NewThanos(baseURL, token string, client *http.Client) *Thanos {
	return newThanos(baseURL, staticBearer(token), client)
}

func newThanos(baseURL string, authorize func(*http.Request) error, client *http.Client) *Thanos {
	if client == nil {
		client = &http.Client{}
	}
	c := *client
	if c.Timeout == 0 || c.Timeout > requestTimeout {
		c.Timeout = requestTimeout
	}
	return &Thanos{baseURL: strings.TrimRight(baseURL, "/"), authorize: authorize, client: &c}
}

func staticBearer(token string) func(*http.Request) error {
	return func(req *http.Request) error {
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		return nil
	}
}

// Authentication modes for a Prometheus endpoint.
const (
	AuthServiceAccount = "serviceAccount"
	AuthBearer         = "bearer"
	AuthBasic          = "basic"
	AuthNone           = "none"
)

// PrometheusConfig describes how to reach a Prometheus compatible endpoint.
// Credential files are read at each request, so rotated values are used.
type PrometheusConfig struct {
	URL          string
	CAFile       string
	CertFile     string
	KeyFile      string
	ServerName   string
	Auth         string
	TokenFile    string
	UsernameFile string
	PasswordFile string
	// ServiceAccountDir holds the pod token and cluster CAs; empty means the
	// standard in-cluster path.
	ServiceAccountDir string
}

// NewInClusterThanos builds a source using the pod service account token and
// trusting ca.crt plus service-ca.crt (when present).
func NewInClusterThanos(baseURL string) (*Thanos, error) {
	return NewPrometheus(PrometheusConfig{URL: baseURL, Auth: AuthServiceAccount})
}

func newInClusterThanos(baseURL, dir string) (*Thanos, error) {
	return NewPrometheus(PrometheusConfig{URL: baseURL, Auth: AuthServiceAccount, ServiceAccountDir: dir})
}

// NewPrometheus builds a source from an explicit configuration. Certificates
// are trusted from the system, the cluster CAs when present, and CAFile.
func NewPrometheus(cfg PrometheusConfig) (*Thanos, error) {
	dir := cfg.ServiceAccountDir
	if dir == "" {
		dir = saDir
	}
	tlsConfig, err := clientTLS(cfg, dir)
	if err != nil {
		return nil, err
	}
	authorize, err := authorizer(cfg, dir)
	if err != nil {
		return nil, err
	}
	client := &http.Client{
		Timeout:   requestTimeout,
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
	}
	return newThanos(cfg.URL, authorize, client), nil
}

func clientTLS(cfg PrometheusConfig, dir string) (*tls.Config, error) {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	for _, f := range []struct {
		path     string
		optional bool
	}{
		{filepath.Join(dir, "ca.crt"), cfg.Auth != AuthServiceAccount},
		{filepath.Join(dir, "service-ca.crt"), true},
		{cfg.CAFile, false},
	} {
		if f.path == "" {
			continue
		}
		pem, err := os.ReadFile(f.path)
		if err != nil {
			if f.optional && errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("sources: read %s: %w", filepath.Base(f.path), err)
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("sources: no valid certificates in %s", filepath.Base(f.path))
		}
	}
	config := &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12, ServerName: cfg.ServerName}
	if cfg.CertFile != "" || cfg.KeyFile != "" {
		if cfg.CertFile == "" || cfg.KeyFile == "" {
			return nil, fmt.Errorf("sources: a client certificate needs both a certificate and a key")
		}
		pair, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("sources: load client certificate: %w", err)
		}
		config.Certificates = []tls.Certificate{pair}
	}
	return config, nil
}

func readSecretFile(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("sources: read %s: %w", filepath.Base(path), err)
	}
	return strings.TrimSpace(string(raw)), nil
}

func authorizer(cfg PrometheusConfig, dir string) (func(*http.Request) error, error) {
	bearerFile := func(path string) func(*http.Request) error {
		return func(req *http.Request) error {
			token, err := readSecretFile(path)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+token)
			return nil
		}
	}
	switch cfg.Auth {
	case "", AuthServiceAccount:
		path := filepath.Join(dir, "token")
		if _, err := readSecretFile(path); err != nil {
			return nil, err
		}
		return bearerFile(path), nil
	case AuthBearer:
		if cfg.TokenFile == "" {
			return nil, fmt.Errorf("sources: bearer authentication needs a token file")
		}
		return bearerFile(cfg.TokenFile), nil
	case AuthBasic:
		if cfg.UsernameFile == "" || cfg.PasswordFile == "" {
			return nil, fmt.Errorf("sources: basic authentication needs a username and a password file")
		}
		return func(req *http.Request) error {
			user, err := readSecretFile(cfg.UsernameFile)
			if err != nil {
				return err
			}
			password, err := readSecretFile(cfg.PasswordFile)
			if err != nil {
				return err
			}
			req.SetBasicAuth(user, password)
			return nil
		}, nil
	case AuthNone:
		return func(*http.Request) error { return nil }, nil
	}
	return nil, fmt.Errorf("sources: unknown authentication %q", cfg.Auth)
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
	return t.queryBy(ctx, q, "pod")
}

// CollectRole returns the value of a role per namespace/target.
func (t *Thanos) CollectRole(ctx context.Context, role Role) (map[string]float64, error) {
	samples, _, err := t.queryBy(ctx, role.Query, role.Target)
	if err != nil {
		return nil, err
	}
	out := make(map[string]float64, len(samples))
	for key, s := range samples {
		out[key] = s.v
	}
	return out, nil
}

func (t *Thanos) queryBy(ctx context.Context, q, targetLabel string) (map[string]sample, map[string][2]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		t.baseURL+"/api/v1/query?query="+url.QueryEscape(q), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("sources: build request: %w", err)
	}
	if err := t.authorize(req); err != nil {
		return nil, nil, err
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
		ns, pod := r.Metric["namespace"], r.Metric[targetLabel]
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
	var restarts map[string]sample
	var restartIDs map[string][2]string
	if sig.Restarts != "" {
		if restarts, restartIDs, err = t.query(ctx, sig.Restarts); err != nil {
			return nil, err
		}
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
	for key, r := range restarts {
		if _, ok := usage[key]; ok {
			continue
		}
		out = append(out, Reading{
			Namespace:    restartIDs[key][0],
			Pod:          restartIDs[key][1],
			Restarts:     int(r.v),
			At:           r.at,
			RestartsOnly: true,
		})
	}
	return out, nil
}
