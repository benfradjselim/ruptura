#!/usr/bin/env bash
# on-create.sh — run once when the Codespace container is first created.
# Installs k3s and auxiliary CLI tools. Kept idempotent so re-runs are safe.
set -euo pipefail

echo "==> Installing k3s (lightweight Kubernetes)"
curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="server \
  --disable=traefik \
  --write-kubeconfig-mode=644" sh -

# Give k3s a moment to write its kubeconfig
sleep 5

echo "==> Configuring KUBECONFIG"
mkdir -p "$HOME/.kube"
cp /etc/rancher/k3s/k3s.yaml "$HOME/.kube/config"
echo 'export KUBECONFIG=/etc/rancher/k3s/k3s.yaml' >> "$HOME/.bashrc"
echo 'export KUBECONFIG=/etc/rancher/k3s/k3s.yaml' >> "$HOME/.profile"

echo "==> Installing grpcurl (OTLP gRPC testing)"
GRPCURL_VERSION="1.9.1"
curl -sSL "https://github.com/fullstorydev/grpcurl/releases/download/v${GRPCURL_VERSION}/grpcurl_${GRPCURL_VERSION}_linux_amd64.tar.gz" \
  | tar -xz -C /usr/local/bin grpcurl

echo "==> Installing yq (YAML query)"
YQ_VERSION="v4.44.1"
curl -sSL "https://github.com/mikefarah/yq/releases/download/${YQ_VERSION}/yq_linux_amd64" \
  -o /usr/local/bin/yq && chmod +x /usr/local/bin/yq

echo "==> on-create.sh done"
