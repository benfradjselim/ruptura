"""Shared pytest fixtures for all MLOps service tests."""
import os
import sqlite3
import tempfile
from typing import Generator

import pytest

# Use in-memory DB for tests
os.environ["DB_PATH"] = ":memory:"
os.environ["LOG_LEVEL"] = "WARNING"


@pytest.fixture
def tmp_db(tmp_path) -> Generator[str, None, None]:
    """Temporary SQLite DB file for tests that need persistence."""
    db_path = str(tmp_path / "test.db")
    os.environ["DB_PATH"] = db_path
    import sys
    sys.path.insert(0, str(tmp_path.parent.parent / "src"))
    from shared import database
    database.init_db(db_path)
    yield db_path
    os.environ["DB_PATH"] = ":memory:"


@pytest.fixture
def db_conn(tmp_db):
    """Raw SQLite connection to the test DB."""
    conn = sqlite3.connect(tmp_db)
    conn.row_factory = sqlite3.Row
    conn.execute("PRAGMA journal_mode=WAL")
    yield conn
    conn.close()
