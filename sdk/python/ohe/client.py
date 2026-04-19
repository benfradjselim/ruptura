"""OHEClient — typed Python client for the OHE REST API."""

from __future__ import annotations

from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

import requests

from .exceptions import OHEError


class OHEClient:
    """Thread-safe OHE API client.

    Quick start::

        c = OHEClient("https://ohe.example.com", api_key="ohe_abc123")
        health = c.health()

    Authentication: pass ``token`` for JWT bearer auth or ``api_key`` for
    long-lived programmatic access. Calling :meth:`login` stores the token
    automatically.
    """

    def __init__(
        self,
        base_url: str,
        *,
        token: str = "",
        api_key: str = "",
        org_id: str = "",
        timeout: float = 30.0,
        session: Optional[requests.Session] = None,
    ) -> None:
        self._base_url = base_url.rstrip("/")
        self._token = token
        self._api_key = api_key
        self._org_id = org_id
        self._timeout = timeout
        self._session = session or requests.Session()

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------

    def _headers(self) -> Dict[str, str]:
        h: Dict[str, str] = {"Content-Type": "application/json", "Accept": "application/json"}
        auth = self._token or self._api_key
        if auth:
            h["Authorization"] = f"Bearer {auth}"
        if self._org_id:
            h["X-Org-ID"] = self._org_id
        return h

    def _request(self, method: str, path: str, *, params: Optional[Dict] = None, json: Any = None) -> Any:
        url = self._base_url + path
        resp = self._session.request(
            method, url, headers=self._headers(), params=params, json=json, timeout=self._timeout
        )
        if resp.status_code >= 400:
            try:
                body = resp.json()
                err = body.get("error") or {}
            except Exception:
                err = {}
            raise OHEError(resp.status_code, err.get("code", ""), err.get("message", resp.text[:200]))
        if resp.status_code == 204 or not resp.content:
            return None
        body = resp.json()
        return body.get("data")

    def _get(self, path: str, params: Optional[Dict] = None) -> Any:
        return self._request("GET", path, params={k: v for k, v in (params or {}).items() if v is not None})

    def _post(self, path: str, json: Any = None) -> Any:
        return self._request("POST", path, json=json)

    def _put(self, path: str, json: Any) -> Any:
        return self._request("PUT", path, json=json)

    def _delete(self, path: str) -> None:
        self._request("DELETE", path)

    @staticmethod
    def _ts(dt: Optional[datetime]) -> Optional[str]:
        if dt is None:
            return None
        if dt.tzinfo is None:
            dt = dt.replace(tzinfo=timezone.utc)
        return dt.isoformat()

    # ------------------------------------------------------------------
    # Auth
    # ------------------------------------------------------------------

    def login(self, username: str, password: str) -> Dict:
        """Authenticate and store the JWT token for subsequent requests."""
        data = self._post("/api/v1/auth/login", {"username": username, "password": password})
        self._token = data["token"]
        return data

    def logout(self) -> None:
        """Invalidate the current JWT token server-side."""
        self._post("/api/v1/auth/logout")
        self._token = ""

    def refresh(self) -> Dict:
        """Exchange the current token for a fresh one."""
        data = self._post("/api/v1/auth/refresh")
        self._token = data["token"]
        return data

    # ------------------------------------------------------------------
    # Health
    # ------------------------------------------------------------------

    def health(self) -> Dict:
        """Return the full health status including version and component checks."""
        return self._get("/api/v1/health")

    def liveness(self) -> None:
        """Verify the process is alive (k8s livenessProbe)."""
        self._get("/api/v1/health/live")

    def readiness(self) -> None:
        """Verify OHE is ready to serve traffic (k8s readinessProbe)."""
        self._get("/api/v1/health/ready")

    def fleet(self) -> Dict:
        """Return aggregated health summary for all known hosts."""
        return self._get("/api/v1/fleet")

    # ------------------------------------------------------------------
    # Metrics
    # ------------------------------------------------------------------

    def metrics_list(self) -> List[str]:
        """Return all metric names known to the server."""
        return self._get("/api/v1/metrics") or []

    def metric_get(self, name: str, host: str = "") -> Dict:
        """Return the latest value for a metric."""
        return self._get(f"/api/v1/metrics/{name}", {"host": host or None})

    def metric_range(
        self, name: str, host: str, from_: datetime, to: datetime
    ) -> List[Dict]:
        """Return time-series data points for a metric."""
        return self._get(
            f"/api/v1/metrics/{name}/range",
            {"host": host, "from": self._ts(from_), "to": self._ts(to)},
        ) or []

    def metric_aggregate(
        self, name: str, host: str, agg: str, from_: datetime, to: datetime
    ) -> float:
        """Return an aggregated value (avg, min, max, p95, p99) for a metric."""
        return self._get(
            f"/api/v1/metrics/{name}/aggregate",
            {"host": host, "agg": agg, "from": self._ts(from_), "to": self._ts(to)},
        )

    # ------------------------------------------------------------------
    # KPIs
    # ------------------------------------------------------------------

    def kpi_get(self, host: str = "") -> Dict:
        """Return the current KPI snapshot for host."""
        return self._get("/api/v1/kpis", {"host": host or None})

    def kpi_predict(self, kpi_name: str, host: str = "", horizon: int = 60) -> Dict:
        """Return an ensemble forecast for a KPI (horizon in minutes)."""
        return self._get(f"/api/v1/kpis/{kpi_name}/predict", {"host": host or None, "horizon": horizon})

    def kpi_multi(self, hosts: List[str]) -> List[Dict]:
        """Return KPI snapshots for multiple hosts simultaneously."""
        return self._get("/api/v1/kpis/multi", {"host": hosts}) or []

    def explain(self, kpi: str, host: str = "") -> Dict:
        """Return the XAI explanation for why a KPI is in its current state."""
        return self._get(f"/api/v1/explain/{kpi}", {"host": host or None})

    def query(self, query: str, from_: datetime, to: datetime, step: int = 60) -> List[Dict]:
        """Execute a QQL query and return results."""
        return self._post(
            "/api/v1/query",
            {"query": query, "from": self._ts(from_), "to": self._ts(to), "step_seconds": step},
        ) or []

    # ------------------------------------------------------------------
    # Alerts
    # ------------------------------------------------------------------

    def alert_list(self, status: str = "") -> List[Dict]:
        """Return all alerts, optionally filtered by status."""
        return self._get("/api/v1/alerts", {"status": status or None}) or []

    def alert_get(self, id: str) -> Dict:
        """Return a single alert by ID."""
        return self._get(f"/api/v1/alerts/{id}")

    def alert_acknowledge(self, id: str) -> None:
        """Acknowledge an alert."""
        self._post(f"/api/v1/alerts/{id}/acknowledge")

    def alert_silence(self, id: str) -> None:
        """Silence an alert."""
        self._post(f"/api/v1/alerts/{id}/silence")

    def alert_delete(self, id: str) -> None:
        """Delete an alert."""
        self._delete(f"/api/v1/alerts/{id}")

    def alert_rule_list(self) -> List[Dict]:
        return self._get("/api/v1/alert-rules") or []

    def alert_rule_create(self, rule: Dict) -> Dict:
        return self._post("/api/v1/alert-rules", rule)

    def alert_rule_update(self, name: str, rule: Dict) -> Dict:
        return self._put(f"/api/v1/alert-rules/{name}", rule)

    def alert_rule_delete(self, name: str) -> None:
        self._delete(f"/api/v1/alert-rules/{name}")

    # ------------------------------------------------------------------
    # Dashboards
    # ------------------------------------------------------------------

    def dashboard_list(self) -> List[Dict]:
        return self._get("/api/v1/dashboards") or []

    def dashboard_get(self, id: str) -> Dict:
        return self._get(f"/api/v1/dashboards/{id}")

    def dashboard_create(self, dashboard: Dict) -> Dict:
        return self._post("/api/v1/dashboards", dashboard)

    def dashboard_update(self, id: str, dashboard: Dict) -> Dict:
        return self._put(f"/api/v1/dashboards/{id}", dashboard)

    def dashboard_delete(self, id: str) -> None:
        self._delete(f"/api/v1/dashboards/{id}")

    # ------------------------------------------------------------------
    # SLOs
    # ------------------------------------------------------------------

    def slo_list(self) -> List[Dict]:
        return self._get("/api/v1/slos") or []

    def slo_get(self, id: str) -> Dict:
        return self._get(f"/api/v1/slos/{id}")

    def slo_create(self, slo: Dict) -> Dict:
        return self._post("/api/v1/slos", slo)

    def slo_update(self, id: str, slo: Dict) -> Dict:
        return self._put(f"/api/v1/slos/{id}", slo)

    def slo_delete(self, id: str) -> None:
        self._delete(f"/api/v1/slos/{id}")

    def slo_status(self, id: str) -> Dict:
        """Return the live compliance state of an SLO."""
        return self._get(f"/api/v1/slos/{id}/status")

    def slo_all_status(self) -> List[Dict]:
        """Return live status for every SLO in the org."""
        return self._get("/api/v1/slos/status") or []

    # ------------------------------------------------------------------
    # Orgs
    # ------------------------------------------------------------------

    def org_list(self) -> List[Dict]:
        return self._get("/api/v1/orgs") or []

    def org_get(self, id: str) -> Dict:
        return self._get(f"/api/v1/orgs/{id}")

    def org_create(self, org: Dict) -> Dict:
        return self._post("/api/v1/orgs", org)

    def org_update(self, id: str, org: Dict) -> Dict:
        return self._put(f"/api/v1/orgs/{id}", org)

    def org_delete(self, id: str) -> None:
        self._delete(f"/api/v1/orgs/{id}")

    def org_member_list(self, org_id: str) -> List[Dict]:
        return self._get(f"/api/v1/orgs/{org_id}/members") or []

    def org_invite(self, org_id: str, username: str, role: str) -> None:
        self._post(f"/api/v1/orgs/{org_id}/members", {"username": username, "role": role})

    # ------------------------------------------------------------------
    # API Keys
    # ------------------------------------------------------------------

    def apikey_list(self) -> List[Dict]:
        return self._get("/api/v1/api-keys") or []

    def apikey_create(self, name: str, role: str, expires_in: str = "") -> Dict:
        """Create a new API key. Returns the plaintext key — store it immediately."""
        return self._post("/api/v1/api-keys", {"name": name, "role": role, "expires_in": expires_in})

    def apikey_delete(self, id: str) -> None:
        self._delete(f"/api/v1/api-keys/{id}")

    # ------------------------------------------------------------------
    # Ingest
    # ------------------------------------------------------------------

    def ingest(self, host: str, metrics: List[Dict], agent_id: str = "") -> None:
        """Push a batch of metrics from an agent (operator+)."""
        self._post("/api/v1/ingest", {"host": host, "metrics": metrics, "agent_id": agent_id})

    # ------------------------------------------------------------------
    # Logs & Traces
    # ------------------------------------------------------------------

    def log_query(
        self,
        service: str = "",
        from_: Optional[datetime] = None,
        to: Optional[datetime] = None,
        limit: int = 0,
    ) -> List[Dict]:
        params: Dict = {}
        if service:
            params["service"] = service
        if from_:
            params["from"] = self._ts(from_)
        if to:
            params["to"] = self._ts(to)
        if limit:
            params["limit"] = limit
        return self._get("/api/v1/logs", params) or []

    def trace_search(self, service: str = "", limit: int = 0) -> List[Dict]:
        return self._get("/api/v1/traces", {"service": service or None, "limit": limit or None}) or []

    def trace_get(self, trace_id: str) -> List[Dict]:
        return self._get(f"/api/v1/traces/{trace_id}") or []
