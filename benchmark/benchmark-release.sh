#!/bin/bash
# Bad release test. Pre-registered 2026-11-04 before any code of the recipe and
# before any run. Run on the lab node under nohup, about 6 h 30.
#
# Deployment demo-shop/release-test, 2 replicas, memory limit 256Mi, RollingUpdate
# maxSurge 1 maxUnavailable 0. Each version allocates WARM MiB over RAMP minutes,
# then LEAK MiB per minute:
#   v1      WARM 60  RAMP 3  LEAK 0   the reference
#   healthy WARM 60  RAMP 3  LEAK 0   same behaviour, new version: must stay silent
#   heavy   WARM 120 RAMP 8  LEAK 0   more memory, stable, long warm-up: must stay silent
#   bad     WARM 60  RAMP 3  LEAK 3   reaches the limit about 65 min after start
# One round: v1 settled, healthy rollout + 25 min, heavy rollout + 25 min, back to v1
# + 15 min, bad rollout watched until both pods OOM or 80 min, undo to v1 + 15 min.
# Three rounds.
#
# Scores, from the rollout start of each version, on release-test pods only:
# - Ruptura: first time /failures lists release-test as kind predicted (any mode);
# - predict_linear on container_memory_working_set_bytes against the limit, 30m/30m
#   and 10m/30m, first firing per pod, read from Prometheus afterwards;
# - a bad-round warning counts with a lead of at least 10 min before the first OOM;
# - false: any predicted failure or firing on release-test during a healthy or heavy
#   phase, or during the settle phases.
# Pass, all three:
#   R1 Ruptura warns 3 of 3 bad releases within 15 min of the rollout start;
#   R2 Ruptura median lead before the first OOM is at least 20 min longer than the
#      better predict_linear setting's median lead;
#   R3 Ruptura has 0 false warnings over the 6 healthy and heavy rollouts and the
#      settle phases.
# Samples: /var/tmp/release.jsonl, phases: /var/tmp/release-phases.log.
set -u
NS=demo-shop; D=release-test; OUT=/var/tmp/release.jsonl; PH=/var/tmp/release-phases.log
API=http://172.30.10.124:80/api/v2/failures
phase() { echo "$(date -u +%FT%TZ) $*" >> $PH; }
sample() {
  local fail
  fail=$(curl -s -m 20 -H "Authorization: Bearer $(cat /var/tmp/.k)" $API | python3 -c 'import json,sys; print(",".join(f["failureMode"]+":"+f["kind"]+":"+str(f.get("etaSeconds",0)) for f in json.load(sys.stdin) if f["target"].endswith("/'$D'")))' 2>/dev/null)
  echo "{\"t\":\"$(date -u +%FT%TZ)\",\"failures\":\"$fail\",\"restarts\":\"$(oc get pods -n $NS -l app=$D -o jsonpath='{range .items[*]}{.metadata.name}={.status.containerStatuses[0].restartCount}/{.status.containerStatuses[0].lastState.terminated.reason} {end}' 2>/dev/null)\"}" >> $OUT
}
watch() { local end=$((SECONDS+$1)); while [ $SECONDS -lt $end ]; do sample; sleep 30; done; }
version() { oc set env deploy/$D -n $NS VERSION=$1 WARM=$2 RAMP=$3 LEAK=$4 >/dev/null; }
oom_both() { [ "$(oc get pods -n $NS -l app=$D -o jsonpath='{range .items[*]}{.status.containerStatuses[0].lastState.terminated.reason}{"\n"}{end}' | grep -c OOMKilled)" -ge 2 ]; }
rm -f $OUT $PH
oc apply -f - >/dev/null <<Y
apiVersion: apps/v1
kind: Deployment
metadata: {name: $D, namespace: $NS, labels: {app: $D}}
spec:
  replicas: 2
  strategy: {type: RollingUpdate, rollingUpdate: {maxSurge: 1, maxUnavailable: 0}}
  selector: {matchLabels: {app: $D}}
  template:
    metadata: {labels: {app: $D}}
    spec:
      containers:
      - name: app
        image: registry.access.redhat.com/ubi9/python-312
        command: [python3, -c, "import os,time\nw,r,l=int(os.environ['WARM']),float(os.environ['RAMP']),float(os.environ['LEAK'])\nb=[];s=time.time()\nwhile True:\n  m=(time.time()-s)/60\n  target=w*min(1,m/r)+l*max(0,m-r)\n  while len(b)<target: b.append(bytearray(1024*1024))\n  time.sleep(5)"]
        env: [{name: VERSION, value: v1}, {name: WARM, value: "60"}, {name: RAMP, value: "3"}, {name: LEAK, value: "0"}]
        resources: {requests: {cpu: 10m, memory: 64Mi}, limits: {memory: 256Mi}}
Y
phase start; watch 1200
for round in 1 2 3; do
  phase "round $round healthy"; version healthy-$round 60 3 0; watch 1500
  phase "round $round heavy"; version heavy-$round 120 8 0; watch 1500
  phase "round $round back to v1"; version v1 60 3 0; watch 900
  phase "round $round bad"; version bad-$round 60 3 3
  end=$((SECONDS+4800)); while [ $SECONDS -lt $end ] && ! oom_both; do sample; sleep 30; done
  phase "round $round undo"; version v1 60 3 0; watch 900
done
phase end
