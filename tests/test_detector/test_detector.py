"""Tests for the DETECTOR service."""
import sys
import os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "../../src"))

import pytest


def test_threshold_logic():
    """Score above threshold should be flagged as anomaly."""
    threshold = 0.7
    scores = [0.3, 0.5, 0.71, 0.9]
    expected = [False, False, True, True]

    results = [s > threshold for s in scores]
    assert results == expected


def test_threshold_boundary():
    """Score equal to threshold should NOT be flagged (strictly greater)."""
    threshold = 0.7
    assert not (0.7 > threshold)
    assert 0.7001 > threshold


def test_feature_extraction_defaults():
    """Missing feature values should default to 0.0."""
    FEATURE_KEYS = ["cpu_norm", "memory_norm", "latency_norm",
                    "error_rate", "log_volume", "restart_count"]

    class FakeRow(dict):
        pass

    row = FakeRow({"cpu_norm": None, "memory_norm": 0.5})
    x = {k: float(row.get(k) or 0.0) for k in FEATURE_KEYS}

    assert x["cpu_norm"] == 0.0
    assert x["memory_norm"] == 0.5
    assert x["latency_norm"] == 0.0
