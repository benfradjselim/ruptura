// Package vault provides a thin client for reading secrets from HashiCorp Vault
// (KV v2) using only stdlib net/http — no Vault SDK dependency.
//
// Usage in OHE:
//
//	v := vault.New(vault.Config{Addr: "http://vault:8200", Token: "s.xxx"})
//	jwt, err := v.Secret("ohe/data/jwt_secret", "value")
//
// Secrets are cached in-process with a configurable TTL and refreshed
// transparently on next access after expiry.
package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const defaultCacheTTL = 5 * time.Minute

// Config holds Vault connection settings. All fields have env-var overrides
// documented in the OHE config guide.
type Config struct {
	// Addr is the Vault server URL, e.g. "https://vault.example.com:8200".
	Addr string
	// Token is the Vault client token (VAULT_TOKEN).
	Token string
	// CacheTTL controls how long secrets are cached. Default: 5m.
	CacheTTL time.Duration
	// HTTPClient is optional; defaults to a 10-second timeout client.
	HTTPClient *http.Client
}

// Client is a Vault KV-v2 secret reader.
type Client struct {
	cfg      Config
	mu       sync.Mutex
	cache    map[string]cachedSecret
}

type cachedSecret struct {
	value   string
	expires time.Time
}

// New creates a Vault client. Returns an error if Addr or Token is empty.
func New(cfg Config) (*Client, error) {
	if cfg.Addr == "" {
		return nil, fmt.Errorf("vault: Addr is required")
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("vault: Token is required")
	}
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = defaultCacheTTL
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{cfg: cfg, cache: make(map[string]cachedSecret)}, nil
}

// Secret fetches the value of key within a KV v2 secret at path.
// path is the mount-relative path WITHOUT the "data/" prefix, e.g. "ohe/jwt_secret".
// key is the field inside the secret's data map.
//
// Results are cached for cfg.CacheTTL.
func (c *Client) Secret(ctx context.Context, path, key string) (string, error) {
	cacheKey := path + ":" + key

	c.mu.Lock()
	if cs, ok := c.cache[cacheKey]; ok && time.Now().Before(cs.expires) {
		c.mu.Unlock()
		return cs.value, nil
	}
	c.mu.Unlock()

	value, err := c.fetch(ctx, path, key)
	if err != nil {
		return "", err
	}

	c.mu.Lock()
	c.cache[cacheKey] = cachedSecret{value: value, expires: time.Now().Add(c.cfg.CacheTTL)}
	c.mu.Unlock()

	return value, nil
}

// Invalidate removes a cached secret, forcing the next call to re-fetch.
func (c *Client) Invalidate(path, key string) {
	c.mu.Lock()
	delete(c.cache, path+":"+key)
	c.mu.Unlock()
}

// fetch performs the actual Vault API call.
func (c *Client) fetch(ctx context.Context, path, key string) (string, error) {
	// KV v2 URL: /v1/<mount>/data/<path>
	// We assume the mount is the first path component.
	url := c.cfg.Addr + "/v1/" + path + "?version=0"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("vault: request: %w", err)
	}
	req.Header.Set("X-Vault-Token", c.cfg.Token)

	resp, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("vault: http: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("vault: secret %q not found", path)
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("vault: permission denied for %q (check token policy)", path)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("vault: unexpected status %d: %s", resp.StatusCode, body)
	}

	// Vault KV v2 response: { data: { data: { key: value, ... }, metadata: {...} } }
	var envelope struct {
		Data struct {
			Data map[string]string `json:"data"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return "", fmt.Errorf("vault: decode: %w", err)
	}

	value, ok := envelope.Data.Data[key]
	if !ok {
		return "", fmt.Errorf("vault: key %q not found in secret %q", key, path)
	}
	return value, nil
}

// MustSecret is like Secret but panics on error — use only during startup.
func (c *Client) MustSecret(ctx context.Context, path, key string) string {
	v, err := c.Secret(ctx, path, key)
	if err != nil {
		panic("vault: " + err.Error())
	}
	return v
}

// Loader returns a function that resolves an OHE config field from Vault.
// If the field already has a non-empty value, it is returned as-is (env/file
// override takes precedence over Vault).
func (c *Client) Loader(ctx context.Context, path, key string) func(existing string) string {
	return func(existing string) string {
		if existing != "" {
			return existing
		}
		v, err := c.Secret(ctx, path, key)
		if err != nil {
			return ""
		}
		return v
	}
}
