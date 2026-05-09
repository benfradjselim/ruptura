#!/usr/bin/env bash
# post-create.sh — deploys the full Ruptura observability lab into k3s.
#
# Stack:
#   ruptura-system: Ruptura (REST :8080, OTLP :4317, BadgerDB PVC)
#   monitoring:     Prometheus, Grafana, OpenTelemetry Collector
#   test-workloads: nginx (3), podinfo (2), stress-app, load-generator
#
# Data flow:
#   load-generator → otel-collector:4318 → ruptura:4317 (OTLP/HTTP)
#   k8s_cluster receiver → otel-collector → ruptura:4317
#   prometheus scrapes ruptura:80/api/v2/metrics every 15s
#   grafana reads prometheus datasource
#
set -euo pipefail
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

log() { echo "==> $*"; }
die() { echo "!!! $*" >&2; exit 1; }

# ── 1. Wait for k3s node ─────────────────────────────────────────────────────
log "Waiting for k3s node to be Ready..."
for i in $(seq 1 30); do
  kubectl get nodes 2>/dev/null | grep -q " Ready" && break
  sleep 5
done
kubectl get nodes | grep -q " Ready" || die "k3s node never became Ready"
log "k3s node Ready."

# ── 2. Namespaces ─────────────────────────────────────────────────────────────
for NS in ruptura-system monitoring test-workloads; do
  kubectl create namespace "$NS" --dry-run=client -o yaml | kubectl apply -f -
done

# ── 3. Ruptura API-key secret (lab value — not secret) ───────────────────────
kubectl -n ruptura-system create secret generic ruptura-secrets \
  --from-literal=api-key=ruptura-lab-key \
  --dry-run=client -o yaml | kubectl apply -f -

# ── 4. Deploy Ruptura ─────────────────────────────────────────────────────────
log "Deploying Ruptura (image: ghcr.io/benfradjselim/ruptura:6.7.0)..."
kubectl apply -f workdir/deploy/rbac.yaml
kubectl apply -f workdir/deploy/configmap.yaml
kubectl apply -f workdir/deploy/pvc.yaml
kubectl apply -f workdir/deploy/central-deployment.yaml

# ── 5. Deploy monitoring stack ────────────────────────────────────────────────
log "Deploying Prometheus..."
kubectl apply -f workdir/deploy/prometheus.yaml

log "Deploying Grafana..."
kubectl apply -f workdir/deploy/grafana.yaml

log "Deploying OpenTelemetry Collector..."
kubectl apply -f workdir/deploy/otel-collector.yaml

# ── 6. Deploy test workloads ──────────────────────────────────────────────────
log "Deploying test workloads (nginx, podinfo, stress-app, load-generator)..."
kubectl apply -f workdir/deploy/test-workloads.yaml

# ── 7. Wait for rollouts ──────────────────────────────────────────────────────
log "Waiting for Ruptura rollout (up to 4 min)..."
kubectl -n ruptura-system rollout status deployment/ruptura --timeout=240s || {
  echo ""
  echo "!!! Ruptura pod did not become ready. Crash diagnostics:"
  echo ""
  echo "--- Pod status ---"
  kubectl -n ruptura-system get pods -o wide
  echo ""
  echo "--- Events ---"
  kubectl -n ruptura-system get events --sort-by='.lastTimestamp' | tail -20
  echo ""
  echo "--- Logs ---"
  POD=$(kubectl -n ruptura-system get pod -l app=ruptura -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
  if [ -n "$POD" ]; then
    kubectl -n ruptura-system logs "$POD" --tail=60 2>/dev/null \
      || kubectl -n ruptura-system logs "$POD" --previous --tail=60 2>/dev/null \
      || echo "(no logs)"
  fi
  echo ""
  echo "Run: bash scripts/lab-verify.sh --diag"
  exit 1
}

log "Waiting for Prometheus..."
kubectl -n monitoring rollout status deployment/prometheus --timeout=120s || true

log "Waiting for Grafana..."
kubectl -n monitoring rollout status deployment/grafana --timeout=120s || true

log "Waiting for OTel Collector..."
kubectl -n monitoring rollout status deployment/otel-collector --timeout=120s || true

log "Waiting for test workloads..."
kubectl -n test-workloads rollout status deployment/nginx         --timeout=120s || true
kubectl -n test-workloads rollout status deployment/podinfo       --timeout=120s || true
kubectl -n test-workloads rollout status deployment/load-generator --timeout=120s || true

# ── 8. Port-forwards ─────────────────────────────────────────────────────────
log "Starting port-forwards..."
# Kill any stale port-forwards from a previous run
pkill -f "kubectl.*port-forward" 2>/dev/null || true
sleep 1

kubectl -n ruptura-system port-forward svc/ruptura       8080:80   --address=0.0.0.0 &
kubectl -n ruptura-system port-forward svc/ruptura-otlp  4317:4317 --address=0.0.0.0 &
kubectl -n monitoring     port-forward svc/prometheus     9090:9090 --address=0.0.0.0 &
kubectl -n monitoring     port-forward svc/grafana        3000:3000 --address=0.0.0.0 &
sleep 5

# ── 9. Quick smoke test ───────────────────────────────────────────────────────
echo ""
log "Smoke test — Ruptura health:"
curl -sf http://localhost:8080/api/v2/health && echo " OK" || echo " FAILED (pod may still be starting)"

log "Smoke test — Ruptura ready:"
curl -sf http://localhost:8080/api/v2/ready  && echo " OK" || echo " FAILED"

log "Smoke test — Ruptura version:"
curl -sf http://localhost:8080/api/v2/version 2>/dev/null | python3 -c "import sys,json; d=json.load(sys.stdin); print(' version:', d.get('version','?'))" 2>/dev/null || echo " (version endpoint varies)"

# ── 10. Seed initial OTLP data so Ruptura has something to show ───────────────
log "Seeding initial OTLP data into Ruptura..."
seed_otlp() {
  SVC=$1; TS=$(date +%s)000000000
  curl -sf -X POST "http://localhost:4317/otlp/v1/metrics" \
    -H "Content-Type: application/json" \
    -d "{\"resourceMetrics\":[{\"resource\":{\"attributes\":[
          {\"key\":\"service.name\",\"value\":{\"stringValue\":\"$SVC\"}},
          {\"key\":\"k8s.namespace.name\",\"value\":{\"stringValue\":\"test-workloads\"}},
          {\"key\":\"k8s.deployment.name\",\"value\":{\"stringValue\":\"$SVC\"}}
        ]},\"scopeMetrics\":[{\"scope\":{\"name\":\"seed\"},\"metrics\":[
          {\"name\":\"cpu_usage\",\"gauge\":{\"dataPoints\":[{\"asDouble\":0.45,\"timeUnixNano\":\"$TS\"}]}},
          {\"name\":\"memory_usage_mb\",\"gauge\":{\"dataPoints\":[{\"asDouble\":180,\"timeUnixNano\":\"$TS\"}]}},
          {\"name\":\"http_error_rate\",\"gauge\":{\"dataPoints\":[{\"asDouble\":0.02,\"timeUnixNano\":\"$TS\"}]}}
        ]}]}]}" > /dev/null 2>&1 && echo "  seeded: $SVC" || echo "  seed skipped: $SVC (OTLP port-forward not ready)"
}
for SVC in nginx podinfo stress-app; do seed_otlp "$SVC"; done

# ── 11. Print access URLs ─────────────────────────────────────────────────────
echo ""
echo "════════════════════════════════════════════════════════════════"
echo " Ruptura Lab — ready"
echo "════════════════════════════════════════════════════════════════"
echo ""

if [ -n "${CODESPACE_NAME:-}" ]; then
  BASE="https://${CODESPACE_NAME}"
  echo " Ruptura API  → ${BASE}-8080.app.github.dev"
  echo " Ruptura OTLP → ${BASE}-4317.app.github.dev  (OTLP/HTTP)"
  echo " Prometheus   → ${BASE}-9090.app.github.dev"
  echo " Grafana      → ${BASE}-3000.app.github.dev  (admin / admin)"
  echo ""
  echo " Direct links (open in browser):"
  echo "   ${BASE}-8080.app.github.dev/api/v2/health"
  echo "   ${BASE}-8080.app.github.dev/api/v2/workloads"
  echo "   ${BASE}-8080.app.github.dev/api/v2/metrics"
  echo "   ${BASE}-9090.app.github.dev/graph?g0.expr=ruptura_kpi"
  echo "   ${BASE}-3000.app.github.dev"
else
  echo " Ruptura API  → http://localhost:8080"
  echo " Ruptura OTLP → http://localhost:4317  (OTLP/HTTP)"
  echo " Prometheus   → http://localhost:9090"
  echo " Grafana      → http://localhost:3000  (admin / admin)"
fi

echo ""
echo " API key: ruptura-lab-key"
echo " Example: curl -H 'Authorization: Bearer ruptura-lab-key' \\"
echo "          http://localhost:8080/api/v2/workloads"
echo ""
echo " Full diagnostic: bash scripts/lab-verify.sh"
echo " Watch live data: bash scripts/lab-verify.sh --otlp"
echo ""
echo " Test workloads running in namespace: test-workloads"
echo "   kubectl get pods -n test-workloads"
echo "   kubectl logs -n test-workloads -l app=load-generator -f"
echo "════════════════════════════════════════════════════════════════"
