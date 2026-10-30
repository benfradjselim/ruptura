package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"testing"
)

const (
	testNS       = "ruptura-system"
	testDepPath  = "/apis/apps/v1/namespaces/ruptura-system/deployments/ri"
	testSecPath  = "/api/v1/namespaces/ruptura-system/secrets/ri-api-key"
	testRoutePth = "/apis/route.openshift.io/v1/namespaces/ruptura-system/routes/ri"
	testCRBPath  = "/apis/rbac.authorization.k8s.io/v1/clusterrolebindings/ruptura-ruptura-system-ri-monitoring-view"
)

// recorder captures request bodies by "METHOD path".
type recorder struct {
	mu     sync.Mutex
	bodies map[string][]byte
}

func (r *recorder) get(method, path string) (map[string]interface{}, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.bodies[method+" "+path]
	if !ok {
		return nil, false
	}
	var out map[string]interface{}
	_ = json.Unmarshal(b, &out)
	return out, true
}

// recordingClient returns a client whose server records every request body.
// When secretMissing is true, GET on the generated Secret returns 404.
func recordingClient(t *testing.T, secretMissing bool) (*k8sClient, *recorder) {
	t.Helper()
	rec := &recorder{bodies: map[string][]byte{}}
	c := fakeServer(t, map[string]http.HandlerFunc{
		"/": func(w http.ResponseWriter, r *http.Request) {
			b, _ := ioutil.ReadAll(r.Body)
			rec.mu.Lock()
			rec.bodies[r.Method+" "+r.URL.Path] = b
			rec.mu.Unlock()
			if secretMissing && r.Method == http.MethodGet && r.URL.Path == testSecPath {
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

func podSpecOf(t *testing.T, rec *recorder) map[string]interface{} {
	t.Helper()
	dep, ok := rec.get(http.MethodPatch, testDepPath)
	if !ok {
		t.Fatal("deployment was not applied")
	}
	spec := dep["spec"].(map[string]interface{})
	tpl := spec["template"].(map[string]interface{})
	return tpl["spec"].(map[string]interface{})
}

func envOf(t *testing.T, rec *recorder) map[string]map[string]interface{} {
	t.Helper()
	ps := podSpecOf(t, rec)
	ctr := ps["containers"].([]interface{})[0].(map[string]interface{})
	out := map[string]map[string]interface{}{}
	for _, e := range ctr["env"].([]interface{}) {
		m := e.(map[string]interface{})
		out[m["name"].(string)] = m
	}
	return out
}

// Defect 1: RunAsUser/FSGroup only on plain Kubernetes.
func TestReconcile_SecurityContextByPlatform(t *testing.T) {
	cases := []struct {
		name    string
		isOCP   bool
		wantIDs bool
	}{
		{"plain kubernetes keeps 65532", false, true},
		{"openshift leaves unset", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, rec := recordingClient(t, false)
			if err := reconcile(context.Background(), c, makeInstance("ri", testNS), tc.isOCP); err != nil {
				t.Fatal(err)
			}
			sc := podSpecOf(t, rec)["securityContext"].(map[string]interface{})
			_, hasUser := sc["runAsUser"]
			_, hasFS := sc["fsGroup"]
			if hasUser != tc.wantIDs || hasFS != tc.wantIDs {
				t.Fatalf("runAsUser=%v fsGroup=%v, want present=%v", hasUser, hasFS, tc.wantIDs)
			}
			if tc.wantIDs && (sc["runAsUser"].(float64) != 65532 || sc["fsGroup"].(float64) != 65532) {
				t.Fatalf("unexpected ids: %v", sc)
			}
		})
	}
}

// Defect 2: default image.
func TestReconcile_DefaultImage(t *testing.T) {
	cases := []struct{ name, image, want string }{
		{"default", "", "docker.io/selimbf/ruptura:v8.3.0-rc.3"},
		{"override", "example.com/x:1", "example.com/x:1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, rec := recordingClient(t, false)
			inst := makeInstance("ri", testNS)
			inst.Spec.Image = tc.image
			if err := reconcile(context.Background(), c, inst, false); err != nil {
				t.Fatal(err)
			}
			ctr := podSpecOf(t, rec)["containers"].([]interface{})[0].(map[string]interface{})
			if ctr["image"] != tc.want {
				t.Fatalf("image = %v, want %s", ctr["image"], tc.want)
			}
		})
	}
	if defaultImage != "docker.io/selimbf/ruptura:v8.3.0-rc.3" {
		t.Fatalf("defaultImage = %s", defaultImage)
	}
}

// Defect 3: one version constant, used in the startup log line.
func TestOperatorVersion(t *testing.T) {
	if operatorVersion != "0.10.0" {
		t.Fatalf("operatorVersion = %s, want 0.10.0", operatorVersion)
	}
	line := startupMessage("30s")
	if !strings.Contains(line, `version="0.10.0"`) || strings.Contains(line, "0.6.9") {
		t.Fatalf("startup line has wrong version: %s", line)
	}
}

// Defect 4: generated API key Secret.
func TestReconcile_GeneratedAPIKey(t *testing.T) {
	cases := []struct {
		name          string
		apiKeyRef     string
		secretMissing bool
		wantSecret    bool
		wantRef       string
	}{
		{"generated when ref empty and secret absent", "", true, true, "ri-api-key"},
		{"existing generated secret is not rotated", "", false, false, "ri-api-key"},
		{"user supplied ref is used, nothing generated", "mine", true, false, "mine"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			old := logger.Writer()
			logger.SetOutput(&logs)
			defer logger.SetOutput(old)

			c, rec := recordingClient(t, tc.secretMissing)
			inst := makeInstance("ri", testNS)
			inst.Spec.APIKeyRef = tc.apiKeyRef
			if err := reconcile(context.Background(), c, inst, false); err != nil {
				t.Fatal(err)
			}
			sec, applied := rec.get(http.MethodPatch, testSecPath)
			if applied != tc.wantSecret {
				t.Fatalf("secret applied = %v, want %v", applied, tc.wantSecret)
			}
			if applied {
				key, _ := sec["stringData"].(map[string]interface{})["api-key"].(string)
				if len(key) < 32 {
					t.Fatalf("generated key too short: %d", len(key))
				}
				if strings.Contains(logs.String(), key) {
					t.Fatal("API key leaked into logs")
				}
			}
			ref := envOf(t, rec)["RUPTURA_API_KEY"]["valueFrom"].(map[string]interface{})["secretKeyRef"].(map[string]interface{})
			if ref["name"] != tc.wantRef || ref["key"] != "api-key" {
				t.Fatalf("secretKeyRef = %v", ref)
			}
		})
	}
}

func TestGenerateAPIKey_UniqueAndHex(t *testing.T) {
	a, err := generateAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := generateAPIKey()
	if a == b || len(a) != 64 {
		t.Fatalf("keys not unique/64 chars: %q %q", a, b)
	}
}

// Defect 5: Thanos URL and ClusterRoleBinding on OpenShift only.
func TestReconcile_ThanosAndMonitoringView(t *testing.T) {
	cases := []struct {
		name  string
		isOCP bool
	}{
		{"openshift", true},
		{"plain kubernetes", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, rec := recordingClient(t, false)
			if err := reconcile(context.Background(), c, makeInstance("ri", testNS), tc.isOCP); err != nil {
				t.Fatal(err)
			}
			th, hasEnv := envOf(t, rec)["RUPTURA_THANOS_URL"]
			crb, hasCRB := rec.get(http.MethodPatch, testCRBPath)
			if hasEnv != tc.isOCP || hasCRB != tc.isOCP {
				t.Fatalf("env=%v crb=%v want %v", hasEnv, hasCRB, tc.isOCP)
			}
			if !tc.isOCP {
				return
			}
			if th["value"] != "https://thanos-querier.openshift-monitoring.svc:9091" {
				t.Fatalf("thanos url = %v", th["value"])
			}
			ref := crb["roleRef"].(map[string]interface{})
			subj := crb["subjects"].([]interface{})[0].(map[string]interface{})
			if ref["name"] != "cluster-monitoring-view" || ref["kind"] != "ClusterRole" ||
				subj["kind"] != "ServiceAccount" || subj["name"] != "ruptura-instance" || subj["namespace"] != testNS {
				t.Fatalf("bad binding: %v", crb)
			}
		})
	}
}

func TestCleanup_DeletesClusterRoleBindingOnOCP(t *testing.T) {
	inst := makeInstance("ri", testNS)
	inst.Metadata.Finalizers = []string{finalizer}
	c, rec := recordingClient(t, false)
	if err := cleanup(c, inst, true); err != nil {
		t.Fatal(err)
	}
	if _, ok := rec.get(http.MethodDelete, testCRBPath); !ok {
		t.Fatal("ClusterRoleBinding not deleted")
	}
}

// Defect 6: Route edge TLS with redirect.
func TestReconcile_RouteEdgeRedirect(t *testing.T) {
	c, rec := recordingClient(t, false)
	if err := reconcile(context.Background(), c, makeInstance("ri", testNS), true); err != nil {
		t.Fatal(err)
	}
	route, ok := rec.get(http.MethodPatch, testRoutePth)
	if !ok {
		t.Fatal("route not applied")
	}
	tls := route["spec"].(map[string]interface{})["tls"].(map[string]interface{})
	if tls["termination"] != "edge" || tls["insecureEdgeTerminationPolicy"] != "Redirect" {
		t.Fatalf("tls = %v", tls)
	}
}
