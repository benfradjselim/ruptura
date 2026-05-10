package models

import "strings"

// WorkloadRef identifies the primary treatment unit in Ruptura.
// In Kubernetes: namespace + kind + name identifies a workload uniquely.
// For non-K8s (bare-metal, VMs): Namespace="default", Kind="host", Name=hostname.
type WorkloadRef struct {
	Cluster   string `json:"cluster,omitempty"`  // optional, defaults to "default"
	Namespace string `json:"namespace"`
	Kind      string `json:"kind"`  // Deployment|StatefulSet|DaemonSet|Job|host
	Name      string `json:"name"` // workload name
	Node      string `json:"node,omitempty"` // infra node — secondary dimension
}

// Key returns the canonical string key used as map key throughout the engine.
func (w WorkloadRef) Key() string {
	if w.Namespace == "" {
		return "default/host/" + w.Name
	}
	return w.Namespace + "/" + w.Kind + "/" + w.Name
}

// IsEmpty returns true when the WorkloadRef carries no meaningful identity.
func (w WorkloadRef) IsEmpty() bool {
	return w.Name == ""
}

// WorkloadRefFromHost creates a degraded WorkloadRef for non-K8s sources.
// This preserves backward compatibility for bare-metal and VM deployments.
func WorkloadRefFromHost(host string) WorkloadRef {
	return WorkloadRef{
		Namespace: "default",
		Kind:      "host",
		Name:      host,
		Node:      host,
	}
}

// WorkloadRefFromKey parses a canonical pipeline key (namespace/kind/name) back into a WorkloadRef.
// Falls back to WorkloadRefFromHost for bare host strings.
func WorkloadRefFromKey(key string) WorkloadRef {
	parts := strings.SplitN(key, "/", 3)
	if len(parts) == 3 {
		return WorkloadRef{
			Namespace: parts[0],
			Kind:      parts[1],
			Name:      parts[2],
		}
	}
	return WorkloadRefFromHost(key)
}

// FirstNonEmpty returns the first non-empty string from the list.
func FirstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
