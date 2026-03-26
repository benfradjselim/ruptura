#!/bin/bash
set -euo pipefail

NAMESPACE="mlops"
IMAGE_REPO="${IMAGE_REPO:-selimbf}"
IMAGE_TAG="${IMAGE_TAG:-v2.0}"

echo "Deploying MLOps V2.0..."

for cmd in kubectl docker; do
    command -v "$cmd" &>/dev/null || { echo "ERROR: $cmd not found"; exit 1; }
done

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "Building Docker images..."
for service in collector processor trainer detector exporter dashboard; do
    echo "  Building $service..."
    docker build \
        --build-arg SERVICE="$service" \
        -t "${IMAGE_REPO}/mlops-${service}:${IMAGE_TAG}" \
        -f "${PROJECT_ROOT}/Dockerfile.v2" \
        "${PROJECT_ROOT}" > /dev/null 2>&1
done

echo "Deploying to Kubernetes..."
kubectl apply -f "${PROJECT_ROOT}/manifests/mlops-v2.0.yaml"

echo "Waiting for pods..."
sleep 10
kubectl wait --for=condition=ready pod -l app -n "$NAMESPACE" --timeout=300s 2>/dev/null || true

kubectl get pods -n "$NAMESPACE"

DASHBOARD_PORT=$(kubectl get svc dashboard -n "$NAMESPACE" -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null || echo "30851")
NODE_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}' 2>/dev/null || echo "localhost")

echo ""
echo "Deployment complete!"
echo "Dashboard: http://${NODE_IP}:${DASHBOARD_PORT}"
