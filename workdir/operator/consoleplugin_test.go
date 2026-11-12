package main

import (
	"context"
	"net/http"
	"sync"
	"testing"
)

const (
	testPluginDepPath = "/apis/apps/v1/namespaces/ruptura-system/deployments/ri-console-plugin"
	testPluginSvcPath = "/api/v1/namespaces/ruptura-system/services/ri-console-plugin"
)

// pluginServer records the methods seen per path and serves a canned
// ConsolePlugin body, so a test can say who backs the cluster-wide plugin.
type pluginServer struct {
	mu      sync.Mutex
	methods map[string][]string
}

func (s *pluginServer) seen(method, path string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.methods[path] {
		if m == method {
			return true
		}
	}
	return false
}

// pluginClient returns a client whose server answers GET on the ConsolePlugin
// with pluginBody, or 404 when it is empty.
func pluginClient(t *testing.T, pluginBody string) (*k8sClient, *pluginServer) {
	t.Helper()
	s := &pluginServer{methods: map[string][]string{}}
	c := fakeServer(t, map[string]http.HandlerFunc{
		"/": func(w http.ResponseWriter, r *http.Request) {
			s.mu.Lock()
			s.methods[r.URL.Path] = append(s.methods[r.URL.Path], r.Method)
			s.mu.Unlock()
			if r.Method == http.MethodGet && r.URL.Path == consolePluginObjectPath {
				if pluginBody == "" {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{}`))
					return
				}
				_, _ = w.Write([]byte(pluginBody))
				return
			}
			_, _ = w.Write([]byte(`{}`))
		},
	})
	return c, s
}

func pluginSpec(enabled *bool, image string) RupturaInstanceSpec {
	return RupturaInstanceSpec{Output: OutputConfig{ConsolePlugin: ConsolePluginOutput{Enabled: enabled, Image: image}}}
}

const ownPluginBody = `{"spec":{"backend":{"service":{"name":"ri-console-plugin","namespace":"ruptura-system"}}}}`
const otherPluginBody = `{"spec":{"backend":{"service":{"name":"other-console-plugin","namespace":"elsewhere"}}}}`

func TestConsolePluginEnabled(t *testing.T) {
	off, on := false, true
	tests := []struct {
		name    string
		enabled *bool
		want    bool
	}{
		{"off by default", nil, false},
		{"explicitly on", &on, true},
		{"disabled", &off, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := consolePluginEnabled(pluginSpec(tt.enabled, "")); got != tt.want {
				t.Fatalf("consolePluginEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConsolePluginImage(t *testing.T) {
	tests := []struct {
		name, spec, related, want string
	}{
		{"built-in default", "", "", defaultConsolePluginImage},
		{"image passed by OLM", "", "mirror.example/plugin@sha256:abc", "mirror.example/plugin@sha256:abc"},
		{"instance spec wins", "registry.example/plugin:1", "mirror.example/plugin@sha256:abc", "registry.example/plugin:1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(relatedConsolePluginImageEnv, tt.related)
			if got := consolePluginImage(pluginSpec(nil, tt.spec)); got != tt.want {
				t.Fatalf("consolePluginImage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReconcileConsolePlugin(t *testing.T) {
	on, off := true, false
	tests := []struct {
		name       string
		isOCP      bool
		enabled    *bool
		pluginBody string
		// expected methods per path, "" means the path is never touched
		deployment, service, plugin string
	}{
		{"enabled on OpenShift applies all three", true, &on, "", http.MethodPatch, http.MethodPatch, http.MethodPatch},
		{"disabled removes its own plugin", true, &off, ownPluginBody, http.MethodDelete, http.MethodDelete, http.MethodDelete},
		{"disabled keeps another instance's plugin", true, &off, otherPluginBody, http.MethodDelete, http.MethodDelete, ""},
		{"disabled with no plugin deletes nothing cluster-wide", true, nil, "", http.MethodDelete, http.MethodDelete, ""},
		{"nothing off OpenShift", false, &on, "", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, s := pluginClient(t, tt.pluginBody)
			if err := reconcileConsolePlugin(c, testNS, "ri", tt.isOCP, pluginSpec(tt.enabled, ""), "ri-api-key", nil); err != nil {
				t.Fatalf("reconcileConsolePlugin: %v", err)
			}
			checks := []struct{ path, method string }{
				{testPluginDepPath, tt.deployment},
				{testPluginSvcPath, tt.service},
				{consolePluginObjectPath, tt.plugin},
			}
			for _, check := range checks {
				for _, method := range []string{http.MethodPatch, http.MethodDelete} {
					if got, want := s.seen(method, check.path), method == check.method; got != want {
						t.Errorf("%s %s seen = %v, want %v", method, check.path, got, want)
					}
				}
			}
		})
	}
}

func TestReconcile_ConsolePluginUsesTheInstanceAPIKey(t *testing.T) {
	on := true
	c, rec := recordingClient(t, false)
	inst := makeInstance("ri", testNS)
	inst.Metadata.Finalizers = []string{finalizer}
	inst.Spec.APIKeyRef = "my-key"
	inst.Spec.Output.ConsolePlugin.Enabled = &on
	if err := reconcile(context.Background(), c, inst, true); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	dep, ok := rec.get(http.MethodPatch, testPluginDepPath)
	if !ok {
		t.Fatal("plugin deployment was not applied")
	}
	volumes := dep["spec"].(map[string]interface{})["template"].(map[string]interface{})["spec"].(map[string]interface{})["volumes"].([]interface{})
	var keySecret string
	for _, v := range volumes {
		vol := v.(map[string]interface{})
		if vol["name"] == "api-key" {
			keySecret = vol["secret"].(map[string]interface{})["secretName"].(string)
		}
	}
	if keySecret != "my-key" {
		t.Fatalf("api-key volume secret = %q, want my-key", keySecret)
	}
	if _, ok := rec.get(http.MethodPatch, consolePluginObjectPath); !ok {
		t.Fatal("ConsolePlugin was not applied")
	}
}

func TestReconcile_ConsolePluginOffByDefault(t *testing.T) {
	c, rec := recordingClient(t, false)
	inst := makeInstance("ri", testNS)
	inst.Metadata.Finalizers = []string{finalizer}
	if err := reconcile(context.Background(), c, inst, true); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if _, ok := rec.get(http.MethodPatch, testPluginDepPath); ok {
		t.Fatal("plugin deployment applied although the plugin is not enabled")
	}
	if _, ok := rec.get(http.MethodPatch, consolePluginObjectPath); ok {
		t.Fatal("ConsolePlugin applied although the plugin is not enabled")
	}
}

func TestCleanup_DeletesConsolePluginOnOCP(t *testing.T) {
	ts := "2026-01-01T00:00:00Z"
	tests := []struct {
		name       string
		isOCP      bool
		pluginBody string
		wantPlugin bool
	}{
		{"own plugin goes with the instance", true, ownPluginBody, true},
		{"another instance's plugin stays", true, otherPluginBody, false},
		{"no plugin API off OpenShift", false, ownPluginBody, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, s := pluginClient(t, tt.pluginBody)
			inst := makeInstance("ri", testNS)
			inst.Metadata.DeletionTimestamp = &ts
			inst.Metadata.Finalizers = []string{finalizer}
			if err := reconcile(context.Background(), c, inst, tt.isOCP); err != nil {
				t.Fatalf("reconcile deletion: %v", err)
			}
			if got := s.seen(http.MethodDelete, testPluginDepPath); got != tt.isOCP {
				t.Errorf("plugin deployment deleted = %v, want %v", got, tt.isOCP)
			}
			if got := s.seen(http.MethodDelete, consolePluginObjectPath); got != tt.wantPlugin {
				t.Errorf("ConsolePlugin deleted = %v, want %v", got, tt.wantPlugin)
			}
		})
	}
}

func TestConsolePluginDeployment_Shape(t *testing.T) {
	dep := consolePluginDeployment(testNS, "ri", "img", "ri-api-key", nil)
	labels := dep.Spec.Template.Metadata.Labels
	if _, ok := labels[instanceLabel]; ok {
		t.Fatalf("plugin pod labels %v carry %s, so the engine Service would route to them", labels, instanceLabel)
	}
	if dep.Metadata.Name != "ri-console-plugin" {
		t.Fatalf("name = %q", dep.Metadata.Name)
	}
	c := dep.Spec.Template.Spec.Containers[0]
	env := map[string]string{}
	for _, e := range c.Env {
		env[e.Name] = e.Value
		if e.ValueFrom != nil {
			t.Fatalf("env %s reads a Secret; the key must be mounted as a file", e.Name)
		}
	}
	if env["RUPTURA_BACKEND_URL"] != "http://ri:8080" {
		t.Fatalf("backend = %q, want the engine Service", env["RUPTURA_BACKEND_URL"])
	}
	if env["RUPTURA_API_KEY_FILE"] != consolePluginKeyMount+"/api-key" {
		t.Fatalf("key file = %q", env["RUPTURA_API_KEY_FILE"])
	}
	if !c.SecurityContext.ReadOnlyRootFilesystem {
		t.Fatal("root filesystem must be read-only")
	}
	if c.ReadinessProbe.HTTPGet.Scheme != "HTTPS" || c.ReadinessProbe.HTTPGet.Port != consolePluginPort {
		t.Fatalf("readiness probe = %+v, want HTTPS on %d", c.ReadinessProbe.HTTPGet, consolePluginPort)
	}
	if sc := dep.Spec.Template.Spec.SecurityContext; sc.RunAsUser != 0 {
		t.Fatalf("OpenShift assigns the UID, got runAsUser %d", sc.RunAsUser)
	}
	svc := consolePluginService(testNS, "ri", nil)
	if svc.Metadata.Annotations[servingCertAnnotation] != "ri-console-plugin-cert" {
		t.Fatalf("service annotations = %v", svc.Metadata.Annotations)
	}
	if svc.Spec.Selector[consolePluginLabel] != "ri" || len(svc.Spec.Selector) != 1 {
		t.Fatalf("service selector = %v", svc.Spec.Selector)
	}
}

func TestConsolePluginObject_ProxiesThroughItsOwnService(t *testing.T) {
	obj := consolePluginObject(testNS, "ri", nil)
	spec := obj["spec"].(map[string]interface{})
	backend := spec["backend"].(map[string]interface{})["service"].(map[string]interface{})
	if backend["name"] != "ri-console-plugin" || backend["namespace"] != testNS || backend["port"] != consolePluginPort {
		t.Fatalf("backend = %v", backend)
	}
	proxies := spec["proxy"].([]interface{})
	if len(proxies) != 1 {
		t.Fatalf("proxies = %v, want one", proxies)
	}
	proxy := proxies[0].(map[string]interface{})
	if proxy["alias"] != consolePluginProxyAlias || proxy["authorization"] != "UserToken" {
		t.Fatalf("proxy = %v", proxy)
	}
	endpoint := proxy["endpoint"].(map[string]interface{})["service"].(map[string]interface{})
	if endpoint["name"] != "ri-console-plugin" || endpoint["namespace"] != testNS {
		t.Fatalf("proxy endpoint = %v", endpoint)
	}
}
