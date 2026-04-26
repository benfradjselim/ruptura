package operator

import "time"

// KairoInstanceSpec defines the desired state of a KairoInstance.
type KairoInstanceSpec struct {
	Image       string `json:"image"`
	Port        int    `json:"port"`
	StorageSize string `json:"storageSize"`
	APIKeyRef   string `json:"apiKeyRef"`
	Replicas    int    `json:"replicas"`
}

// KairoInstanceStatus reflects the observed state.
type KairoInstanceStatus struct {
	Ready   bool      `json:"ready"`
	Message string    `json:"message"`
	Updated time.Time `json:"updated"`
}

// KairoInstance is the CRD object.
type KairoInstance struct {
	APIVersion string              `json:"apiVersion"`
	Kind       string              `json:"kind"`
	Metadata   ObjectMeta          `json:"metadata"`
	Spec       KairoInstanceSpec   `json:"spec"`
	Status     KairoInstanceStatus `json:"status,omitempty"`
}

// ObjectMeta mirrors k8s ObjectMeta (minimal).
type ObjectMeta struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// DeploymentSpec is what the operator creates for each KairoInstance.
type DeploymentSpec struct {
	Name        string
	Namespace   string
	Image       string
	Port        int
	StorageSize string
	APIKeyRef   string
	Replicas    int
}
