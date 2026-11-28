package main

import "fmt"

const (
	// agentServiceAccount runs the node agent. It is apart from the engine's
	// account so that only the agent may use the host-mount SCC.
	agentServiceAccount = "ruptura-agent"
	// agentSCCRole lets the agent use the OpenShift hostmount-anyuid SCC:
	// read-only host directories and root, needed for the root-only OVN audit
	// log, and no host namespaces.
	agentSCCRole = "ruptura-agent-scc"
	agentLabel   = "ruptura.io/agent"
	// otlpHTTPPort is the engine's OTLP/HTTP receiver.
	otlpHTTPPort = 4318
)

// agentEnabled reports whether the instance asks for the node agent.
func agentEnabled(spec RupturaInstanceSpec) bool { return spec.Input.Profile == "deep" }

func agentName(name string) string { return name + "-agent" }

func agentPaths(ns, name string) (daemonSet, serviceAccount, readerBinding, sccBinding string) {
	return fmt.Sprintf("/apis/apps/v1/namespaces/%s/daemonsets/%s", ns, agentName(name)),
		fmt.Sprintf("/api/v1/namespaces/%s/serviceaccounts/%s", ns, agentServiceAccount),
		"/apis/rbac.authorization.k8s.io/v1/clusterrolebindings/" + agentReaderBindingName(ns, name),
		fmt.Sprintf("/apis/rbac.authorization.k8s.io/v1/namespaces/%s/rolebindings/%s-scc", ns, agentName(name))
}

func agentReaderBindingName(ns, name string) string {
	return fmt.Sprintf("ruptura-%s-%s-agent-reader", ns, name)
}

// reconcileAgent deploys the node agent on every node when the instance uses
// the deep profile, or removes it otherwise.
func reconcileAgent(c *k8sClient, ns, name, image string, isOCP bool, spec RupturaInstanceSpec, labels map[string]string) error {
	if !agentEnabled(spec) {
		return deleteAgent(c, ns, name)
	}
	dsPath, saPath, readerPath, sccPath := agentPaths(ns, name)
	sa := map[string]interface{}{"apiVersion": "v1", "kind": "ServiceAccount",
		"metadata": map[string]interface{}{"name": agentServiceAccount, "namespace": ns, "labels": labels}}
	if err := c.apply(saPath, sa); err != nil {
		return fmt.Errorf("ServiceAccount: %w", err)
	}
	if err := c.apply(readerPath, agentBinding("ClusterRoleBinding", agentReaderBindingName(ns, name), "", readerRole, ns, labels)); err != nil {
		return fmt.Errorf("reader ClusterRoleBinding: %w", err)
	}
	if isOCP {
		if err := c.apply(sccPath, agentBinding("RoleBinding", agentName(name)+"-scc", ns, agentSCCRole, ns, labels)); err != nil {
			return fmt.Errorf("SCC RoleBinding: %w", err)
		}
	}
	return c.apply(dsPath, agentDaemonSet(ns, name, image, labels))
}

// deleteAgent removes the agent objects; absent objects are not an error.
// The ServiceAccount is shared by the instances of the namespace and stays.
func deleteAgent(c *k8sClient, ns, name string) error {
	dsPath, _, readerPath, sccPath := agentPaths(ns, name)
	for _, path := range []string{dsPath, readerPath, sccPath} {
		if err := c.delete(path); err != nil {
			return err
		}
	}
	return nil
}

func agentBinding(kind, bindingName, bindingNS, role, ns string, labels map[string]string) map[string]interface{} {
	meta := map[string]interface{}{"name": bindingName, "labels": labels}
	if bindingNS != "" {
		meta["namespace"] = bindingNS
	}
	return map[string]interface{}{
		"apiVersion": "rbac.authorization.k8s.io/v1",
		"kind":       kind,
		"metadata":   meta,
		"roleRef":    map[string]interface{}{"apiGroup": "rbac.authorization.k8s.io", "kind": "ClusterRole", "name": role},
		"subjects":   []interface{}{map[string]interface{}{"kind": "ServiceAccount", "name": agentServiceAccount, "namespace": ns}},
	}
}

func agentDaemonSet(ns, name, image string, labels map[string]string) map[string]interface{} {
	hostDirs := []struct{ volume, host, mount string }{
		{"proc", "/proc", "/host/proc"},
		{"cgroup", "/sys/fs/cgroup", "/host/sys/fs/cgroup"},
		{"ovn-log", "/var/log/ovn", "/host/var/log/ovn"},
	}
	var volumes, mounts []interface{}
	for _, d := range hostDirs {
		volumes = append(volumes, map[string]interface{}{"name": d.volume,
			"hostPath": map[string]interface{}{"path": d.host, "type": "DirectoryOrCreate"}})
		mounts = append(mounts, map[string]interface{}{"name": d.volume, "mountPath": d.mount, "readOnly": true})
	}
	volumes[0].(map[string]interface{})["hostPath"].(map[string]interface{})["type"] = "Directory"
	selector := map[string]string{agentLabel: name}
	return map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "DaemonSet",
		"metadata":   map[string]interface{}{"name": agentName(name), "namespace": ns, "labels": labels},
		"spec": map[string]interface{}{
			"selector": map[string]interface{}{"matchLabels": selector},
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{"labels": map[string]string{appLabel: "ruptura-agent", agentLabel: name}},
				"spec": map[string]interface{}{
					"serviceAccountName": agentServiceAccount,
					"volumes":            volumes,
					// Every node, control plane and tainted nodes included.
					"tolerations": []interface{}{map[string]interface{}{"operator": "Exists"}},
					"containers": []interface{}{map[string]interface{}{
						"name":    "agent",
						"image":   image,
						"command": []string{"/ruptura-agent"},
						"env": []interface{}{
							map[string]interface{}{"name": "NODE_NAME", "valueFrom": map[string]interface{}{"fieldRef": map[string]interface{}{"fieldPath": "spec.nodeName"}}},
							map[string]interface{}{"name": "RUPTURA_OTLP_ENDPOINT", "value": fmt.Sprintf("http://%s.%s:%d", name, ns, otlpHTTPPort)},
							map[string]interface{}{"name": "RUPTURA_HOST_ROOT", "value": "/host"},
						},
						"volumeMounts":             mounts,
						"terminationMessagePolicy": "FallbackToLogsOnError",
						"resources": map[string]interface{}{
							"requests": map[string]string{"cpu": "5m", "memory": "24Mi"},
							"limits":   map[string]string{"memory": "96Mi"},
						},
						"securityContext": map[string]interface{}{
							// Root reads the root-only OVN audit log; nothing else.
							"runAsUser":                0,
							"runAsNonRoot":             false,
							"allowPrivilegeEscalation": false,
							"readOnlyRootFilesystem":   true,
							// No seccompProfile: hostmount-anyuid allows none.
							"capabilities": map[string]interface{}{"drop": []string{"ALL"}},
						},
					}},
				},
			},
		},
	}
}
