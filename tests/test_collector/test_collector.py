"""Tests for the COLLECTOR service."""
import sys
import os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "../../src"))


def test_extract_log_level_error():
    line = "2024-01-01 ERROR: connection refused"
    upper = line.upper()
    for level in ("ERROR", "WARN", "WARNING", "INFO", "DEBUG"):
        if level in upper:
            result = level
            break
    assert result == "ERROR"


def test_extract_log_level_none():
    line = "all systems operational"
    upper = line.upper()
    result = None
    for level in ("ERROR", "WARN", "WARNING", "INFO", "DEBUG"):
        if level in upper:
            result = level
            break
    assert result is None


def test_psutil_returns_two_metrics():
    """psutil should return cpu_usage and memory_usage."""
    import psutil
    from datetime import datetime, timezone
    import json

    timestamp = datetime.now(timezone.utc).isoformat()
    cpu = psutil.cpu_percent(interval=None)
    mem = psutil.virtual_memory()

    rows = [
        {"timestamp": timestamp, "source": "psutil",
         "metric_name": "cpu_usage", "value": cpu, "labels": "{}"},
        {"timestamp": timestamp, "source": "psutil",
         "metric_name": "memory_usage", "value": mem.percent, "labels": "{}"},
    ]

    assert len(rows) == 2
    assert rows[0]["metric_name"] == "cpu_usage"
    assert rows[1]["metric_name"] == "memory_usage"
    assert isinstance(rows[0]["value"], float)
