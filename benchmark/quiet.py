#!/usr/bin/env python3
"""Nov 16 benchmark, parts C and D: findings on workloads with no injected fault,
and the resources Ruptura uses, over one window.

Frozen 2026-11-03, before any Nov 16 data. Dated amendments only before the window.

Usage on the lab node (same pattern as checkpoint.py):
  python3 /tmp/quiet.py 2026-11-16T00:00 2026-11-16T12:00 /var/tmp/fleet.jsonl ...

C. Every workload outside FAULTS counts as having no injected fault, in every
   namespace. A finding is an episode, split by gaps over 5 min:
   - Ruptura: any failure listed for the workload in the fleet log;
   - opponent: any rule of opponent-rules.yaml true for its full `for`.
   Every finding is printed with its workload and kind, to be classified by hand as
   real (Kubernetes shows the condition) or false, and both counts are reported.
   Pass: Ruptura has no more findings than the opponent rule set, and at most 3
   false ones in 12 h.
D. Report only, no pass rule: CPU of the engine container, mean and highest 5 min
   rate, and highest working set memory, for the engine and the operator.
"""
import collections, datetime as D, json, re, subprocess, sys, urllib.parse as U

STEP, GAP = 60, 300
FAULTS = re.compile(r'^demo-shop/(leak-orders|step-leak|accel-leak|oom-spike|periodic-spike|sawtooth-batch|cpu-batch|'
                    r'bad-image|missing-mount|crash-pay|rollout-test|det-.*|shop-.*)$')
OPPONENT = [
    ('KubePodCrashLooping', 900, 'max_over_time(kube_pod_container_status_waiting_reason{reason="CrashLoopBackOff",job="kube-state-metrics"}[5m]) >= 1'),
    ('KubePodNotReady', 900, 'sum by (namespace, pod) (max by (namespace, pod) (kube_pod_status_phase{job="kube-state-metrics", phase=~"Pending|Unknown"}'
     ' unless ignoring(phase) (kube_pod_status_unschedulable{job="kube-state-metrics"} == 1)) * on (namespace, pod) group_left(owner_kind)'
     ' topk by (namespace, pod) (1, max by (namespace, pod, owner_kind) (kube_pod_owner{owner_kind!="Job"}))) > 0'),
    ('KubeDeploymentReplicasMismatch', 900, '(kube_deployment_spec_replicas{job="kube-state-metrics"} > kube_deployment_status_replicas_available{job="kube-state-metrics"})'
     ' and (changes(kube_deployment_status_replicas_updated{job="kube-state-metrics"}[5m]) == 0)'),
    ('KubeDeploymentRolloutStuck', 900, 'kube_deployment_status_condition{condition="Progressing", status="false", job="kube-state-metrics"} != 0'),
    ('KubeContainerWaiting', 3600, 'kube_pod_container_status_waiting_reason{reason!="CrashLoopBackOff", job="kube-state-metrics"} > 0'),
    ('CPUThrottlingHigh', 900, 'sum by (namespace, pod) (increase(container_cpu_cfs_throttled_periods_total{container!=""}[5m]))'
     ' / sum by (namespace, pod) (increase(container_cpu_cfs_periods_total{container!=""}[5m])) > 0.25'),
    ('KubernetesContainerOomKiller', 0, '(kube_pod_container_status_restarts_total - kube_pod_container_status_restarts_total offset 10m >= 1)'
     ' and ignoring(reason) min_over_time(kube_pod_container_status_last_terminated_reason{reason="OOMKilled"}[10m]) == 1'),
]


def ts(text):
    return int(D.datetime.fromisoformat(text[:19]).replace(tzinfo=D.timezone.utc).timestamp())


START, END, FLEET = ts(sys.argv[1]), ts(sys.argv[2]), sys.argv[3:]


def query(expr):
    url = 'http://localhost:9090/api/v1/query_range?' + U.urlencode(dict(query=expr, start=START, end=END, step=STEP))
    out = subprocess.check_output(['oc', 'exec', '-n', 'openshift-monitoring', 'prometheus-k8s-0', '-c', 'prometheus', '--',
                                   'curl', '-s', '-m', '110', url], stderr=subprocess.DEVNULL)
    data = json.loads(out)
    if 'data' not in data:
        raise SystemExit(data)
    return data['data']['result']


def workload(metric):
    namespace = metric.get('namespace', '')
    name = metric.get('deployment') or re.sub(r'-[a-z0-9]{6,10}-[a-z0-9]{5}$', '', metric.get('pod', ''))
    return namespace + '/' + name


def episodes(times):
    out = []
    for t in sorted(times):
        if out and t - out[-1][1] <= GAP:
            out[-1][1] = t
        else:
            out.append([t, t])
    return out


def opponent_findings():
    found = collections.defaultdict(list)
    for name, hold, expr in OPPONENT:
        times = collections.defaultdict(set)
        for s in query(expr):
            times[workload(s['metric'])].update(int(t) for t, _ in s['values'])
        for key, values in times.items():
            for start, end in episodes(values):
                if end - start >= hold and not FAULTS.match(key):
                    found[key].append((name, start + hold))
    return found


def ruptura_findings():
    times = collections.defaultdict(lambda: collections.defaultdict(set))
    for path in FLEET:
        for line in open(path):
            try:
                row = json.loads(line)
            except ValueError:
                continue
            t = ts(row['t'])
            if not START <= t <= END:
                continue
            for host in row['fleet'].get('hosts', []):
                failure = host.get('failure')
                if failure:
                    parts = host['host'].split('/')
                    times[parts[0] + '/' + parts[-1]][failure['failureMode']].add(t)
    found = collections.defaultdict(list)
    for key, modes in times.items():
        if FAULTS.match(key):
            continue
        for mode, values in modes.items():
            found[key] += [(mode, start) for start, _ in episodes(values)]
    return found


def resources():
    for label, sel in (('engine', 'namespace="ruptura-h",container="ruptura"'), ('operator', 'namespace="ruptura-system",container="ruptura-operator"')):
        cpu = [float(v) for s in query(f'sum(rate(container_cpu_usage_seconds_total{{{sel}}}[5m]))') for _, v in s['values']]
        mem = [float(v) for s in query(f'sum(container_memory_working_set_bytes{{{sel}}})') for _, v in s['values']]
        if cpu and mem:
            print(f'  {label}: CPU mean {sum(cpu) / len(cpu) * 1000:.0f}m, highest {max(cpu) * 1000:.0f}m, memory highest {max(mem) / 2**20:.0f} Mi')
        else:
            print(f'  {label}: no data')


def show(title, found):
    total = sum(len(v) for v in found.values())
    print(f'\n{title}: {total} findings on {len(found)} workloads without injected fault')
    for key in sorted(found):
        for kind, at in sorted(found[key], key=lambda x: x[1]):
            print(f'  {D.datetime.utcfromtimestamp(at):%H:%M} {key} {kind}')


print(f'window {sys.argv[1]} to {sys.argv[2]} UTC')
show('Ruptura', ruptura_findings())
show('Opponent rules', opponent_findings())
print('\nResources')
resources()
