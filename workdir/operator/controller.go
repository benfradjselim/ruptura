package main

import (
	"context"
	"fmt"
	"time"
)

const (
	defaultImage       = "ghcr.io/benfradjselim/ruptura:v7.0.18"
	defaultStorageSize = "10Gi"
	defaultEdition     = "community"
	appLabel           = "app.kubernetes.io/name"
	managedByLabel     = "app.kubernetes.io/managed-by"
	instanceLabel      = "app.kubernetes.io/instance"
	finalizer          = "ruptura.io/cleanup"
)

// hasFinalizer reports whether inst already carries our finalizer.
func hasFinalizer(inst RupturaInstance) bool {
	for _, f := range inst.Metadata.Finalizers {
		if f == finalizer {
			return true
		}
	}
	return false
}

// removeFinalizer returns a new slice with our finalizer removed.
func removeFinalizer(inst RupturaInstance) []string {
	out := make([]string, 0, len(inst.Metadata.Finalizers))
	for _, f := range inst.Metadata.Finalizers {
		if f != finalizer {
			out = append(out, f)
		}
	}
	return out
}

// cleanup deletes all resources owned by a RupturaInstance then removes the
// finalizer so the API server can complete the deletion.
func cleanup(c *k8sClient, inst RupturaInstance, isOCP bool) error {
	ns := inst.Metadata.Namespace
	name := inst.Metadata.Name

	if isOCP {
		if err := c.delete(fmt.Sprintf("/apis/route.openshift.io/v1/namespaces/%s/routes/%s", ns, name)); err != nil {
			return fmt.Errorf("delete Route: %w", err)
		}
	}
	if err := c.delete(fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments/%s", ns, name)); err != nil {
		return fmt.Errorf("delete Deployment: %w", err)
	}
	if err := c.delete(fmt.Sprintf("/api/v1/namespaces/%s/services/%s", ns, name)); err != nil {
		return fmt.Errorf("delete Service: %w", err)
	}
	if err := c.delete(fmt.Sprintf("/api/v1/namespaces/%s/persistentvolumeclaims/%s-data", ns, name)); err != nil {
		return fmt.Errorf("delete PVC: %w", err)
	}
	if err := c.delete(fmt.Sprintf("/api/v1/namespaces/%s/serviceaccounts/ruptura-instance", ns)); err != nil {
		return fmt.Errorf("delete ServiceAccount: %w", err)
	}
	return c.patchFinalizers(inst, removeFinalizer(inst))
}

// reconcile drives a single RupturaInstance toward its desired state.
// It is idempotent: server-side apply handles create-or-update for all resources.
func reconcile(ctx context.Context, c *k8sClient, inst RupturaInstance, isOCP bool) error {
	// Handle deletion: run cleanup then remove our finalizer.
	if inst.Metadata.DeletionTimestamp != nil {
		if hasFinalizer(inst) {
			return cleanup(c, inst, isOCP)
		}
		return nil
	}

	// Ensure our finalizer is registered before touching any resources.
	if !hasFinalizer(inst) {
		updated := append(inst.Metadata.Finalizers, finalizer)
		if err := c.patchFinalizers(inst, updated); err != nil {
			return fmt.Errorf("add finalizer: %w", err)
		}
	}

	ns := inst.Metadata.Namespace
	name := inst.Metadata.Name

	image := inst.Spec.Image
	if image == "" {
		image = defaultImage
	}
	storageSize := inst.Spec.StorageSize
	if storageSize == "" {
		storageSize = defaultStorageSize
	}
	edition := inst.Spec.Edition
	if edition == "" {
		edition = defaultEdition
	}
	replicas := inst.Spec.Replicas
	if replicas < 1 {
		replicas = 1
	}

	labels := map[string]string{
		appLabel:       "ruptura",
		managedByLabel: "ruptura-operator",
		instanceLabel:  name,
	}

	if err := reconcileServiceAccount(c, ns, labels); err != nil {
		return fmt.Errorf("ServiceAccount: %w", err)
	}
	if err := reconcilePVC(c, ns, name, storageSize, labels); err != nil {
		return fmt.Errorf("PVC: %w", err)
	}
	if err := reconcileDeployment(c, ns, name, image, edition, replicas, inst.Spec, labels); err != nil {
		return fmt.Errorf("Deployment: %w", err)
	}
	if err := reconcileService(c, ns, name, labels); err != nil {
		return fmt.Errorf("Service: %w", err)
	}
	if isOCP {
		if err := reconcileRoute(c, ns, name, labels); err != nil {
			return fmt.Errorf("Route: %w", err)
		}
	}

	return updateStatus(c, inst)
}

func reconcileServiceAccount(c *k8sClient, ns string, labels map[string]string) error {
	sa := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ServiceAccount",
		"metadata": map[string]interface{}{
			"name":      "ruptura-instance",
			"namespace": ns,
			"labels":    labels,
		},
	}
	path := fmt.Sprintf("/api/v1/namespaces/%s/serviceaccounts/ruptura-instance", ns)
	return c.apply(path, sa)
}

func reconcilePVC(c *k8sClient, ns, name, size string, labels map[string]string) error {
	pvc := PVC{
		APIVersion: "v1",
		Kind:       "PersistentVolumeClaim",
		Metadata: ObjectMeta{
			Name:      name + "-data",
			Namespace: ns,
			Labels:    labels,
		},
		Spec: PVCSpec{
			AccessModes: []string{"ReadWriteOnce"},
			Resources: PVCResourceRequirements{
				Requests: map[string]string{"storage": size},
			},
		},
	}
	path := fmt.Sprintf("/api/v1/namespaces/%s/persistentvolumeclaims/%s-data", ns, name)
	return c.apply(path, pvc)
}

func reconcileDeployment(c *k8sClient, ns, name, image, edition string, replicas int32, spec RupturaInstanceSpec, labels map[string]string) error {
	env := []EnvVar{
		{Name: "RUPTURA_EDITION", Value: edition},
	}
	if spec.APIKeyRef != "" {
		env = append(env, EnvVar{
			Name: "RUPTURA_API_KEY",
			ValueFrom: &EnvVarSource{
				SecretKeyRef: &SecretKeySelector{
					Name:     spec.APIKeyRef,
					Key:      "api-key",
					Optional: true,
				},
			},
		})
	}
	if spec.IngestRPS > 0 {
		env = append(env, EnvVar{
			Name:  "RUPTURA_INGEST_RPS",
			Value: fmt.Sprintf("%d", spec.IngestRPS),
		})
	}

	resources := spec.Resources
	if len(resources.Requests) == 0 && len(resources.Limits) == 0 {
		resources = ResourceRequirements{
			Requests: map[string]string{"cpu": "100m", "memory": "128Mi"},
			Limits:   map[string]string{"cpu": "1000m", "memory": "512Mi"},
		}
	}

	falseVal := false
	trueVal := true
	_ = falseVal

	dep := Deployment{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Metadata: ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    labels,
		},
		Spec: DeploymentSpec{
			Replicas: replicas,
			Selector: map[string]interface{}{
				"matchLabels": map[string]string{instanceLabel: name},
			},
			Strategy: map[string]interface{}{
				"type": "Recreate",
			},
			Template: PodTemplateSpec{
				Metadata: ObjectMeta{
					Labels: map[string]string{
						appLabel:      "ruptura",
						instanceLabel: name,
					},
				},
				Spec: PodSpec{
					ServiceAccountName: "ruptura-instance",
					SecurityContext: &PodSecurityContext{
						RunAsNonRoot: true,
						RunAsUser:    65532,
						FSGroup:      65532,
					},
					Containers: []Container{
						{
							Name:  "ruptura",
							Image: image,
							Args: []string{
								"--port=8080",
								"--storage=/var/lib/ruptura/data",
							},
							Ports: []ContainerPort{
								{Name: "http", ContainerPort: 8080, Protocol: "TCP"},
								{Name: "otlp", ContainerPort: 4317, Protocol: "TCP"},
							},
							Env: env,
							VolumeMounts: []VolumeMount{
								{Name: "data", MountPath: "/var/lib/ruptura/data"},
							},
							Resources: resources,
							SecurityContext: &ContainerSecurityContext{
								AllowPrivilegeEscalation: false,
								RunAsNonRoot:             trueVal,
							},
							LivenessProbe: &Probe{
								HTTPGet:             HTTPGetAction{Path: "/api/v2/health", Port: 8080},
								InitialDelaySeconds: 10,
								PeriodSeconds:       15,
								FailureThreshold:    3,
								TimeoutSeconds:      5,
							},
							ReadinessProbe: &Probe{
								HTTPGet:             HTTPGetAction{Path: "/api/v2/health", Port: 8080},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
								FailureThreshold:    3,
								TimeoutSeconds:      3,
							},
						},
					},
					Volumes: []Volume{
						{Name: "data", PersistentVolumeClaim: &PVCVolumeSource{ClaimName: name + "-data"}},
					},
				},
			},
		},
	}

	path := fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments/%s", ns, name)
	return c.apply(path, dep)
}

func reconcileService(c *k8sClient, ns, name string, labels map[string]string) error {
	svc := Service{
		APIVersion: "v1",
		Kind:       "Service",
		Metadata: ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    labels,
		},
		Spec: ServiceSpec{
			Selector: map[string]string{instanceLabel: name},
			Ports: []ServicePort{
				{Name: "http", Port: 8080, TargetPort: 8080, Protocol: "TCP"},
				{Name: "otlp", Port: 4317, TargetPort: 4317, Protocol: "TCP"},
			},
		},
	}
	path := fmt.Sprintf("/api/v1/namespaces/%s/services/%s", ns, name)
	return c.apply(path, svc)
}

// reconcileRoute creates an OpenShift Route that exposes the Ruptura HTTP API
// with edge TLS termination. Only called when running on OpenShift.
func reconcileRoute(c *k8sClient, ns, name string, labels map[string]string) error {
	route := Route{
		APIVersion: "route.openshift.io/v1",
		Kind:       "Route",
		Metadata: ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    labels,
		},
		Spec: RouteSpec{
			To: RouteTargetReference{
				Kind: "Service",
				Name: name,
			},
			Port: RoutePort{
				TargetPort: "http",
			},
			TLS: &RouteTLS{
				Termination: "edge",
			},
		},
	}
	path := fmt.Sprintf("/apis/route.openshift.io/v1/namespaces/%s/routes/%s", ns, name)
	return c.apply(path, route)
}

func updateStatus(c *k8sClient, inst RupturaInstance) error {
	ns := inst.Metadata.Namespace
	name := inst.Metadata.Name

	var dep DeploymentWithStatus
	depPath := fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments/%s", ns, name)
	_ = c.get(depPath, &dep)

	phase := "Running"
	if dep.Status.ReadyReplicas == 0 {
		phase = "Pending"
	}

	status := RupturaInstanceStatus{
		Phase:              phase,
		ReadyReplicas:      dep.Status.ReadyReplicas,
		AvailableReplicas:  dep.Status.AvailableReplicas,
		LastReconcileTime:  time.Now().UTC().Format(time.RFC3339),
		ObservedGeneration: inst.Metadata.Generation,
	}

	path := fmt.Sprintf("/apis/ruptura.io/v1alpha1/namespaces/%s/rupturainstances/%s", ns, name)
	return c.patchStatus(path, status)
}
