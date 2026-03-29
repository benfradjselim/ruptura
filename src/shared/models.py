"""Pydantic models shared across all MLOps microservices."""
from __future__ import annotations

from typing import List, Optional

from pydantic import BaseModel


# ---------------------------------------------------------------------------
# Common
# ---------------------------------------------------------------------------

class HealthResponse(BaseModel):
    status: str
    service: str


# ---------------------------------------------------------------------------
# Collector (port 8001)
# ---------------------------------------------------------------------------

class CollectResponse(BaseModel):
    collected: int
    timestamp: str


class CollectorStatus(BaseModel):
    last_run: Optional[str] = None
    total_collected: int = 0


# ---------------------------------------------------------------------------
# Processor (port 8002)
# ---------------------------------------------------------------------------

class ProcessRequest(BaseModel):
    batch_ids: Optional[List[int]] = None


class ProcessResponse(BaseModel):
    processed: int
    features: List[str]


# ---------------------------------------------------------------------------
# Trainer (port 8003)
# ---------------------------------------------------------------------------

class TrainRequest(BaseModel):
    processed_ids: Optional[List[int]] = None


class TrainResponse(BaseModel):
    trained_on: int
    model_version: int


class ModelInfo(BaseModel):
    version: int
    n_trees: int
    height: int
    window_size: int
    samples_seen: int


class ThresholdRequest(BaseModel):
    threshold: float


class ThresholdResponse(BaseModel):
    threshold: float


# ---------------------------------------------------------------------------
# Detector (port 8004)
# ---------------------------------------------------------------------------

class DetectRequest(BaseModel):
    processed_ids: Optional[List[int]] = None


class DetectionResult(BaseModel):
    id: int
    timestamp: str
    anomaly_score: float
    is_anomaly: bool
    pod_name: Optional[str] = None
    namespace: Optional[str] = None


class DetectResponse(BaseModel):
    results: List[DetectionResult]


# ---------------------------------------------------------------------------
# Exporter (port 8005) / Dashboard
# ---------------------------------------------------------------------------

class AnomalySummary(BaseModel):
    total_anomalies_24h: int
    anomaly_rate: float
    total_predictions: int


class TimeSeriesPoint(BaseModel):
    timestamp: str
    value: float
    is_anomaly: bool = False


class DashboardData(BaseModel):
    anomaly_series: List[TimeSeriesPoint]
    metric_series: List[TimeSeriesPoint]
    summary: AnomalySummary
    recent_anomalies: List[DetectionResult]
    window: str

# ---------------------------------------------------------------------------
# V3 Models: Metric Predictor
# ---------------------------------------------------------------------------

class MetricForecast(BaseModel):
    """Prédictions métriques sur 7 jours"""
    cpu_forecast: List[float]
    memory_forecast: List[float]
    latency_forecast: List[float]
    global_risk: str
    risk_score: float
    predicted_at: str


class MetricPredictionResponse(BaseModel):
    """Réponse pour l'endpoint des prédictions"""
    id: int
    timestamp: str
    predicted_at: str
    cpu_forecast: str
    memory_forecast: str
    latency_forecast: str
    global_risk: str
    risk_score: float
