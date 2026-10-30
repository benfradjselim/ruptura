#!/usr/bin/env python3
"""Score a recorded Ruptura fleet run against the prediction gate.

Input is a JSONL file of {"t": ISO8601, "fleet": <GET /api/v2/fleet>} samples.
A failure is a restart_count increase on the target workload. A warning is any
calibrated sample in the same cycle that reports degraded, critical, or a fused
rupture index at or above the alert threshold. Exits 0 only when the gate passes.
"""
import argparse
import json
import sys
from datetime import datetime

ALERT_FRI = 1.5
WARNING_STATES = {"degraded", "critical"}
READY_STATES = {"healthy", "degraded", "critical"}


def parse_time(value):
    return datetime.strptime(value[:19], "%Y-%m-%dT%H:%M:%S")


def load_samples(path, target, control):
    samples = []
    with open(path, encoding="utf-8") as handle:
        for line in handle:
            row = json.loads(line)
            hosts = {h["host"]: h for h in row["fleet"].get("hosts", [])}
            samples.append((parse_time(row["t"]), hosts))
    return [s for s in samples if target in s[1] or control in s[1]]


def is_warning(host):
    if host.get("state") not in READY_STATES:
        return False
    return (
        host.get("state") in WARNING_STATES
        or host.get("fused_rupture_index", 0) >= ALERT_FRI
    )


def score(samples, target, control):
    leads = []
    first_warning = None
    previous = None
    for when, hosts in samples:
        host = hosts.get(target)
        if host is None:
            continue
        restarts = host.get("restart_count") or 0
        if previous is not None and restarts > previous:
            lead = None
            if first_warning is not None:
                lead = round((when - first_warning).total_seconds() / 60, 1)
            leads.append(lead)
            first_warning = None
        elif first_warning is None and is_warning(host):
            first_warning = when
        previous = restarts

    last_hosts = samples[-1][1] if samples else {}
    false_alarms = sum(1 for _, h in samples if control in h and is_warning(h[control]))
    return {
        "failures": len(leads),
        "lead_minutes": leads,
        "control_false_alarms": false_alarms,
        "fri_nonzero_hosts": sum(
            1 for h in last_hosts.values() if h.get("fused_rupture_index", 0) > 0
        ),
        "total_hosts": len(last_hosts),
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("recording")
    parser.add_argument("--target", default="demo-shop/Deployment/leak-orders")
    parser.add_argument("--control", default="demo-shop/Deployment/stable-web")
    parser.add_argument("--min-lead", type=float, default=10.0)
    parser.add_argument("--min-failures", type=int, default=3)
    args = parser.parse_args()

    samples = load_samples(args.recording, args.target, args.control)
    result = score(samples, args.target, args.control)
    predicted = [v for v in result["lead_minutes"] if v is not None and v >= args.min_lead]
    result["predicted"] = len(predicted)
    result["gate_pass"] = (
        result["failures"] >= args.min_failures
        and len(predicted) == result["failures"]
        and result["control_false_alarms"] == 0
    )
    print(json.dumps(result, indent=2))
    return 0 if result["gate_pass"] else 1


if __name__ == "__main__":
    sys.exit(main())
