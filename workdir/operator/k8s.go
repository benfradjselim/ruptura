package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	saTokenFile   = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	caCertFile    = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	namespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	fieldManager  = "ruptura-operator"
)

var (
	errNotFound = fmt.Errorf("not found")
	errConflict = fmt.Errorf("conflict")
)

// k8sClient is a minimal Kubernetes API client using the in-cluster service account.
// It has no external dependencies — all communication is plain HTTPS with JSON.
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
		ns = []byte("ruptura-system")
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

// get decodes a JSON response from the given API path into out.
func (c *k8sClient) get(path string, out interface{}) error {
	req, _ := http.NewRequest(http.MethodGet, c.host+path, nil)
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
		return fmt.Errorf("GET %s → %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// apply performs a server-side apply (PATCH) of the given object.
func (c *k8sClient) apply(path string, obj interface{}) error {
	body, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	req, _ := http.NewRequest(http.MethodPatch, c.host+path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/apply-patch+yaml")
	req.Header.Set("Accept", "application/json")

	q := req.URL.Query()
	q.Set("fieldManager", fieldManager)
	q.Set("force", "true")
	req.URL.RawQuery = q.Encode()

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("apply %s → %d: %s", path, resp.StatusCode, statusMessage(resp.Body))
	}
	return nil
}

// statusMessage returns the message of a Kubernetes Status body, so an error
// says why the API server refused the request.
func statusMessage(body io.Reader) string {
	var st struct {
		Message string `json:"message"`
	}
	raw, _ := io.ReadAll(io.LimitReader(body, 64<<10))
	if json.Unmarshal(raw, &st) == nil && st.Message != "" {
		return st.Message
	}
	return strings.TrimSpace(string(raw))
}

// patchStatus updates the /status subresource with a merge-patch.
func (c *k8sClient) patchStatus(path string, status interface{}) error {
	body, err := json.Marshal(map[string]interface{}{"status": status})
	if err != nil {
		return err
	}
	req, _ := http.NewRequest(http.MethodPatch, c.host+path+"/status", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/merge-patch+json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("patchStatus %s → %d", path, resp.StatusCode)
	}
	return nil
}

// routeAPIAvailable returns true when the OpenShift Route API group is present.
func (c *k8sClient) routeAPIAvailable() bool {
	req, _ := http.NewRequest(http.MethodGet, c.host+"/apis/route.openshift.io/v1", nil)
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// delete removes a resource by path. A 404 is treated as success (already gone).
func (c *k8sClient) delete(path string) error {
	req, _ := http.NewRequest(http.MethodDelete, c.host+path, nil)
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("delete %s → %d", path, resp.StatusCode)
	}
	return nil
}

// patchFinalizers updates the finalizers list on a RupturaInstance via merge-patch.
func (c *k8sClient) patchFinalizers(inst RupturaInstance, finalizers []string) error {
	body, err := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{"finalizers": finalizers},
	})
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/apis/ruptura.io/v1alpha1/namespaces/%s/rupturainstances/%s",
		inst.Metadata.Namespace, inst.Metadata.Name)
	req, _ := http.NewRequest(http.MethodPatch, c.host+path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/merge-patch+json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("patchFinalizers %s/%s → %d",
			inst.Metadata.Namespace, inst.Metadata.Name, resp.StatusCode)
	}
	return nil
}

func isNotFound(err error) bool { return err == errNotFound }

// send writes an object with POST or PUT. A 409 is errConflict, so a caller can
// tell a lost race from a failure.
func (c *k8sClient) send(method, path string, obj interface{}) error {
	body, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	req, _ := http.NewRequest(method, c.host+path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusConflict:
		return errConflict
	case resp.StatusCode == http.StatusNotFound:
		return errNotFound
	case resp.StatusCode >= 400:
		return fmt.Errorf("%s %s → %d", method, path, resp.StatusCode)
	}
	return nil
}
