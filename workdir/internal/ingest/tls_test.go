package ingest

import (
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
	"os"
	"path/filepath"
	"testing"
	"time"
)

type pair struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
	der  []byte
}

func newPair(t *testing.T, name string, parent *pair) *pair {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: name},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		IsCA:        parent == nil, BasicConstraintsValid: true,
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
	return &pair{cert: cert, key: key, der: der}
}

func (p *pair) files(t *testing.T, dir, name string) (string, string) {
	t.Helper()
	certFile, keyFile := filepath.Join(dir, name+".crt"), filepath.Join(dir, name+".key")
	keyDER, _ := x509.MarshalECPrivateKey(p.key)
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: p.der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	return certFile, keyFile
}

func (p *pair) tlsCert() tls.Certificate {
	return tls.Certificate{Certificate: [][]byte{p.der}, PrivateKey: p.key}
}

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().String()
}

func TestStartHTTP_TLS(t *testing.T) {
	dir := t.TempDir()
	ca := newPair(t, "ca", nil)
	caFile, _ := ca.files(t, dir, "ca")
	serverCert, serverKey := newPair(t, "ruptura", ca).files(t, dir, "server")
	client := newPair(t, "collector", ca)
	pool := x509.NewCertPool()
	pool.AddCert(ca.cert)

	cases := []struct {
		name       string
		clientCA   string
		clientCert *tls.Certificate
		ok         bool
	}{
		{"TLS without client certificates", "", nil, true},
		{"mTLS with a client certificate", caFile, func() *tls.Certificate { c := client.tlsCert(); return &c }(), true},
		{"mTLS without a client certificate", caFile, nil, false},
	}
	for _, tc := range cases {
		config, err := ServerTLS(serverCert, serverKey, tc.clientCA)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		e := New(nil, nil, nil, nil, nil)
		e.SetTLS(config)
		addr := freeAddr(t)
		if err := e.StartHTTP(addr); err != nil {
			t.Fatal(err)
		}
		clientTLS := &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
		if tc.clientCert != nil {
			clientTLS.Certificates = []tls.Certificate{*tc.clientCert}
		}
		httpClient := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{TLSClientConfig: clientTLS}}
		var resp *http.Response
		for i := 0; i < 20; i++ {
			resp, err = httpClient.Post("https://"+addr+"/v1/metrics", "application/json", nil)
			if err == nil || tc.ok == false && i > 3 {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		if resp != nil {
			resp.Body.Close()
		}
		if (err == nil) != tc.ok {
			t.Errorf("%s: err = %v, want ok %v", tc.name, err, tc.ok)
		}
		_ = e.httpServer.Close()
	}
}

func TestServerTLS_Invalid(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := newPair(t, "ruptura", newPair(t, "ca", nil)).files(t, dir, "server")
	junk := filepath.Join(dir, "junk.crt")
	if err := os.WriteFile(junk, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := map[string][3]string{
		"missing certificate": {filepath.Join(dir, "absent.crt"), keyFile, ""},
		"missing client CA":   {certFile, keyFile, filepath.Join(dir, "absent-ca.crt")},
		"invalid client CA":   {certFile, keyFile, junk},
	}
	for name, files := range cases {
		if _, err := ServerTLS(files[0], files[1], files[2]); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
