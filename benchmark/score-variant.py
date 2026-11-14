#!/usr/bin/env python3
"""Scores the bad release and the diverging replica tests, as pre-registered in
the headers of benchmark-release.sh and benchmark-replica.sh. Written before
either test was scored. Runs on the lab node:

  python3 score-variant.py release   (reads /var/tmp/release.jsonl and release-phases.log)
  python3 score-variant.py replica   (reads /var/tmp/replica.jsonl and replica-phases.log)
"""
import datetime as D, json, re, statistics as st, subprocess, sys, urllib.parse as U

KIND = sys.argv[1]
NAME = {'release': 'release-test', 'replica': 'replica-test'}[KIND]
STEP = 30


def ts(text):
    return D.datetime.fromisoformat(text.replace('Z', '+00:00')).timestamp()


def fmt(t):
    return D.datetime.fromtimestamp(t, D.timezone.utc).strftime('%H:%M') if t else '-'


phases = [(ts(l.split()[0]), ' '.join(l.split()[1:])) for l in open('/var/tmp/%s-phases.log' % KIND) if l.strip()]
samples = [json.loads(l) for l in open('/var/tmp/%s.jsonl' % KIND) if l.strip()]
for s in samples:
    s['ts'] = ts(s['t'])
    s['pred'] = [f.split(':') for f in s['failures'].split(',') if ':predicted:' in f]
end_all = phases[-1][0]


def phase_spans(label):
    out = []
    for i, (t, name) in enumerate(phases):
        if re.search(label, name):
            out.append((t, phases[i + 1][0] if i + 1 < len(phases) else end_all, name))
    return out


def prom_fires(window, horizon, start, end):
    expr = ('max by (pod) (predict_linear(container_memory_working_set_bytes{namespace="demo-shop",pod=~"%s-.*",container="app"}[%s], %d)) '
            '> on (pod) max by (pod) (kube_pod_container_resource_limits{namespace="demo-shop",pod=~"%s-.*",resource="memory"})') % (NAME, window, horizon, NAME)
    url = 'http://localhost:9090/api/v1/query_range?' + U.urlencode(dict(query=expr, start=int(start), end=int(end), step=STEP))
    raw = subprocess.check_output(['oc', 'exec', '-n', 'openshift-monitoring', 'prometheus-k8s-0', '-c', 'prometheus', '--', 'curl', '-s', '-m', '110', url])
    fires = []
    for series in json.loads(raw)['data']['result']:
        fires += [(float(t), series['metric'].get('pod', '')) for t, _ in series['values']]
    return sorted(fires)


def first_pred(start, end, pod_filter=None):
    for s in samples:
        if start <= s['ts'] < end and s['pred']:
            if pod_filter is None or any(p[2].endswith(pod_filter) for p in s['pred']):
                return s['ts'], s['pred']
    return None, None


def first_oom(start, end, pod=None):
    for s in samples:
        if start <= s['ts'] < end and 'OOMKilled' in s.get('restarts', s.get('pods', '')):
            if pod is None or re.search(re.escape(pod) + r'=[^ ]*OOMKilled', s.get('pods', '')):
                return s['ts']
    return None


report = {'test': KIND, 'start': fmt(phases[0][0]), 'end': fmt(end_all)}
settings = {'30m/30m': ('30m', 1800), '10m/30m': ('10m', 1800)}
rows, leads = [], {'ruptura': [], '30m/30m': [], '10m/30m': []}
if KIND == 'release':
    targets = phase_spans(r'round \d bad')
    calm = [p for p in phase_spans(r'healthy|heavy|back to v1|undo|^start')]
else:
    targets = phase_spans(r'round \d one ')
    calm = phase_spans(r'quiet|^start')
    allp = phase_spans(r'round \d all')
for start, end, name in targets:
    leaker = name.split()[-1] if KIND == 'replica' else None
    oom_end = end + 3600
    oom = first_oom(start, oom_end, leaker)
    warn, pred = first_pred(start, oom or end)
    row = {'phase': name, 'start': fmt(start), 'oom': fmt(oom), 'ruptura_warn_min': round((warn - start) / 60, 1) if warn else None,
           'ruptura_modes': sorted({p[0] for p in pred}) if pred else [], 'ruptura_lead_min': round((oom - warn) / 60, 1) if warn and oom else None}
    if leaker:
        row['names_leaker'] = bool(pred) and any(p[2].endswith(leaker) for p in pred)
    for label, (w, h) in settings.items():
        fires = [t for t, pod in prom_fires(w, h, start, oom or end) if leaker is None or pod == leaker]
        f = fires[0] if fires else None
        row[label + '_warn_min'] = round((f - start) / 60, 1) if f else None
        lead = (oom - f) / 60 if f and oom and oom - f >= 0 else 0.0
        row[label + '_lead_min'] = round(lead, 1)
        leads[label].append(lead)
    leads['ruptura'].append(row['ruptura_lead_min'] or 0.0)
    rows.append(row)
report['targets'] = rows
false = []
for start, end, name in calm:
    for s in samples:
        if start <= s['ts'] < end and s['pred']:
            false.append((name, s['t'], s['failures']))
if KIND == 'replica':
    for start, end, name in allp:
        for s in samples:
            if start <= s['ts'] < end and 'replica_divergence' in s['failures']:
                false.append((name, s['t'], s['failures']))
report['false'] = len(false)
report['false_first'] = false[:5]
calm_fires = {}
for label, (w, h) in settings.items():
    n = 0
    for start, end, name in calm:
        n += len({pod for _, pod in prom_fires(w, h, start, end)})
    calm_fires[label] = n
report['predict_linear_calm_firing_pods'] = calm_fires
med = {k: round(st.median(v), 1) if v else 0 for k, v in leads.items()}
report['median_lead_min'] = med
better = max(med['30m/30m'], med['10m/30m'])
if KIND == 'release':
    r1 = len(rows) == 3 and all(r['ruptura_warn_min'] is not None and r['ruptura_warn_min'] <= 15 for r in rows)
else:
    r1 = len(rows) == 3 and all(r['ruptura_warn_min'] is not None and r['ruptura_warn_min'] <= 15 and r['names_leaker'] for r in rows)
report['rules'] = {'R1': r1, 'R2': med['ruptura'] >= better + 20, 'R3': len(false) == 0}
report['pass'] = all(report['rules'].values())
print(json.dumps(report, indent=1))
