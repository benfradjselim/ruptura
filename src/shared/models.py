"""Shared Pydantic models for inter-service communication."""
from datetime import datetime
from typing import Optional
from pydantic import BaseModel


class HealthResponse(BaseModel):
    status: str
    service: str
    timestamp: datetime = None

    def model_post_init(self, __context) -> None:
        if self.timestamp is None:
            self.timestamp = datetime.utcnow()


class ProcessRequest(BaseModel):
    batch_ids: list[int] = []


class ProcessResponse(BaseModel):
    processed: int
    features: list[str]


class TrainRequest(BaseModel):
    processed_ids: list[int] = []


class TrainResponse(BaseModel):
    trained_on: int
    model_version: int


class DetectRequest(BaseModel):
    processed_ids: list[int] = []


class DetectionResult(BaseModel):
    id: int
    timestamp: str
    anomaly_score: float
    is_anomaly: bool
    pod_name: Optional[str] = None
    namespace: Optional[str] = None


class DetectResponse(BaseModel):
    results: list[DetectionResult]


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


class CollectResponse(BaseModel):
    collected: int
    timestamp: str


class CollectorStatus(BaseModel):
    last_run: Optional[str]
    total_collected: int


class AnomalySummary(BaseModel):
    total_anomalies_24h: int
    anomaly_rate: float
    total_predictions: int


class TimeSeriesPoint(BaseModel):
    timestamp: str
    value: float
    is_anomaly: bool = False


class DashboardData(BaseModel):
    anomaly_series: list[TimeSeriesPoint]
    metric_series: list[TimeSeriesPoint]
    summary: AnomalySummary
    recent_anomalies: list[DetectionResult]
    window: str
