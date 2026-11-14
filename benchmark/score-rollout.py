import json, sys
from datetime import datetime, timedelta
ts = lambda s: datetime.strptime(s, "%Y-%m-%dT%H:%M:%SZ")
phases = [(ts(l[:20]), l[21:].strip()) for l in open(sys.argv[2]) if l.strip()]
rows = [json.loads(l) for l in open(sys.argv[1]) if l.strip()]
for r in rows: r["t"] = ts(r["t"])
def window(i):
    start = phases[i][0]; end = phases[i+1][0] if i+1 < len(phases) else rows[-1]["t"]
    return [r for r in rows if start <= r["t"] < end]
R1 = R2 = R3 = True; R4 = not any("replicas_unavailable" in r["failures"] for r in rows)
for i, (start, name) in enumerate(phases):
    w = window(i)
    if name.endswith("stuck rollout"):
        false_at = next((r["t"] for r in w if r["progressing"].startswith("False")), None)
        listed = next((r["t"] for r in w if "rollout_stuck" in r["failures"] and false_at and r["t"] >= false_at), None)
        ok = bool(false_at and listed and listed - false_at <= timedelta(minutes=5))
        R1 &= ok
        print(f"{name}: Progressing=False {false_at and false_at.time()}, rollout_stuck {listed and listed.time()}, delay {(listed-false_at).seconds if ok else '-'} s, {'ok' if ok else 'FAIL'}")
    elif name.endswith("undo"):
        late = [r for r in w if "rollout_stuck" in r["failures"] and r["t"] - start > timedelta(minutes=5)]
        last = max((r["t"] for r in w if "rollout_stuck" in r["failures"]), default=None)
        R2 &= not late
        print(f"{name}: last rollout_stuck {last and (last-start).seconds} s after undo, {'ok' if not late else 'FAIL'}")
    elif name.endswith("healthy rollout"):
        listed = sorted({r["failures"] for r in w if r["failures"]})
        R3 &= not listed
        print(f"{name}: {'nothing listed, ok' if not listed else 'FAIL ' + str(listed)}")
others = sorted({f for r in rows for f in r["failures"].split(",") if f and not f.startswith("rollout_stuck")})
print("other failures ever listed:", others or "none")
print(f"R1 {R1}  R2 {R2}  R3 {R3}  R4 {R4}  -> {'PASS' if R1 and R2 and R3 and R4 else 'FAIL'}")
