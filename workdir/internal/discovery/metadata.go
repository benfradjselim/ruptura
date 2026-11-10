package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// k8sWorkload is the rich parsed shape of a Deployment/StatefulSet/DaemonSet.
type k8sWorkload struct {
	Metadata struct {
		Name              string            `json:"name"`
		Namespace         string            `json:"namespace"`
		Labels            map[string]string `json:"labels"`
		CreationTimestamp string            `json:"creationTimestamp"`
		ResourceVersion   string            `json:"resourceVersion"`
		Generation        int64             `json:"generation"`
	} `json:"metadata"`
	Spec struct {
		Replicas int `json:"replicas"`
		Template struct {
			Spec struct {
				Containers []struct {
					Image     string `json:"image"`
					Resources struct {
						Requests struct {
							CPU    string `json:"cpu"`
							Memory string `json:"memory"`
						} `json:"requests"`
						Limits struct {
							CPU    string `json:"cpu"`
							Memory string `json:"memory"`
						} `json:"limits"`
					} `json:"resources"`
				} `json:"containers"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
	Status struct {
		Replicas          int `json:"replicas"`
		ReadyReplicas     int `json:"readyReplicas"`
		AvailableReplicas int `json:"availableReplicas"`
		Conditions        []struct {
			Type               string `json:"type"`
			LastTransitionTime string `json:"lastTransitionTime"`
		} `json:"conditions"`
	} `json:"status"`
}

// k8sPod is the parsed shape of a Pod.
type k8sPod struct {
	Metadata struct {
		Name            string `json:"name"`
		Namespace       string `json:"namespace"`
		ResourceVersion string `json:"resourceVersion"`
		OwnerReferences []struct {
			Kind string `json:"kind"`
			Name string `json:"name"`
		} `json:"ownerReferences"`
	} `json:"metadata"`
	Spec struct {
		NodeName string `json:"nodeName"`
	} `json:"spec"`
	Status struct {
		Phase             string `json:"phase"`
		ContainerStatuses []struct {
			RestartCount int `json:"restartCount"`
		} `json:"containerStatuses"`
	} `json:"status"`
}

// workloadListResp is the JSON shape of a LIST response for workloads.
type workloadListResp struct {
	Metadata struct {
		ResourceVersion string `json:"resourceVersion"`
		Continue        string `json:"continue"`
	} `json:"metadata"`
	Items []k8sWorkload `json:"items"`
}

// podListResp is the JSON shape of a LIST response for pods.
type podListResp struct {
	Metadata struct {
		ResourceVersion string `json:"resourceVersion"`
		Continue        string `json:"continue"`
	} `json:"metadata"`
	Items []k8sPod `json:"items"`
}

func toWorkloadMeta(w k8sWorkload, kind string) WorkloadMeta {
	m := WorkloadMeta{
		Namespace: w.Metadata.Namespace,
		Kind:      kind,
		Name:      w.Metadata.Name,
		Labels:    w.Metadata.Labels,
		Replicas: ReplicaStatus{
			Desired:   w.Spec.Replicas,
			Ready:     w.Status.ReadyReplicas,
			Available: w.Status.AvailableReplicas,
		},
		Generation: w.Metadata.Generation,
		LastDeploy: w.Metadata.CreationTimestamp,
	}

	// Use last Available condition transition time as a better proxy for last deploy.
	for _, c := range w.Status.Conditions {
		if c.Type == "Available" && c.LastTransitionTime != "" {
			m.LastDeploy = c.LastTransitionTime
		}
	}

	if len(w.Spec.Template.Spec.Containers) > 0 {
		c := w.Spec.Template.Spec.Containers[0]
		m.Image = c.Image
		m.Resources = ResourceInfo{
			Requests: ResourceSet{CPU: c.Resources.Requests.CPU, Memory: c.Resources.Requests.Memory},
			Limits:   ResourceSet{CPU: c.Resources.Limits.CPU, Memory: c.Resources.Limits.Memory},
		}
	}
	return m
}

func toCachedPod(p k8sPod) (cachedPod, bool) {
	if len(p.Metadata.OwnerReferences) == 0 {
		return cachedPod{}, false
	}
	owner := p.Metadata.OwnerReferences[0]
	restarts := 0
	if len(p.Status.ContainerStatuses) > 0 {
		restarts = p.Status.ContainerStatuses[0].RestartCount
	}
	return cachedPod{
		namespace: p.Metadata.Namespace,
		ownerKind: owner.Kind,
		ownerName: owner.Name,
		info: PodInfo{
			Name:     p.Metadata.Name,
			Node:     p.Spec.NodeName,
			Status:   p.Status.Phase,
			Restarts: restarts,
		},
	}, true
}

// listWorkloads performs a full LIST for one resource type, populates cache.
func (inf *Informer) listWorkloads(ctx context.Context, path, kind string) (string, error) {
	url := fmt.Sprintf("%s/%s?limit=500", inf.apiBase, path)
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+inf.token)

		resp, err := inf.httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("list %s: %w", path, err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", fmt.Errorf("read list %s: %w", path, err)
		}

		var lr workloadListResp
		if err := json.Unmarshal(body, &lr); err != nil {
			return "", fmt.Errorf("decode list %s: %w", path, err)
		}
		for _, item := range lr.Items {
			if item.Metadata.Namespace == "" || item.Metadata.Name == "" {
				continue
			}
			inf.cache.setWorkload(item.Metadata.Namespace, kind, item.Metadata.Name, toWorkloadMeta(item, kind))
		}
		if lr.Metadata.Continue == "" {
			return lr.Metadata.ResourceVersion, nil
		}
		url = fmt.Sprintf("%s/%s?limit=500&continue=%s", inf.apiBase, path, lr.Metadata.Continue)
	}
}

// errWatchExpired reports that the API server no longer holds the requested
// resourceVersion, so the cache must be rebuilt with a LIST.
var errWatchExpired = errors.New("resource version expired")

// rawWatchEvent is the envelope shared by every WATCH event; Object is decoded
// by the caller once the type is known.
type rawWatchEvent struct {
	Type   string          `json:"type"`
	Object json.RawMessage `json:"object"`
}

// openWatch starts a WATCH on path from rv. A 410 answer is errWatchExpired.
func (inf *Informer) openWatch(ctx context.Context, path, rv string) (*http.Response, error) {
	url := fmt.Sprintf("%s/%s?watch=true&allowWatchBookmarks=true&resourceVersion=%s&timeoutSeconds=300", inf.apiBase, path, rv)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+inf.token)
	resp, err := inf.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("watch %s: %w", path, err)
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return resp, nil
	case http.StatusGone:
		resp.Body.Close()
		return nil, errWatchExpired
	default:
		resp.Body.Close()
		return nil, fmt.Errorf("watch %s: unexpected status %d", path, resp.StatusCode)
	}
}

// streamWatch decodes events until the stream ends and returns the last
// resourceVersion seen, from which the next WATCH resumes. The server closing
// the stream after timeoutSeconds is the normal end and returns a nil error.
// An ERROR event, which carries 410 when rv has expired, returns errWatchExpired.
func streamWatch(body io.Reader, rv string, handle func(eventType string, object json.RawMessage) string) (string, error) {
	dec := json.NewDecoder(body)
	for {
		var ev rawWatchEvent
		if err := dec.Decode(&ev); err != nil {
			if errors.Is(err, io.EOF) {
				return rv, nil
			}
			return rv, fmt.Errorf("decode watch: %w", err)
		}
		switch ev.Type {
		case "ERROR":
			return rv, errWatchExpired
		case "BOOKMARK":
			var b struct {
				Metadata struct {
					ResourceVersion string `json:"resourceVersion"`
				} `json:"metadata"`
			}
			if json.Unmarshal(ev.Object, &b) == nil && b.Metadata.ResourceVersion != "" {
				rv = b.Metadata.ResourceVersion
			}
		default:
			if next := handle(ev.Type, ev.Object); next != "" {
				rv = next
			}
		}
	}
}

// watchWorkloads streams WATCH events for one resource type, updates the cache
// and returns the resourceVersion to resume from.
func (inf *Informer) watchWorkloads(ctx context.Context, path, kind, rv string) (string, error) {
	resp, err := inf.openWatch(ctx, path, rv)
	if err != nil {
		return rv, err
	}
	defer resp.Body.Close()
	return streamWatch(resp.Body, rv, func(eventType string, object json.RawMessage) string {
		var m k8sWorkload
		if json.Unmarshal(object, &m) != nil {
			return ""
		}
		if m.Metadata.Namespace == "" || m.Metadata.Name == "" {
			return m.Metadata.ResourceVersion
		}
		switch eventType {
		case "ADDED", "MODIFIED":
			meta := toWorkloadMeta(m, kind)
			inf.cache.recordChange(m.Metadata.Namespace, kind, m.Metadata.Name, meta.Image, meta.Generation, time.Now())
			inf.cache.setWorkload(m.Metadata.Namespace, kind, m.Metadata.Name, meta)
		case "DELETED":
			inf.cache.deleteWorkload(m.Metadata.Namespace, kind, m.Metadata.Name)
		}
		return m.Metadata.ResourceVersion
	})
}

// listPods performs a full LIST of pods and populates the pod cache.
func (inf *Informer) listPods(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/api/v1/pods?limit=500", inf.apiBase)
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+inf.token)

		resp, err := inf.httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("list pods: %w", err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", fmt.Errorf("read list pods: %w", err)
		}

		var lr podListResp
		if err := json.Unmarshal(body, &lr); err != nil {
			return "", fmt.Errorf("decode list pods: %w", err)
		}
		for _, p := range lr.Items {
			if cp, ok := toCachedPod(p); ok {
				inf.cache.upsertPod(cp)
			}
		}
		if lr.Metadata.Continue == "" {
			return lr.Metadata.ResourceVersion, nil
		}
		url = fmt.Sprintf("%s/api/v1/pods?limit=500&continue=%s", inf.apiBase, lr.Metadata.Continue)
	}
}

// watchPods streams WATCH events for pods, updates the pod cache and returns
// the resourceVersion to resume from.
func (inf *Informer) watchPods(ctx context.Context, rv string) (string, error) {
	resp, err := inf.openWatch(ctx, "api/v1/pods", rv)
	if err != nil {
		return rv, err
	}
	defer resp.Body.Close()
	return streamWatch(resp.Body, rv, func(eventType string, object json.RawMessage) string {
		var p k8sPod
		if json.Unmarshal(object, &p) != nil {
			return ""
		}
		switch eventType {
		case "ADDED", "MODIFIED":
			if cp, ok := toCachedPod(p); ok {
				inf.cache.upsertPod(cp)
			}
		case "DELETED":
			if p.Metadata.Namespace != "" && p.Metadata.Name != "" {
				inf.cache.deletePod(p.Metadata.Namespace, p.Metadata.Name)
			}
		}
		return p.Metadata.ResourceVersion
	})
}
