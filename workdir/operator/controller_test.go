package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeServer builds a test HTTP server that records PATCH/DELETE calls and
// returns canned responses. It wires a k8sClient that talks to it.
func fakeServer(t *testing.T, handlers map[string]http.HandlerFunc) *k8sClient {
	t.Helper()
	mux := http.NewServeMux()
	for path, h := range handlers {
		mux.HandleFunc(path, h)
	}
	// Catch-all: return 200 with an empty JSON object so apply() succeeds.
	// Skip if the caller already registered "/".
	if _, hasRoot := handlers["/"]; !hasRoot {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		})
	}
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)

	return &k8sClient{
		host:   srv.URL,
		token:  "test-token",
		ns:     "ruptura-system",
		client: srv.Client(),
	}
}

func makeInstance(name, ns string) RupturaInstance {
	return RupturaInstance{
		APIVersion: "ruptura.io/v1alpha1",
		Kind:       "RupturaInstance",
		Metadata:   ObjectMeta{Name: name, Namespace: ns},
		Spec:       RupturaInstanceSpec{Edition: "community", StorageSize: "1Gi"},
	}
}

// ── hasFinalizer / removeFinalizer ───────────────────────────────────────────

func TestHasFinalizer(t *testing.T) {
	inst := makeInstance("ri", "ns")
	if hasFinalizer(inst) {
		t.Fatal("expected no finalizer on fresh instance")
	}
	inst.Metadata.Finalizers = []string{finalizer}
	if !hasFinalizer(inst) {
		t.Fatal("expected finalizer to be present")
	}
}

func TestRemoveFinalizer(t *testing.T) {
	inst := makeInstance("ri", "ns")
	inst.Metadata.Finalizers = []string{"other.io/x", finalizer, "other.io/y"}
	got := removeFinalizer(inst)
	for _, f := range got {
		if f == finalizer {
			t.Fatalf("finalizer %q still present after removal", finalizer)
		}
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 remaining finalizers, got %d", len(got))
	}
}

// ── reconcile: finalizer registration ────────────────────────────────────────

func TestReconcile_AddsFinalizer(t *testing.T) {
	patched := false
	inst := makeInstance("ri", "ruptura-system")

	c := fakeServer(t, map[string]http.HandlerFunc{
		"/apis/ruptura.io/v1alpha1/namespaces/ruptura-system/rupturainstances/ri": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPatch {
				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				meta, _ := body["metadata"].(map[string]interface{})
				fins, _ := meta["finalizers"].([]interface{})
				for _, f := range fins {
					if f == finalizer {
						patched = true
					}
				}
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		},
	})

	if err := reconcile(context.Background(), c, inst, false); err != nil {
		t.Fatalf("reconcile returned error: %v", err)
	}
	if !patched {
		t.Fatal("expected finalizer to be patched onto instance")
	}
}

// ── reconcile: deletion flow ─────────────────────────────────────────────────

func TestReconcile_DeletionRunsCleanup(t *testing.T) {
	ts := "2026-01-01T00:00:00Z"
	inst := makeInstance("ri", "ruptura-system")
	inst.Metadata.DeletionTimestamp = &ts
	inst.Metadata.Finalizers = []string{finalizer}

	deleted := map[string]bool{}
	finalizerRemoved := false

	c := fakeServer(t, map[string]http.HandlerFunc{
		// capture DELETE calls
		"/apis/apps/v1/namespaces/ruptura-system/deployments/ri": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodDelete {
				deleted["deployment"] = true
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		},
		"/api/v1/namespaces/ruptura-system/services/ri": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodDelete {
				deleted["service"] = true
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		},
		"/api/v1/namespaces/ruptura-system/persistentvolumeclaims/ri-data": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodDelete {
				deleted["pvc"] = true
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		},
		// capture finalizer removal PATCH
		"/apis/ruptura.io/v1alpha1/namespaces/ruptura-system/rupturainstances/ri": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPatch {
				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				meta, _ := body["metadata"].(map[string]interface{})
				fins, _ := meta["finalizers"].([]interface{})
				if len(fins) == 0 {
					finalizerRemoved = true
				}
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		},
	})

	if err := reconcile(context.Background(), c, inst, false); err != nil {
		t.Fatalf("reconcile deletion returned error: %v", err)
	}

	for _, resource := range []string{"deployment", "service", "pvc"} {
		if !deleted[resource] {
			t.Errorf("expected %s to be deleted", resource)
		}
	}
	if !finalizerRemoved {
		t.Error("expected finalizer to be removed after cleanup")
	}
}

func TestReconcile_DeletionWithoutFinalizer_IsNoop(t *testing.T) {
	ts := "2026-01-01T00:00:00Z"
	inst := makeInstance("ri", "ruptura-system")
	inst.Metadata.DeletionTimestamp = &ts
	// no finalizer — someone else already removed it

	deleteCalled := false
	trackDelete := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalled = true
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}
	c := fakeServer(t, map[string]http.HandlerFunc{
		"/apis/apps/v1/namespaces/ruptura-system/deployments/ri":                    trackDelete,
		"/api/v1/namespaces/ruptura-system/services/ri":                             trackDelete,
		"/api/v1/namespaces/ruptura-system/persistentvolumeclaims/ri-data":          trackDelete,
		"/apis/route.openshift.io/v1/namespaces/ruptura-system/routes/ri":           trackDelete,
	})

	if err := reconcile(context.Background(), c, inst, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleteCalled {
		t.Fatal("expected no DELETE calls when finalizer absent")
	}
}

// ── cleanup: OpenShift Route deletion ────────────────────────────────────────

func TestCleanup_DeletesRouteOnOCP(t *testing.T) {
	inst := makeInstance("ri", "ruptura-system")
	inst.Metadata.Finalizers = []string{finalizer}

	routeDeleted := false
	c := fakeServer(t, map[string]http.HandlerFunc{
		"/apis/route.openshift.io/v1/namespaces/ruptura-system/routes/ri": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodDelete {
				routeDeleted = true
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		},
	})

	if err := cleanup(c, inst, true); err != nil {
		t.Fatalf("cleanup returned error: %v", err)
	}
	if !routeDeleted {
		t.Error("expected Route to be deleted on OCP")
	}
}

func TestCleanup_SkipsRouteOnVanillaK8s(t *testing.T) {
	inst := makeInstance("ri", "ruptura-system")
	inst.Metadata.Finalizers = []string{finalizer}

	routeCalled := false
	c := fakeServer(t, map[string]http.HandlerFunc{
		"/apis/route.openshift.io/v1/namespaces/ruptura-system/routes/ri": func(w http.ResponseWriter, r *http.Request) {
			routeCalled = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		},
	})

	if err := cleanup(c, inst, false); err != nil {
		t.Fatalf("cleanup returned error: %v", err)
	}
	if routeCalled {
		t.Error("Route endpoint should not be called on vanilla K8s")
	}
}

// ── delete: 404 treated as success ───────────────────────────────────────────

func TestDelete_404IsSuccess(t *testing.T) {
	c := fakeServer(t, map[string]http.HandlerFunc{
		"/apis/apps/v1/namespaces/ns/deployments/gone": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"reason":"NotFound"}`))
		},
	})
	if err := c.delete("/apis/apps/v1/namespaces/ns/deployments/gone"); err != nil {
		t.Fatalf("expected 404 to be treated as success, got: %v", err)
	}
}

// ── cleanupEvictedPods ───────────────────────────────────────────────────────

func TestCleanupEvictedPods_DeletesFailedPods(t *testing.T) {
	pods := PodList{
		Items: []Pod{
			{Metadata: ObjectMeta{Name: "ruptura-abc"}, Status: PodStatus{Phase: "Failed", Reason: "Evicted"}},
			{Metadata: ObjectMeta{Name: "ruptura-def"}, Status: PodStatus{Phase: "Running"}},
		},
	}
	podsJSON, _ := json.Marshal(pods)
	deleted := []string{}

	c := fakeServer(t, map[string]http.HandlerFunc{
		"/api/v1/namespaces/ns/pods": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(podsJSON)
		},
		"/api/v1/namespaces/ns/pods/ruptura-abc": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodDelete {
				deleted = append(deleted, "ruptura-abc")
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		},
		"/api/v1/namespaces/ns/pods/ruptura-def": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodDelete {
				deleted = append(deleted, "ruptura-def")
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		},
	})

	count, err := cleanupEvictedPods(c, "ns", "ri")
	if err != nil {
		t.Fatalf("cleanupEvictedPods returned error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 pod deleted, got %d", count)
	}
	if len(deleted) != 1 || deleted[0] != "ruptura-abc" {
		t.Errorf("expected only ruptura-abc to be deleted, got %v", deleted)
	}
}

func TestCleanupEvictedPods_NoPods(t *testing.T) {
	podsJSON, _ := json.Marshal(PodList{})
	c := fakeServer(t, map[string]http.HandlerFunc{
		"/api/v1/namespaces/ns/pods": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(podsJSON)
		},
	})
	count, err := cleanupEvictedPods(c, "ns", "ri")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 deleted, got %d", count)
	}
}

// ── reconcile: eviction cooldown ─────────────────────────────────────────────

func TestReconcile_EvictionStorm_ScalesToZero(t *testing.T) {
	pods := PodList{
		Items: []Pod{
			{Metadata: ObjectMeta{Name: "p1"}, Status: PodStatus{Phase: "Failed", Reason: "Evicted"}},
			{Metadata: ObjectMeta{Name: "p2"}, Status: PodStatus{Phase: "Failed", Reason: "Evicted"}},
			{Metadata: ObjectMeta{Name: "p3"}, Status: PodStatus{Phase: "Failed", Reason: "Evicted"}},
		},
	}
	podsJSON, _ := json.Marshal(pods)

	var deploymentReplicas float64 = -1
	annotationSet := ""

	inst := makeInstance("ri", "ruptura-system")

	c := fakeServer(t, map[string]http.HandlerFunc{
		"/api/v1/namespaces/ruptura-system/pods": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(podsJSON)
		},
		"/apis/apps/v1/namespaces/ruptura-system/deployments/ri": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPatch {
				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				if spec, ok := body["spec"].(map[string]interface{}); ok {
					if r, ok := spec["replicas"].(float64); ok {
						deploymentReplicas = r
					}
				}
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		},
		"/apis/ruptura.io/v1alpha1/namespaces/ruptura-system/rupturainstances/ri": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPatch {
				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				if meta, ok := body["metadata"].(map[string]interface{}); ok {
					if ann, ok := meta["annotations"].(map[string]interface{}); ok {
						if v, ok := ann[evictionCooldownAnnotation].(string); ok {
							annotationSet = v
						}
					}
				}
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		},
	})

	if err := reconcile(context.Background(), c, inst, false); err != nil {
		t.Fatalf("reconcile returned error: %v", err)
	}
	if deploymentReplicas != 0 {
		t.Errorf("expected replicas=0 during eviction storm, got %v", deploymentReplicas)
	}
	if annotationSet == "" {
		t.Error("expected eviction cooldown annotation to be set")
	}
}

func TestReconcile_CooldownActive_SkipsRecreate(t *testing.T) {
	// Instance already has an active cooldown annotation.
	inst := makeInstance("ri", "ruptura-system")
	inst.Metadata.Annotations = map[string]string{
		evictionCooldownAnnotation: time.Now().Add(2 * time.Minute).UTC().Format(time.RFC3339),
	}
	// Empty pod list — no new evictions.
	podsJSON, _ := json.Marshal(PodList{})

	var deploymentReplicas float64 = 1 // expect it to remain 0 after apply

	c := fakeServer(t, map[string]http.HandlerFunc{
		"/api/v1/namespaces/ruptura-system/pods": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(podsJSON)
		},
		"/apis/apps/v1/namespaces/ruptura-system/deployments/ri": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPatch {
				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				if spec, ok := body["spec"].(map[string]interface{}); ok {
					if r, ok := spec["replicas"].(float64); ok {
						deploymentReplicas = r
					}
				}
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		},
	})

	if err := reconcile(context.Background(), c, inst, false); err != nil {
		t.Fatalf("reconcile returned error: %v", err)
	}
	if deploymentReplicas != 0 {
		t.Errorf("expected replicas=0 during active cooldown, got %v", deploymentReplicas)
	}
}

// ── runLoop: reconciles all items in list ────────────────────────────────────

func TestRunLoop_ReconcilesList(t *testing.T) {
	reconciled := []string{}

	list := RupturaInstanceList{
		Items: []RupturaInstance{
			makeInstance("a", "ruptura-system"),
			makeInstance("b", "ruptura-system"),
		},
	}
	listJSON, _ := json.Marshal(list)

	c := fakeServer(t, map[string]http.HandlerFunc{
		"/apis/ruptura.io/v1alpha1/rupturainstances": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(listJSON)
		},
		"/": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "rupturainstances") {
				parts := strings.Split(r.URL.Path, "/")
				reconciled = append(reconciled, parts[len(parts)-1])
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		},
	})

	runLoop(context.Background(), c, false)

	if len(reconciled) < 2 {
		t.Fatalf("expected at least 2 instances reconciled, got %d", len(reconciled))
	}
}
