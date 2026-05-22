package main

// RupturaInstance is the CRD that users apply to deploy a Ruptura instance.
type RupturaInstance struct {
	APIVersion string                `json:"apiVersion"`
	Kind       string                `json:"kind"`
	Metadata   ObjectMeta            `json:"metadata"`
	Spec       RupturaInstanceSpec   `json:"spec"`
	Status     RupturaInstanceStatus `json:"status,omitempty"`
}

// RupturaInstanceList wraps a list API response.
type RupturaInstanceList struct {
	Items    []RupturaInstance `json:"items"`
	Metadata ListMeta          `json:"metadata"`
}

// RupturaInstanceSpec is the desired state declared by the user.
type RupturaInstanceSpec struct {
	// Image overrides the default Ruptura container image.
	Image string `json:"image,omitempty"`
	// Replicas must be 1 — BadgerDB is single-writer. Allowed for future multi-reader work.
	Replicas int32 `json:"replicas,omitempty"`
	// StorageSize is the PVC size for BadgerDB persistence (default: 10Gi).
	StorageSize string `json:"storageSize,omitempty"`
	// APIKeyRef is the name of a Secret whose "api-key" key becomes RUPTURA_API_KEY.
	APIKeyRef string `json:"apiKeyRef,omitempty"`
	// Edition is "community" or "autopilot" (default: community).
	Edition string `json:"edition,omitempty"`
	// IngestRPS is the token-bucket rate limit on the ingest HTTP server (default: 1000).
	IngestRPS int32 `json:"ingestRPS,omitempty"`
	// Resources sets CPU/memory requests and limits for the Ruptura container.
	Resources ResourceRequirements `json:"resources,omitempty"`
}

// RupturaInstanceStatus is the observed state written back by the operator.
type RupturaInstanceStatus struct {
	Phase             string `json:"phase"`
	ReadyReplicas     int32  `json:"readyReplicas"`
	AvailableReplicas int32  `json:"availableReplicas"`
	Message           string `json:"message,omitempty"`
	LastReconcileTime string `json:"lastReconcileTime,omitempty"`
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// ObjectMeta is a minimal subset of k8s ObjectMeta.
type ObjectMeta struct {
	Name                string            `json:"name"`
	Namespace           string            `json:"namespace"`
	Generation          int64             `json:"generation,omitempty"`
	ResourceVersion     string            `json:"resourceVersion,omitempty"`
	DeletionTimestamp   *string           `json:"deletionTimestamp,omitempty"`
	Finalizers          []string          `json:"finalizers,omitempty"`
	Labels              map[string]string `json:"labels,omitempty"`
	Annotations         map[string]string `json:"annotations,omitempty"`
}

type ListMeta struct {
	ResourceVersion string `json:"resourceVersion"`
}

// ResourceRequirements mirrors k8s v1.ResourceRequirements.
type ResourceRequirements struct {
	Requests map[string]string `json:"requests,omitempty"`
	Limits   map[string]string `json:"limits,omitempty"`
}

// --- Kubernetes resource shapes used during reconciliation ---

type Deployment struct {
	APIVersion string         `json:"apiVersion"`
	Kind       string         `json:"kind"`
	Metadata   ObjectMeta     `json:"metadata"`
	Spec       DeploymentSpec `json:"spec"`
}

type DeploymentSpec struct {
	Replicas int32                  `json:"replicas"`
	Selector map[string]interface{} `json:"selector"`
	Strategy map[string]interface{} `json:"strategy,omitempty"`
	Template PodTemplateSpec        `json:"template"`
}

type PodTemplateSpec struct {
	Metadata ObjectMeta `json:"metadata"`
	Spec     PodSpec    `json:"spec"`
}

type PodSpec struct {
	ServiceAccountName string      `json:"serviceAccountName,omitempty"`
	SecurityContext    *PodSecurityContext `json:"securityContext,omitempty"`
	Containers         []Container `json:"containers"`
	Volumes            []Volume    `json:"volumes,omitempty"`
}

type PodSecurityContext struct {
	RunAsNonRoot bool  `json:"runAsNonRoot"`
	RunAsUser    int64 `json:"runAsUser,omitempty"`
	FSGroup      int64 `json:"fsGroup,omitempty"`
}

type Container struct {
	Name            string               `json:"name"`
	Image           string               `json:"image"`
	Args            []string             `json:"args,omitempty"`
	Ports           []ContainerPort      `json:"ports,omitempty"`
	Env             []EnvVar             `json:"env,omitempty"`
	VolumeMounts    []VolumeMount        `json:"volumeMounts,omitempty"`
	LivenessProbe   *Probe               `json:"livenessProbe,omitempty"`
	ReadinessProbe  *Probe               `json:"readinessProbe,omitempty"`
	Resources       ResourceRequirements `json:"resources,omitempty"`
	SecurityContext *ContainerSecurityContext `json:"securityContext,omitempty"`
}

type ContainerSecurityContext struct {
	AllowPrivilegeEscalation bool `json:"allowPrivilegeEscalation"`
	ReadOnlyRootFilesystem   bool `json:"readOnlyRootFilesystem,omitempty"`
	RunAsNonRoot             bool `json:"runAsNonRoot"`
}

type ContainerPort struct {
	Name          string `json:"name"`
	ContainerPort int32  `json:"containerPort"`
	Protocol      string `json:"protocol"`
}

type EnvVar struct {
	Name      string        `json:"name"`
	Value     string        `json:"value,omitempty"`
	ValueFrom *EnvVarSource `json:"valueFrom,omitempty"`
}

type EnvVarSource struct {
	SecretKeyRef *SecretKeySelector `json:"secretKeyRef,omitempty"`
}

type SecretKeySelector struct {
	Name     string `json:"name"`
	Key      string `json:"key"`
	Optional bool   `json:"optional,omitempty"`
}

type Probe struct {
	HTTPGet             HTTPGetAction `json:"httpGet"`
	InitialDelaySeconds int32         `json:"initialDelaySeconds"`
	PeriodSeconds       int32         `json:"periodSeconds"`
	FailureThreshold    int32         `json:"failureThreshold"`
	TimeoutSeconds      int32         `json:"timeoutSeconds,omitempty"`
}

type HTTPGetAction struct {
	Path string `json:"path"`
	Port int32  `json:"port"`
}

type Volume struct {
	Name                  string               `json:"name"`
	PersistentVolumeClaim *PVCVolumeSource     `json:"persistentVolumeClaim,omitempty"`
}

type PVCVolumeSource struct {
	ClaimName string `json:"claimName"`
}

type VolumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
}

// Service shapes

type Service struct {
	APIVersion string      `json:"apiVersion"`
	Kind       string      `json:"kind"`
	Metadata   ObjectMeta  `json:"metadata"`
	Spec       ServiceSpec `json:"spec"`
}

type ServiceSpec struct {
	Selector map[string]string `json:"selector"`
	Ports    []ServicePort     `json:"ports"`
	Type     string            `json:"type,omitempty"`
}

type ServicePort struct {
	Name       string `json:"name"`
	Port       int32  `json:"port"`
	TargetPort int32  `json:"targetPort"`
	Protocol   string `json:"protocol"`
}

// PVC shapes

type PVC struct {
	APIVersion string     `json:"apiVersion"`
	Kind       string     `json:"kind"`
	Metadata   ObjectMeta `json:"metadata"`
	Spec       PVCSpec    `json:"spec"`
}

type PVCSpec struct {
	AccessModes []string              `json:"accessModes"`
	Resources   PVCResourceRequirements `json:"resources"`
}

type PVCResourceRequirements struct {
	Requests map[string]string `json:"requests"`
}

// OpenShift Route shapes

type Route struct {
	APIVersion string      `json:"apiVersion"`
	Kind       string      `json:"kind"`
	Metadata   ObjectMeta  `json:"metadata"`
	Spec       RouteSpec   `json:"spec"`
}

type RouteSpec struct {
	To   RouteTargetReference `json:"to"`
	Port RoutePort            `json:"port"`
	TLS  *RouteTLS            `json:"tls,omitempty"`
}

type RouteTargetReference struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Weight *int32 `json:"weight,omitempty"`
}

type RoutePort struct {
	TargetPort string `json:"targetPort"`
}

type RouteTLS struct {
	Termination string `json:"termination"`
}

// DeploymentWithStatus is used to read replica counts from the API.
type DeploymentWithStatus struct {
	Status struct {
		ReadyReplicas     int32 `json:"readyReplicas"`
		AvailableReplicas int32 `json:"availableReplicas"`
	} `json:"status"`
}

// Pod, PodList, and PodStatus are used to detect and clean up evicted pods.
type Pod struct {
	Metadata ObjectMeta `json:"metadata"`
	Status   PodStatus  `json:"status"`
}

type PodList struct {
	Items    []Pod    `json:"items"`
	Metadata ListMeta `json:"metadata"`
}

type PodStatus struct {
	Phase  string `json:"phase,omitempty"`
	Reason string `json:"reason,omitempty"`
}
