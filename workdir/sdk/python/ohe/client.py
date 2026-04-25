"""OHE API client — thin wrapper around the OHE REST API using only stdlib."""

from __future__ import annotations

import json
import urllib.parse
import urllib.request
from dataclasses import dataclass, field, asdict
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional
from urllib.error import HTTPError


class APIError(Exception):
    """Raised when the OHE server returns HTTP 4xx or 5xx."""

    def __init__(self, status_code: int, body: str) -> None:
        self.status_code = status_code
        self.body = body
        super().__init__(f"OHE API error {status_code}: {body}")


@dataclass
class Metric:
    host: str
    name: str
    value: float
    timestamp: Optional[datetime] = field(default=None)

    def to_dict(self) -> Dict[str, Any]:
        ts = self.timestamp or datetime.now(timezone.utc)
        return {
            "host": self.host,
            "name": self.name,
            "value": self.value,
            "timestamp": ts.strftime("%Y-%m-%dT%H:%M:%SZ"),
        }


@dataclass
class MetricPoint:
    timestamp: datetime
    value: float


@dataclass
class Alert:
    id: str
    host: str
    metric: str = ""
    value: float = 0.0
    threshold: float = 0.0
    severity: str = ""
    status: str = ""
    created_at: Optional[datetime] = None


@dataclass
class AlertRule:
    name: str
    metric: str
    threshold: float
    severity: str


@dataclass
class Dashboard:
    id: str = ""
    name: str = ""
    panels: Any = None
    created_at: Optional[datetime] = None


@dataclass
class SLO:
    id: str = ""
    name: str = ""
    metric: str = ""
    target: float = 0.0


@dataclass
class SLOStatus:
    slo_id: str
    current: float
    target: float
    error_budget: float
    compliant: bool


class Client:
    """Authenticated OHE API client.

    Args:
        base_url: Root URL of the OHE server, e.g. ``"https://ohe.example.com"``.
        token: JWT bearer token or API key (``ohe_*``).
        timeout: Request timeout in seconds (default 30).
    """

    def __init__(self, base_url: str, token: str, timeout: float = 30.0) -> None:
        self._base = base_url.rstrip("/")
        self._token = token
        self._timeout = timeout

    # ------------------------------------------------------------------
    # Low-level helpers
    # ------------------------------------------------------------------

    def _request(
        self,
        method: str,
        path: str,
        body: Optional[Dict] = None,
    ) -> Any:
        url = self._base + path
        data: Optional[bytes] = None
        if body is not None:
            data = json.dumps(body).encode()

        req = urllib.request.Request(url, data=data, method=method)
        req.add_header("Authorization", f"Bearer {self._token}")
        if data is not None:
            req.add_header("Content-Type", "application/json")

        try:
            with urllib.request.urlopen(req, timeout=self._timeout) as resp:
                raw = resp.read()
                if not raw:
                    return None
                return json.loads(raw)
        except HTTPError as exc:
            body_text = exc.read().decode(errors="replace")
            raise APIError(exc.code, body_text) from exc

    def _get(self, path: str) -> Any:
        return self._request("GET", path)

    def _post(self, path: str, body: Optional[Dict] = None) -> Any:
        return self._request("POST", path, body or {})

    def _put(self, path: str, body: Dict) -> Any:
        return self._request("PUT", path, body)

    def _delete(self, path: str) -> None:
        self._request("DELETE", path)

    @staticmethod
    def _unwrap(resp: Any) -> Any:
        if isinstance(resp, dict) and "data" in resp:
            return resp["data"]
        return resp

    # ------------------------------------------------------------------
    # Health
    # ------------------------------------------------------------------

    def health(self) -> bool:
        """Return True if the server is reachable and healthy."""
        try:
            self._get("/api/v1/health")
            return True
        except (APIError, Exception):
            return False

    # ------------------------------------------------------------------
    # Metrics
    # ------------------------------------------------------------------

    def ingest(self, metrics: List[Metric]) -> None:
        """Push a list of metric observations to OHE."""
        self._post("/api/v1/ingest", {"metrics": [m.to_dict() for m in metrics]})

    def metric_range(
        self, name: str, host: str, from_: datetime, to: datetime
    ) -> List[MetricPoint]:
        """Query historical values for a metric between *from_* and *to*."""
        params = urllib.parse.urlencode(
            {
                "host": host,
                "from": from_.strftime("%Y-%m-%dT%H:%M:%SZ"),
                "to": to.strftime("%Y-%m-%dT%H:%M:%SZ"),
            }
        )
        name_enc = urllib.parse.quote(name, safe="")
        resp = self._get(f"/api/v1/metrics/{name_enc}/range?{params}")
        raw = self._unwrap(resp) or []
        return [
            MetricPoint(
                timestamp=datetime.fromisoformat(p["timestamp"].replace("Z", "+00:00")),
                value=float(p["value"]),
            )
            for p in raw
        ]

    # ------------------------------------------------------------------
    # Alerts
    # ------------------------------------------------------------------

    def list_alerts(self) -> List[Alert]:
        """Return all alerts (active and resolved)."""
        raw = self._unwrap(self._get("/api/v1/alerts")) or []
        return [self._parse_alert(a) for a in raw]

    def get_alert(self, alert_id: str) -> Alert:
        """Return a single alert by ID."""
        enc = urllib.parse.quote(alert_id, safe="")
        raw = self._unwrap(self._get(f"/api/v1/alerts/{enc}"))
        return self._parse_alert(raw)

    def acknowledge_alert(self, alert_id: str) -> None:
        """Acknowledge an alert by ID."""
        enc = urllib.parse.quote(alert_id, safe="")
        self._post(f"/api/v1/alerts/{enc}/acknowledge")

    @staticmethod
    def _parse_alert(d: Dict) -> Alert:
        return Alert(
            id=d.get("id", ""),
            host=d.get("host", ""),
            metric=d.get("metric", ""),
            value=float(d.get("value", 0)),
            threshold=float(d.get("threshold", 0)),
            severity=d.get("severity", ""),
            status=d.get("status", ""),
        )

    # ------------------------------------------------------------------
    # Alert rules
    # ------------------------------------------------------------------

    def list_alert_rules(self) -> List[AlertRule]:
        """Return all configured alert rules."""
        raw = self._unwrap(self._get("/api/v1/alert-rules")) or []
        return [AlertRule(**{k: r[k] for k in ("name", "metric", "threshold", "severity") if k in r}) for r in raw]

    def create_alert_rule(self, rule: AlertRule) -> AlertRule:
        """Create a new alert rule."""
        resp = self._post("/api/v1/alert-rules", asdict(rule))
        raw = self._unwrap(resp)
        return AlertRule(**{k: raw[k] for k in ("name", "metric", "threshold", "severity") if k in raw})

    def update_alert_rule(self, name: str, rule: AlertRule) -> None:
        """Replace an existing rule by name."""
        enc = urllib.parse.quote(name, safe="")
        self._put(f"/api/v1/alert-rules/{enc}", asdict(rule))

    def delete_alert_rule(self, name: str) -> None:
        """Remove a rule by name."""
        enc = urllib.parse.quote(name, safe="")
        self._delete(f"/api/v1/alert-rules/{enc}")

    # ------------------------------------------------------------------
    # Dashboards
    # ------------------------------------------------------------------

    def list_dashboards(self) -> List[Dashboard]:
        """Return all dashboards visible to the caller."""
        raw = self._unwrap(self._get("/api/v1/dashboards")) or []
        return [Dashboard(id=d.get("id", ""), name=d.get("name", "")) for d in raw]

    def get_dashboard(self, dashboard_id: str) -> Dashboard:
        """Return a single dashboard by ID."""
        enc = urllib.parse.quote(dashboard_id, safe="")
        raw = self._unwrap(self._get(f"/api/v1/dashboards/{enc}"))
        return Dashboard(id=raw.get("id", ""), name=raw.get("name", ""), panels=raw.get("panels"))

    def create_dashboard(self, dashboard: Dashboard) -> Dashboard:
        """Save a new dashboard and return it with server-assigned ID."""
        body = {"name": dashboard.name}
        if dashboard.panels is not None:
            body["panels"] = dashboard.panels
        raw = self._unwrap(self._post("/api/v1/dashboards", body))
        return Dashboard(id=raw.get("id", ""), name=raw.get("name", ""))

    def delete_dashboard(self, dashboard_id: str) -> None:
        """Delete a dashboard by ID."""
        enc = urllib.parse.quote(dashboard_id, safe="")
        self._delete(f"/api/v1/dashboards/{enc}")

    # ------------------------------------------------------------------
    # SLOs
    # ------------------------------------------------------------------

    def list_slos(self) -> List[SLO]:
        """Return all SLOs for the caller's org."""
        raw = self._unwrap(self._get("/api/v1/slos")) or []
        return [SLO(id=s.get("id", ""), name=s.get("name", ""), metric=s.get("metric", ""), target=float(s.get("target", 0))) for s in raw]

    def get_slo(self, slo_id: str) -> SLO:
        """Return a single SLO by ID."""
        enc = urllib.parse.quote(slo_id, safe="")
        raw = self._unwrap(self._get(f"/api/v1/slos/{enc}"))
        return SLO(id=raw.get("id", ""), name=raw.get("name", ""), metric=raw.get("metric", ""), target=float(raw.get("target", 0)))

    def create_slo(self, slo: SLO) -> SLO:
        """Create a new SLO."""
        body = {"name": slo.name, "metric": slo.metric, "target": slo.target}
        raw = self._unwrap(self._post("/api/v1/slos", body))
        return SLO(id=raw.get("id", ""), name=raw.get("name", ""), metric=raw.get("metric", ""), target=float(raw.get("target", 0)))

    def slo_status(self, slo_id: str) -> SLOStatus:
        """Return the current compliance state of an SLO."""
        enc = urllib.parse.quote(slo_id, safe="")
        raw = self._unwrap(self._get(f"/api/v1/slos/{enc}/status"))
        return SLOStatus(
            slo_id=raw.get("slo_id", slo_id),
            current=float(raw.get("current", 0)),
            target=float(raw.get("target", 0)),
            error_budget=float(raw.get("error_budget", 0)),
            compliant=bool(raw.get("compliant", False)),
        )

    def delete_slo(self, slo_id: str) -> None:
        """Delete an SLO by ID."""
        enc = urllib.parse.quote(slo_id, safe="")
        self._delete(f"/api/v1/slos/{enc}")
