"""Shared configuration loader for all MLOps microservices."""
import os
from typing import Any, List, Optional

DEFAULTS = {
    "LOG_LEVEL": "INFO",
    "DB_PATH": "/data/mlops.db",
    "COLLECTOR_PORT": 8001,
    "COLLECTOR_URL": "http://collector:8001",
    "COLLECT_INTERVAL_SEC": 15,
    "PROMETHEUS_URL": "http://prometheus:9090",
    "PROCESSOR_PORT": 8002,
    "PROCESSOR_URL": "http://processor:8002",
    "NORMALIZATION_METHOD": "minmax",
    "FEATURE_WINDOW_SEC": 300,
    "BATCH_SIZE": 256,
    "TRAINER_PORT": 8003,
    "TRAINER_URL": "http://trainer:8003",
    "HST_N_TREES": 25,
    "HST_HEIGHT": 15,
    "HST_WINDOW_SIZE": 250,
    "MODEL_SAVE_EVERY_N": 100,
    "DETECTOR_PORT": 8004,
    "DETECTOR_URL": "http://detector:8004",
    "ANOMALY_THRESHOLD": 0.7,
    "SCORE_BATCH_SIZE": 100,
    "EXPORTER_PORT": 8005,
    "EXPORTER_URL": "http://exporter:8005",
    "METRICS_PREFIX": "mlops_anomaly",
    "DASHBOARD_PORT": 8501,
    "DASHBOARD_REFRESH_SEC": 5,
    "METRIC_PREDICTOR_PORT": 8008,
    "METRIC_PREDICTOR_URL": "http://metric-predictor:8008",
    "METRIC_PREDICTION_INTERVAL_SEC": 60,
    "METRIC_FORECAST_DAYS": 7,
    "METRIC_MIN_HISTORY": 10,
}


def get(key: str, default: Any = None) -> Any:
    raw = os.environ.get(key)
    if raw is not None:
        return raw
    if default is not None:
        return default
    return DEFAULTS.get(key)


def get_str(key: str, default: Optional[str] = None) -> str:
    value = get(key, default)
    if value is None:
        raise KeyError(f"Configuration key '{key}' not found")
    return str(value)


def get_int(key: str, default: Optional[int] = None) -> int:
    value = get(key, default)
    if value is None:
        raise KeyError(f"Configuration key '{key}' not found")
    return int(value)


def get_float(key: str, default: Optional[float] = None) -> float:
    value = get(key, default)
    if value is None:
        raise KeyError(f"Configuration key '{key}' not found")
    return float(value)


def get_bool(key: str, default: Optional[bool] = None) -> bool:
    value = get(key, default)
    if value is None:
        raise KeyError(f"Configuration key '{key}' not found")
    if isinstance(value, bool):
        return value
    return str(value).lower() in ("1", "true", "yes", "on")


def get_list(key: str, separator: str = ",", default: Optional[List[str]] = None) -> List[str]:
    value = get(key)
    if value is None:
        if default is not None:
            return default
        raise KeyError(f"Configuration key '{key}' not found")
    return [v.strip() for v in str(value).split(separator) if v.strip()]
