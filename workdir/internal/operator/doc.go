// Package operator implements a lightweight Kubernetes operator for RupturaInstance CRDs.
// It manages the lifecycle (create/update/delete) of ruptura Deployments and Services
// without requiring controller-runtime — all Kubernetes interactions go through the
// KubeClient interface, making the reconciler fully testable without a cluster.
package operator
