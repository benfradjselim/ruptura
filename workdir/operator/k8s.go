// Package main implements the OHE operator — a lightweight K8s controller
// that reconciles OHECluster custom resources to Deployments / DaemonSets.
//
// It uses only the Go standard library: it talks directly to the K8s API
// server over HTTPS, reading the in-cluster service account token.
// No controller-runtime or client-go dependency is needed.
package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

const (
	saTokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	caCertFile  = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	namespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

// k8sClient is a minimal K8s API client using the in-cluster service account.
type k8sClient struct {
	host   string
	token  string
	ns     string
	client *http.Client
}

func newK8sClient() (*k8sClient, error) {
	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	port := os.Getenv("KUBERNETES_SERVICE_PORT")
	if host == "" || port == "" {
		return nil, fmt.Errorf("not running inside a Kubernetes cluster (KUBERNETES_SERVICE_HOST not set)")
	}

	token, err := os.ReadFile(saTokenFile)
	if err != nil {
		return nil, fmt.Errorf("read service account token: %w", err)
	}

	ca, err := os.ReadFile(caCertFile)
	if err != nil {
		return nil, fmt.Errorf("read CA cert: %w", err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(ca)

	ns, err := os.ReadFile(namespaceFile)
	if err != nil {
		ns = []byte("ohe-system")
	}

	return &k8sClient{
		host:  "https://" + host + ":" + port,
		token: string(token),
		ns:    string(ns),
		client: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{RootCAs: pool},
			},
		},
	}, nil
}

func (c *k8sClient) get(path string, out interface{}) error {
	req, err := http.NewRequest(http.MethodGet, c.host+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return errNotFound
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("K8s API %s returned %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *k8sClient) apply(path string, obj interface{}) error {
	body, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPatch, c.host+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/apply-patch+yaml")
	req.Header.Set("Accept", "application/json")

	// Server-side apply with force
	q := req.URL.Query()
	q.Set("fieldManager", "ohe-operator")
	q.Set("force", "true")
	req.URL.RawQuery = q.Encode()

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("apply %s returned %d", path, resp.StatusCode)
	}
	return nil
}

func (c *k8sClient) patchStatus(path string, status interface{}) error {
	body, err := json.Marshal(map[string]interface{}{"status": status})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPatch, c.host+path+"/status", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/merge-patch+json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("patchStatus %s returned %d", path, resp.StatusCode)
	}
	return nil
}

var errNotFound = fmt.Errorf("not found")

func isNotFound(err error) bool { return err == errNotFound }
