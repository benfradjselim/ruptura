package k8smetrics

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

func inClusterCreds() (apiBase, token string, client *http.Client, err error) {
	tokenBytes, err := os.ReadFile(inClusterTokenPath)
	if err != nil {
		return "", "", nil, fmt.Errorf("k8smetrics: read service account token: %w", err)
	}
	caBytes, err := os.ReadFile(inClusterCAPath)
	if err != nil {
		return "", "", nil, fmt.Errorf("k8smetrics: read CA bundle: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caBytes) {
		return "", "", nil, fmt.Errorf("k8smetrics: could not parse CA bundle")
	}
	c := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:     &tls.Config{RootCAs: pool},
			IdleConnTimeout:     60 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
	return k8sAPIBase, strings.TrimSpace(string(tokenBytes)), c, nil
}
