package discovery

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	inClusterTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	inClusterCAPath    = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	k8sAPIBase         = "https://kubernetes.default.svc"
)

// InClusterCreds is the exported form of inClusterCreds. External packages
// (e.g. infra collectors) call this instead of duplicating the credential logic.
func InClusterCreds() (apiBase, token string, client *http.Client, err error) {
	return inClusterCreds()
}

// inClusterCreds reads the pod's ServiceAccount token and CA cert and returns
// an authenticated HTTP client. Returns an error when not running inside a pod.
func inClusterCreds() (apiBase, token string, client *http.Client, err error) {
	tokenBytes, err := os.ReadFile(inClusterTokenPath)
	if err != nil {
		return "", "", nil, fmt.Errorf("discovery: read service account token: %w", err)
	}
	caBytes, err := os.ReadFile(inClusterCAPath)
	if err != nil {
		return "", "", nil, fmt.Errorf("discovery: read CA bundle: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caBytes) {
		return "", "", nil, fmt.Errorf("discovery: could not parse CA bundle")
	}
	c := &http.Client{
		Timeout: 0, // no global timeout — watch streams are long-lived
		Transport: &http.Transport{
			TLSClientConfig:     &tls.Config{RootCAs: pool},
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
	return k8sAPIBase, strings.TrimSpace(string(tokenBytes)), c, nil
}
