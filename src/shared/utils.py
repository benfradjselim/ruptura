import logging
import os
from datetime import datetime
from typing import List, Optional

def setup_logging(service_name):
    """Configure le logging pour un service"""
    logging.basicConfig(
        level=logging.INFO,
        format=f'%(asctime)s - {service_name} - %(levelname)s - %(message)s'
    )
    return logging.getLogger(service_name)

def get_db_path():
    """Retourne le chemin de la base de données"""
    return os.environ.get('DB_PATH', '/data/mlops.db')

def now():
    """Retourne le timestamp actuel"""
    return datetime.now().isoformat()


# ---------------------------------------------------------------------------
# V3 Utility Functions - Metric Predictor
# ---------------------------------------------------------------------------

class SimpleLinearPredictor:
    """Régression linéaire simple pour les prédictions de métriques"""
    
    def __init__(self):
        self.slope = 0.0
        self.intercept = 0.0
        self.is_trained = False
    
    def train(self, values: List[float]) -> bool:
        """Entraîne le modèle sur les valeurs historiques"""
        if len(values) < 3:
            return False
        
        n = len(values)
        x = list(range(n))
        x_mean = sum(x) / n
        y_mean = sum(values) / n
        
        numerator = sum((x[i] - x_mean) * (values[i] - y_mean) for i in range(n))
        denominator = sum((x[i] - x_mean) ** 2 for i in range(n))
        
        if denominator != 0:
            self.slope = numerator / denominator
            self.intercept = y_mean - self.slope * x_mean
            self.is_trained = True
            return True
        
        return False
    
    def predict(self, steps: int = 7) -> Optional[List[float]]:
        """Prédit les valeurs futures"""
        if not self.is_trained:
            return None
        
        predictions = []
        for i in range(steps):
            pred = self.intercept + self.slope * (len(range(steps)) + i)
            predictions.append(max(0.0, min(1.0, pred)))
        
        return predictions


def calculate_risk_score(forecast_values: List[float]) -> float:
    """Calcule le score de risque basé sur les 3 dernières valeurs"""
    if not forecast_values:
        return 0.0
    
    weights = [0.5, 0.3, 0.2]
    recent = forecast_values[-3:] if len(forecast_values) >= 3 else forecast_values
    
    weighted_sum = sum(w * v for w, v in zip(weights[:len(recent)], recent))
    return weighted_sum * 100


def get_risk_level(score: float) -> str:
    """Convertit un score en niveau de risque"""
    if score > 70:
        return "CRITICAL"
    elif score > 40:
        return "HIGH"
    elif score > 20:
        return "MEDIUM"
    else:
        return "LOW"


def format_forecast_for_db(forecast: List[float]) -> str:
    """Formate la liste de prédictions pour stockage en base"""
    return ','.join([f"{v:.4f}" for v in forecast])


def parse_forecast_from_db(forecast_str: str) -> List[float]:
    """Parse une chaîne de prédictions depuis la base"""
    if not forecast_str:
        return []
    return [float(v) for v in forecast_str.split(',')]
