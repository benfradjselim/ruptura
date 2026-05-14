package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// k8sWorkload is the rich parsed shape of a Deployment/StatefulSet/DaemonSet.
type k8sWorkload struct {
	Metadata struct {
		Name              string            `json:"name"`
		Namespace         string            `json:"namespace"`
		Labels            map[string]string `json:"labels"`
		CreationTimestamp string            `json:"creationTimestamp"`
		ResourceVersion   string            `json:"resourceVersion"`
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
		Phase            string `json:"phase"`
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

// workloadWatchEvent wraps a single k8sWorkload watch event.
type workloadWatchEvent struct {
	Type   string      `json:"type"`
	Object k8sWorkload `json:"object"`
}

// podWatchEvent wraps a single k8sPod watch event.
type podWatchEvent struct {
	Type   string `json:"type"`
	Object k8sPod `json:"object"`
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

// watchWorkloads streams WATCH events for one resource type and updates cache.
func (inf *Informer) watchWorkloads(ctx context.Context, path, kind, rv string) error {
	url := fmt.Sprintf("%s/%s?watch=true&resourceVersion=%s&timeoutSeconds=300", inf.apiBase, path, rv)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+inf.token)

	resp, err := inf.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("watch %s: %w", path, err)
	}
	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var ev workloadWatchEvent
		if err := dec.Decode(&ev); err != nil {
			return fmt.Errorf("decode watch %s: %w", path, err)
		}
		m := ev.Object
		if m.Metadata.Namespace == "" || m.Metadata.Name == "" {
			continue
		}
		switch ev.Type {
		case "ADDED", "MODIFIED":
			inf.cache.setWorkload(m.Metadata.Namespace, kind, m.Metadata.Name, toWorkloadMeta(m, kind))
		case "DELETED":
			inf.cache.deleteWorkload(m.Metadata.Namespace, kind, m.Metadata.Name)
		}
	}
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

// watchPods streams WATCH events for pods and updates the pod cache.
func (inf *Informer) watchPods(ctx context.Context, rv string) error {
	url := fmt.Sprintf("%s/api/v1/pods?watch=true&resourceVersion=%s&timeoutSeconds=300", inf.apiBase, rv)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+inf.token)

	resp, err := inf.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("watch pods: %w", err)
	}
	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var ev podWatchEvent
		if err := dec.Decode(&ev); err != nil {
			return fmt.Errorf("decode watch pods: %w", err)
		}
		p := ev.Object
		switch ev.Type {
		case "ADDED", "MODIFIED":
			if cp, ok := toCachedPod(p); ok {
				inf.cache.upsertPod(cp)
			}
		case "DELETED":
			if p.Metadata.Namespace != "" && p.Metadata.Name != "" {
				inf.cache.deletePod(p.Metadata.Namespace, p.Metadata.Name)
			}
		}
	}
}
