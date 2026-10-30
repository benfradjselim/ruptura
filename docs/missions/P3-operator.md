# Mission P3-operator

- Component: C13-packaging
- Allowed paths: workdir/operator/, bundle/, operators/ruptura-operator/, catalog/ruptura-operator/
- Defects to fix:
  1. The reconciled Deployment forces `RunAsUser` and `FSGroup` 65532, which OpenShift refuses. On OpenShift, leave both unset. Keep them on plain Kubernetes.
  2. Default image is `ghcr.io/benfradjselim/ruptura:v6.8.1`. Use `docker.io/selimbf/ruptura:v8.3.0-rc.3`.
  3. The operator logs version 0.6.9. Report the real version from one constant.
  4. Reconcile generates and stores an API key Secret when `apiKeyRef` is empty, and passes `RUPTURA_API_KEY` to the engine. Never log it.
  5. On OpenShift, set `RUPTURA_THANOS_URL` to `https://thanos-querier.openshift-monitoring.svc:9091`, and create a ClusterRoleBinding from the instance service account to `cluster-monitoring-view`.
  6. Route must use edge TLS with redirect.
  7. Prepare a new bundle version 0.10.0 in `operators/ruptura-operator/0.10.0/` and `bundle/`: `com.redhat.openshift.versions: "v4.12"` meaning that version and later, `replaces: ruptura-operator.v0.9.1`, updated RBAC, and `relatedImages`.
- Numeric gate: table-driven tests cover defects 1 to 6, and `cd workdir/operator && GOFLAGS=-p=1 go test ./...` passes.
- Constraints: raw Kubernetes API client as already used, no client-go, Go 1.18, godoc on exported symbols, English only. Do not open any pull request, publish images, or touch a cluster. Do not use `git stash`.
- Stop rule: 2 attempts without progress, return to the lead.
- Report: defects fixed, tests before and after, changed files, blockers, 150 words maximum.
