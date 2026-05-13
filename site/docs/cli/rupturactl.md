# ruptura-ctl

`ruptura-ctl` **v1.0.0** is the command-line interface for Ruptura. It runs **outside the pod** and speaks to the Ruptura REST API — the same API the dashboard uses. You point it at any running instance (local Docker, Kubernetes service, or OpenShift Route) with `--url` or the `RUPTURA_URL` environment variable.

!!! info "Independent versioning"
    `ruptura-ctl` is versioned **independently** from the server. You can run `ruptura-ctl v1.0.0` against any `ruptura >= v6.8.x`. Check your CLI version with `ruptura-ctl version`.

---

## Install

### Binary (recommended)

Download a pre-built binary from the [GitHub Releases page](https://github.com/benfradjselim/ruptura/releases):

=== "Linux (amd64)"

    ```bash
    curl -Lo ruptura-ctl \
      https://github.com/benfradjselim/ruptura/releases/latest/download/ruptura-ctl-linux-amd64
    chmod +x ruptura-ctl
    sudo mv ruptura-ctl /usr/local/bin/
    ruptura-ctl version
    # ruptura-ctl v1.0.0
    ```

=== "Linux (arm64)"

    ```bash
    curl -Lo ruptura-ctl \
      https://github.com/benfradjselim/ruptura/releases/latest/download/ruptura-ctl-linux-arm64
    chmod +x ruptura-ctl
    sudo mv ruptura-ctl /usr/local/bin/
    ```

=== "macOS (Apple Silicon)"

    ```bash
    curl -Lo ruptura-ctl \
      https://github.com/benfradjselim/ruptura/releases/latest/download/ruptura-ctl-darwin-arm64
    chmod +x ruptura-ctl
    sudo mv ruptura-ctl /usr/local/bin/
    ```

=== "macOS (Intel)"

    ```bash
    curl -Lo ruptura-ctl \
      https://github.com/benfradjselim/ruptura/releases/latest/download/ruptura-ctl-darwin-amd64
    chmod +x ruptura-ctl
    sudo mv ruptura-ctl /usr/local/bin/
    ```

Verify:

```bash
ruptura-ctl version
# ruptura-ctl v1.0.0
```

### Go install

Requires Go 1.21+:

```bash
go install github.com/benfradjselim/ruptura/cmd/ruptura-ctl@latest
```

### Build from source

```bash
git clone https://github.com/benfradjselim/ruptura.git
cd ruptura/workdir
go build -o ruptura-ctl ./cmd/ruptura-ctl
sudo mv ruptura-ctl /usr/local/bin/
```

### kubectl plugin

Install `ruptura-ctl` as a `kubectl` plugin so you can invoke it as `kubectl ruptura`:

```bash
# Download the binary
curl -Lo kubectl-ruptura \
  https://github.com/benfradjselim/ruptura/releases/latest/download/ruptura-ctl-linux-amd64
chmod +x kubectl-ruptura
sudo mv kubectl-ruptura /usr/local/bin/

# Verify kubectl picks it up
kubectl ruptura version
```

kubectl discovers plugins by looking for executables named `kubectl-<name>` anywhere on your `PATH`. The dash in `kubectl-ruptura` maps to the space in `kubectl ruptura`.

---

## Connect to a running instance

`ruptura-ctl` connects to the Ruptura API via HTTP. Set `RUPTURA_URL` (and optionally `RUPTURA_API_KEY`) before running any command.

### Kubernetes — NodePort

If Ruptura is exposed via NodePort (default in the Helm chart):

```bash
export RUPTURA_URL=http://<node-ip>:<nodeport>
export RUPTURA_API_KEY=<your-api-key>

ruptura-ctl version
ruptura-ctl health
ruptura-ctl status
```

### Kubernetes — port-forward

Forward the Ruptura service to localhost:

```bash
# Terminal 1 — keep this running
kubectl port-forward svc/ruptura 8080:80 -n ruptura-system

# Terminal 2 — use the CLI
export RUPTURA_URL=http://localhost:8080
export RUPTURA_API_KEY=<your-api-key>

ruptura-ctl status
ruptura-ctl get workloads
```

Retrieve the API key from the cluster secret if you don't have it locally:

```bash
export RUPTURA_API_KEY=$(
  kubectl get secret ruptura-secret \
    -n ruptura-system \
    -o jsonpath='{.data.api-key}' | base64 -d
)
```

### Kubernetes — in-cluster Job

Run a one-shot `ruptura-ctl` command as a Kubernetes Job, useful in CI pipelines or GitOps workflows:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: ruptura-ctl-check
  namespace: ruptura-system
spec:
  ttlSecondsAfterFinished: 120
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: ctl
          image: ghcr.io/benfradjselim/ruptura-ctl:latest
          command: ["ruptura-ctl", "status"]
          env:
            - name: RUPTURA_URL
              value: "http://ruptura.ruptura-system.svc.cluster.local:80"
            - name: RUPTURA_API_KEY
              valueFrom:
                secretKeyRef:
                  name: ruptura-secret
                  key: api-key
```

```bash
kubectl apply -f ruptura-ctl-job.yaml
kubectl logs -n ruptura-system job/ruptura-ctl-check -f
```

### OpenShift — Route access

On OpenShift the operator automatically creates a Route with edge TLS. Use the Route hostname directly:

```bash
ROUTE=$(oc get route ruptura -n ruptura-system -o jsonpath='{.spec.host}')

export RUPTURA_URL=https://$ROUTE
export RUPTURA_API_KEY=$(
  oc get secret ruptura-secret \
    -n ruptura-system \
    -o jsonpath='{.data.api-key}' | base64 -d
)

ruptura-ctl status
```

### Docker (local)

```bash
export RUPTURA_URL=http://localhost:8080
export RUPTURA_API_KEY=<your-api-key>
ruptura-ctl status
```

---

## Configuration

All flags can be set via environment variables:

| Flag | Env var | Default | Description |
|------|---------|---------|-------------|
| `--url` | `RUPTURA_URL` | `http://localhost:8080` | Ruptura API base URL |
| `--api-key` | `RUPTURA_API_KEY` | _(none)_ | Bearer token |
| `--output` | — | `table` | Output format: `table` \| `json` \| `wide` |
| `--namespace` | — | _(all)_ | Filter by namespace |
| `--timeout` | — | `15` | Request timeout (seconds) |
| `--no-color` | — | `false` | Disable ANSI colors |

### Shell completion

```bash
# Bash
ruptura-ctl completion bash > /etc/bash_completion.d/ruptura-ctl

# Zsh
ruptura-ctl completion zsh > "${fpath[1]}/_ruptura-ctl"

# Fish
ruptura-ctl completion fish | source
```

---

## Command reference

### `version`

Print the CLI version (independently versioned from the server):

```bash
ruptura-ctl version
# ruptura-ctl v1.0.0
```

### `health`

Raw server health, version, uptime, and ingestion counters:

```bash
ruptura-ctl health
ruptura-ctl health -o json
```

Example output:

```
Server:   ruptura v6.8.13 (community)
Uptime:   14h32m
Status:   ok

Ingest (samples received)
  metrics (Prometheus/OTLP)  840 200
  logs    (OTLP :4317)        13 826
  traces  (OTLP :4317)        42 131
```

### `status`

Show health and a summary of all monitored workloads:

```bash
ruptura-ctl status
ruptura-ctl status -n production
```

### `get`

List resources:

```bash
ruptura-ctl get workloads               # all workloads
ruptura-ctl get workloads -n production # filter by namespace
ruptura-ctl get ruptures                # active rupture events
ruptura-ctl get actions                 # pending and recent actions
ruptura-ctl get suppressions            # active maintenance windows
ruptura-ctl get anomalies               # detected anomalies
```

Add `-o json` to any command for machine-readable output:

```bash
ruptura-ctl get workloads -o json | jq '.[].health_score.value'
```

### `describe`

Full KPI snapshot for a single workload:

```bash
ruptura-ctl describe workload production/Deployment/payment-api
```

### `explain`

Human-readable narrative explanation of a rupture event:

```bash
ruptura-ctl explain rpt_abc123
ruptura-ctl explain production/Deployment/payment-api
```

### `actions`

Manage Tier-2 (human-approval) actions:

```bash
ruptura-ctl actions                    # list pending actions
ruptura-ctl actions approve act_abc123
ruptura-ctl actions reject  act_abc123
ruptura-ctl actions emergency-stop     # halt all action dispatch immediately
```

### `suppress`

Create and manage maintenance windows to mute action dispatch during planned downtime:

```bash
# Suppress a single workload for 30 minutes
ruptura-ctl suppress create "production/Deployment/payment-api" 30m \
  --reason "rolling deploy"

# Suppress an entire namespace for 1 hour
ruptura-ctl suppress create "production/*" 1h --reason "k8s cluster upgrade"

ruptura-ctl suppress list
ruptura-ctl suppress delete <id>
```

### `weights`

View or override per-workload signal weights used to compute HealthScore:

```bash
ruptura-ctl weights get
ruptura-ctl weights set \
  --selector "production/*" \
  --stress 0.4 --fatigue 0.25 --mood 0.15 \
  --pressure 0.10 --humidity 0.05 --contagion 0.05
```

### `sim`

Inject synthetic failure patterns for testing and demos:

```bash
ruptura-ctl sim patterns                     # list available patterns
ruptura-ctl sim inject cascade-failure       # default workload, 60s
ruptura-ctl sim inject memory-leak \
  --workload production/Deployment/payment-api \
  --duration 120
```

Available patterns: `cascade-failure` · `memory-leak` · `traffic-surge` · `slow-burn`

### `completion`

Generate shell completion:

```bash
ruptura-ctl completion bash
ruptura-ctl completion zsh
ruptura-ctl completion fish
ruptura-ctl completion powershell
```
