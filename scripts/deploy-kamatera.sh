#!/usr/bin/env bash
# deploy-kamatera.sh — Deploy or upgrade Ruptura on the Kamatera lab.
# Usage: bash scripts/deploy-kamatera.sh [--tag v7.0.5] [--ui] [--demo-workloads]
set -euo pipefail

CHART="oci://ghcr.io/benfradjselim/charts/ruptura"
RELEASE="ruptura"
NAMESPACE="ruptura-system"
TAG="${RUPTURA_TAG:-v7.0.5}"
WITH_UI=false
WITH_DEMO=false

for arg in "$@"; do
  case $arg in
    --tag=*) TAG="${arg#*=}" ;;
    --ui) WITH_UI=true ;;
    --demo-workloads) WITH_DEMO=true ;;
  esac
done

echo "==> Deploying Ruptura ${TAG} to namespace ${NAMESPACE}"
echo "    Chart: ${CHART}"
echo "    UI enabled: ${WITH_UI}"

# Check kubectl is available
kubectl cluster-info >/dev/null 2>&1 || { echo "ERROR: kubectl not connected to cluster"; exit 1; }

# Create namespace if needed
kubectl get namespace "${NAMESPACE}" >/dev/null 2>&1 || kubectl create namespace "${NAMESPACE}"

# Remove disk-pressure taint if present (safety)
NODE=$(kubectl get nodes -o jsonpath='{.items[0].metadata.name}')
if kubectl get node "${NODE}" -o jsonpath='{.spec.taints}' | grep -q "disk-pressure"; then
  echo "==> Removing disk-pressure taint from ${NODE}"
  kubectl taint node "${NODE}" node.kubernetes.io/disk-pressure:NoSchedule- 2>/dev/null || true
fi

# Helm upgrade/install
HELM_ARGS=(
  upgrade --install "${RELEASE}" "${CHART}"
  --namespace "${NAMESPACE}"
  --set "image.tag=${TAG}"
  --set "image.pullPolicy=Always"
  --set "service.type=NodePort"
  --set "otlpService.type=NodePort"
  --set "otlpNodePort=31470"
  --wait
  --timeout=5m
)

if [[ "$WITH_UI" == "true" ]]; then
  HELM_ARGS+=(
    --set "ui.enabled=true"
    --set "ui.image.tag=${TAG}"
    --set "ui.service.type=NodePort"
    --set "ui.nodePort=31380"
  )
fi

helm "${HELM_ARGS[@]}"

echo ""
echo "==> Deployment complete. Pods:"
kubectl get pods -n "${NAMESPACE}" -o wide

echo ""
echo "==> Services:"
kubectl get svc -n "${NAMESPACE}"

# Smoke test
echo ""
echo "==> Smoke testing API..."
API_PORT=$(kubectl get svc -n "${NAMESPACE}" "${RELEASE}" -o jsonpath='{.spec.ports[?(@.name=="api")].nodePort}' 2>/dev/null || echo 31080)
NODE_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}')
RUPTURA_URL="http://${NODE_IP}:${API_PORT}"

for i in {1..30}; do
  if curl -sf "${RUPTURA_URL}/api/v2/health" >/dev/null 2>&1; then
    echo "    PASS health: 200 OK"
    break
  fi
  sleep 2
done || echo "    WARN: health check timed out"

curl -sf "${RUPTURA_URL}/api/v2/health" 2>/dev/null | python3 -m json.tool 2>/dev/null || true

# Deploy demo workloads if requested
if [[ "$WITH_DEMO" == "true" ]]; then
  echo ""
  echo "==> Deploying demo workloads..."
  SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
  kubectl apply -f "${SCRIPT_DIR}/../deploy/demo-workloads.yaml"
  echo "    Demo workloads deployed in namespace: ruptura-demo"
  kubectl get pods -n ruptura-demo
fi

echo ""
echo "==> Done. Ruptura UI: http://185.229.225.115:31380"
echo "    API:             http://185.229.225.115:${API_PORT}"
