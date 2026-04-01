#!/usr/bin/env python3
"""
Dashboard V2 + V3 - MLOps Anomaly Detection
Affiche les métriques v2 et les prédictions v3 du metric_predictor
"""

import streamlit as st
import requests
import pandas as pd
import plotly.graph_objects as go
from datetime import datetime
import os

# Configuration
EXPORTER_URL = os.getenv('EXPORTER_URL', 'http://exporter:8005')
METRIC_PREDICTOR_URL = os.getenv('METRIC_PREDICTOR_URL', 'http://metric-predictor:8008')
DASHBOARD_REFRESH_SEC = int(os.getenv('DASHBOARD_REFRESH_SEC', '5'))
ANOMALY_THRESHOLD = float(os.getenv('ANOMALY_THRESHOLD', '0.7'))

st.set_page_config(
    page_title="MLOps Anomaly Detection v3",
    page_icon="🔮",
    layout="wide",
    initial_sidebar_state="expanded"
)

# Custom CSS
st.markdown("""
<style>
    .stButton > button { width: 100%; }
    .risk-critical { background-color: #ff4b4b; color: white; padding: 0.5rem; border-radius: 0.5rem; text-align: center; }
    .risk-high { background-color: #ffa500; color: white; padding: 0.5rem; border-radius: 0.5rem; text-align: center; }
    .risk-medium { background-color: #ffde59; color: black; padding: 0.5rem; border-radius: 0.5rem; text-align: center; }
    .risk-low { background-color: #00cc96; color: white; padding: 0.5rem; border-radius: 0.5rem; text-align: center; }
</style>
""", unsafe_allow_html=True)

# Sidebar
st.sidebar.title("🔮 MLOps Anomaly Detection v3")
st.sidebar.markdown("---")
page = st.sidebar.radio(
    "Navigation",
    ["📊 Dashboard Principal (v2)", "📈 Prédictions Métriques (v3)"]
)

st.sidebar.markdown("---")
st.sidebar.info(
    "**Version:** v3.0\n\n"
    "- Services v2: 8001-8005\n"
    "- Metric Predictor: 8008\n"
    "- Dashboard: 8501"
)

# ---------------------------------------------------------------------------
# Dashboard Principal (v2)
# ---------------------------------------------------------------------------

def fetch_anomalies():
    """Fetch anomalies from exporter"""
    try:
        # Correction: utiliser /dashboard-data au lieu de /dashboard/data
        response = requests.get(f"{EXPORTER_URL}/dashboard-data", timeout=5)
        if response.status_code == 200:
            return response.json()
    except Exception as e:
        st.error(f"Erreur connexion exporter: {e}")
    return None

def plot_metric_series(data, title, metric_key, color='blue'):
    """Plot metric time series"""
    fig = go.Figure()
    
    if data and 'metric_series' in data:
        series = data['metric_series']
        timestamps = [s['timestamp'] for s in series]
        values = [s.get(metric_key, 0) for s in series]
        
        fig.add_trace(go.Scatter(
            x=timestamps, y=values, mode='lines',
            name=title, line=dict(color=color, width=2)
        ))
        
        if 'anomaly_series' in data:
            anomalies = data['anomaly_series']
            anomaly_times = [a['timestamp'] for a in anomalies if a.get('is_anomaly')]
            anomaly_values = [a.get(metric_key, 0) for a in anomalies if a.get('is_anomaly')]
            if anomaly_values:
                fig.add_trace(go.Scatter(
                    x=anomaly_times, y=anomaly_values, mode='markers',
                    name='Anomalies', marker=dict(color='red', size=10, symbol='x')
                ))
    
    fig.update_layout(
        title=title,
        xaxis_title="Temps",
        yaxis_title="Valeur (normalisée)",
        height=400,
        hovermode='x unified'
    )
    return fig

def dashboard_principal():
    """Display v2 dashboard"""
    st.header("📊 Dashboard Principal - Détection d'Anomalies")
    
    data = fetch_anomalies()
    
    if data:
        col1, col2, col3 = st.columns(3)
        with col1:
            st.metric("Total Prédictions", data['summary']['total_predictions'])
        with col2:
            st.metric("Anomalies (24h)", data['summary']['total_anomalies_24h'],
                     delta=f"{data['summary']['anomaly_rate']:.1%}")
        with col3:
            st.metric("Seuil", ANOMALY_THRESHOLD)
        
        tab1, tab2, tab3 = st.tabs(["CPU", "Mémoire", "Anomalies Récentes"])
        
        with tab1:
            fig = plot_metric_series(data, "CPU Usage (normalisé)", "cpu_norm", 'red')
            st.plotly_chart(fig, use_container_width=True)
        
        with tab2:
            fig = plot_metric_series(data, "Mémoire Usage (normalisé)", "memory_norm", 'green')
            st.plotly_chart(fig, use_container_width=True)
        
        with tab3:
            if data.get('recent_anomalies'):
                df = pd.DataFrame(data['recent_anomalies'])
                st.dataframe(df[['timestamp', 'anomaly_score', 'pod_name']], use_container_width=True)
            else:
                st.info("Aucune anomalie récente")
    else:
        st.warning("Impossible de récupérer les données depuis l'exporter")

# ---------------------------------------------------------------------------
# V3: Metric Predictor
# ---------------------------------------------------------------------------

def fetch_metric_predictions():
    """Fetch metric predictions from v3 service"""
    try:
        response = requests.get(f"{METRIC_PREDICTOR_URL}/forecast", timeout=5)
        if response.status_code == 200:
            return response.json()
    except Exception as e:
        st.error(f"Erreur connexion metric-predictor: {e}")
    return None

def fetch_metric_predictions_history():
    """Fetch predictions history"""
    try:
        response = requests.get(f"{METRIC_PREDICTOR_URL}/predictions", timeout=5)
        if response.status_code == 200:
            return response.json()
    except Exception as e:
        return []

def fetch_predictor_stats():
    """Fetch predictor stats"""
    try:
        response = requests.get(f"{METRIC_PREDICTOR_URL}/stats", timeout=5)
        if response.status_code == 200:
            return response.json()
    except Exception as e:
        return None

def plot_forecast(forecast_data, title, values, color):
    """Plot forecast data"""
    fig = go.Figure()
    days = list(range(1, len(values) + 1))
    
    fig.add_trace(go.Scatter(
        x=days, y=values, mode='lines+markers',
        name='Prédiction', line=dict(color=color, width=3, dash='dot'),
        marker=dict(size=8, color=color)
    ))
    
    fig.update_layout(
        title=title,
        xaxis_title="Jours",
        yaxis_title="Valeur (normalisée)",
        height=400,
        xaxis=dict(tickmode='linear', tick0=1, dtick=1)
    )
    
    return fig

def metric_predictions_page():
    """Display v3 metric predictions"""
    st.header("📈 Prédictions Métriques - 7 Jours")
    
    forecast = fetch_metric_predictions()
    stats = fetch_predictor_stats()
    
    if forecast:
        risk = forecast.get('global_risk', 'LOW')
        risk_score = forecast.get('risk_score', 0)
        
        risk_class = {
            'CRITICAL': 'risk-critical',
            'HIGH': 'risk-high',
            'MEDIUM': 'risk-medium',
            'LOW': 'risk-low'
        }.get(risk, 'risk-low')
        
        col1, col2, col3, col4 = st.columns(4)
        with col1:
            st.markdown(f'<div class="{risk_class}">'
                       f'<h3>⚠️ Risque Global</h3>'
                       f'<h2>{risk}</h2>'
                       f'<p>Score: {risk_score:.1f}</p>'
                       f'</div>', unsafe_allow_html=True)
        
        with col2:
            st.metric("Horizon Prédiction", "7 jours")
        with col3:
            st.metric("Dernière mise à jour", datetime.now().strftime("%H:%M:%S"))
        with col4:
            if stats:
                st.metric("Points d'historique", stats.get('history_points', 0))
        
        cpu_values = [float(v) for v in forecast.get('cpu_forecast', [])]
        memory_values = [float(v) for v in forecast.get('memory_forecast', [])]
        latency_values = [float(v) for v in forecast.get('latency_forecast', [])]
        
        tab1, tab2, tab3 = st.tabs(["📊 CPU Forecast", "💾 Mémoire Forecast", "⏱️ Latence Forecast"])
        
        with tab1:
            if cpu_values:
                fig = plot_forecast(forecast, "Prédiction CPU - 7 Jours", cpu_values, 'red')
                st.plotly_chart(fig, use_container_width=True)
                
                df_cpu = pd.DataFrame({
                    'Jour': range(1, len(cpu_values) + 1),
                    'Prédiction CPU': [f"{v:.2%}" for v in cpu_values]
                })
                st.dataframe(df_cpu, use_container_width=True)
        
        with tab2:
            if memory_values:
                fig = plot_forecast(forecast, "Prédiction Mémoire - 7 Jours", memory_values, 'green')
                st.plotly_chart(fig, use_container_width=True)
                
                df_memory = pd.DataFrame({
                    'Jour': range(1, len(memory_values) + 1),
                    'Prédiction Mémoire': [f"{v:.2%}" for v in memory_values]
                })
                st.dataframe(df_memory, use_container_width=True)
        
        with tab3:
            if latency_values:
                fig = plot_forecast(forecast, "Prédiction Latence - 7 Jours", latency_values, 'blue')
                st.plotly_chart(fig, use_container_width=True)
                
                df_latency = pd.DataFrame({
                    'Jour': range(1, len(latency_values) + 1),
                    'Prédiction Latence': [f"{v:.2%}" for v in latency_values]
                })
                st.dataframe(df_latency, use_container_width=True)
        
        st.subheader("📜 Historique des Prédictions")
        history = fetch_metric_predictions_history()
        if history:
            df_history = pd.DataFrame(history)
            df_history['timestamp'] = pd.to_datetime(df_history['timestamp'])
            df_history = df_history.sort_values('timestamp', ascending=False)
            st.dataframe(
                df_history[['timestamp', 'global_risk', 'risk_score']].head(10),
                use_container_width=True
            )
            
            fig_risk = go.Figure()
            df_history_asc = df_history.sort_values('timestamp')
            fig_risk.add_trace(go.Scatter(
                x=df_history_asc['timestamp'],
                y=df_history_asc['risk_score'],
                mode='lines+markers',
                name='Score de risque',
                line=dict(color='orange', width=2)
            ))
            fig_risk.update_layout(
                title="Évolution du score de risque",
                xaxis_title="Date",
                yaxis_title="Score",
                height=300
            )
            st.plotly_chart(fig_risk, use_container_width=True)
    
    else:
        st.warning("⚠️ Service metric-predictor non disponible sur le port 8008")
        st.info("Le service metric-predictor est en cours de démarrage. Les prédictions seront disponibles dans quelques instants.")

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

if page == "📊 Dashboard Principal (v2)":
    dashboard_principal()
elif page == "📈 Prédictions Métriques (v3)":
    metric_predictions_page()
