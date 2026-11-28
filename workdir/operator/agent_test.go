package main

import (
	"context"
	"net/http"
	"testing"
)

const (
	testAgentDSPath   = "/apis/apps/v1/namespaces/ruptura-system/daemonsets/ri-agent"
	testAgentSAPath   = "/api/v1/namespaces/ruptura-system/serviceaccounts/ruptura-agent"
	testAgentCRBPath  = "/apis/rbac.authorization.k8s.io/v1/clusterrolebindings/ruptura-ruptura-system-ri-agent-reader"
	testAgentSCCPath  = "/apis/rbac.authorization.k8s.io/v1/namespaces/ruptura-system/rolebindings/ri-agent-scc"
	testAgentProfile  = "deep"
	testAgentOTLPPort = "http://ri.ruptura-system:4318"
)

func TestReconcile_AgentWithDeepProfile(t *testing.T) {
	c, rec := mcpClient(t)
	inst := makeInstance("ri", testNS)
	inst.Spec.Input.Profile = testAgentProfile
	inst.Spec.Image = "registry.example/ruptura:v9"
	if err := reconcile(context.Background(), c, inst, true); err != nil {
		t.Fatal(err)
	}
	ds, ok := rec.get(http.MethodPatch, testAgentDSPath)
	if !ok {
		t.Fatal("agent DaemonSet not applied")
	}
	pod := ds["spec"].(map[string]interface{})["template"].(map[string]interface{})["spec"].(map[string]interface{})
	if pod["serviceAccountName"] != agentServiceAccount {
		t.Fatalf("service account = %v", pod["serviceAccountName"])
	}
	container := pod["containers"].([]interface{})[0].(map[string]interface{})
	if container["image"] != "registry.example/ruptura:v9" || container["command"].([]interface{})[0] != "/ruptura-agent" {
		t.Fatalf("container = %v", container)
	}
	env := map[string]interface{}{}
	for _, e := range container["env"].([]interface{}) {
		m := e.(map[string]interface{})
		env[m["name"].(string)] = m
	}
	if env["RUPTURA_OTLP_ENDPOINT"].(map[string]interface{})["value"] != testAgentOTLPPort {
		t.Fatalf("endpoint = %v", env["RUPTURA_OTLP_ENDPOINT"])
	}
	if env["NODE_NAME"].(map[string]interface{})["valueFrom"] == nil {
		t.Fatal("NODE_NAME not taken from the pod's node")
	}
	volumes := map[string]bool{}
	for _, v := range pod["volumes"].([]interface{}) {
		volumes[v.(map[string]interface{})["name"].(string)] = true
	}
	for _, m := range container["volumeMounts"].([]interface{}) {
		mount := m.(map[string]interface{})
		if mount["readOnly"] != true {
			t.Fatalf("host mount not read-only: %v", m)
		}
		if !volumes[mount["name"].(string)] {
			t.Fatalf("mount %v has no volume in %v", mount["name"], pod["volumes"])
		}
	}
	for _, path := range []string{testAgentSAPath, testAgentCRBPath, testAgentSCCPath} {
		if _, ok := rec.get(http.MethodPatch, path); !ok {
			t.Fatalf("%s not applied", path)
		}
	}
}

func TestReconcile_AgentRemovedWithoutDeepProfile(t *testing.T) {
	c, rec := mcpClient(t)
	if err := reconcile(context.Background(), c, makeInstance("ri", testNS), false); err != nil {
		t.Fatal(err)
	}
	if _, ok := rec.get(http.MethodPatch, testAgentDSPath); ok {
		t.Fatal("agent applied without the deep profile")
	}
	for _, path := range []string{testAgentDSPath, testAgentCRBPath} {
		if _, ok := rec.get(http.MethodDelete, path); !ok {
			t.Fatalf("%s not deleted", path)
		}
	}
}
