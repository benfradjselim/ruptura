package discovery

import (
	"strings"
	"sync"
)

// WorkloadMeta holds the cached Kubernetes metadata for one workload.
type WorkloadMeta struct {
	Namespace  string            `json:"namespace"`
	Kind       string            `json:"kind"`
	Name       string            `json:"name"`
	Replicas   ReplicaStatus     `json:"replicas"`
	Image      string            `json:"image"`
	Resources  ResourceInfo      `json:"resources"`
	Labels     map[string]string `json:"labels"`
	LastDeploy string            `json:"last_deploy"` // RFC3339, empty when unknown
	Pods       []PodInfo         `json:"pods"`
}

// ReplicaStatus mirrors k8s replica counts.
type ReplicaStatus struct {
	Desired   int `json:"desired"`
	Ready     int `json:"ready"`
	Available int `json:"available"`
}

// ResourceInfo holds requests and limits for the primary container.
type ResourceInfo struct {
	Requests ResourceSet `json:"requests"`
	Limits   ResourceSet `json:"limits"`
}

// ResourceSet is a CPU + memory pair from k8s resource requirements.
type ResourceSet struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// PodInfo is a lightweight snapshot of a single pod.
type PodInfo struct {
	Name     string `json:"name"`
	Node     string `json:"node"`
	Status   string `json:"status"`
	Restarts int    `json:"restarts"`
}

// cachedPod stores a pod and its owner reference for later workload association.
type cachedPod struct {
	info      PodInfo
	namespace string
	ownerKind string // "StatefulSet", "DaemonSet", or "ReplicaSet"
	ownerName string // direct owner name
}

// MetadataCache stores WorkloadMeta and pod data concurrency-safely.
type MetadataCache struct {
	mu        sync.RWMutex
	workloads map[string]*WorkloadMeta // key: "namespace/kind/name"
	pods      []cachedPod              // all known pods across namespaces
}

func newMetadataCache() *MetadataCache {
	return &MetadataCache{workloads: make(map[string]*WorkloadMeta)}
}

func workloadKey(ns, kind, name string) string {
	return ns + "/" + kind + "/" + name
}

func (c *MetadataCache) setWorkload(ns, kind, name string, m WorkloadMeta) {
	key := workloadKey(ns, kind, name)
	c.mu.Lock()
	c.workloads[key] = &m
	c.mu.Unlock()
}

func (c *MetadataCache) deleteWorkload(ns, kind, name string) {
	key := workloadKey(ns, kind, name)
	c.mu.Lock()
	delete(c.workloads, key)
	c.mu.Unlock()
}

// Get returns the WorkloadMeta for the given workload, with pods filled in.
func (c *MetadataCache) Get(ns, kind, name string) (WorkloadMeta, bool) {
	key := workloadKey(ns, kind, name)
	c.mu.RLock()
	m, ok := c.workloads[key]
	pods := c.podsForWorkload(ns, kind, name)
	c.mu.RUnlock()
	if !ok {
		return WorkloadMeta{}, false
	}
	cp := *m
	cp.Pods = pods
	return cp, true
}

// podsForWorkload matches cached pods against the workload; caller must hold at least RLock.
func (c *MetadataCache) podsForWorkload(ns, kind, name string) []PodInfo {
	var out []PodInfo
	for _, p := range c.pods {
		if p.namespace != ns {
			continue
		}
		switch kind {
		case "StatefulSet", "DaemonSet":
			if p.ownerKind == kind && p.ownerName == name {
				out = append(out, p.info)
			}
		case "Deployment":
			// Pods are owned by a ReplicaSet whose name is <deployment>-<hash>.
			if p.ownerKind == "ReplicaSet" && isRSOwnedBy(p.ownerName, name) {
				out = append(out, p.info)
			}
		}
	}
	return out
}

// isRSOwnedBy returns true when rsName matches "<deploymentName>-<suffix>".
func isRSOwnedBy(rsName, deploymentName string) bool {
	if !strings.HasPrefix(rsName, deploymentName+"-") {
		return false
	}
	suffix := rsName[len(deploymentName)+1:]
	return len(suffix) > 0 && !strings.Contains(suffix, "/")
}

// ResolvePodOwner returns the owning Deployment/StatefulSet/DaemonSet for a
// pod already observed by the pod watch loop (runPodLoop), or ok=false if
// the pod isn't cached yet or its owner can't be resolved (e.g. a bare Pod
// with no matching controller, or a ReplicaSet whose owning Deployment
// hasn't been listed yet). For ReplicaSet-owned pods (the Deployment case),
// the ReplicaSet name is matched against known Deployment names in the same
// namespace via the same "<deployment>-<hash>" prefix rule podsForWorkload
// already uses for the reverse (workload -> pods) lookup.
func (c *MetadataCache) ResolvePodOwner(ns, podName string) (kind, name string, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var pod *cachedPod
	for i := range c.pods {
		if c.pods[i].namespace == ns && c.pods[i].info.Name == podName {
			pod = &c.pods[i]
			break
		}
	}
	if pod == nil {
		return "", "", false
	}

	switch pod.ownerKind {
	case "StatefulSet", "DaemonSet":
		return pod.ownerKind, pod.ownerName, true
	case "ReplicaSet":
		for _, m := range c.workloads {
			if m.Kind != "Deployment" || m.Namespace != ns {
				continue
			}
			if isRSOwnedBy(pod.ownerName, m.Name) {
				return "Deployment", m.Name, true
			}
		}
	}
	return "", "", false
}

func (c *MetadataCache) upsertPod(p cachedPod) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, existing := range c.pods {
		if existing.namespace == p.namespace && existing.info.Name == p.info.Name {
			c.pods[i] = p
			return
		}
	}
	c.pods = append(c.pods, p)
}

func (c *MetadataCache) deletePod(ns, name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := c.pods[:0]
	for _, p := range c.pods {
		if p.namespace == ns && p.info.Name == name {
			continue
		}
		out = append(out, p)
	}
	c.pods = out
}
