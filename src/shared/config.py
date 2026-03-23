"""Shared configuration loader for all MLOps microservices."""
import os
from typing import Any


DEFAULTS: dict[str, Any] = {
    # COLLECTOR
    "COLLECT_INTERVAL_SEC": 15,
    "PROMETHEUS_URL": "http://prometheus:9090",
    "PROMETHEUS_QUERIES": "cpu_usage_seconds_total,memory_working_set_bytes,http_request_duration_seconds",
    "K8S_NAMESPACES": "default,monitoring",
    "K8S_LOG_TAIL_LINES": 100,
    "COLLECTOR_PORT": 8001,
    "COLLECTOR_URL": "http://collector:8001",

    # PROCESSOR
    "PROCESSOR_PORT": 8002,
    "PROCESSOR_URL": "http://processor:8002",
    "NORMALIZATION_METHOD": "minmax",
    "FEATURE_WINDOW_SEC": 60,
    "BATCH_SIZE": 50,

    # TRAINER
    "TRAINER_PORT": 8003,
    "TRAINER_URL": "http://trainer:8003",
    "HST_N_TREES": 10,
    "HST_HEIGHT": 8,
    "HST_WINDOW_SIZE": 250,
    "MODEL_SAVE_EVERY_N": 100,

    # DETECTOR
    "DETECTOR_PORT": 8004,
    "DETECTOR_URL": "http://detector:8004",
    "ANOMALY_THRESHOLD": 0.7,
    "SCORE_BATCH_SIZE": 50,

    # EXPORTER
    "EXPORTER_PORT": 8005,
    "EXPORTER_URL": "http://exporter:8005",
    "METRICS_PREFIX": "mlops_anomaly",
    "EXPORT_WINDOW_HOURS": 24,

    # DASHBOARD
    "DASHBOARD_PORT": 8501,
    "DASHBOARD_REFRESH_SEC": 5,

    # SHARED
    "DB_PATH": "/data/mlops.db",
    "LOG_LEVEL": "INFO",
}


def get(key: str) -> Any:
    """Return config value: env var overrides default."""
    raw = os.environ.get(key)
    if raw is None:
        return DEFAULTS.get(key)
    default = DEFAULTS.get(key)
    if isinstance(default, int):
        return int(raw)
    if isinstance(default, float):
        return float(raw)
    return raw


def get_str(key: str) -> str:
    return str(get(key))


def get_int(key: str) -> int:
    return int(get(key))


def get_float(key: str) -> float:
    return float(get(key))
