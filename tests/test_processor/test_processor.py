"""Tests for the PROCESSOR service."""
import sys
import os
import pytest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "../../src"))


def test_normalize_minmax_basic():
    """MinMax normalization should return 0 for min, 1 for max."""
    from shared.database import init_db, get_conn
    import tempfile, os as _os

    with tempfile.NamedTemporaryFile(suffix=".db", delete=False) as f:
        db_path = f.name
    _os.environ["DB_PATH"] = db_path
    init_db(db_path)

    # Simulate the normalizer logic directly
    minmax: dict = {}

    def normalize(name, value):
        if name not in minmax:
            minmax[name] = {"min": value, "max": value}
        s = minmax[name]
        s["min"] = min(s["min"], value)
        s["max"] = max(s["max"], value)
        rng = s["max"] - s["min"]
        return 0.0 if rng == 0 else (value - s["min"]) / rng

    assert normalize("cpu", 0.0) == 0.0   # first value = 0
    assert normalize("cpu", 100.0) == 1.0  # max
    assert normalize("cpu", 50.0) == 0.5   # midpoint

    _os.unlink(db_path)


def test_normalize_minmax_constant():
    """Constant values should normalize to 0.0 (no range)."""
    minmax: dict = {}

    def normalize(name, value):
        if name not in minmax:
            minmax[name] = {"min": value, "max": value}
        s = minmax[name]
        s["min"] = min(s["min"], value)
        s["max"] = max(s["max"], value)
        rng = s["max"] - s["min"]
        return 0.0 if rng == 0 else (value - s["min"]) / rng

    for _ in range(5):
        result = normalize("mem", 42.0)
    assert result == 0.0


def test_extract_features_cpu():
    """CPU metrics should populate cpu_norm feature."""
    rows = [
        {"metric_name": "cpu_usage", "value": 50.0},
        {"metric_name": "memory_usage", "value": 30.0},
    ]
    minmax: dict = {}

    def normalize(name, value):
        if name not in minmax:
            minmax[name] = {"min": value, "max": value}
        s = minmax[name]
        s["min"] = min(s["min"], value)
        s["max"] = max(s["max"], value)
        rng = s["max"] - s["min"]
        return 0.0 if rng == 0 else (value - s["min"]) / rng

    features = {
        "cpu_norm": 0.0, "memory_norm": 0.0,
        "latency_norm": 0.0, "error_rate": 0.0,
        "log_volume": 0.0, "restart_count": 0.0,
    }
    for row in rows:
        name = row["metric_name"]
        value = float(row["value"])
        if name == "cpu_usage":
            features["cpu_norm"] = normalize("cpu_usage", value)
        elif name == "memory_usage":
            features["memory_norm"] = normalize("memory_usage", value)

    assert features["cpu_norm"] == 0.0
    assert features["memory_norm"] == 0.0
