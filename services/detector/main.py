"""DETECTOR service (port 8004) - Real-time anomaly scoring."""
import asyncio
import hashlib
import hmac
import logging
import os
import sys
from datetime import datetime, timezone

import dill
import uvicorn
from fastapi import FastAPI

sys.path.insert(0, "/app")
from shared import config, database
from shared.models import (
    DetectRequest, DetectResponse, DetectionResult,
    HealthResponse, ThresholdRequest, ThresholdResponse,
)

logging.basicConfig(
    level=config.get_str("LOG_LEVEL"),
    format="%(asctime)s [%(name)s] %(levelname)s %(message)s",
)
logger = logging.getLogger("detector")

app = FastAPI(title="MLOps Detector", version="2.0.0")

_FEATURE_KEYS = [
    "cpu_norm", "memory_norm", "latency_norm",
    "error_rate", "log_volume", "restart_count",
]

_HMAC_KEY = os.environ.get("MODEL_HMAC_KEY", "mlops-default-dev-key").encode()

_model = None
_model_version = 0
_samples_seen = 0
_threshold = config.get_float("ANOMALY_THRESHOLD")
_model_lock = asyncio.Lock()


def _verify_and_load(signed: bytes):
    if len(signed) < 32:
        raise ValueError("Model blob too short to contain HMAC signature")
    mac, blob = signed[:32], signed[32:]
    expected = hmac.new(_HMAC_KEY, blob, hashlib.sha256).digest()
    if not hmac.compare_digest(mac, expected):
        raise ValueError("Model blob HMAC verification failed — possible tampering")
    return dill.loads(blob)


def _load_model():
    with database.get_conn() as conn:
        row = conn.execute(
            "SELECT version, model_blob, samples_seen FROM model_state ORDER BY version DESC LIMIT 1"
        ).fetchone()
    if row is None:
        return None, 0, 0
    model = _verify_and_load(row["model_blob"])
    logger.info("Loaded model version=%d samples_seen=%d", row["version"], row["samples_seen"])
    return model, row["version"], row["samples_seen"]


async def _score_batch(processed_ids: list[int] | None = None) -> list[DetectionResult]:
    global _model, _model_version, _samples_seen

    warmup_required = config.get_int("HST_WINDOW_SIZE")
    limit = config.get_int("SCORE_BATCH_SIZE")

    async with _model_lock:
        with database.get_conn() as conn:
            latest_row = conn.execute(
                "SELECT version FROM model_state ORDER BY version DESC LIMIT 1"
            ).fetchone()
        latest_version = latest_row["version"] if latest_row else 0

        if _model is None or latest_version > _model_version:
            _model, _model_version, _samples_seen = _load_model()
            if _model_version:
                logger.info("Refreshed model version=%d samples_seen=%d warmup_required=%d",
                            _model_version, _samples_seen, warmup_required)

        if _model is None:
            logger.warning("No model available yet — skipping detection")
            return []

        in_warmup = _samples_seen < warmup_required
        if in_warmup:
            logger.info("Warm-up phase (%d/%d samples) — scores suppressed",
                        _samples_seen, warmup_required)

        with database.get_conn() as conn:
            if processed_ids:
                ph = ",".join("?" * len(processed_ids))
                rows = conn.execute(
                    f"SELECT * FROM processed_data WHERE id IN ({ph}) AND scored=0",
                    processed_ids,
                ).fetchall()
            else:
                rows = conn.execute(
                    "SELECT * FROM processed_data WHERE scored=0 ORDER BY created_at LIMIT ?",
                    (limit,),
                ).fetchall()

            if not rows:
                return []

            results = []
            anomaly_rows = []
            timestamp = datetime.now(timezone.utc).isoformat()

            for row in rows:
                x = {k: float(row[k] or 0.0) for k in _FEATURE_KEYS}
                raw_score = _model.score_one(x)

                if in_warmup:
                    score = 0.0
                    is_anomaly = False
                else:
                    score = raw_score
                    is_anomaly = score > _threshold

                results.append(DetectionResult(
                    id=row["id"],
                    timestamp=row["timestamp"],
                    anomaly_score=score,
                    is_anomaly=is_anomaly,
                    pod_name=row["pod_name"],
                    namespace=row["namespace"],
                ))

                anomaly_rows.append((
                    row["id"], timestamp, score,
                    1 if is_anomaly else 0,
                    _threshold, _model_version,
                    row["pod_name"], row["namespace"],
                ))

            conn.executemany(
                """INSERT INTO anomalies
                   (processed_id, timestamp, anomaly_score, is_anomaly,
                    threshold_used, model_version, pod_name, namespace)
                   VALUES (?,?,?,?,?,?,?,?)""",
                anomaly_rows,
            )
            conn.executemany(
                "UPDATE processed_data SET scored=1 WHERE id=?",
                [(r["id"],) for r in rows],
            )

        anomaly_count = sum(1 for r in results if r.is_anomaly)
        logger.info("Scored %d rows — %d anomalies (threshold=%.2f, warmup=%s)",
                    len(results), anomaly_count, _threshold, in_warmup)
        return results


async def _reconciliation_loop() -> None:
    while True:
        await asyncio.sleep(60)
        try:
            results = await _score_batch()
            if results:
                logger.info("Reconciliation scored %d rows", len(results))
        except Exception as exc:
            logger.error("Reconciliation error: %s", exc)


@app.on_event("startup")
async def startup() -> None:
    global _model, _model_version, _samples_seen, _threshold
    database.init_db()
    _model, _model_version, _samples_seen = _load_model()
    with database.get_conn() as conn:
        row = conn.execute(
            "SELECT value FROM config WHERE key='anomaly_threshold'"
        ).fetchone()
        if row:
            _threshold = float(row["value"])
    asyncio.create_task(_reconciliation_loop())
    logger.info("Detector started — threshold=%.2f model_version=%d samples_seen=%d",
                _threshold, _model_version, _samples_seen)


@app.get("/health", response_model=HealthResponse)
async def health() -> HealthResponse:
    return HealthResponse(status="ok", service="detector")


@app.post("/detect", response_model=DetectResponse)
async def detect(req: DetectRequest) -> DetectResponse:
    results = await _score_batch(req.processed_ids or None)
    return DetectResponse(results=results)


@app.get("/anomalies")
async def get_anomalies(limit: int = 50) -> dict:
    with database.get_conn() as conn:
        rows = conn.execute(
            "SELECT * FROM anomalies ORDER BY timestamp DESC LIMIT ?",
            (limit,),
        ).fetchall()
    return {"anomalies": [dict(r) for r in rows]}


@app.get("/warmup-status")
async def warmup_status() -> dict:
    warmup_required = config.get_int("HST_WINDOW_SIZE")
    return {
        "samples_seen": _samples_seen,
        "warmup_required": warmup_required,
        "in_warmup": _samples_seen < warmup_required,
        "warmup_progress_pct": min(100, round(_samples_seen / warmup_required * 100, 1)),
        "model_version": _model_version,
    }


@app.get("/threshold", response_model=ThresholdResponse)
async def get_threshold() -> ThresholdResponse:
    return ThresholdResponse(threshold=_threshold)


@app.put("/threshold", response_model=ThresholdResponse)
async def set_threshold(req: ThresholdRequest) -> ThresholdResponse:
    global _threshold
    _threshold = req.threshold
    with database.get_conn() as conn:
        conn.execute(
            "INSERT OR REPLACE INTO config (key, value, updated_at) "
            "VALUES ('anomaly_threshold', ?, datetime('now'))",
            (str(req.threshold),),
        )
    logger.info("Threshold updated to %.2f", _threshold)
    return ThresholdResponse(threshold=_threshold)


if __name__ == "__main__":
    uvicorn.run("main:app", host="0.0.0.0",
                port=config.get_int("DETECTOR_PORT"), reload=False)
