package main

import (
	"fmt"
	"os"
	"strings"
)

const (
	defaultConsolePluginImage = "ghcr.io/benfradjselim/ruptura-console-plugin:plugin-v8.7.8"
	// relatedConsolePluginImageEnv is set by OLM from the CSV, so a
	// disconnected mirror can rewrite the plugin image by digest.
	relatedConsolePluginImageEnv = "RELATED_IMAGE_RUPTURA_CONSOLE_PLUGIN"
	// consolePluginLabel selects plugin pods. Plugin pods never carry
	// instanceLabel, so the engine Service never routes to them.
	consolePluginLabel = "ruptura.io/console-plugin"
	// consolePluginPort is the HTTPS port the console reaches the plugin on.
	consolePluginPort = 9443
	// consolePluginObjectName names the cluster-wide ConsolePlugin. The CSV
	// lists it in console.openshift.io/plugins, so OLM offers to enable it.
	consolePluginObjectName = "ruptura"
	// consolePluginProxyAlias is the proxy the plugin reaches the engine through.
	consolePluginProxyAlias = "engine"
	// servingCertAnnotation asks OpenShift for a serving certificate Secret.
	servingCertAnnotation = "service.beta.openshift.io/serving-cert-secret-name"
	// consolePluginCertMount holds the serving certificate inside the pod.
	consolePluginCertMount = "/var/serving-cert"
	// consolePluginProxyBinary is the proxy helper shipped in the plugin image.
	consolePluginProxyBinary = "/usr/local/bin/ruptura-plugin-proxy"
	// consolePluginKeyMount holds the engine API key inside the pod. The key is
	// mounted as a file and never passed through the environment.
	consolePluginKeyMount = "/var/run/ruptura/api-key"
	// consolePluginTmpMount is the only writable path of the plugin container.
	consolePluginTmpMount = "/tmp"
	// consolePluginObjectPath is the cluster-wide ConsolePlugin API path.
	consolePluginObjectPath = "/apis/console.openshift.io/v1/consoleplugins/" + consolePluginObjectName
)

// consolePluginName is the name of the plugin Deployment, Service and
// serving certificate Secret.
// consolePluginSuffix ends the name of an instance's plugin Deployment and
// Service.
const consolePluginSuffix = "-console-plugin"

func consolePluginName(name string) string { return name + consolePluginSuffix }

// consolePluginCertName is the name of the serving certificate Secret that
// OpenShift writes for the plugin Service.
func consolePluginCertName(name string) string { return consolePluginName(name) + "-cert" }

// consolePluginEnabled reports whether the instance wants the OpenShift
// console plugin, false unless enabled.
func consolePluginEnabled(spec RupturaInstanceSpec) bool {
	return spec.Output.ConsolePlugin.Enabled != nil && *spec.Output.ConsolePlugin.Enabled
}

// consolePluginImage picks the plugin image: the instance spec, then the image
// OLM passes to the operator, then the built-in default.
func consolePluginImage(spec RupturaInstanceSpec) string {
	if spec.Output.ConsolePlugin.Image != "" {
		return spec.Output.ConsolePlugin.Image
	}
	if related := os.Getenv(relatedConsolePluginImageEnv); related != "" {
		return related
	}
	return defaultConsolePluginImage
}

func consolePluginPaths(ns, name string) (deployment, service string) {
	pn := consolePluginName(name)
	return fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments/%s", ns, pn),
		fmt.Sprintf("/api/v1/namespaces/%s/services/%s", ns, pn)
}

// reconcileConsolePlugin deploys the OpenShift console plugin, which serves
// the plugin assets and proxies the read endpoints of the engine with the API
// key added server-side, or removes it when disabled. Off OpenShift there is
// no console to extend, so nothing is done.
//
// The returned note, when not empty, explains why the plugin was not deployed
// and goes into the instance status message.
func reconcileConsolePlugin(c *k8sClient, ns, name string, isOCP bool, spec RupturaInstanceSpec, apiKeyRef string, labels map[string]string) (string, error) {
	if !isOCP {
		return "", nil
	}
	if !consolePluginEnabled(spec) {
		return "", deleteConsolePlugin(c, ns, name)
	}
	owner, err := consolePluginOwner(c)
	if err != nil {
		return "", err
	}
	if owner != "" && owner != ns+"/"+consolePluginName(name) {
		// Another instance provides the cluster's only Ruptura plugin. Leave it
		// alone and say so, rather than taking it over on every reconcile.
		if err := deleteConsolePluginWorkload(c, ns, name); err != nil {
			return "", err
		}
		return fmt.Sprintf("console plugin already provided by %s", strings.TrimSuffix(owner, consolePluginSuffix)), nil
	}
	depPath, svcPath := consolePluginPaths(ns, name)
	if err := c.apply(depPath, consolePluginDeployment(ns, name, consolePluginImage(spec), apiKeyRef, labels)); err != nil {
		return "", err
	}
	if err := c.apply(svcPath, consolePluginService(ns, name, labels)); err != nil {
		return "", err
	}
	return "", c.apply(consolePluginObjectPath, consolePluginObject(ns, name, labels))
}

// consolePluginOwner returns namespace/service of the Service backing the
// cluster's Ruptura ConsolePlugin, or "" when there is none.
func consolePluginOwner(c *k8sClient) (string, error) {
	var plugin consolePluginBackend
	err := c.get(consolePluginObjectPath, &plugin)
	if isNotFound(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	svc := plugin.Spec.Backend.Service
	if svc.Name == "" {
		return "", nil
	}
	return svc.Namespace + "/" + svc.Name, nil
}

// deleteConsolePluginWorkload removes this instance's plugin Deployment and
// Service; absent objects are not an error.
func deleteConsolePluginWorkload(c *k8sClient, ns, name string) error {
	depPath, svcPath := consolePluginPaths(ns, name)
	for _, path := range []string{depPath, svcPath} {
		if err := c.delete(path); err != nil {
			return err
		}
	}
	return nil
}

// deleteConsolePlugin removes the plugin objects; absent objects are not an
// error. The cluster-wide ConsolePlugin is removed only when it points to this
// instance, so an instance going away never unplugs another one.
func deleteConsolePlugin(c *k8sClient, ns, name string) error {
	if err := deleteConsolePluginWorkload(c, ns, name); err != nil {
		return err
	}
	owned, err := ownsConsolePlugin(c, ns, name)
	if err != nil || !owned {
		return err
	}
	return c.delete(consolePluginObjectPath)
}

// consolePluginBackend is the part of a ConsolePlugin that says which Service
// serves it.
type consolePluginBackend struct {
	Spec struct {
		Backend struct {
			Service struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"service"`
		} `json:"backend"`
	} `json:"spec"`
}

// ownsConsolePlugin reports whether the ConsolePlugin is backed by this
// instance. An absent ConsolePlugin is owned by nobody.
func ownsConsolePlugin(c *k8sClient, ns, name string) (bool, error) {
	var plugin consolePluginBackend
	err := c.get(consolePluginObjectPath, &plugin)
	if isNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	svc := plugin.Spec.Backend.Service
	return svc.Name == consolePluginName(name) && svc.Namespace == ns, nil
}

func consolePluginDeployment(ns, name, image, apiKeyRef string, labels map[string]string) Deployment {
	pn := consolePluginName(name)
	podLabels := map[string]string{appLabel: "ruptura-console-plugin", consolePluginLabel: name}
	probe := func(initial int32) *Probe {
		return &Probe{
			HTTPGet:             HTTPGetAction{Path: "/plugin-manifest.json", Port: consolePluginPort, Scheme: "HTTPS"},
			InitialDelaySeconds: initial,
			PeriodSeconds:       15,
			FailureThreshold:    3,
			TimeoutSeconds:      3,
		}
	}
	resources := ResourceRequirements{
		Requests: map[string]string{"cpu": "10m", "memory": "32Mi"},
		Limits:   map[string]string{"memory": "128Mi"},
	}
	restricted := &ContainerSecurityContext{
		AllowPrivilegeEscalation: false,
		ReadOnlyRootFilesystem:   true,
		RunAsNonRoot:             true,
		Capabilities:             &Capabilities{Drop: []string{"ALL"}},
	}
	return Deployment{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Metadata:   ObjectMeta{Name: pn, Namespace: ns, Labels: labels},
		Spec: DeploymentSpec{
			Replicas: 1,
			Selector: map[string]interface{}{"matchLabels": map[string]string{consolePluginLabel: name}},
			Template: PodTemplateSpec{
				Metadata: ObjectMeta{Labels: podLabels},
				Spec: PodSpec{
					// OpenShift assigns the UID from the namespace SCC range.
					SecurityContext: &PodSecurityContext{RunAsNonRoot: true, SeccompProfile: &SeccompProfile{Type: "RuntimeDefault"}},
					Containers: []Container{
						{
							// nginx serves the plugin assets and hands the read
							// endpoints to the proxy container on localhost. It
							// never sees the API key.
							Name:                     "console-plugin",
							Image:                    image,
							TerminationMessagePolicy: "FallbackToLogsOnError",
							Ports:                    []ContainerPort{{Name: "https", ContainerPort: consolePluginPort, Protocol: "TCP"}},
							VolumeMounts: []VolumeMount{
								{Name: "serving-cert", MountPath: consolePluginCertMount, ReadOnly: true},
								{Name: "tmp", MountPath: consolePluginTmpMount},
							},
							Resources:       resources,
							SecurityContext: restricted,
							LivenessProbe:   probe(10),
							ReadinessProbe:  probe(3),
						},
						{
							// The proxy checks each console user's access to the
							// namespaces in the answer and adds the API key, which
							// only this container mounts.
							Name:                     "proxy",
							Image:                    image,
							Command:                  []string{consolePluginProxyBinary},
							TerminationMessagePolicy: "FallbackToLogsOnError",
							Env: []EnvVar{
								{Name: "RUPTURA_BACKEND_URL", Value: fmt.Sprintf("http://%s:8080", name)},
								{Name: "RUPTURA_API_KEY_FILE", Value: consolePluginKeyMount + "/" + apiKeySecretKey},
							},
							VolumeMounts:    []VolumeMount{{Name: "api-key", MountPath: consolePluginKeyMount, ReadOnly: true}},
							Resources:       resources,
							SecurityContext: restricted,
						},
					},
					Volumes: []Volume{
						{Name: "serving-cert", Secret: &SecretVolumeSource{SecretName: consolePluginCertName(name)}},
						{Name: "api-key", Secret: &SecretVolumeSource{SecretName: apiKeyRef, Items: []KeyToPath{{Key: apiKeySecretKey, Path: apiKeySecretKey}}}},
						{Name: "tmp", EmptyDir: &EmptyDirVolumeSource{SizeLimit: "16Mi"}},
					},
				},
			},
		},
	}
}

func consolePluginService(ns, name string, labels map[string]string) Service {
	return Service{
		APIVersion: "v1",
		Kind:       "Service",
		Metadata: ObjectMeta{
			Name:        consolePluginName(name),
			Namespace:   ns,
			Labels:      labels,
			Annotations: map[string]string{servingCertAnnotation: consolePluginCertName(name)},
		},
		Spec: ServiceSpec{
			Selector: map[string]string{consolePluginLabel: name},
			Ports:    []ServicePort{{Name: "https", Port: consolePluginPort, TargetPort: consolePluginPort, Protocol: "TCP"}},
		},
	}
}

// consolePluginObject is the cluster-wide ConsolePlugin: the console loads the
// plugin assets from the plugin Service and proxies the engine API through the
// same Service under the alias, so the browser never holds the API key. The
// proxy forwards the console user's token (UserToken): the console answers
// only signed-in users, and the plugin checks the token against the API server
// before it swaps it for the API key.
func consolePluginObject(ns, name string, labels map[string]string) map[string]interface{} {
	service := map[string]interface{}{
		"name":      consolePluginName(name),
		"namespace": ns,
		"port":      consolePluginPort,
	}
	backend := map[string]interface{}{"name": service["name"], "namespace": ns, "port": consolePluginPort, "basePath": "/"}
	return map[string]interface{}{
		"apiVersion": "console.openshift.io/v1",
		"kind":       "ConsolePlugin",
		"metadata":   map[string]interface{}{"name": consolePluginObjectName, "labels": labels},
		"spec": map[string]interface{}{
			"displayName": "Ruptura",
			"backend":     map[string]interface{}{"type": "Service", "service": backend},
			"proxy": []interface{}{map[string]interface{}{
				"alias":         consolePluginProxyAlias,
				"authorization": "UserToken",
				"endpoint":      map[string]interface{}{"type": "Service", "service": service},
			}},
		},
	}
}
