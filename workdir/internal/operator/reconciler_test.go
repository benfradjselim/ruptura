package operator

import (
	"context"
	"errors"
	"testing"
)

func testInstance(name, ns string, replicas int) KairoInstance {
	return KairoInstance{
		APIVersion: "kairo.io/v1",
		Kind:       "KairoInstance",
		Metadata:   ObjectMeta{Name: name, Namespace: ns},
		Spec: KairoInstanceSpec{
			Image:       "ghcr.io/benfradjselim/kairo-core:v6.1",
			Port:        8080,
			StorageSize: "5Gi",
			APIKeyRef:   "kairo-secret",
			Replicas:    replicas,
		},
	}
}

func TestReconcile_CreateNew(t *testing.T) {
	stub := NewStubKubeClient()
	r := NewReconciler(stub)
	inst := testInstance("my-kairo", "default", 1)

	if err := r.Reconcile(context.Background(), inst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := stub.Deployments["default/my-kairo"]; !ok {
		t.Error("expected deployment to be created")
	}
	if _, ok := stub.Services["default/my-kairo"]; !ok {
		t.Error("expected service to be ensured")
	}
}

func TestReconcile_UpdateExisting(t *testing.T) {
	stub := NewStubKubeClient()
	stub.Deployments["default/my-kairo"] = DeploymentSpec{Name: "my-kairo", Namespace: "default"}
	r := NewReconciler(stub)
	inst := testInstance("my-kairo", "default", 2)

	if err := r.Reconcile(context.Background(), inst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := stub.Deployments["default/my-kairo"]
	if got.Replicas != 2 {
		t.Errorf("want replicas=2 got %d", got.Replicas)
	}
}

func TestReconcile_DefaultReplicas(t *testing.T) {
	stub := NewStubKubeClient()
	r := NewReconciler(stub)
	inst := testInstance("x", "prod", 0)

	if err := r.Reconcile(context.Background(), inst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := stub.Deployments["prod/x"]
	if got.Replicas != 1 {
		t.Errorf("want default replicas=1 got %d", got.Replicas)
	}
}

func TestReconcile_ErrorOnCreate(t *testing.T) {
	stub := NewStubKubeClient()
	stub.Errors["CreateDeployment"] = errors.New("quota exceeded")
	r := NewReconciler(stub)
	inst := testInstance("fail", "default", 1)

	err := r.Reconcile(context.Background(), inst)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDelete_RemovesAll(t *testing.T) {
	stub := NewStubKubeClient()
	stub.Deployments["default/kairo"] = DeploymentSpec{Name: "kairo", Namespace: "default"}
	stub.Services["default/kairo"] = DeploymentSpec{Name: "kairo", Namespace: "default"}
	r := NewReconciler(stub)
	inst := testInstance("kairo", "default", 1)

	if err := r.Delete(context.Background(), inst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := stub.Deployments["default/kairo"]; ok {
		t.Error("deployment should have been deleted")
	}
	if _, ok := stub.Services["default/kairo"]; ok {
		t.Error("service should have been deleted")
	}
}

func TestDelete_ErrorPropagation(t *testing.T) {
	stub := NewStubKubeClient()
	stub.Errors["DeleteDeployment"] = errors.New("forbidden")
	r := NewReconciler(stub)
	inst := testInstance("x", "default", 1)

	err := r.Delete(context.Background(), inst)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
