"""Tests for RupturaClient using the responses library to mock HTTP calls."""

import pytest
import responses as resp_lib

from ruptura import RupturaClient, RupturaError

BASE = "https://ruptura.test"


def client(**kwargs):
    return RupturaClient(BASE, api_key="rpt_test_key", **kwargs)


# ---------------------------------------------------------------------------
# Health & Readiness
# ---------------------------------------------------------------------------


@resp_lib.activate
def test_health():
    resp_lib.add(
        resp_lib.GET,
        f"{BASE}/api/v2/health",
        json={"status": "ok", "rupture_detection": "active", "message": ""},
    )
    h = client().health()
    assert h["status"] == "ok"
    assert h["rupture_detection"] == "active"


@resp_lib.activate
def test_ready_succeeds():
    resp_lib.add(resp_lib.GET, f"{BASE}/api/v2/ready", status=200, body=b"")
    client().ready()  # should not raise


@resp_lib.activate
def test_ready_unavailable_raises():
    resp_lib.add(resp_lib.GET, f"{BASE}/api/v2/ready", status=503, body=b"")
    with pytest.raises(RupturaError) as exc_info:
        client().ready()
    assert exc_info.value.status_code == 503


# ---------------------------------------------------------------------------
# Prometheus metrics
# ---------------------------------------------------------------------------


@resp_lib.activate
def test_metrics_prometheus_returns_text():
    prom_text = "# HELP rpt_uptime_seconds\nrpt_uptime_seconds 42.0\n"
    resp_lib.add(resp_lib.GET, f"{BASE}/api/v2/metrics", body=prom_text, content_type="text/plain")
    result = client().metrics_prometheus()
    assert "rpt_uptime_seconds" in result


# ---------------------------------------------------------------------------
# Ruptures
# ---------------------------------------------------------------------------


@resp_lib.activate
def test_ruptures_returns_list():
    events = [{"host": "web-01", "metric": "cpu", "rupture_index": 0.92, "timestamp": "2026-01-01T00:00:00Z"}]
    resp_lib.add(resp_lib.GET, f"{BASE}/api/v2/ruptures", json=events)
    result = client().ruptures()
    assert len(result) == 1
    assert result[0]["host"] == "web-01"


@resp_lib.activate
def test_ruptures_empty():
    resp_lib.add(resp_lib.GET, f"{BASE}/api/v2/ruptures", json=[])
    assert client().ruptures() == []


@resp_lib.activate
def test_rupture_for_host():
    event = {"host": "db-01", "metric": "mem", "rupture_index": 0.75, "timestamp": "2026-01-01T00:00:00Z"}
    resp_lib.add(resp_lib.GET, f"{BASE}/api/v2/rupture/db-01", json=event)
    result = client().rupture("db-01")
    assert result["rupture_index"] == 0.75


# ---------------------------------------------------------------------------
# KPI
# ---------------------------------------------------------------------------


@resp_lib.activate
def test_kpi_returns_value():
    kpi_data = [{"name": "stress", "host": "web-01", "value": 0.6, "timestamp": "2026-01-01T00:00:00Z"}]
    resp_lib.add(resp_lib.GET, f"{BASE}/api/v2/kpi/stress/web-01", json=kpi_data)
    result = client().kpi("stress", "web-01")
    assert result[0]["value"] == 0.6


# ---------------------------------------------------------------------------
# Actions
# ---------------------------------------------------------------------------


@resp_lib.activate
def test_actions_returns_list():
    resp_lib.add(resp_lib.GET, f"{BASE}/api/v2/actions", json=[])
    assert client().actions() == []


@resp_lib.activate
def test_emergency_stop():
    resp_lib.add(resp_lib.POST, f"{BASE}/api/v2/actions/emergency-stop", json={"emergency_stop": True})
    result = client().emergency_stop()
    assert result["emergency_stop"] is True


# ---------------------------------------------------------------------------
# Context
# ---------------------------------------------------------------------------


@resp_lib.activate
def test_add_context():
    entry = {"id": "c1", "type": "maintenance_window", "service": "api", "note": "deploy"}
    resp_lib.add(resp_lib.POST, f"{BASE}/api/v2/context", json=entry, status=201)
    result = client().add_context({"type": "maintenance_window", "service": "api"})
    assert result["id"] == "c1"


@resp_lib.activate
def test_list_contexts():
    entries = [{"id": "c1", "type": "load_test"}]
    resp_lib.add(resp_lib.GET, f"{BASE}/api/v2/context", json=entries)
    result = client().list_contexts()
    assert len(result) == 1
    assert result[0]["type"] == "load_test"


@resp_lib.activate
def test_delete_context():
    resp_lib.add(resp_lib.DELETE, f"{BASE}/api/v2/context/c1", status=204, body=b"")
    client().delete_context("c1")  # should not raise


# ---------------------------------------------------------------------------
# Explainability
# ---------------------------------------------------------------------------


@resp_lib.activate
def test_explain_not_found_raises():
    resp_lib.add(resp_lib.GET, f"{BASE}/api/v2/explain/r-999", status=404, json={"error": "not found"})
    with pytest.raises(RupturaError) as exc_info:
        client().explain("r-999")
    assert exc_info.value.status_code == 404


@resp_lib.activate
def test_explain_formula_not_found_raises():
    resp_lib.add(
        resp_lib.GET,
        f"{BASE}/api/v2/explain/r-999/formula",
        status=404,
        json={"error": "not found"},
    )
    with pytest.raises(RupturaError) as exc_info:
        client().explain_formula("r-999")
    assert exc_info.value.status_code == 404


# ---------------------------------------------------------------------------
# Auth header
# ---------------------------------------------------------------------------


@resp_lib.activate
def test_api_key_sets_bearer_header():
    resp_lib.add(resp_lib.GET, f"{BASE}/api/v2/health", json={"status": "ok"})
    c = RupturaClient(BASE, api_key="rpt_mykey")
    c.health()
    assert resp_lib.calls[0].request.headers["Authorization"] == "Bearer rpt_mykey"


@resp_lib.activate
def test_no_auth_header_when_no_key():
    resp_lib.add(resp_lib.GET, f"{BASE}/api/v2/health", json={"status": "ok"})
    c = RupturaClient(BASE)
    c.health()
    assert "Authorization" not in resp_lib.calls[0].request.headers


# ---------------------------------------------------------------------------
# Error handling
# ---------------------------------------------------------------------------


@resp_lib.activate
def test_server_error_raises_rpt_error():
    resp_lib.add(
        resp_lib.GET,
        f"{BASE}/api/v2/health",
        status=503,
        json={"error": "storage down"},
    )
    with pytest.raises(RupturaError) as exc_info:
        client().health()
    assert exc_info.value.status_code == 503
    assert "storage down" in exc_info.value.message


@resp_lib.activate
def test_rpt_error_str():
    resp_lib.add(resp_lib.GET, f"{BASE}/api/v2/health", status=503, json={"error": "down"})
    with pytest.raises(RupturaError) as exc_info:
        client().health()
    assert "503" in str(exc_info.value)
