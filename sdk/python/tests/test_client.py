"""Tests for OHEClient using the responses library to mock HTTP calls."""

import pytest
import responses as resp_lib

from ohe import OHEClient, OHEError

BASE = "https://ohe.test"


def client(**kwargs):
    return OHEClient(BASE, token="test-token", **kwargs)


def api_wrap(data):
    return {"success": True, "data": data, "timestamp": "2026-01-01T00:00:00Z"}


# ---------------------------------------------------------------------------
# Health
# ---------------------------------------------------------------------------


@resp_lib.activate
def test_health():
    resp_lib.add(resp_lib.GET, f"{BASE}/api/v1/health", json=api_wrap({"status": "ok", "version": "5.0.0"}))
    h = client().health()
    assert h["status"] == "ok"
    assert h["version"] == "5.0.0"


@resp_lib.activate
def test_liveness():
    resp_lib.add(resp_lib.GET, f"{BASE}/api/v1/health/live", json=api_wrap({"status": "alive"}))
    client().liveness()  # should not raise


@resp_lib.activate
def test_readiness():
    resp_lib.add(resp_lib.GET, f"{BASE}/api/v1/health/ready", json=api_wrap({"status": "ready"}))
    client().readiness()


# ---------------------------------------------------------------------------
# Auth
# ---------------------------------------------------------------------------


@resp_lib.activate
def test_login_stores_token():
    resp_lib.add(
        resp_lib.POST,
        f"{BASE}/api/v1/auth/login",
        json=api_wrap({"token": "jwt-new", "expires": 3600, "user": {"username": "admin"}}),
    )
    c = OHEClient(BASE)
    result = c.login("admin", "secret")
    assert result["token"] == "jwt-new"
    assert c._token == "jwt-new"


@resp_lib.activate
def test_login_bad_credentials_raises():
    resp_lib.add(
        resp_lib.POST,
        f"{BASE}/api/v1/auth/login",
        status=401,
        json={"error": {"code": "UNAUTHORIZED", "message": "bad creds"}},
    )
    c = OHEClient(BASE)
    with pytest.raises(OHEError) as exc_info:
        c.login("x", "y")
    assert exc_info.value.status_code == 401
    assert exc_info.value.code == "UNAUTHORIZED"


@resp_lib.activate
def test_logout_clears_token():
    resp_lib.add(resp_lib.POST, f"{BASE}/api/v1/auth/logout", json=api_wrap(None))
    c = client()
    c.logout()
    assert c._token == ""


# ---------------------------------------------------------------------------
# Metrics
# ---------------------------------------------------------------------------


@resp_lib.activate
def test_metrics_list():
    resp_lib.add(resp_lib.GET, f"{BASE}/api/v1/metrics", json=api_wrap(["cpu_percent", "mem_percent"]))
    assert client().metrics_list() == ["cpu_percent", "mem_percent"]


@resp_lib.activate
def test_metric_get():
    resp_lib.add(
        resp_lib.GET,
        f"{BASE}/api/v1/metrics/cpu_percent",
        json=api_wrap({"name": "cpu_percent", "value": 42.5, "host": "web-01"}),
    )
    m = client().metric_get("cpu_percent", "web-01")
    assert m["value"] == 42.5


@resp_lib.activate
def test_metric_range_returns_points():
    from datetime import datetime, timezone

    pts = [{"timestamp": "2026-01-01T00:00:00Z", "value": 55.0}]
    resp_lib.add(resp_lib.GET, f"{BASE}/api/v1/metrics/cpu_percent/range", json=api_wrap(pts))
    now = datetime.now(timezone.utc)
    result = client().metric_range("cpu_percent", "web-01", now, now)
    assert len(result) == 1
    assert result[0]["value"] == 55.0


# ---------------------------------------------------------------------------
# Alerts
# ---------------------------------------------------------------------------


@resp_lib.activate
def test_alert_list():
    alerts = [{"id": "a1", "name": "high-cpu", "severity": "critical", "status": "active"}]
    resp_lib.add(resp_lib.GET, f"{BASE}/api/v1/alerts", json=api_wrap(alerts))
    result = client().alert_list()
    assert len(result) == 1
    assert result[0]["id"] == "a1"


@resp_lib.activate
def test_alert_acknowledge():
    resp_lib.add(resp_lib.POST, f"{BASE}/api/v1/alerts/a1/acknowledge", json=api_wrap(None))
    client().alert_acknowledge("a1")  # should not raise


@resp_lib.activate
def test_alert_silence():
    resp_lib.add(resp_lib.POST, f"{BASE}/api/v1/alerts/a1/silence", json=api_wrap(None))
    client().alert_silence("a1")


# ---------------------------------------------------------------------------
# SLOs
# ---------------------------------------------------------------------------


@resp_lib.activate
def test_slo_create_and_get():
    slo = {"id": "slo-1", "name": "uptime", "target": 99.9, "window": "30d"}
    resp_lib.add(resp_lib.POST, f"{BASE}/api/v1/slos", json=api_wrap(slo))
    resp_lib.add(resp_lib.GET, f"{BASE}/api/v1/slos/slo-1", json=api_wrap(slo))

    c = client()
    created = c.slo_create({"name": "uptime", "target": 99.9, "window": "30d"})
    assert created["id"] == "slo-1"

    got = c.slo_get("slo-1")
    assert got["target"] == 99.9


@resp_lib.activate
def test_slo_status():
    status = {"slo": {"id": "slo-1"}, "state": "healthy", "compliance_pct": 99.95}
    resp_lib.add(resp_lib.GET, f"{BASE}/api/v1/slos/slo-1/status", json=api_wrap(status))
    result = client().slo_status("slo-1")
    assert result["state"] == "healthy"


# ---------------------------------------------------------------------------
# Ingest
# ---------------------------------------------------------------------------


@resp_lib.activate
def test_ingest():
    resp_lib.add(resp_lib.POST, f"{BASE}/api/v1/ingest", json=api_wrap({"written": 1}))
    client().ingest("web-01", [{"name": "cpu", "value": 55.0}], agent_id="ag-1")


# ---------------------------------------------------------------------------
# API Keys
# ---------------------------------------------------------------------------


@resp_lib.activate
def test_apikey_create_returns_plaintext():
    payload = {"id": "k1", "name": "ci", "role": "operator", "key": "ohe_secret123", "active": True}
    resp_lib.add(resp_lib.POST, f"{BASE}/api/v1/api-keys", json=api_wrap(payload))
    key = client().apikey_create("ci", "operator")
    assert key["key"] == "ohe_secret123"


@resp_lib.activate
def test_apikey_delete():
    resp_lib.add(resp_lib.DELETE, f"{BASE}/api/v1/api-keys/k1", json=api_wrap(None))
    client().apikey_delete("k1")  # should not raise


# ---------------------------------------------------------------------------
# Error handling
# ---------------------------------------------------------------------------


@resp_lib.activate
def test_server_error_raises_ohe_error():
    resp_lib.add(
        resp_lib.GET,
        f"{BASE}/api/v1/health",
        status=503,
        json={"error": {"code": "UNAVAILABLE", "message": "storage down"}},
    )
    with pytest.raises(OHEError) as exc_info:
        client().health()
    assert exc_info.value.status_code == 503
    assert exc_info.value.code == "UNAVAILABLE"


@resp_lib.activate
def test_api_key_auth_header():
    """WithAPIKey sets Authorization: Bearer ohe_xxx."""
    resp_lib.add(resp_lib.GET, f"{BASE}/api/v1/health", json=api_wrap({"status": "ok"}))
    c = OHEClient(BASE, api_key="ohe_mykey")
    c.health()
    assert resp_lib.calls[0].request.headers["Authorization"] == "Bearer ohe_mykey"


@resp_lib.activate
def test_org_id_header():
    resp_lib.add(resp_lib.GET, f"{BASE}/api/v1/health", json=api_wrap({"status": "ok"}))
    c = OHEClient(BASE, token="tok", org_id="acme")
    c.health()
    assert resp_lib.calls[0].request.headers["X-Org-ID"] == "acme"
