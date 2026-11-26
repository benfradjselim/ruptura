#!/usr/bin/env python3
"""Score the catalog batch 1 run against docs/engine/CATALOG-BATCH-1.md.

Usage: score-batch1.py <dir with events.jsonl api.jsonl truth.jsonl>
Written before the run's data was read. T4 stop check added for rerun 1, before its data.
"""
import json
import sys
from datetime import datetime, timedelta
from pathlib import Path


def ts(s):
    return datetime.strptime(s, "%Y-%m-%dT%H:%M:%SZ")


def load(path):
    return [json.loads(line) for line in Path(path).read_text().splitlines() if line.strip()]


d = Path(sys.argv[1])
events = {e["event"]: ts(e["t"]) for e in load(d / "events.jsonl")}
api = load(d / "api.jsonl")
truth = load(d / "truth.jsonl")
results = []


def appearances(mode, target):
    """Poll times at which the failure was listed."""
    out = []
    for snap in api:
        for f in snap.get("failures") or []:
            if f.get("failureMode") == mode and f.get("target") == target:
                out.append((ts(snap["t"]), f))
                break
    return out


def listed_target(prefix):
    return any(
        f.get("target", "").startswith(prefix)
        for snap in api for f in (snap.get("failures") or [])
    )


def first_after(times, after):
    return next((t for t, _ in times if t >= after), None)


def cleared_after(times, after):
    """First poll at or after `after` where the failure is no longer listed."""
    listed = {t for t, _ in times}
    return next((ts(s["t"]) for s in api if ts(s["t"]) >= after and ts(s["t"]) not in listed), None)


def check(name, ok, detail):
    results.append((name, ok, detail))


def minutes(delta):
    return None if delta is None else round(delta.total_seconds() / 60, 1)


# T1 HPA
t1 = appearances("hpa_maxed_out", "ruptura-batch1/t1-burn")
at_max = next((ts(r["t"]) for r in truth if r["hpa"] and r["hpa"].split("/")[0] == r["hpa"].split("/")[1]), None)
raise_t1 = first_after(t1, at_max) if at_max else None
delay = minutes(raise_t1 - at_max) if raise_t1 else None
check("T1 raise 10-11 min after max", delay is not None and 10 <= delay <= 11, f"max at {at_max}, raised at {raise_t1}, delay {delay} min")
below = next((ts(r["t"]) for r in truth if "t1 load stopped" in events and ts(r["t"]) > events["t1 load stopped"]
              and r["hpa"] and r["hpa"].split("/")[0] < r["hpa"].split("/")[1]), None)
clear_t1 = cleared_after(t1, below) if below else None
check("T1 clear within 6 min of drop below max", clear_t1 is not None and minutes(clear_t1 - below) <= 6,
      f"below max at {below}, cleared at {clear_t1}")
check("T1 control silent", not listed_target("ruptura-batch1/t1-idle"), "")

# T2 PDB
t2 = appearances("pdb_blocks_drain", "ruptura-batch1/t2-one")
created = events.get("t2 created")
raise_t2 = first_after(t2, created)
delay = minutes(raise_t2 - created) if raise_t2 else None
check("T2 raise 10-11 min after creation", delay is not None and 10 <= delay <= 11, f"raised at {raise_t2}, delay {delay} min")
deleted = events.get("t2 pdb deleted")
clear_t2 = cleared_after(t2, deleted) if deleted else None
check("T2 clear within 3 min of deletion", clear_t2 is not None and minutes(clear_t2 - deleted) <= 3, f"deleted {deleted}, cleared {clear_t2}")
check("T2 control silent", not listed_target("ruptura-batch1/t2-two"), "")

# T3 Job
t3 = appearances("job_failed", "ruptura-batch1/t3-fail")
failed_at = None
for r in truth:
    for part in (r["job"] or "").split():
        if part.startswith("Failed="):
            failed_at = ts(part.split("=", 1)[1])
    if failed_at:
        break
raise_t3 = first_after(t3, failed_at) if failed_at else None
delay = minutes(raise_t3 - failed_at) if raise_t3 else None
check("T3 raise within 2 min of Failed", delay is not None and delay <= 2, f"Failed at {failed_at}, raised {raise_t3}, delay {delay} min")
deleted = events.get("t3 jobs deleted")
clear_t3 = cleared_after(t3, deleted) if deleted else None
check("T3 clear within 2 min of deletion", clear_t3 is not None and minutes(clear_t3 - deleted) <= 2, f"deleted {deleted}, cleared {clear_t3}")
check("T3 control silent", not listed_target("ruptura-batch1/t3-ok"), "")


def full_time(points, capacity):
    """Least-squares line through (t, used); time when used reaches capacity."""
    if len(points) < 2:
        return None
    t0 = points[0][0]
    xs = [(t - t0).total_seconds() for t, _ in points]
    ys = [u for _, u in points]
    n = len(xs)
    mx, my = sum(xs) / n, sum(ys) / n
    sxx = sum((x - mx) ** 2 for x in xs)
    if sxx == 0:
        return None
    slope = sum((x - mx) * (y - my) for x, y in zip(xs, ys)) / sxx
    if slope <= 0:
        return None
    return t0 + timedelta(seconds=(capacity - (my - slope * mx)) / slope)


def score_forecasts(name, forecasts, full, lead_min, window_min):
    if full is None or not forecasts:
        check(f"{name} first forecast >= {lead_min} min before full", False, f"full {full}, {len(forecasts)} forecasts")
        check(f"{name} accuracy from {window_min} min before full", False, "")
        return
    first = forecasts[0][0]
    check(f"{name} first forecast >= {lead_min} min before full", minutes(full - first) >= lead_min, f"first {first}, full {full}, lead {minutes(full - first)} min")
    bad = []
    for t, f in forecasts:
        left = (full - t).total_seconds()
        if left > window_min * 60 or left <= 0:
            continue
        eta, early, late = f.get("etaSeconds", 0), f.get("etaEarlySeconds", 0), f.get("etaLateSeconds", 0)
        inside = early and late and early <= left <= late
        close = abs(eta - left) <= 0.25 * left
        if not (inside or close):
            bad.append(f"{t:%H:%M:%S} eta {eta}s true {int(left)}s")
    check(f"{name} accuracy from {window_min} min before full", not bad, "; ".join(bad[:5]) or "all within range or 25 %")


# T4 node filesystem
fs_points, capacity = [], None
for r in truth:
    if r["fs"] and "/" in r["fs"]:
        used, size = (float(x) for x in r["fs"].strip("/").split("/")[:2])
        capacity = size
        if not fs_points or used > fs_points[-1][1]:
            fs_points.append((ts(r["t"]), used))
full4 = full_time(fs_points, capacity) if capacity else None
fs_target = next((f["target"] for s in api for f in (s.get("failures") or [])
                  if f.get("failureMode") == "node_filesystem_filling" and f.get("target", "").endswith("/dev/loop1")), None)
score_forecasts("T4", appearances("node_filesystem_filling", fs_target) if fs_target else [], full4, 15, 20)
t4_stop = events.get("t4 fill stopped at 95%")
if t4_stop and fs_target:
    after = [t for t, _ in appearances("node_filesystem_filling", fs_target) if t > t4_stop + timedelta(minutes=2)]
    check("T4 stop: forecast withdrawn within 2 min of the fill stop", not after, f"stop {t4_stop}, still listed at {after[:3]}")
else:
    check("T4 stop: forecast withdrawn within 2 min of the fill stop", False, f"stop {t4_stop}, target {fs_target}")
check("T4 control silent", not any(f.get("target", "").endswith("/dev/loop2") for s in api for f in (s.get("failures") or [])), "")

# T5 quota
q_points = [(ts(r["t"]), float(r["quota_cm"])) for r in truth if r["quota_cm"]]
reached = next((t for t, n in q_points if n >= 60), None)
rising = [(t, n) for t, n in q_points if reached is None or t <= reached]
full5 = reached or full_time(rising, 60)
score_forecasts("T5", appearances("quota_exhaustion", "ruptura-b1-quota/q/configmaps"), full5, 10, 15)
check("T5 control silent", not listed_target("ruptura-b1-qctl/"), "")

for name, ok, detail in results:
    print(f"{'PASS' if ok else 'FAIL'}  {name}  {detail}")
print(f"\n{sum(ok for _, ok, _ in results)}/{len(results)} checks passed")
