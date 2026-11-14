#!/usr/bin/env python3
"""Nov 9 checkpoint: Ruptura against predict_linear on the same window.

Frozen 2026-11-03, before any checkpoint data. No setting below changes after
seeing a result; a change needs a new dated amendment in the design.

Runs on the lab node:
  (echo "cat > /tmp/checkpoint.py <<'P'"; cat scripts/lab-eval/checkpoint.py; echo P;
   echo "python3 /tmp/checkpoint.py 2026-11-09T08:00 2026-11-09T14:00 /var/tmp/fleet.jsonl") | /root/lab.sh

Failures: container restarts of the scored workloads, all of them OOM by design.
  leak-orders  linear leak, reference, a tie or loss is expected
  step-leak    stepped leak
  accel-leak   accelerating leak
  oom-spike    sudden jump, no trend
  periodic-spike  sudden jump every 45 min, not a crash loop (amendment 3)
Contestants: predict_linear on working set against the memory limit, 30m window
30m horizon and 10m window 30m horizon; Ruptura, a predicted failure listed with
an ETA of 30 min or less. Ruptura is scored twice: saturation forecasts only, and
with the failure memory (recurrent_failure) added.
Warning of a failure, two measures, both printed:
  fresh     the first rising edge inside the cycle. This is the primary measure.
  standing  the alert is on at some point from 2 min after the previous failure,
            lead from that first time. Secondary, it credits leftovers.
The share of cycle time firing is printed too, since an alert on all the time
carries little information. Lead under 5 min counts as missed.
Amendment 2026-11-03, after a dry run on 2026-11-02 data and before any
checkpoint data: the first version used the fresh measure only, which scored a
standing recurrent warning on oom-spike as 0 of 41 and cut predict_linear
30m/30m to 5 to 8 min against 26 to 33 min measured before.
Amendment 2, 2026-11-03 01:00 UTC, still before any checkpoint data: fresh is
primary again. The timeline of leak-orders 20:00 to 22:00 on 2026-11-02 shows
predict_linear 30m/30m firing 20:31 to 20:40 around the 20:36 OOM: its 30m window
still holds the previous ramp, so the alert outlives the failure by about 4 min
and standing credits that leftover as a 28 min lead on the next cycle. oom-spike
was 0 of 41 because crash_loop is listed instead of the forecast, not because of
the measure.
Amendment 3, same time, before any data of it: periodic-spike added, a fault the
failure memory can warn on without being a crash loop. oom-spike stays as it is.
A cycle counts only when every contestant had data at its start: 30 min after
the window start, and 25 min after the first Ruptura sample (calibration).
False alarms: a firing episode, from rising to falling edge, not followed by a
restart of the same workload within 35 min of its start. Counted on every
workload of the cluster, and listed for the controls sawtooth-batch, stable-web
and cpu-batch.

Nov 9 checkpoint, pre-registered 2026-11-03 01:20 UTC, before any of its data.
Window: 2026-11-09 06:00 to 12:00 UTC. The engine runs from before 05:00 UTC and is
not redeployed until 12:15 UTC. `checkpoint-preflight.sh` must pass at about 05:45.
Validity: no engine restart inside the window, and every scored fault has at
least 5 scored cycles. If either fails, the window is void and reruns the next day
with these same rules; it is never rescored with other rules.
Position tested: on memory failures Ruptura warns earlier than the quiet
predict_linear setting, with fewer false alarms than the fast one, and warns on a
periodic spike that no trend rule sees.
  C1 trends (leak-orders, step-leak, accel-leak pooled, fresh measure): Ruptura
     saturation warns at least as many cycles as predict_linear 30m/30m, with a
     median lead at least 5 min longer.
  C2 noise: Ruptura with failure memory has at most half the cluster false
     episodes of predict_linear 10m/30m, and at most 2 more than 30m/30m.
  C3 periodic-spike: Ruptura with failure memory warns at least 60 percent of
     scored cycles; each predict_linear setting warns at most 1.
  oom-spike is reported but not judged: a crash loop, detected, not forecast.
Confirmed if C1, C2 and C3 pass. Otherwise the position is corrected by stating
each failed criterion as measured, and the result is recorded as it is.
"""
import collections, datetime as D, json, re, subprocess, sys, urllib.parse as U

STEP, HORIZON, MIN_LEAD, MATCH, GRACE = 30, 1800, 300, 2100, 120
SCORED = ['leak-orders', 'step-leak', 'accel-leak', 'oom-spike', 'periodic-spike']
CONTROLS = ['sawtooth-batch', 'stable-web', 'cpu-batch']
RULES = [('30m', 1800), ('10m', 1800)]
CALIBRATION = 1500


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


def workload(namespace, pod):
    return namespace + '/' + re.sub(r'-[a-z0-9]{6,10}-[a-z0-9]{5}$', '', pod)


def series_by_workload(result):
    times = collections.defaultdict(set)
    for s in result:
        key = workload(s['metric'].get('namespace', ''), s['metric'].get('pod', ''))
        times[key].update(int(t) for t, _ in s['values'])
    return times


def restarts():
    out = collections.defaultdict(list)
    for s in query('sum by (namespace,pod)(kube_pod_container_status_restarts_total)'):
        v = s['values']
        key = workload(s['metric']['namespace'], s['metric']['pod'])
        out[key] += [int(b[0]) for a, b in zip(v, v[1:]) if float(b[1]) > float(a[1])]
    return {k: sorted(v) for k, v in out.items()}


def episodes(samples, step):
    """Turns firing sample times into (start, end) episodes."""
    out = []
    for t in sorted(samples):
        if out and t - out[-1][1] <= step * 2:
            out[-1][1] = t
        else:
            out.append([t, t])
    return out


def leads(eps, failures, eligible_from):
    """Returns per cycle (standing lead, fresh lead, share of the cycle firing)."""
    result, prev = [], None
    for f in failures:
        if prev is None or prev < eligible_from:
            prev = f
            continue
        open_from = prev + GRACE
        on = [max(s, open_from) for s, e in eps if e >= open_from and s < f]
        rising = [s for s, _ in eps if prev < s < f]
        firing = sum(min(e + STEP, f) - max(s, prev) for s, e in eps if e >= prev and s < f)
        lead = lambda t: round((f - t) / 60, 1) if t and f - t >= MIN_LEAD else None
        result.append((lead(on[0] if on else None), lead(rising[0] if rising else None), round(firing / (f - prev), 2)))
        prev = f
    return result


def false_alarms(fires, failures_of):
    false = collections.Counter()
    for key, eps in fires.items():
        for start, _ in eps:
            if not any(start <= f <= start + MATCH for f in failures_of.get(key, [])):
                false[key] += 1
    return false


def ruptura():
    samples = []
    for path in FLEET:
        for line in open(path):
            try:
                row = json.loads(line)
            except ValueError:
                continue
            t = ts(row['t'])
            if START <= t <= END:
                samples.append((t, row['fleet'].get('hosts', [])))
    samples.sort(key=lambda s: s[0])
    step = max(STEP, min((b[0] - a[0] for a, b in zip(samples, samples[1:])), default=STEP))
    saturation, memory = collections.defaultdict(set), collections.defaultdict(set)
    for t, hosts in samples:
        for host in hosts:
            failure = host.get('failure') or {}
            if failure.get('kind') != 'predicted' or not 0 < (failure.get('etaSeconds') or 0) <= HORIZON:
                continue
            parts = host['host'].split('/')
            key = parts[0] + '/' + parts[-1]
            memory[key].add(t)
            if failure.get('failureMode') == 'saturation':
                saturation[key].add(t)
    first = samples[0][0] if samples else END
    to_eps = lambda d: {k: episodes(v, step) for k, v in d.items()}
    return first, to_eps(saturation), to_eps(memory)


def main():
    failures = restarts()
    first, saturation, memory = ruptura()
    eligible = max(START + 1800, first + CALIBRATION)
    entries = []
    for window, horizon in RULES:
        use = 'sum by (namespace,pod)(container_memory_working_set_bytes{container!="",container!="POD"})'
        limit = 'sum by (namespace,pod)(kube_pod_container_resource_limits{resource="memory"})'
        fired = series_by_workload(query(f'(predict_linear(({use})[{window}:30s],{horizon}) > {limit}) and ({use} > 0)'))
        entries.append((f'predict_linear {window}/{horizon // 60}m', {k: episodes(v, STEP) for k, v in fired.items()}))
    entries += [('Ruptura saturation', saturation), ('Ruptura + failure memory', memory)]

    print(f'window {sys.argv[1]} to {sys.argv[2]} UTC, cycles count from {D.datetime.utcfromtimestamp(eligible):%H:%M} UTC')
    keys = {name: next((k for k in failures if k.endswith('/' + name)), 'demo-shop/' + name) for name in SCORED + CONTROLS}
    for name, fires in entries:
        print(f'\n{name}')
        total, warned, fresh, all_leads = 0, 0, 0, []
        for wl in SCORED:
            got = leads(fires.get(keys[wl], []), failures.get(keys[wl], []), eligible)
            hit = [c[1] for c in got if c[1]]
            fresh += sum(1 for c in got if c[0])
            total, warned, all_leads = total + len(got), warned + len(hit), all_leads + hit
            share = round(sum(c[2] for c in got) / len(got), 2) if got else None
            print(f'  {wl:12} {len(hit)} of {len(got)} warned fresh, {sum(1 for c in got if c[0])} standing, firing share {share}')
            print(f'  {"":12} standing leads {[c[0] for c in got]}')
            print(f'  {"":12} fresh leads    {[c[1] for c in got]}')
        false = false_alarms(fires, failures)
        median = sorted(all_leads)[len(all_leads) // 2] if all_leads else None
        print(f'  total {warned} of {total} warned fresh, {fresh} standing, median fresh lead {median} min')
        print(f'  false episodes on controls ' + ', '.join(f'{c} {false[keys[c]]}' for c in CONTROLS))
        print(f'  false episodes on the cluster {sum(false.values())} on {len(false)} workloads, worst {false.most_common(4)}')


main()
