package discovery

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/benfradjselim/ruptura/pkg/models"
)

// resource describes a single k8s resource type to watch.
type resource struct {
	path string // e.g. "apis/apps/v1/deployments"
	kind string // e.g. "Deployment"
}

// k8sObjectMeta is the minimal JSON shape we need from any k8s object.
type k8sObjectMeta struct {
	Metadata struct {
		Name            string `json:"name"`
		Namespace       string `json:"namespace"`
		ResourceVersion string `json:"resourceVersion"`
	} `json:"metadata"`
	Items []k8sObjectMeta `json:"items,omitempty"`
}

// watchEvent is the envelope for a single watch stream event.
type watchEvent struct {
	Type   string        `json:"type"`   // ADDED|MODIFIED|DELETED|BOOKMARK|ERROR
	Object k8sObjectMeta `json:"object"`
}

// listResponse carries the top-level resourceVersion for a LIST response.
type listResponse struct {
	Metadata struct {
		ResourceVersion string `json:"resourceVersion"`
	} `json:"metadata"`
	Items []k8sObjectMeta `json:"items"`
}

// watchResource runs a perpetual LIST+WATCH loop for one resource type.
// It calls onAdd/onDelete for each event and reconnects on errors. onFirstList
// (may be nil) is called once, after the very first successful LIST — not on
// subsequent relists (410 Gone, stream drops, etc.), which is why it's
// signaled here rather than left for the caller to infer from onAdd (a
// resource type with zero live objects would otherwise never signal at all).
// Exported as a method so tests can inject a custom apiBase and httpClient.
func (inf *Informer) watchResource(ctx context.Context, res resource, onAdd, onDelete func(models.WorkloadRef), onFirstList func()) {
	backoff := time.Second
	const maxBackoff = 30 * time.Second
	firstListDone := false

	for {
		if ctx.Err() != nil {
			return
		}

		rv, err := inf.listAll(ctx, res, onAdd)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			inf.logf("discovery: list %s failed: %v — retrying in %s", res.path, err, backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff = minDuration(backoff*2, maxBackoff)
			continue
		}
		backoff = time.Second // reset after successful list

		if !firstListDone {
			firstListDone = true
			if onFirstList != nil {
				onFirstList()
			}
		}

		// WATCH from the resource version we got from the list.
		relistNeeded, err := inf.watchStream(ctx, res, rv, onAdd, onDelete)
		if err != nil && ctx.Err() == nil {
			inf.logf("discovery: watch %s error: %v — re-listing", res.path, err)
		}
		if !relistNeeded && ctx.Err() != nil {
			return
		}
		// Either explicit relist (410 Gone) or stream ended — loop back to LIST.
	}
}

// listAll performs a paginated LIST, calling onAdd for each item.
// Returns the resourceVersion to use for the subsequent WATCH.
func (inf *Informer) listAll(ctx context.Context, res resource, onAdd func(models.WorkloadRef)) (string, error) {
	url := fmt.Sprintf("%s/%s?limit=500", inf.apiBase, res.path)
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+inf.token)

		resp, err := inf.httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("list %s: %w", res.path, err)
		}

		var lr listResponse
		if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
			resp.Body.Close()
			return "", fmt.Errorf("list %s decode: %w", res.path, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("list %s: unexpected status %d", res.path, resp.StatusCode)
		}

		for _, item := range lr.Items {
			if item.Metadata.Namespace == "" || item.Metadata.Name == "" {
				continue
			}
			onAdd(models.WorkloadRef{
				Namespace: item.Metadata.Namespace,
				Kind:      res.kind,
				Name:      item.Metadata.Name,
			})
		}

		// No continue token → list complete.
		return lr.Metadata.ResourceVersion, nil
	}
}

// watchStream opens a WATCH connection and processes events until the stream ends
// or the context is cancelled. Returns (relistNeeded=true, nil) on 410 Gone.
func (inf *Informer) watchStream(ctx context.Context, res resource, rv string, onAdd, onDelete func(models.WorkloadRef)) (relistNeeded bool, err error) {
	url := fmt.Sprintf("%s/%s?watch=true&allowWatchBookmarks=true&resourceVersion=%s&timeoutSeconds=300",
		inf.apiBase, res.path, rv)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+inf.token)

	resp, err := inf.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("watch %s: %w", res.path, err)
	}
	defer resp.Body.Close()

	// 410 Gone means resourceVersion is too old — caller must re-list.
	if resp.StatusCode == http.StatusGone {
		return true, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("watch %s: unexpected status %d", res.path, resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20) // 1 MB max line

	for scanner.Scan() {
		if ctx.Err() != nil {
			return false, nil
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var ev watchEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue // skip malformed lines
		}

		switch ev.Type {
		case "ADDED", "MODIFIED":
			m := ev.Object.Metadata
			if m.Namespace != "" && m.Name != "" {
				onAdd(models.WorkloadRef{
					Namespace: m.Namespace,
					Kind:      res.kind,
					Name:      m.Name,
				})
			}
		case "DELETED":
			m := ev.Object.Metadata
			if m.Namespace != "" && m.Name != "" {
				onDelete(models.WorkloadRef{
					Namespace: m.Namespace,
					Kind:      res.kind,
					Name:      m.Name,
				})
			}
		case "ERROR":
			// Check for 410 Gone inside the event stream.
			var errObj struct {
				Code int `json:"code"`
			}
			raw, _ := json.Marshal(ev.Object)
			_ = json.Unmarshal(raw, &errObj)
			if errObj.Code == http.StatusGone {
				return true, nil
			}
		case "BOOKMARK":
			// Update rv from bookmark (not needed since we always re-list on reconnect).
		}
	}

	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		return false, fmt.Errorf("watch %s stream: %w", res.path, err)
	}
	return false, nil
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
