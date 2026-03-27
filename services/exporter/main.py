"""MLOps Exporter - Version corrigée"""
import logging
import sys
import os
from datetime import datetime, timezone, timedelta
import uvicorn
from fastapi import FastAPI, Response
import sqlite3

sys.path.insert(0, "/app")
from shared import config

logging.basicConfig(level=config.get_str("LOG_LEVEL", "INFO"))
logger = logging.getLogger("exporter")

app = FastAPI(title="MLOps Exporter")
DB_PATH = config.get_str("DB_PATH", "/data/mlops.db")

def get_db():
    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row
    return conn

@app.get("/health")
async def health():
    return {"status": "ok", "service": "exporter"}

@app.get("/summary")
async def summary():
    try:
        conn = get_db()
        # Total anomalies 24h
        total = conn.execute("SELECT COUNT(*) as count FROM anomalies WHERE timestamp >= datetime('now', '-24 hours')").fetchone()["count"]
        # Anomalies réelles (> seuil)
        real = conn.execute("SELECT COUNT(*) as count FROM anomalies WHERE timestamp >= datetime('now', '-24 hours') AND anomaly_score > 0.7").fetchone()["count"]
        conn.close()
        rate = real / total if total > 0 else 0.0
        return {
            "total_anomalies_24h": total,
            "anomaly_rate": rate,
            "total_predictions": total
        }
    except Exception as e:
        logger.error(f"Error in summary: {e}")
        return {"total_anomalies_24h": 0, "anomaly_rate": 0.0, "total_predictions": 0}

@app.get("/dashboard-data")
async def dashboard_data(window: str = "24h"):
    try:
        hours = 24
        if window.endswith("h"):
            try:
                hours = max(1, min(int(window[:-1]), 168))
            except:
                pass
        
        since = (datetime.now(timezone.utc) - timedelta(hours=hours)).isoformat()
        conn = get_db()
        
        # Anomaly series
        rows = conn.execute(
            "SELECT timestamp, anomaly_score, is_anomaly FROM anomalies "
            "WHERE timestamp >= ? ORDER BY timestamp LIMIT 500",
            (since,)
        ).fetchall()
        anomaly_series = [
            {"timestamp": r["timestamp"], "value": float(r["anomaly_score"]), 
             "is_anomaly": bool(r["is_anomaly"])} 
            for r in rows
        ]
        
        # Metric series
        rows = conn.execute(
            "SELECT timestamp, value FROM raw_metrics "
            "WHERE timestamp >= ? ORDER BY timestamp LIMIT 500",
            (since,)
        ).fetchall()
        metric_series = [
            {"timestamp": r["timestamp"], "value": float(r["value"])} 
            for r in rows
        ]
        
        # Recent anomalies
        rows = conn.execute(
            "SELECT id, timestamp, anomaly_score, is_anomaly, pod_name, namespace "
            "FROM anomalies ORDER BY timestamp DESC LIMIT 20"
        ).fetchall()
        recent = [
            {
                "id": r["id"],
                "timestamp": r["timestamp"],
                "anomaly_score": float(r["anomaly_score"]),
                "is_anomaly": bool(r["is_anomaly"]),
                "pod_name": r["pod_name"] or "unknown",
                "namespace": r["namespace"] or "mlops"
            }
            for r in rows
        ]
        
        # Summary
        total = conn.execute("SELECT COUNT(*) as count FROM anomalies WHERE timestamp >= datetime('now', '-24 hours')").fetchone()["count"]
        real = conn.execute("SELECT COUNT(*) as count FROM anomalies WHERE timestamp >= datetime('now', '-24 hours') AND anomaly_score > 0.7").fetchone()["count"]
        conn.close()
        
        return {
            "anomaly_series": anomaly_series,
            "metric_series": metric_series,
            "summary": {
                "total_anomalies_24h": total,
                "anomaly_rate": real / total if total > 0 else 0.0,
                "total_predictions": total
            },
            "recent_anomalies": recent,
            "window": window
        }
    except Exception as e:
        logger.error(f"Error in dashboard-data: {e}")
        return {"error": str(e)}

@app.get("/metrics")
async def metrics():
    from prometheus_client import generate_latest, CONTENT_TYPE_LATEST
    return Response(content=generate_latest(), media_type=CONTENT_TYPE_LATEST)

if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=8005)
