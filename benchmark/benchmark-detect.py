#!/usr/bin/env python3
"""Times failure detection: Ruptura against the rules a user would need.

Opponents are the rules shipped with OpenShift with their namespace filter
removed, with their own `for`, and two community rules where OpenShift 4.22 ships
none (CPU throttling, OOM kill). See opponent-rules.yaml. Each round uses new
workloads, so no contestant keeps state from the last one. A fault counts as
detected by Ruptura only with its expected failure mode.

Run on the lab node. Appends one JSON line per fault and round to DETECT_OUT,
/var/tmp/detect.jsonl by default. DETECT_ROUNDS sets the rounds, 3 by default.
"""
import json
import os
import subprocess
import time
import urllib.parse as U
import urllib.request

NS = "demo-shop"
API = "http://172.30.10.124:80/api/v2"
OUT = os.environ.get("DETECT_OUT", "/var/tmp/detect.jsonl")
ROUNDS = int(os.environ.get("DETECT_ROUNDS", "3"))
WATCH_SECONDS = 22 * 60
TICK = 15

def manifest(name, spec):
    return {"apiVersion": "apps/v1", "kind": "Deployment", "metadata": {"name": name, "namespace": NS, "labels": {"benchmark": "detect"}},
            "spec": {"replicas": 1, "selector": {"matchLabels": {"app": name}},
                     "template": {"metadata": {"labels": {"app": name}}, "spec": spec}}}

def replicas(body, count):
    body["spec"]["replicas"] = count
    return body

UBI = "registry.access.redhat.com/ubi9/ubi-minimal"
RES = {"requests": {"cpu": "10m", "memory": "16Mi"}, "limits": {"cpu": "50m", "memory": "64Mi"}}
MIN15 = 15 * 60
# name: (deployment, opponent expression, opponent `for` in seconds, expected Ruptura mode)
FAULTS = {
    "det-crash": (manifest("det-crash", {"containers": [{"name": "app", "image": UBI, "command": ["sh", "-c", "sleep 5; exit 1"], "resources": RES}]}),
                  'max_over_time(kube_pod_container_status_waiting_reason{namespace="demo-shop",reason="CrashLoopBackOff",pod=~"det-crash.*"}[5m]) >= 1',
                  MIN15, "crash_loop"),
    "det-image": (manifest("det-image", {"containers": [{"name": "app", "image": "registry.access.redhat.com/ubi9/does-not-exist:2", "resources": RES}]}),
                  'sum by (pod)(kube_pod_status_phase{namespace="demo-shop",phase=~"Pending|Unknown|Failed",pod=~"det-image.*"}) > 0',
                  MIN15, "startup_stuck"),
    "det-mount": (manifest("det-mount", {"containers": [{"name": "app", "image": UBI, "command": ["sleep", "infinity"], "resources": RES,
                                                         "volumeMounts": [{"name": "s", "mountPath": "/s"}]}],
                                         "volumes": [{"name": "s", "secret": {"secretName": "absent-secret"}}]}),
                  'sum by (pod)(kube_pod_status_phase{namespace="demo-shop",phase=~"Pending|Unknown|Failed",pod=~"det-mount.*"}) > 0',
                  MIN15, "startup_stuck"),
    # Two replicas that refuse to share a node: on a single node lab one stays Pending.
    "det-replicas": (replicas(manifest("det-replicas", {"affinity": {"podAntiAffinity": {"requiredDuringSchedulingIgnoredDuringExecution": [
                                                        {"labelSelector": {"matchLabels": {"app": "det-replicas"}}, "topologyKey": "kubernetes.io/hostname"}]}},
                                                    "containers": [{"name": "app", "image": UBI, "command": ["sleep", "infinity"], "resources": RES}]}), 2),
                     '(kube_deployment_spec_replicas{namespace="demo-shop",deployment="det-replicas"} > kube_deployment_status_replicas_available{namespace="demo-shop",deployment="det-replicas"})'
                     ' and (changes(kube_deployment_status_replicas_updated{namespace="demo-shop",deployment="det-replicas"}[5m]) == 0)',
                     MIN15, "replicas_unavailable"),
    # Community rule CPUThrottlingHigh, not shipped with OpenShift 4.22: over 25 percent throttled for 15 min.
    "det-throttle": (manifest("det-throttle", {"containers": [{"name": "app", "image": UBI, "command": ["sh", "-c", "while true; do :; done"],
                                                               "resources": {"requests": {"cpu": "10m", "memory": "16Mi"}, "limits": {"cpu": "20m", "memory": "64Mi"}}}]}),
                     'sum(increase(container_cpu_cfs_throttled_periods_total{namespace="demo-shop",pod=~"det-throttle.*",container!=""}[5m]))'
                     ' / sum(increase(container_cpu_cfs_periods_total{namespace="demo-shop",pod=~"det-throttle.*",container!=""}[5m])) > 0.25',
                     MIN15, "cpu_throttling"),
    # Community rule KubernetesContainerOomKiller, not shipped with OpenShift 4.22, fires at once.
    "det-oom": (manifest("det-oom", {"containers": [{"name": "app", "image": UBI, "command": ["sh", "-c", "sleep 60; head -c 100m /dev/zero | tail; sleep 600"], "resources": RES}]}),
                '(kube_pod_container_status_restarts_total{namespace="demo-shop",pod=~"det-oom.*"} - kube_pod_container_status_restarts_total{namespace="demo-shop",pod=~"det-oom.*"} offset 10m >= 1)'
                ' and ignoring(reason) min_over_time(kube_pod_container_status_last_terminated_reason{namespace="demo-shop",pod=~"det-oom.*",reason="OOMKilled"}[10m]) == 1',
                0, "oom_killed"),
}

def oc(*args, stdin=None):
    return subprocess.run(["oc", *args], input=stdin, capture_output=True, text=True, check=False)

def rule_true(expr):
    url = "http://localhost:9090/api/v1/query?" + U.urlencode({"query": expr})
    got = oc("exec", "-n", "openshift-monitoring", "prometheus-k8s-0", "-c", "prometheus", "--", "curl", "-s", "-m", "10", url)
    try:
        return bool(json.loads(got.stdout)["data"]["result"])
    except (ValueError, KeyError):
        return False

def failures():
    req = urllib.request.Request(API + "/failures", headers={"Authorization": "Bearer " + open("/var/tmp/.k").read().strip()})
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            return json.load(resp)
    except OSError:
        return []

def renamed(name, body, expr, number):
    """Gives each round a new workload, so no contestant keeps state from the last one."""
    fresh = f"{name}-r{number}"
    return fresh, json.loads(json.dumps(body).replace(name, fresh)), expr.replace(name, fresh)

def run_round(number):
    faults = {}
    for name, (body, expr, rule_for, expected) in FAULTS.items():
        fresh, fresh_body, fresh_expr = renamed(name, body, expr, number)
        faults[fresh] = (fresh_body, fresh_expr, rule_for, expected)
    start = time.time()
    for name, (body, _, _, _) in faults.items():
        oc("apply", "-f", "-", stdin=json.dumps(body))
    seen = {name: None for name in faults}
    mode = {name: None for name in faults}
    first_true = {name: None for name in faults}
    while time.time() - start < WATCH_SECONDS:
        results = failures()
        now = time.time()
        for name, (_, expr, _, expected) in faults.items():
            if seen[name] is None:
                match = [r for r in results if r["target"].endswith("/" + name)]
                if match and mode[name] is None:
                    mode[name] = match[0]["failureMode"]
                if any(r["failureMode"] == expected for r in match):
                    seen[name] = now - start
            if first_true[name] is None and rule_true(expr):
                first_true[name] = time.time() - start
        time.sleep(TICK)
    for name, (_, _, rule_for, expected) in faults.items():
        rule = first_true[name] + rule_for if first_true[name] is not None else None
        row = {"round": number, "fault": name, "expected": expected, "first_mode": mode[name],
               "ruptura_seconds": seen[name], "rule_seconds": rule}
        with open(OUT, "a") as out:
            out.write(json.dumps(row) + "\n")
    for name in faults:
        oc("delete", "deploy", name, "-n", NS, "--ignore-not-found")

if __name__ == "__main__":
    os.environ["KUBECONFIG"] = "/etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/localhost.kubeconfig"
    for number in range(ROUNDS):
        run_round(number)
    with open(OUT, "a") as out:
        out.write(json.dumps({"done": True}) + "\n")
