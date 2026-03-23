"""EXPORTER service (port 8005) - Prometheus metrics + dashboard data aggregation."""
import logging
import sys
from datetime import datetime, timezone, timedelta

import uvicorn
from fastapi import FastAPI, Response
from prometheus_client import Counter, Gauge, generate_latest, CONTENT_TYPE_LATEST

sys.path.insert(0, "/app")
from shared import config, database
from shared.models import (
    AnomalySummary, DashboardData, DetectionResult, HealthResponse,
    TimeSeriesPoint,
)

logging.basicConfig(
    level=config.get_str("LOG_LEVEL"),
    format="%(asctime)s [%(name)s] %(levelname)s %(message)s",
)
logger = logging.getLogger("exporter")

app = FastAPI(title="MLOps Exporter", version="1.0.0")

PREFIX = config.get_str("METRICS_PREFIX")

# Prometheus metrics
_anomaly_total = Counter(f"{PREFIX}_total", "Total anomalies detected")
_anomaly_rate = Gauge(f"{PREFIX}_rate", "Anomaly rate (last 24h)")
_predictions_total = Counter(f"{PREFIX}_predictions_total", "Total predictions")
_cpu_gauge = Gauge(f"{PREFIX}_cpu_usage", "Latest CPU usage (normalized)")
_memory_gauge = Gauge(f"{PREFIX}_memory_usage", "Latest memory usage (normalized)")

_last_export_count = 0


def _get_summary(window_hours: int = 24) -> AnomalySummary:
    since = (datetime.now(timezone.utc) - timedelta(hours=window_hours)).isoformat()
    with database.get_conn() as conn:
        total = conn.execute(
            "SELECT COUNT(*) FROM anomalies WHERE timestamp >= ?", (since,)
        ).fetchone()[0]
        anomalies = conn.execute(
            "SELECT COUNT(*) FROM anomalies WHERE is_anomaly=1 AND timestamp >= ?", (since,)
        ).fetchone()[0]
    rate = anomalies / total if total > 0 else 0.0
    return AnomalySummary(
        total_anomalies_24h=anomalies,
        anomaly_rate=rate,
        total_predictions=total,
    )


def _get_anomaly_series(window_hours: int = 24) -> list[TimeSeriesPoint]:
    since = (datetime.now(timezone.utc) - timedelta(hours=window_hours)).isoformat()
    with database.get_conn() as conn:
        rows = conn.execute(
            """SELECT timestamp, anomaly_score, is_anomaly
               FROM anomalies WHERE timestamp >= ?
               ORDER BY timestamp""",
            (since,),
        ).fetchall()
    return [
        TimeSeriesPoint(
            timestamp=r["timestamp"],
            value=r["anomaly_score"],
            is_anomaly=bool(r["is_anomaly"]),
        )
        for r in rows
    ]


def _get_metric_series(window_hours: int = 24) -> list[TimeSeriesPoint]:
    since = (datetime.now(timezone.utc) - timedelta(hours=window_hours)).isoformat()
    with database.get_conn() as conn:
        rows = conn.execute(
            """SELECT timestamp, cpu_norm as value
               FROM processed_data WHERE timestamp >= ?
               ORDER BY timestamp""",
            (since,),
        ).fetchall()
    return [
        TimeSeriesPoint(timestamp=r["timestamp"], value=r["value"] or 0.0)
        for r in rows
    ]


def _get_recent_anomalies(limit: int = 20) -> list[DetectionResult]:
    with database.get_conn() as conn:
        rows = conn.execute(
            """SELECT a.id, a.timestamp, a.anomaly_score, a.is_anomaly, a.pod_name, a.namespace
               FROM anomalies a
               WHERE a.is_anomaly=1
               ORDER BY a.timestamp DESC
               LIMIT ?""",
            (limit,),
        ).fetchall()
    return [
        DetectionResult(
            id=r["id"],
            timestamp=r["timestamp"],
            anomaly_score=r["anomaly_score"],
            is_anomaly=bool(r["is_anomaly"]),
            pod_name=r["pod_name"],
            namespace=r["namespace"],
        )
        for r in rows
    ]


def _update_prometheus_gauges() -> None:
    """Update Prometheus gauges from latest DB data."""
    with database.get_conn() as conn:
        latest_metric = conn.execute(
            "SELECT cpu_norm, memory_norm FROM processed_data ORDER BY created_at DESC LIMIT 1"
        ).fetchone()
        if latest_metric:
            _cpu_gauge.set(latest_metric["cpu_norm"] or 0.0)
            _memory_gauge.set(latest_metric["memory_norm"] or 0.0)

        summary = conn.execute(
            """SELECT COUNT(*) as total, SUM(is_anomaly) as anomalies
               FROM anomalies
               WHERE timestamp >= datetime('now', '-24 hours')"""
        ).fetchone()
        if summary:
            total = summary["total"] or 0
            anomalies = summary["anomalies"] or 0
            _anomaly_rate.set(anomalies / total if total > 0 else 0.0)


@app.on_event("startup")
async def startup() -> None:
    database.init_db()
    logger.info("Exporter started")


@app.get("/health", response_model=HealthResponse)
async def health() -> HealthResponse:
    return HealthResponse(status="ok", service="exporter")


@app.get("/metrics")
async def metrics() -> Response:
    _update_prometheus_gauges()
    return Response(content=generate_latest(), media_type=CONTENT_TYPE_LATEST)


@app.get("/dashboard-data", response_model=DashboardData)
async def dashboard_data(window: str = "1h") -> DashboardData:
    hours = int(window.replace("h", "")) if window.endswith("h") else 1
    return DashboardData(
        anomaly_series=_get_anomaly_series(hours),
        metric_series=_get_metric_series(hours),
        summary=_get_summary(hours),
        recent_anomalies=_get_recent_anomalies(),
        window=window,
    )


@app.get("/summary", response_model=AnomalySummary)
async def summary() -> AnomalySummary:
    return _get_summary()


if __name__ == "__main__":
    uvicorn.run("main:app", host="0.0.0.0", port=config.get_int("EXPORTER_PORT"), reload=False)
