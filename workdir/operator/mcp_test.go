package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"
)

const (
	testMCPDepPath   = "/apis/apps/v1/namespaces/ruptura-system/deployments/ri-mcp"
	testMCPSvcPath   = "/api/v1/namespaces/ruptura-system/services/ri-mcp"
	testMCPTokenPath = "/api/v1/namespaces/ruptura-system/secrets/ri-mcp-token"
)

func TestMCPEnabled(t *testing.T) {
	off, on := false, true
	tests := []struct {
		name    string
		enabled *bool
		want    bool
	}{
		{"off by default", nil, false},
		{"enabled", &on, true},
		{"explicitly off", &off, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mcpEnabled(RupturaInstanceSpec{Output: OutputConfig{MCP: MCPOutput{Enabled: tt.enabled}}}); got != tt.want {
				t.Fatalf("mcpEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

// mcpClient records every request; GET on the MCP token Secret returns 404
// so the operator generates it.
func mcpClient(t *testing.T) (*k8sClient, *recorder) {
	t.Helper()
	rec := &recorder{bodies: map[string][]byte{}}
	c := fakeServer(t, map[string]http.HandlerFunc{
		"/": func(w http.ResponseWriter, r *http.Request) {
			b, _ := ioutil.ReadAll(r.Body)
			rec.mu.Lock()
			rec.bodies[r.Method+" "+r.URL.Path] = b
			rec.mu.Unlock()
			if r.Method == http.MethodGet && r.URL.Path == testMCPTokenPath {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		},
	})
	return c, rec
}

func TestReconcile_MCPEnabled(t *testing.T) {
	c, rec := mcpClient(t)
	inst := makeInstance("ri", testNS)
	on := true
	inst.Spec.Output.MCP.Enabled = &on
	inst.Spec.Image = "registry.example/ruptura:v9"
	if err := reconcile(context.Background(), c, inst, true); err != nil {
		t.Fatal(err)
	}

	token, ok := rec.get(http.MethodPatch, testMCPTokenPath)
	if !ok {
		t.Fatal("token Secret not created")
	}
	if v, _ := token["stringData"].(map[string]interface{})["token"].(string); len(v) < 32 {
		t.Fatalf("token too short: %d", len(v))
	}

	raw, ok := rec.get(http.MethodPatch, testMCPDepPath)
	if !ok {
		t.Fatal("MCP Deployment not applied")
	}
	var dep Deployment
	b, _ := json.Marshal(raw)
	if err := json.Unmarshal(b, &dep); err != nil {
		t.Fatal(err)
	}
	container := dep.Spec.Template.Spec.Containers[0]
	if container.Image != "registry.example/ruptura:v9" {
		t.Fatalf("image = %q, want the engine image", container.Image)
	}
	wantCmd := []string{"/ruptura-mcp", "--listen=:8090", "--engine=http://ri:8080"}
	if len(container.Command) != len(wantCmd) {
		t.Fatalf("command = %v", container.Command)
	}
	for i := range wantCmd {
		if container.Command[i] != wantCmd[i] {
			t.Fatalf("command = %v, want %v", container.Command, wantCmd)
		}
	}
	refs := map[string]SecretKeySelector{}
	for _, env := range container.Env {
		if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
			refs[env.Name] = *env.ValueFrom.SecretKeyRef
		}
	}
	if refs["RUPTURA_API_KEY"] != (SecretKeySelector{Name: "ri-api-key", Key: "api-key"}) {
		t.Fatalf("API key ref = %+v", refs["RUPTURA_API_KEY"])
	}
	if refs["RUPTURA_MCP_TOKEN"] != (SecretKeySelector{Name: "ri-mcp-token", Key: "token"}) {
		t.Fatalf("token ref = %+v", refs["RUPTURA_MCP_TOKEN"])
	}
	if _, ok := dep.Spec.Template.Metadata.Labels[instanceLabel]; ok {
		t.Fatal("MCP pods carry the engine instance label, so the engine Service would route to them")
	}
	if _, ok := rec.get(http.MethodPatch, testMCPSvcPath); !ok {
		t.Fatal("MCP Service not applied")
	}
}

func TestReconcile_MCPOffDeletesItsObjects(t *testing.T) {
	c, rec := mcpClient(t)
	if err := reconcile(context.Background(), c, makeInstance("ri", testNS), true); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{testMCPDepPath, testMCPSvcPath, testMCPTokenPath} {
		if _, ok := rec.get(http.MethodDelete, path); !ok {
			t.Errorf("%s not deleted while the MCP server is off", path)
		}
		if _, ok := rec.get(http.MethodPatch, path); ok {
			t.Errorf("%s applied while the MCP server is off", path)
		}
	}
}

func TestCleanup_DeletesMCP(t *testing.T) {
	inst := makeInstance("ri", testNS)
	inst.Metadata.Finalizers = []string{finalizer}
	c, rec := recordingClient(t, false)
	if err := cleanup(c, inst, true); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{testMCPDepPath, testMCPSvcPath, testMCPTokenPath} {
		if _, ok := rec.get(http.MethodDelete, path); !ok {
			t.Errorf("%s not deleted with the instance", path)
		}
	}
}
