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


def _extract_features(metrics: list) -> dict:
    """Map raw metric rows to normalized feature vector."""
    features: dict[str, float] = {
        "cpu_norm": 0.0,
        "memory_norm": 0.0,
        "latency_norm": 0.0,
        "error_rate": 0.0,
        "log_volume": 0.0,
        "restart_count": 0.0,
    }
    for row in metrics:
        name = row["metric_name"]
        value = float(row["value"])
        if name == "cpu_usage":
            features["cpu_norm"] = _normalize_minmax("cpu_usage", value)
        elif name == "memory_usage":
            features["memory_norm"] = _normalize_minmax("memory_usage", value)
        elif name == "latency_ms":
            features["latency_norm"] = _normalize_minmax("latency_ms", value)
    return features


async def _process_batch(batch_ids: list[int] | None = None) -> int:
    """Process unprocessed raw metrics rows into processed_data."""
    database.init_db()
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

        features = _extract_features(rows)
        timestamp = datetime.now(timezone.utc).isoformat()
        processed_ids = []

        for row in rows:
            cur = conn.execute(
                """INSERT INTO processed_data
                   (timestamp, source_type, source_id,
                    cpu_norm, memory_norm, latency_norm,
                    error_rate, log_volume, restart_count,
                    pod_name, namespace)
                   VALUES (?,?,?,?,?,?,?,?,?,?,?)""",
                (
                    timestamp, "metric", row["id"],
                    features["cpu_norm"], features["memory_norm"],
                    features["latency_norm"], features["error_rate"],
                    features["log_volume"], features["restart_count"],
                    None, None,
                ),
            )
            processed_ids.append(cur.lastrowid)

        # Mark raw metrics as processed
        ids_to_mark = [r["id"] for r in rows]
        conn.executemany(
            "UPDATE raw_metrics SET processed=1 WHERE id=?",
            [(i,) for i in ids_to_mark],
        )

    if processed_ids:
        await _notify_downstream(processed_ids)

    logger.info("Processed %d rows -> %d features", len(rows), len(processed_ids))
    return len(processed_ids)


async def _notify_downstream(processed_ids: list[int]) -> None:
    """Fire-and-forget: notify trainer and detector in parallel."""
    trainer_url = config.get_str("TRAINER_URL")
    detector_url = config.get_str("DETECTOR_URL")
    payload = {"processed_ids": processed_ids}

    async def post(url: str, data: dict) -> None:
        try:
            async with httpx.AsyncClient(timeout=5.0) as client:
                await client.post(url, json=data)
        except Exception as exc:
            logger.debug("Downstream notify failed %s: %s", url, exc)

    await asyncio.gather(
        post(f"{trainer_url}/train", payload),
        post(f"{detector_url}/detect", payload),
        return_exceptions=True,
    )


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
