package main

import (
	"fmt"
	"os"
)

const (
	defaultConsoleImage = "ghcr.io/benfradjselim/ruptura-ui:ui-v8.7.7"
	// relatedConsoleImageEnv is set by OLM from the CSV, so a disconnected
	// mirror can rewrite the console image by digest.
	relatedConsoleImageEnv = "RELATED_IMAGE_RUPTURA_UI"
	// consoleLabel selects console pods. Console pods never carry
	// instanceLabel, so the engine Service never routes to them.
	consoleLabel = "ruptura.io/console"
	consolePort  = 8080
)

// consoleName is the name of the console Deployment, Service and Route. It
// differs from ruptura-ui so a console applied by hand is never taken over.
func consoleName(name string) string { return name + "-console" }

// consoleEnabled reports whether the instance wants the console, true unless
// disabled.
func consoleEnabled(spec RupturaInstanceSpec) bool {
	return spec.Console.Enabled == nil || *spec.Console.Enabled
}

// consoleImage picks the console image: the instance spec, then the image OLM
// passes to the operator, then the built-in default.
func consoleImage(spec RupturaInstanceSpec) string {
	if spec.Console.Image != "" {
		return spec.Console.Image
	}
	if related := os.Getenv(relatedConsoleImageEnv); related != "" {
		return related
	}
	return defaultConsoleImage
}

func consolePaths(ns, name string) (deployment, service, route string) {
	cn := consoleName(name)
	return fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments/%s", ns, cn),
		fmt.Sprintf("/api/v1/namespaces/%s/services/%s", ns, cn),
		fmt.Sprintf("/apis/route.openshift.io/v1/namespaces/%s/routes/%s", ns, cn)
}

// reconcileConsole deploys the console, which serves the UI and proxies /api/
// to the engine Service, or removes it when disabled.
func reconcileConsole(c *k8sClient, ns, name string, isOCP bool, spec RupturaInstanceSpec, labels map[string]string) error {
	depPath, svcPath, routePath := consolePaths(ns, name)
	if !consoleEnabled(spec) {
		return deleteConsole(c, ns, name, isOCP)
	}
	if err := c.apply(depPath, consoleDeployment(ns, name, consoleImage(spec), isOCP, labels)); err != nil {
		return err
	}
	if err := c.apply(svcPath, consoleService(ns, name, labels)); err != nil {
		return err
	}
	if isOCP {
		return c.apply(routePath, consoleRoute(ns, name, labels))
	}
	return nil
}

// deleteConsole removes the console objects; absent objects are not an error.
func deleteConsole(c *k8sClient, ns, name string, isOCP bool) error {
	depPath, svcPath, routePath := consolePaths(ns, name)
	paths := []string{depPath, svcPath}
	if isOCP {
		paths = append(paths, routePath)
	}
	for _, path := range paths {
		if err := c.delete(path); err != nil {
			return err
		}
	}
	return nil
}

func consoleDeployment(ns, name, image string, isOCP bool, labels map[string]string) Deployment {
	podLabels := map[string]string{appLabel: "ruptura-console", consoleLabel: name}
	podSC := &PodSecurityContext{RunAsNonRoot: true, SeccompProfile: &SeccompProfile{Type: "RuntimeDefault"}}
	if !isOCP {
		podSC.RunAsUser = 101
	}
	probe := func(initial int32) *Probe {
		return &Probe{
			HTTPGet:             HTTPGetAction{Path: "/index.html", Port: consolePort},
			InitialDelaySeconds: initial,
			PeriodSeconds:       15,
			FailureThreshold:    3,
			TimeoutSeconds:      3,
		}
	}
	return Deployment{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Metadata:   ObjectMeta{Name: consoleName(name), Namespace: ns, Labels: labels},
		Spec: DeploymentSpec{
			Replicas: 1,
			Selector: map[string]interface{}{"matchLabels": map[string]string{consoleLabel: name}},
			Template: PodTemplateSpec{
				Metadata: ObjectMeta{Labels: podLabels},
				Spec: PodSpec{
					SecurityContext: podSC,
					Containers: []Container{{
						Name:                     "console",
						Image:                    image,
						TerminationMessagePolicy: "FallbackToLogsOnError",
						Ports:                    []ContainerPort{{Name: "http", ContainerPort: consolePort, Protocol: "TCP"}},
						Env:                      []EnvVar{{Name: "RUPTURA_BACKEND_URL", Value: fmt.Sprintf("http://%s:8080", name)}},
						Resources: ResourceRequirements{
							Requests: map[string]string{"cpu": "10m", "memory": "32Mi"},
							Limits:   map[string]string{"memory": "128Mi"},
						},
						SecurityContext: &ContainerSecurityContext{
							AllowPrivilegeEscalation: false,
							RunAsNonRoot:             true,
							Capabilities:             &Capabilities{Drop: []string{"ALL"}},
						},
						LivenessProbe:  probe(10),
						ReadinessProbe: probe(3),
					}},
				},
			},
		},
	}
}

func consoleService(ns, name string, labels map[string]string) Service {
	return Service{
		APIVersion: "v1",
		Kind:       "Service",
		Metadata:   ObjectMeta{Name: consoleName(name), Namespace: ns, Labels: labels},
		Spec: ServiceSpec{
			Selector: map[string]string{consoleLabel: name},
			Ports:    []ServicePort{{Name: "http", Port: consolePort, TargetPort: consolePort, Protocol: "TCP"}},
		},
	}
}

func consoleRoute(ns, name string, labels map[string]string) Route {
	return Route{
		APIVersion: "route.openshift.io/v1",
		Kind:       "Route",
		Metadata:   ObjectMeta{Name: consoleName(name), Namespace: ns, Labels: labels},
		Spec: RouteSpec{
			To:   RouteTargetReference{Kind: "Service", Name: consoleName(name)},
			Port: RoutePort{TargetPort: "http"},
			TLS:  &RouteTLS{Termination: "edge", InsecureEdgeTerminationPolicy: "Redirect"},
		},
	}
}
