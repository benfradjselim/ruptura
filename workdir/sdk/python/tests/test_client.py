"""Unit tests for the OHE Python SDK client."""

import json
import sys
import os
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, HTTPServer
from threading import Thread
from unittest import TestCase, main

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from ohe import Client, APIError, Metric, AlertRule, Dashboard, SLO


def _json_resp(handler, code: int, data) -> None:
    body = json.dumps({"data": data}).encode()
    handler.send_response(code)
    handler.send_header("Content-Type", "application/json")
    handler.send_header("Content-Length", str(len(body)))
    handler.end_headers()
    handler.wfile.write(body)


class StubServer:
    """Minimal in-process HTTP stub server for tests."""

    def __init__(self, routes: dict):
        self._routes = routes
        parent = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass  # silence request logs

            def _dispatch(self, method):
                key = (method, self.path.split("?")[0])
                handler_fn = parent._routes.get(key)
                if handler_fn is None:
                    self.send_response(404)
                    self.end_headers()
                    return
                body = None
                if self.headers.get("Content-Length"):
                    body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                handler_fn(self, body)

            def do_GET(self):
                self._dispatch("GET")

            def do_POST(self):
                self._dispatch("POST")

            def do_PUT(self):
                self._dispatch("PUT")

            def do_DELETE(self):
                self._dispatch("DELETE")

        self._server = HTTPServer(("127.0.0.1", 0), Handler)
        self._thread = Thread(target=self._server.serve_forever, daemon=True)
        self._thread.start()

    @property
    def url(self) -> str:
        host, port = self._server.server_address
        return f"http://{host}:{port}"

    def close(self):
        self._server.shutdown()


class TestHealth(TestCase):
    def test_healthy(self):
        srv = StubServer({
            ("GET", "/api/v1/health"): lambda h, _: _json_resp(h, 200, {"status": "ok"}),
        })
        c = Client(srv.url, "tok")
        self.assertTrue(c.health())
        srv.close()

    def test_unhealthy(self):
        srv = StubServer({
            ("GET", "/api/v1/health"): lambda h, _: (h.send_response(503), h.end_headers()),
        })
        c = Client(srv.url, "tok")
        self.assertFalse(c.health())
        srv.close()

    def test_unreachable(self):
        c = Client("http://127.0.0.1:1", "tok", timeout=0.1)
        self.assertFalse(c.health())


class TestIngest(TestCase):
    def test_ingest_ok(self):
        received = []

        def handle(h, body):
            received.append(body)
            h.send_response(204)
            h.end_headers()

        srv = StubServer({("POST", "/api/v1/ingest"): handle})
        c = Client(srv.url, "tok")
        c.ingest([Metric(host="web-01", name="cpu_percent", value=72.5)])
        self.assertEqual(len(received), 1)
        self.assertEqual(received[0]["metrics"][0]["host"], "web-01")
        srv.close()

    def test_ingest_quota_error(self):
        srv = StubServer({
            ("POST", "/api/v1/ingest"): lambda h, _: (
                h.send_response(402),
                h.send_header("Content-Length", "22"),
                h.end_headers(),
                h.wfile.write(b'{"error":"quota_exceeded"}'),
            ),
        })
        c = Client(srv.url, "tok")
        with self.assertRaises(APIError) as ctx:
            c.ingest([Metric("h", "cpu", 1.0)])
        self.assertEqual(ctx.exception.status_code, 402)
        srv.close()


class TestMetricRange(TestCase):
    def test_returns_points(self):
        points = [
            {"timestamp": "2024-01-01T00:00:00Z", "value": 55.0},
            {"timestamp": "2024-01-01T01:00:00Z", "value": 60.0},
        ]
        srv = StubServer({
            ("GET", "/api/v1/metrics/cpu_percent/range"): lambda h, _: _json_resp(h, 200, points),
        })
        c = Client(srv.url, "tok")
        result = c.metric_range("cpu_percent", "web-01",
                                datetime(2024, 1, 1, tzinfo=timezone.utc),
                                datetime(2024, 1, 2, tzinfo=timezone.utc))
        self.assertEqual(len(result), 2)
        self.assertAlmostEqual(result[0].value, 55.0)
        srv.close()


class TestAlerts(TestCase):
    def test_list_alerts(self):
        data = [{"id": "a1", "host": "db-01", "severity": "critical",
                 "metric": "cpu", "value": 95, "threshold": 90, "status": "active"}]
        srv = StubServer({("GET", "/api/v1/alerts"): lambda h, _: _json_resp(h, 200, data)})
        c = Client(srv.url, "tok")
        alerts = c.list_alerts()
        self.assertEqual(len(alerts), 1)
        self.assertEqual(alerts[0].id, "a1")
        srv.close()

    def test_get_alert(self):
        data = {"id": "a1", "host": "db-01", "severity": "critical",
                "metric": "cpu", "value": 95, "threshold": 90, "status": "active"}
        srv = StubServer({("GET", "/api/v1/alerts/a1"): lambda h, _: _json_resp(h, 200, data)})
        c = Client(srv.url, "tok")
        a = c.get_alert("a1")
        self.assertEqual(a.id, "a1")
        self.assertEqual(a.severity, "critical")
        srv.close()

    def test_acknowledge(self):
        called = []
        srv = StubServer({
            ("POST", "/api/v1/alerts/a1/acknowledge"): lambda h, _: (called.append(1), h.send_response(200), h.end_headers()),
        })
        c = Client(srv.url, "tok")
        c.acknowledge_alert("a1")
        self.assertEqual(len(called), 1)
        srv.close()


class TestAlertRules(TestCase):
    def test_crud(self):
        rules_store = [{"name": "r1", "metric": "cpu", "threshold": 90.0, "severity": "warning"}]

        def list_h(h, _):
            _json_resp(h, 200, rules_store)

        def create_h(h, body):
            rules_store.append(body)
            _json_resp(h, 201, body)

        def update_h(h, body):
            h.send_response(200)
            h.end_headers()

        def delete_h(h, _):
            h.send_response(200)
            h.end_headers()

        srv = StubServer({
            ("GET", "/api/v1/alert-rules"): list_h,
            ("POST", "/api/v1/alert-rules"): create_h,
            ("PUT", "/api/v1/alert-rules/r1"): update_h,
            ("DELETE", "/api/v1/alert-rules/r1"): delete_h,
        })
        c = Client(srv.url, "tok")
        listed = c.list_alert_rules()
        self.assertEqual(len(listed), 1)

        created = c.create_alert_rule(AlertRule("r2", "mem", 85.0, "critical"))
        self.assertEqual(created.name, "r2")

        c.update_alert_rule("r1", AlertRule("r1", "cpu", 95.0, "critical"))
        c.delete_alert_rule("r1")
        srv.close()


class TestDashboards(TestCase):
    def test_crud(self):
        srv = StubServer({
            ("GET", "/api/v1/dashboards"): lambda h, _: _json_resp(h, 200, [{"id": "d1", "name": "Overview"}]),
            ("GET", "/api/v1/dashboards/d1"): lambda h, _: _json_resp(h, 200, {"id": "d1", "name": "Overview"}),
            ("POST", "/api/v1/dashboards"): lambda h, b: _json_resp(h, 201, {"id": "d2", "name": b.get("name", "")}),
            ("DELETE", "/api/v1/dashboards/d1"): lambda h, _: (h.send_response(200), h.end_headers()),
        })
        c = Client(srv.url, "tok")
        listed = c.list_dashboards()
        self.assertEqual(listed[0].id, "d1")
        got = c.get_dashboard("d1")
        self.assertEqual(got.name, "Overview")
        created = c.create_dashboard(Dashboard(name="New"))
        self.assertEqual(created.id, "d2")
        c.delete_dashboard("d1")
        srv.close()


class TestSLOs(TestCase):
    def test_crud(self):
        srv = StubServer({
            ("GET", "/api/v1/slos"): lambda h, _: _json_resp(h, 200, [{"id": "s1", "name": "uptime", "metric": "health", "target": 99.9}]),
            ("GET", "/api/v1/slos/s1"): lambda h, _: _json_resp(h, 200, {"id": "s1", "name": "uptime", "metric": "health", "target": 99.9}),
            ("POST", "/api/v1/slos"): lambda h, b: _json_resp(h, 201, {"id": "s2", **b}),
            ("GET", "/api/v1/slos/s1/status"): lambda h, _: _json_resp(h, 200, {"slo_id": "s1", "current": 99.95, "target": 99.9, "error_budget": 0.05, "compliant": True}),
            ("DELETE", "/api/v1/slos/s1"): lambda h, _: (h.send_response(200), h.end_headers()),
        })
        c = Client(srv.url, "tok")
        listed = c.list_slos()
        self.assertEqual(listed[0].id, "s1")
        got = c.get_slo("s1")
        self.assertAlmostEqual(got.target, 99.9)
        created = c.create_slo(SLO(name="latency", metric="p99", target=99.0))
        self.assertEqual(created.id, "s2")
        st = c.slo_status("s1")
        self.assertTrue(st.compliant)
        c.delete_slo("s1")
        srv.close()


if __name__ == "__main__":
    main()
