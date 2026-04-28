"""RupturaClient — typed Python client for the Ruptura v6 REST API."""

from __future__ import annotations

from typing import Any, Dict, List, Optional

import requests

from .exceptions import RupturaError


class RupturaClient:
    """Thread-safe Ruptura v6 API client.

    Quick start::

        c = RupturaClient("http://localhost:8080", api_key="rpt_abc123")
        health = c.health()
    """

    def __init__(
        self,
        base_url: str,
        *,
        api_key: str = "",
        timeout: float = 30.0,
        session: Optional[requests.Session] = None,
    ) -> None:
        self._base_url = base_url.rstrip("/")
        self._api_key = api_key
        self._timeout = timeout
        self._session = session or requests.Session()

    def _headers(self) -> Dict[str, str]:
        h: Dict[str, str] = {"Content-Type": "application/json", "Accept": "application/json"}
        if self._api_key:
            h["Authorization"] = f"Bearer {self._api_key}"
        return h

    def _request(self, method: str, path: str, *, json: Any = None) -> Any:
        url = self._base_url + path
        resp = self._session.request(
            method, url, headers=self._headers(), json=json, timeout=self._timeout
        )
        if resp.status_code >= 400:
            try:
                msg = resp.json().get("error", resp.text[:200])
            except Exception:
                msg = resp.text[:200]
            raise RupturaError(resp.status_code, msg)
        if resp.status_code == 204 or not resp.content:
            return None
        return resp.json()

    def _get(self, path: str) -> Any:
        return self._request("GET", path)

    def _post(self, path: str, json: Any = None) -> Any:
        return self._request("POST", path, json=json)

    def _delete(self, path: str) -> None:
        self._request("DELETE", path)

    # ------------------------------------------------------------------
    # Health & Readiness
    # ------------------------------------------------------------------

    def health(self) -> Dict:
        """Return server health status (status, rupture_detection, message)."""
        return self._get("/api/v2/health")

    def ready(self) -> None:
        """Verify Ruptura is ready to serve traffic (k8s readinessProbe)."""
        self._get("/api/v2/ready")

    # ------------------------------------------------------------------
    # Metrics (Prometheus)
    # ------------------------------------------------------------------

    def metrics_prometheus(self) -> str:
        """Return raw Prometheus text exposition from /api/v2/metrics."""
        url = self._base_url + "/api/v2/metrics"
        h = {}
        if self._api_key:
            h["Authorization"] = f"Bearer {self._api_key}"
        resp = self._session.get(url, headers=h, timeout=self._timeout)
        if resp.status_code >= 400:
            raise RupturaError(resp.status_code, resp.text[:200])
        return resp.text

    # ------------------------------------------------------------------
    # Ruptures
    # ------------------------------------------------------------------

    def ruptures(self) -> List[Dict]:
        """Return all recent rupture events."""
        return self._get("/api/v2/ruptures") or []

    def rupture(self, host: str) -> Dict:
        """Return the latest rupture event for a specific host."""
        return self._get(f"/api/v2/rupture/{host}")

    # ------------------------------------------------------------------
    # KPIs
    # ------------------------------------------------------------------

    def kpi(self, name: str, host: str) -> Any:
        """Return the latest KPI value for (name, host)."""
        return self._get(f"/api/v2/kpi/{name}/{host}")

    # ------------------------------------------------------------------
    # Actions
    # ------------------------------------------------------------------

    def actions(self) -> List[Dict]:
        """Return all pending action recommendations."""
        return self._get("/api/v2/actions") or []

    def emergency_stop(self) -> Dict:
        """Trigger an emergency stop — halts all automated actions."""
        return self._post("/api/v2/actions/emergency-stop")

    # ------------------------------------------------------------------
    # Context
    # ------------------------------------------------------------------

    def add_context(self, entry: Dict) -> Dict:
        """Register a manual context entry (maintenance window, load test, etc.)."""
        return self._post("/api/v2/context", entry)

    def list_contexts(self) -> List[Dict]:
        """Return all active context entries."""
        return self._get("/api/v2/context") or []

    def delete_context(self, id: str) -> None:
        """Remove a manual context entry by ID."""
        self._delete(f"/api/v2/context/{id}")

    # ------------------------------------------------------------------
    # Explainability
    # ------------------------------------------------------------------

    def explain(self, rupture_id: str) -> Dict:
        """Return the XAI explanation for a rupture event."""
        return self._get(f"/api/v2/explain/{rupture_id}")

    def explain_formula(self, rupture_id: str) -> Dict:
        """Return the formula audit trace with all intermediate values."""
        return self._get(f"/api/v2/explain/{rupture_id}/formula")
