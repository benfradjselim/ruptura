package sources

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type testCert struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
	der  []byte
}

func issue(t *testing.T, name string, parent *testCert, isCA bool) *testCert {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     []string{name},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		IsCA:         isCA, BasicConstraintsValid: true,
	}
	signer, signerKey := tmpl, key
	if parent != nil {
		signer, signerKey = parent.cert, parent.key
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, signer, &key.PublicKey, signerKey)
	if err != nil {
		t.Fatal(err)
	}
	cert, _ := x509.ParseCertificate(der)
	return &testCert{cert: cert, key: key, der: der}
}

func (c *testCert) write(t *testing.T, dir, name string) (certFile, keyFile string) {
	t.Helper()
	certFile, keyFile = filepath.Join(dir, name+".crt"), filepath.Join(dir, name+".key")
	keyDER, _ := x509.MarshalECPrivateKey(c.key)
	mustWrite(t, certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.der}))
	mustWrite(t, keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
	return certFile, keyFile
}

func mustWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

const okBody = `{"status":"success","data":{"result":[]}}`

func TestNewPrometheus_Authentication(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "token"), []byte("sa-token\n"))
	mustWrite(t, filepath.Join(dir, "ca.crt"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: issue(t, "cluster", nil, true).der}))
	mustWrite(t, filepath.Join(dir, "bearer"), []byte("user-token"))
	mustWrite(t, filepath.Join(dir, "user"), []byte("alice"))
	mustWrite(t, filepath.Join(dir, "password"), []byte("secret\n"))

	cases := []struct {
		name string
		cfg  PrometheusConfig
		want string
	}{
		{"default is the service account", PrometheusConfig{}, "Bearer sa-token"},
		{"service account", PrometheusConfig{Auth: AuthServiceAccount}, "Bearer sa-token"},
		{"bearer file", PrometheusConfig{Auth: AuthBearer, TokenFile: filepath.Join(dir, "bearer")}, "Bearer user-token"},
		{"basic", PrometheusConfig{Auth: AuthBasic, UsernameFile: filepath.Join(dir, "user"), PasswordFile: filepath.Join(dir, "password")}, "Basic YWxpY2U6c2VjcmV0"},
		{"none", PrometheusConfig{Auth: AuthNone}, ""},
	}
	for _, tc := range cases {
		var got string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got = r.Header.Get("Authorization")
			_, _ = w.Write([]byte(okBody))
		}))
		tc.cfg.URL, tc.cfg.ServiceAccountDir = srv.URL, dir
		source, err := NewPrometheus(tc.cfg)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if _, _, err := source.query(context.Background(), "up"); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		srv.Close()
		if got != tc.want {
			t.Errorf("%s: Authorization = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestNewPrometheus_TokenRotation(t *testing.T) {
	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "bearer")
	mustWrite(t, tokenFile, []byte("first"))
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(okBody))
	}))
	defer srv.Close()
	source, err := NewPrometheus(PrometheusConfig{URL: srv.URL, Auth: AuthBearer, TokenFile: tokenFile, ServiceAccountDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, tokenFile, []byte("second"))
	if _, _, err := source.query(context.Background(), "up"); err != nil {
		t.Fatal(err)
	}
	if got != "Bearer second" {
		t.Errorf("Authorization = %q, want the rotated token", got)
	}
}

func TestNewPrometheus_InvalidConfig(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]PrometheusConfig{
		"unknown auth":           {Auth: "kerberos"},
		"bearer without file":    {Auth: AuthBearer},
		"basic without password": {Auth: AuthBasic, UsernameFile: "u"},
		"missing CA file":        {Auth: AuthNone, CAFile: filepath.Join(dir, "absent.crt")},
		"cert without key":       {Auth: AuthNone, CertFile: filepath.Join(dir, "c.crt")},
		"missing sa token":       {Auth: AuthServiceAccount},
	}
	for name, cfg := range cases {
		cfg.URL, cfg.ServiceAccountDir = "https://prometheus:9090", dir
		if _, err := NewPrometheus(cfg); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestNewPrometheus_MutualTLS(t *testing.T) {
	dir := t.TempDir()
	ca := issue(t, "test-ca", nil, true)
	caFile, _ := ca.write(t, dir, "ca")
	serverCert := issue(t, "prometheus.example", ca, false)
	clientCert := issue(t, "ruptura", ca, false)
	clientCertFile, clientKeyFile := clientCert.write(t, dir, "client")

	pool := x509.NewCertPool()
	pool.AddCert(ca.cert)
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(okBody))
	}))
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{serverCert.der}, PrivateKey: serverCert.key}},
		ClientCAs:    pool, ClientAuth: tls.RequireAndVerifyClientCert,
	}
	srv.StartTLS()
	defer srv.Close()

	base := PrometheusConfig{URL: srv.URL, Auth: AuthNone, CAFile: caFile, ServerName: "prometheus.example", ServiceAccountDir: dir}
	withCert := base
	withCert.CertFile, withCert.KeyFile = clientCertFile, clientKeyFile
	cases := []struct {
		name string
		cfg  PrometheusConfig
		ok   bool
	}{
		{"CA and client certificate", withCert, true},
		{"no client certificate", base, false},
	}
	for _, tc := range cases {
		source, err := NewPrometheus(tc.cfg)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		_, _, err = source.query(context.Background(), "up")
		if (err == nil) != tc.ok {
			t.Errorf("%s: err = %v, want ok %v", tc.name, err, tc.ok)
		}
	}
}
