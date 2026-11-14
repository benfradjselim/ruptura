#!/bin/bash
# Terrain check before a checkpoint window. Run: /root/lab.sh < checkpoint-preflight.sh
# Prints one PASS or FAIL line per check, then READY or NOT READY.
export KUBECONFIG=/etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/localhost.kubeconfig
NS=demo-shop; fail=0
ok() { echo "PASS $*"; }
ko() { echo "FAIL $*"; fail=1; }
prom() { oc exec -n openshift-monitoring prometheus-k8s-0 -c prometheus -- curl -s -m 30 --data-urlencode "query=$1" http://localhost:9090/api/v1/query 2>/dev/null; }
value() { python3 -c 'import json,sys; r=json.load(sys.stdin)["data"]["result"]; print(int(float(r[0]["value"][1])) if r else -1)'; }

for w in leak-orders step-leak accel-leak oom-spike periodic-spike sawtooth-batch stable-web cpu-batch; do
  phase=$(oc get pods -n $NS --no-headers 2>/dev/null | awk -v w="$w" '$1 ~ "^"w"-[a-z0-9]+-[a-z0-9]+$" {print $3}' | sort -u | tr '\n' ' ')
  case "$phase" in "Running "|"OOMKilled "|"CrashLoopBackOff ") ok "$w pod $phase";; *) ko "$w pod state '$phase'";; esac
done
# OOM faults must be cycling: restarts in the last 3 h.
for w in leak-orders:3 step-leak:3 accel-leak:3 oom-spike:10 periodic-spike:3; do
  n=$(prom "sum(increase(kube_pod_container_status_restarts_total{namespace=\"$NS\",pod=~\"${w%%:*}-.*\"}[3h]))" | value)
  [ "$n" -ge "${w##*:}" ] && ok "${w%%:*} restarted $n times in 3 h" || ko "${w%%:*} restarted $n times in 3 h, want ${w##*:} or more"
done
# Controls must not restart.
for w in sawtooth-batch stable-web cpu-batch; do
  n=$(prom "sum(increase(kube_pod_container_status_restarts_total{namespace=\"$NS\",pod=~\"$w-.*\"}[3h]))" | value)
  [ "$n" -le 0 ] && ok "$w no restart in 3 h" || ko "$w restarted $n times in 3 h"
done
# periodic-spike needs 6 failures before the failure memory can forecast it.
n=$(prom "max(kube_pod_container_status_restarts_total{namespace=\"$NS\",pod=~\"periodic-spike-.*\"})" | value)
[ "$n" -ge 6 ] && ok "periodic-spike has $n failures" || ko "periodic-spike has $n failures, memory needs 6"
# Engine running, started long enough ago, sampler writing.
# Container start, so a crash restart inside the pod counts too.
start=$(oc get pods -n ruptura-h -l 'app.kubernetes.io/name=ruptura,!app.kubernetes.io/component' -o jsonpath='{.items[0].status.containerStatuses[0].state.running.startedAt}' 2>/dev/null)
if [ -z "$start" ]; then ko "engine container start time not found"
else
  age=$(( $(date +%s) - $(date -d "$start" +%s) ))
  [ "$age" -ge 2700 ] && ok "engine up since $start" || ko "engine up only ${age}s since $start, want 45 min before the window"
fi
pgrep -f sampler.sh >/dev/null && ok "sampler running" || ko "sampler not running"
fresh=$(( $(date +%s) - $(stat -c %Y /var/tmp/fleet.jsonl 2>/dev/null || echo 0) ))
[ "$fresh" -le 180 ] && ok "fleet log written ${fresh}s ago" || ko "fleet log stale, ${fresh}s"
code=$(curl -s -o /dev/null -w '%{http_code}' -m 20 -H "Authorization: Bearer $(cat /var/tmp/.k)" http://172.30.10.124:80/api/v2/failures)
[ "$code" = 200 ] && ok "engine API answers" || ko "engine API returned $code"
[ $fail = 0 ] && echo READY || echo "NOT READY"
