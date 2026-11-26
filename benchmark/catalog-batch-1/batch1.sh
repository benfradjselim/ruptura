#!/bin/bash
# Catalog batch 1 run (docs/engine/CATALOG-BATCH-1.md). Runs on the lab node as root.
# Writes /var/tmp/batch1/events.jsonl (what the run did) and api.jsonl (every
# /api/v2/failures answer of the test instance, every 30 s).
set -u
OUT=/var/tmp/batch1; mkdir -p "$OUT"; cd "$OUT"
API=http://$(oc -n ruptura-b1 get svc b1 -o jsonpath='{.spec.clusterIP}'):8080/api/v2/failures
KEY=$(cat /var/tmp/.kb1)
IMG=registry.access.redhat.com/ubi9/ubi-minimal:latest
NS=ruptura-batch1
ev() { printf '{"t":"%s","event":"%s"}\n' "$(date -u +%FT%TZ)" "$1" >> events.jsonl; }

record() {
  while [ ! -f stop ]; do
    body=$(curl -s -m 20 -H "Authorization: Bearer $KEY" "$API")
    printf '{"t":"%s","failures":%s}\n' "$(date -u +%FT%TZ)" "${body:-null}" >> api.jsonl
    truth >> truth.jsonl
    sleep 30
  done
}

truth() { # ground truth for scoring, one line per poll
  hpa=$(oc -n $NS get hpa t1-burn -o jsonpath='{.status.currentReplicas}/{.spec.maxReplicas}' 2>/dev/null)
  pdb=$(oc -n $NS get pdb t2-one -o jsonpath='{.status.disruptionsAllowed}/{.status.expectedPods}' 2>/dev/null)
  job=$(oc -n $NS get job t3-fail -o jsonpath='{range .status.conditions[*]}{.type}={.lastTransitionTime} {end}' 2>/dev/null)
  fs=$(df -B1 --output=used,size /var/mnt/b1fill 2>/dev/null | tail -1 | tr -s ' ' '/')
  cm=$(oc -n ruptura-b1-quota get cm --no-headers 2>/dev/null | wc -l)
  printf '{"t":"%s","hpa":"%s","pdb":"%s","job":"%s","fs":"%s","quota_cm":%s}\n' "$(date -u +%FT%TZ)" "$hpa" "$pdb" "$job" "$fs" "$cm"
}

deploy() { # name replicas command
  cat <<Y | oc apply -n $NS -f - >/dev/null
apiVersion: apps/v1
kind: Deployment
metadata: {name: $1}
spec:
  replicas: $2
  selector: {matchLabels: {app: $1}}
  template:
    metadata: {labels: {app: $1}}
    spec:
      containers:
      - name: c
        image: $IMG
        command: ["sh", "-c", "$3"]
        resources: {requests: {cpu: 50m, memory: 32Mi}, limits: {cpu: 200m, memory: 64Mi}}
Y
}

fill_fs() { # loop filesystem, 2 MiB every 15 s, stops at 95 %
  mnt=/var/mnt/b1fill
  while [ ! -f stop ]; do
    mountpoint -q "$mnt" || { ev "t4 not mounted, fill stopped"; break; }
    used=$(df --output=pcent "$mnt" | tail -1 | tr -dc 0-9)
    if [ "$used" -ge 95 ]; then ev "t4 fill stopped at ${used}%"; break; fi
    dd if=/dev/urandom of="$mnt/f$(date +%s%N)" bs=1M count=2 status=none
    sleep 15
  done
}

loopfs() { # name
  truncate -s 512M /var/tmp/$1.img
  mkfs.ext4 -q -m 0 -F /var/tmp/$1.img
  mkdir -p /var/mnt/$1 && mount -o loop /var/tmp/$1.img /var/mnt/$1
}

case "${1:-run}" in
run)
  rm -f stop events.jsonl api.jsonl truth.jsonl
  ev "start"
  oc create ns $NS >/dev/null || { ev "namespace $NS not created"; touch stop; exit 1; }
  record &
  # T1 HPA
  deploy t1-burn 1 "while :; do :; done"
  deploy t1-idle 1 "sleep infinity"
  oc -n $NS autoscale deploy/t1-burn --min=1 --max=2 --cpu-percent=50 >/dev/null
  oc -n $NS autoscale deploy/t1-idle --min=1 --max=3 --cpu-percent=50 >/dev/null
  ev "t1 created"
  # T2 PDB
  deploy t2-one 1 "sleep infinity"
  deploy t2-two 2 "sleep infinity"
  oc -n $NS create pdb t2-one --selector=app=t2-one --min-available=1 >/dev/null
  oc -n $NS create pdb t2-two --selector=app=t2-two --max-unavailable=1 >/dev/null
  ev "t2 created"
  # T3 Job
  for j in "t3-fail 1" "t3-ok 0"; do set -- $j
    cat <<Y | oc apply -n $NS -f - >/dev/null
apiVersion: batch/v1
kind: Job
metadata: {name: $1}
spec:
  backoffLimit: 0
  template:
    spec:
      restartPolicy: Never
      containers: [{name: c, image: $IMG, command: ["sh", "-c", "exit $2"]}]
Y
  done
  ev "t3 created"
  # T4 node filesystem
  loopfs b1fill && loopfs b1ctl && ev "t4 mounted"
  fill_fs &
  # T5 quota
  for q in ruptura-b1-quota ruptura-b1-qctl; do oc create ns $q >/dev/null; oc -n $q create quota q --hard=configmaps=60 >/dev/null; done
  for i in 1 2 3 4 5; do oc -n ruptura-b1-qctl create cm static-$i >/dev/null; done
  ev "t5 created"
  i=0
  while [ ! -f stop ]; do
    i=$((i+1)); n=$(oc -n ruptura-b1-quota get cm --no-headers 2>/dev/null | wc -l)
    if [ "$n" -lt 60 ]; then oc -n ruptura-b1-quota create cm c-$i >/dev/null 2>&1 || ev "t5 refused at $n"; fi
    case $i in
      40) oc -n $NS delete pdb t2-one >/dev/null; ev "t2 pdb deleted" ;;
      30) oc -n $NS delete job t3-fail t3-ok >/dev/null; ev "t3 jobs deleted" ;;
      60) oc -n $NS patch deploy t1-burn --type=json -p '[{"op":"replace","path":"/spec/template/spec/containers/0/command","value":["sh","-c","sleep infinity"]}]' >/dev/null; ev "t1 load stopped" ;;
      160) ev "end"; touch stop ;;
    esac
    sleep 30
  done
  ;;
clean)
  touch stop; sleep 2
  oc delete ns $NS ruptura-b1-quota ruptura-b1-qctl --wait=false >/dev/null 2>&1
  for m in b1fill b1ctl; do umount /var/mnt/$m 2>/dev/null; rm -rf /var/mnt/$m /var/tmp/$m.img; done
  ;;
esac
