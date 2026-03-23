"""TRAINER service (port 8003) - River HalfSpaceTrees online learning."""
import asyncio
import logging
import sys

import dill
import uvicorn
from fastapi import FastAPI, HTTPException

sys.path.insert(0, "/app")
from shared import config, database
from shared.models import (
    HealthResponse, ModelInfo, ThresholdRequest, ThresholdResponse,
    TrainRequest, TrainResponse,
)

logging.basicConfig(
    level=config.get_str("LOG_LEVEL"),
    format="%(asctime)s [%(name)s] %(levelname)s %(message)s",
)
logger = logging.getLogger("trainer")

app = FastAPI(title="MLOps Trainer", version="1.0.0")

_FEATURE_KEYS = ["cpu_norm", "memory_norm", "latency_norm", "error_rate", "log_volume", "restart_count"]

# In-memory model state
_model = None
_model_version = 0
_samples_seen = 0
_samples_since_save = 0


def _make_model():
    from river.anomaly import HalfSpaceTrees
    return HalfSpaceTrees(
        n_trees=config.get_int("HST_N_TREES"),
        height=config.get_int("HST_HEIGHT"),
        window_size=config.get_int("HST_WINDOW_SIZE"),
        seed=42,
    )


def _load_latest_model() -> tuple:
    """Load latest model from DB. Returns (model, version, samples_seen)."""
    with database.get_conn() as conn:
        row = conn.execute(
            "SELECT * FROM model_state ORDER BY version DESC LIMIT 1"
        ).fetchone()
    if row is None:
        logger.info("No saved model found, initializing fresh HST model")
        return _make_model(), 0, 0
    model = dill.loads(row["model_blob"])
    logger.info("Loaded model version=%d samples_seen=%d", row["version"], row["samples_seen"])
    return model, row["version"], row["samples_seen"]


def _save_model(model, version: int, samples_seen: int) -> None:
    """Serialize model to DB."""
    blob = dill.dumps(model)
    with database.get_conn() as conn:
        conn.execute(
            """INSERT OR REPLACE INTO model_state
               (version, model_blob, n_trees, height, window_size, samples_seen)
               VALUES (?,?,?,?,?,?)""",
            (
                version, blob,
                config.get_int("HST_N_TREES"),
                config.get_int("HST_HEIGHT"),
                config.get_int("HST_WINDOW_SIZE"),
                samples_seen,
            ),
        )
    logger.info("Saved model version=%d samples=%d", version, samples_seen)


async def _train_batch(processed_ids: list[int] | None = None) -> int:
    """Learn from processed data rows."""
    global _model, _model_version, _samples_seen, _samples_since_save

    limit = config.get_int("BATCH_SIZE")
    save_every = config.get_int("MODEL_SAVE_EVERY_N")

    with database.get_conn() as conn:
        if processed_ids:
            ph = ",".join("?" * len(processed_ids))
            rows = conn.execute(
                f"SELECT * FROM processed_data WHERE id IN ({ph}) AND trained=0",
                processed_ids,
            ).fetchall()
        else:
            rows = conn.execute(
                "SELECT * FROM processed_data WHERE trained=0 ORDER BY created_at LIMIT ?",
                (limit,),
            ).fetchall()

        if not rows:
            return 0

        for row in rows:
            x = {k: float(row[k] or 0.0) for k in _FEATURE_KEYS}
            _model.learn_one(x)
            _samples_seen += 1
            _samples_since_save += 1

        # Mark as trained
        conn.executemany(
            "UPDATE processed_data SET trained=1 WHERE id=?",
            [(r["id"],) for r in rows],
        )

    # Increment version and save periodically
    if _samples_since_save >= save_every:
        _model_version += 1
        _save_model(_model, _model_version, _samples_seen)
        _samples_since_save = 0

    logger.info("Trained on %d samples (total=%d)", len(rows), _samples_seen)
    return len(rows)


async def _reconciliation_loop() -> None:
    while True:
        await asyncio.sleep(60)
        try:
            n = await _train_batch()
            if n:
                logger.info("Reconciliation trained on %d rows", n)
        except Exception as exc:
            logger.error("Reconciliation error: %s", exc)


@app.on_event("startup")
async def startup() -> None:
    global _model, _model_version, _samples_seen
    database.init_db()
    _model, _model_version, _samples_seen = _load_latest_model()
    asyncio.create_task(_reconciliation_loop())
    logger.info("Trainer started with model version=%d", _model_version)


@app.get("/health", response_model=HealthResponse)
async def health() -> HealthResponse:
    return HealthResponse(status="ok", service="trainer")


@app.post("/train", response_model=TrainResponse)
async def train(req: TrainRequest) -> TrainResponse:
    count = await _train_batch(req.processed_ids or None)
    return TrainResponse(trained_on=count, model_version=_model_version)


@app.get("/model/info", response_model=ModelInfo)
async def model_info() -> ModelInfo:
    return ModelInfo(
        version=_model_version,
        n_trees=config.get_int("HST_N_TREES"),
        height=config.get_int("HST_HEIGHT"),
        window_size=config.get_int("HST_WINDOW_SIZE"),
        samples_seen=_samples_seen,
    )


@app.post("/model/reset")
async def model_reset() -> dict:
    global _model, _model_version, _samples_seen, _samples_since_save
    _model = _make_model()
    _model_version += 1
    _samples_seen = 0
    _samples_since_save = 0
    _save_model(_model, _model_version, 0)
    return {"reset": True, "new_version": _model_version}


if __name__ == "__main__":
    uvicorn.run("main:app", host="0.0.0.0", port=config.get_int("TRAINER_PORT"), reload=False)
