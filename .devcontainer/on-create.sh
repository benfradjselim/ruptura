#!/usr/bin/env bash
# on-create.sh — run once when the Codespace container is first created.
# Installs k3s and auxiliary CLI tools. Kept idempotent so re-runs are safe.
set -euo pipefail

echo "==> Installing k3s binary (no systemd)"
curl -sfL https://github.com/k3s-io/k3s/releases/download/v1.29.5+k3s1/k3s \
  -o /usr/local/bin/k3s
chmod +x /usr/local/bin/k3s

echo "==> Creating /dev/kmsg if missing (required by kubelet in containers)"
if [ ! -e /dev/kmsg ]; then
  mknod /dev/kmsg c 1 11 2>/dev/null || true
  chmod 666 /dev/kmsg 2>/dev/null || true
fi

echo "==> Starting k3s server"
mkdir -p /var/lib/rancher/k3s /etc/rancher/k3s
nohup k3s server \
  --snapshotter=native \
  --disable=traefik \
  --disable=servicelb \
  --write-kubeconfig=/etc/rancher/k3s/k3s.yaml \
  --write-kubeconfig-mode=644 \
  > /var/log/k3s.log 2>&1 &

echo "==> Waiting for k3s node to be Ready (up to 90s)..."
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
for i in $(seq 1 18); do
  sleep 5
  kubectl get nodes 2>/dev/null | grep -q " Ready" && break
  echo "    waiting... $((i*5))s"
done

kubectl get nodes 2>/dev/null | grep -q " Ready" \
  && echo "==> k3s node Ready" \
  || echo "!!! k3s not ready yet — check /var/log/k3s.log"

echo "==> Configuring KUBECONFIG"
mkdir -p "$HOME/.kube"
cp /etc/rancher/k3s/k3s.yaml "$HOME/.kube/config" 2>/dev/null || true
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
