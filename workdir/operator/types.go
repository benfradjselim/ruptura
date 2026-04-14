package main

// OHECluster is the Go representation of the OHECluster CRD.
type OHECluster struct {
	APIVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	Metadata   ObjectMeta        `json:"metadata"`
	Spec       OHEClusterSpec    `json:"spec"`
	Status     OHEClusterStatus  `json:"status,omitempty"`
}

// OHEClusterList wraps a list API response.
type OHEClusterList struct {
	Items    []OHECluster `json:"items"`
	Metadata ListMeta     `json:"metadata"`
}

// OHEClusterSpec is the desired state.
type OHEClusterSpec struct {
	Mode        string           `json:"mode"`
	Replicas    int32            `json:"replicas,omitempty"`
	Image       string           `json:"image,omitempty"`
	CentralURL  string           `json:"centralURL,omitempty"`
	StorageSize string           `json:"storageSize,omitempty"`
	AuthEnabled bool             `json:"authEnabled,omitempty"`
	Resources   ResourceRequirements `json:"resources,omitempty"`
}

// OHEClusterStatus is the observed state.
type OHEClusterStatus struct {
	ReadyReplicas       int32  `json:"readyReplicas"`
	AvailableReplicas   int32  `json:"availableReplicas"`
	Phase               string `json:"phase"`
	Message             string `json:"message,omitempty"`
	LastReconcileTime   string `json:"lastReconcileTime,omitempty"`
	ObservedGeneration  int64  `json:"observedGeneration,omitempty"`
}

// ObjectMeta is a subset of k8s ObjectMeta.
type ObjectMeta struct {
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace"`
	Generation      int64             `json:"generation,omitempty"`
	ResourceVersion string            `json:"resourceVersion,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Annotations     map[string]string `json:"annotations,omitempty"`
}

type ListMeta struct {
	ResourceVersion string `json:"resourceVersion"`
}

// ResourceRequirements mirrors K8s v1.ResourceRequirements.
type ResourceRequirements struct {
	Requests map[string]string `json:"requests,omitempty"`
	Limits   map[string]string `json:"limits,omitempty"`
}

// Deployment is the minimal K8s Deployment shape used for reconciliation.
type Deployment struct {
	APIVersion string         `json:"apiVersion"`
	Kind       string         `json:"kind"`
	Metadata   ObjectMeta     `json:"metadata"`
	Spec       DeploymentSpec `json:"spec"`
}

type DeploymentSpec struct {
	Replicas int32                  `json:"replicas"`
	Selector map[string]interface{} `json:"selector"`
	Template PodTemplateSpec        `json:"template"`
	Strategy map[string]interface{} `json:"strategy,omitempty"`
}

type PodTemplateSpec struct {
	Metadata ObjectMeta  `json:"metadata"`
	Spec     PodSpec     `json:"spec"`
}

type PodSpec struct {
	ServiceAccountName string      `json:"serviceAccountName,omitempty"`
	Containers         []Container `json:"containers"`
	Volumes            []Volume    `json:"volumes,omitempty"`
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
	Name string `json:"name"`
	Key  string `json:"key"`
}

type Probe struct {
	HTTPGet             HTTPGetAction `json:"httpGet"`
	InitialDelaySeconds int32         `json:"initialDelaySeconds"`
	PeriodSeconds       int32         `json:"periodSeconds"`
	FailureThreshold    int32         `json:"failureThreshold"`
}

type HTTPGetAction struct {
	Path string `json:"path"`
	Port string `json:"port"`
}

type Volume struct {
	Name                  string                       `json:"name"`
	PersistentVolumeClaim *PVCVolumeSource             `json:"persistentVolumeClaim,omitempty"`
	ConfigMap             *ConfigMapVolumeSource       `json:"configMap,omitempty"`
}

type PVCVolumeSource struct {
	ClaimName string `json:"claimName"`
}

type ConfigMapVolumeSource struct {
	Name string `json:"name"`
}

type VolumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
	ReadOnly  bool   `json:"readOnly,omitempty"`
}

// DeploymentStatus — minimal shape for reading ready/available replica counts.
type DeploymentStatus struct {
	ReadyReplicas     int32 `json:"readyReplicas"`
	AvailableReplicas int32 `json:"availableReplicas"`
}

type DeploymentWithStatus struct {
	Status DeploymentStatus `json:"status"`
}
