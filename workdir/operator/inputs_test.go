package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io/ioutil"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func testCertPEM(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, _ := x509.MarshalECPrivateKey(key)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}

// objectServer serves Secrets and ConfigMaps from memory and records writes.
type objectServer struct {
	mu        sync.Mutex
	objects   map[string]string
	writes    map[string][]byte
	hasDeploy bool
}

func newObjectServer(t *testing.T) (*k8sClient, *objectServer) {
	t.Helper()
	s := &objectServer{objects: map[string]string{}, writes: map[string][]byte{}}
	c := fakeServer(t, map[string]http.HandlerFunc{"/": func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if r.Method == http.MethodGet {
			if body, ok := s.objects[r.URL.Path]; ok {
				_, _ = w.Write([]byte(body))
				return
			}
			if r.URL.Path == testDepPath && s.hasDeploy {
				_, _ = w.Write([]byte(`{"status":{"readyReplicas":1,"availableReplicas":1}}`))
				return
			}
			if strings.Contains(r.URL.Path, "/secrets/") || strings.Contains(r.URL.Path, "/configmaps/") || r.URL.Path == testDepPath {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{}`))
				return
			}
		}
		b, _ := ioutil.ReadAll(r.Body)
		s.writes[r.Method+" "+r.URL.Path] = b
		_, _ = w.Write([]byte(`{}`))
	}})
	return c, s
}

func (s *objectServer) secret(name string, data map[string][]byte) {
	encoded := map[string]string{}
	for k, v := range data {
		encoded[k] = base64.StdEncoding.EncodeToString(v)
	}
	b, _ := json.Marshal(map[string]interface{}{"data": encoded})
	s.objects["/api/v1/namespaces/"+testNS+"/secrets/"+name] = string(b)
}

func (s *objectServer) configMap(name string, data map[string]string) {
	b, _ := json.Marshal(map[string]interface{}{"data": data})
	s.objects["/api/v1/namespaces/"+testNS+"/configmaps/"+name] = string(b)
}

func (s *objectServer) written(method, path string) (map[string]interface{}, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.writes[method+" "+path]
	var out map[string]interface{}
	_ = json.Unmarshal(b, &out)
	return out, ok
}

func secretRef(name, key string) *ValueSource {
	return &ValueSource{SecretKeyRef: &KeySelector{Name: name, Key: key}}
}

func configMapRef(name, key string) *ValueSource {
	return &ValueSource{ConfigMapKeyRef: &KeySelector{Name: name, Key: key}}
}

func fullInput() InputConfig {
	return InputConfig{
		Prometheus: &PrometheusSource{
			URL:           "https://prometheus.example:9090",
			TLS:           &ClientTLS{CA: configMapRef("prom-ca", "ca.crt"), Cert: secretRef("prom-client", "tls.crt"), Key: secretRef("prom-client", "tls.key"), ServerName: "prometheus.example"},
			Authorization: &Authorization{BearerToken: secretRef("prom-token", "token")},
		},
		OTLP: &OTLPInput{TLS: &ServerTLS{Cert: secretRef("otlp-server", "tls.crt"), Key: secretRef("otlp-server", "tls.key"), ClientCA: configMapRef("prom-ca", "ca.crt")}},
	}
}

func seed(t *testing.T, s *objectServer, token string) {
	t.Helper()
	cert, key := testCertPEM(t)
	s.configMap("prom-ca", map[string]string{"ca.crt": string(cert)})
	s.secret("prom-client", map[string][]byte{"tls.crt": cert, "tls.key": key})
	s.secret("otlp-server", map[string][]byte{"tls.crt": cert, "tls.key": key})
	s.secret("prom-token", map[string][]byte{"token": []byte(token)})
}

func TestCheckInputs(t *testing.T) {
	cases := []struct {
		name  string
		input InputConfig
		want  string
	}{
		{"complete", fullInput(), ""},
		{"two sources", InputConfig{Prometheus: &PrometheusSource{URL: "https://p", TLS: &ClientTLS{CA: &ValueSource{SecretKeyRef: &KeySelector{"a", "b"}, ConfigMapKeyRef: &KeySelector{"a", "b"}}}}}, "exactly one of secretKeyRef"},
		{"key in a ConfigMap", InputConfig{Prometheus: &PrometheusSource{URL: "https://p", TLS: &ClientTLS{Cert: secretRef("c", "crt"), Key: configMapRef("c", "key")}}}, "must come from a Secret"},
		{"cert without key", InputConfig{Prometheus: &PrometheusSource{URL: "https://p", TLS: &ClientTLS{Cert: secretRef("c", "crt")}}}, "cert and key go together"},
		{"tls on http", InputConfig{Prometheus: &PrometheusSource{URL: "http://p", TLS: &ClientTLS{CA: configMapRef("c", "ca")}}}, "must use https"},
		{"two auth methods", InputConfig{Prometheus: &PrometheusSource{URL: "https://p", Authorization: &Authorization{BearerToken: secretRef("t", "t"), ServiceAccountToken: true}}}, "exactly one of bearerToken"},
		{"no auth method", InputConfig{Prometheus: &PrometheusSource{URL: "https://p", Authorization: &Authorization{}}}, "exactly one of bearerToken"},
		{"basic without password", InputConfig{Prometheus: &PrometheusSource{URL: "https://p", Authorization: &Authorization{BasicAuth: &BasicAuth{Username: secretRef("u", "u")}}}}, "username and password are required"},
		{"otlp without key", InputConfig{OTLP: &OTLPInput{TLS: &ServerTLS{Cert: secretRef("s", "crt")}}}, "cert and key are required"},
		{"empty key name", InputConfig{Prometheus: &PrometheusSource{URL: "https://p", Authorization: &Authorization{BearerToken: secretRef("t", "")}}}, "name and key are required"},
	}
	for _, tc := range cases {
		got := strings.Join(checkInputs(tc.input), "; ")
		if (tc.want == "") != (got == "") || !strings.Contains(got, tc.want) {
			t.Errorf("%s: problems = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestResolveInputs_MountsAndEnv(t *testing.T) {
	c, s := newObjectServer(t)
	seed(t, s, "token-1")
	out, err := resolveInputs(c, testNS, fullInput())
	if err != nil {
		t.Fatal(err)
	}
	env := map[string]string{}
	for _, e := range out.env {
		env[e.Name] = e.Value
	}
	want := map[string]string{
		"RUPTURA_PROMETHEUS_AUTH":         "bearer",
		"RUPTURA_PROMETHEUS_CA_FILE":      "/etc/ruptura/prometheus/ca.crt",
		"RUPTURA_PROMETHEUS_CERT_FILE":    "/etc/ruptura/prometheus/tls.crt",
		"RUPTURA_PROMETHEUS_KEY_FILE":     "/etc/ruptura/prometheus/tls.key",
		"RUPTURA_PROMETHEUS_TOKEN_FILE":   "/etc/ruptura/prometheus/token",
		"RUPTURA_PROMETHEUS_SERVER_NAME":  "prometheus.example",
		"RUPTURA_OTLP_TLS_CERT_FILE":      "/etc/ruptura/otlp/tls.crt",
		"RUPTURA_OTLP_TLS_KEY_FILE":       "/etc/ruptura/otlp/tls.key",
		"RUPTURA_OTLP_TLS_CLIENT_CA_FILE": "/etc/ruptura/otlp/client-ca.crt",
	}
	for name, value := range want {
		if env[name] != value {
			t.Errorf("%s = %q, want %q", name, env[name], value)
		}
	}
	if len(out.volumes) != 2 || len(out.mounts) != 2 || out.volumes[0].Projected == nil || len(out.volumes[0].Projected.Sources) != 4 {
		t.Fatalf("volumes = %+v, mounts = %+v", out.volumes, out.mounts)
	}
	first := out.hash
	seed(t, s, "token-2")
	rotated, err := resolveInputs(c, testNS, fullInput())
	if err != nil {
		t.Fatal(err)
	}
	if first == "" || rotated.hash == first {
		t.Errorf("hash must change when a referenced value changes: %q then %q", first, rotated.hash)
	}
}

func TestResolveInputs_AuthModes(t *testing.T) {
	c, s := newObjectServer(t)
	s.secret("basic", map[string][]byte{"user": []byte("alice"), "pass": []byte("secret")})
	cases := []struct {
		auth *Authorization
		want string
	}{
		{nil, "none"},
		{&Authorization{ServiceAccountToken: true}, "serviceAccount"},
		{&Authorization{BasicAuth: &BasicAuth{Username: secretRef("basic", "user"), Password: secretRef("basic", "pass")}}, "basic"},
	}
	for _, tc := range cases {
		out, err := resolveInputs(c, testNS, InputConfig{Prometheus: &PrometheusSource{URL: "http://prometheus:9090", Authorization: tc.auth}})
		if err != nil {
			t.Fatal(err)
		}
		if out.env[0].Name != "RUPTURA_PROMETHEUS_AUTH" || out.env[0].Value != tc.want {
			t.Errorf("auth = %+v, want %s", out.env, tc.want)
		}
	}
}

func TestResolveInputs_Problems(t *testing.T) {
	cert, key := testCertPEM(t)
	otherCert, _ := testCertPEM(t)
	cases := []struct {
		name string
		prep func(*objectServer)
		want string
	}{
		{"missing Secret", func(s *objectServer) {}, "Secret prom-token"},
		{"missing key", func(s *objectServer) { s.secret("prom-token", map[string][]byte{"other": []byte("x")}) }, "has no key token"},
		{"empty token", func(s *objectServer) { s.secret("prom-token", map[string][]byte{"token": []byte("  ")}) }, "the value is empty"},
		{"invalid CA", func(s *objectServer) {
			s.secret("prom-token", map[string][]byte{"token": []byte("t")})
			s.configMap("prom-ca", map[string]string{"ca.crt": "not a certificate"})
		}, "no valid PEM certificate"},
		{"mismatched pair", func(s *objectServer) {
			s.secret("prom-token", map[string][]byte{"token": []byte("t")})
			s.configMap("prom-ca", map[string]string{"ca.crt": string(cert)})
			s.secret("prom-client", map[string][]byte{"tls.crt": otherCert, "tls.key": key})
		}, "do not form a valid pair"},
	}
	input := fullInput()
	input.OTLP = nil
	for _, tc := range cases {
		c, s := newObjectServer(t)
		s.configMap("prom-ca", map[string]string{"ca.crt": string(cert)})
		s.secret("prom-client", map[string][]byte{"tls.crt": cert, "tls.key": key})
		tc.prep(s)
		_, err := resolveInputs(c, testNS, input)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", tc.name, err, tc.want)
		}
		if err != nil && strings.Contains(err.Error(), "BEGIN") {
			t.Errorf("%s: error leaks a value: %v", tc.name, err)
		}
	}
}

func TestReconcile_BadInputs(t *testing.T) {
	statusPath := "/apis/ruptura.io/v1alpha1/namespaces/" + testNS + "/rupturainstances/ri/status"
	for _, tc := range []struct {
		name      string
		hasDeploy bool
		phase     string
	}{
		{"new instance deploys nothing", false, "Failed"},
		{"running instance is kept", true, "Degraded"},
	} {
		c, s := newObjectServer(t)
		s.hasDeploy = tc.hasDeploy
		inst := makeInstance("ri", testNS)
		inst.Metadata.Finalizers = []string{finalizer}
		inst.Spec.Input = fullInput()
		if err := reconcile(context.Background(), c, inst, false); err != nil {
			t.Fatal(err)
		}
		if _, ok := s.written(http.MethodPatch, testDepPath); ok {
			t.Errorf("%s: the Deployment must not be written", tc.name)
		}
		status, ok := s.written(http.MethodPatch, statusPath)
		if !ok {
			t.Fatalf("%s: no status written", tc.name)
		}
		st := status["status"].(map[string]interface{})
		if st["phase"] != tc.phase || !strings.Contains(st["message"].(string), "prom-token") {
			t.Errorf("%s: status = %v", tc.name, st)
		}
	}
}

func TestReconcile_InputsMountedWithHash(t *testing.T) {
	c, s := newObjectServer(t)
	seed(t, s, "token-1")
	inst := makeInstance("ri", testNS)
	inst.Metadata.Finalizers = []string{finalizer}
	inst.Spec.Input = fullInput()
	if err := reconcile(context.Background(), c, inst, false); err != nil {
		t.Fatal(err)
	}
	dep, ok := s.written(http.MethodPatch, testDepPath)
	if !ok {
		t.Fatal("Deployment not applied")
	}
	tpl := dep["spec"].(map[string]interface{})["template"].(map[string]interface{})
	annotations, _ := tpl["metadata"].(map[string]interface{})["annotations"].(map[string]interface{})
	if annotations[inputsHashAnnotation] == nil {
		t.Errorf("pod template annotations = %v", annotations)
	}
	volumes := tpl["spec"].(map[string]interface{})["volumes"].([]interface{})
	names := []string{}
	for _, v := range volumes {
		names = append(names, v.(map[string]interface{})["name"].(string))
	}
	if strings.Join(names, ",") != "data,prometheus-credentials,otlp-tls" {
		t.Errorf("volumes = %v", names)
	}
}
