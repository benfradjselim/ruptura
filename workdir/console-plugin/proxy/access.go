package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// AccessChecker says whether the holder of a token may see a namespace.
type AccessChecker interface {
	Allowed(ctx context.Context, token, namespace string) (bool, error)
}

// ssarChecker asks the API server with a SelfSubjectAccessReview sent with the
// user's own token, which any authenticated user may do for itself. A user may
// see a namespace when it may list its pods: that is what the console shows
// on the workload pages the plugin extends.
type ssarChecker struct {
	apiServer string
	client    *http.Client
}

// errUnauthenticated is returned when the API server refuses the token.
var errUnauthenticated = fmt.Errorf("token refused by the API server")

func newAccessChecker(apiServer, caFile string, timeout time.Duration) (*ssarChecker, error) {
	ca, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read CA %s: %w", caFile, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("no certificate in %s", caFile)
	}
	return &ssarChecker{
		apiServer: apiServer,
		client: &http.Client{
			Timeout:   timeout,
			Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}},
		},
	}, nil
}

type accessReview struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Spec       struct {
		ResourceAttributes struct {
			Namespace string `json:"namespace"`
			Verb      string `json:"verb"`
			Resource  string `json:"resource"`
		} `json:"resourceAttributes"`
	} `json:"spec"`
	Status struct {
		Allowed bool `json:"allowed"`
	} `json:"status,omitempty"`
}

// Allowed posts the review and reads status.allowed. A 401 is reported as
// errUnauthenticated so the caller can answer the browser accordingly.
func (s *ssarChecker) Allowed(ctx context.Context, token, namespace string) (bool, error) {
	var review accessReview
	review.APIVersion = "authorization.k8s.io/v1"
	review.Kind = "SelfSubjectAccessReview"
	review.Spec.ResourceAttributes.Namespace = namespace
	review.Spec.ResourceAttributes.Verb = "list"
	review.Spec.ResourceAttributes.Resource = "pods"
	body, err := json.Marshal(review)
	if err != nil {
		return false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apiServer+"/apis/authorization.k8s.io/v1/selfsubjectaccessreviews", bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return false, errUnauthenticated
	}
	if resp.StatusCode >= 300 {
		return false, fmt.Errorf("access review for %s: HTTP %d", namespace, resp.StatusCode)
	}
	var out accessReview
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&out); err != nil {
		return false, fmt.Errorf("access review for %s: %w", namespace, err)
	}
	return out.Status.Allowed, nil
}

// cachedChecker remembers decisions for a while, keyed by a hash of the token
// and the namespace, so a page refresh does not ask the API server again for
// every namespace. Tokens are never stored.
type cachedChecker struct {
	next AccessChecker
	ttl  time.Duration
	now  func() time.Time
	mu   sync.Mutex
	seen map[string]decision
}

type decision struct {
	allowed bool
	until   time.Time
}

func newCachedChecker(next AccessChecker, ttl time.Duration) *cachedChecker {
	return &cachedChecker{next: next, ttl: ttl, now: time.Now, seen: map[string]decision{}}
}

func cacheKey(token, namespace string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:]) + "/" + namespace
}

// Allowed answers from the cache when the decision is still fresh. Errors are
// never cached.
func (c *cachedChecker) Allowed(ctx context.Context, token, namespace string) (bool, error) {
	key := cacheKey(token, namespace)
	now := c.now()
	c.mu.Lock()
	if d, ok := c.seen[key]; ok && now.Before(d.until) {
		c.mu.Unlock()
		return d.allowed, nil
	}
	c.mu.Unlock()
	allowed, err := c.next.Allowed(ctx, token, namespace)
	if err != nil {
		return false, err
	}
	c.mu.Lock()
	c.seen[key] = decision{allowed: allowed, until: now.Add(c.ttl)}
	// Keep the cache bounded: forget everything that expired.
	for k, d := range c.seen {
		if !now.Before(d.until) {
			delete(c.seen, k)
		}
	}
	c.mu.Unlock()
	return allowed, nil
}
