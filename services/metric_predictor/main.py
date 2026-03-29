#!/usr/bin/env python3
"""METRIC PREDICTOR service (port 8008) - Prédictions métriques sur 7 jours."""

import sys
import os
import json
import logging
import threading
import time
from datetime import datetime
from typing import List, Optional

sys.path.insert(0, "/app")

from flask import Flask, jsonify, request
from shared import config, database
from shared.utils import SimpleLinearPredictor, calculate_risk_score, get_risk_level, format_forecast_for_db

logging.basicConfig(
    level=config.get_str("LOG_LEVEL"),
    format="%(asctime)s [%(name)s] %(levelname)s %(message)s",
)
logger = logging.getLogger("metric_predictor")

app = Flask(__name__)

# Configuration
PREDICTION_INTERVAL = config.get_int("METRIC_PREDICTION_INTERVAL_SEC", 60)
FORECAST_DAYS = config.get_int("METRIC_FORECAST_DAYS", 7)
MIN_HISTORY = config.get_int("METRIC_MIN_HISTORY", 10)

# Modèles
cpu_predictor = SimpleLinearPredictor()
memory_predictor = SimpleLinearPredictor()
latency_predictor = SimpleLinearPredictor()


def get_metric_history(limit: int = 100) -> List[tuple]:
    """Récupère l'historique des métriques depuis processed_data"""
    with database.get_conn() as conn:
        cursor = conn.cursor()
        cursor.execute("""
            SELECT timestamp, cpu_norm, memory_norm, latency_norm
            FROM processed_data
            WHERE cpu_norm IS NOT NULL
            ORDER BY timestamp ASC
            LIMIT ?
        """, (limit,))
        rows = cursor.fetchall()
        return [(row["timestamp"], row["cpu_norm"], row["memory_norm"], row["latency_norm"]) for row in rows]


def train_models(history: List[tuple]) -> bool:
    """Entraîne les modèles sur l'historique"""
    if len(history) < MIN_HISTORY:
        logger.warning(f"Historique insuffisant: {len(history)}/{MIN_HISTORY}")
        return False

    cpu_values = [h[1] for h in history if h[1] is not None]
    memory_values = [h[2] for h in history if h[2] is not None]
    latency_values = [h[3] for h in history if h[3] is not None]

    cpu_predictor.train(cpu_values)
    memory_predictor.train(memory_values)
    latency_predictor.train(latency_values)

    logger.info(f"Modèles entraînés sur {len(history)} points")
    return True


def make_predictions() -> Optional[dict]:
    """Génère les prédictions et les sauvegarde en base"""
    history = get_metric_history()
    if not train_models(history):
        return None

    cpu_forecast = cpu_predictor.predict(FORECAST_DAYS)
    memory_forecast = memory_predictor.predict(FORECAST_DAYS)
    latency_forecast = latency_predictor.predict(FORECAST_DAYS)

    if not cpu_forecast:
        return None

    # Calcul du risque global
    risk_score = calculate_risk_score(cpu_forecast[-3:])
    global_risk = get_risk_level(risk_score)

    forecast_data = {
        "cpu_forecast": format_forecast_for_db(cpu_forecast),
        "memory_forecast": format_forecast_for_db(memory_forecast),
        "latency_forecast": format_forecast_for_db(latency_forecast),
        "global_risk": global_risk,
        "risk_score": risk_score,
    }

    # Sauvegarde en base
    with database.get_conn() as conn:
        cursor = conn.cursor()
        cursor.execute("""
            INSERT INTO metric_predictions (timestamp, predicted_at, cpu_forecast, memory_forecast, latency_forecast, global_risk, risk_score)
            VALUES (?, ?, ?, ?, ?, ?, ?)
        """, (
            datetime.now().isoformat(),
            datetime.now().isoformat(),
            forecast_data["cpu_forecast"],
            forecast_data["memory_forecast"],
            forecast_data["latency_forecast"],
            forecast_data["global_risk"],
            forecast_data["risk_score"],
        ))
        conn.commit()

    logger.info(f"Prédictions sauvegardées - Risque: {global_risk} ({risk_score:.1f})")
    return forecast_data


def prediction_loop():
    """Boucle de prédiction en arrière-plan"""
    while True:
        try:
            make_predictions()
        except Exception as e:
            logger.error(f"Erreur dans la boucle de prédiction: {e}")
        time.sleep(PREDICTION_INTERVAL)


@app.route("/health", methods=["GET"])
def health():
    return jsonify({
        "status": "ok",
        "service": "metric-predictor",
        "version": "v3.0"
    })


@app.route("/predictions", methods=["GET"])
def get_predictions():
    """Récupère l'historique des prédictions"""
    limit = request.args.get("limit", 10, type=int)
    with database.get_conn() as conn:
        cursor = conn.cursor()
        cursor.execute("""
            SELECT id, timestamp, predicted_at, global_risk, risk_score,
                   cpu_forecast, memory_forecast, latency_forecast
            FROM metric_predictions
            ORDER BY timestamp DESC
            LIMIT ?
        """, (limit,))
        rows = cursor.fetchall()
        return jsonify([dict(row) for row in rows])


@app.route("/forecast", methods=["GET"])
def get_forecast():
    """Récupère la dernière prédiction"""
    with database.get_conn() as conn:
        cursor = conn.cursor()
        cursor.execute("""
            SELECT cpu_forecast, memory_forecast, latency_forecast, global_risk, risk_score
            FROM metric_predictions
            ORDER BY timestamp DESC
            LIMIT 1
        """)
        row = cursor.fetchone()
        if not row:
            return jsonify({"error": "Aucune prédiction disponible"}), 404

        return jsonify({
            "cpu_forecast": row["cpu_forecast"].split(","),
            "memory_forecast": row["memory_forecast"].split(","),
            "latency_forecast": row["latency_forecast"].split(","),
            "global_risk": row["global_risk"],
            "risk_score": row["risk_score"]
        })


@app.route("/train", methods=["POST"])
def force_train():
    """Force l'entraînement et la prédiction"""
    forecast = make_predictions()
    if forecast:
        return jsonify({"status": "success", "forecast": forecast})
    return jsonify({"status": "error", "message": "Entraînement échoué"}), 500


@app.route("/stats", methods=["GET"])
def get_stats():
    """Statistiques du modèle"""
    history = get_metric_history()
    with database.get_conn() as conn:
        cursor = conn.cursor()
        cursor.execute("SELECT COUNT(*) FROM metric_predictions")
        pred_count = cursor.fetchone()[0]

    return jsonify({
        "history_points": len(history),
        "min_history_required": MIN_HISTORY,
        "predictions_count": pred_count,
        "forecast_days": FORECAST_DAYS,
        "cpu_trained": cpu_predictor.is_trained,
        "memory_trained": memory_predictor.is_trained,
        "latency_trained": latency_predictor.is_trained,
    })


if __name__ == "__main__":
    # Initialisation de la base (tables v3 déjà ajoutées dans database.py)
    database.init_db()

    # Démarrer la boucle de prédiction
    thread = threading.Thread(target=prediction_loop, daemon=True)
    thread.start()

    port = config.get_int("METRIC_PREDICTOR_PORT", 8008)
    logger.info(f"Démarrage du Metric Predictor sur le port {port}")
    app.run(host="0.0.0.0", port=port, debug=False)
