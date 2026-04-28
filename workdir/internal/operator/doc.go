// Package operator implements a lightweight Kubernetes operator for KairoInstance CRDs.
// It manages the lifecycle (create/update/delete) of kairo-core Deployments and Services
// without requiring controller-runtime — all Kubernetes interactions go through the
// KubeClient interface, making the reconciler fully testable without a cluster.
package operator
