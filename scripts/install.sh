#!/bin/bash
# MLOps Anomaly Detection - One-Click Helm Installation
set -euo pipefail

NAMESPACE="mlops"
RELEASE="mlops-anomaly"
CHART_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/helm"
IMAGE_REPO="${IMAGE_REPO:-mlops-anomaly}"
IMAGE_TAG="${IMAGE_TAG:-latest}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log() { echo -e "${BLUE}[INFO]${NC} $1"; }
ok()  { echo -e "${GREEN}[OK]${NC}  $1"; }
warn(){ echo -e "${YELLOW}[WARN]${NC} $1"; }
err() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# ---- Input validation ----
_validate_tag() {
    local val="$1" name="$2"
    if ! [[ "$val" =~ ^[a-zA-Z0-9._/-]+$ ]]; then
        err "$name contains invalid characters. Only [a-zA-Z0-9._/-] are allowed."
    fi
}
_validate_tag "$IMAGE_REPO" "IMAGE_REPO"
_validate_tag "$IMAGE_TAG"  "IMAGE_TAG"

# ---- Prerequisites ----
for cmd in kubectl helm docker; do
    command -v "$cmd" &>/dev/null || err "$cmd not found. Please install $cmd."
done
ok "Prerequisites checked"

# ---- Build Docker images ----
log "Building Docker images..."
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

for service in collector processor trainer detector exporter dashboard; do
    log "  Building $service..."
    docker build \
        --build-arg SERVICE="$service" \
        -t "${IMAGE_REPO}-${service}:${IMAGE_TAG}" \
        -f "${PROJECT_ROOT}/Dockerfile" \
        "${PROJECT_ROOT}" \
        --quiet
    ok "  ${IMAGE_REPO}-${service}:${IMAGE_TAG} built"
done

# ---- Load images to cluster (for kind/minikube) ----
if command -v kind &>/dev/null; then
    CLUSTER=$(kind get clusters 2>/dev/null | head -1)
    if [ -n "$CLUSTER" ]; then
        log "Loading images into kind cluster '$CLUSTER'..."
        for service in collector processor trainer detector exporter dashboard; do
            kind load docker-image "${IMAGE_REPO}-${service}:${IMAGE_TAG}" --name "$CLUSTER" 2>/dev/null || true
        done
        ok "Images loaded into kind"
    fi
elif command -v minikube &>/dev/null && minikube status &>/dev/null 2>&1; then
    log "Loading images into minikube..."
    for service in collector processor trainer detector exporter dashboard; do
        minikube image load "${IMAGE_REPO}-${service}:${IMAGE_TAG}" 2>/dev/null || true
    done
    ok "Images loaded into minikube"
fi

# ---- Helm install/upgrade ----
log "Installing Helm chart '$RELEASE' in namespace '$NAMESPACE'..."

helm upgrade --install "$RELEASE" "$CHART_DIR" \
    --namespace "$NAMESPACE" \
    --create-namespace \
    --set image.repository="$IMAGE_REPO" \
    --set image.tag="$IMAGE_TAG" \
    --set image.pullPolicy=IfNotPresent \
    --wait \
    --timeout 5m

ok "Helm chart installed"

# ---- Wait for pods ----
log "Waiting for pods to be ready..."
kubectl wait --for=condition=ready pod \
    -l "app.kubernetes.io/name=mlops-anomaly-detection" \
    -n "$NAMESPACE" \
    --timeout=300s 2>/dev/null || \
kubectl get pods -n "$NAMESPACE"

# ---- Display URLs ----
echo ""
echo -e "${GREEN}============================================${NC}"
echo -e "${GREEN}  MLOps Anomaly Detection - INSTALLED       ${NC}"
echo -e "${GREEN}============================================${NC}"
echo ""

# Dashboard (NodePort)
DASHBOARD_PORT=$(kubectl get svc dashboard -n "$NAMESPACE" \
    -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null || echo "8501")

NODE_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}' 2>/dev/null || echo "localhost")

echo -e "  ${BLUE}Dashboard:${NC}   http://${NODE_IP}:${DASHBOARD_PORT}"
echo -e "  ${BLUE}Collector:${NC}   kubectl port-forward svc/collector 8001:8001 -n $NAMESPACE"
echo -e "  ${BLUE}Processor:${NC}   kubectl port-forward svc/processor 8002:8002 -n $NAMESPACE"
echo -e "  ${BLUE}Trainer:${NC}     kubectl port-forward svc/trainer 8003:8003 -n $NAMESPACE"
echo -e "  ${BLUE}Detector:${NC}    kubectl port-forward svc/detector 8004:8004 -n $NAMESPACE"
echo -e "  ${BLUE}Exporter:${NC}    kubectl port-forward svc/exporter 8005:8005 -n $NAMESPACE"
echo ""
echo -e "  ${BLUE}Logs:${NC}        kubectl logs -f deploy/collector -n $NAMESPACE"
echo -e "  ${BLUE}Status:${NC}      kubectl get pods -n $NAMESPACE"
echo -e "  ${BLUE}Uninstall:${NC}   helm uninstall $RELEASE -n $NAMESPACE"
echo ""
