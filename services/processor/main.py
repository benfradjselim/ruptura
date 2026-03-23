"""PROCESSOR service (port 8002) - Data cleaning, normalization, feature engineering."""
import asyncio
import logging
import sys
from datetime import datetime, timezone

import httpx
import uvicorn
from fastapi import FastAPI

sys.path.insert(0, "/app")
from shared import config, database
from shared.models import HealthResponse, ProcessRequest, ProcessResponse

logging.basicConfig(
    level=config.get_str("LOG_LEVEL"),
    format="%(asctime)s [%(name)s] %(levelname)s %(message)s",
)
logger = logging.getLogger("processor")

app = FastAPI(title="MLOps Processor", version="1.0.0")

# Running min/max for online MinMax normalization
_minmax: dict[str, dict[str, float]] = {}


def _normalize_minmax(name: str, value: float) -> float:
    """Online MinMax normalization: returns value in [0, 1]."""
    if name not in _minmax:
        _minmax[name] = {"min": value, "max": value}
    stats = _minmax[name]
    stats["min"] = min(stats["min"], value)
    stats["max"] = max(stats["max"], value)
    rng = stats["max"] - stats["min"]
    if rng == 0:
        return 0.0
    return (value - stats["min"]) / rng


def _build_feature_vector(metrics_by_name: dict[str, float]) -> dict[str, float]:
    """Build a normalized feature vector from a dict of {metric_name: value}."""
    return {
        "cpu_norm": _normalize_minmax("cpu_usage", metrics_by_name.get("cpu_usage", 0.0)),
        "memory_norm": _normalize_minmax("memory_usage", metrics_by_name.get("memory_usage", 0.0)),
        "latency_norm": _normalize_minmax("latency_ms", metrics_by_name.get("latency_ms", 0.0)),
        "error_rate": 0.0,
        "log_volume": 0.0,
        "restart_count": 0.0,
    }


async def _process_batch(batch_ids: list[int] | None = None) -> int:
    """Process unprocessed raw metrics rows into processed_data.

    Groups raw metrics rows by timestamp so each collection event produces
    exactly one processed_data row with a complete feature vector.
    """
    limit = config.get_int("BATCH_SIZE")

    with database.get_conn() as conn:
        if batch_ids:
            placeholders = ",".join("?" * len(batch_ids))
            rows = conn.execute(
                f"SELECT * FROM raw_metrics WHERE id IN ({placeholders}) AND processed=0",
                batch_ids,
            ).fetchall()
        else:
            rows = conn.execute(
                "SELECT * FROM raw_metrics WHERE processed=0 ORDER BY created_at LIMIT ?",
                (limit,),
            ).fetchall()

        if not rows:
            return 0

        # Group rows by timestamp: each collection event emits multiple
        # metric rows (cpu, memory, latency) with the same timestamp.
        # Build one feature vector per timestamp group.
        from collections import defaultdict
        groups: dict[str, dict] = defaultdict(lambda: {"metrics": {}, "ids": []})
        for row in rows:
            ts = row["timestamp"]
            groups[ts]["metrics"][row["metric_name"]] = float(row["value"])
            groups[ts]["ids"].append(row["id"])

        processed_ids = []
        for ts, group in groups.items():
            features = _build_feature_vector(group["metrics"])
            cur = conn.execute(
                """INSERT INTO processed_data
                   (timestamp, source_type, source_id,
                    cpu_norm, memory_norm, latency_norm,
                    error_rate, log_volume, restart_count,
                    pod_name, namespace)
                   VALUES (?,?,?,?,?,?,?,?,?,?,?)""",
                (
                    ts, "metric", group["ids"][0],
                    features["cpu_norm"], features["memory_norm"],
                    features["latency_norm"], features["error_rate"],
                    features["log_volume"], features["restart_count"],
                    None, None,
                ),
            )
            processed_ids.append(cur.lastrowid)

        # Mark all raw metrics as processed
        conn.executemany(
            "UPDATE raw_metrics SET processed=1 WHERE id=?",
            [(i,) for r in rows for i in [r["id"]]],
        )

    if processed_ids:
        results = await _notify_downstream(processed_ids)
        for svc, exc in results.items():
            if exc:
                logger.debug("Downstream %s notify failed: %s", svc, exc)

    logger.info("Processed %d raw rows -> %d feature vectors", len(rows), len(processed_ids))
    return len(processed_ids)


async def _notify_downstream(processed_ids: list[int]) -> dict[str, Exception | None]:
    """Notify trainer and detector in parallel; returns {service: error_or_None}."""
    trainer_url = config.get_str("TRAINER_URL")
    detector_url = config.get_str("DETECTOR_URL")
    payload = {"processed_ids": processed_ids}

    async def post(name: str, url: str) -> tuple[str, Exception | None]:
        try:
            async with httpx.AsyncClient(timeout=5.0) as client:
                await client.post(url, json=payload)
            return name, None
        except Exception as exc:
            return name, exc

    results = await asyncio.gather(
        post("trainer", f"{trainer_url}/train"),
        post("detector", f"{detector_url}/detect"),
    )
    return dict(results)


async def _reconciliation_loop() -> None:
    """Reprocess any missed rows every 60 seconds."""
    while True:
        await asyncio.sleep(60)
        try:
            n = await _process_batch()
            if n:
                logger.info("Reconciliation processed %d rows", n)
        except Exception as exc:
            logger.error("Reconciliation error: %s", exc)


@app.on_event("startup")
async def startup() -> None:
    database.init_db()
    asyncio.create_task(_reconciliation_loop())
    logger.info("Processor started")


@app.get("/health", response_model=HealthResponse)
async def health() -> HealthResponse:
    return HealthResponse(status="ok", service="processor")


@app.post("/process", response_model=ProcessResponse)
async def process(req: ProcessRequest) -> ProcessResponse:
    count = await _process_batch(req.batch_ids or None)
    return ProcessResponse(
        processed=count,
        features=["cpu_norm", "memory_norm", "latency_norm", "error_rate", "log_volume", "restart_count"],
    )


@app.get("/stats")
async def stats() -> dict:
    with database.get_conn() as conn:
        total = conn.execute("SELECT COUNT(*) FROM processed_data").fetchone()[0]
        unprocessed = conn.execute(
            "SELECT COUNT(*) FROM raw_metrics WHERE processed=0"
        ).fetchone()[0]
    return {"total_processed": total, "pending_raw": unprocessed}


if __name__ == "__main__":
    uvicorn.run("main:app", host="0.0.0.0", port=config.get_int("PROCESSOR_PORT"), reload=False)
