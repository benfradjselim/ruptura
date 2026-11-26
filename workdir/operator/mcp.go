package main

import "fmt"

const (
	// mcpLabel selects MCP server pods. They never carry instanceLabel, so
	// the engine Service never routes to them.
	mcpLabel = "ruptura.io/mcp"
	mcpPort  = 8090
	// mcpTokenKey is the Secret data key holding the bearer token MCP
	// clients send.
	mcpTokenKey = "token"
)

// mcpName is the name of the MCP server Deployment and Service.
func mcpName(name string) string { return name + "-mcp" }

// mcpTokenSecret is the Secret holding the token MCP clients send.
func mcpTokenSecret(name string) string { return name + "-mcp-token" }

// mcpEnabled reports whether the instance wants the MCP server, false unless
// enabled.
func mcpEnabled(spec RupturaInstanceSpec) bool {
	return spec.Output.MCP.Enabled != nil && *spec.Output.MCP.Enabled
}

func mcpPaths(ns, name string) (deployment, service, secret string) {
	mn := mcpName(name)
	return fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments/%s", ns, mn),
		fmt.Sprintf("/api/v1/namespaces/%s/services/%s", ns, mn),
		fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", ns, mcpTokenSecret(name))
}

// reconcileMCP deploys the MCP server from the engine image, with a generated
// client token, or removes it when disabled. It has no Route: clients reach it
// inside the cluster, through a gateway or a route the user adds with TLS.
func reconcileMCP(c *k8sClient, ns, name, image, apiKeyRef string, spec RupturaInstanceSpec, labels map[string]string) error {
	if !mcpEnabled(spec) {
		return deleteMCP(c, ns, name)
	}
	if err := ensureGeneratedSecret(c, ns, mcpTokenSecret(name), mcpTokenKey, labels); err != nil {
		return fmt.Errorf("token Secret: %w", err)
	}
	depPath, svcPath, _ := mcpPaths(ns, name)
	if err := c.apply(depPath, mcpDeployment(ns, name, image, apiKeyRef, labels)); err != nil {
		return err
	}
	return c.apply(svcPath, mcpService(ns, name, labels))
}

// deleteMCP removes the MCP server objects, token included; absent objects
// are not an error.
func deleteMCP(c *k8sClient, ns, name string) error {
	depPath, svcPath, secretPath := mcpPaths(ns, name)
	for _, path := range []string{depPath, svcPath, secretPath} {
		if err := c.delete(path); err != nil {
			return err
		}
	}
	return nil
}

func mcpDeployment(ns, name, image, apiKeyRef string, labels map[string]string) Deployment {
	secretEnv := func(env, secret, key string) EnvVar {
		return EnvVar{Name: env, ValueFrom: &EnvVarSource{SecretKeyRef: &SecretKeySelector{Name: secret, Key: key}}}
	}
	probe := func(initial int32) *Probe {
		return &Probe{
			HTTPGet:             HTTPGetAction{Path: "/healthz", Port: mcpPort},
			InitialDelaySeconds: initial,
			PeriodSeconds:       15,
			FailureThreshold:    3,
			TimeoutSeconds:      3,
		}
	}
	return Deployment{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Metadata:   ObjectMeta{Name: mcpName(name), Namespace: ns, Labels: labels},
		Spec: DeploymentSpec{
			Replicas: 1,
			Selector: map[string]interface{}{"matchLabels": map[string]string{mcpLabel: name}},
			Template: PodTemplateSpec{
				Metadata: ObjectMeta{Labels: map[string]string{appLabel: "ruptura-mcp", mcpLabel: name}},
				Spec: PodSpec{
					SecurityContext: &PodSecurityContext{RunAsNonRoot: true, SeccompProfile: &SeccompProfile{Type: "RuntimeDefault"}},
					Containers: []Container{{
						Name:                     "mcp",
						Image:                    image,
						Command:                  []string{"/ruptura-mcp", fmt.Sprintf("--listen=:%d", mcpPort), fmt.Sprintf("--engine=http://%s:8080", name)},
						TerminationMessagePolicy: "FallbackToLogsOnError",
						Ports:                    []ContainerPort{{Name: "mcp", ContainerPort: mcpPort, Protocol: "TCP"}},
						Env: []EnvVar{
							secretEnv("RUPTURA_API_KEY", apiKeyRef, apiKeySecretKey),
							secretEnv("RUPTURA_MCP_TOKEN", mcpTokenSecret(name), mcpTokenKey),
						},
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

func mcpService(ns, name string, labels map[string]string) Service {
	return Service{
		APIVersion: "v1",
		Kind:       "Service",
		Metadata:   ObjectMeta{Name: mcpName(name), Namespace: ns, Labels: labels},
		Spec: ServiceSpec{
			Selector: map[string]string{mcpLabel: name},
			Ports:    []ServicePort{{Name: "mcp", Port: mcpPort, TargetPort: mcpPort, Protocol: "TCP"}},
		},
	}
}
