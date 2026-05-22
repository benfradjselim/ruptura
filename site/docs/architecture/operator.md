# Kubernetes Operator

Ruptura ships a Kubernetes operator (`ruptura-operator` v0.7.0) that manages `RupturaInstance` custom resources. The operator polls the API server every 30 seconds, reconciles each instance to the desired state using server-side apply, and handles deletion via a finalizer.

The operator is available on [OperatorHub](https://operatorhub.io/) (community) and is being certified for the [Red Hat OperatorHub](https://catalog.redhat.com/software/operators) (OpenShift catalog).

## CRD: RupturaInstance

```yaml
apiVersion: ruptura.io/v1alpha1
kind: RupturaInstance
metadata:
  name: ruptura
  namespace: ruptura-system
spec:
  # All fields are optional — sensible defaults are provided.
  image: ghcr.io/benfradjselim/ruptura:v7.0.23  # container image (default: bundled version)
  edition: community                              # community (read-only actions) | autopilot (full execution)
  storageSize: 10Gi                               # PVC size for BadgerDB (default: 10Gi)
  replicas: 1                                    # must be 1 — BadgerDB is single-writer (default: 1)
  apiKeyRef: ruptura-secret                      # name of a K8s Secret with key 'api-key' (optional)
  ingestRPS: 1000                                # token-bucket rate limit on ingest (optional)
  resources:
    requests:
      cpu: "50m"
      memory: "64Mi"
    limits:
      cpu: "1000m"
      memory: "512Mi"
```

### Spec field reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `image` | string | `ghcr.io/benfradjselim/ruptura:v7.0.23` | Container image to run |
| `edition` | string | `community` | `community` (read-only actions) or `autopilot` (full T1 execution) |
| `storageSize` | string | `10Gi` | PVC size for BadgerDB persistent storage |
| `replicas` | integer | `1` | Number of replicas — must be 1 (BadgerDB is single-writer) |
| `apiKeyRef` | string | _(none)_ | Name of a `Secret` containing key `api-key` for REST API auth |
| `ingestRPS` | integer | `1000` | Ingest token-bucket rate limit (requests/second) |
| `resources` | object | cpu 50m/1000m · mem 64Mi/512Mi | Container resource requests and limits |

## Resources created

For each `RupturaInstance`, the operator creates and reconciles:

| Resource | Name | Purpose |
|----------|------|---------|
| `ServiceAccount` | `ruptura-instance` | Identity for the Ruptura pod |
| `PersistentVolumeClaim` | `{name}-data` | BadgerDB data directory |
| `Deployment` | `{name}` | Runs the Ruptura container with `system-cluster-critical` priority class |
| `Service` | `{name}` | ClusterIP — ports 8080 (REST API) and 4317 (OTLP ingest) |
| `Route` _(OpenShift only)_ | `{name}` | Edge-TLS terminated public route |

All resources carry finalizer `ruptura.io/cleanup`. On deletion, the operator deletes all owned resources before removing the finalizer.

## Eviction-loop protection (v0.7.0)

On memory-constrained nodes, pods can enter an eviction loop: the kubelet evicts the pod, the Deployment controller recreates it, and it gets evicted again. v0.7.0 detects and breaks this loop automatically.

**How it works:**

1. Before each reconcile, the operator lists all pods for the instance and deletes any with `phase=Failed` or `reason=Evicted`.
2. If 3 or more evicted pods are cleaned up in a single reconcile, the operator sets `replicas=0` and stamps an annotation `ruptura.io/eviction-cooldown-until=<RFC3339>` on the `RupturaInstance`.
3. For the next 3 minutes the operator keeps `replicas=0` — the Deployment controller cannot recreate the pod while replicas is explicitly zero.
4. After the cooldown expires the annotation is cleared and normal reconciliation resumes (replicas=1).

The cooldown state is visible in `.status.phase` (`EvictionCooldown`) and `.status.message`:

```bash
kubectl get rupturainstance ruptura -n ruptura-system
# NAME      PHASE               READY   AVAILABLE   AGE
# ruptura   EvictionCooldown    0       0           5m

kubectl describe rupturainstance ruptura -n ruptura-system
# Status:
#   Phase:    EvictionCooldown
#   Message:  eviction cooldown active until 2026-05-22T10:05:00Z (3 evicted pods cleaned)
```

To cancel a cooldown manually:

```bash
kubectl annotate rupturainstance ruptura -n ruptura-system \
  ruptura.io/eviction-cooldown-until- --overwrite
```

## Install via Red Hat OperatorHub (OpenShift)

On OpenShift 4.12+, install directly from the embedded OperatorHub in the web console, or via CLI:

```bash
# Subscribe via the Red Hat marketplace source
kubectl apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: ruptura-operator
  namespace: openshift-operators
spec:
  channel: stable
  name: ruptura-operator
  source: redhat-marketplace
  sourceNamespace: openshift-marketplace
EOF

# Wait for the operator CSV to become ready
kubectl wait csv -n openshift-operators \
  -l operators.coreos.com/ruptura-operator.openshift-operators \
  --for=jsonpath='{.status.phase}'=Succeeded --timeout=120s
```

Then create a `RupturaInstance` — on OpenShift the operator will also create a `Route` with edge TLS:

```bash
kubectl create namespace ruptura-system
kubectl apply -f - <<EOF
apiVersion: ruptura.io/v1alpha1
kind: RupturaInstance
metadata:
  name: ruptura
  namespace: ruptura-system
spec:
  edition: community
  storageSize: 10Gi
EOF

# Check the Route
oc get route ruptura -n ruptura-system
```

!!! note "Image certification"
    Both `ghcr.io/benfradjselim/ruptura` and `ghcr.io/benfradjselim/ruptura-operator` are built on
    `registry.access.redhat.com/ubi9/ubi-micro` and carry all required Red Hat container labels.
    This satisfies the `BasedOnUBI` and `HasRequiredLabel` preflight checks in the Red Hat certification pipeline.

## Install via OLM / OperatorHub (recommended)

If your cluster runs OLM (Kubernetes or OpenShift):

```bash
# Kubernetes — install OLM first if not present
curl -sL https://github.com/operator-framework/operator-lifecycle-manager/releases/latest/download/install.sh | bash -s latest

# Create a Subscription to install the operator
kubectl apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: ruptura-operator
  namespace: operators
spec:
  channel: stable
  name: ruptura-operator
  source: operatorhubio-catalog
  sourceNamespace: olm
EOF

# Wait for the operator to be ready
kubectl wait --for=condition=Ready pod -l app.kubernetes.io/name=ruptura -n operators --timeout=120s
```

Then create a `RupturaInstance`:

```bash
kubectl create namespace ruptura-system
kubectl apply -f - <<EOF
apiVersion: ruptura.io/v1alpha1
kind: RupturaInstance
metadata:
  name: ruptura
  namespace: ruptura-system
spec:
  edition: community
  storageSize: 10Gi
EOF
```

## Install manually (without OLM)

```bash
# Apply CRD
kubectl apply -f https://raw.githubusercontent.com/benfradjselim/ruptura/main/workdir/deploy/crd/rupturainstances.ruptura.io.yaml

# Apply operator Deployment + RBAC
kubectl apply -f https://raw.githubusercontent.com/benfradjselim/ruptura/main/workdir/deploy/operator.yaml

# Verify the operator is running
kubectl get pods -n ruptura-system -l app.kubernetes.io/component=operator

# Create a RupturaInstance
kubectl apply -f - <<EOF
apiVersion: ruptura.io/v1alpha1
kind: RupturaInstance
metadata:
  name: ruptura
  namespace: ruptura-system
spec:
  edition: community
  storageSize: 10Gi
EOF
```

## Status

```bash
kubectl get rupturainstance ruptura -n ruptura-system
kubectl describe rupturainstance ruptura -n ruptura-system
```

The operator updates `.status` after every reconcile:

```yaml
status:
  phase: Running          # Running | Pending | EvictionCooldown
  message: ""             # set during EvictionCooldown with details
  readyReplicas: 1
  availableReplicas: 1
  lastReconcileTime: "2026-05-22T10:00:00Z"
  observedGeneration: 1
```

## Reconciliation loop

The operator polls every 30 seconds (configurable via `--interval`). Each tick:

1. **List** all `RupturaInstance` resources cluster-wide.
2. For each instance:
   - If `deletionTimestamp` is set → run `cleanup()` (delete all owned resources) then remove the finalizer.
   - Otherwise → ensure the finalizer is registered, then:
     - Clean up any evicted pods.
     - Check / set eviction cooldown.
     - Server-side apply ServiceAccount, PVC, Deployment (with `system-cluster-critical` priority class), Service, and (on OpenShift) Route.
3. **Update status** by reading the owned Deployment's ready/available replica counts.

## RBAC requirements

The operator's service account (`ruptura-operator`) requires:

```yaml
rules:
  - apiGroups: ["ruptura.io"]
    resources: ["rupturainstances", "rupturainstances/status"]
    verbs: ["get", "list", "watch", "update", "patch"]
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: [""]
    resources: ["services", "persistentvolumeclaims", "serviceaccounts"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "delete"]
  - apiGroups: ["route.openshift.io"]
    resources: ["routes"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

## Prometheus metrics

The operator exposes its own Prometheus metrics on `:9090/metrics`:

| Metric | Type | Description |
|--------|------|-------------|
| `ruptura_instances_total` | gauge | Number of `RupturaInstance` resources in the cluster |
| `ruptura_reconcile_errors_total` | gauge | Total reconcile errors since startup |

Liveness and readiness probes check `GET /healthz` on the same port.

## Multiple instances

You can run multiple `RupturaInstance` resources in different namespaces for team isolation:

```bash
kubectl apply -f production-instance.yaml   # namespace: production
kubectl apply -f staging-instance.yaml      # namespace: staging
```

Each instance manages its own independent BadgerDB storage, ServiceAccount, and API key.
