#!/bin/bash
# Diverging replica test. Pre-registered 2026-11-04 before any code of the recipe
# and before any run. Run on the lab node under nohup, about 7 h 30.
#
# Deployment demo-shop/replica-test, 3 replicas, memory limit 256Mi, 60 MiB at rest.
# A pod leaks LEAK MiB per minute while its annotation ruptura-test/leak=LEAK is
# set, read through the downward API. One round:
#   quiet 25 min          no pod leaks
#   one   until OOM       one pod chosen by the script leaks 3 MiB/min (about 65 min)
#   restart + 15 min      rollout restart, settle
#   all   30 min          the three pods leak 3 MiB/min
#   restart + 15 min      rollout restart, settle
# Three rounds.
#
# Scores, on replica-test only:
# - Ruptura: first time /failures lists replica-test as kind predicted, and its pod;
# - predict_linear on container_memory_working_set_bytes against the limit, 30m/30m
#   and 10m/30m, first firing per pod, read from Prometheus afterwards;
# - lead: time from that first warning to the OOM of the leaking pod.
# Pass, all three:
#   R1 in 3 of 3 "one" phases, Ruptura warns within 15 min of the leak start and the
#      warning names the leaking pod;
#   R2 Ruptura median lead is at least 20 min longer than the better predict_linear
#      setting's median lead on the leaking pod;
#   R3 replica_divergence is never listed during "quiet" and "all" phases, and no
#      predicted failure is listed during "quiet" phases.
# Samples: /var/tmp/replica.jsonl, phases: /var/tmp/replica-phases.log.
set -u
NS=demo-shop; D=replica-test; OUT=/var/tmp/replica.jsonl; PH=/var/tmp/replica-phases.log
API=http://172.30.10.124:80/api/v2/failures
phase() { echo "$(date -u +%FT%TZ) $*" >> $PH; }
sample() {
  local fail
  fail=$(curl -s -m 20 -H "Authorization: Bearer $(cat /var/tmp/.k)" $API | python3 -c 'import json,sys; print(",".join(f["failureMode"]+":"+f["kind"]+":"+f.get("pod","")+":"+str(f.get("etaSeconds",0)) for f in json.load(sys.stdin) if f["target"].endswith("/'$D'")))' 2>/dev/null)
  echo "{\"t\":\"$(date -u +%FT%TZ)\",\"failures\":\"$fail\",\"pods\":\"$(oc get pods -n $NS -l app=$D -o jsonpath='{range .items[*]}{.metadata.name}={.metadata.annotations.ruptura-test/leak}/{.status.containerStatuses[0].restartCount}/{.status.containerStatuses[0].lastState.terminated.reason} {end}' 2>/dev/null)\"}" >> $OUT
}
watch() { local end=$((SECONDS+$1)); while [ $SECONDS -lt $end ]; do sample; sleep 30; done; }
pods() { oc get pods -n $NS -l app=$D --field-selector=status.phase=Running -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}'; }
restart() { oc rollout restart deploy/$D -n $NS >/dev/null; oc rollout status deploy/$D -n $NS --timeout=300s >/dev/null; }
rm -f $OUT $PH
oc apply -f - >/dev/null <<Y
apiVersion: apps/v1
kind: Deployment
metadata: {name: $D, namespace: $NS, labels: {app: $D}}
spec:
  replicas: 3
  selector: {matchLabels: {app: $D}}
  template:
    metadata: {labels: {app: $D}}
    spec:
      containers:
      - name: app
        image: registry.access.redhat.com/ubi9/python-312
        command: [python3, -c, "import time,re\nb=[bytearray(1024*1024) for _ in range(60)]\nwhile True:\n  a=open('/podinfo/annotations').read()\n  m=re.search(r'ruptura-test/leak=\"([0-9.]+)\"',a)\n  if m:\n    for _ in range(int(float(m.group(1))*10/60*6)): b.append(bytearray(102400))\n  time.sleep(6)"]
        resources: {requests: {cpu: 10m, memory: 64Mi}, limits: {memory: 256Mi}}
        volumeMounts: [{name: podinfo, mountPath: /podinfo}]
      volumes:
      - name: podinfo
        downwardAPI: {items: [{path: annotations, fieldRef: {fieldPath: metadata.annotations}}]}
Y
oc rollout status deploy/$D -n $NS --timeout=300s >/dev/null
phase start; watch 1200
for round in 1 2 3; do
  phase "round $round quiet"; watch 1500
  leaker=$(pods | head -1)
  oc annotate pod $leaker -n $NS ruptura-test/leak=3 --overwrite >/dev/null
  phase "round $round one $leaker"
  end=$((SECONDS+5400)); while [ $SECONDS -lt $end ]; do sample; [ "$(oc get pod $leaker -n $NS -o jsonpath='{.status.containerStatuses[0].lastState.terminated.reason}' 2>/dev/null)" = OOMKilled ] && break; sleep 30; done
  phase "round $round oom or timeout"; restart; phase "round $round settle"; watch 900
  for p in $(pods); do oc annotate pod $p -n $NS ruptura-test/leak=3 --overwrite >/dev/null; done
  phase "round $round all"; watch 1800
  restart; phase "round $round settle"; watch 900
done
phase end
