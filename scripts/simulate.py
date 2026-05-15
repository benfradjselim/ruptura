#!/usr/bin/env python3
"""
Ruptura workload simulator — injects 5 synthetic workloads with distinct
failure modes via the /api/v2/write endpoint (Prometheus remote-write JSON).

Usage:
    python3 scripts/simulate.py [--host HOST] [--port PORT] [--interval SEC]

Default target: http://185.229.225.115:31470

Workloads:
  gateway          — stable, healthy, normal traffic
  order-service    — CPU-stressed, rising fatigue, heading toward rupture
  payment-api      — high error rate + contagion spreading
  cache-worker     — sudden traffic spike (velocity + throughput surge)
  ml-inference     — brand new workload (calibrating, no history)
"""

import argparse
import json
import math
import random
import sys
import time
import urllib.request
import urllib.error

DEFAULT_HOST = "185.229.225.115"
DEFAULT_PORT = 31470
DEFAULT_INTERVAL = 5  # seconds between pushes

NAMESPACE = "default"


def ts_ms() -> int:
    return int(time.time() * 1000)


def jitter(base: float, pct: float = 0.05) -> float:
    """Add ±pct% random noise so values don't look synthetic."""
    return base * (1 + random.uniform(-pct, pct))


def post(url: str, payload: dict) -> None:
    body = json.dumps(payload).encode()
    req = urllib.request.Request(
        url,
        data=body,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=5) as resp:
            if resp.status not in (200, 204):
                print(f"  [WARN] {resp.status} from {url}", flush=True)
    except urllib.error.URLError as e:
        print(f"  [ERR] {url}: {e}", flush=True)


def make_timeseries(deployment: str, metric: str, value: float) -> dict:
    """Build one timeseries entry for /api/v2/write.
    The 'host' label must equal the workload key (namespace/Kind/name)
    because the pipeline indexes by host string, not workload ref."""
    host = f"{NAMESPACE}/Deployment/{deployment}"
    return {
        "Labels": [
            {"Name": "__name__",  "Value": metric},
            {"Name": "host",      "Value": host},
            {"Name": "namespace", "Value": NAMESPACE},
            {"Name": "deployment","Value": deployment},
        ],
        "Samples": [{"Value": round(value, 4), "Timestamp": ts_ms()}],
    }


def push_workload(url: str, deployment: str, metrics: dict) -> None:
    timeseries = [
        make_timeseries(deployment, name, value)
        for name, value in metrics.items()
    ]
    post(url, {"timeseries": timeseries})


def workload_gateway(t: float) -> dict:
    """Stable, well-behaved service — all green."""
    return {
        "cpu_percent":    jitter(22.0),
        "memory_percent": jitter(38.0),
        "latency_p99":    jitter(95.0),     # ms
        "error_rate":     jitter(0.003),    # ~0.3%
        "request_rate":   jitter(320.0),    # req/s
    }


def workload_order_service(t: float) -> dict:
    """CPU-stressed, gradually worsening — classic slow-burn rupture."""
    # Stress rises over ~10 minutes then plateaus around 85%
    cycle = (t % 600) / 600          # 0..1 over 10 min
    cpu   = 45 + 45 * math.sin(cycle * math.pi / 2)  # 45→90
    lat   = 120 + 400 * (cpu / 90)   # latency grows with CPU
    err   = 0.01 + 0.05 * (cpu / 90) # errors creep up too
    return {
        "cpu_percent":    jitter(cpu, 0.03),
        "memory_percent": jitter(62.0),
        "latency_p99":    jitter(lat, 0.08),
        "error_rate":     jitter(err, 0.10),
        "request_rate":   jitter(180.0),
    }


def workload_payment_api(t: float) -> dict:
    """High error rate service — errors spike every ~2 minutes."""
    # Error bursts every 120 s
    burst = math.sin(math.pi * (t % 120) / 60) ** 2  # 0→1→0
    error_rate = 0.08 + 0.35 * burst
    # Latency spikes during error burst (timeout accumulation)
    lat = 200 + 2800 * burst
    return {
        "cpu_percent":    jitter(55.0 + 20 * burst),
        "memory_percent": jitter(70.0),
        "latency_p99":    jitter(lat, 0.10),
        "error_rate":     jitter(error_rate, 0.05),
        "request_rate":   jitter(95.0),
    }


def workload_cache_worker(t: float) -> dict:
    """Traffic spike — request rate and throughput surge every ~3 minutes."""
    # Sawtooth spike every 180 s
    phase = (t % 180) / 180
    spike = max(0.0, math.sin(phase * 2 * math.pi)) ** 0.5
    rps   = 150 + 1200 * spike
    cpu   = 18 + 55 * spike
    tput  = 40 + 800 * spike          # MB/s equivalent
    return {
        "cpu_percent":    jitter(cpu, 0.04),
        "memory_percent": jitter(44.0 + 20 * spike),
        "latency_p99":    jitter(60 + 300 * spike, 0.06),
        "error_rate":     jitter(0.005 + 0.02 * spike),
        "request_rate":   jitter(rps, 0.05),
        "throughput":     jitter(tput, 0.08),
    }


def workload_ml_inference(t: float) -> dict:
    """Brand-new workload — very noisy, calibrating phase."""
    # Erratic values typical of an uncalibrated service
    noise = random.gauss(0, 1)
    return {
        "cpu_percent":    max(1, jitter(48 + 20 * noise, 0.20)),
        "memory_percent": max(1, jitter(55 + 10 * noise, 0.15)),
        "latency_p99":    max(10, jitter(350 + 150 * noise, 0.25)),
        "error_rate":     max(0, jitter(0.04 + 0.03 * noise, 0.30)),
        "request_rate":   max(1, jitter(60 + 30 * noise, 0.20)),
    }


WORKLOADS = [
    ("gateway",         workload_gateway,        "stable    "),
    ("order-service",   workload_order_service,  "stressed  "),
    ("payment-api",     workload_payment_api,    "errors    "),
    ("cache-worker",    workload_cache_worker,   "traffic↑  "),
    ("ml-inference",    workload_ml_inference,   "calibrating"),
]


def main() -> None:
    p = argparse.ArgumentParser(description="Ruptura workload simulator")
    p.add_argument("--host",     default=DEFAULT_HOST)
    p.add_argument("--port",     type=int, default=DEFAULT_PORT)
    p.add_argument("--interval", type=float, default=DEFAULT_INTERVAL)
    args = p.parse_args()

    url = f"http://{args.host}:{args.port}/api/v2/write"
    print(f"Ruptura Workload Simulator")
    print(f"Target : {url}")
    print(f"Interval: {args.interval}s")
    print(f"Workloads: {len(WORKLOADS)}")
    print()

    t0 = time.time()
    tick = 0

    while True:
        t = time.time() - t0
        tick += 1
        print(f"[tick {tick:04d}  t={t:.0f}s]", flush=True)

        for deployment, fn, label in WORKLOADS:
            metrics = fn(t)
            push_workload(url, deployment, metrics)
            cpu = metrics.get("cpu_percent", 0)
            err = metrics.get("error_rate", 0)
            rps = metrics.get("request_rate", 0)
            print(
                f"  {deployment:<22} [{label}]  "
                f"cpu={cpu:5.1f}%  err={err*100:4.1f}%  rps={rps:6.1f}",
                flush=True,
            )

        print(flush=True)
        time.sleep(args.interval)


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print("\nSimulator stopped.")
        sys.exit(0)
