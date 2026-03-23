"""DASHBOARD service (port 8501) - Streamlit real-time anomaly visualization."""
import os
import sys
import time
from datetime import datetime

import pandas as pd
import plotly.express as px
import plotly.graph_objects as go
import requests
import streamlit as st

sys.path.insert(0, "/app")
from shared import config

EXPORTER_URL = config.get_str("EXPORTER_URL")
REFRESH_SEC = config.get_int("DASHBOARD_REFRESH_SEC")

st.set_page_config(
    page_title="MLOps Anomaly Detection",
    page_icon="🔍",
    layout="wide",
    initial_sidebar_state="expanded",
)

# Custom CSS for alert styling
st.markdown(
    """
    <style>
    .anomaly-alert { background-color: #ff4b4b; padding: 10px; border-radius: 5px; color: white; }
    .metric-card { background-color: #0e1117; border: 1px solid #333; border-radius: 8px; padding: 15px; }
    </style>
    """,
    unsafe_allow_html=True,
)


def _fetch(endpoint: str, params: dict | None = None) -> dict | None:
    try:
        resp = requests.get(f"{EXPORTER_URL}{endpoint}", params=params, timeout=5)
        resp.raise_for_status()
        return resp.json()
    except Exception as exc:
        st.warning(f"Exporter unavailable: {exc}")
        return None


def render_header() -> None:
    st.title("MLOps Anomaly Detection Dashboard")
    st.caption(f"Real-time monitoring | Refreshes every {REFRESH_SEC}s | {datetime.utcnow().strftime('%Y-%m-%d %H:%M:%S')} UTC")


def render_summary(summary: dict) -> None:
    col1, col2, col3 = st.columns(3)
    with col1:
        st.metric("Anomalies (24h)", summary.get("total_anomalies_24h", 0))
    with col2:
        rate = summary.get("anomaly_rate", 0.0)
        st.metric("Anomaly Rate", f"{rate:.1%}")
    with col3:
        st.metric("Total Predictions", summary.get("total_predictions", 0))


def render_anomaly_chart(series: list[dict]) -> None:
    if not series:
        st.info("No anomaly data yet.")
        return

    df = pd.DataFrame(series)
    df["timestamp"] = pd.to_datetime(df["timestamp"])
    df["color"] = df["is_anomaly"].map({True: "Anomaly", False: "Normal"})

    fig = px.scatter(
        df,
        x="timestamp",
        y="value",
        color="color",
        color_discrete_map={"Anomaly": "#ff4b4b", "Normal": "#00cc88"},
        title="Anomaly Scores Over Time",
        labels={"value": "Anomaly Score", "timestamp": "Time"},
    )
    fig.add_hline(
        y=float(os.environ.get("ANOMALY_THRESHOLD", "0.7")),
        line_dash="dash",
        line_color="orange",
        annotation_text="Threshold",
    )
    fig.update_layout(height=350, template="plotly_dark")
    st.plotly_chart(fig, use_container_width=True)


def render_metric_chart(series: list[dict]) -> None:
    if not series:
        st.info("No metric data yet.")
        return

    df = pd.DataFrame(series)
    df["timestamp"] = pd.to_datetime(df["timestamp"])

    fig = go.Figure()
    fig.add_trace(go.Scatter(
        x=df["timestamp"],
        y=df["value"],
        mode="lines",
        name="CPU (normalized)",
        line={"color": "#4b9eff"},
    ))
    fig.update_layout(
        title="CPU Usage (Normalized)",
        height=300,
        template="plotly_dark",
        yaxis_title="Value (0-1)",
    )
    st.plotly_chart(fig, use_container_width=True)


def render_recent_anomalies(anomalies: list[dict]) -> None:
    if not anomalies:
        st.success("No recent anomalies detected.")
        return

    st.subheader(f"Recent Anomalies ({len(anomalies)})")
    df = pd.DataFrame(anomalies)[["timestamp", "anomaly_score", "pod_name", "namespace"]]
    df["anomaly_score"] = df["anomaly_score"].round(4)
    df = df.rename(columns={
        "timestamp": "Time",
        "anomaly_score": "Score",
        "pod_name": "Pod",
        "namespace": "Namespace",
    })
    st.dataframe(df, use_container_width=True)


# ---------------------------------------------------------------------------
# Main app
# ---------------------------------------------------------------------------

def main() -> None:
    # Sidebar controls
    with st.sidebar:
        st.header("Controls")
        window = st.selectbox("Time Window", ["1h", "6h", "24h"], index=0)
        auto_refresh = st.checkbox("Auto Refresh", value=True)
        if st.button("Refresh Now"):
            st.rerun()

    render_header()

    data = _fetch("/dashboard-data", {"window": window})
    summary_data = _fetch("/summary")

    if summary_data:
        render_summary(summary_data)

    st.divider()

    col_left, col_right = st.columns([2, 1])

    with col_left:
        if data:
            render_anomaly_chart(data.get("anomaly_series", []))
            render_metric_chart(data.get("metric_series", []))

    with col_right:
        if data:
            render_recent_anomalies(data.get("recent_anomalies", []))
        else:
            st.info("Waiting for data from exporter...")

    if auto_refresh:
        time.sleep(REFRESH_SEC)
        st.rerun()


if __name__ == "__main__":
    main()
else:
    main()
