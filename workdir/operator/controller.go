package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

const (
	defaultImage       = "docker.io/selimbf/ruptura:v8.5.1"
	defaultStorageSize = "10Gi"
	defaultEdition     = "community"
	appLabel           = "app.kubernetes.io/name"
	managedByLabel     = "app.kubernetes.io/managed-by"
	instanceLabel      = "app.kubernetes.io/instance"
	finalizer          = "ruptura.io/cleanup"
	// thanosURL is the OpenShift in-cluster Thanos querier endpoint.
	thanosURL = "https://thanos-querier.openshift-monitoring.svc:9091"
	// monitoringViewRole is the ClusterRole granting read access to cluster metrics.
	monitoringViewRole = "cluster-monitoring-view"
	// readerRole grants instances read access to workloads and events.
	readerRole = "ruptura-instance-reader"
	// instanceServiceAccount is the ServiceAccount used by instance pods.
	instanceServiceAccount = "ruptura-instance"
	// apiKeySecretKey is the Secret data key holding the engine API key.
	apiKeySecretKey = "api-key"
	// apiKeyBytes is the number of random bytes in a generated API key.
	apiKeyBytes = 32
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
		if err := c.delete(clusterRoleBindingPath(ns, name)); err != nil {
			return fmt.Errorf("delete ClusterRoleBinding: %w", err)
		}
	}
	if err := c.delete(readerBindingPath(ns, name)); err != nil {
		return fmt.Errorf("delete reader ClusterRoleBinding: %w", err)
	}
	if err := c.delete(fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", ns, generatedSecretName(name))); err != nil {
		return fmt.Errorf("delete Secret: %w", err)
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

	if err := validateSpec(inst.Spec); err != nil {
		log.Printf("invalid RupturaInstance %s/%s: %v", inst.Metadata.Namespace, inst.Metadata.Name, err)
		return writeInvalidStatus(c, inst, err)
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
	apiKeyRef := inst.Spec.APIKeyRef
	if apiKeyRef == "" {
		apiKeyRef = generatedSecretName(name)
		if err := ensureAPIKeySecret(c, ns, apiKeyRef, labels); err != nil {
			return fmt.Errorf("API key Secret: %w", err)
		}
	}
	if err := reconcileBinding(c, readerBindingPath(ns, name), readerBindingName(ns, name), readerRole, ns, labels); err != nil {
		return fmt.Errorf("reader ClusterRoleBinding: %w", err)
	}
	if isOCP {
		if err := reconcileMonitoringBinding(c, ns, name, labels); err != nil {
			return fmt.Errorf("ClusterRoleBinding: %w", err)
		}
	}
	if err := reconcileDeployment(c, ns, name, image, edition, apiKeyRef, replicas, isOCP, inst.Spec, labels); err != nil {
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

func reconcileDeployment(c *k8sClient, ns, name, image, edition, apiKeyRef string, replicas int32, isOCP bool, spec RupturaInstanceSpec, labels map[string]string) error {
	env := []EnvVar{
		{Name: "RUPTURA_EDITION", Value: edition},
	}
	env = append(env, EnvVar{
		Name: "RUPTURA_API_KEY",
		ValueFrom: &EnvVarSource{
			SecretKeyRef: &SecretKeySelector{
				Name:     apiKeyRef,
				Key:      apiKeySecretKey,
				Optional: true,
			},
		},
	})
	if isOCP {
		env = append(env, EnvVar{Name: "RUPTURA_THANOS_URL", Value: thanosURL})
	}
	env = append(env, configEnv(spec)...)
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

	trueVal := true

	// OpenShift assigns UID and fsGroup from the namespace SCC range and
	// rejects fixed values, so they are set on plain Kubernetes only.
	podSC := &PodSecurityContext{RunAsNonRoot: true}
	if !isOCP {
		podSC.RunAsUser = 65532
		podSC.FSGroup = 65532
	}

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
					ServiceAccountName: instanceServiceAccount,
					SecurityContext:    podSC,
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
				Termination:                   "edge",
				InsecureEdgeTerminationPolicy: "Redirect",
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

// writeInvalidStatus reports a spec that was refused. Nothing is deployed.
func writeInvalidStatus(c *k8sClient, inst RupturaInstance, problem error) error {
	status := RupturaInstanceStatus{
		Phase:              "Failed",
		Message:            problem.Error(),
		LastReconcileTime:  time.Now().UTC().Format(time.RFC3339),
		ObservedGeneration: inst.Metadata.Generation,
	}
	path := fmt.Sprintf("/apis/ruptura.io/v1alpha1/namespaces/%s/rupturainstances/%s", inst.Metadata.Namespace, inst.Metadata.Name)
	return c.patchStatus(path, status)
}

// selectorString renders labels in a stable order, so a Deployment is not
// rewritten when nothing changed.
func selectorString(labels map[string]string) string {
	pairs := make([]string, 0, len(labels))
	for key, value := range labels {
		pairs = append(pairs, key+"="+value)
	}
	sort.Strings(pairs)
	return strings.Join(pairs, ",")
}

func alertmanagerURLs(channels []AlertChannel) string {
	urls := make([]string, 0, len(channels))
	for _, channel := range channels {
		if channel.Alertmanager != nil {
			urls = append(urls, channel.Alertmanager.URL)
		}
	}
	return strings.Join(urls, ",")
}

// configEnv passes the declared configuration to the engine.
func configEnv(spec RupturaInstanceSpec) []EnvVar {
	values := []EnvVar{
		{Name: "RUPTURA_LEVEL_SCOPE", Value: spec.Level.Scope},
		{Name: "RUPTURA_LEVEL_SELECTOR", Value: selectorString(spec.Level.Selector.MatchLabels)},
		{Name: "RUPTURA_INPUT_PROFILE", Value: spec.Input.Profile},
		{Name: "RUPTURA_SENSITIVITY", Value: spec.Analysis.Sensitivity},
		{Name: "RUPTURA_HORIZON", Value: spec.Analysis.Horizon},
		{Name: "LOG_LEVEL", Value: spec.Logging.Level},
		{Name: "RUPTURA_ALERTMANAGER_URLS", Value: alertmanagerURLs(spec.Alerting.Channels)},
	}
	env := make([]EnvVar, 0, len(values))
	for _, value := range values {
		if value.Value != "" {
			env = append(env, value)
		}
	}
	return env
}

// generatedSecretName returns the name of the operator-generated API key Secret.
func generatedSecretName(name string) string { return name + "-api-key" }

// generateAPIKey returns a hex-encoded random API key.
func generateAPIKey() (string, error) {
	b := make([]byte, apiKeyBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ensureAPIKeySecret creates the API key Secret when it does not exist yet.
// An existing Secret is never overwritten, so the key is stable across
// reconciles. The key value is never logged.
func ensureAPIKeySecret(c *k8sClient, ns, secretName string, labels map[string]string) error {
	path := fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", ns, secretName)
	var existing map[string]interface{}
	err := c.get(path, &existing)
	if err == nil {
		return nil
	}
	if !isNotFound(err) {
		return err
	}
	key, err := generateAPIKey()
	if err != nil {
		return fmt.Errorf("generate API key: %w", err)
	}
	secret := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"type":       "Opaque",
		"metadata": map[string]interface{}{
			"name":      secretName,
			"namespace": ns,
			"labels":    labels,
		},
		"stringData": map[string]string{apiKeySecretKey: key},
	}
	return c.apply(path, secret)
}

// clusterRoleBindingPath returns the API path of the per-instance monitoring binding.
func clusterRoleBindingPath(ns, name string) string {
	return fmt.Sprintf("/apis/rbac.authorization.k8s.io/v1/clusterrolebindings/ruptura-%s-%s-monitoring-view", ns, name)
}

// reconcileMonitoringBinding binds the instance ServiceAccount to
// cluster-monitoring-view so the engine can query Thanos on OpenShift.
func reconcileMonitoringBinding(c *k8sClient, ns, name string, labels map[string]string) error {
	return reconcileBinding(c, clusterRoleBindingPath(ns, name), fmt.Sprintf("ruptura-%s-%s-monitoring-view", ns, name), monitoringViewRole, ns, labels)
}

func readerBindingName(ns, name string) string { return fmt.Sprintf("ruptura-%s-%s-reader", ns, name) }

func readerBindingPath(ns, name string) string {
	return "/apis/rbac.authorization.k8s.io/v1/clusterrolebindings/" + readerBindingName(ns, name)
}

// reconcileBinding binds the instance ServiceAccount to a cluster role.
func reconcileBinding(c *k8sClient, path, bindingName, role, ns string, labels map[string]string) error {
	crb := map[string]interface{}{
		"apiVersion": "rbac.authorization.k8s.io/v1",
		"kind":       "ClusterRoleBinding",
		"metadata": map[string]interface{}{
			"name":   bindingName,
			"labels": labels,
		},
		"roleRef": map[string]interface{}{
			"apiGroup": "rbac.authorization.k8s.io",
			"kind":     "ClusterRole",
			"name":     role,
		},
		"subjects": []interface{}{map[string]interface{}{
			"kind":      "ServiceAccount",
			"name":      instanceServiceAccount,
			"namespace": ns,
		}},
	}
	return c.apply(path, crb)
}
