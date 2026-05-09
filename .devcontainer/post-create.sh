#!/usr/bin/env bash
# post-create.sh — deploys the Ruptura + Prometheus + Grafana stack into k3s
# and runs a quick smoke test to confirm everything is healthy.
# Safe to re-run (kubectl apply is idempotent).
set -euo pipefail

export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

# ── Wait for k3s to be ready ────────────────────────────────────────────────
echo "==> Waiting for k3s node to become Ready..."
until kubectl get nodes 2>/dev/null | grep -q " Ready"; do sleep 3; done
echo "    Node ready."

# ── Namespaces ───────────────────────────────────────────────────────────────
kubectl create namespace ruptura-system --dry-run=client -o yaml | kubectl apply -f -
kubectl create namespace monitoring      --dry-run=client -o yaml | kubectl apply -f -

# ── API key secret (lab: deterministic value, not secret) ────────────────────
kubectl -n ruptura-system create secret generic ruptura-secrets \
  --from-literal=api-key=ruptura-lab-insecure-key \
  --dry-run=client -o yaml | kubectl apply -f -

# ── Core stack ───────────────────────────────────────────────────────────────
echo "==> Applying Ruptura manifests..."
kubectl apply -f workdir/deploy/rbac.yaml
kubectl apply -f workdir/deploy/configmap.yaml
kubectl apply -f workdir/deploy/pvc.yaml
kubectl apply -f workdir/deploy/central-deployment.yaml

echo "==> Applying Prometheus manifests..."
kubectl apply -f workdir/deploy/prometheus.yaml

echo "==> Applying Grafana manifests..."
kubectl apply -f workdir/deploy/grafana.yaml

# ── Wait for pods ────────────────────────────────────────────────────────────
echo "==> Waiting for Ruptura pod to be Running (up to 3 min)..."
kubectl -n ruptura-system rollout status deployment/ruptura --timeout=180s || {
  echo ""
  echo "!!! Ruptura pod did not become ready. Collecting diagnostics:"
  echo ""
  echo "--- Pod status ---"
  kubectl -n ruptura-system get pods -o wide
  echo ""
  echo "--- Pod describe ---"
  kubectl -n ruptura-system describe pods -l app=ruptura
  echo ""
  echo "--- Pod logs (last 50 lines) ---"
  kubectl -n ruptura-system logs -l app=ruptura --tail=50 || true
  echo ""
  echo "See scripts/lab-verify.sh for a full diagnostic run."
  exit 1
}

echo "==> Waiting for Prometheus..."
kubectl -n monitoring rollout status deployment/prometheus --timeout=120s || true

echo "==> Waiting for Grafana..."
kubectl -n monitoring rollout status deployment/grafana --timeout=120s || true

# ── Port-forwards (background) ───────────────────────────────────────────────
echo "==> Starting port-forwards in background..."
kubectl -n ruptura-system port-forward svc/ruptura       8080:80   --address=0.0.0.0 &
kubectl -n ruptura-system port-forward svc/ruptura-otlp  4317:4317 --address=0.0.0.0 &
kubectl -n monitoring     port-forward svc/prometheus     9090:9090 --address=0.0.0.0 &
kubectl -n monitoring     port-forward svc/grafana        3000:3000 --address=0.0.0.0 &

# Let port-forwards stabilise
sleep 4

# ── Quick smoke test ─────────────────────────────────────────────────────────
echo ""
echo "==> Smoke test: Ruptura health"
curl -sf http://localhost:8080/api/v2/health && echo " OK" || echo " FAILED"

echo "==> Smoke test: Ruptura ready"
curl -sf http://localhost:8080/api/v2/ready  && echo " OK" || echo " FAILED"

echo ""
echo "========================================================"
echo " Ruptura Lab is up!"
echo "  API:        http://localhost:8080"
echo "  OTLP HTTP:  http://localhost:4317"
echo "  Prometheus: http://localhost:9090"
echo "  Grafana:    http://localhost:3000  (admin/admin)"
echo ""
echo "  Run a full diagnostic:  bash scripts/lab-verify.sh"
echo "========================================================"
