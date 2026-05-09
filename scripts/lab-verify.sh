#!/usr/bin/env bash
# lab-verify.sh — end-to-end diagnostic + smoke test for the Ruptura Codespace lab.
#
# Usage:
#   bash scripts/lab-verify.sh           # full run
#   bash scripts/lab-verify.sh --diag    # crash diagnostics only
#   bash scripts/lab-verify.sh --otlp    # OTLP + Prometheus check only
#
set -euo pipefail

export KUBECONFIG="${KUBECONFIG:-/etc/rancher/k3s/k3s.yaml}"

RUPTURA_URL="${RUPTURA_URL:-http://localhost:8080}"
OTLP_URL="${OTLP_URL:-http://localhost:4317}"
PROM_URL="${PROM_URL:-http://localhost:9090}"
API_KEY="${RUPTURA_API_KEY:-ruptura-lab-insecure-key}"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
PASS=0; FAIL=0

ok()   { echo -e "${GREEN}  PASS${NC}  $*"; (( PASS++ )); }
fail() { echo -e "${RED}  FAIL${NC}  $*"; (( FAIL++ )); }
warn() { echo -e "${YELLOW}  WARN${NC}  $*"; }
hdr()  { echo ""; echo "── $* ──────────────────────────────────────────"; }

# ── 1. Pod health ────────────────────────────────────────────────────────────
pod_diagnostics() {
  hdr "Pod status (ruptura-system)"
  kubectl -n ruptura-system get pods -o wide || true

  hdr "Pod crash analysis"
  POD=$(kubectl -n ruptura-system get pod -l app=ruptura -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
  if [[ -z "$POD" ]]; then
    fail "No ruptura pod found"
    return
  fi

  PHASE=$(kubectl -n ruptura-system get pod "$POD" -o jsonpath='{.status.phase}')
  READY=$(kubectl -n ruptura-system get pod "$POD" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}')
  RESTARTS=$(kubectl -n ruptura-system get pod "$POD" -o jsonpath='{.status.containerStatuses[0].restartCount}')

  echo "  Pod:      $POD"
  echo "  Phase:    $PHASE"
  echo "  Ready:    $READY"
  echo "  Restarts: $RESTARTS"

  if [[ "$PHASE" == "Running" && "$READY" == "True" ]]; then
    ok "Pod is Running and Ready"
  else
    fail "Pod not Ready — Phase=$PHASE Ready=$READY Restarts=$RESTARTS"
  fi

  # Crash reason
  WAITING_REASON=$(kubectl -n ruptura-system get pod "$POD" \
    -o jsonpath='{.status.containerStatuses[0].state.waiting.reason}' 2>/dev/null || echo "")
  LAST_REASON=$(kubectl -n ruptura-system get pod "$POD" \
    -o jsonpath='{.status.containerStatuses[0].lastState.terminated.reason}' 2>/dev/null || echo "")
  LAST_EXIT=$(kubectl -n ruptura-system get pod "$POD" \
    -o jsonpath='{.status.containerStatuses[0].lastState.terminated.exitCode}' 2>/dev/null || echo "")

  if [[ -n "$WAITING_REASON" ]]; then
    fail "Container waiting: reason=$WAITING_REASON"
  fi
  if [[ -n "$LAST_REASON" ]]; then
    warn "Last termination: reason=$LAST_REASON exitCode=$LAST_EXIT"
  fi

  hdr "Recent pod logs"
  kubectl -n ruptura-system logs "$POD" --tail=60 2>/dev/null || \
    kubectl -n ruptura-system logs "$POD" --previous --tail=60 2>/dev/null || \
    echo "  (no logs available)"

  hdr "Pod events"
  kubectl -n ruptura-system get events \
    --field-selector "involvedObject.name=$POD" \
    --sort-by='.lastTimestamp' | tail -20 || true
}

# ── 2. REST API checks ───────────────────────────────────────────────────────
api_checks() {
  hdr "Ruptura REST API ($RUPTURA_URL)"

  if curl -sf "$RUPTURA_URL/api/v2/health" -o /dev/null; then
    ok "GET /api/v2/health → 200"
  else
    fail "GET /api/v2/health — not reachable (is port-forward running?)"
    return
  fi

  if curl -sf "$RUPTURA_URL/api/v2/ready" -o /dev/null; then
    ok "GET /api/v2/ready → 200"
  else
    fail "GET /api/v2/ready"
  fi

  METRICS=$(curl -sf "$RUPTURA_URL/api/v2/metrics" || echo "")
  if echo "$METRICS" | grep -q "ruptura_kpi"; then
    ok "GET /api/v2/metrics → contains ruptura_kpi"
  else
    warn "GET /api/v2/metrics — ruptura_kpi not yet present (send OTLP data first)"
  fi

  WORKLOADS=$(curl -sf -H "Authorization: Bearer $API_KEY" \
    "$RUPTURA_URL/api/v2/workloads" 2>/dev/null || echo "{}")
  if echo "$WORKLOADS" | grep -q "\["; then
    ok "GET /api/v2/workloads → authenticated list returned"
  else
    warn "GET /api/v2/workloads → empty or unauthenticated"
  fi
}

# ── 3. OTLP ingest ───────────────────────────────────────────────────────────
send_otlp_metrics() {
  hdr "OTLP metrics ingest ($OTLP_URL)"

  NOW_NS=$(date +%s)000000000

  PAYLOAD=$(cat <<EOF
{
  "resourceMetrics": [{
    "resource": {
      "attributes": [
        {"key": "host.name",    "value": {"stringValue": "lab-host"}},
        {"key": "service.name", "value": {"stringValue": "demo-app"}}
      ]
    },
    "scopeMetrics": [{
      "scope": {"name": "lab-verify"},
      "metrics": [
        {
          "name": "http_requests_total",
          "sum": {
            "dataPoints": [{
              "asInt": "142",
              "timeUnixNano": "$NOW_NS",
              "attributes": [
                {"key": "method", "value": {"stringValue": "GET"}},
                {"key": "status", "value": {"stringValue": "200"}}
              ]
            }],
            "isMonotonic": true
          }
        },
        {
          "name": "cpu_usage",
          "gauge": {
            "dataPoints": [{
              "asDouble": 0.73,
              "timeUnixNano": "$NOW_NS",
              "attributes": [
                {"key": "host", "value": {"stringValue": "lab-host"}}
              ]
            }]
          }
        },
        {
          "name": "memory_usage_bytes",
          "gauge": {
            "dataPoints": [{
              "asDouble": 314572800,
              "timeUnixNano": "$NOW_NS"
            }]
          }
        }
      ]
    }]
  }]
}
EOF
)

  STATUS=$(curl -sf -o /dev/null -w "%{http_code}" \
    -X POST "$OTLP_URL/otlp/v1/metrics" \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD" 2>/dev/null || echo "000")

  if [[ "$STATUS" == "200" ]]; then
    ok "POST /otlp/v1/metrics → 200 (3 data points sent)"
  else
    fail "POST /otlp/v1/metrics → HTTP $STATUS"
  fi
}

send_otlp_traces() {
  hdr "OTLP traces ingest ($OTLP_URL)"

  NOW_NS=$(date +%s)000000000
  END_NS=$(( $(date +%s) + 1 ))000000000

  PAYLOAD=$(cat <<EOF
{
  "resourceSpans": [{
    "resource": {
      "attributes": [
        {"key": "service.name", "value": {"stringValue": "demo-app"}},
        {"key": "host.name",    "value": {"stringValue": "lab-host"}}
      ]
    },
    "scopeSpans": [{
      "scope": {"name": "lab-verify"},
      "spans": [
        {
          "traceId":       "aabbccddeeff00112233445566778899",
          "spanId":        "aabbccdd11223344",
          "parentSpanId":  "",
          "name":          "HTTP GET /api/v2/health",
          "startTimeUnixNano": "$NOW_NS",
          "endTimeUnixNano":   "$END_NS",
          "status": {"code": 1},
          "attributes": [
            {"key": "http.method",      "value": {"stringValue": "GET"}},
            {"key": "http.status_code", "value": {"intValue": "200"}}
          ]
        },
        {
          "traceId":       "aabbccddeeff00112233445566778899",
          "spanId":        "bbccdd1122334455",
          "parentSpanId":  "",
          "name":          "DB query",
          "startTimeUnixNano": "$NOW_NS",
          "endTimeUnixNano":   "$END_NS",
          "status": {"code": 2},
          "attributes": [
            {"key": "db.system", "value": {"stringValue": "postgres"}},
            {"key": "error",     "value": {"stringValue": "timeout"}}
          ]
        }
      ]
    }]
  }]
}
EOF
)

  STATUS=$(curl -sf -o /dev/null -w "%{http_code}" \
    -X POST "$OTLP_URL/otlp/v1/traces" \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD" 2>/dev/null || echo "000")

  if [[ "$STATUS" == "200" ]]; then
    ok "POST /otlp/v1/traces → 200 (2 spans: 1 ok, 1 error)"
  else
    fail "POST /otlp/v1/traces → HTTP $STATUS"
  fi
}

send_otlp_logs() {
  hdr "OTLP logs ingest ($OTLP_URL)"

  NOW_NS=$(date +%s)000000000

  PAYLOAD=$(cat <<EOF
{
  "resourceLogs": [{
    "resource": {
      "attributes": [
        {"key": "service.name", "value": {"stringValue": "demo-app"}},
        {"key": "host.name",    "value": {"stringValue": "lab-host"}}
      ]
    },
    "scopeLogs": [{
      "scope": {"name": "lab-verify"},
      "logRecords": [
        {
          "timeUnixNano": "$NOW_NS",
          "severityText": "INFO",
          "severityNumber": 9,
          "body": {"stringValue": "request processed in 42ms"},
          "attributes": [
            {"key": "request_id", "value": {"stringValue": "req-abc-123"}}
          ]
        },
        {
          "timeUnixNano": "$NOW_NS",
          "severityText": "ERROR",
          "severityNumber": 17,
          "body": {"stringValue": "database connection pool exhausted"},
          "attributes": [
            {"key": "db",      "value": {"stringValue": "postgres"}},
            {"key": "pool_id", "value": {"stringValue": "primary"}}
          ]
        }
      ]
    }]
  }]
}
EOF
)

  STATUS=$(curl -sf -o /dev/null -w "%{http_code}" \
    -X POST "$OTLP_URL/otlp/v1/logs" \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD" 2>/dev/null || echo "000")

  if [[ "$STATUS" == "200" ]]; then
    ok "POST /otlp/v1/logs → 200 (2 log records: info + error)"
  else
    fail "POST /otlp/v1/logs → HTTP $STATUS"
  fi
}

# ── 4. Prometheus scraping ───────────────────────────────────────────────────
prometheus_checks() {
  hdr "Prometheus scraping ($PROM_URL)"

  if ! curl -sf "$PROM_URL/-/ready" -o /dev/null; then
    warn "Prometheus not reachable at $PROM_URL — skipping"
    return
  fi
  ok "Prometheus is up"

  TARGETS=$(curl -sf "$PROM_URL/api/v1/targets" | python3 -c "
import sys, json
d = json.load(sys.stdin)
for t in d['data']['activeTargets']:
    print(t['scrapeUrl'], t['health'])
" 2>/dev/null || echo "parse error")

  echo "$TARGETS" | while read -r line; do
    url=$(echo "$line" | awk '{print $1}')
    health=$(echo "$line" | awk '{print $2}')
    if [[ "$health" == "up" ]]; then
      ok "Prometheus target UP: $url"
    else
      fail "Prometheus target DOWN: $url (health=$health)"
    fi
  done

  # Query ruptura_kpi metric
  RUPTURA_KPI=$(curl -sf \
    "$PROM_URL/api/v1/query?query=ruptura_kpi" 2>/dev/null || echo "")
  RESULT_COUNT=$(echo "$RUPTURA_KPI" | python3 -c "
import sys, json
d = json.load(sys.stdin)
print(len(d.get('data',{}).get('result',[])))
" 2>/dev/null || echo "0")

  if [[ "$RESULT_COUNT" -gt 0 ]]; then
    ok "ruptura_kpi metric present in Prometheus ($RESULT_COUNT series)"
    # Show sample KPI values
    echo "$RUPTURA_KPI" | python3 -c "
import sys, json
d = json.load(sys.stdin)
for r in d['data']['result'][:6]:
    m = r['metric']
    v = r['value'][1]
    print(f\"    {m.get('signal','?'):15s} workload={m.get('workload','?')} value={v}\")
" 2>/dev/null || true
  else
    warn "ruptura_kpi not yet in Prometheus (wait ~15s after sending OTLP data)"
  fi
}

# ── 5. KPI API verification ──────────────────────────────────────────────────
kpi_checks() {
  hdr "Ruptura KPI API"

  EVENTS=$(curl -sf "$RUPTURA_URL/api/v2/events?limit=5" 2>/dev/null || echo "[]")
  COUNT=$(echo "$EVENTS" | python3 -c "import sys,json; print(len(json.load(sys.stdin)))" 2>/dev/null || echo "0")
  if [[ "$COUNT" -gt 0 ]]; then
    ok "GET /api/v2/events → $COUNT recent events"
  else
    warn "GET /api/v2/events → empty (ingest data first)"
  fi

  FORECAST=$(curl -sf "$RUPTURA_URL/api/v2/forecast?workload=default/Deployment/demo-app&horizon=60" \
    2>/dev/null | python3 -c "
import sys, json
d = json.load(sys.stdin)
if isinstance(d, list) and len(d) > 0:
    print(f'{len(d)} data points')
elif isinstance(d, dict):
    print(d)
" 2>/dev/null || echo "empty")
  echo "    Forecast for demo-app: $FORECAST"

  SUPPRESS=$(curl -sf -X POST "$RUPTURA_URL/api/v2/suppressions" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d '{"workload":"default/Deployment/demo-app","start":"2026-01-01T00:00:00Z","end":"2026-01-01T01:00:00Z"}' \
    -o /dev/null -w "%{http_code}" 2>/dev/null || echo "000")
  if [[ "$SUPPRESS" =~ ^(200|201|409)$ ]]; then
    ok "POST /api/v2/suppressions → HTTP $SUPPRESS"
  else
    warn "POST /api/v2/suppressions → HTTP $SUPPRESS"
  fi
}

# ── Main ─────────────────────────────────────────────────────────────────────
MODE="${1:-}"

case "$MODE" in
  --diag)
    pod_diagnostics
    ;;
  --otlp)
    send_otlp_metrics
    send_otlp_traces
    send_otlp_logs
    sleep 2
    api_checks
    prometheus_checks
    ;;
  *)
    pod_diagnostics
    api_checks
    send_otlp_metrics
    send_otlp_traces
    send_otlp_logs
    sleep 3
    api_checks   # re-check after ingest
    prometheus_checks
    kpi_checks
    ;;
esac

echo ""
echo "────────────────────────────────────────────────"
echo " Results: ${GREEN}${PASS} passed${NC}  ${RED}${FAIL} failed${NC}"
echo "────────────────────────────────────────────────"

if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
