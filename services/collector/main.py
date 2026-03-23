"""COLLECTOR service (port 8001) - Prometheus metrics + K8s logs ingestion."""
import asyncio
import json
import logging
import os
import sys
import time
from datetime import datetime, timezone
from typing import Optional

import httpx
import psutil
import uvicorn
from fastapi import FastAPI, HTTPException

sys.path.insert(0, "/app")
from shared import config, database
from shared.models import CollectResponse, CollectorStatus, HealthResponse

logging.basicConfig(
    level=config.get_str("LOG_LEVEL"),
    format="%(asctime)s [%(name)s] %(levelname)s %(message)s",
)
logger = logging.getLogger("collector")

app = FastAPI(title="MLOps Collector", version="1.0.0")

_state: dict = {"last_run": None, "total_collected": 0}


# ---------------------------------------------------------------------------
# Prometheus scraper
# ---------------------------------------------------------------------------

async def scrape_prometheus() -> list[dict]:
    """Query Prometheus for system metrics."""
    prometheus_url = config.get_str("PROMETHEUS_URL")
    queries = {
        "cpu_usage": 'avg(rate(node_cpu_seconds_total{mode!="idle"}[1m])) * 100',
        "memory_usage": "node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes",
        "latency_ms": 'histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[1m])) * 1000',
    }
    rows = []
    timestamp = datetime.now(timezone.utc).isoformat()
    async with httpx.AsyncClient(timeout=5.0) as client:
        for metric_name, query in queries.items():
            try:
                resp = await client.get(
                    f"{prometheus_url}/api/v1/query", params={"query": query}
                )
                resp.raise_for_status()
                data = resp.json()
                for result in data.get("data", {}).get("result", []):
                    value = float(result["value"][1])
                    labels = json.dumps(result.get("metric", {}))
                    rows.append({
                        "timestamp": timestamp,
                        "source": "prometheus",
                        "metric_name": metric_name,
                        "value": value,
                        "labels": labels,
                    })
            except Exception as exc:
                logger.warning("Prometheus query failed for %s: %s", metric_name, exc)
    return rows


def scrape_psutil() -> list[dict]:
    """Fallback: collect local system metrics via psutil."""
    timestamp = datetime.now(timezone.utc).isoformat()
    cpu = psutil.cpu_percent(interval=1)
    mem = psutil.virtual_memory()
    return [
        {
            "timestamp": timestamp,
            "source": "psutil",
            "metric_name": "cpu_usage",
            "value": cpu,
            "labels": "{}",
        },
        {
            "timestamp": timestamp,
            "source": "psutil",
            "metric_name": "memory_usage",
            "value": mem.percent,
            "labels": "{}",
        },
    ]


# ---------------------------------------------------------------------------
# K8s log collector
# ---------------------------------------------------------------------------

async def collect_k8s_logs() -> list[dict]:
    """Collect pod logs via Kubernetes API."""
    rows: list[dict] = []
    try:
        from kubernetes import client as k8s_client, config as k8s_config

        try:
            k8s_config.load_incluster_config()
        except Exception:
            k8s_config.load_kube_config()

        v1 = k8s_client.CoreV1Api()
        namespaces = config.get_str("K8S_NAMESPACES").split(",")
        tail = config.get_int("K8S_LOG_TAIL_LINES")
        timestamp = datetime.now(timezone.utc).isoformat()

        for ns in namespaces:
            try:
                pods = v1.list_namespaced_pod(namespace=ns.strip())
                for pod in pods.items:
                    for container in pod.spec.containers:
                        try:
                            log_text = v1.read_namespaced_pod_log(
                                name=pod.metadata.name,
                                namespace=ns.strip(),
                                container=container.name,
                                tail_lines=tail,
                            )
                            for line in log_text.splitlines():
                                if not line.strip():
                                    continue
                                level = _extract_log_level(line)
                                rows.append({
                                    "timestamp": timestamp,
                                    "namespace": ns.strip(),
                                    "pod_name": pod.metadata.name,
                                    "container": container.name,
                                    "log_level": level,
                                    "message": line[:1000],
                                })
                        except Exception as log_exc:
                            logger.debug(
                                "Failed to read logs for %s/%s: %s",
                                pod.metadata.name, container.name, log_exc,
                            )
            except Exception as exc:
                logger.debug("K8s log collect error for ns %s: %s", ns, exc)
    except ImportError:
        logger.debug("kubernetes package not available, skipping log collection")
    return rows


def _extract_log_level(line: str) -> Optional[str]:
    upper = line.upper()
    for level in ("ERROR", "WARN", "WARNING", "INFO", "DEBUG"):
        if level in upper:
            return level
    return None


# ---------------------------------------------------------------------------
# Core collection logic
# ---------------------------------------------------------------------------

async def run_collection() -> int:
    """Collect metrics + logs, write to DB, trigger processor."""
    database.init_db()

    # Collect metrics
    try:
        metrics = await scrape_prometheus()
    except Exception:
        metrics = []

    if not metrics:
        loop = asyncio.get_event_loop()
        metrics = await loop.run_in_executor(None, scrape_psutil)

    # Collect logs
    logs = await collect_k8s_logs()

    # Write to DB
    collected = 0
    with database.get_conn() as conn:
        for m in metrics:
            conn.execute(
                "INSERT INTO raw_metrics (timestamp, source, metric_name, value, labels) "
                "VALUES (?, ?, ?, ?, ?)",
                (m["timestamp"], m["source"], m["metric_name"], m["value"], m["labels"]),
            )
            collected += 1

        for log in logs:
            conn.execute(
                "INSERT INTO raw_logs (timestamp, namespace, pod_name, container, log_level, message) "
                "VALUES (?, ?, ?, ?, ?, ?)",
                (log["timestamp"], log["namespace"], log["pod_name"],
                 log["container"], log["log_level"], log["message"]),
            )
            collected += 1

    # Trigger processor (fire-and-forget)
    processor_url = config.get_str("PROCESSOR_URL")
    try:
        async with httpx.AsyncClient(timeout=5.0) as client:
            await client.post(f"{processor_url}/process", json={})
    except Exception as exc:
        logger.debug("Processor trigger failed (will retry): %s", exc)

    _state["last_run"] = datetime.now(timezone.utc).isoformat()
    _state["total_collected"] = _state["total_collected"] + collected
    logger.info("Collected %d data points", collected)
    return collected


# ---------------------------------------------------------------------------
# Background collection loop
# ---------------------------------------------------------------------------

async def _collection_loop() -> None:
    interval = config.get_int("COLLECT_INTERVAL_SEC")
    while True:
        try:
            await run_collection()
        except Exception as exc:
            logger.error("Collection error: %s", exc)
        await asyncio.sleep(interval)


@app.on_event("startup")
async def startup() -> None:
    database.init_db()
    asyncio.create_task(_collection_loop())
    logger.info("Collector started, interval=%ds", config.get_int("COLLECT_INTERVAL_SEC"))


# ---------------------------------------------------------------------------
# API endpoints
# ---------------------------------------------------------------------------

@app.get("/health", response_model=HealthResponse)
async def health() -> HealthResponse:
    return HealthResponse(status="ok", service="collector")


@app.post("/collect", response_model=CollectResponse)
async def collect() -> CollectResponse:
    count = await run_collection()
    return CollectResponse(
        collected=count,
        timestamp=datetime.now(timezone.utc).isoformat(),
    )


@app.get("/status", response_model=CollectorStatus)
async def status() -> CollectorStatus:
    return CollectorStatus(
        last_run=_state["last_run"],
        total_collected=_state["total_collected"],
    )


if __name__ == "__main__":
    uvicorn.run("main:app", host="0.0.0.0", port=config.get_int("COLLECTOR_PORT"), reload=False)
