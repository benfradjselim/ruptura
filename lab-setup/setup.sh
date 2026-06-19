#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════════════
# Ruptura Lab — Full cluster setup script
# Civo K3s cluster, NYC1, 2× Small nodes
#
# Usage:
#   export KUBECONFIG=~/civo-lab-ruptura-kubeconfig
#   bash setup.sh
#
# What this does:
#   1. Generates secrets (API keys, master key)
#   2. Creates namespaces
#   3. Deploys Prometheus + kube-state-metrics + OTel Collector
#   4. Deploys Ruptura community engine + UI
#   5. Deploys 6 synthetic test apps
#   6. Prints access URLs + credentials
# ═══════════════════════════════════════════════════════════════════════════
set -euo pipefail

RED='\033[0;31m'; GRN='\033[0;32m'; YLW='\033[0;33m'
CYN='\033[0;36m'; BLD='\033[1m'; RST='\033[0m'

log()  { echo -e "${GRN}▶${RST} $*"; }
warn() { echo -e "${YLW}⚠${RST}  $*"; }
err()  { echo -e "${RED}✗${RST}  $*"; exit 1; }
info() { echo -e "${CYN}ℹ${RST}  $*"; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ── Preflight ───────────────────────────────────────────────────────────────
log "Preflight checks..."
command -v kubectl >/dev/null 2>&1 || err "kubectl not found. Install it first: https://kubernetes.io/docs/tasks/tools/"
command -v openssl >/dev/null 2>&1 || err "openssl not found"

kubectl cluster-info 2>/dev/null | head -1 || err "Cannot connect to cluster. Check KUBECONFIG."
NODE_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="ExternalIP")].address}' 2>/dev/null | awk '{print $1}')
if [ -z "$NODE_IP" ]; then
  NODE_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[0].address}')
fi
log "Cluster reachable. Node IP: ${BLD}${NODE_IP}${RST}"

# ── Generate secrets ────────────────────────────────────────────────────────
log "Generating secrets..."
RUPTURA_API_KEY=$(openssl rand -hex 32)
RUPTURA_MASTER_KEY=$(openssl rand -hex 32)
RUPTURA_LICENSE_KEY="community-lab-$(openssl rand -hex 8)"

# Save to local file (for your records)
cat > "${SCRIPT_DIR}/.secrets" << EOF
# Ruptura Lab Secrets — generated $(date -u +"%Y-%m-%dT%H:%M:%SZ")
# Keep this file secure — never commit to git
RUPTURA_API_KEY=${RUPTURA_API_KEY}
RUPTURA_MASTER_KEY=${RUPTURA_MASTER_KEY}
NODE_IP=${NODE_IP}

# Access URLs
RUPTURA_API_URL=http://${NODE_IP}:31468
RUPTURA_UI_URL=http://${NODE_IP}:31469
RUPTURA_OTLP_URL=${NODE_IP}:31470
PROMETHEUS_URL=http://${NODE_IP}:$(kubectl get svc prometheus -n monitoring --ignore-not-found -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null || echo "9090")
EOF
chmod 600 "${SCRIPT_DIR}/.secrets"
info "Secrets saved to ${SCRIPT_DIR}/.secrets"

# ── 1. Namespaces ───────────────────────────────────────────────────────────
log "Creating namespaces..."
kubectl apply -f "${SCRIPT_DIR}/00-namespaces.yaml"

# ── 2. Ruptura secrets ──────────────────────────────────────────────────────
log "Creating Ruptura secrets..."
kubectl create secret generic ruptura-secrets \
  --namespace ruptura-system \
  --from-literal=api-key="${RUPTURA_API_KEY}" \
  --from-literal=master-key="${RUPTURA_MASTER_KEY}" \
  --dry-run=client -o yaml | kubectl apply -f -

# ── 3. Monitoring stack ─────────────────────────────────────────────────────
log "Deploying Prometheus + kube-state-metrics..."
kubectl apply -f "${SCRIPT_DIR}/01-prometheus.yaml"

log "Deploying OpenTelemetry Collector..."
kubectl apply -f "${SCRIPT_DIR}/02-otel-collector.yaml"

# ── 4. Ruptura community ────────────────────────────────────────────────────
log "Deploying Ruptura community engine..."
kubectl apply -f "${SCRIPT_DIR}/03-ruptura-community.yaml"

# ── 5. Test apps ────────────────────────────────────────────────────────────
log "Deploying synthetic test applications..."
kubectl apply -f "${SCRIPT_DIR}/05-test-apps.yaml"

# ── Wait for rollout ────────────────────────────────────────────────────────
log "Waiting for Ruptura to be ready..."
kubectl rollout status deployment/ruptura -n ruptura-system --timeout=180s || warn "Ruptura rollout timeout — check logs with: kubectl logs -n ruptura-system deploy/ruptura"

log "Waiting for test apps..."
for app in gateway order-service payment-api cache-worker ml-inference db-proxy; do
  kubectl rollout status deployment/${app} -n test-apps --timeout=120s 2>/dev/null || warn "${app} not ready yet"
done

# ── Get actual NodePorts ────────────────────────────────────────────────────
RUPTURA_PORT=$(kubectl get svc ruptura -n ruptura-system -o jsonpath='{.spec.ports[?(@.name=="api")].nodePort}' 2>/dev/null || echo "31468")
RUPTURA_UI_PORT=$(kubectl get svc ruptura-ui -n ruptura-system -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null || echo "31469")
PROM_PORT=$(kubectl get svc prometheus -n monitoring -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null || echo "?")

# ── Summary ─────────────────────────────────────────────────────────────────
echo ""
echo -e "${BLD}═══════════════════════════════════════════════════════════${RST}"
echo -e "${BLD}  Ruptura Lab — Setup Complete${RST}"
echo -e "${BLD}═══════════════════════════════════════════════════════════${RST}"
echo ""
echo -e "${CYN}Cluster node:${RST}      ${NODE_IP}"
echo ""
echo -e "${GRN}Ruptura Dashboard:${RST} http://${NODE_IP}:${RUPTURA_UI_PORT}"
echo -e "${GRN}Ruptura API:${RST}       http://${NODE_IP}:${RUPTURA_PORT}/api/v2/health"
echo -e "${GRN}Prometheus:${RST}        http://${NODE_IP}:${PROM_PORT}"
echo ""
echo -e "${YLW}API Key:${RST}           ${RUPTURA_API_KEY}"
echo ""
echo -e "${BLD}Test apps deployed:${RST}"
echo -e "  gateway       — stable (healthy baseline)"
echo -e "  order-service — degraded (CPU + memory leak)"
echo -e "  payment-api   — at-risk (high errors + contagion)"
echo -e "  cache-worker  — spike (burst every 2 min)"
echo -e "  ml-inference  — calibrating (new workload)"
echo -e "  db-proxy      — stable (very healthy)"
echo ""
echo -e "${YLW}Wait 5–10 min for baselines to calibrate before checking Fleet.${RST}"
echo ""
echo -e "Secrets saved to: ${SCRIPT_DIR}/.secrets"
echo -e "${BLD}═══════════════════════════════════════════════════════════${RST}"

# ── Quick health check ──────────────────────────────────────────────────────
echo ""
log "Quick health check in 10s..."
sleep 10
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 5 "http://${NODE_IP}:${RUPTURA_PORT}/api/v2/health" 2>/dev/null || echo "000")
if [ "${HTTP_CODE}" = "200" ]; then
  log "Ruptura API is responding ✓"
else
  warn "Ruptura API not responding yet (HTTP ${HTTP_CODE}) — wait 30s and retry: curl http://${NODE_IP}:${RUPTURA_PORT}/api/v2/health"
fi
