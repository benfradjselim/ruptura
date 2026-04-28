package operator

import (
	"context"
	"fmt"
)

// Reconciler drives the control loop for KairoInstance objects.
type Reconciler struct {
	client KubeClient
}

// NewReconciler creates a Reconciler backed by the given KubeClient.
func NewReconciler(client KubeClient) *Reconciler {
	return &Reconciler{client: client}
}

// Reconcile ensures the cluster state matches the desired KairoInstance spec.
// It is idempotent: calling it multiple times has the same effect as once.
func (r *Reconciler) Reconcile(ctx context.Context, inst KairoInstance) error {
	spec := toDeploymentSpec(inst)

	exists, err := r.client.DeploymentExists(ctx, spec.Namespace, spec.Name)
	if err != nil {
		return fmt.Errorf("check deployment: %w", err)
	}

	if !exists {
		if err := r.client.CreateDeployment(ctx, spec); err != nil {
			return fmt.Errorf("create deployment: %w", err)
		}
	} else {
		if err := r.client.UpdateDeployment(ctx, spec); err != nil {
			return fmt.Errorf("update deployment: %w", err)
		}
	}

	if err := r.client.EnsureService(ctx, spec); err != nil {
		return fmt.Errorf("ensure service: %w", err)
	}

	return nil
}

// Delete removes all resources owned by the given KairoInstance.
func (r *Reconciler) Delete(ctx context.Context, inst KairoInstance) error {
	spec := toDeploymentSpec(inst)
	if err := r.client.DeleteDeployment(ctx, spec.Namespace, spec.Name); err != nil {
		return fmt.Errorf("delete deployment: %w", err)
	}
	return r.client.DeleteService(ctx, spec.Namespace, spec.Name)
}

func toDeploymentSpec(inst KairoInstance) DeploymentSpec {
	replicas := inst.Spec.Replicas
	if replicas < 1 {
		replicas = 1
	}
	return DeploymentSpec{
		Name:        inst.Metadata.Name,
		Namespace:   inst.Metadata.Namespace,
		Image:       inst.Spec.Image,
		Port:        inst.Spec.Port,
		StorageSize: inst.Spec.StorageSize,
		APIKeyRef:   inst.Spec.APIKeyRef,
		Replicas:    replicas,
	}
}
