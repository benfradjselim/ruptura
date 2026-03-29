"""SQLite connection manager with WAL mode for concurrent access."""
import logging
import os
import sqlite3
from contextlib import contextmanager
from typing import Generator

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Schema DDL – all tables for every service
# ---------------------------------------------------------------------------
SCHEMA_SQL = """
CREATE TABLE IF NOT EXISTS raw_metrics (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp   TEXT    NOT NULL,
    source      TEXT    NOT NULL DEFAULT 'prometheus',
    metric_name TEXT    NOT NULL,
    value       REAL    NOT NULL,
    labels      TEXT,
    processed   INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT    NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_raw_metrics_processed ON raw_metrics(processed);
CREATE INDEX IF NOT EXISTS idx_raw_metrics_timestamp ON raw_metrics(timestamp);

CREATE TABLE IF NOT EXISTS raw_logs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp   TEXT    NOT NULL,
    namespace   TEXT    NOT NULL,
    pod_name    TEXT    NOT NULL,
    container   TEXT    NOT NULL,
    log_level   TEXT,
    message     TEXT    NOT NULL,
    processed   INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT    NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_raw_logs_processed ON raw_logs(processed);

CREATE TABLE IF NOT EXISTS processed_data (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp       TEXT    NOT NULL,
    source_type     TEXT    NOT NULL,
    source_id       INTEGER NOT NULL,
    cpu_norm        REAL,
    memory_norm     REAL,
    latency_norm    REAL,
    error_rate      REAL,
    log_volume      REAL,
    restart_count   REAL,
    pod_name        TEXT,
    namespace       TEXT,
    trained         INTEGER NOT NULL DEFAULT 0,
    scored          INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT    NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_processed_trained ON processed_data(trained);
CREATE INDEX IF NOT EXISTS idx_processed_scored  ON processed_data(scored);

CREATE TABLE IF NOT EXISTS model_state (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    version      INTEGER NOT NULL UNIQUE,
    model_blob   BLOB    NOT NULL,
    n_trees      INTEGER NOT NULL,
    height       INTEGER NOT NULL,
    window_size  INTEGER NOT NULL,
    samples_seen INTEGER NOT NULL DEFAULT 0,
    created_at   TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS anomalies (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    processed_id   INTEGER NOT NULL,
    timestamp      TEXT    NOT NULL,
    anomaly_score  REAL    NOT NULL,
    is_anomaly     INTEGER NOT NULL,
    threshold_used REAL    NOT NULL,
    model_version  INTEGER NOT NULL,
    pod_name       TEXT,
    namespace      TEXT,
    created_at     TEXT    NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (processed_id) REFERENCES processed_data(id)
);
CREATE INDEX IF NOT EXISTS idx_anomalies_timestamp  ON anomalies(timestamp);
CREATE INDEX IF NOT EXISTS idx_anomalies_is_anomaly ON anomalies(is_anomaly);

CREATE TABLE IF NOT EXISTS config (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

INSERT OR IGNORE INTO config VALUES ('anomaly_threshold',    '0.7',  datetime('now'));
INSERT OR IGNORE INTO config VALUES ('collect_interval_sec', '15',   datetime('now'));
INSERT OR IGNORE INTO config VALUES ('dashboard_refresh_sec','5',    datetime('now'));
"""


def _make_connection(db_path: str) -> sqlite3.Connection:
    conn = sqlite3.connect(db_path, check_same_thread=False)
    conn.row_factory = sqlite3.Row
    conn.execute("PRAGMA journal_mode=WAL")
    conn.execute("PRAGMA busy_timeout=30000")
    conn.execute("PRAGMA synchronous=NORMAL")
    return conn


def init_db() -> None:
    """Create all tables if they don't exist."""
    db_path = os.environ.get("DB_PATH", "/data/mlops.db")
    path = os.path.dirname(db_path)
    if path:
        os.makedirs(path, exist_ok=True)
    conn = _make_connection(db_path)
    try:
        conn.executescript(SCHEMA_SQL)
        conn.commit()
    finally:
        conn.close()
    logger.info("Database initialized at %s", db_path)


@contextmanager
def get_conn() -> Generator[sqlite3.Connection, None, None]:
    """Context manager yielding a WAL-mode SQLite connection."""
    db_path = os.environ.get("DB_PATH", "/data/mlops.db")
    path = os.path.dirname(db_path)
    if path:
        os.makedirs(path, exist_ok=True)
    conn = _make_connection(db_path)
    try:
        yield conn
        conn.commit()
    except Exception:
        conn.rollback()
        raise
    finally:
        conn.close()

-- V3 Tables: Metric Predictions
CREATE TABLE IF NOT EXISTS metric_predictions (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp        TEXT    NOT NULL,
    predicted_at     TEXT    NOT NULL,
    cpu_forecast     TEXT    NOT NULL,
    memory_forecast  TEXT    NOT NULL,
    latency_forecast TEXT    NOT NULL,
    global_risk      TEXT    NOT NULL,
    risk_score       REAL    NOT NULL,
    created_at       TEXT    NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_metric_predictions_timestamp ON metric_predictions(timestamp);
