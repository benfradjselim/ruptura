package main

import (
	"fmt"
	"time"
	"github.com/benfradjselim/kairo-core/pkg/logger"
)

const (
	defaultImage       = "ghcr.io/benfradjselim/ohe:4.0.0"
	defaultStorageSize = "10Gi"
)

// reconcile is the core control loop for a single OHECluster resource.
// It ensures the desired Deployment (or DaemonSet for agent mode) exists
// and has the correct spec, then updates the status subresource.
func reconcile(c *k8sClient, cluster OHECluster) {
	ns := cluster.Metadata.Namespace
	name := cluster.Metadata.Name
	spec := cluster.Spec

	logger.Default.Info("reconcile", "ns", ns, "name", name, "mode", spec.Mode, "replicas", replicas(spec))

	image := spec.Image
	if image == "" {
		image = defaultImage
	}

	var err error
	switch spec.Mode {
	case "central":
		err = reconcileCentral(c, cluster, image)
	case "agent":
		err = reconcileAgent(c, cluster, image)
	default:
		err = fmt.Errorf("unknown mode %q", spec.Mode)
	}

	phase, msg := "Running", ""
	if err != nil {
		phase, msg = "Failed", err.Error()
		logger.Default.Error("reconcile error", "ns", ns, "name", name, "err", err)
	}

	// Read ready replica count from the managed Deployment
	ready, available := readDeploymentStatus(c, ns, name)

	status := OHEClusterStatus{
		Phase:              phase,
		Message:            msg,
		ReadyReplicas:      ready,
		AvailableReplicas:  available,
		LastReconcileTime:  time.Now().UTC().Format(time.RFC3339),
		ObservedGeneration: cluster.Metadata.Generation,
	}

	path := fmt.Sprintf("/apis/ohe.io/v1alpha1/namespaces/%s/oheclusters/%s", ns, name)
	if err := c.patchStatus(path, status); err != nil {
		logger.Default.Error("status update error", "ns", ns, "name", name, "err", err)
	}
}

func reconcileCentral(c *k8sClient, cluster OHECluster, image string) error {
	ns := cluster.Metadata.Namespace
	name := cluster.Metadata.Name
	spec := cluster.Spec

	rep := replicas(spec)

	// Ensure PVC exists before creating the Deployment that references it.
	pvc := PVC{
		APIVersion: "v1",
		Kind:       "PersistentVolumeClaim",
		Metadata: ObjectMeta{
			Name:      name + "-data",
			Namespace: ns,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "ohe",
				"app.kubernetes.io/component":  "central",
				"app.kubernetes.io/managed-by": "ohe-operator",
			},
		},
		Spec: PVCSpec{
			AccessModes: []string{"ReadWriteOnce"},
			Resources: map[string]interface{}{
				"requests": map[string]string{"storage": "10Gi"},
			},
		},
	}
	pvcPath := fmt.Sprintf("/api/v1/namespaces/%s/persistentvolumeclaims/%s", ns, name+"-data")
	if err := c.apply(pvcPath, pvc); err != nil {
		return fmt.Errorf("reconcile PVC: %w", err)
	}

	dep := Deployment{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Metadata: ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				"app.kubernetes.io/name":      "ohe",
				"app.kubernetes.io/component": "central",
				"app.kubernetes.io/managed-by": "ohe-operator",
			},
		},
		Spec: DeploymentSpec{
			Replicas: rep,
			Selector: map[string]interface{}{
				"matchLabels": map[string]string{"app": name},
			},
			Strategy: map[string]interface{}{
				"type": "Recreate", // Badger is single-writer
			},
			Template: PodTemplateSpec{
				Metadata: ObjectMeta{
					Labels: map[string]string{"app": name},
				},
				Spec: PodSpec{
					ServiceAccountName: "ohe-central",
					Containers: []Container{
						{
							Name:  "ohe",
							Image: image,
							Args: []string{
								"central",
								"--config=/etc/ohe/config.yaml",
								"--storage=/var/lib/ohe/data",
							},
							Ports: []ContainerPort{
								{Name: "http", ContainerPort: 8080, Protocol: "TCP"},
							},
							Env: []EnvVar{
								{
									Name: "OHE_JWT_SECRET",
									ValueFrom: &EnvVarSource{
										SecretKeyRef: &SecretKeySelector{
											Name: "ohe-secrets",
											Key:  "jwt-secret",
										},
									},
								},
							},
							VolumeMounts: []VolumeMount{
								{Name: "data", MountPath: "/var/lib/ohe/data"},
								{Name: "config", MountPath: "/etc/ohe", ReadOnly: true},
							},
							LivenessProbe: &Probe{
								HTTPGet:             HTTPGetAction{Path: "/api/v1/health/live", Port: "http"},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
								FailureThreshold:    3,
							},
							ReadinessProbe: &Probe{
								HTTPGet:             HTTPGetAction{Path: "/api/v1/health/ready", Port: "http"},
								InitialDelaySeconds: 3,
								PeriodSeconds:       5,
								FailureThreshold:    2,
							},
							Resources: resourcesOrDefault(spec.Resources, "100m", "128Mi", "500m", "512Mi"),
						},
					},
					Volumes: []Volume{
						{Name: "data", PersistentVolumeClaim: &PVCVolumeSource{ClaimName: name + "-data"}},
						{Name: "config", ConfigMap: &ConfigMapVolumeSource{Name: "ohe-config"}},
					},
				},
			},
		},
	}

	path := fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments/%s", ns, name)
	return c.apply(path, dep)
}

func reconcileAgent(c *k8sClient, cluster OHECluster, image string) error {
	// Agent mode uses a DaemonSet; for simplicity the operator creates/updates
	// a Deployment here (a real production operator would use a DaemonSet).
	ns := cluster.Metadata.Namespace
	name := cluster.Metadata.Name
	spec := cluster.Spec

	central := spec.CentralURL
	if central == "" {
		central = "http://ohe-central." + ns + ".svc.cluster.local"
	}

	dep := Deployment{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Metadata: ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "ohe",
				"app.kubernetes.io/component":  "agent",
				"app.kubernetes.io/managed-by": "ohe-operator",
			},
		},
		Spec: DeploymentSpec{
			Replicas: replicas(spec),
			Selector: map[string]interface{}{
				"matchLabels": map[string]string{"app": name},
			},
			Template: PodTemplateSpec{
				Metadata: ObjectMeta{
					Labels: map[string]string{"app": name},
				},
				Spec: PodSpec{
					ServiceAccountName: "ohe-agent",
					Containers: []Container{
						{
							Name:    "ohe-agent",
							Image:   image,
							Command: []string{"/ohe", "agent"}, // override ENTRYPOINT (defaults to 'central')
							Args: []string{
								"--central-url=" + central,
								"--interval=15s",
							},
							Ports: []ContainerPort{
								{Name: "http", ContainerPort: 8080, Protocol: "TCP"}, // OHE listens on 8080
							},
							LivenessProbe: &Probe{
								HTTPGet:             HTTPGetAction{Path: "/api/v1/health/live", Port: "http"},
								InitialDelaySeconds: 5,
								PeriodSeconds:       15,
								FailureThreshold:    3,
							},
							Resources: resourcesOrDefault(spec.Resources, "50m", "64Mi", "200m", "128Mi"),
						},
					},
				},
			},
		},
	}

	path := fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments/%s", ns, name)
	return c.apply(path, dep)
}

func readDeploymentStatus(c *k8sClient, ns, name string) (ready, available int32) {
	var dep DeploymentWithStatus
	path := fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments/%s", ns, name)
	if err := c.get(path, &dep); err != nil {
		return 0, 0
	}
	return dep.Status.ReadyReplicas, dep.Status.AvailableReplicas
}

func replicas(spec OHEClusterSpec) int32 {
	if spec.Replicas <= 0 {
		return 1
	}
	return spec.Replicas
}

// resourcesOrDefault returns spec.Resources if non-empty, else sensible defaults.
func resourcesOrDefault(r ResourceRequirements, reqCPU, reqMem, limCPU, limMem string) ResourceRequirements {
	if len(r.Requests) > 0 || len(r.Limits) > 0 {
		return r
	}
	return ResourceRequirements{
		Requests: map[string]string{"cpu": reqCPU, "memory": reqMem},
		Limits:   map[string]string{"cpu": limCPU, "memory": limMem},
	}
}
