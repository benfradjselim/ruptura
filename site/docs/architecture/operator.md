# Kubernetes Operator

Kairo v6.1 ships a Kubernetes operator that manages `RupturaInstance` custom resources. The operator reconciles a Deployment, Service, and PersistentVolumeClaim for each instance, keeping the cluster state aligned with the declared spec.

## CRD: RupturaInstance

```yaml
apiVersion: ruptura.io/v1alpha1
kind: RupturaInstance
metadata:
  name: production
  namespace: ruptura-system
spec:
  image: ruptura:6.1.0        # container image to run
  port: 8080                      # HTTP port (REST API)
  storageSize: 20Gi               # PVC size for BadgerDB
  apiKey:
    secretRef: ruptura-api-key      # K8s Secret containing the API key
  replicas: 1                     # number of replicas (default: 1)
```

## Resources created

For each `RupturaInstance`, the operator creates:

| Resource | Name | Purpose |
|----------|------|---------|
| `Deployment` | `{name}` | Runs the ruptura container |
| `Service` | `{name}` | ClusterIP on the specified port |
| `PersistentVolumeClaim` | `{name}-storage` | BadgerDB data directory |

## Deploy the operator

```bash
# Apply CRD + RBAC + operator Deployment
kubectl apply -f deploy/operator/

# Verify the operator is running
kubectl get pods -n ruptura-system -l app=ruptura-operator

# Create a RupturaInstance
kubectl apply -f - <<EOF
apiVersion: ruptura.io/v1alpha1
kind: RupturaInstance
metadata:
  name: production
  namespace: ruptura-system
spec:
  image: ruptura:6.1.0
  port: 8080
  storageSize: 20Gi
  apiKey:
    secretRef: ruptura-api-key
EOF
```

## Reconciliation loop

The operator uses `controller-runtime` and follows the standard Kubernetes reconcile pattern:

1. **Observe** — read current cluster state for the `RupturaInstance`
2. **Diff** — compare with desired spec
3. **Act** — create / update / delete owned resources
4. **Status** — update `RupturaInstanceStatus` with `ready` and `message`

```go
type RupturaInstanceStatus struct {
    Ready   bool      `json:"ready"`
    Message string    `json:"message"`
    Updated time.Time `json:"updated"`
}
```

Check status:

```bash
kubectl get rupturainstance production -o jsonpath='{.status}' | python3 -m json.tool
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
  - apiGroups: ["ruptura.io"]
    resources: ["rupturainstances", "rupturainstances/status"]
    verbs: ["get", "list", "watch", "update", "patch"]
```

## Multiple instances

You can run multiple `RupturaInstance` resources in different namespaces for tenant isolation:

```bash
kubectl apply -f production-instance.yaml   # namespace: production
kubectl apply -f staging-instance.yaml      # namespace: staging
```

Each instance manages its own independent BadgerDB storage and API key.
