#!/bin/bash
# Stuck rollout test. Run on the lab node under nohup, about 50 minutes.
# Pre-registered pass, written before the first run on 2026-11-03:
# - rollout_stuck listed for rollout-test in 3 of 3 stuck rounds, within 5 min
#   of Kubernetes setting Progressing=False;
# - it clears within 5 min after the undo;
# - 0 of 3 healthy rollouts list any failure for rollout-test;
# - replicas_unavailable is never listed for rollout-test, old pods stay up.
# Samples go to /var/tmp/rollout.jsonl, phases to /var/tmp/rollout-phases.log.
set -u
NS=demo-shop; D=rollout-test; OUT=/var/tmp/rollout.jsonl; PH=/var/tmp/rollout-phases.log
API=http://172.30.10.124:80/api/v2/failures
phase() { echo "$(date -u +%FT%TZ) $*" >> $PH; }
sample() {
  local prog fail
  prog=$(oc get deploy $D -n $NS -o jsonpath='{.status.conditions[?(@.type=="Progressing")].status}/{.status.conditions[?(@.type=="Progressing")].reason}/{.status.availableReplicas}' 2>/dev/null)
  fail=$(curl -s -m 20 -H "Authorization: Bearer $(cat /var/tmp/.k)" $API | python3 -c 'import json,sys; print(",".join(f["failureMode"]+":"+f["state"] for f in json.load(sys.stdin) if f["target"].endswith("/'$D'")))' 2>/dev/null)
  echo "{\"t\":\"$(date -u +%FT%TZ)\",\"progressing\":\"$prog\",\"failures\":\"$fail\"}" >> $OUT
}
watch() { local end=$((SECONDS+$1)); while [ $SECONDS -lt $end ]; do sample; sleep 15; done; }
probe() { oc patch deploy $D -n $NS --type=json -p "[{\"op\":\"replace\",\"path\":\"/spec/template/spec/containers/0/readinessProbe/exec/command\",\"value\":[\"$1\"]},{\"op\":\"replace\",\"path\":\"/spec/template/spec/containers/0/env/0/value\",\"value\":\"$2\"}]" >/dev/null; }
rm -f $OUT $PH
oc apply -f - >/dev/null <<Y
apiVersion: apps/v1
kind: Deployment
metadata: {name: $D, namespace: $NS, labels: {app: $D}}
spec:
  replicas: 2
  progressDeadlineSeconds: 120
  strategy: {type: RollingUpdate, rollingUpdate: {maxSurge: 1, maxUnavailable: 0}}
  selector: {matchLabels: {app: $D}}
  template:
    metadata: {labels: {app: $D}}
    spec:
      containers:
      - name: app
        image: registry.access.redhat.com/ubi9/ubi-minimal
        command: [sh, -c, 'while true; do sleep 30; done']
        env: [{name: VERSION, value: v1}]
        readinessProbe: {exec: {command: ["true"]}, periodSeconds: 5}
        resources: {requests: {cpu: 5m, memory: 16Mi}, limits: {memory: 64Mi}}
Y
oc rollout status deploy/$D -n $NS --timeout=180s >/dev/null; phase start
watch 300
for r in 1 2 3; do
  phase "round $r healthy rollout"; probe true "v1-$r"; watch 300
  phase "round $r stuck rollout"; probe false "bad-$r"; watch 480
  phase "round $r undo"; probe true "v1-$r-back"; watch 300
done
phase end
