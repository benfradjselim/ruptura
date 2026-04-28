# Kubernetes Operator

Kairo v6.1 ships a Kubernetes operator that manages `KairoInstance` custom resources. The operator reconciles a Deployment, Service, and PersistentVolumeClaim for each instance, keeping the cluster state aligned with the declared spec.

## CRD: KairoInstance

```yaml
apiVersion: kairo.io/v1alpha1
kind: KairoInstance
metadata:
  name: production
  namespace: kairo-system
spec:
  image: kairo-core:6.1.0        # container image to run
  port: 8080                      # HTTP port (REST API)
  storageSize: 20Gi               # PVC size for BadgerDB
  apiKey:
    secretRef: kairo-api-key      # K8s Secret containing the API key
  replicas: 1                     # number of replicas (default: 1)
```

## Resources created

For each `KairoInstance`, the operator creates:

| Resource | Name | Purpose |
|----------|------|---------|
| `Deployment` | `{name}` | Runs the kairo-core container |
| `Service` | `{name}` | ClusterIP on the specified port |
| `PersistentVolumeClaim` | `{name}-storage` | BadgerDB data directory |

## Deploy the operator

```bash
# Apply CRD + RBAC + operator Deployment
kubectl apply -f deploy/operator/

# Verify the operator is running
kubectl get pods -n kairo-system -l app=kairo-operator

# Create a KairoInstance
kubectl apply -f - <<EOF
apiVersion: kairo.io/v1alpha1
kind: KairoInstance
metadata:
  name: production
  namespace: kairo-system
spec:
  image: kairo-core:6.1.0
  port: 8080
  storageSize: 20Gi
  apiKey:
    secretRef: kairo-api-key
EOF
```

## Reconciliation loop

The operator uses `controller-runtime` and follows the standard Kubernetes reconcile pattern:

1. **Observe** — read current cluster state for the `KairoInstance`
2. **Diff** — compare with desired spec
3. **Act** — create / update / delete owned resources
4. **Status** — update `KairoInstanceStatus` with `ready` and `message`

```go
type KairoInstanceStatus struct {
    Ready   bool      `json:"ready"`
    Message string    `json:"message"`
    Updated time.Time `json:"updated"`
}
```

Check status:

```bash
kubectl get kairoinstance production -o jsonpath='{.status}' | python3 -m json.tool
```

## RBAC requirements

The operator service account requires:

```yaml
rules:
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: [""]
    resources: ["services", "persistentvolumeclaims"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["kairo.io"]
    resources: ["kairoinstances", "kairoinstances/status"]
    verbs: ["get", "list", "watch", "update", "patch"]
```

## Multiple instances

You can run multiple `KairoInstance` resources in different namespaces for tenant isolation:

```bash
kubectl apply -f production-instance.yaml   # namespace: production
kubectl apply -f staging-instance.yaml      # namespace: staging
```

Each instance manages its own independent BadgerDB storage and API key.
