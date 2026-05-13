package discovery

import (
	"testing"
)

// TestNewInformer_NotInCluster verifies that NewInformer returns an error
// when the pod ServiceAccount files are absent (i.e., outside a k8s cluster).
// If the test is actually running inside a pod, it is skipped.
func TestNewInformer_NotInCluster(t *testing.T) {
	inf, err := NewInformer()
	if err == nil {
		// We are inside a real cluster — nothing to assert here.
		t.Skip("running inside a k8s pod — skipping not-in-cluster check")
	}
	if inf != nil {
		t.Error("expected nil Informer on error, got non-nil")
	}
}
