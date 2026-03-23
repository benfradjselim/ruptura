"""Tests for the TRAINER service - River HalfSpaceTrees model."""
import sys
import os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "../../src"))

import pytest


def test_hst_model_learn_one():
    """HalfSpaceTrees learn_one should not raise."""
    pytest.importorskip("river")
    from river.anomaly import HalfSpaceTrees

    model = HalfSpaceTrees(n_trees=5, height=4, window_size=50, seed=42)
    x = {"cpu_norm": 0.5, "memory_norm": 0.3, "latency_norm": 0.1,
         "error_rate": 0.0, "log_volume": 0.2, "restart_count": 0.0}
    model.learn_one(x)  # Should not raise


def test_hst_model_score_one_returns_float():
    """score_one should return a float in [0, 1] after warm-up."""
    pytest.importorskip("river")
    from river.anomaly import HalfSpaceTrees

    model = HalfSpaceTrees(n_trees=5, height=4, window_size=10, seed=42)
    x_normal = {"cpu_norm": 0.3, "memory_norm": 0.2, "latency_norm": 0.1,
                "error_rate": 0.0, "log_volume": 0.1, "restart_count": 0.0}

    # Warm up
    for _ in range(20):
        model.learn_one(x_normal)

    score = model.score_one(x_normal)
    assert isinstance(score, float)
    assert 0.0 <= score <= 1.0


def test_hst_anomaly_scores_higher():
    """Anomalous data should score higher than normal after training."""
    pytest.importorskip("river")
    from river.anomaly import HalfSpaceTrees

    model = HalfSpaceTrees(n_trees=10, height=8, window_size=50, seed=42)
    normal = {"cpu_norm": 0.2, "memory_norm": 0.15, "latency_norm": 0.1,
              "error_rate": 0.01, "log_volume": 0.1, "restart_count": 0.0}
    anomaly = {"cpu_norm": 0.99, "memory_norm": 0.98, "latency_norm": 0.95,
               "error_rate": 0.9, "log_volume": 0.95, "restart_count": 1.0}

    for _ in range(100):
        model.learn_one(normal)

    score_normal = model.score_one(normal)
    score_anomaly = model.score_one(anomaly)

    assert score_anomaly > score_normal, (
        f"Anomaly score {score_anomaly:.4f} should be > normal score {score_normal:.4f}"
    )


def test_model_serialization():
    """Model should survive dill serialize/deserialize cycle."""
    pytest.importorskip("river")
    pytest.importorskip("dill")
    import dill
    from river.anomaly import HalfSpaceTrees

    model = HalfSpaceTrees(n_trees=5, height=4, window_size=20, seed=42)
    x = {"cpu_norm": 0.4, "memory_norm": 0.3, "latency_norm": 0.2,
         "error_rate": 0.0, "log_volume": 0.1, "restart_count": 0.0}
    for _ in range(10):
        model.learn_one(x)

    blob = dill.dumps(model)
    restored = dill.loads(blob)

    original_score = model.score_one(x)
    restored_score = restored.score_one(x)
    assert abs(original_score - restored_score) < 1e-10
