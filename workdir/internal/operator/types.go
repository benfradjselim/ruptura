package operator

import "time"

// RupturaInstanceSpec defines the desired state of a RupturaInstance.
type RupturaInstanceSpec struct {
	Image       string `json:"image"`
	Port        int    `json:"port"`
	StorageSize string `json:"storageSize"`
	APIKeyRef   string `json:"apiKeyRef"`
	Replicas    int    `json:"replicas"`
}

// RupturaInstanceStatus reflects the observed state.
type RupturaInstanceStatus struct {
	Ready   bool      `json:"ready"`
	Message string    `json:"message"`
	Updated time.Time `json:"updated"`
}

// RupturaInstance is the CRD object.
type RupturaInstance struct {
	APIVersion string              `json:"apiVersion"`
	Kind       string              `json:"kind"`
	Metadata   ObjectMeta          `json:"metadata"`
	Spec       RupturaInstanceSpec   `json:"spec"`
	Status     RupturaInstanceStatus `json:"status,omitempty"`
}

// ObjectMeta mirrors k8s ObjectMeta (minimal).
type ObjectMeta struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// DeploymentSpec is what the operator creates for each RupturaInstance.
type DeploymentSpec struct {
	Name        string
	Namespace   string
	Image       string
	Port        int
	StorageSize string
	APIKeyRef   string
	Replicas    int
}
