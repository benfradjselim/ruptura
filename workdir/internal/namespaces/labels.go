// Package namespaces keeps the labels of every namespace, read from the
// Kubernetes API, so results can be limited to namespaces a team opts in.
package namespaces

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/benfradjselim/ruptura/internal/discovery"
	"github.com/benfradjselim/ruptura/pkg/logger"
)

const (
	refreshInterval = time.Minute
	requestTimeout  = 30 * time.Second
	pageSize        = 500
)

// Labels refreshes namespace labels. It is safe for concurrent use.
type Labels struct {
	apiBase string
	token   string
	client  *http.Client
	mu      sync.RWMutex
	labels  map[string]map[string]string
}

// New returns a reader with the pod's in-cluster credentials.
func New() (*Labels, error) {
	apiBase, token, client, err := discovery.InClusterCreds()
	if err != nil {
		return nil, fmt.Errorf("namespaces: %w", err)
	}
	return &Labels{apiBase: apiBase, token: token, client: client}, nil
}

// Of returns the labels of a namespace, and false when it is not known yet.
func (l *Labels) Of(namespace string) (map[string]string, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	labels, ok := l.labels[namespace]
	return labels, ok
}

// Run refreshes the labels until ctx ends.
func (l *Labels) Run(ctx context.Context) {
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()
	for {
		if err := l.refresh(ctx); err != nil {
			logger.Default.Warn("namespace labels refresh failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

type namespaceList struct {
	Metadata struct {
		Continue string `json:"continue"`
	} `json:"metadata"`
	Items []struct {
		Metadata struct {
			Name   string            `json:"name"`
			Labels map[string]string `json:"labels"`
		} `json:"metadata"`
	} `json:"items"`
}

func (l *Labels) refresh(ctx context.Context) error {
	found := map[string]map[string]string{}
	next := ""
	for {
		query := url.Values{"limit": {fmt.Sprint(pageSize)}}
		if next != "" {
			query.Set("continue", next)
		}
		page, err := l.get(ctx, "api/v1/namespaces?"+query.Encode())
		if err != nil {
			return err
		}
		for _, item := range page.Items {
			found[item.Metadata.Name] = item.Metadata.Labels
		}
		if page.Metadata.Continue == "" {
			break
		}
		next = page.Metadata.Continue
	}
	l.mu.Lock()
	l.labels = found
	l.mu.Unlock()
	return nil
}

func (l *Labels) get(ctx context.Context, path string) (namespaceList, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.apiBase+"/"+path, nil)
	if err != nil {
		return namespaceList{}, err
	}
	req.Header.Set("Authorization", "Bearer "+l.token)
	resp, err := l.client.Do(req)
	if err != nil {
		return namespaceList{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return namespaceList{}, fmt.Errorf("list namespaces: status %d", resp.StatusCode)
	}
	var page namespaceList
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return namespaceList{}, fmt.Errorf("list namespaces: decode: %w", err)
	}
	return page, nil
}
