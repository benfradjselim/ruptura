# MCP server

`ruptura-mcp` lets an AI assistant ask Ruptura what is failing or about to fail, over the [Model Context Protocol](https://modelcontextprotocol.io). It reads the engine API and offers no tool that changes the cluster.

It ships in the engine image, as `/ruptura-mcp`, from release 8.7.12, and as a binary on each [GitHub release](https://github.com/benfradjselim/ruptura/releases) for Linux and macOS.

## Tools

| Tool | What it answers |
|---|---|
| `list_failures` | Failures Ruptura names now, most urgent first. Filters: `namespace`, `level` (pod, workload, namespace, node), `kind` (predicted or detected), `within_minutes` (predicted failures expected within that time, plus every detected one), `limit`. |
| `failures_for` | Every failure about one workload, pod or node, with evidence and the recommended action. |
| `impact_of` | Which services call a workload and which it calls, from the cluster configuration and from traces when available, with the failures named on it. |
| `coverage` | How many watched pods Ruptura can protect, and which ones it cannot. |
| `engine_status` | Version, edition and readiness of the engine. |

Every tool is marked read-only. Each result carries the engine's own fields: `target`, `failureMode`, `kind`, `state`, `etaSeconds` with its early-late range, `evidence`, `recommendedAction` and `affects`.

Acting on failures (restart, scale, roll back), compound failures that combine several signals, and failures spreading across levels are part of Ruptura Autopilot, not this server.

## Run it in the cluster

The server reads the engine with its API key and asks its own clients for a separate bearer token. In HTTP mode it does not start without one.

```bash
oc -n ruptura-system create secret generic ruptura-mcp-token \
  --from-literal=token="$(openssl rand -hex 32)"
```

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ruptura-mcp
  namespace: ruptura-system
spec:
  replicas: 1
  selector:
    matchLabels: {app: ruptura-mcp}
  template:
    metadata:
      labels: {app: ruptura-mcp}
    spec:
      containers:
        - name: mcp
          image: docker.io/selimbf/ruptura:v8.7.12
          command: ["/ruptura-mcp", "--listen=:8090", "--engine=http://ruptura:8080"]
          env:
            - name: RUPTURA_API_KEY
              valueFrom: {secretKeyRef: {name: ruptura-api-key, key: api-key}}
            - name: RUPTURA_MCP_TOKEN
              valueFrom: {secretKeyRef: {name: ruptura-mcp-token, key: token}}
          ports:
            - {name: mcp, containerPort: 8090}
          readinessProbe:
            httpGet: {path: /healthz, port: mcp}
          resources:
            requests: {cpu: 10m, memory: 32Mi}
            limits: {memory: 128Mi}
          securityContext:
            allowPrivilegeEscalation: false
            runAsNonRoot: true
            capabilities: {drop: [ALL]}
            seccompProfile: {type: RuntimeDefault}
---
apiVersion: v1
kind: Service
metadata:
  name: ruptura-mcp
  namespace: ruptura-system
spec:
  selector: {app: ruptura-mcp}
  ports:
    - {name: mcp, port: 8090, targetPort: mcp}
```

Use the name of your instance in `--engine` and in the API key secret: an instance named `ruptura` serves on `http://ruptura:8080` and keeps its key in the `ruptura-api-key` secret.

Clients connect to `http://ruptura-mcp.ruptura-system:8090/mcp` (streamable HTTP) and send `Authorization: Bearer <token>`. Behind an MCP gateway, register that address and let the gateway hold the token. The server speaks plain HTTP: keep it inside the cluster, or put TLS in front with a route or the gateway, so the token never travels in clear.

## Run it on your desktop

Assistants that start MCP servers as local commands use stdio. Reach the engine through a route or a port-forward:

```bash
oc -n ruptura-system port-forward svc/ruptura 8080:8080
```

```json
{
  "mcpServers": {
    "ruptura": {
      "command": "/usr/local/bin/ruptura-mcp",
      "env": {
        "RUPTURA_URL": "http://localhost:8080",
        "RUPTURA_API_KEY": "<engine API key>"
      }
    }
  }
}
```

## Settings

| Setting | Meaning |
|---|---|
| `--engine`, `RUPTURA_URL` | Engine address. Default `http://ruptura:8080`. |
| `--listen` | Serve streamable HTTP on this address, for example `:8090`. Empty: stdio. |
| `RUPTURA_API_KEY`, `RUPTURA_API_KEY_FILE` | Engine API key, or a file that holds it. |
| `RUPTURA_MCP_TOKEN`, `RUPTURA_MCP_TOKEN_FILE` | Bearer token clients must send in HTTP mode. Required in HTTP mode. |

The server holds no state and keeps no data: each tool call reads the engine.
